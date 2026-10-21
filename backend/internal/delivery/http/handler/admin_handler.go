package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/id"

	"github.com/go-chi/chi/v5"
)

type AdminHandler struct {
	adminUC      *admin.AdminUseCase
	userRepo     domain.UserRepository
	machineRepo  domain.MachineRepository
	catalogRepo  domain.CatalogRepository
	txRepo       domain.TransactionRepository
	referralRepo domain.ReferralRepository
	rouletteRepo domain.RouletteRepository
	announceRepo domain.AnnouncementRepository
	configRepo   domain.ConfigRepository
	promoRepo    domain.PromoRepository
	logRepo      domain.LogRepository
	levelRepo    domain.LevelRepository
	identityRepo domain.IdentityRepository
	hub          *ws.Hub
}

func (h *AdminHandler) SetIdentityRepository(repo domain.IdentityRepository) { h.identityRepo = repo }

func (h *AdminHandler) ListPendingIdentities(w http.ResponseWriter, r *http.Request) {
	if h.identityRepo == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "Identidade indisponível", "IDENTITY_UNAVAILABLE")
		return
	}
	actor, err := h.userRepo.GetByID(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil || actor == nil || actor.Role != domain.RoleDiretoria {
		middleware.RespondError(w, http.StatusForbidden, "Apenas diretoria pode revisar identidade", "IDENTITY_REVIEW_FORBIDDEN")
		return
	}
	ids, err := h.identityRepo.Pending(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "Fila de identidade indisponível", "IDENTITY_UNAVAILABLE")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{"userIds": ids})
}

func (h *AdminHandler) ReviewIdentity(w http.ResponseWriter, r *http.Request) {
	if h.identityRepo == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "Identidade indisponível", "IDENTITY_UNAVAILABLE")
		return
	}
	actor, err := h.userRepo.GetByID(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil || actor == nil || actor.Role != domain.RoleDiretoria {
		middleware.RespondError(w, http.StatusForbidden, "Apenas diretoria pode revisar identidade", "IDENTITY_REVIEW_FORBIDDEN")
		return
	}
	if actor.ID == chi.URLParam(r, "userId") {
		middleware.RespondError(w, http.StatusForbidden, "A própria identidade exige outro revisor", "IDENTITY_SELF_REVIEW_FORBIDDEN")
		return
	}
	var req struct {
		Status        string `json:"status"`
		EvidenceRef   string `json:"evidenceRef"`
		TwoFactorCode string `json:"twoFactorCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}
	if !actor.TwoFactorEnabled || !crypto.VerifyTOTPCode(actor.TwoFactorSecret, req.TwoFactorCode) {
		middleware.RespondError(w, http.StatusForbidden, "Código 2FA inválido ou ausente", "TWO_FACTOR_REQUIRED")
		return
	}
	err = h.identityRepo.Decide(r.Context(), chi.URLParam(r, "userId"), actor.ID, req.Status, req.EvidenceRef)
	if err != nil {
		status, code := http.StatusServiceUnavailable, "IDENTITY_UNAVAILABLE"
		if errors.Is(err, domain.ErrIdentityState) {
			status, code = http.StatusConflict, "IDENTITY_STATE_INVALID"
		}
		middleware.RespondError(w, status, "Revisão não concluída", code)
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{"identityStatus": req.Status})
}

func (h *AdminHandler) InspectIdentity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.identityRepo == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "Identidade indisponível", "IDENTITY_UNAVAILABLE")
		return
	}
	actor, err := h.userRepo.GetByID(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil || actor == nil || actor.Role != domain.RoleDiretoria {
		middleware.RespondError(w, http.StatusForbidden, "Apenas diretoria pode revisar identidade", "IDENTITY_REVIEW_FORBIDDEN")
		return
	}
	var req struct {
		TwoFactorCode string `json:"twoFactorCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}
	if !actor.TwoFactorEnabled || !crypto.VerifyTOTPCode(actor.TwoFactorSecret, req.TwoFactorCode) {
		middleware.RespondError(w, http.StatusForbidden, "Código 2FA inválido ou ausente", "TWO_FACTOR_REQUIRED")
		return
	}
	person, err := h.identityRepo.Inspect(r.Context(), chi.URLParam(r, "userId"), actor.ID)
	if err != nil {
		status, code := http.StatusServiceUnavailable, "IDENTITY_UNAVAILABLE"
		if errors.Is(err, domain.ErrIdentityState) {
			status, code = http.StatusConflict, "IDENTITY_STATE_INVALID"
		}
		middleware.RespondError(w, status, "Identidade não disponível para revisão", code)
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{"cpf": person.CPF, "legalName": person.LegalName, "email": person.Email, "identityStatus": "pending"})
}

func NewAdminHandler(
	adminUC *admin.AdminUseCase,
	userRepo domain.UserRepository,
	machineRepo domain.MachineRepository,
	catalogRepo domain.CatalogRepository,
	txRepo domain.TransactionRepository,
	referralRepo domain.ReferralRepository,
	rouletteRepo domain.RouletteRepository,
	announceRepo domain.AnnouncementRepository,
	configRepo domain.ConfigRepository,
	promoRepo domain.PromoRepository,
	logRepo domain.LogRepository,
	levelRepo domain.LevelRepository,
	hub *ws.Hub,
) *AdminHandler {
	return &AdminHandler{
		adminUC:      adminUC,
		userRepo:     userRepo,
		machineRepo:  machineRepo,
		catalogRepo:  catalogRepo,
		txRepo:       txRepo,
		referralRepo: referralRepo,
		rouletteRepo: rouletteRepo,
		announceRepo: announceRepo,
		configRepo:   configRepo,
		promoRepo:    promoRepo,
		logRepo:      logRepo,
		levelRepo:    levelRepo,
		hub:          hub,
	}
}

// 1. DRE & MÉTRICAS FINANCEIRAS
func (h *AdminHandler) GetDRE(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.adminUC.GetDRE(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "DRE_CALCULATION_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    metrics,
	})
}

