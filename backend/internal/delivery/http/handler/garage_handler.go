package handler

import (
	"encoding/json"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/usecase/garage"
)

type GarageHandler struct {
	garageUC *garage.GarageUseCase
	hub      *ws.Hub
}

func NewGarageHandler(garageUC *garage.GarageUseCase, hub *ws.Hub) *GarageHandler {
	return &GarageHandler{
		garageUC: garageUC,
		hub:      hub,
	}
}

func (h *GarageHandler) GetMachines(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	machines, err := h.garageUC.GetUserMachines(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "GARAGE_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, machines)
}

func (h *GarageHandler) Collect(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		MachineID string `json:"machineId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MachineID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Informe o ID da máquina", "INVALID_MACHINE_ID")
		return
	}

	resp, err := h.garageUC.CollectYield(r.Context(), userID, req.MachineID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "COLLECT_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "collect_success", resp)
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}

func (h *GarageHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		State string `json:"state"` // "aguardando" | "disponivel" | "expirado" | "reset"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	err := h.garageUC.SimulateMachineState(r.Context(), userID, req.State)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "SIMULATION_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Estado da garagem simulado com sucesso para: " + req.State,
	})
}
