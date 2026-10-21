package handler

import (
	"net/http"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/usecase/ranking"
)

type RankingHandler struct {
	rankingUC *ranking.RankingUseCase
}

func NewRankingHandler(rankingUC *ranking.RankingUseCase) *RankingHandler {
	return &RankingHandler{rankingUC: rankingUC}
}

func (h *RankingHandler) GetWeeklyRanking(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
		return
	}

	board, err := h.rankingUC.GetWeeklyRanking(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error(), "RANKING_FETCH_FAILED")
		return
	}

	middleware.RespondJSON(w, http.StatusOK, board)
}