// 2. GESTÃO DE OPERADORES
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	users, total, err := h.adminUC.ListUsers(r.Context(), query, status, limit, offset)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "USERS_LIST_FAILED")
		return
	}

	var formattedList []map[string]interface{}
	for _, u := range users {
		formattedList = append(formattedList, map[string]interface{}{
			"id":                     u.ID,
			"username":               u.Username,
			"phone":                  u.Phone,
			"balance":                u.Balance,
			"yieldBalanceAvailable":  u.YieldBalanceAvailable,
			"nivel":                  u.Nivel,
			"xp":                     u.XP,
			"coletasCompleted":       u.ColetasCompleted,
			"withdrawLocked":         u.WithdrawLocked,
			"referralBonusSuspended": u.ReferralBonusSuspended,
			"multiAccountFlag":       u.MultiAccountFlag,
			"role":                   u.Role,
			"createdAt":              u.CreatedAt,
			"financialStats": map[string]interface{}{
				"totalDeposited": 0.0,
				"totalWithdrawn": 0.0,
				"netLtvProfit":   u.Balance,
			},
		})
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   total,
		"data":    formattedList,
	})
}

func (h *AdminHandler) AdjustBalance(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "ID do operador obrigatório", "INVALID_USER_ID")
		return
	}

	var req struct {
		Type   string  `json:"type"` // "add" | "subtract"
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "amount"); err != nil || req.Amount <= 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Informe um valor de ajuste válido", "INVALID_ADJUSTMENT")
		return
	}
	if req.Type != "add" && req.Type != "subtract" && req.Type != "sub" {
		middleware.RespondError(w, http.StatusBadRequest, "Tipo de ajuste inválido", "INVALID_ADJUSTMENT")
		return
	}

	delta := req.Amount
	if req.Type == "subtract" || req.Type == "sub" {
		delta = -req.Amount
	}

	user, err := h.adminUC.AdjustUserBalance(r.Context(), userID, delta, middleware.GetUserID(r.Context()), req.Reason, r.Header.Get("Idempotency-Key"))
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "ADJUST_BALANCE_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "balance_updated", map[string]interface{}{
			"newBalance": user.Balance,
			"reason":     req.Reason,
		})
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Saldo ajustado com sucesso",
		"user":    user,
	})
}

