package handler

import (
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
)

type AnnouncementHandler struct {
	announceRepo domain.AnnouncementRepository
}

func NewAnnouncementHandler(announceRepo domain.AnnouncementRepository) *AnnouncementHandler {
	return &AnnouncementHandler{announceRepo: announceRepo}
}

func (h *AnnouncementHandler) GetActiveAnnouncements(w http.ResponseWriter, r *http.Request) {
	list, err := h.announceRepo.GetActive(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "ANNOUNCEMENTS_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, list)
}
