package postgres

import (
	"context"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RankingRepository struct {
	pool *pgxpool.Pool
}

func NewRankingRepository(pool *pgxpool.Pool) *RankingRepository {
	return &RankingRepository{pool: pool}
}

var defaultLeaderboardPrizes = []struct {
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

var RecifeLocation *time.Location

func init() {
	var err error
	RecifeLocation, err = time.LoadLocation("America/Recife")
	if err != nil {
		RecifeLocation = time.FixedZone("America/Recife", -3*3600)
	}
}

// CurrentWeekPeriodStart calcula a segunda-feira 00:00:00 do ciclo semanal em America/Recife
func CurrentWeekPeriodStart(t time.Time) time.Time {
	tRecife := t.In(RecifeLocation)
	weekday := int(tRecife.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysSinceMonday := weekday - 1
	return time.Date(tRecife.Year(), tRecife.Month(), tRecife.Day()-daysSinceMonday, 0, 0, 0, 0, RecifeLocation)
}

func (r *RankingRepository) GetWeeklyLeaderboard(ctx context.Context, limit int) ([]*domain.LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	periodStart := CurrentWeekPeriodStart(time.Now())

	query := `
		SELECT user_id, username, weekly_tonnage, updated_at
		FROM mining_race_weekly
		WHERE period_start = $1
		ORDER BY weekly_tonnage DESC, user_id ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, periodStart, limit)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar ranking semanal: %w", err)
	}
	defer rows.Close()

	var list []*domain.LeaderboardEntry
	rank := 1

	for rows.Next() {
		var entry domain.LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Username, &entry.WeeklyTonnage, &entry.UpdatedAt); err != nil {
			return nil, err
		}

		entry.Rank = rank
		if rank-1 < len(defaultLeaderboardPrizes) {
			entry.Prize = defaultLeaderboardPrizes[rank-1].Prize
			entry.Badge = defaultLeaderboardPrizes[rank-1].Badge
		} else {
			entry.Prize = "Selo VIP Participante"
			entry.Badge = "⭐ TOP 10"
		}

		list = append(list, &entry)
		rank++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante iteração do ranking: %w", err)
	}

	return list, nil
}

func (r *RankingRepository) GetUserRank(ctx context.Context, userID string) (*domain.LeaderboardEntry, error) {
	periodStart := CurrentWeekPeriodStart(time.Now())

	query := `
		WITH ranked AS (
			SELECT 
				user_id, username, weekly_tonnage, updated_at,
				ROW_NUMBER() OVER (ORDER BY weekly_tonnage DESC, user_id ASC) as rank
			FROM mining_race_weekly
			WHERE period_start = $1
		)
		SELECT rank, user_id, username, weekly_tonnage, updated_at
		FROM ranked
		WHERE user_id = $2
	`

	var entry domain.LeaderboardEntry
	err := r.pool.QueryRow(ctx, query, periodStart, userID).Scan(
		&entry.Rank, &entry.UserID, &entry.Username, &entry.WeeklyTonnage, &entry.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &domain.LeaderboardEntry{
				Rank:          99,
				UserID:        userID,
				Username:      "Operador",
				WeeklyTonnage: 0,
				Prize:         "Selo VIP Participante",
				Badge:         "MINERADOR INICIANTE",
				UpdatedAt:     time.Now().In(RecifeLocation),
			}, nil
		}
		return nil, err
	}

	if entry.Rank <= len(defaultLeaderboardPrizes) {
		entry.Prize = defaultLeaderboardPrizes[entry.Rank-1].Prize
		entry.Badge = defaultLeaderboardPrizes[entry.Rank-1].Badge
	} else {
		entry.Prize = "Selo VIP Participante"
		entry.Badge = "MINERADOR EM ATIVIDADE"
	}

	return &entry, nil
}

func (r *RankingRepository) AddOrUpdateWeeklyTonnage(ctx context.Context, userID, username string, tonnage float64) error {
	periodStart := CurrentWeekPeriodStart(time.Now())

	query := `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (period_start, user_id) DO UPDATE SET
			username = EXCLUDED.username,
			weekly_tonnage = mining_race_weekly.weekly_tonnage + EXCLUDED.weekly_tonnage,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.pool.Exec(ctx, query, periodStart, userID, username, tonnage)
	return err
}

func (r *RankingRepository) ResetWeeklyLeaderboard(ctx context.Context) error {
	periodStart := CurrentWeekPeriodStart(time.Now())
	_, err := r.pool.Exec(ctx, "DELETE FROM mining_race_weekly WHERE period_start = $1", periodStart)
	return err
}