func (h *AdminHandler) UpdateUserLevel(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	var req struct {
		Nivel int `json:"nivel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Nivel <= 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Informe um nível válido", "INVALID_LEVEL")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.RespondError(w, http.StatusNotFound, "Operador não encontrado", "OPERATOR_NOT_FOUND")
		return
	}

	user.Nivel = req.Nivel
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "LEVEL_UPDATE_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Nível de carreira atualizado com sucesso",
		"nivel":   user.Nivel,
	})
}

func (h *AdminHandler) ToggleWithdrawLock(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.RespondError(w, http.StatusNotFound, "Operador não encontrado", "OPERATOR_NOT_FOUND")
		return
	}

	user.WithdrawLocked = !user.WithdrawLocked
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "LOCK_UPDATE_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"withdrawLocked": user.WithdrawLocked,
		"message":        "Status de trava de saque alterado com sucesso",
	})
}

func (h *AdminHandler) ToggleCommissionLock(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.RespondError(w, http.StatusNotFound, "Operador não encontrado", "OPERATOR_NOT_FOUND")
		return
	}

	user.ReferralBonusSuspended = !user.ReferralBonusSuspended
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "COMMISSION_LOCK_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":                true,
		"referralBonusSuspended": user.ReferralBonusSuspended,
		"message":                "Status de suspensão de comissão alterado com sucesso",
	})
}

func (h *AdminHandler) InjectMachine(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	var req struct {
		MachineModel string `json:"machineModel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MachineModel == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Informe o modelo da máquina", "INVALID_MACHINE_MODEL")
		return
	}

	catItem, err := h.adminUC.InjectBonusMachine(r.Context(), userID, req.MachineModel)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "INJECT_MACHINE_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Máquina bônus inserida na garagem do operador com sucesso!",
		"machine": catItem,
	})
}

func (h *AdminHandler) ResetUserSpin(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.RespondError(w, http.StatusNotFound, "Operador não encontrado", "OPERATOR_NOT_FOUND")
		return
	}

	user.AlreadySpunToday = false
	user.LastSpinDate = ""
	user.LastSpinAt = nil
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao resetar giro do operador", "INTERNAL_ERROR")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Giro diário da roleta resetado com sucesso para este operador",
	})
}

// 3. FILA DE SAQUES & MODERAÇÃO
func (h *AdminHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	txs, total, err := h.adminUC.ListWithdrawals(r.Context(), status, limit, offset)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "WITHDRAWALS_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   total,
		"data":    txs,
	})
}

func (h *AdminHandler) ListFinancialReconciliation(w http.ResponseWriter, r *http.Request) {
	issues, err := h.adminUC.ListFinancialReconciliation(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "RECONCILIATION_FAILED")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "issues": issues})
}

