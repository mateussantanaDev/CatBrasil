package ranking_test

import (
	"context"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/ranking"
)

type mockRankingRepo struct {
	entries []*domain.LeaderboardEntry
}

func (m *mockRankingRepo) GetWeeklyLeaderboard(ctx context.Context, limit int) ([]*domain.LeaderboardEntry, error) {
	return m.entries, nil
}
func (m *mockRankingRepo) GetUserRank(ctx context.Context, userID string) (*domain.LeaderboardEntry, error) {
	for _, e := range m.entries {
		if e.UserID == userID {
			return e, nil
		}
	}
	return nil, nil
}
func (m *mockRankingRepo) AddOrUpdateWeeklyTonnage(ctx context.Context, userID, username string, tonnage float64) error {
	return nil
}
func (m *mockRankingRepo) ResetWeeklyLeaderboard(ctx context.Context) error {
	m.entries = nil
	return nil
}

func TestRankingUseCase_GetWeeklyRanking(t *testing.T) {
	ctx := context.Background()
	repo := &mockRankingRepo{
		entries: []*domain.LeaderboardEntry{
			{Rank: 1, UserID: "USR-1", Username: "miner_pro", WeeklyTonnage: 5400.0, UpdatedAt: time.Now()},
			{Rank: 2, UserID: "USR-2", Username: "cat_operator", WeeklyTonnage: 3200.0, UpdatedAt: time.Now()},
		},
	}

	uc := ranking.NewRankingUseCase(repo)

	res, err := uc.GetWeeklyRanking(ctx, "USR-1")
	if err != nil {
		t.Fatalf("falha ao obter ranking semanal: %v", err)
	}

	if len(res.Leaderboard) != 2 {
		t.Errorf("esperado 2 entradas no leaderboard, obteve: %d", len(res.Leaderboard))
	}

	if res.UserRank == nil || res.UserRank.Rank != 1 {
		t.Errorf("esperado usuário no Rank 1")
	}

	if res.Leaderboard[0].Prize != "Escavadeira CAT 320 TURBO (7 Dias)" {
		t.Errorf("prêmio do 1º colocado incorreto: %s", res.Leaderboard[0].Prize)
	}
}
