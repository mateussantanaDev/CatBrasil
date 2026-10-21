package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userSelectFields = `
	id, username, phone, password_hash, security_password, role,
	balance, yield_balance_available, xp, nivel, coletas_completed,
	trator_rented, already_spun_today, promo_unlocked, free_withdraw_active,
	withdraw_locked, referral_bonus_suspended, multi_account_flag,
	required_rollover, current_rollover, ip_address, device_fingerprint,
	two_factor_secret, two_factor_enabled,
	motor_c15_count, esteira_reforcada_count, filtro_oleo_count, turbo_garrett_count,
	insurance_vouchers_count, max_machines_allowed, has_max_level_plaque, season_number, custom_referral_commission_percent,
	last_referral_created_at, level12_reached_at, level12_bonus_suspended,
	created_at, updated_at, version,
	COALESCE(last_spin_date, ''), last_spin_at
`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var roleStr string
	var lastSpinDate string
	var lastSpinAt *time.Time

	err := row.Scan(
		&u.ID, &u.Username, &u.Phone, &u.PasswordHash, &u.SecurityPassword, &roleStr,
		&u.Balance, &u.YieldBalanceAvailable, &u.XP, &u.Nivel, &u.ColetasCompleted,
		&u.TratorRented, &u.AlreadySpunToday, &u.PromoUnlocked, &u.FreeWithdrawActive,
		&u.WithdrawLocked, &u.ReferralBonusSuspended, &u.MultiAccountFlag,
		&u.RequiredRollover, &u.CurrentRollover, &u.IPAddress, &u.DeviceFingerprint,
		&u.TwoFactorSecret, &u.TwoFactorEnabled,
		&u.MotorC15Count, &u.EsteiraReforcadaCount, &u.FiltroOleoCount, &u.TurboGarrettCount,
		&u.InsuranceVouchersCount, &u.MaxMachinesAllowed, &u.HasMaxLevelPlaque, &u.SeasonNumber, &u.CustomReferralCommissionPercent,
		&u.LastReferralCreatedAt, &u.Level12ReachedAt, &u.Level12BonusSuspended,
		&u.CreatedAt, &u.UpdatedAt, &u.Version,
		&lastSpinDate, &lastSpinAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("erro ao mapear operador: %w", err)
	}

	u.LastSpinDate = lastSpinDate
	u.LastSpinAt = lastSpinAt
	if u.LastSpinDate != "" {
		todayRecife := time.Now().In(RecifeLocation).Format("2006-01-02")
		u.AlreadySpunToday = (u.LastSpinDate == todayRecife)
	}

	u.Role = domain.UserRole(roleStr)
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE id = $1", userSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanUser(row)
}

// GetByIDForUpdate é legado: o lock desta consulta termina antes do retorno.
// Mutações financeiras devem usar os métodos que possuem uma transação própria.
func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE id = $1 FOR UPDATE", userSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanUser(row)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE LOWER(username) = LOWER($1)", userSelectFields)
	row := r.pool.QueryRow(ctx, query, username)
	return scanUser(row)
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := fmt.Sprintf("SELECT %s FROM users WHERE phone = $1", userSelectFields)
	row := r.pool.QueryRow(ctx, query, phone)
	return scanUser(row)
}