func (h *AdminHandler) ApproveWithdrawal(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txId")
	if txID == "" {
		var req struct {
			TxID string `json:"txId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		txID = req.TxID
	}
	if txID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "ID da transação obrigatório", "INVALID_TX_ID")
		return
	}

	tx, err := h.adminUC.ApproveWithdrawal(r.Context(), txID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "APPROVE_FAILED")
		return
	}

	// Nenhum evento WebSocket é emitido aqui: aprovar/enviar um saque não é
	// pagamento. O cliente só recebe withdraw_paid (liquidação) ou
	// withdraw_rejected (cancelamento/estorno) quando o gateway conciliar via
	// webhook. A confirmação para o operador da diretoria é esta resposta HTTP.
	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Saque enviado para processamento; o pagamento só é confirmado após liquidação do gateway.",
		"tx":      tx,
	})
}

func (h *AdminHandler) RejectWithdrawal(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "txId")
	var req struct {
		TxID        string `json:"txId"`
		LockAccount bool   `json:"lockAccount"`
		Reason      string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if txID == "" {
		txID = req.TxID
	}
	if txID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "ID da transação obrigatório", "INVALID_TX_ID")
		return
	}

	tx, err := h.adminUC.RejectWithdrawal(r.Context(), txID, req.Reason, req.LockAccount)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "REJECT_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(tx.UserID, "withdraw_rejected", tx)
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Saque rejeitado e saldo estornado com sucesso.",
		"tx":      tx,
	})
}

// 4. ROLETA & PROBABILIDADES
func (h *AdminHandler) GetRouletteMatrix(w http.ResponseWriter, r *http.Request) {
	matrix, err := h.adminUC.GetRouletteMatrix(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "ROULETTE_MATRIX_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    matrix,
	})
}

func (h *AdminHandler) UpdateRouletteMatrix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slices []*domain.RouletteSlice `json:"slices"`
		Rules  *domain.RouletteRules   `json:"rules"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "slices[].amount"); err != nil || len(req.Slices) == 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Array de fatias da roleta obrigatório", "INVALID_SLICES")
		return
	}

	err := h.adminUC.UpdateRouletteConfig(r.Context(), req.Slices, req.Rules)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "INVALID_ROULETTE_PROBABILITY")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Matriz de probabilidades da roleta salva com sucesso!",
		"slices":  req.Slices,
	})
}

// 5. CONFIGURAÇÕES GLOBAIS

// GatewayConfigReadDTO define a visão estrita de leitura das configurações do gateway,
// expondo apenas parâmetros operacionais não sensíveis e flags booleanas indicando
// se as credenciais / URLs foram configuradas, sem expor nenhum valor ou máscara.
type GatewayConfigReadDTO struct {
	Name                          string  `json:"name"`
	DepositFeePercent             float64 `json:"depositFeePercent"`
	WithdrawFeeFixed              float64 `json:"withdrawFeeFixed"`
	PixExpirationMinutes          int     `json:"pixExpirationMinutes"`
	ActiveProvider                string  `json:"activeProvider"`
	SuitPaySandbox                bool    `json:"suitpaySandbox"`
	SuitPayEnabled                bool    `json:"suitpayEnabled"`
	ParadiseEnabled               bool    `json:"paradiseEnabled"`
	SuitPayClientIDConfigured     bool    `json:"suitpayClientIdConfigured"`
	SuitPayClientSecretConfigured bool    `json:"suitpayClientSecretConfigured"`
	SuitPayCallbackURLConfigured  bool    `json:"suitpayCallbackUrlConfigured"`
	ParadiseSecretKeyConfigured   bool    `json:"paradiseSecretKeyConfigured"`
	ParadiseProductHashConfigured bool    `json:"paradiseProductHashConfigured"`
	ParadisePostbackURLConfigured bool    `json:"paradisePostbackUrlConfigured"`
}

// SystemConfigReadDTO define a projeção estrita de leitura do sistema entregue ao frontend admin.
type SystemConfigReadDTO struct {
	ID                            int                  `json:"id"`
	GlobalWithdrawalsLocked       bool                 `json:"globalWithdrawalsLocked"`
	AutoApproveEnabled            bool                 `json:"autoApproveEnabled"`
	AutoApproveWithdrawalsUnder   float64              `json:"autoApproveWithdrawalsUnder"`
	ReferralCommissionPercent     float64              `json:"referralCommissionPercent"`
	RequiredReferralsCount        int                  `json:"requiredReferralsCount"`
	RequireReferralsForWithdrawal bool                 `json:"requireReferralsForWithdrawal"`
	AntiSelfReferralLock          bool                 `json:"antiSelfReferralLock"`
	BonusPerMutiraoAmount         float64              `json:"bonusPerMutiraoAmount"`
	CollectionGraceMinutes        int                  `json:"collectionGraceMinutes"`
	Gateway                       GatewayConfigReadDTO `json:"gateway"`
	StrictModalMode               bool                 `json:"strictModalMode"`
	RequireDepositToSpin          bool                 `json:"requireDepositToSpin"`
	SeasonDurationDays            int                  `json:"seasonDurationDays"`
	CurrentSeasonNumber           int                  `json:"currentSeasonNumber"`
	SeasonStartedAt               time.Time            `json:"seasonStartedAt"`
	Level12MaintenanceDays        int                  `json:"level12MaintenanceDays"`
	Level12RequiredReferrals      int                  `json:"level12RequiredReferrals"`
	CreatedAt                     time.Time            `json:"createdAt"`
	UpdatedAt                     time.Time            `json:"updatedAt"`
}

