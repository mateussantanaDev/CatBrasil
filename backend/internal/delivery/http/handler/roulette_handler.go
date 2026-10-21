package handler

import (
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/usecase/roulette"
)

type RouletteHandler struct {
	rouletteUC *roulette.RouletteUseCase
	hub        *ws.Hub
}

func NewRouletteHandler(rouletteUC *roulette.RouletteUseCase, hub *ws.Hub) *RouletteHandler {
	return &RouletteHandler{
		rouletteUC: rouletteUC,
		hub:        hub,
	}
}

func (h *RouletteHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	slices, err := h.rouletteUC.GetConfig(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "ROULETTE_CONFIG_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, slices)
}

func (h *RouletteHandler) Spin(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	resp, err := h.rouletteUC.Spin(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "SPIN_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "roulette_won", resp)
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *RouletteHandler) AssembleParts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	resp, err := h.rouletteUC.AssembleParts(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "ASSEMBLE_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "machine_assembled", resp)
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *RouletteHandler) ResetSpin(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	err := h.rouletteUC.ResetUserSpin(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "RESET_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Giro diário resetado com sucesso para testes.",
	})
}