func (r *UserRepository) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM users 
		WHERE (device_fingerprint = $1 AND device_fingerprint != '') 
		   OR (ip_address = $2 AND ip_address != '')
	`, userSelectFields)

	rows, err := r.pool.Query(ctx, query, fingerprint, ip)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar usuários por fingerprint/ip: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

type userExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	return createUser(ctx, r.pool, u)
}

func createUser(ctx context.Context, execer userExecer, u *domain.User) error {
	wallet, err := walletDecimals(u)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO users (
			id, username, phone, password_hash, security_password, role,
			balance, yield_balance_available, xp, nivel, coletas_completed,
			trator_rented, already_spun_today, promo_unlocked, free_withdraw_active,
			withdraw_locked, referral_bonus_suspended, multi_account_flag,
			required_rollover, current_rollover, ip_address, device_fingerprint,
			two_factor_secret, two_factor_enabled,
			motor_c15_count, esteira_reforcada_count, filtro_oleo_count, turbo_garrett_count,
			insurance_vouchers_count, max_machines_allowed, has_max_level_plaque, season_number, custom_referral_commission_percent,
			last_referral_created_at, level12_reached_at, level12_bonus_suspended,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18,
			$19, $20, $21, $22,
			$23, $24,
			$25, $26, $27, $28,
			$29, $30, $31, $32, $33,
			$34, $35, $36,
			$37, $38
		)
	`

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	u.UpdatedAt = time.Now()

	role := string(u.Role)
	if role == "" {
		role = string(domain.RoleUser)
	}

	maxMachines := u.MaxMachinesAllowed
	if maxMachines <= 0 {
		maxMachines = 15
	}
	seasonNum := u.SeasonNumber
	if seasonNum <= 0 {
		seasonNum = 1
	}
	commissionRate := u.CustomReferralCommissionPercent
	if commissionRate <= 0 {
		commissionRate = 5.00
	}

	_, err = execer.Exec(ctx, query,
		u.ID, u.Username, u.Phone, u.PasswordHash, u.SecurityPassword, role,
		wallet[0], wallet[1], u.XP, u.Nivel, u.ColetasCompleted,
		u.TratorRented, u.AlreadySpunToday, u.PromoUnlocked, u.FreeWithdrawActive,
		u.WithdrawLocked, u.ReferralBonusSuspended, u.MultiAccountFlag,
		wallet[2], wallet[3], u.IPAddress, u.DeviceFingerprint,
		u.TwoFactorSecret, u.TwoFactorEnabled,
		u.MotorC15Count, u.EsteiraReforcadaCount, u.FiltroOleoCount, u.TurboGarrettCount,
		u.InsuranceVouchersCount, maxMachines, u.HasMaxLevelPlaque, seasonNum, commissionRate,
		u.LastReferralCreatedAt, u.Level12ReachedAt, u.Level12BonusSuspended,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("falha ao inserir operador no banco: %w", err)
	}

	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	wallet, err := walletDecimals(u)
	if err != nil {
		return err
	}
	query := `
		UPDATE users SET
			username = $1, phone = $2, password_hash = $3, security_password = $4, role = $5,
			balance = $6, yield_balance_available = $7, xp = $8, nivel = $9, coletas_completed = $10,
			trator_rented = $11, already_spun_today = $12, promo_unlocked = $13, free_withdraw_active = $14,
			withdraw_locked = $15, referral_bonus_suspended = $16, multi_account_flag = $17,
			required_rollover = $18, current_rollover = $19, ip_address = $20, device_fingerprint = $21,
			two_factor_secret = $22, two_factor_enabled = $23,
			motor_c15_count = $24, esteira_reforcada_count = $25, filtro_oleo_count = $26, turbo_garrett_count = $27,
			insurance_vouchers_count = $28, max_machines_allowed = $29, has_max_level_plaque = $30,
			season_number = $31, custom_referral_commission_percent = $32,
			last_referral_created_at = $33, level12_reached_at = $34, level12_bonus_suspended = $35,
			last_spin_date = $36, last_spin_at = $37,
			updated_at = $38
		WHERE id = $39 AND version = $40
		RETURNING version
	`

	u.UpdatedAt = time.Now()

	maxMachines := u.MaxMachinesAllowed
	if maxMachines <= 0 {
		maxMachines = 15
	}
	seasonNum := u.SeasonNumber
	if seasonNum <= 0 {
		seasonNum = 1
	}
	commissionRate := u.CustomReferralCommissionPercent
	if commissionRate <= 0 {
		commissionRate = 5.00
	}

	var lastSpinDate *string
	if u.AlreadySpunToday && u.LastSpinDate != "" {
		lastSpinDate = &u.LastSpinDate
	}
	var lastSpinAt *time.Time
	if u.AlreadySpunToday {
		lastSpinAt = u.LastSpinAt
	}

	err = r.pool.QueryRow(ctx, query,
		u.Username, u.Phone, u.PasswordHash, u.SecurityPassword, string(u.Role),
		wallet[0], wallet[1], u.XP, u.Nivel, u.ColetasCompleted,
		u.TratorRented, u.AlreadySpunToday, u.PromoUnlocked, u.FreeWithdrawActive,
		u.WithdrawLocked, u.ReferralBonusSuspended, u.MultiAccountFlag,
		wallet[2], wallet[3], u.IPAddress, u.DeviceFingerprint,
		u.TwoFactorSecret, u.TwoFactorEnabled,
		u.MotorC15Count, u.EsteiraReforcadaCount, u.FiltroOleoCount, u.TurboGarrettCount,
		u.InsuranceVouchersCount, maxMachines, u.HasMaxLevelPlaque, seasonNum, commissionRate,
		u.LastReferralCreatedAt, u.Level12ReachedAt, u.Level12BonusSuspended,
		lastSpinDate, lastSpinAt,
		u.UpdatedAt, u.ID, u.Version,
	).Scan(&u.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrConcurrentModification
		}
		return fmt.Errorf("falha ao atualizar dados do operador: %w", err)
	}

	return nil
}

func walletDecimals(u *domain.User) ([4]string, error) {
	var result [4]string
	for i, value := range []float64{u.Balance, u.YieldBalanceAvailable, u.RequiredRollover, u.CurrentRollover} {
		cents, err := money.FromFloat64(value)
		if err != nil {
			return result, err
		}
		result[i] = cents.String()
	}
	return result, nil
}

func (r *UserRepository) AdjustBalance(ctx context.Context, id string, deltaBalance, deltaYield float64) (*domain.User, error) {
	balanceCents, err := money.FromFloat64(deltaBalance)
	if err != nil {
		return nil, err
	}
	yieldCents, err := money.FromFloat64(deltaYield)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		UPDATE users SET
			balance = balance + $1,
			yield_balance_available = GREATEST(0, yield_balance_available + $2),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING %s
	`, userSelectFields)

	row := r.pool.QueryRow(ctx, query, balanceCents.String(), yieldCents.String(), id)
	return scanUser(row)
}

func (r *UserRepository) List(ctx context.Context, queryFilter, status string, limit, offset int) ([]*domain.User, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if queryFilter != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(username) LIKE $%d OR LOWER(phone) LIKE $%d OR LOWER(id) LIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(queryFilter)+"%")
		argIdx++
	}

	if status == "bloqueados" {
		conditions = append(conditions, "withdraw_locked = TRUE")
	} else if status == "ativos" {
		conditions = append(conditions, "withdraw_locked = FALSE")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("falha ao contar operadores: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	dataQuery := fmt.Sprintf(`
		SELECT %s FROM users
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, userSelectFields, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao listar operadores: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	return users, total, nil
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *UserRepository) ResetDailySpins(ctx context.Context) error {
	todayRecife := time.Now().In(RecifeLocation).Format("2006-01-02")
	_, err := r.pool.Exec(ctx, `
		UPDATE users 
		SET already_spun_today = FALSE, updated_at = CURRENT_TIMESTAMP 
		WHERE already_spun_today = TRUE 
		  AND (last_spin_date IS NULL OR last_spin_date < $1)
	`, todayRecife)
	return err
}