func (h *AdminHandler) GetSystemConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configRepo.GetConfig(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao consultar configurações do sistema", "CONFIG_FETCH_FAILED")
		return
	}

	dto := SystemConfigReadDTO{
		ID:                            cfg.ID,
		GlobalWithdrawalsLocked:       cfg.GlobalWithdrawalsLocked,
		AutoApproveEnabled:            cfg.AutoApproveEnabled,
		AutoApproveWithdrawalsUnder:   cfg.AutoApproveWithdrawalsUnder,
		ReferralCommissionPercent:     cfg.ReferralCommissionPercent,
		RequiredReferralsCount:        cfg.RequiredReferralsCount,
		RequireReferralsForWithdrawal: cfg.RequireReferralsForWithdrawal,
		AntiSelfReferralLock:          cfg.AntiSelfReferralLock,
		BonusPerMutiraoAmount:         cfg.BonusPerMutiraoAmount,
		CollectionGraceMinutes:        cfg.CollectionGraceMinutes,
		Gateway: GatewayConfigReadDTO{
			Name:                          cfg.Gateway.Name,
			DepositFeePercent:             cfg.Gateway.DepositFeePercent,
			WithdrawFeeFixed:              cfg.Gateway.WithdrawFeeFixed,
			PixExpirationMinutes:          cfg.Gateway.PixExpirationMinutes,
			ActiveProvider:                cfg.Gateway.ActiveProvider,
			SuitPaySandbox:                cfg.Gateway.SuitPaySandbox,
			SuitPayEnabled:                cfg.Gateway.SuitPayEnabled,
			ParadiseEnabled:               cfg.Gateway.ParadiseEnabled,
			SuitPayClientIDConfigured:     cfg.Gateway.SuitPayClientID != "",
			SuitPayClientSecretConfigured: cfg.Gateway.SuitPayClientSecret != "",
			SuitPayCallbackURLConfigured:  cfg.Gateway.SuitPayCallbackURL != "",
			ParadiseSecretKeyConfigured:   cfg.Gateway.ParadiseSecretKey != "",
			ParadiseProductHashConfigured: cfg.Gateway.ParadiseProductHash != "",
			ParadisePostbackURLConfigured: cfg.Gateway.ParadisePostbackURL != "",
		},
		StrictModalMode:          cfg.StrictModalMode,
		RequireDepositToSpin:     cfg.RequireDepositToSpin,
		SeasonDurationDays:       cfg.SeasonDurationDays,
		CurrentSeasonNumber:      cfg.CurrentSeasonNumber,
		SeasonStartedAt:          cfg.SeasonStartedAt,
		Level12MaintenanceDays:   cfg.Level12MaintenanceDays,
		Level12RequiredReferrals: cfg.Level12RequiredReferrals,
		CreatedAt:                cfg.CreatedAt,
		UpdatedAt:                cfg.UpdatedAt,
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    dto,
	})
}

