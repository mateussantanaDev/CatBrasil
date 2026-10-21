package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/emv"
	"catbrasil/backend/pkg/gateway"
	"catbrasil/backend/pkg/id"
	"catbrasil/backend/pkg/identity"
)

type PixDepositRequest struct {
	UserID string  `json:"userId"`
	Amount float64 `json:"amount"`
}

type PixDepositResponse struct {
	TxID              string  `json:"txId"`
	Amount            float64 `json:"amount"`
	PixCode           string  `json:"pixCode"`
	QRCodeURL         string  `json:"qrCodeUrl"`
	Status            string  `json:"status"`
	ExpirationMinutes int     `json:"expirationMinutes"`
	GatewayProvider   string  `json:"gatewayProvider,omitempty"`
	Message           string  `json:"message"`
}

type WebhookPayload struct {
	TxID       string  `json:"txId"`
	Status     string  `json:"status"`
	PaidAmount float64 `json:"paidAmount"`
	Signature  string  `json:"signature"`
}

func (w *WebhookPayload) UnmarshalJSON(data []byte) error {
	type alias WebhookPayload
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw struct {
		PaidAmount json.RawMessage `json:"paidAmount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cents, err := money.ParseJSONNumber(string(raw.PaidAmount))
	if err != nil {
		return err
	}
	*w = WebhookPayload(decoded)
	w.PaidAmount = cents.Float64()
	return nil
}

type WebhookResult struct {
	Success        bool    `json:"success"`
	Message        string  `json:"message"`
	Kind           string  `json:"kind,omitempty"`
	Status         string  `json:"status,omitempty"`
	TxID           string  `json:"txId,omitempty"`
	UserID         string  `json:"userId,omitempty"`
	NewBalance     float64 `json:"newBalance,omitempty"`
	PaidAmount     float64 `json:"paidAmount,omitempty"`
	Outcome        string  `json:"outcome,omitempty"`
	Error          string  `json:"error,omitempty"`
	ErrorCode      string  `json:"errorCode,omitempty"`
	HTTPStatusCode int     `json:"httpStatusCode,omitempty"`
}

type PayoutRequest struct {
	UserID           string  `json:"userId"`
	Amount           float64 `json:"amount"`
	PixKey           string  `json:"pixKey"`
	SecurityPassword string  `json:"password"`
	TwoFactorCode    string  `json:"twoFactorCode"`
	IdempotencyKey   string  `json:"-"`
}

func (r *PayoutRequest) UnmarshalJSON(data []byte) error {
	type alias PayoutRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw struct {
		Amount json.RawMessage `json:"amount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cents, err := money.ParseJSONNumber(string(raw.Amount))
	if err != nil {
		return err
	}
	*r = PayoutRequest(decoded)
	r.Amount = cents.Float64()
	return nil
}

type PayoutResponse struct {
	TxID            string  `json:"txId"`
	RequestedAmount float64 `json:"requestedAmount"`
	NetAmount       float64 `json:"netAmount"`
	Fee             float64 `json:"fee"`
	NewBalance      float64 `json:"newBalance"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
}

type PaymentUseCase struct {
	userRepo      domain.UserRepository
	txRepo        domain.TransactionRepository
	referralRepo  domain.ReferralRepository
	configRepo    domain.ConfigRepository
	logRepo       domain.LogRepository
	webhookSecret string
	options       Options
}

type Mode string

const (
	ModeSimulated Mode = "simulated"
	ModeSuitPay   Mode = "suitpay"
)

type Options struct {
	Mode                 Mode
	IdentityRepo         domain.IdentityRepository
	LiveEnabled          bool
	Environment          string
	PixSandboxURL        string
	FinancialRepository  FinancialRepository
	PayoutApplier        PayoutOutcomeApplier
	DepositApplier       DepositEventApplier
	DepositTracker       DepositSubmissionTracker
	DepositIssueRecorder DepositSubmissionTracker
}

func (uc *PaymentUseCase) newSuitPayClient(clientID, clientSecret string, sandbox bool, callbackURL string) *gateway.SuitPayClient {
	client := gateway.NewSuitPayClient(clientID, clientSecret, sandbox, callbackURL)
	if uc.options.Environment == "staging" && sandbox {
		client.BaseURL = uc.options.PixSandboxURL
	}
	return client
}

type FinancialRepository interface {
	ConfirmDeposit(context.Context, string, float64, string, string) (*domain.Transaction, float64, error)
	ReservePayout(context.Context, *domain.Transaction, func(*domain.User) error) (*domain.Transaction, float64, bool, error)
	ClaimPayout(context.Context, string) (*domain.Transaction, error)
	RecordPayoutSubmission(context.Context, string, string) (*domain.Transaction, error)
	ConfirmPayout(context.Context, string, string, float64) (*domain.Transaction, error)
	CancelPayout(context.Context, string, string, float64) (*domain.Transaction, error)
	ApplyPayoutCallback(context.Context, string, string, float64) (*domain.Transaction, error)
	GetPayoutByExternalID(context.Context, string) (*domain.Transaction, error)
}

type DepositQRRepository interface {
	AttachDepositQRCode(context.Context, string, string, string, string) (*domain.Transaction, error)
}

// PayoutOutcomeApplier applies an authenticated cash-out callback atomically and
// reports the exact outcome of THIS call: "applied" (financial state changed
// for the first time), "duplicate" (valid event already reconciled, no new
// effect) or "queued" (durable trail committed for reconciliation without any
// state or balance change). It is resolved optionally so existing
// FinancialRepository implementations and mocks keep compiling.
type PayoutOutcomeApplier interface {
	ApplyPayoutCallbackWithOutcome(ctx context.Context, externalID, status string, amount float64) (*domain.Transaction, string, error)
}

// DepositEventApplier applies deposit webhook events atomically at the database level.
// Outcome must be "applied", "duplicate", or "queued".
type DepositEventApplier interface {
	ApplyDepositEvent(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error)
}

// DepositSubmissionTracker tracks the submission lifecycle of external deposits,
// recording initiation, ambiguous submission issues, and intent persistence.
type DepositSubmissionTracker interface {
	MarkDepositSubmissionStarted(ctx context.Context, txID string) error
	MarkDepositSubmissionIssue(ctx context.Context, txID, reason, externalID string) error
	MarkDepositIntentSent(ctx context.Context, requestNumber, externalID string) (*domain.Transaction, error)
}

// DepositSubmissionIssueRecorder is retained as an alias for backward compatibility.
type DepositSubmissionIssueRecorder = DepositSubmissionTracker

func NewPaymentUseCase(
	userRepo domain.UserRepository,
	txRepo domain.TransactionRepository,
	referralRepo domain.ReferralRepository,
	configRepo domain.ConfigRepository,
	logRepo domain.LogRepository,
	webhookSecret string,
	options Options,
) *PaymentUseCase {
	if options.FinancialRepository == nil {
		if financial, ok := txRepo.(FinancialRepository); ok {
			options.FinancialRepository = financial
		}
	}
	if options.PayoutApplier == nil {
		// Auto-resolve PayoutApplier exclusively from the effective FinancialRepository.
		// Never fall back across repositories to txRepo when an explicit or inferred
		// FinancialRepository lacks PayoutOutcomeApplier, as that would split financial
		// truth across different stores. If the effective FinancialRepository does not
		// implement PayoutOutcomeApplier, leave options.PayoutApplier nil so payout
		// callbacks fail closed (PAYOUT_APPLIER_UNAVAILABLE, HTTP 500).
		if applier, ok := options.FinancialRepository.(PayoutOutcomeApplier); ok {
			options.PayoutApplier = applier
		}
	}
	if options.DepositApplier == nil {
		if applier, ok := txRepo.(DepositEventApplier); ok {
			options.DepositApplier = applier
		} else if applier, ok := options.FinancialRepository.(DepositEventApplier); ok {
			options.DepositApplier = applier
		}
	}
	if options.DepositTracker == nil {
		if tracker, ok := txRepo.(DepositSubmissionTracker); ok {
			options.DepositTracker = tracker
		} else if tracker, ok := options.FinancialRepository.(DepositSubmissionTracker); ok {
			options.DepositTracker = tracker
		}
	}
	if options.DepositIssueRecorder == nil && options.DepositTracker != nil {
		options.DepositIssueRecorder = options.DepositTracker
	}
	return &PaymentUseCase{
		userRepo:      userRepo,
		txRepo:        txRepo,
		referralRepo:  referralRepo,
		configRepo:    configRepo,
		logRepo:       logRepo,
		webhookSecret: webhookSecret,
		options:       options,
	}
}

func (uc *PaymentUseCase) recordDepositSubmissionIssue(txID, reason, externalID string) error {
	if uc.options.DepositTracker == nil {
		return fmt.Errorf("tracker de submissão indisponível")
	}
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return uc.options.DepositTracker.MarkDepositSubmissionIssue(recCtx, txID, reason, externalID)
}

// CreatePixDeposit gera uma cobrança SuitPay ou, somente em desenvolvimento,
// um código de demonstração sem valor financeiro.
func (uc *PaymentUseCase) CreatePixDeposit(ctx context.Context, req PixDepositRequest) (*PixDepositResponse, error) {
	if !validAmount(req.Amount) {
		return nil, fmt.Errorf("valor de depósito inválido")
	}
	if uc.options.Mode == ModeSimulated && uc.options.Environment != "development" {
		return nil, fmt.Errorf("%w: simulação indisponível neste ambiente", domain.ErrDepositUnavailable)
	}

	user, err := uc.userRepo.GetByID(ctx, req.UserID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}
	var person domain.IdentityPerson
	if uc.options.Mode == ModeSuitPay {
		if uc.options.IdentityRepo == nil {
			return nil, domain.ErrIdentityNotVerified
		}
		person, err = uc.options.IdentityRepo.VerifiedPerson(ctx, user.ID)
		if err != nil {
			return nil, err
		}
	}

	txID, err := newTransactionID("PIX")
	if err != nil {
		return nil, err
	}
	expirationMinutes := 15

	var pixCode string
	var qrCodeURL string
	var gatewayProvider = string(ModeSimulated)

	// 1. Busca configurações do Gateway
	var cfg *domain.SystemConfig
	if uc.configRepo != nil {
		cfg, err = uc.configRepo.GetConfig(ctx)
	} else if uc.options.Mode == ModeSuitPay {
		return nil, fmt.Errorf("%w: configuração SuitPay ausente", domain.ErrDepositUnavailable)
	}
	if err != nil && uc.options.Mode == ModeSuitPay {
		return nil, fmt.Errorf("%w: configuração do gateway indisponível: %v", domain.ErrDepositUnavailable, err)
	}
	if cfg != nil && cfg.Gateway.PixExpirationMinutes > 0 {
		expirationMinutes = cfg.Gateway.PixExpirationMinutes
	}

	// 2. Se SuitPay estiver configurada e ativa, gera via API oficial
	var externalID string
	if uc.options.Mode == ModeSuitPay {
		if uc.options.Environment == "production" && !uc.options.LiveEnabled {
			return nil, fmt.Errorf("%w: SuitPay real ainda não foi liberada", domain.ErrDepositUnavailable)
		}
		if cfg == nil || !cfg.Gateway.SuitPayEnabled || cfg.Gateway.SuitPayClientID == "" || cfg.Gateway.SuitPayClientSecret == "" {
			return nil, fmt.Errorf("%w: gateway SuitPay não configurado para cobranças", domain.ErrDepositUnavailable)
		}
		if _, ok := uc.txRepo.(DepositQRRepository); !ok {
			return nil, fmt.Errorf("%w: persistência de cobrança SuitPay indisponível", domain.ErrDepositUnavailable)
		}
		if uc.options.DepositTracker == nil {
			return nil, fmt.Errorf("%w: rastreador de submissão SuitPay indisponível", domain.ErrDepositUnavailable)
		}
		if uc.options.Environment == "production" && cfg.Gateway.SuitPaySandbox {
			return nil, fmt.Errorf("%w: sandbox SuitPay não é permitido em production", domain.ErrDepositUnavailable)
		}
		if uc.options.Environment != "production" && !cfg.Gateway.SuitPaySandbox {
			return nil, fmt.Errorf("%w: ambiente não produtivo exige sandbox SuitPay", domain.ErrDepositUnavailable)
		}
		if uc.options.Environment == "staging" && uc.options.PixSandboxURL == "" {
			return nil, fmt.Errorf("%w: staging exige sandbox SuitPay isolado", domain.ErrDepositUnavailable)
		}
		gatewayProvider = string(ModeSuitPay)
		intent := &domain.Transaction{ID: txID, UserID: user.ID, Tipo: "Depósito PIX", Valor: req.Amount, Status: domain.TxStatusPendente, GatewayProvider: gatewayProvider, CreatedAt: time.Now()}
		if err := uc.txRepo.Create(ctx, intent); err != nil {
			return nil, fmt.Errorf("%w: erro ao registrar intenção PIX: %v", domain.ErrDepositUnavailable, err)
		}

		if err := uc.options.DepositTracker.MarkDepositSubmissionStarted(ctx, txID); err != nil {
			return nil, fmt.Errorf("%w: erro ao registrar início da submissão: %v", domain.ErrDepositUnavailable, err)
		}

		suitClient := uc.newSuitPayClient(
			cfg.Gateway.SuitPayClientID,
			cfg.Gateway.SuitPayClientSecret,
			cfg.Gateway.SuitPaySandbox,
			cfg.Gateway.SuitPayCallbackURL,
		)

		spResp, err := suitClient.CreatePixPayment(ctx, gateway.SuitPayPixPaymentRequest{
			RequestNumber: txID,
			Amount:        req.Amount,
			DueDate:       time.Now().Add(time.Duration(expirationMinutes) * time.Minute).Format("2006-01-02"),
			Client: gateway.SuitPayClientData{
				Name:        person.LegalName,
				Document:    person.CPF,
				Email:       person.Email,
				PhoneNumber: user.Phone,
			},
		})

		if err != nil {
			reason := fmt.Sprintf("gateway_submission_issue: %v", err)
			if recErr := uc.recordDepositSubmissionIssue(txID, reason, ""); recErr != nil {
				return nil, fmt.Errorf("%w: falha no gateway e na persistência de auditoria", domain.ErrDepositUnavailable)
			}
			return nil, fmt.Errorf("%w: falha de comunicação com o gateway", domain.ErrDepositUnavailable)
		}
		if spResp == nil || spResp.PaymentCode == "" || spResp.IDTransaction == "" {
			var extID string
			if spResp != nil {
				extID = spResp.IDTransaction
			}
			reason := "gateway_incomplete_response: missing payment code or transaction id"
			if spResp != nil && spResp.Message != "" {
				reason = fmt.Sprintf("gateway_incomplete_response: %s", spResp.Message)
			}
			if recErr := uc.recordDepositSubmissionIssue(txID, reason, extID); recErr != nil {
				return nil, fmt.Errorf("%w: resposta SuitPay incompleta e falha na persistência de auditoria", domain.ErrDepositUnavailable)
			}
			return nil, fmt.Errorf("%w: resposta de cobrança SuitPay incompleta", domain.ErrDepositUnavailable)
		}
		externalID = spResp.IDTransaction
		pixCode = spResp.PaymentCode
		if spResp.PaymentCodeBase64 != "" {
			qrCodeURL = fmt.Sprintf("data:image/png;base64,%s", spResp.PaymentCodeBase64)
		}

		if _, err := uc.options.DepositTracker.MarkDepositIntentSent(ctx, txID, externalID); err != nil {
			reason := fmt.Sprintf("failed_to_mark_intent_sent: %v", err)
			if recErr := uc.recordDepositSubmissionIssue(txID, reason, externalID); recErr != nil {
				return nil, fmt.Errorf("%w: erro ao registrar envio de intenção de depósito e falha na persistência de auditoria", domain.ErrDepositUnavailable)
			}
			return nil, fmt.Errorf("%w: erro ao registrar envio de intenção de depósito", domain.ErrDepositUnavailable)
		}
	} else if uc.options.Mode == ModeSimulated {
		pixKey := fmt.Sprintf("pix-%s@catbrasil.app", txID)
		pixCode = emv.GeneratePixPayload(pixKey, txID, "CatBrasil Mining", "SAO PAULO", req.Amount)
		externalID = fmt.Sprintf("SIM-%s", txID)
	} else {
		return nil, fmt.Errorf("%w: modo PIX inválido", domain.ErrDepositUnavailable)
	}
	if uc.options.Mode == ModeSuitPay {
		expirationMinutes = 0
	} // A API informa dueDate, sem garantia de expiração em minutos.

	now := time.Now()
	tx := &domain.Transaction{
		ID:              txID,
		UserID:          user.ID,
		Tipo:            "Depósito PIX",
		Valor:           req.Amount,
		Status:          domain.TxStatusPendente,
		ExternalID:      &externalID,
		GatewayProvider: gatewayProvider,
		PixCode:         &pixCode,
		QRCodeURL:       &qrCodeURL,
		CreatedAt:       now,
	}

	if uc.options.Mode == ModeSuitPay {
		tx, err = uc.txRepo.(DepositQRRepository).AttachDepositQRCode(ctx, txID, externalID, pixCode, qrCodeURL)
		if err != nil {
			reason := fmt.Sprintf("failed_to_attach_qr: %v", err)
			if recErr := uc.recordDepositSubmissionIssue(txID, reason, externalID); recErr != nil {
				return nil, fmt.Errorf("%w: cobrança gerada, mas erro ao associar QR e falha na persistência de auditoria", domain.ErrDepositUnavailable)
			}
			return nil, fmt.Errorf("%w: cobrança gerada, mas associação local pendente", domain.ErrDepositUnavailable)
		}
	} else if err := uc.txRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("erro ao registrar transação de depósito: %w", err)
	}

	message := "QR Code PIX gerado com sucesso. Aguardando pagamento."
	if uc.options.Mode == ModeSimulated {
		message = "PIX de demonstração gerado; não realize pagamento real."
	} else if uc.options.Environment == "staging" {
		message = "Cobrança PIX de sandbox gerada; não realize pagamento real."
	}
	return &PixDepositResponse{
		TxID:              txID,
		Amount:            req.Amount,
		PixCode:           pixCode,
		QRCodeURL:         qrCodeURL,
		Status:            string(tx.Status),
		ExpirationMinutes: expirationMinutes,
		GatewayProvider:   gatewayProvider,
		Message:           message,
	}, nil
}

// GetUserTransactions retorna o histórico de transações com filtro opcional
func (uc *PaymentUseCase) GetUserTransactions(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	if uc.txRepo == nil {
		return nil, nil
	}
	return uc.txRepo.GetByUserID(ctx, userID, filter)
}

// ProcessSuitPayWebhook processa a notificação oficial recebida da SuitPay
func (uc *PaymentUseCase) ProcessSuitPayWebhook(ctx context.Context, raw []byte) *WebhookResult {
	if uc.options.Mode != ModeSuitPay {
		return &WebhookResult{
			Success:        false,
			Error:          "callback SuitPay indisponível neste modo",
			ErrorCode:      "SUITPAY_DISABLED",
			HTTPStatusCode: http.StatusUnauthorized,
		}
	}
	cfg, err := uc.configRepo.GetConfig(ctx)
	if err != nil {
		return &WebhookResult{
			Success:        false,
			Error:          "configuração do gateway temporariamente indisponível",
			ErrorCode:      "CONFIG_UNAVAILABLE",
			HTTPStatusCode: http.StatusServiceUnavailable,
		}
	}
	if cfg == nil || cfg.Gateway.SuitPayClientSecret == "" {
		return &WebhookResult{
			Success:        false,
			Error:          "segredo do gateway não configurado",
			ErrorCode:      "CONFIG_UNAVAILABLE",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}
	if uc.options.Environment == "production" && cfg.Gateway.SuitPaySandbox {
		return &WebhookResult{
			Success:        false,
			Error:          "sandbox SuitPay não é permitido em production",
			ErrorCode:      "SANDBOX_NOT_ALLOWED_IN_PROD",
			HTTPStatusCode: http.StatusUnauthorized,
		}
	}
	cb, err := gateway.VerifySuitPayWebhook(raw, cfg.Gateway.SuitPayClientSecret)
	if err != nil {
		return &WebhookResult{
			Success:        false,
			Error:          err.Error(),
			ErrorCode:      "INVALID_SIGNATURE",
			HTTPStatusCode: http.StatusUnauthorized,
		}
	}
	if strings.EqualFold(cb.TypeTransaction, "PIX_CASHOUT") || strings.EqualFold(cb.TypeTransaction, "CASHOUT") {
		return uc.processPayoutCallback(ctx, cb)
	}

	requestNumber := strings.TrimSpace(cb.RequestNumber)
	externalID := formatTransactionID(cb.IDTransaction)
	rawStatus := strings.ToUpper(strings.TrimSpace(cb.StatusTransaction))
	amount := cb.Value

	if requestNumber == "" && externalID == "" {
		return &WebhookResult{
			Success:        false,
			Error:          "identificador da transação ausente no callback SuitPay",
			ErrorCode:      "MISSING_IDENTIFIER",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}

	// Fail closed in all environments if DepositApplier interface is absent
	if uc.options.DepositApplier == nil {
		return &WebhookResult{
			Success:        false,
			Error:          "repositório com suporte a eventos de depósito indisponível",
			ErrorCode:      "DEPOSIT_APPLIER_UNAVAILABLE",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}

	rawType := strings.ToUpper(strings.TrimSpace(cb.TypeTransaction))
	var statusToSend string
	if rawType != "PIX" {
		const prefix = "UNSUPPORTED_TYPE:"
		maxLen := 32 - len(prefix)
		suffix := rawType
		if suffix == "" {
			suffix = "EMPTY"
		}
		if len(suffix) > maxLen {
			suffix = suffix[:maxLen]
		}
		statusToSend = prefix + suffix
	} else {
		statusToSend = rawStatus
	}
	if len(statusToSend) > 32 {
		statusToSend = statusToSend[:32]
	}

	// Apply authenticated event through the transactional repository contract
	tx, outcome, err := uc.options.DepositApplier.ApplyDepositEvent(ctx, requestNumber, externalID, statusToSend, amount)
	if err != nil {
		// Persistence failed: NEVER confirm webhook to the gateway.
		// Return generic message without leaking SQL / internal database errors.
		return &WebhookResult{
			Success:        false,
			Error:          "falha temporária ao persistir evento de pagamento",
			ErrorCode:      "PERSISTENCE_FAILED",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}

	var userID, currentTxID string
	var txStatus string
	if tx != nil {
		userID = tx.UserID
		currentTxID = tx.ID
		txStatus = string(tx.Status)
	}
	if currentTxID == "" {
		currentTxID = requestNumber
	}

	switch outcome {
	case "applied":
		if statusToSend == "CHARGEBACK" {
			if uc.logRepo != nil && userID != "" {
				_ = uc.logRepo.LogSecurity(ctx, &userID, fmt.Sprintf("PIX_CHARGEBACK_R$%.2f", amount), "", "", "")
			}
			return &WebhookResult{
				Success:        true,
				Outcome:        "applied",
				Message:        fmt.Sprintf("Chargeback PIX de R$ %.2f processado", amount),
				Kind:           "chargeback",
				Status:         txStatus,
				TxID:           currentTxID,
				UserID:         userID,
				PaidAmount:     amount,
				HTTPStatusCode: http.StatusOK,
			}
		}
		if statusToSend == "PAID_OUT" {
			// Out-of-order transition: the repository applied PAID_OUT and
			// immediately reversed it because an earlier CHARGEBACK was already
			// recorded for the same external id. The durable outcome is a
			// reversal, never a successful deposit.
			if strings.EqualFold(txStatus, string(domain.TxStatusCancelado)) {
				if uc.logRepo != nil && userID != "" {
					_ = uc.logRepo.LogSecurity(ctx, &userID, fmt.Sprintf("PIX_CHARGEBACK_R$%.2f", amount), "", "", "")
				}
				return &WebhookResult{
					Success:        true,
					Outcome:        "applied",
					Message:        fmt.Sprintf("Pagamento PIX de R$ %.2f estornado por chargeback antecipado", amount),
					Kind:           "chargeback",
					Status:         txStatus,
					TxID:           currentTxID,
					UserID:         userID,
					PaidAmount:     amount,
					HTTPStatusCode: http.StatusOK,
				}
			}
			if uc.logRepo != nil && userID != "" {
				_ = uc.logRepo.LogSecurity(ctx, &userID, fmt.Sprintf("PIX_CONFIRMED_R$%.2f", amount), "", "", "")
			}
			return &WebhookResult{
				Success:        true,
				Outcome:        "applied",
				Message:        fmt.Sprintf("Pagamento PIX de R$ %.2f confirmado com sucesso!", amount),
				Kind:           "deposit",
				Status:         txStatus,
				TxID:           currentTxID,
				UserID:         userID,
				PaidAmount:     amount,
				HTTPStatusCode: http.StatusOK,
			}
		}
		return &WebhookResult{
			Success:        true,
			Outcome:        "applied",
			Message:        fmt.Sprintf("Evento PIX com status %s registrado", statusToSend),
			Kind:           "deposit",
			Status:         txStatus,
			TxID:           currentTxID,
			UserID:         userID,
			PaidAmount:     amount,
			HTTPStatusCode: http.StatusOK,
		}

	case "duplicate":
		// A replayed event must never look like new credit: the outcome stays
		// "duplicate" and a cancelled transaction is classified as chargeback.
		dupKind := "deposit"
		dupMessage := fmt.Sprintf("Transação #%s já havia sido processada anteriormente (Idempotente)", currentTxID)
		if strings.EqualFold(txStatus, string(domain.TxStatusCancelado)) {
			dupKind = "chargeback"
			dupMessage = fmt.Sprintf("Transação #%s já estornada por chargeback (Idempotente)", currentTxID)
		}
		return &WebhookResult{
			Success:        true,
			Outcome:        "duplicate",
			Message:        dupMessage,
			Kind:           dupKind,
			Status:         txStatus,
			TxID:           currentTxID,
			UserID:         userID,
			PaidAmount:     amount,
			HTTPStatusCode: http.StatusOK,
		}

	case "queued":
		return &WebhookResult{
			Success:        true,
			Outcome:        "queued",
			Message:        "Evento autenticado recebido e enfileirado para conciliação",
			Kind:           "deposit",
			Status:         "queued",
			TxID:           currentTxID,
			UserID:         userID,
			PaidAmount:     amount,
			HTTPStatusCode: http.StatusOK,
		}

	default:
		return &WebhookResult{
			Success:        true,
			Outcome:        outcome,
			Message:        fmt.Sprintf("Evento registrado com desfecho: %s", outcome),
			Kind:           "deposit",
			Status:         txStatus,
			TxID:           currentTxID,
			UserID:         userID,
			PaidAmount:     amount,
			HTTPStatusCode: http.StatusOK,
		}
	}
}

// ProcessWebhook aceita apenas notificações assinadas no modo simulado.
func (uc *PaymentUseCase) ProcessWebhook(ctx context.Context, payload WebhookPayload, signature string) *WebhookResult {
	if uc.options.Mode != ModeSimulated || uc.webhookSecret == "" || signature == "" {
		return &WebhookResult{
			Error:          "webhook genérico indisponível ou sem assinatura",
			ErrorCode:      "WEBHOOK_UNAVAILABLE",
			HTTPStatusCode: http.StatusUnauthorized,
		}
	}
	payloadString := fmt.Sprintf("%s:%s:%.2f", payload.TxID, payload.Status, payload.PaidAmount)
	if !crypto.VerifyHMACSHA256(payloadString, signature, uc.webhookSecret) {
		return &WebhookResult{
			Error:          "Assinatura HMAC inválida. Webhook rejeitado por segurança.",
			ErrorCode:      "INVALID_SIGNATURE",
			HTTPStatusCode: http.StatusUnauthorized,
		}
	}
	return uc.processConfirmedDeposit(ctx, payload, "SIM-"+payload.TxID)
}

func (uc *PaymentUseCase) ProcessSimulatedWebhook(ctx context.Context, payload WebhookPayload) *WebhookResult {
	if uc.options.Environment != "development" || uc.options.Mode != ModeSimulated {
		return &WebhookResult{
			Error:          "simulação PIX indisponível",
			ErrorCode:      "SIMULATION_UNAVAILABLE",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}
	return uc.processConfirmedDeposit(ctx, payload, "SIM-"+payload.TxID)
}

func (uc *PaymentUseCase) processConfirmedDeposit(ctx context.Context, payload WebhookPayload, externalID string) *WebhookResult {
	status := strings.ToUpper(strings.TrimSpace(payload.Status))
	if status != "PAID" && status != "PAID_OUT" && status != "APPROVED" && status != "CONFIRMED" && status != "SUCESSO" {
		return &WebhookResult{
			Error:          "status de pagamento não confirmado",
			ErrorCode:      "UNCONFIRMED_STATUS",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}
	if uc.options.FinancialRepository != nil {
		if uc.options.Mode == ModeSuitPay && (externalID == "" || externalID == "<nil>") {
			return &WebhookResult{
				Error:          "ID da SuitPay ausente",
				ErrorCode:      "MISSING_EXTERNAL_ID",
				HTTPStatusCode: http.StatusBadRequest,
			}
		}
		tx, balance, err := uc.options.FinancialRepository.ConfirmDeposit(ctx, payload.TxID, payload.PaidAmount, string(uc.options.Mode), externalID)
		if err != nil {
			return &WebhookResult{
				Error:          err.Error(),
				ErrorCode:      "PERSISTENCE_FAILED",
				HTTPStatusCode: http.StatusInternalServerError,
			}
		}
		return &WebhookResult{
			Success:        true,
			Outcome:        "applied",
			Message:        "Pagamento PIX conciliado",
			Kind:           "deposit",
			Status:         string(tx.Status),
			TxID:           tx.ID,
			UserID:         tx.UserID,
			NewBalance:     balance,
			PaidAmount:     payload.PaidAmount,
			HTTPStatusCode: http.StatusOK,
		}
	}
	if uc.options.Environment == "production" {
		return &WebhookResult{
			Error:          "repositório financeiro transacional indisponível",
			ErrorCode:      "FINANCIAL_REPO_UNAVAILABLE",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}
	tx, err := uc.txRepo.GetByIDForUpdate(ctx, payload.TxID)
	if err != nil || tx == nil {
		return &WebhookResult{
			Success:        false,
			Error:          fmt.Sprintf("Transação #%s não encontrada", payload.TxID),
			ErrorCode:      "TRANSACTION_NOT_FOUND",
			HTTPStatusCode: http.StatusNotFound,
		}
	}

	if tx.Status == domain.TxStatusSucesso {
		return &WebhookResult{
			Success:        true,
			Outcome:        "duplicate",
			Kind:           "deposit",
			Status:         string(tx.Status),
			Message:        fmt.Sprintf("Transação #%s já havia sido confirmada anteriormente (Idempotente)", payload.TxID),
			TxID:           tx.ID,
			UserID:         tx.UserID,
			HTTPStatusCode: http.StatusOK,
		}
	}
	if tx.Status != domain.TxStatusPendente || tx.Tipo != "Depósito PIX" {
		return &WebhookResult{
			Error:          "transação não elegível para crédito PIX",
			ErrorCode:      "INELIGIBLE_TRANSACTION",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}
	if uc.options.Mode == ModeSuitPay && tx.GatewayProvider != string(ModeSuitPay) {
		return &WebhookResult{
			Error:          "provedor da transação não confere",
			ErrorCode:      "PROVIDER_MISMATCH",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}

	paidAmount := payload.PaidAmount
	if !validAmount(paidAmount) || !money.EqualFloat64(paidAmount, tx.Valor) {
		return &WebhookResult{
			Error:          "valor do pagamento não confere com a cobrança",
			ErrorCode:      "AMOUNT_MISMATCH",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}

	user, err := uc.userRepo.GetByIDForUpdate(ctx, tx.UserID)
	if err != nil || user == nil {
		return &WebhookResult{
			Success:        false,
			Error:          "Operador associado à transação não encontrado",
			ErrorCode:      "USER_NOT_FOUND",
			HTTPStatusCode: http.StatusNotFound,
		}
	}

	// Verifica comissão de primeiro depósito antes de qualquer escrita persistida
	var comID string
	var refToUpdate *domain.Referral
	var referrerToUpdate *domain.User
	var commPercent float64
	var commission float64

	if uc.referralRepo != nil {
		ref, _ := uc.referralRepo.GetByReferredName(ctx, user.Username)
		if ref != nil && ref.Status != domain.ReferralStatusAtivo {
			referrer, _ := uc.userRepo.GetByIDForUpdate(ctx, ref.ReferrerID)
			if referrer != nil && !referrer.ReferralBonusSuspended && !referrer.MultiAccountFlag {
				var idErr error
				comID, idErr = id.New("com")
				if idErr != nil {
					return &WebhookResult{
						Success:        false,
						Outcome:        "failed",
						Error:          "falha ao processar evento financeiro",
						ErrorCode:      "INTERNAL_ERROR",
						HTTPStatusCode: http.StatusInternalServerError,
					}
				}
				commPercent = 5.0
				if referrer.CustomReferralCommissionPercent > 0 {
					commPercent = referrer.CustomReferralCommissionPercent
				}
				if referrer.Nivel >= 12 && referrer.Level12BonusSuspended {
					commPercent = 5.0
				}
				commission = paidAmount * (commPercent / 100.0)
				refToUpdate = ref
				referrerToUpdate = referrer
			}
		}
	}

	newBalance := user.Balance + paidAmount
	newRollover := user.RequiredRollover + paidAmount
	user.Balance = newBalance
	user.RequiredRollover = newRollover
	_ = uc.userRepo.Update(ctx, user)

	now := time.Now()
	tx.Status = domain.TxStatusSucesso
	tx.ValorEfetivoPago = &paidAmount
	tx.PaidAt = &now
	_ = uc.txRepo.Update(ctx, tx)

	if refToUpdate != nil && referrerToUpdate != nil {
		refToUpdate.Status = domain.ReferralStatusAtivo
		refToUpdate.Deposito = paidAmount
		_ = uc.referralRepo.Update(ctx, refToUpdate)

		referrerToUpdate.Balance += commission
		_ = uc.userRepo.Update(ctx, referrerToUpdate)

		refTx := &domain.Transaction{
			ID:        comID,
			UserID:    referrerToUpdate.ID,
			Tipo:      fmt.Sprintf("Comissão %.1f%% (1º Depósito %s)", commPercent, user.Username),
			Valor:     commission,
			Status:    domain.TxStatusSucesso,
			CreatedAt: now,
			PaidAt:    &now,
		}
		_ = uc.txRepo.Create(ctx, refTx)
	}

	if uc.logRepo != nil {
		_ = uc.logRepo.LogSecurity(ctx, &user.ID, fmt.Sprintf("PIX_CONFIRMED_R$%.2f", paidAmount), user.IPAddress, user.DeviceFingerprint, "")
	}

	return &WebhookResult{
		Success:        true,
		Outcome:        "applied",
		Kind:           "deposit",
		Status:         string(tx.Status),
		Message:        fmt.Sprintf("Pagamento PIX de R$ %.2f confirmado com sucesso!", paidAmount),
		TxID:           tx.ID,
		UserID:         user.ID,
		NewBalance:     newBalance,
		PaidAmount:     paidAmount,
		HTTPStatusCode: http.StatusOK,
	}
}

// RequestPayout processa a solicitação de saque com conferência de senha financeira, rollover e suporte a SuitPay
func (uc *PaymentUseCase) RequestPayout(ctx context.Context, req PayoutRequest) (*PayoutResponse, error) {
	if uc.options.Environment == "production" && uc.options.Mode == ModeSuitPay && !uc.options.LiveEnabled {
		return nil, fmt.Errorf("SuitPay real ainda não foi liberada para este release")
	}
	if uc.options.FinancialRepository != nil {
		return uc.requestPayoutAtomic(ctx, req)
	}
	if uc.options.Mode == ModeSuitPay {
		return nil, fmt.Errorf("caminho legado não transacional não suporta modo SuitPay")
	}
	if uc.options.Environment == "production" {
		return nil, fmt.Errorf("repositório financeiro transacional indisponível")
	}
	if !validAmount(req.Amount) || req.Amount < 50.0 {
		return nil, domain.ErrMinimumWithdrawalAmount
	}

	cfg, _ := uc.configRepo.GetConfig(ctx)
	if cfg != nil && cfg.GlobalWithdrawalsLocked {
		return nil, domain.ErrGlobalWithdrawLocked
	}

	user, err := uc.userRepo.GetByIDForUpdate(ctx, req.UserID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	if user.WithdrawLocked {
		return nil, domain.ErrWithdrawLocked
	}

	if user.Balance < req.Amount {
		return nil, domain.ErrInsufficientBalance
	}

	if user.SecurityPassword == "" {
		return nil, domain.ErrSecurityPasswordNotSet
	}

	if !crypto.CheckPassword(strings.TrimSpace(req.SecurityPassword), user.SecurityPassword) {
		return nil, domain.ErrInvalidSecurityPassword
	}

	if !user.HasRequiredRollover() {
		diff := user.RequiredRollover - user.CurrentRollover
		return nil, fmt.Errorf("para realizar saques, é necessário ter um giro mínimo de R$ %.2f em frotas. Faltam R$ %.2f em aluguéis",
			user.RequiredRollover, diff)
	}

	if req.Amount > 500.0 && user.TwoFactorEnabled {
		if !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(req.TwoFactorCode)) {
			return nil, domain.ErrInvalid2FACode
		}
	}

	amountCents, err := money.Positive(req.Amount)
	if err != nil {
		return nil, err
	}
	feeCents, err := amountCents.Percent(500)
	if err != nil {
		return nil, err
	}
	if user.FreeWithdrawActive {
		feeCents = 0
	}
	fee := feeCents.Float64()
	netAmount := (amountCents - feeCents).Float64()

	txID, err := id.New("WDR")
	if err != nil {
		return nil, err
	}

	newBalance := user.Balance - req.Amount
	user.Balance = newBalance
	user.FreeWithdrawActive = false
	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	now := time.Now()
	pixKey := strings.TrimSpace(req.PixKey)
	status := domain.TxStatusPendente

	// Se a aprovação automática estiver habilitada
	if cfg != nil && cfg.AutoApproveEnabled && req.Amount <= cfg.AutoApproveWithdrawalsUnder {
		if uc.options.Mode == ModeSimulated && uc.options.Environment != "production" {
			// Em modo simulado fora de produção, aprova automaticamente direto
			status = domain.TxStatusSucesso
		}
	}

	tx := &domain.Transaction{
		ID:        txID,
		UserID:    user.ID,
		Tipo:      "Saque PIX",
		Valor:     -req.Amount,
		Status:    status,
		PixKey:    &pixKey,
		NetAmount: &netAmount,
		FeeAmount: &fee,
		CreatedAt: now,
	}
	if status == domain.TxStatusSucesso {
		tx.PaidAt = &now
	}

	if err := uc.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	_ = uc.logRepo.LogSecurity(ctx, &user.ID, fmt.Sprintf("WITHDRAW_REQUESTED_R$%.2f", req.Amount), user.IPAddress, user.DeviceFingerprint, "")

	return &PayoutResponse{
		TxID:            txID,
		RequestedAmount: req.Amount,
		NetAmount:       netAmount,
		Fee:             fee,
		NewBalance:      newBalance,
		Status:          string(status),
		Message:         fmt.Sprintf("Solicitação de saque PIX de R$ %.2f registrada! Líquido a receber: R$ %.2f", req.Amount, netAmount),
	}, nil
}

func newTransactionID(prefix string) (string, error) {
	return id.New(prefix)
}

func formatTransactionID(raw interface{}) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if v == math.Trunc(v) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		s := strings.TrimSpace(fmt.Sprint(raw))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func validAmount(amount float64) bool {
	_, err := money.Positive(amount)
	return err == nil
}

func (uc *PaymentUseCase) requestPayoutAtomic(ctx context.Context, req PayoutRequest) (*PayoutResponse, error) {
	if !validAmount(req.Amount) || req.Amount < 50 {
		return nil, domain.ErrMinimumWithdrawalAmount
	}
	pixKey := strings.TrimSpace(req.PixKey)
	if pixKey == "" || len(pixKey) > 255 {
		return nil, fmt.Errorf("chave PIX inválida")
	}
	key := strings.TrimSpace(req.IdempotencyKey)
	if key == "" || len(key) > 128 {
		return nil, fmt.Errorf("Idempotency-Key obrigatório e limitado a 128 caracteres")
	}
	cfg, err := uc.configRepo.GetConfig(ctx)
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("configuração financeira indisponível")
	}
	if cfg.GlobalWithdrawalsLocked {
		return nil, domain.ErrGlobalWithdrawLocked
	}
	if uc.options.Mode == ModeSuitPay && (!cfg.Gateway.SuitPayEnabled || cfg.Gateway.SuitPayClientID == "" || cfg.Gateway.SuitPayClientSecret == "" || (uc.options.Environment == "production" && cfg.Gateway.SuitPaySandbox)) {
		return nil, fmt.Errorf("gateway de saque indisponível")
	}
	if uc.options.Mode != ModeSuitPay && uc.options.Mode != ModeSimulated {
		return nil, fmt.Errorf("modo de pagamento inválido")
	}
	if uc.options.Environment == "production" && uc.options.Mode == ModeSimulated {
		return nil, fmt.Errorf("saques simulados indisponíveis em produção")
	}
	if uc.options.Mode == ModeSuitPay {
		if uc.options.IdentityRepo == nil {
			return nil, fmt.Errorf("serviço de verificação de identidade indisponível")
		}
		person, err := uc.options.IdentityRepo.VerifiedPerson(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("identidade não verificada para saque: %w", err)
		}
		if _, err := identity.NormalizeCPF(person.CPF); err != nil {
			return nil, fmt.Errorf("documento de identidade inválido para saque")
		}
	}
	id, err := newTransactionID("WDR")
	if err != nil {
		return nil, err
	}
	withdrawal := &domain.Transaction{ID: id, UserID: req.UserID, Tipo: "Saque PIX", Valor: -req.Amount, PixKey: &pixKey, IdempotencyKey: &key, GatewayProvider: string(uc.options.Mode)}
	validate := func(user *domain.User) error {
		if user.WithdrawLocked {
			return domain.ErrWithdrawLocked
		}
		if user.SecurityPassword == "" {
			return domain.ErrSecurityPasswordNotSet
		}
		if !crypto.CheckPassword(strings.TrimSpace(req.SecurityPassword), user.SecurityPassword) {
			return domain.ErrInvalidSecurityPassword
		}
		if !user.HasRequiredRollover() {
			return fmt.Errorf("giro mínimo em frotas não atingido: faltam R$ %.2f", user.RequiredRollover-user.CurrentRollover)
		}
		if req.Amount > 500 && user.TwoFactorEnabled && !crypto.VerifyTOTPCode(user.TwoFactorSecret, strings.TrimSpace(req.TwoFactorCode)) {
			return domain.ErrInvalid2FACode
		}
		return nil
	}
	reserved, balance, replay, err := uc.options.FinancialRepository.ReservePayout(ctx, withdrawal, validate)
	if err != nil {
		return nil, err
	}
	if !replay && cfg.AutoApproveEnabled && req.Amount <= cfg.AutoApproveWithdrawalsUnder {
		reserved, err = uc.dispatchPayout(ctx, reserved, cfg)
		if err != nil {
			return nil, err
		}
	}
	if uc.logRepo != nil && !replay {
		_ = uc.logRepo.LogSecurity(ctx, &req.UserID, fmt.Sprintf("WITHDRAW_RESERVED_R$%.2f", req.Amount), "", "", "")
	}
	return &PayoutResponse{TxID: reserved.ID, RequestedAmount: -reserved.Valor, NetAmount: *reserved.NetAmount, Fee: *reserved.FeeAmount, NewBalance: balance, Status: string(reserved.Status), Message: "Solicitação de saque registrada; aguarde confirmação da transferência."}, nil
}

// ApprovePayout sends a pending manual withdrawal through the same gateway
// path as automatic withdrawals. Calling it twice never sends twice.
func (uc *PaymentUseCase) ApprovePayout(ctx context.Context, id string) (*domain.Transaction, error) {
	if uc.options.FinancialRepository == nil {
		return nil, fmt.Errorf("repositório financeiro transacional indisponível")
	}
	withdrawal, err := uc.txRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if withdrawal.Tipo != "Saque PIX" {
		return nil, domain.ErrInvalidTransactionStatus
	}
	if withdrawal.Status != domain.TxStatusPendente {
		return withdrawal, nil
	}
	if withdrawal.GatewayProvider != string(uc.options.Mode) {
		return nil, fmt.Errorf("provedor de gateway do saque (%s) incompatível com modo configurado (%s)", withdrawal.GatewayProvider, uc.options.Mode)
	}
	if uc.options.Mode == ModeSimulated && uc.options.Environment == "production" {
		return nil, fmt.Errorf("saques simulados indisponíveis em produção")
	}
	cfg, err := uc.configRepo.GetConfig(ctx)
	if err != nil || cfg == nil || cfg.GlobalWithdrawalsLocked {
		return nil, domain.ErrGlobalWithdrawLocked
	}
	return uc.dispatchPayout(ctx, withdrawal, cfg)
}

func (uc *PaymentUseCase) dispatchPayout(ctx context.Context, withdrawal *domain.Transaction, cfg *domain.SystemConfig) (*domain.Transaction, error) {
	if uc.options.Environment == "production" && uc.options.Mode == ModeSuitPay && !uc.options.LiveEnabled {
		return nil, fmt.Errorf("SuitPay real ainda não foi liberada para este release")
	}
	if uc.options.Mode == ModeSuitPay && (!cfg.Gateway.SuitPayEnabled || cfg.Gateway.SuitPayClientID == "" || cfg.Gateway.SuitPayClientSecret == "" || (uc.options.Environment == "production" && cfg.Gateway.SuitPaySandbox)) {
		return nil, fmt.Errorf("gateway de saque indisponível")
	}
	if uc.options.Mode == ModeSimulated && uc.options.Environment == "production" {
		return nil, fmt.Errorf("simulação indisponível em produção")
	}

	var normalizedCPF string
	if uc.options.Mode == ModeSuitPay {
		if uc.options.IdentityRepo == nil {
			return nil, fmt.Errorf("serviço de verificação de identidade indisponível")
		}
		person, err := uc.options.IdentityRepo.VerifiedPerson(ctx, withdrawal.UserID)
		if err != nil {
			return nil, fmt.Errorf("identidade não verificada para envio de saque: %w", err)
		}
		cpf, err := identity.NormalizeCPF(person.CPF)
		if err != nil {
			return nil, fmt.Errorf("documento de identidade inválido para envio de saque")
		}
		normalizedCPF = cpf
	}

	claimed, err := uc.options.FinancialRepository.ClaimPayout(ctx, withdrawal.ID)
	if err != nil {
		current, getErr := uc.txRepo.GetByID(ctx, withdrawal.ID)
		if getErr == nil && current.Status != domain.TxStatusPendente {
			return current, nil
		}
		return nil, err
	}
	if uc.options.Mode == ModeSimulated && uc.options.Environment != "production" {
		external := "SIM-" + claimed.ID
		if _, err = uc.options.FinancialRepository.RecordPayoutSubmission(ctx, claimed.ID, external); err != nil {
			return nil, err
		}
		return uc.options.FinancialRepository.ConfirmPayout(ctx, claimed.ID, external, *claimed.NetAmount)
	}
	if uc.options.Mode != ModeSuitPay || !cfg.Gateway.SuitPayEnabled || cfg.Gateway.SuitPayClientID == "" || cfg.Gateway.SuitPayClientSecret == "" || (uc.options.Environment == "production" && cfg.Gateway.SuitPaySandbox) {
		return nil, fmt.Errorf("gateway de saque indisponível; saque reservado para conciliação")
	}
	client := uc.newSuitPayClient(cfg.Gateway.SuitPayClientID, cfg.Gateway.SuitPayClientSecret, cfg.Gateway.SuitPaySandbox, cfg.Gateway.SuitPayCallbackURL)
	response, err := client.TransferPix(ctx, gateway.SuitPayPixTransferRequest{
		Value:              *claimed.NetAmount,
		Key:                *claimed.PixKey,
		TypeKey:            gateway.DetectPixKeyType(*claimed.PixKey),
		DocumentValidation: normalizedCPF,
		ExternalID:         claimed.ID,
	})
	if err != nil || response == nil || response.Response != "OK" || response.IDTransaction == "" {
		// The HTTP outcome may be uncertain. Keep the reservation and the
		// one-way processing state for operator reconciliation.
		return claimed, nil
	}
	return uc.options.FinancialRepository.RecordPayoutSubmission(ctx, claimed.ID, response.IDTransaction)
}

func (uc *PaymentUseCase) processPayoutCallback(ctx context.Context, cb *gateway.SuitPayWebhookCallback) *WebhookResult {
	if uc.options.FinancialRepository == nil {
		return &WebhookResult{
			Success:        false,
			Error:          "repositório financeiro transacional indisponível",
			ErrorCode:      "FINANCIAL_REPO_UNAVAILABLE",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}
	id := formatTransactionID(cb.IDTransaction)
	if id == "" || !validAmount(cb.Value) {
		return &WebhookResult{
			Success:        false,
			Error:          "callback de saque incompleto",
			ErrorCode:      "INCOMPLETE_PAYOUT_CALLBACK",
			HTTPStatusCode: http.StatusBadRequest,
		}
	}
	// Fail closed when the outcome-aware applier is absent: cash-out state must
	// never be guessed without the applied/duplicate/queued contract.
	applier := uc.options.PayoutApplier
	if applier == nil {
		return &WebhookResult{
			Success:        false,
			Error:          "repositório financeiro sem suporte a conciliação de saque",
			ErrorCode:      "PAYOUT_APPLIER_UNAVAILABLE",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}
	// No pre-read of the status: an authenticated event of any status reaches
	// the repository, which durably queues unknown/conflicting statuses instead
	// of cancelling or refunding anything.
	status := strings.ToUpper(strings.TrimSpace(cb.StatusTransaction))
	tx, outcome, err := applier.ApplyPayoutCallbackWithOutcome(ctx, id, status, cb.Value)
	if err != nil {
		return &WebhookResult{
			Success:        false,
			Error:          "falha temporária ao persistir callback de saque",
			ErrorCode:      "PERSISTENCE_FAILED",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}

	switch outcome {
	case "applied":
		// Proof of a financial change: the repository must return the updated
		// transaction in an exact terminal state (sucesso or cancelado).
		if tx == nil || !isPayoutTerminalStatus(tx.Status) {
			return payoutCallbackInconsistentResult()
		}
		return &WebhookResult{
			Success:        true,
			Message:        "Saque conciliado",
			Kind:           "payout",
			Outcome:        "applied",
			Status:         string(tx.Status),
			TxID:           tx.ID,
			UserID:         tx.UserID,
			PaidAmount:     cb.Value,
			HTTPStatusCode: http.StatusOK,
		}

	case "duplicate":
		// A valid event already reconciled: the current terminal transaction
		// must be present (PostgreSQL repository contract); without it there is
		// no proof of prior settlement, so the gateway is not acknowledged.
		if tx == nil || !isPayoutTerminalStatus(tx.Status) {
			return payoutCallbackInconsistentResult()
		}
		return &WebhookResult{
			Success:        true,
			Message:        "Callback de saque já conciliado anteriormente (Idempotente)",
			Kind:           "payout",
			Outcome:        "duplicate",
			Status:         string(tx.Status),
			TxID:           tx.ID,
			UserID:         tx.UserID,
			PaidAmount:     cb.Value,
			HTTPStatusCode: http.StatusOK,
		}

	case "queued":
		// Durable trail without state change: the repository contract returns
		// (nil, "queued", nil); a transaction here means an inconsistent reply.
		if tx != nil {
			return payoutCallbackInconsistentResult()
		}
		return &WebhookResult{
			Success:        true,
			Message:        "Callback de saque registrado para conciliação",
			Kind:           "payout",
			Outcome:        "queued",
			Status:         "queued",
			PaidAmount:     cb.Value,
			HTTPStatusCode: http.StatusOK,
		}

	default:
		// Unknown outcome: without proof of durable persistence the gateway
		// must NOT be acknowledged. Fail closed with a stable error code and a
		// generic message that never leaks SQL details or the arbitrary outcome.
		return &WebhookResult{
			Success:        false,
			Error:          "falha temporária ao processar callback de saque",
			ErrorCode:      "PAYOUT_CALLBACK_UNKNOWN_OUTCOME",
			HTTPStatusCode: http.StatusInternalServerError,
		}
	}
}

// isPayoutTerminalStatus reports whether a payout transaction reached an exact
// terminal state (sucesso or cancelado), the only states that prove a durable
// settlement or refund for the applied/duplicate outcomes.
func isPayoutTerminalStatus(status domain.TxStatus) bool {
	return status == domain.TxStatusSucesso || status == domain.TxStatusCancelado
}

// payoutCallbackInconsistentResult fail-closes inconsistent applied/duplicate/
// queued replies: no ACK to the gateway and no fields that could trigger a
// withdraw_paid/withdraw_rejected notification. The message is generic and
// carries no SQL, outcome or transaction details.
func payoutCallbackInconsistentResult() *WebhookResult {
	return &WebhookResult{
		Success:        false,
		Error:          "falha temporária ao processar callback de saque",
		ErrorCode:      "PAYOUT_CALLBACK_INVALID_RESULT",
		HTTPStatusCode: http.StatusInternalServerError,
	}
}
