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

type PromoRepository struct {
	pool *pgxpool.Pool
}

func NewPromoRepository(pool *pgxpool.Pool) *PromoRepository {
	return &PromoRepository{pool: pool}
}

const promoSelectFields = `
	id, title, target_type, target_id, machine_id,
	custom_price, custom_return_percent_per_day, custom_max_purchases,
	active, created_at, updated_at
`

func scanPromo(row pgx.Row) (*domain.UserPromo, error) {
	var p domain.UserPromo
	err := row.Scan(
		&p.ID, &p.Title, &p.TargetType, &p.TargetID, &p.MachineID,
		&p.CustomPrice, &p.CustomReturnPercentPerDay, &p.CustomMaxPurchases,
		&p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPromoNotFound
		}
		return nil, fmt.Errorf("erro ao mapear promoção customizada: %w", err)
	}
	return &p, nil
}

func (r *PromoRepository) GetAll(ctx context.Context) ([]*domain.UserPromo, error) {
	query := fmt.Sprintf("SELECT %s FROM user_promos ORDER BY created_at DESC", promoSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar promoções: %w", err)
	}
	defer rows.Close()

	var list []*domain.UserPromo
	for rows.Next() {
		p, err := scanPromo(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}

	return list, nil
}

func (r *PromoRepository) GetByTarget(ctx context.Context, targetType, targetID string) ([]*domain.UserPromo, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM user_promos 
		WHERE active = TRUE AND (
			target_type = 'all' OR 
			(target_type = $1 AND target_id = $2)
		)
		ORDER BY created_at DESC
	`, promoSelectFields)

	rows, err := r.pool.Query(ctx, query, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar promoções do alvo: %w", err)
	}
	defer rows.Close()

	var list []*domain.UserPromo
	for rows.Next() {
		p, err := scanPromo(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}

	return list, nil
}

func (r *PromoRepository) GetByID(ctx context.Context, id string) (*domain.UserPromo, error) {
	query := fmt.Sprintf("SELECT %s FROM user_promos WHERE id = $1", promoSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanPromo(row)
}

func (r *PromoRepository) Create(ctx context.Context, p *domain.UserPromo) error {
	price, err := money.FromFloat64(p.CustomPrice)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO user_promos (
			id, title, target_type, target_id, machine_id,
			custom_price, custom_return_percent_per_day, custom_max_purchases,
			active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11
		)
	`

	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()

	_, err = r.pool.Exec(ctx, query,
		p.ID, p.Title, p.TargetType, p.TargetID, p.MachineID,
		price.String(), p.CustomReturnPercentPerDay, p.CustomMaxPurchases,
		p.Active, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *PromoRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM user_promos WHERE id = $1", id)
	return err
}
