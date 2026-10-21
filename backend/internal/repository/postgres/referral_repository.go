package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReferralRepository struct {
	pool *pgxpool.Pool
}

func NewReferralRepository(pool *pgxpool.Pool) *ReferralRepository {
	return &ReferralRepository{pool: pool}
}

const referralSelectFields = `
	id, referrer_id, referred_user_id, nome, status, deposito, created_at, updated_at
`

func scanReferral(row pgx.Row) (*domain.Referral, error) {
	var ref domain.Referral
	var statusStr string

	err := row.Scan(
		&ref.ID, &ref.ReferrerID, &ref.ReferredUserID, &ref.Nome,
		&statusStr, &ref.Deposito, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("indicado não encontrado")
		}
		return nil, fmt.Errorf("erro ao mapear afiliado: %w", err)
	}

	ref.Status = domain.ReferralStatus(statusStr)
	return &ref, nil
}

func (r *ReferralRepository) GetByReferrerID(ctx context.Context, referrerID string) ([]*domain.Referral, error) {
	query := fmt.Sprintf("SELECT %s FROM referrals WHERE referrer_id = $1 ORDER BY created_at DESC", referralSelectFields)
	rows, err := r.pool.Query(ctx, query, referrerID)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar indicados: %w", err)
	}
	defer rows.Close()

	var list []*domain.Referral
	for rows.Next() {
		ref, err := scanReferral(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, ref)
	}

	return list, nil
}

func (r *ReferralRepository) GetAll(ctx context.Context) ([]*domain.Referral, error) {
	query := fmt.Sprintf("SELECT %s FROM referrals ORDER BY created_at DESC", referralSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar todos os indicados: %w", err)
	}
	defer rows.Close()

	var list []*domain.Referral
	for rows.Next() {
		ref, err := scanReferral(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, ref)
	}

	return list, nil
}

func (r *ReferralRepository) GetByID(ctx context.Context, id string) (*domain.Referral, error) {
	query := fmt.Sprintf("SELECT %s FROM referrals WHERE id = $1", referralSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanReferral(row)
}

func (r *ReferralRepository) GetByReferredName(ctx context.Context, name string) (*domain.Referral, error) {
	query := fmt.Sprintf("SELECT %s FROM referrals WHERE LOWER(nome) = LOWER($1)", referralSelectFields)
	row := r.pool.QueryRow(ctx, query, name)
	return scanReferral(row)
}

func (r *ReferralRepository) Create(ctx context.Context, ref *domain.Referral) error {
	query := `
		INSERT INTO referrals (
			id, referrer_id, referred_user_id, nome, status, deposito, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = time.Now()
	}
	ref.UpdatedAt = time.Now()

	status := string(ref.Status)
	if status == "" {
		status = string(domain.ReferralStatusInativo)
	}

	_, err := r.pool.Exec(ctx, query,
		ref.ID, ref.ReferrerID, ref.ReferredUserID, ref.Nome,
		status, ref.Deposito, ref.CreatedAt, ref.UpdatedAt,
	)
	return err
}

func (r *ReferralRepository) Update(ctx context.Context, ref *domain.Referral) error {
	query := `
		UPDATE referrals SET
			nome = $1, status = $2, deposito = $3, updated_at = $4
		WHERE id = $5
	`

	ref.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, query,
		ref.Nome, string(ref.Status), ref.Deposito, ref.UpdatedAt, ref.ID,
	)
	return err
}

func (r *ReferralRepository) CountActiveByReferrerID(ctx context.Context, referrerID string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM referrals WHERE referrer_id = $1 AND status = 'ativo'"
	err := r.pool.QueryRow(ctx, query, referrerID).Scan(&count)
	return count, err
}
