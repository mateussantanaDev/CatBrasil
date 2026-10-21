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

type ConfigRepository struct {
	pool *pgxpool.Pool
}

func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{pool: pool}
}

const configSelectFields = `
	id, global_withdrawals_locked, auto_approve_enabled, auto_approve_withdrawals_under,
	referral_commission_percent, required_referrals_count, require_referrals_for_withdrawal,
	anti_self_referral_lock, bonus_per_mutirao_amount, collection_grace_minutes,
	gateway_name, deposit_fee_percent, withdraw_fee_fixed, pix_expiration_minutes,
	COALESCE(gateway_provider, 'simulated'),
	COALESCE(suitpay_client_id, ''), COALESCE(suitpay_client_secret, ''),
	COALESCE(suitpay_sandbox, false), COALESCE(suitpay_callback_url, ''), COALESCE(suitpay_enabled, false),
	COALESCE(paradise_secret_key, ''), COALESCE(paradise_product_hash, ''),
	COALESCE(paradise_postback_url, ''), COALESCE(paradise_enabled, false),
	strict_modal_mode, require_deposit_to_spin,
	COALESCE(season_duration_days, 75), COALESCE(current_season_number, 1), COALESCE(season_started_at, CURRENT_TIMESTAMP),
	COALESCE(level12_maintenance_days, 30), COALESCE(level12_required_referrals, 1),
	created_at, updated_at
`

func scanConfig(row pgx.Row) (*domain.SystemConfig, error) {
	var cfg domain.SystemConfig
	err := row.Scan(
		&cfg.ID, &cfg.GlobalWithdrawalsLocked, &cfg.AutoApproveEnabled, &cfg.AutoApproveWithdrawalsUnder,
		&cfg.ReferralCommissionPercent, &cfg.RequiredReferralsCount, &cfg.RequireReferralsForWithdrawal,
		&cfg.AntiSelfReferralLock, &cfg.BonusPerMutiraoAmount, &cfg.CollectionGraceMinutes,
		&cfg.Gateway.Name, &cfg.Gateway.DepositFeePercent, &cfg.Gateway.WithdrawFeeFixed, &cfg.Gateway.PixExpirationMinutes,
		&cfg.Gateway.ActiveProvider,
		&cfg.Gateway.SuitPayClientID, &cfg.Gateway.SuitPayClientSecret,
		&cfg.Gateway.SuitPaySandbox, &cfg.Gateway.SuitPayCallbackURL, &cfg.Gateway.SuitPayEnabled,
		&cfg.Gateway.ParadiseSecretKey, &cfg.Gateway.ParadiseProductHash,
		&cfg.Gateway.ParadisePostbackURL, &cfg.Gateway.ParadiseEnabled,
		&cfg.StrictModalMode, &cfg.RequireDepositToSpin,
		&cfg.SeasonDurationDays, &cfg.CurrentSeasonNumber, &cfg.SeasonStartedAt,
		&cfg.Level12MaintenanceDays, &cfg.Level12RequiredReferrals,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("configuração global do sistema não encontrada")
		}
		return nil, fmt.Errorf("erro ao mapear configuração do sistema: %w", err)
	}
	return &cfg, nil
}

func (r *ConfigRepository) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	query := fmt.Sprintf("SELECT %s FROM system_config WHERE id = 1", configSelectFields)
	row := r.pool.QueryRow(ctx, query)
	return scanConfig(row)
}

