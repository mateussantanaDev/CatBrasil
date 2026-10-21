package handler

import (
	"encoding/json"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
)

type ReferralHandler struct {
	referralRepo domain.ReferralRepository
	userRepo     domain.UserRepository
}

func NewReferralHandler(referralRepo domain.ReferralRepository, userRepo domain.UserRepository) *ReferralHandler {
	return &ReferralHandler{
		referralRepo: referralRepo,
		userRepo:     userRepo,
	}
}

func (h *ReferralHandler) GetReferrals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	isSuspended := false
	if h.userRepo != nil {
		user, _ := h.userRepo.GetByID(r.Context(), userID)
		if user != nil {
			isSuspended = user.ReferralBonusSuspended
		}
	}

	var referrals []*domain.Referral
	if h.referralRepo != nil {
		var err error
		referrals, err = h.referralRepo.GetByReferrerID(r.Context(), userID)
		if err != nil {
			middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "REFERRALS_FETCH_FAILED")
			return
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"referralLink":   "https://catbrasil.app/ref/" + userID,
		"commissionRate": 5.0,
		"isSuspended":    isSuspended,
		"referrals":      referrals,
	})
}

func (h *ReferralHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	referrals, _ := h.referralRepo.GetByReferrerID(r.Context(), userID)
	if len(referrals) > 0 {
		status := domain.ReferralStatusInativo
		deposito := 0.0
		if req.Active {
			status = domain.ReferralStatusAtivo
			deposito = 350.00
		}
		referrals[0].Status = status
		referrals[0].Deposito = deposito
		_ = h.referralRepo.Update(r.Context(), referrals[0])
	} else {
		status := domain.ReferralStatusInativo
		deposito := 0.0
		if req.Active {
			status = domain.ReferralStatusAtivo
			deposito = 350.00
		}
		newRef := &domain.Referral{
			ID:         "ref-simulated",
			ReferrerID: userID,
			Nome:       "Maria Oliveira (Simulado)",
			Status:     status,
			Deposito:   deposito,
		}
		_ = h.referralRepo.Create(r.Context(), newRef)
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Status de indicados de rede atualizado no simulador.",
	})
}