func (h *AdminHandler) UpdateGatewayConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                   string  `json:"name"`
		DepositFeePercent      float64 `json:"depositFeePercent"`
		WithdrawFeeFixed       float64 `json:"withdrawFeeFixed"`
		PixExpirationMinutes   int     `json:"pixExpirationMinutes"`
		CollectionGraceMinutes int     `json:"collectionGraceMinutes"`
		ActiveProvider         string  `json:"activeProvider"`
		SuitPayClientID        string  `json:"suitpayClientId"`
		SuitPayClientSecret    string  `json:"suitpayClientSecret"`
		SuitPaySandbox         bool    `json:"suitpaySandbox"`
		SuitPayCallbackURL     string  `json:"suitpayCallbackUrl"`
		SuitPayEnabled         bool    `json:"suitpayEnabled"`
		ParadiseSecretKey      string  `json:"paradiseSecretKey"`
		ParadiseProductHash    string  `json:"paradiseProductHash"`
		ParadisePostbackURL    string  `json:"paradisePostbackUrl"`
		ParadiseEnabled        bool    `json:"paradiseEnabled"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "depositFeePercent", "withdrawFeeFixed"); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	gw := domain.GatewayConfig{
		Name:                 req.Name,
		DepositFeePercent:    req.DepositFeePercent,
		WithdrawFeeFixed:     req.WithdrawFeeFixed,
		PixExpirationMinutes: req.PixExpirationMinutes,
		ActiveProvider:       req.ActiveProvider,
		SuitPayClientID:      req.SuitPayClientID,
		SuitPayClientSecret:  req.SuitPayClientSecret,
		SuitPaySandbox:       req.SuitPaySandbox,
		SuitPayCallbackURL:   req.SuitPayCallbackURL,
		SuitPayEnabled:       req.SuitPayEnabled,
		ParadiseSecretKey:    req.ParadiseSecretKey,
		ParadiseProductHash:  req.ParadiseProductHash,
		ParadisePostbackURL:  req.ParadisePostbackURL,
		ParadiseEnabled:      req.ParadiseEnabled,
	}

	err := h.configRepo.UpdateGatewayConfig(r.Context(), gw, req.CollectionGraceMinutes)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao salvar configurações do gateway", "CONFIG_UPDATE_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Parâmetros do Gateway e tolerância de coleta atualizados",
	})
}

func (h *AdminHandler) ToggleGlobalLock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Locked bool `json:"locked"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	err := h.configRepo.ToggleGlobalWithdrawLock(r.Context(), req.Locked)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao alterar trava global de saques", "GLOBAL_LOCK_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"locked":  req.Locked,
		"message": "Comutador de trava de emergência global atualizado",
	})
}

func (h *AdminHandler) UpdateAutoApprove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoApproveEnabled          bool    `json:"autoApproveEnabled"`
		AutoApproveWithdrawalsUnder float64 `json:"autoApproveWithdrawalsUnder"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "autoApproveWithdrawalsUnder"); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	err := h.configRepo.UpdateAutoApprove(r.Context(), req.AutoApproveEnabled, req.AutoApproveWithdrawalsUnder)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao salvar aprovação automática", "AUTO_APPROVE_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Regras de aprovação automática de saques salvas com sucesso",
	})
}

func (h *AdminHandler) UpdateReferralsConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReferralCommissionPercent     float64 `json:"referralCommissionPercent"`
		RequiredReferralsCount        int     `json:"requiredReferralsCount"`
		BonusPerMutiraoAmount         float64 `json:"bonusPerMutiraoAmount"`
		RequireReferralsForWithdrawal bool    `json:"requireReferralsForWithdrawal"`
		AntiSelfReferralLock          bool    `json:"antiSelfReferralLock"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "referralCommissionPercent", "bonusPerMutiraoAmount"); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	err := h.configRepo.UpdateReferralConfig(
		r.Context(),
		req.ReferralCommissionPercent,
		req.RequiredReferralsCount,
		req.BonusPerMutiraoAmount,
		req.RequireReferralsForWithdrawal,
		req.AntiSelfReferralLock,
	)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao salvar configurações de afiliados", "REFERRALS_CONFIG_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Regras globais de afiliados atualizadas com sucesso",
	})
}