func (r *ConfigRepository) UpdateConfig(ctx context.Context, cfg *domain.SystemConfig) error {
	approve, err := money.FromFloat64(cfg.AutoApproveWithdrawalsUnder)
	if err != nil {
		return err
	}
	bonus, err := money.FromFloat64(cfg.BonusPerMutiraoAmount)
	if err != nil {
		return err
	}
	fee, err := money.FromFloat64(cfg.Gateway.WithdrawFeeFixed)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO system_config (
			id, global_withdrawals_locked, auto_approve_enabled, auto_approve_withdrawals_under,
			referral_commission_percent, required_referrals_count, require_referrals_for_withdrawal,
			anti_self_referral_lock, bonus_per_mutirao_amount, collection_grace_minutes,
			gateway_name, deposit_fee_percent, withdraw_fee_fixed, pix_expiration_minutes, gateway_provider,
			suitpay_client_id, suitpay_client_secret, suitpay_sandbox, suitpay_callback_url, suitpay_enabled,
			paradise_secret_key, paradise_product_hash, paradise_postback_url, paradise_enabled,
			strict_modal_mode, require_deposit_to_spin, created_at, updated_at
		) VALUES (
			1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27
		)
		ON CONFLICT (id) DO UPDATE SET
			global_withdrawals_locked = EXCLUDED.global_withdrawals_locked,
			auto_approve_enabled = EXCLUDED.auto_approve_enabled,
			auto_approve_withdrawals_under = EXCLUDED.auto_approve_withdrawals_under,
			referral_commission_percent = EXCLUDED.referral_commission_percent,
			required_referrals_count = EXCLUDED.required_referrals_count,
			require_referrals_for_withdrawal = EXCLUDED.require_referrals_for_withdrawal,
			anti_self_referral_lock = EXCLUDED.anti_self_referral_lock,
			bonus_per_mutirao_amount = EXCLUDED.bonus_per_mutirao_amount,
			collection_grace_minutes = EXCLUDED.collection_grace_minutes,
			gateway_name = EXCLUDED.gateway_name,
			deposit_fee_percent = EXCLUDED.deposit_fee_percent,
			withdraw_fee_fixed = EXCLUDED.withdraw_fee_fixed,
			pix_expiration_minutes = EXCLUDED.pix_expiration_minutes,
			gateway_provider = EXCLUDED.gateway_provider,
			suitpay_client_id = COALESCE(NULLIF(EXCLUDED.suitpay_client_id, ''), system_config.suitpay_client_id),
			suitpay_client_secret = COALESCE(NULLIF(EXCLUDED.suitpay_client_secret, ''), system_config.suitpay_client_secret),
			suitpay_sandbox = EXCLUDED.suitpay_sandbox,
			suitpay_callback_url = COALESCE(NULLIF(EXCLUDED.suitpay_callback_url, ''), system_config.suitpay_callback_url),
			suitpay_enabled = EXCLUDED.suitpay_enabled,
			paradise_secret_key = COALESCE(NULLIF(EXCLUDED.paradise_secret_key, ''), system_config.paradise_secret_key),
			paradise_product_hash = COALESCE(NULLIF(EXCLUDED.paradise_product_hash, ''), system_config.paradise_product_hash),
			paradise_postback_url = COALESCE(NULLIF(EXCLUDED.paradise_postback_url, ''), system_config.paradise_postback_url),
			paradise_enabled = EXCLUDED.paradise_enabled,
			strict_modal_mode = EXCLUDED.strict_modal_mode,
			require_deposit_to_spin = EXCLUDED.require_deposit_to_spin,
			updated_at = CURRENT_TIMESTAMP
	`

	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now()
	}
	cfg.UpdatedAt = time.Now()

	provider := cfg.Gateway.ActiveProvider
	if provider == "" {
		provider = "simulated"
	}

	_, err = r.pool.Exec(ctx, query,
		cfg.GlobalWithdrawalsLocked, cfg.AutoApproveEnabled, approve.String(),
		cfg.ReferralCommissionPercent, cfg.RequiredReferralsCount, cfg.RequireReferralsForWithdrawal,
		cfg.AntiSelfReferralLock, bonus.String(), cfg.CollectionGraceMinutes,
		cfg.Gateway.Name, cfg.Gateway.DepositFeePercent, fee.String(), cfg.Gateway.PixExpirationMinutes, provider,
		cfg.Gateway.SuitPayClientID, cfg.Gateway.SuitPayClientSecret,
		cfg.Gateway.SuitPaySandbox, cfg.Gateway.SuitPayCallbackURL, cfg.Gateway.SuitPayEnabled,
		cfg.Gateway.ParadiseSecretKey, cfg.Gateway.ParadiseProductHash,
		cfg.Gateway.ParadisePostbackURL, cfg.Gateway.ParadiseEnabled,
		cfg.StrictModalMode, cfg.RequireDepositToSpin, cfg.CreatedAt, cfg.UpdatedAt,
	)
	return err
}

func (r *ConfigRepository) ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error {
	query := "UPDATE system_config SET global_withdrawals_locked = $1, updated_at = CURRENT_TIMESTAMP WHERE id = 1"
	_, err := r.pool.Exec(ctx, query, locked)
	return err
}

func (r *ConfigRepository) UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error {
	value, err := money.FromFloat64(under)
	if err != nil {
		return err
	}
	query := "UPDATE system_config SET auto_approve_enabled = $1, auto_approve_withdrawals_under = $2, updated_at = CURRENT_TIMESTAMP WHERE id = 1"
	_, err = r.pool.Exec(ctx, query, enabled, value.String())
	return err
}

func (r *ConfigRepository) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, graceMinutes int) error {
	fee, err := money.FromFloat64(gw.WithdrawFeeFixed)
	if err != nil {
		return err
	}
	provider := gw.ActiveProvider
	if provider == "" {
		provider = "simulated"
	}

	query := `
		UPDATE system_config SET
			gateway_name = $1,
			deposit_fee_percent = $2,
			withdraw_fee_fixed = $3,
			pix_expiration_minutes = $4,
			collection_grace_minutes = $5,
			gateway_provider = $6,
			suitpay_client_id = COALESCE(NULLIF($7, ''), suitpay_client_id),
			suitpay_client_secret = COALESCE(NULLIF($8, ''), suitpay_client_secret),
			suitpay_sandbox = $9,
			suitpay_callback_url = COALESCE(NULLIF($10, ''), suitpay_callback_url),
			suitpay_enabled = $11,
			paradise_secret_key = COALESCE(NULLIF($12, ''), paradise_secret_key),
			paradise_product_hash = COALESCE(NULLIF($13, ''), paradise_product_hash),
			paradise_postback_url = COALESCE(NULLIF($14, ''), paradise_postback_url),
			paradise_enabled = $15,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	_, err = r.pool.Exec(ctx, query,
		gw.Name, gw.DepositFeePercent, fee.String(), gw.PixExpirationMinutes, graceMinutes,
		provider,
		gw.SuitPayClientID, gw.SuitPayClientSecret, gw.SuitPaySandbox, gw.SuitPayCallbackURL, gw.SuitPayEnabled,
		gw.ParadiseSecretKey, gw.ParadiseProductHash, gw.ParadisePostbackURL, gw.ParadiseEnabled,
	)
	return err
}

