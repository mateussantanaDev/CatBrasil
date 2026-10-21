package handler

import (
	"encoding/json"
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/usecase/shop"
)

type ShopHandler struct {
	shopUC *shop.ShopUseCase
	hub    *ws.Hub
}

func NewShopHandler(shopUC *shop.ShopUseCase, hub *ws.Hub) *ShopHandler {
	return &ShopHandler{
		shopUC: shopUC,
		hub:    hub,
	}
}

func (h *ShopHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.shopUC.GetCatalog(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "CATALOG_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, catalog)
}

func (h *ShopHandler) RentMachine(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	var req struct {
		MachineID    string `json:"machineId"`
		MachineModel string `json:"machineModel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Payload inválido", "INVALID_PAYLOAD")
		return
	}

	catalogID := req.MachineID
	if catalogID == "" {
		catalogID = req.MachineModel
	}
	if catalogID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "Informe o modelo de máquina para aluguel", "INVALID_MODEL")
		return
	}

	resp, err := h.shopUC.RentMachine(r.Context(), userID, catalogID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error(), "RENT_FAILED")
		return
	}

	if h.hub != nil {
		h.hub.NotifyUser(userID, "machine_rented", resp)
	}

	middleware.RespondJSON(w, http.StatusOK, resp)
}