// 6. COMUNICADOS RICH MEDIA
func (h *AdminHandler) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	list, err := h.announceRepo.GetAll(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "ANNOUNCEMENTS_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    list,
	})
}

func (h *AdminHandler) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var ann domain.Announcement
	if err := json.NewDecoder(r.Body).Decode(&ann); err != nil || ann.Title == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Título do comunicado obrigatório", "INVALID_ANNOUNCEMENT")
		return
	}

	if ann.ID == "" {
		evtID, err := id.New("evt")
		if err != nil {
			middleware.RespondError(w, http.StatusInternalServerError, "falha ao gerar ID do comunicado", "ID_GENERATION_FAILED")
			return
		}
		ann.ID = evtID
	}
	ann.Active = true

	if err := h.announceRepo.Create(r.Context(), &ann); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "CREATE_ANNOUNCEMENT_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.Broadcast("announcement_created", ann)
	}

	middleware.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Comunicado Rich Media publicado com sucesso!",
		"data":    ann,
	})
}

func (h *AdminHandler) ToggleAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ann, err := h.announceRepo.ToggleActive(r.Context(), id)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "TOGGLE_ANNOUNCEMENT_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    ann,
	})
}

func (h *AdminHandler) DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.announceRepo.Delete(r.Context(), id); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "DELETE_ANNOUNCEMENT_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Comunicado removido com sucesso",
	})
}

// 7. CATÁLOGO DE FROTA & OFERTAS
func (h *AdminHandler) SaveFleetCatalog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Catalog []*domain.FleetCatalogItem `json:"catalog"`
	}
	if err := decodeExactMonetaryJSON(r.Body, &req, "catalog[].price", "catalog[].dailyYield", "catalog[].totalContractReturn"); err != nil || len(req.Catalog) == 0 {
		middleware.RespondError(w, http.StatusBadRequest, "Catálogo de frotas obrigatório", "INVALID_CATALOG")
		return
	}

	err := h.catalogRepo.SaveCatalog(r.Context(), req.Catalog)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "SAVE_CATALOG_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Catálogo global de frota salvo com sucesso!",
	})
}

func (h *AdminHandler) CreatePromo(w http.ResponseWriter, r *http.Request) {
	var promo domain.UserPromo
	if err := decodeExactMonetaryJSON(r.Body, &promo, "customPrice"); err != nil || promo.MachineID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Dados da promoção inválidos", "INVALID_PROMO")
		return
	}

	if promo.ID == "" {
		promoID, err := id.New("promo")
		if err != nil {
			middleware.RespondError(w, http.StatusInternalServerError, "falha ao gerar ID da promoção", "ID_GENERATION_FAILED")
			return
		}
		promo.ID = promoID
	}
	promo.Active = true

	if err := h.promoRepo.Create(r.Context(), &promo); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "CREATE_PROMO_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Oferta promocional customizada criada com sucesso!",
		"data":    promo,
	})
}

func (h *AdminHandler) DeletePromo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.promoRepo.Delete(r.Context(), id); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "DELETE_PROMO_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Oferta promocional removida com sucesso",
	})
}

// 8. LOGS DE AUDITORIA E SEGURANÇA
func (h *AdminHandler) GetSecurityLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	userIDParam := r.URL.Query().Get("userId")
	var userID *string
	if userIDParam != "" {
		userID = &userIDParam
	}

	if h.logRepo == nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*domain.SecurityLog{},
		})
		return
	}

	logs, err := h.logRepo.GetSecurityLogs(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "FETCH_SECURITY_LOGS_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    logs,
	})
}

func (h *AdminHandler) GetUsageLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if h.logRepo == nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*domain.UsageLog{},
		})
		return
	}

	logs, err := h.logRepo.GetUsageLogs(r.Context(), limit, offset)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "FETCH_USAGE_LOGS_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    logs,
	})
}

