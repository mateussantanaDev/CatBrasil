package domain

import (
	"context"
	"time"
)

// LeaderboardEntry representa um participante na Corrida Semanal de Mineração
type LeaderboardEntry struct {
	Rank          int       `json:"rank"`
	UserID        string    `json:"userId"`
	Username      string    `json:"username"`
	WeeklyTonnage float64   `json:"weeklyTonnage"`
	Prize         string    `json:"prize"`
	Badge         string    `json:"badge"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// RankingLeaderboard consolida os dados do ranking semanal para a interface
type RankingLeaderboard struct {
	Leaderboard       []*LeaderboardEntry `json:"leaderboard"`
	UserRank          *LeaderboardEntry   `json:"userRank"`
	SeasonEndsInHours int                 `json:"seasonEndsInHours"`
}

// RankingRepository define o contrato da camada de persistência para o Ranking Semanal
type RankingRepository interface {
	GetWeeklyLeaderboard(ctx context.Context, limit int) ([]*LeaderboardEntry, error)
	GetUserRank(ctx context.Context, userID string) (*LeaderboardEntry, error)
	AddOrUpdateWeeklyTonnage(ctx context.Context, userID, username string, tonnage float64) error
	ResetWeeklyLeaderboard(ctx context.Context) error
}
