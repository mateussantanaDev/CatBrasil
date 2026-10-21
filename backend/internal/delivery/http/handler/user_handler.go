package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/auth"
	"catbrasil/backend/internal/usecase/garage"
	"catbrasil/backend/pkg/identity"
)

type UserHandler struct {
	authUC       *auth.AuthUseCase
	garageUC     *garage.GarageUseCase
	userRepo     domain.UserRepository
	referralRepo domain.ReferralRepository
	identityRepo domain.IdentityRepository
}

func (h *UserHandler) SetIdentityRepository(repo domain.IdentityRepository) { h.identityRepo = repo }

func NewUserHandler(
	authUC *auth.AuthUseCase,
	garageUC *garage.GarageUseCase,
	userRepo domain.UserRepository,
	referralRepo domain.ReferralRepository,
) *UserHandler {
	return &UserHandler{
		authUC:       authUC,
		garageUC:     garageUC,
		userRepo:     userRepo,
		referralRepo: referralRepo,
	}
}

func (h *UserHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.RespondError(w, http.StatusNotFound, "Operador não encontrado", "OPERATOR_NOT_FOUND")
		return
	}

	activeCount := 0
	isSuspended := user.ReferralBonusSuspended
	var referrals []*domain.Referral
	if h.referralRepo != nil {
		referrals, _ = h.referralRepo.GetByReferrerID(r.Context(), userID)
		for _, ref := range referrals {
			if ref.Status == domain.ReferralStatusAtivo {
				activeCount++
			}
		}
	}

	identityStatus := "missing"
	if h.identityRepo != nil {
		identityStatus, err = h.identityRepo.Status(r.Context(), userID)
		if err != nil {
			middleware.RespondError(w, http.StatusServiceUnavailable, "Identidade indisponível", "IDENTITY_UNAVAILABLE")
			return
		}
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"id":                              user.ID,
		"username":                        user.Username,
		"role":                            user.Role,
		"phone":                           user.Phone,
		"balance":                         user.Balance,
		"yieldBalanceAvailable":           user.YieldBalanceAvailable,
		"xp":                              user.XP,
		"nivel":                           user.Nivel,
		"hasSecurityPassword":             user.SecurityPassword != "",
		"identityStatus":                  identityStatus,
		"promoUnlocked":                   user.PromoUnlocked,
		"freeWithdrawActive":              user.FreeWithdrawActive,
		"coletasCompleted":                user.ColetasCompleted,
		"tratorRented":                    user.TratorRented,
		"alreadySpunToday":                user.AlreadySpunToday,
		"withdrawLocked":                  user.WithdrawLocked,
		"referralBonusSuspended":          isSuspended,
		"requiredRollover":                user.RequiredRollover,
		"currentRollover":                 user.CurrentRollover,
		"rolloverProgressPercent":         user.RolloverProgressPercent(),
		"twoFactorEnabled":                user.TwoFactorEnabled,
		"partsInventory":                  user.GetPartsInventory(),
		"referralsCount":                  len(referrals),
		"activeReferralsCount":            activeCount,
		"insuranceVouchersCount":          user.InsuranceVouchersCount,
		"maxMachinesAllowed":              user.MaxMachinesAllowed,
		"hasMaxLevelPlaque":               user.HasMaxLevelPlaque,
		"seasonNumber":                    user.SeasonNumber,
		"customReferralCommissionPercent": user.CustomReferralCommissionPercent,
	})
}

func (h *UserHandler) SubmitIdentity(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}
	if h.identityRepo == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "Identidade indisponível", "IDENTITY_UNAVAILABLE")
		return
	}
	var req struct {
		CPF       string `json:"cpf"`
		LegalName string `json:"legalName"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}
	err := h.identityRepo.Submit(r.Context(), userID, domain.IdentityPerson{CPF: req.CPF, LegalName: req.LegalName, Email: req.Email})
	if err != nil {
		code, status := "IDENTITY_UNAVAILABLE", http.StatusServiceUnavailable
		if errors.Is(err, identity.ErrInvalidCPF) || errors.Is(err, identity.ErrInvalidIdentity) {
			code, status = "INVALID_IDENTITY", http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrIdentityAlreadySubmitted) || errors.Is(err, domain.ErrIdentityAlreadyExists) {
			code, status = "IDENTITY_CONFLICT", http.StatusConflict
		}
		middleware.RespondError(w, status, err.Error(), code)
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{"identityStatus": "pending"})
}

func (h *UserHandler) SaveSecurityPassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		Password        string `json:"password"`
		CurrentPassword string `json:"currentPassword"`
		AccountPassword string `json:"accountPassword"`
		TwoFactorCode   string `json:"twoFactorCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	err := h.authUC.UpdateSecurityPassword(r.Context(), userID, req.Password, req.CurrentPassword, req.AccountPassword, req.TwoFactorCode)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			middleware.RespondError(w, http.StatusForbidden, "Senha da conta incorreta", "REAUTH_FAILED")
			return
		}
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "SECURITY_PASSWORD_ERROR")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Senha de segurança financeira atualizada com sucesso.",
	})
}

func (h *UserHandler) LevelUp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	resp, err := h.garageUC.LevelUp(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "LEVEL_UP_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *UserHandler) GetBattlePass(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	status, err := h.garageUC.GetBattlePassStatus(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "BATTLE_PASS_STATUS_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

func (h *UserHandler) RecoverInsurance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	resp, err := h.garageUC.RecoverLostDaysWithInsurance(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "INSURANCE_RECOVERY_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}