// 9. PASSE DE BATALHA & GESTÃO DOS 12 NÍVEIS
func (h *AdminHandler) GetLevels(w http.ResponseWriter, r *http.Request) {
	if h.levelRepo == nil {
		middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*domain.LevelConfig{},
		})
		return
	}

	levels, err := h.levelRepo.GetAll(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "FETCH_LEVELS_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    levels,
	})
}

func (h *AdminHandler) UpdateLevel(w http.ResponseWriter, r *http.Request) {
	levelStr := chi.URLParam(r, "level")
	levelNum, err := strconv.Atoi(levelStr)
	if err != nil || levelNum < 1 || levelNum > 12 {
		middleware.RespondError(w, http.StatusBadRequest, "Nível inválido (deve ser entre 1 e 12)", "INVALID_LEVEL")
		return
	}

	var req domain.LevelConfig
	if err := decodeExactMonetaryJSON(r.Body, &req, "commissionPercent", "bonusYieldPercent"); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Corpo de requisição inválido", "INVALID_BODY")
		return
	}
	req.Level = levelNum

	if err := h.levelRepo.Update(r.Context(), &req); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "UPDATE_LEVEL_FAILED")
		return
	}

	if h.logRepo != nil {
		gestorID := middleware.GetUserID(r.Context())
		_ = h.logRepo.LogSecurity(r.Context(), nil, fmt.Sprintf("GESTOR_%s_ALTEROU_REGRAS_NIVEL_%d", gestorID, levelNum), middleware.ClientIPFromRequest(r), "", req.Title)
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Regras e premiações do Nível %d atualizadas com sucesso!", levelNum),
		"data":    req,
	})
}

func (h *AdminHandler) ResetSeason(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DurationDays int `json:"durationDays"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.DurationDays <= 0 {
		req.DurationDays = 75
	}

	if err := h.configRepo.ResetSeason(r.Context(), req.DurationDays); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao reiniciar temporada de ranking", "RESET_SEASON_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.Broadcast("season_reset_completed", map[string]interface{}{
			"message": "Nova Temporada do Ranking CatBrasil (75 Dias) iniciada! O pódio da Corrida de Mineração foi reiniciado. Todos os níveis de carreira permanecem intactos!",
		})
	}

	if h.logRepo != nil {
		gestorID := middleware.GetUserID(r.Context())
		_ = h.logRepo.LogSecurity(r.Context(), nil, fmt.Sprintf("GESTOR_%s_REINICIOU_TEMPORADA_RANKING", gestorID), middleware.ClientIPFromRequest(r), "", fmt.Sprintf("Duração: %d dias", req.DurationDays))
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Temporada de Ranking reiniciada com sucesso! Histórico gravado e novo ciclo de 75 dias ativo. Níveis de carreira preservados.",
	})
}

func (h *AdminHandler) UpdateLevel12Maintenance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaintenanceDays   int `json:"maintenanceDays"`
		RequiredReferrals int `json:"requiredReferrals"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	if req.MaintenanceDays <= 0 {
		req.MaintenanceDays = 30
	}
	if req.RequiredReferrals <= 0 {
		req.RequiredReferrals = 1
	}

	if err := h.configRepo.UpdateLevel12Config(r.Context(), req.MaintenanceDays, req.RequiredReferrals); err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "Falha ao atualizar parâmetros de manutenção do nível 12", "UPDATE_LEVEL12_CONFIG_FAILED")
		return
	}

	if h.logRepo != nil {
		gestorID := middleware.GetUserID(r.Context())
		_ = h.logRepo.LogSecurity(r.Context(), nil, fmt.Sprintf("GESTOR_%s_ATUALIZOU_MANUTENCAO_NIVEL12", gestorID), middleware.ClientIPFromRequest(r), "", fmt.Sprintf("Tolerância: %d dias, Novos convidados: %d", req.MaintenanceDays, req.RequiredReferrals))
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Parâmetros de manutenção e atividade do Nível 12 atualizados com sucesso!",
	})
}
