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

type MachineRepository struct {
	pool *pgxpool.Pool
}

func NewMachineRepository(pool *pgxpool.Pool) *MachineRepository {
	return &MachineRepository{pool: pool}
}

const machineSelectFields = `
	id, user_id, name, cost, yield_amount, last_collected_at,
	state, override_state, override_seconds_remaining, created_at, updated_at
`

func scanMachine(row pgx.Row) (*domain.Machine, error) {
	var m domain.Machine
	var stateStr string

	err := row.Scan(
		&m.ID, &m.UserID, &m.Name, &m.Cost, &m.YieldAmount, &m.LastCollectedAt,
		&stateStr, &m.OverrideState, &m.OverrideSecondsRemaining, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMachineNotFound
		}
		return nil, fmt.Errorf("erro ao mapear máquina da garagem: %w", err)
	}

	m.State = domain.MachineState(stateStr)
	return &m, nil
}

func (r *MachineRepository) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	query := fmt.Sprintf("SELECT %s FROM machines WHERE id = $1", machineSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanMachine(row)
}

func (r *MachineRepository) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	// Consulta legada: o lock não cobre escritas subsequentes fora desta chamada.
	query := fmt.Sprintf("SELECT %s FROM machines WHERE id = $1 FOR UPDATE", machineSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanMachine(row)
}

func (r *MachineRepository) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	query := fmt.Sprintf("SELECT %s FROM machines WHERE user_id = $1 ORDER BY created_at ASC", machineSelectFields)
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar frota do operador: %w", err)
	}
	defer rows.Close()

	var list []*domain.Machine
	for rows.Next() {
		m, err := scanMachine(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}

	return list, nil
}

func (r *MachineRepository) Create(ctx context.Context, m *domain.Machine) error {
	query := `
		INSERT INTO machines (
			id, user_id, name, cost, yield_amount, last_collected_at,
			state, override_state, override_seconds_remaining, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
	`

	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.UpdatedAt = time.Now()

	state := string(m.State)
	if state == "" {
		state = string(domain.MachineStateAguardando)
	}

	_, err := r.pool.Exec(ctx, query,
		m.ID, m.UserID, m.Name, m.Cost, m.YieldAmount, m.LastCollectedAt,
		state, m.OverrideState, m.OverrideSecondsRemaining, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("falha ao inserir máquina na garagem: %w", err)
	}

	return nil
}

func (r *MachineRepository) Update(ctx context.Context, m *domain.Machine) error {
	query := `
		UPDATE machines SET
			name = $1, cost = $2, yield_amount = $3, last_collected_at = $4,
			state = $5, override_state = $6, override_seconds_remaining = $7,
			updated_at = $8
		WHERE id = $9
	`

	m.UpdatedAt = time.Now()

	cmdTag, err := r.pool.Exec(ctx, query,
		m.Name, m.Cost, m.YieldAmount, m.LastCollectedAt,
		string(m.State), m.OverrideState, m.OverrideSecondsRemaining,
		m.UpdatedAt, m.ID,
	)
	if err != nil {
		return fmt.Errorf("falha ao atualizar máquina: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrMachineNotFound
	}

	return nil
}

func (r *MachineRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM machines WHERE id = $1", id)
	return err
}
