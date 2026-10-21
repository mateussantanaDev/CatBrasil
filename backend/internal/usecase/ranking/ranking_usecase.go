package ranking

import (
	"context"

	"catbrasil/backend/internal/domain"
)

type RankingUseCase struct {
	rankingRepo domain.RankingRepository
}

func NewRankingUseCase(rankingRepo domain.RankingRepository) *RankingUseCase {
	return &RankingUseCase{rankingRepo: rankingRepo}
}

var officialPrizes = []struct {
	Prize string
	Badge string
}{
	{Prize: "Escavadeira CAT 320 TURBO (7 Dias)", Badge: "👑 LENDÁRIO"},
	{Prize: "Pá Carregadeira CAT 966M (7 Dias)", Badge: "🥇 OURO"},
	{Prize: "Motoniveladora CAT 140M (7 Dias)", Badge: "🥈 PRATA"},
	{Prize: "Bônus +20% XP", Badge: "🥉 BRONZE"},
	{Prize: "Bônus +15% XP", Badge: "⭐ TOP 5"},
	{Prize: "Selo VIP Ouro", Badge: "⭐ TOP 10"},
	{Prize: "Selo VIP Prata", Badge: "⭐ TOP 10"},
	{Prize: "Selo VIP Prata", Badge: "⭐ TOP 10"},
	{Prize: "Selo VIP Bronze", Badge: "⭐ TOP 10"},
	{Prize: "Selo VIP Bronze", Badge: "⭐ TOP 10"},
}

// GetWeeklyRanking retorna o placar oficial dos 10 maiores mineradores e a posição do usuário logado
func (uc *RankingUseCase) GetWeeklyRanking(ctx context.Context, userID string) (*domain.RankingLeaderboard, error) {
	rows, err := uc.rankingRepo.GetWeeklyLeaderboard(ctx, 10)
	if err != nil {
		return nil, err
	}

	leaderboard := make([]*domain.LeaderboardEntry, len(rows))
	var userRank *domain.LeaderboardEntry

	for i, r := range rows {
		prize := "Selo VIP Participante"
		badge := "⭐ TOP 10"
		if i < len(officialPrizes) {
			prize = officialPrizes[i].Prize
			badge = officialPrizes[i].Badge
		}

		entry := &domain.LeaderboardEntry{
			Rank:          i + 1,
			UserID:        r.UserID,
			Username:      r.Username,
			WeeklyTonnage: r.WeeklyTonnage,
			Prize:         prize,
			Badge:         badge,
			UpdatedAt:     r.UpdatedAt,
		}
		leaderboard[i] = entry

		if r.UserID == userID {
			userRank = entry
		}
	}

	if userRank == nil {
		userRank = &domain.LeaderboardEntry{
			Rank:          99,
			UserID:        userID,
			Username:      "Operador",
			WeeklyTonnage: 0.0,
			Badge:         "MINERADOR INICIANTE",
			Prize:         "Selo VIP Participante",
		}
	}

	return &domain.RankingLeaderboard{
		Leaderboard:       leaderboard,
		UserRank:          userRank,
		SeasonEndsInHours: 48,
	}, nil
}
