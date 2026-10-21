package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouletteRepository struct {
	pool *pgxpool.Pool
}

func NewRouletteRepository(pool *pgxpool.Pool) *RouletteRepository {
	return &RouletteRepository{pool: pool}
}

const rouletteSelectFields = `
	id, label, amount, probability, color, text_color, created_at, updated_at
`

func scanRouletteSlice(row pgx.Row) (*domain.RouletteSlice, error) {
	var s domain.RouletteSlice
	err := row.Scan(
		&s.ID, &s.Label, &s.Amount, &s.Probability, &s.Color, &s.TextColor,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidRouletteSlice
		}
		return nil, fmt.Errorf("erro ao mapear fatia da roleta: %w", err)
	}
	return &s, nil
}

func (r *RouletteRepository) GetSlices(ctx context.Context) ([]*domain.RouletteSlice, error) {
	query := fmt.Sprintf("SELECT %s FROM roulette_slices ORDER BY id ASC", rouletteSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar fatias da roleta: %w", err)
	}
	defer rows.Close()

	var list []*domain.RouletteSlice
	for rows.Next() {
		s, err := scanRouletteSlice(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}

	return list, nil
}

func (r *RouletteRepository) GetSliceByID(ctx context.Context, id int) (*domain.RouletteSlice, error) {
	query := fmt.Sprintf("SELECT %s FROM roulette_slices WHERE id = $1", rouletteSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanRouletteSlice(row)
}

func (r *RouletteRepository) SaveSlices(ctx context.Context, slices []*domain.RouletteSlice) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação da roleta: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Limpa fatias anteriores
	if _, err := tx.Exec(ctx, "DELETE FROM roulette_slices"); err != nil {
		return fmt.Errorf("falha ao limpar fatias anteriores: %w", err)
	}

	insertQuery := `
		INSERT INTO roulette_slices (
			id, label, amount, probability, color, text_color, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	now := time.Now()
	for _, s := range slices {
		amount, err := money.FromFloat64(s.Amount)
		if err != nil {
			return err
		}
		if s.CreatedAt.IsZero() {
			s.CreatedAt = now
		}
		s.UpdatedAt = now

		_, err = tx.Exec(ctx, insertQuery,
			s.ID, s.Label, amount.String(), s.Probability, s.Color, s.TextColor, s.CreatedAt, s.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("falha ao salvar fatia #%d: %w", s.ID, err)
		}
	}

	return tx.Commit(ctx)
}
