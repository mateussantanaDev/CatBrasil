package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"catbrasil/backend/internal/usecase/payment"
)

type FinancialHandler struct {
	paymentUC *payment.PaymentUseCase
	hub       *ws.Hub
}

func NewFinancialHandler(paymentUC *payment.PaymentUseCase, hub *ws.Hub) *FinancialHandler {
	return &FinancialHandler{
		paymentUC: paymentUC,
		hub:       hub,
	}
}

func (h *FinancialHandler) GeneratePixDeposit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		Amount json.RawMessage `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Informe um valor de depósito válido", "INVALID_AMOUNT")
		return
	}
	amount, amountErr := money.ParseJSONNumber(string(req.Amount))
	if amountErr != nil || amount <= 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Informe um valor de depósito válido", "INVALID_AMOUNT")
		return
	}

	resp, err := h.paymentUC.CreatePixDeposit(r.Context(), payment.PixDepositRequest{
		UserID: userID,
		Amount: amount.Float64(),
	})
	if err != nil {
		if errors.Is(err, domain.ErrIdentityNotVerified) {
			middleware.RespondError(w, http.StatusForbidden, err.Error(), "IDENTITY_NOT_VERIFIED")
			return
		}
		if errors.Is(err, domain.ErrDepositUnavailable) {
			middleware.RespondError(w, http.StatusServiceUnavailable, "Depósitos PIX indisponíveis. Verifique seu histórico antes de tentar novamente.", "PIX_PROVIDER_UNAVAILABLE")
			return
		}
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "PIX_GENERATION_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *FinancialHandler) RequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req payment.PayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}
	req.UserID = userID
	req.IdempotencyKey = r.Header.Get("Idempotency-Key")

	resp, err := h.paymentUC.RequestPayout(r.Context(), req)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "WITHDRAW_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "withdraw_requested", resp)
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *FinancialHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	filter := r.URL.Query().Get("filter")
	list, err := h.paymentUC.GetUserTransactions(r.Context(), userID, filter)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "TRANSACTIONS_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, list)
}

func (h *FinancialHandler) HandlePixWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Falha ao ler corpo do webhook", "WEBHOOK_READ_FAILED")
		return
	}

	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Signature")
	}

	var payload payment.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload JSON de webhook inválido", "INVALID_WEBHOOK_JSON")
		return
	}

	result := h.paymentUC.ProcessWebhook(r.Context(), payload, signature)
	if !result.Success {
		status := http.StatusUnauthorized
		errCode := "WEBHOOK_REJECTED"
		if result.HTTPStatusCode != 0 {
			status = result.HTTPStatusCode
		}
		if result.ErrorCode != "" {
			errCode = result.ErrorCode
		}
		middleware.RespondError(w, status, result.Error, errCode)
		return
	}

	// Notifica o operador em tempo real via WebSocket
	if h.hub != nil && result.UserID != "" && result.Outcome == "applied" {
		h.hub.NotifyUser(result.UserID, "pix_confirmed", result)
	}

	middleware.RespondJSON(w, http.StatusOK, result)
}

func (h *FinancialHandler) SimulateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TxID       string          `json:"txId"`
		PaidAmount json.RawMessage `json:"paidAmount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TxID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Informe o txId da transação", "INVALID_TX_ID")
		return
	}
	paidAmount, amountErr := money.ParseJSONNumber(string(req.PaidAmount))
	if amountErr != nil || paidAmount <= 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Valor de pagamento inválido", "INVALID_AMOUNT")
		return
	}

	payload := payment.WebhookPayload{
		TxID:       req.TxID,
		Status:     "paid",
		PaidAmount: paidAmount.Float64(),
	}

	result := h.paymentUC.ProcessSimulatedWebhook(r.Context(), payload)
	if !result.Success {
		middleware.RespondError(w, http.StatusBadRequest, result.Error, "SIMULATION_FAILED")
		return
	}

	if h.hub != nil && result.UserID != "" && result.Outcome == "applied" {
		h.hub.NotifyUser(result.UserID, "pix_confirmed", result)
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Simulação de Webhook PIX executada com sucesso!",
		"result":  result,
	})
}

// SuitPayWebhookNotificationEvent maps a SuitPay webhook result to the realtime
// event emitted to the operator. Duplicates never re-emit pix_confirmed (they
// represent no new credit), reversed deposits emit pix_chargeback and payout
// callbacks only notify on the exact applied outcome: withdraw_paid for a
// settlement (sucesso) and withdraw_rejected for a gateway cancellation/refund
// (cancelado). Duplicate/queued payout events emit nothing.
func SuitPayWebhookNotificationEvent(result *payment.WebhookResult) string {
	if result == nil {
		return ""
	}
	switch {
	case result.Kind == "payout":
		if result.Outcome != "applied" {
			return ""
		}
		switch result.Status {
		case string(domain.TxStatusSucesso):
			return "withdraw_paid"
		case string(domain.TxStatusCancelado):
			return "withdraw_rejected"
		default:
			return ""
		}
	case result.Outcome == "duplicate":
		return "pix_duplicate"
	case result.Kind == "chargeback":
		return "pix_chargeback"
	case result.Outcome == "queued":
		return "pix_queued"
	default:
		return "pix_confirmed"
	}
}

// HandleSuitPayWebhook processa as notificações oficiais da SuitPay
func (h *FinancialHandler) HandleSuitPayWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Falha ao ler corpo do webhook SuitPay", "WEBHOOK_READ_FAILED")
		return
	}

	result := h.paymentUC.ProcessSuitPayWebhook(r.Context(), body)
	if !result.Success {
		status := http.StatusUnauthorized
		errCode := "SUITPAY_WEBHOOK_REJECTED"
		if result.HTTPStatusCode != 0 {
			status = result.HTTPStatusCode
		}
		if result.ErrorCode != "" {
			errCode = result.ErrorCode
		}
		middleware.RespondError(w, status, result.Error, errCode)
		return
	}

	if h.hub != nil && result.UserID != "" {
		if event := SuitPayWebhookNotificationEvent(result); event != "" {
			h.hub.NotifyUser(result.UserID, event, result)
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"response": "OK",
		"message":  "Webhook SuitPay processado com sucesso!",
		"result":   result,
	})
}
