package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LevelRepository struct {
	pool *pgxpool.Pool
}

func NewLevelRepository(pool *pgxpool.Pool) *LevelRepository {
	return &LevelRepository{pool: pool}
}

const levelSelectFields = `
	l.level, l.title, l.tier_name, l.required_referrals, l.required_machines, l.required_coletas, l.required_tonnage,
	l.unlocked_machine_id, COALESCE(f.name, '') as unlocked_machine_name,
	l.commission_percent, l.bonus_yield_percent, l.has_insurance_voucher, l.insurance_days_recovered,
	l.has_extra_roulette_spin, l.has_vip_roulette, l.max_machines_allowed, l.special_reward_text,
	l.badge_name, l.badge_icon, l.created_at, l.updated_at
`

func scanLevelConfig(row pgx.Row) (*domain.LevelConfig, error) {
	var c domain.LevelConfig
	var unlockedMachineID sql.NullString
	var unlockedMachineName sql.NullString

	err := row.Scan(
		&c.Level, &c.Title, &c.TierName, &c.RequiredReferrals, &c.RequiredMachines, &c.RequiredColetas, &c.RequiredTonnage,
		&unlockedMachineID, &unlockedMachineName,
		&c.CommissionPercent, &c.BonusYieldPercent, &c.HasInsuranceVoucher, &c.InsuranceDaysRecovered,
		&c.HasExtraRouletteSpin, &c.HasVIPRoulette, &c.MaxMachinesAllowed, &c.SpecialRewardText,
		&c.BadgeName, &c.BadgeIcon, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("nível não encontrado: %w", err)
		}
		return nil, fmt.Errorf("erro ao ler nível: %w", err)
	}

	if unlockedMachineID.Valid && unlockedMachineID.String != "" {
		str := unlockedMachineID.String
		c.UnlockedMachineID = &str
	}
	if unlockedMachineName.Valid {
		c.UnlockedMachineName = unlockedMachineName.String
	}

	return &c, nil
}

func (r *LevelRepository) GetAll(ctx context.Context) ([]*domain.LevelConfig, error) {
	query := fmt.Sprintf(`
		SELECT %s 
		FROM level_configs l
		LEFT JOIN fleet_catalog f ON l.unlocked_machine_id = f.id
		ORDER BY l.level ASC
	`, levelSelectFields)

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar níveis: %w", err)
	}
	defer rows.Close()

	var levels []*domain.LevelConfig
	for rows.Next() {
		l, err := scanLevelConfig(rows)
		if err != nil {
			return nil, err
		}
		levels = append(levels, l)
	}

	return levels, nil
}

func (r *LevelRepository) GetByLevel(ctx context.Context, level int) (*domain.LevelConfig, error) {
	query := fmt.Sprintf(`
		SELECT %s 
		FROM level_configs l
		LEFT JOIN fleet_catalog f ON l.unlocked_machine_id = f.id
		WHERE l.level = $1
	`, levelSelectFields)

	row := r.pool.QueryRow(ctx, query, level)
	return scanLevelConfig(row)
}

func (r *LevelRepository) Update(ctx context.Context, c *domain.LevelConfig) error {
	query := `
		UPDATE level_configs
		SET 
			title = $1,
			tier_name = $2,
			required_referrals = $3,
			required_machines = $4,
			required_coletas = $5,
			required_tonnage = $6,
			unlocked_machine_id = $7,
			commission_percent = $8,
			bonus_yield_percent = $9,
			has_insurance_voucher = $10,
			insurance_days_recovered = $11,
			has_extra_roulette_spin = $12,
			has_vip_roulette = $13,
			max_machines_allowed = $14,
			special_reward_text = $15,
			badge_name = $16,
			badge_icon = $17,
			updated_at = $18
		WHERE level = $19
	`
	now := time.Now()
	_, err := r.pool.Exec(ctx, query,
		c.Title,
		c.TierName,
		c.RequiredReferrals,
		c.RequiredMachines,
		c.RequiredColetas,
		c.RequiredTonnage,
		c.UnlockedMachineID,
		c.CommissionPercent,
		c.BonusYieldPercent,
		c.HasInsuranceVoucher,
		c.InsuranceDaysRecovered,
		c.HasExtraRouletteSpin,
		c.HasVIPRoulette,
		c.MaxMachinesAllowed,
		c.SpecialRewardText,
		c.BadgeName,
		c.BadgeIcon,
		now,
		c.Level,
	)
	if err != nil {
		return fmt.Errorf("falha ao atualizar nível %d: %w", c.Level, err)
	}
	c.UpdatedAt = now
	return nil
}