func (r *ConfigRepository) UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error {
	bonus, err := money.FromFloat64(bonusMutirao)
	if err != nil {
		return err
	}
	query := `
		UPDATE system_config SET
			referral_commission_percent = $1,
			required_referrals_count = $2,
			bonus_per_mutirao_amount = $3,
			require_referrals_for_withdrawal = $4,
			anti_self_referral_lock = $5,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	_, err = r.pool.Exec(ctx, query, commPercent, reqCount, bonus.String(), reqForWith, antiSelf)
	return err
}

func (r *ConfigRepository) ResetSeason(ctx context.Context, durationDays int) error {
	if durationDays <= 0 {
		durationDays = 75
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação para reset da temporada: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Arquivar Pódio e Histórico da Temporada de Ranking encerrada
	archiveQuery := `
		INSERT INTO ranking_season_history (
			season_number, user_id, username, rank_position, tonnage_extracted, reward_granted, closed_at
		)
		SELECT 
			sc.current_season_number, u.id, u.username, 
			ROW_NUMBER() OVER (ORDER BY m.weekly_tonnage DESC)::int,
			COALESCE(m.weekly_tonnage, 0),
			CASE 
				WHEN ROW_NUMBER() OVER (ORDER BY m.weekly_tonnage DESC) = 1 THEN 'Troféu Ouro + R$ 1.500 PIX'
				WHEN ROW_NUMBER() OVER (ORDER BY m.weekly_tonnage DESC) = 2 THEN 'Troféu Prata + R$ 800 PIX'
				WHEN ROW_NUMBER() OVER (ORDER BY m.weekly_tonnage DESC) = 3 THEN 'Troféu Bronze + R$ 400 PIX'
				ELSE 'Certificado de Participação'
			END,
			CURRENT_TIMESTAMP
		FROM mining_race_weekly m
		JOIN users u ON m.user_id = u.id
		CROSS JOIN (SELECT current_season_number FROM system_config WHERE id = 1) sc
		ORDER BY m.weekly_tonnage DESC
	`
	_, _ = tx.Exec(ctx, archiveQuery)

	// 2. Reiniciar exclusivamente o ranking de mineração (os níveis de carreira dos usuários são perpétuos)
	if _, err := tx.Exec(ctx, "TRUNCATE TABLE mining_race_weekly"); err != nil {
		return fmt.Errorf("falha ao reiniciar ranking da temporada: %w", err)
	}

	// 3. Incrementar número da temporada de ranking e definir novo ciclo de 75 dias
	updateConfigQuery := `
		UPDATE system_config SET
			current_season_number = current_season_number + 1,
			season_duration_days = $1,
			season_started_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	if _, err := tx.Exec(ctx, updateConfigQuery, durationDays); err != nil {
		return fmt.Errorf("falha ao avançar temporada no system_config: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *ConfigRepository) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	if maintenanceDays <= 0 {
		maintenanceDays = 30
	}
	if requiredReferrals <= 0 {
		requiredReferrals = 1
	}
	query := `
		UPDATE system_config SET
			level12_maintenance_days = $1,
			level12_required_referrals = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	_, err := r.pool.Exec(ctx, query, maintenanceDays, requiredReferrals)
	return err
}
