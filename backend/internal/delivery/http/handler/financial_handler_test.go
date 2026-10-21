package handler_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/payment"
)

func TestGeneratePixDepositUnavailableReturns503WithoutQR(t *testing.T) {
	uc := payment.NewPaymentUseCase(&mockUserRepo{}, &mockTxRepo{}, nil, nil, nil, "", payment.Options{
		Mode: payment.ModeSimulated, Environment: "production",
	})
	h := handler.NewFinancialHandler(uc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pix/generate", strings.NewReader(`{"amount":50}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "payer"))
	w := httptest.NewRecorder()
	h.GeneratePixDeposit(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperado HTTP 503, obteve %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "PIX_PROVIDER_UNAVAILABLE") || strings.Contains(w.Body.String(), "pixCode") {
		t.Fatalf("resposta deve indicar indisponibilidade sem QR: %s", w.Body.String())
	}
}

type mockUserRepo struct{}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Username: "operador"}, nil
}
func (m *mockUserRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Username: "operador"}, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepo) Count(ctx context.Context) (int64, error)  { return 1, nil }
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

type mockTxRepo struct {
	applyFunc           func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error)
	approveWithdrawalFn func(ctx context.Context, txID string) (*domain.Transaction, error)
	rejectWithdrawalFn  func(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error)
}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return nil, 0, nil
}
func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	if m.approveWithdrawalFn != nil {
		return m.approveWithdrawalFn(ctx, txID)
	}
	return nil, nil
}
func (m *mockTxRepo) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	if m.rejectWithdrawalFn != nil {
		return m.rejectWithdrawalFn(ctx, txID, reason, lockAccount)
	}
	return nil, nil
}
func (m *mockTxRepo) ApplyDepositEvent(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, requestNumber, externalID, status, amount)
	}
	return nil, "", nil
}

type mockConfigRepo struct {
	secret  string
	err     error
	enabled *bool
}

func (m *mockConfigRepo) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	enabled := true
	if m.enabled != nil {
		enabled = *m.enabled
	}
	return &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      enabled,
			SuitPayClientSecret: m.secret,
		},
	}, nil
}
func (m *mockConfigRepo) UpdateConfig(ctx context.Context, cfg *domain.SystemConfig) error {
	return nil
}
func (m *mockConfigRepo) ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error { return nil }
func (m *mockConfigRepo) UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error {
	return nil
}
func (m *mockConfigRepo) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, graceMinutes int) error {
	return nil
}
func (m *mockConfigRepo) UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error {
	return nil
}
func (m *mockConfigRepo) ResetSeason(ctx context.Context, durationDays int) error { return nil }
func (m *mockConfigRepo) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	return nil
}

func makeSignedWebhook(idTx any, typeTx, statusTx string, value float64, reqNumber, secret string) []byte {
	valStr := fmt.Sprintf("%.2f", value)
	idStr := fmt.Sprint(idTx)
	values := idStr + typeTx + statusTx + valStr + reqNumber
	digest := sha256.Sum256([]byte(secret + values))
	hash := hex.EncodeToString(digest[:])
	return []byte(fmt.Sprintf(`{"idTransaction":%s,"typeTransaction":"%s","statusTransaction":"%s","value":%s,"requestNumber":"%s","hash":"%s"}`,
		idStr, typeTx, statusTx, valStr, reqNumber, hash))
}

func TestFinancialHandler_HandleSuitPayWebhook_SignatureRejection(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	// Tampered webhook with wrong secret
	body := makeSignedWebhook(10, "PIX", "PAID_OUT", 100.00, "REQ-10", "wrong-secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status 401 Unauthorized para assinatura inválida, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errorCode"] != "INVALID_SIGNATURE" {
		t.Fatalf("esperado errorCode INVALID_SIGNATURE, obteve %v", resp["errorCode"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PersistenceFailure(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return nil, "", errors.New("deadlock / db timeout")
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(11, "PIX", "PAID_OUT", 100.00, "REQ-11", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	// MUST return 500 so SuitPay does NOT consider it delivered and retries
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado status 500 Internal Server Error para falha de persistência, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errorCode"] != "PERSISTENCE_FAILED" {
		t.Fatalf("esperado errorCode PERSISTENCE_FAILED, obteve %v", resp["errorCode"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_DurableReceiptConfirmation(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-ok", Status: domain.TxStatusSucesso}, "applied", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	hub := ws.NewHub()
	h := handler.NewFinancialHandler(uc, hub)

	body := makeSignedWebhook(12, "PIX", "PAID_OUT", 250.00, "REQ-12", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para confirmação durable, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["response"] != "OK" {
		t.Fatalf("esperado response OK para gateway, obteve %v", resp["response"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_ChargebackConfirmation(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			if status != "CHARGEBACK" {
				return nil, "", fmt.Errorf("status inesperado: %s", status)
			}
			return &domain.Transaction{ID: requestNumber, UserID: "usr-cb", Status: domain.TxStatusCancelado}, "applied", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(13, "PIX", "CHARGEBACK", 120.00, "REQ-13", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para chargeback, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["response"] != "OK" {
		t.Fatalf("esperado response OK, obteve %v", resp["response"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_QueuedConfirmation(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-q", Status: domain.TxStatusProcessando}, "queued", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	// Unrecognized authenticated event
	body := makeSignedWebhook(14, "PIX", "MANUAL_REVIEW", 80.00, "REQ-14", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para evento enfileirado, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["response"] != "OK" {
		t.Fatalf("esperado response OK, obteve %v", resp["response"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_ConfigUnavailableReturns503(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{
		secret: secret,
		err:    errors.New("db pool down"),
	}
	txRepo := &mockTxRepo{}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(15, "PIX", "PAID_OUT", 100.00, "REQ-15", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperado status 503 Service Unavailable para falha de config, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errorCode"] != "CONFIG_UNAVAILABLE" {
		t.Fatalf("esperado errorCode CONFIG_UNAVAILABLE, obteve %v", resp["errorCode"])
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_KillSwitchOffStillAcceptsLegitimateCallback(t *testing.T) {
	secret := "secret-correct"
	disabled := false
	cfgRepo := &mockConfigRepo{
		secret:  secret,
		enabled: &disabled, // kill switch OFF
	}
	var appliedReq string
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			appliedReq = requestNumber
			return &domain.Transaction{ID: requestNumber, UserID: "usr-ks-h", Status: domain.TxStatusSucesso}, "applied", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(16, "PIX", "PAID_OUT", 50.00, "REQ-16", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para callback com kill switch off, obteve %d", rec.Code)
	}
	if appliedReq != "REQ-16" {
		t.Fatalf("esperado appliedReq REQ-16, obteve %s", appliedReq)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PersistenceFailure_GenericMessage(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return nil, "", errors.New("pq: deadlocked relation transactions_pkey (SQLSTATE 40P01)")
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(17, "PIX", "PAID_OUT", 100.00, "REQ-17", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado status 500, obteve %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errorCode"] != "PERSISTENCE_FAILED" {
		t.Fatalf("esperado errorCode PERSISTENCE_FAILED, obteve %v", resp["errorCode"])
	}
	// Verify error message is generic and does not leak internal SQL error
	errMsg, _ := resp["error"].(string)
	if strings.Contains(errMsg, "deadlocked") || strings.Contains(errMsg, "SQLSTATE") {
		t.Fatalf("detalhes internos vazaram na resposta do webhook: %s", errMsg)
	}
	if errMsg != "falha temporária ao persistir evento de pagamento" {
		t.Fatalf("mensagem inesperada: %s", errMsg)
	}
}

func TestSuitPayWebhookNotificationEvent_Mapping(t *testing.T) {
	cases := []struct {
		name   string
		result *payment.WebhookResult
		want   string
	}{
		{
			name:   "nil result emits nothing",
			result: nil,
			want:   "",
		},
		{
			name:   "applied deposit confirms credit",
			result: &payment.WebhookResult{Outcome: "applied", Kind: "deposit", Status: "sucesso"},
			want:   "pix_confirmed",
		},
		{
			name:   "applied chargeback emits chargeback",
			result: &payment.WebhookResult{Outcome: "applied", Kind: "chargeback", Status: "cancelado"},
			want:   "pix_chargeback",
		},
		{
			name:   "duplicate deposit never re-emits pix_confirmed",
			result: &payment.WebhookResult{Outcome: "duplicate", Kind: "deposit", Status: "sucesso"},
			want:   "pix_duplicate",
		},
		{
			name:   "duplicate chargeback emits duplicate not chargeback",
			result: &payment.WebhookResult{Outcome: "duplicate", Kind: "chargeback", Status: "cancelado"},
			want:   "pix_duplicate",
		},
		{
			name:   "queued event emits pix_queued",
			result: &payment.WebhookResult{Outcome: "queued", Kind: "deposit", Status: "queued"},
			want:   "pix_queued",
		},
		{
			name:   "payout applied settlement emits withdraw_paid",
			result: &payment.WebhookResult{Outcome: "applied", Kind: "payout", Status: "sucesso"},
			want:   "withdraw_paid",
		},
		{
			name:   "payout applied cancellation emits withdraw_rejected",
			result: &payment.WebhookResult{Outcome: "applied", Kind: "payout", Status: "cancelado"},
			want:   "withdraw_rejected",
		},
		{
			name:   "payout duplicate never re-notifies",
			result: &payment.WebhookResult{Outcome: "duplicate", Kind: "payout", Status: "sucesso"},
			want:   "",
		},
		{
			name:   "payout queued emits no realtime ws event",
			result: &payment.WebhookResult{Outcome: "queued", Kind: "payout", Status: "queued"},
			want:   "",
		},
		{
			name:   "payout applied with unexpected status emits nothing",
			result: &payment.WebhookResult{Outcome: "applied", Kind: "payout", Status: "processando"},
			want:   "",
		},
		{
			name:   "payout unknown outcome emits nothing",
			result: &payment.WebhookResult{Outcome: "unknown", Kind: "payout", Status: "sucesso"},
			want:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := handler.SuitPayWebhookNotificationEvent(tc.result); got != tc.want {
				t.Fatalf("evento esperado %q, obteve %q", tc.want, got)
			}
		})
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_EarlyReversalEmitsChargeback(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			// PAID_OUT applied but immediately reversed by an earlier chargeback.
			return &domain.Transaction{ID: requestNumber, UserID: "usr-rev-h", Status: domain.TxStatusCancelado}, "applied", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(18, "PIX", "PAID_OUT", 70.00, "REQ-18", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para desfecho durável de estorno, obteve %d", rec.Code)
	}

	var resp struct {
		Response string                `json:"response"`
		Result   payment.WebhookResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Response != "OK" {
		t.Fatalf("esperado response OK para gateway, obteve %v", resp.Response)
	}
	if resp.Result.Outcome != "applied" || resp.Result.Kind != "chargeback" {
		t.Fatalf("esperado applied/chargeback para PAID_OUT revertido, obteve: %+v", resp.Result)
	}
	if resp.Result.Status != string(domain.TxStatusCancelado) {
		t.Fatalf("status cancelado deve ser preservado: %s", resp.Result.Status)
	}
	if event := handler.SuitPayWebhookNotificationEvent(&resp.Result); event != "pix_chargeback" {
		t.Fatalf("evento realtime deve ser pix_chargeback, obteve: %s", event)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_DuplicateEmitsNoPixConfirmed(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-dup-h", Status: domain.TxStatusSucesso}, "duplicate", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(19, "PIX", "PAID_OUT", 55.00, "REQ-19", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK para duplicata idempotente, obteve %d", rec.Code)
	}

	var resp struct {
		Response string                `json:"response"`
		Result   payment.WebhookResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.Outcome != "duplicate" {
		t.Fatalf("esperado outcome duplicate, obteve: %+v", resp.Result)
	}
	event := handler.SuitPayWebhookNotificationEvent(&resp.Result)
	if event == "pix_confirmed" {
		t.Fatal("evento duplicado NUNCA pode re-emitir pix_confirmed como novo crédito")
	}
	if event != "pix_duplicate" {
		t.Fatalf("evento realtime deve ser pix_duplicate, obteve: %s", event)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_LargeNumericIDExactMatch(t *testing.T) {
	secret := "secret-correct"
	cfgRepo := &mockConfigRepo{secret: secret}
	var gotExternal string
	txRepo := &mockTxRepo{
		applyFunc: func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			gotExternal = externalID
			return &domain.Transaction{ID: requestNumber, UserID: "usr-big-h", Status: domain.TxStatusSucesso}, "applied", nil
		},
	}
	userRepo := &mockUserRepo{}
	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(int64(9007199254740993), "PIX", "PAID_OUT", 33.00, "REQ-BIG", secret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200 OK, obteve %d", rec.Code)
	}
	if gotExternal != "9007199254740993" {
		t.Fatalf("external ID numérico grande perdeu precisão no handler: %q", gotExternal)
	}
}

type mockPayoutFinancialRepo struct {
	outcomeFunc func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error)
}

func (m *mockPayoutFinancialRepo) ConfirmDeposit(ctx context.Context, id string, amount float64, ext, method string) (*domain.Transaction, float64, error) {
	return nil, 0, nil
}
func (m *mockPayoutFinancialRepo) ReservePayout(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
	return tx, 0, false, nil
}
func (m *mockPayoutFinancialRepo) ClaimPayout(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) RecordPayoutSubmission(ctx context.Context, id, ext string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) ConfirmPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) CancelPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) ApplyPayoutCallback(ctx context.Context, ext, status string, val float64) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) GetPayoutByExternalID(ctx context.Context, ext string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockPayoutFinancialRepo) ApplyPayoutCallbackWithOutcome(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
	if m.outcomeFunc != nil {
		return m.outcomeFunc(ctx, ext, status, val)
	}
	return nil, "queued", nil
}

// legacyFinancialRepoOnly implements FinancialRepository WITHOUT the
// outcome-aware callback, reproducing repositories that predate P0.3.
type legacyFinancialRepoOnly struct{}

func (legacyFinancialRepoOnly) ConfirmDeposit(ctx context.Context, id string, amount float64, ext, method string) (*domain.Transaction, float64, error) {
	return nil, 0, nil
}
func (legacyFinancialRepoOnly) ReservePayout(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
	return tx, 0, false, nil
}
func (legacyFinancialRepoOnly) ClaimPayout(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) RecordPayoutSubmission(ctx context.Context, id, ext string) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) ConfirmPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) CancelPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) ApplyPayoutCallback(ctx context.Context, ext, status string, val float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) GetPayoutByExternalID(ctx context.Context, ext string) (*domain.Transaction, error) {
	return nil, nil
}

func newPayoutHandler(t *testing.T, secret string, repo payment.FinancialRepository) *handler.FinancialHandler {
	t.Helper()
	cfgRepo := &mockConfigRepo{secret: secret}
	uc := payment.NewPaymentUseCase(&mockUserRepo{}, &mockTxRepo{}, nil, cfgRepo, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: repo,
	})
	return handler.NewFinancialHandler(uc, nil)
}

func callPayoutWebhook(t *testing.T, h *handler.FinancialHandler, status string, value float64) *payment.WebhookResult {
	t.Helper()
	body := makeSignedWebhook(900, "PIX_CASHOUT", status, value, "", "secret-correct")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleSuitPayWebhook(rec, req)

	var resp struct {
		Response string                `json:"response"`
		Result   payment.WebhookResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta inválida (%d): %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d: %+v", rec.Code, resp.Result)
	}
	return &resp.Result
}

func TestFinancialHandler_HandleSuitPayWebhook_PayoutAppliedEmitsWithdrawPaid(t *testing.T) {
	repo := &mockPayoutFinancialRepo{
		outcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: "WDR-WS-1", UserID: "usr-ws-1", Status: domain.TxStatusSucesso}, "applied", nil
		},
	}
	h := newPayoutHandler(t, "secret-correct", repo)

	result := callPayoutWebhook(t, h, "PAID_OUT", 95.00)
	if result.Outcome != "applied" || result.Kind != "payout" {
		t.Fatalf("esperado applied/payout, obteve: %+v", result)
	}
	if event := handler.SuitPayWebhookNotificationEvent(result); event != "withdraw_paid" {
		t.Fatalf("liquidação de saque deve emitir withdraw_paid, obteve: %q", event)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PayoutCanceledEmitsWithdrawRejected(t *testing.T) {
	repo := &mockPayoutFinancialRepo{
		outcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: "WDR-WS-2", UserID: "usr-ws-2", Status: domain.TxStatusCancelado}, "applied", nil
		},
	}
	h := newPayoutHandler(t, "secret-correct", repo)

	result := callPayoutWebhook(t, h, "CANCELED", 95.00)
	if result.Outcome != "applied" || result.Status != string(domain.TxStatusCancelado) {
		t.Fatalf("esperado applied/cancelado, obteve: %+v", result)
	}
	if event := handler.SuitPayWebhookNotificationEvent(result); event != "withdraw_rejected" {
		t.Fatalf("estorno de saque deve emitir withdraw_rejected, obteve: %q", event)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PayoutReplayEmitsNoWebSocketEvent(t *testing.T) {
	calls := 0
	repo := &mockPayoutFinancialRepo{
		outcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			calls++
			return &domain.Transaction{ID: "WDR-WS-3", UserID: "usr-ws-3", Status: domain.TxStatusSucesso}, "duplicate", nil
		},
	}
	h := newPayoutHandler(t, "secret-correct", repo)

	first := callPayoutWebhook(t, h, "PAID_OUT", 95.00)
	second := callPayoutWebhook(t, h, "PAID_OUT", 95.00)

	if calls != 2 {
		t.Fatalf("replays devem chegar ao repositório para deduplicação, obteve %d chamadas", calls)
	}
	for i, result := range []*payment.WebhookResult{first, second} {
		if result.Outcome != "duplicate" {
			t.Fatalf("replay %d deve permanecer duplicate, obteve: %+v", i+1, result)
		}
		if event := handler.SuitPayWebhookNotificationEvent(result); event != "" {
			t.Fatalf("replay %d NUNCA pode reemitir evento WS, obteve: %q", i+1, event)
		}
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PayoutQueuedEmitsNoWebSocketEvent(t *testing.T) {
	repo := &mockPayoutFinancialRepo{
		outcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return nil, "queued", nil
		},
	}
	h := newPayoutHandler(t, "secret-correct", repo)

	result := callPayoutWebhook(t, h, "DOCUMENT_VALIDATE", 95.00)
	if result.Outcome != "queued" || result.Kind != "payout" {
		t.Fatalf("esperado queued/payout para status autenticado desconhecido, obteve: %+v", result)
	}
	if event := handler.SuitPayWebhookNotificationEvent(result); event != "" {
		t.Fatalf("evento enfileirado NUNCA pode notificar o cliente, obteve: %q", event)
	}
}

func TestFinancialHandler_HandleSuitPayWebhook_PayoutWithoutOutcomeApplierFailsClosed(t *testing.T) {
	// FinancialRepository without ApplyPayoutCallbackWithOutcome: cash-out must
	// fail closed with a generic 500 instead of guessing state.
	cfgRepo := &mockConfigRepo{secret: "secret-correct"}
	uc := payment.NewPaymentUseCase(&mockUserRepo{}, &mockTxRepo{}, nil, cfgRepo, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: legacyFinancialRepoOnly{},
	})
	h := handler.NewFinancialHandler(uc, nil)

	body := makeSignedWebhook(901, "PIX_CASHOUT", "PAID_OUT", 95.00, "", "secret-correct")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/suitpay", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleSuitPayWebhook(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado 500 fail-closed sem applier de outcome, obteve %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["errorCode"] != "PAYOUT_APPLIER_UNAVAILABLE" {
		t.Fatalf("esperado errorCode PAYOUT_APPLIER_UNAVAILABLE, obteve %v", resp["errorCode"])
	}
}
