package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	LockCronDailyReset  int64 = 8847192001
	LockCronWeeklyClose int64 = 8847192002
)

var ErrLockedByAnotherReplica = errors.New("tarefa bloqueada por outra réplica")

// RewardMachineID gera um ID determinístico, estável e <= 64 caracteres baseado no hash de periodKey e userID
func RewardMachineID(periodKey, userID string) string {
	h := sha256.Sum256([]byte(periodKey + ":" + userID))
	return fmt.Sprintf("mach-reward-%x", h[:24]) // 12 + 48 = 60 caracteres <= 64
}

type CronRepository struct {
	pool *pgxpool.Pool
}

func NewCronRepository(pool *pgxpool.Pool) *CronRepository {
	return &CronRepository{pool: pool}
}

// ResetDailySpinsAtomic executa o reset diário de giros em UMA transação com advisory lock,
// persistência em cron_executions e proteção contra redefinição de usuários que já giraram no dia corrente.
func (r *CronRepository) ResetDailySpinsAtomic(ctx context.Context, periodKey string, spinDate string) (alreadyDone bool, affected int64, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrLockedByAnotherReplica) {
			_ = r.RecordFailure(ctx, "daily_roulette_reset", periodKey, err)
		}
	}()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("iniciar transação de reset diário: %w", err)
	}
	defer tx.Rollback(ctx)

	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", LockCronDailyReset).Scan(&locked); err != nil {
		return false, 0, fmt.Errorf("adquirir lock consultivo diário: %w", err)
	}
	if !locked {
		return false, 0, ErrLockedByAnotherReplica
	}

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status FROM cron_executions
		WHERE job_type = 'daily_roulette_reset' AND period_key = $1
		FOR UPDATE
	`, periodKey).Scan(&status)

	if err == nil && status == "completed" {
		return true, 0, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, 0, fmt.Errorf("consultar cron_executions diário: %w", err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `
			INSERT INTO cron_executions (job_type, period_key, status, started_at, attempt_count)
			VALUES ('daily_roulette_reset', $1, 'running', CURRENT_TIMESTAMP, 1)
		`, periodKey)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE cron_executions
			SET status = 'running', attempt_count = attempt_count + 1, started_at = CURRENT_TIMESTAMP, last_error = ''
			WHERE job_type = 'daily_roulette_reset' AND period_key = $1
		`, periodKey)
	}
	if err != nil {
		return false, 0, fmt.Errorf("registrar status running diário: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET already_spun_today = FALSE, updated_at = CURRENT_TIMESTAMP
		WHERE already_spun_today = TRUE
		  AND (last_spin_date IS NULL OR last_spin_date < $1)
	`, spinDate)
	if err != nil {
		return false, 0, fmt.Errorf("executar reset diário de usuários: %w", err)
	}

	affected = tag.RowsAffected()

	_, err = tx.Exec(ctx, `
		UPDATE cron_executions
		SET status = 'completed', finished_at = CURRENT_TIMESTAMP, metadata = json_build_object('users_reset', $2::int)
		WHERE job_type = 'daily_roulette_reset' AND period_key = $1
	`, periodKey, affected)
	if err != nil {
		return false, 0, fmt.Errorf("concluir cron_executions diário: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("commit transação reset diário: %w", err)
	}

	return false, affected, nil
}

// CloseWeeklySeasonAtomic fecha a semana em UMA transação atômica: adquire lock consultivo,
// consulta o ranking da semana encerrada (isolado por period_start), injeta máquina prêmio com ID único,
// registra conclusão em cron_executions e mantém intactas novas coletas da semana seguinte.
func (r *CronRepository) CloseWeeklySeasonAtomic(
	ctx context.Context,
	periodKey string,
	periodStart time.Time,
	periodEnd time.Time,
) (res *domain.WeeklyClosureResult, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrLockedByAnotherReplica) {
			_ = r.RecordFailure(ctx, "weekly_season_close", periodKey, err)
		}
	}()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iniciar transação de fechamento semanal: %w", err)
	}
	defer tx.Rollback(ctx)

	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", LockCronWeeklyClose).Scan(&locked); err != nil {
		return nil, fmt.Errorf("adquirir lock consultivo semanal: %w", err)
	}
	if !locked {
		return nil, ErrLockedByAnotherReplica
	}

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status FROM cron_executions
		WHERE job_type = 'weekly_season_close' AND period_key = $1
		FOR UPDATE
	`, periodKey).Scan(&status)

	if err == nil && status == "completed" {
		return &domain.WeeklyClosureResult{
			PeriodKey:     periodKey,
			PeriodStart:   periodStart,
			PeriodEnd:     periodEnd,
			AlreadyClosed: true,
		}, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("consultar cron_executions semanal: %w", err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `
			INSERT INTO cron_executions (job_type, period_key, status, started_at, attempt_count)
			VALUES ('weekly_season_close', $1, 'running', CURRENT_TIMESTAMP, 1)
		`, periodKey)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE cron_executions
			SET status = 'running', attempt_count = attempt_count + 1, started_at = CURRENT_TIMESTAMP, last_error = ''
			WHERE job_type = 'weekly_season_close' AND period_key = $1
		`, periodKey)
	}
	if err != nil {
		return nil, fmt.Errorf("registrar status running semanal: %w", err)
	}

	// 1. Obter Top 10 da semana encerrada (isolado por period_start)
	rows, err := tx.Query(ctx, `
		SELECT user_id, username, weekly_tonnage, updated_at
		FROM mining_race_weekly
		WHERE period_start = $1
		ORDER BY weekly_tonnage DESC, user_id ASC
		LIMIT 10
	`, periodStart)
	if err != nil {
		return nil, fmt.Errorf("consultar ranking da semana encerrada: %w", err)
	}

	var leaderboard []*domain.LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry domain.LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Username, &entry.WeeklyTonnage, &entry.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		entry.Rank = rank
		if rank-1 < len(defaultLeaderboardPrizes) {
			entry.Prize = defaultLeaderboardPrizes[rank-1].Prize
			entry.Badge = defaultLeaderboardPrizes[rank-1].Badge
		}
		leaderboard = append(leaderboard, &entry)
		rank++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("erro durante iteração do ranking: %w", err)
	}
	rows.Close()

	var topWinner *domain.LeaderboardEntry
	var rewardMachine *domain.Machine

	// 2. Injeta máquina prêmio com ID único derivado do período caso haja vencedor com pontuação
	if len(leaderboard) > 0 && leaderboard[0].UserID != "" && leaderboard[0].WeeklyTonnage > 0 {
		topWinner = leaderboard[0]
		machineID := RewardMachineID(periodKey, topWinner.UserID)

		now := time.Now()
		rewardMachine = &domain.Machine{
			ID:              machineID,
			UserID:          topWinner.UserID,
			Name:            "Escavadeira CAT 320 TURBO (Prêmio TOP 1) 👑",
			Cost:            0.00,
			YieldAmount:     150.00,
			LastCollectedAt: now,
			State:           domain.MachineStateAguardando,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO machines (id, user_id, name, cost, yield_amount, last_collected_at, state, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
		`, rewardMachine.ID, rewardMachine.UserID, rewardMachine.Name, rewardMachine.Cost, rewardMachine.YieldAmount, rewardMachine.LastCollectedAt, rewardMachine.State, rewardMachine.CreatedAt, rewardMachine.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("inserir máquina prêmio com ID periódico: %w", err)
		}
	}

	// 3. Registrar sucesso em cron_executions na MESMA transação
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"top_winner_user_id": func() string {
			if topWinner != nil {
				return topWinner.UserID
			}
			return ""
		}(),
		"top_winner_tonnage": func() float64 {
			if topWinner != nil {
				return topWinner.WeeklyTonnage
			}
			return 0
		}(),
		"leaderboard_count": len(leaderboard),
		"reward_machine_id": func() string {
			if rewardMachine != nil {
				return rewardMachine.ID
			}
			return ""
		}(),
	})

	_, err = tx.Exec(ctx, `
		UPDATE cron_executions
		SET status = 'completed', finished_at = CURRENT_TIMESTAMP, metadata = $2
		WHERE job_type = 'weekly_season_close' AND period_key = $1
	`, periodKey, metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("concluir cron_executions semanal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transação fechamento semanal: %w", err)
	}

	return &domain.WeeklyClosureResult{
		PeriodKey:     periodKey,
		PeriodStart:   periodStart,
		PeriodEnd:     periodEnd,
		TopWinner:     topWinner,
		RewardMachine: rewardMachine,
		AlreadyClosed: false,
		Leaderboard:   leaderboard,
	}, nil
}

// GetExecution busca a execução persistida no banco
func (r *CronRepository) GetExecution(ctx context.Context, jobType, periodKey string) (*domain.CronExecution, error) {
	var exec domain.CronExecution
	var metaBytes []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, job_type, period_key, status, started_at, finished_at, attempt_count, last_error, metadata
		FROM cron_executions
		WHERE job_type = $1 AND period_key = $2
	`, jobType, periodKey).Scan(
		&exec.ID, &exec.JobType, &exec.PeriodKey, &exec.Status,
		&exec.StartedAt, &exec.FinishedAt, &exec.AttemptCount,
		&exec.LastError, &metaBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &exec.Metadata)
	}
	return &exec, nil
}

// ClassifyErrorCategory converte qualquer erro em uma categoria estática segura,
// garantindo que strings arbitrárias, DSNs ou PII jamais sejam gravadas no banco ou logs.
func ClassifyErrorCategory(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, ErrLockedByAnotherReplica) {
		return "db_locked_by_replica"
	}
	sanitized := SanitizeDBError(err)
	if sanitized != nil {
		switch {
		case errors.Is(sanitized, ErrDBPingFailed),
			errors.Is(sanitized, ErrDBPoolInitFailed),
			errors.Is(sanitized, ErrDBConnection):
			return "db_connection_failed"
		case errors.Is(sanitized, ErrDBSchemaCheck),
			errors.Is(sanitized, ErrDBMigration):
			return "db_schema_error"
		default:
			return "db_error"
		}
	}
	return "db_error"
}

// RecordFailure registra o status 'failed' com attempt_count e categoria fixa em last_error em transação separada
// para garantir auditabilidade durável sem jamás gravar DSNs ou strings brutas de banco.
// Não marca falha quando failureErr for ErrLockedByAnotherReplica.
func (r *CronRepository) RecordFailure(ctx context.Context, jobType, periodKey string, failureErr error) error {
	if failureErr == nil || errors.Is(failureErr, ErrLockedByAnotherReplica) {
		return nil
	}

	recordCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		recordCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Persiste EXCLUSIVAMENTE a categoria fixa estática, prevenindo vazamento de DSN/credenciais em cron_executions.last_error
	errMsg := ClassifyErrorCategory(failureErr)
	query := `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ($1, $2, 'failed', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, $3)
		ON CONFLICT (job_type, period_key) DO UPDATE
		SET status = 'failed',
		    attempt_count = cron_executions.attempt_count + 1,
		    last_error = EXCLUDED.last_error,
		    finished_at = CURRENT_TIMESTAMP
		WHERE cron_executions.status <> 'completed'
	`
	_, err := r.pool.Exec(recordCtx, query, jobType, periodKey, errMsg)
	if err != nil {
		sanitized := SanitizeDBError(err)
		log.Printf("⚠️ [CRON AUDIT] Falha ao persistir status failed em cron_executions (%s): %s\n", jobType, sanitized.Error())
		return sanitized
	}
	return nil
}

// CronObservabilityMetric contém métricas agregadas por job_type para observabilidade
type CronObservabilityMetric struct {
	JobType              string
	LastSuccessTimestamp int64
	RecentFailures       int64
}

// CronObservabilityData encapsula todos os dados do cron para o endpoint de métricas
type CronObservabilityData struct {
	Jobs                 []CronObservabilityMetric
	PendingWeeklyPeriods int64
	PendingDailyPeriods  int64
}

// GetCronObservabilityMetrics consulta cron_executions sem expor IDs, period_key ou mensagens brutas de erro
func (r *CronRepository) GetCronObservabilityMetrics(ctx context.Context) (*CronObservabilityData, error) {
	query := `
		SELECT job_type,
		       COALESCE(EXTRACT(EPOCH FROM MAX(CASE WHEN status = 'completed' THEN finished_at END)), 0)::bigint AS last_success,
		       COUNT(CASE WHEN status = 'failed' AND started_at >= NOW() - INTERVAL '24 hours' THEN 1 END) AS recent_failures
		FROM cron_executions
		WHERE job_type IN ('daily_roulette_reset', 'weekly_season_close')
		GROUP BY job_type
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, SanitizeDBError(err)
	}
	defer rows.Close()

	results := make([]CronObservabilityMetric, 0, 2)
	for rows.Next() {
		var m CronObservabilityMetric
		if err := rows.Scan(&m.JobType, &m.LastSuccessTimestamp, &m.RecentFailures); err != nil {
			return nil, SanitizeDBError(err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, SanitizeDBError(err)
	}

	// Consulta semanas pendentes em mining_race_weekly strictly anteriores à semana corrente em America/Recife
	// Em banco novo ou sem temporadas vencidas aguardando fechamento, retorna 0 (prevenindo alertas falsos).
	var pendingCount int64
	queryPending := `
		SELECT COUNT(DISTINCT period_start)
		FROM mining_race_weekly
		WHERE period_start < (date_trunc('week', CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife') AT TIME ZONE 'America/Recife')
		  AND NOT EXISTS (
		      SELECT 1 FROM cron_executions
		      WHERE job_type = 'weekly_season_close'
		        AND period_key = to_char(period_start AT TIME ZONE 'America/Recife', 'IYYY-"W"IW')
		        AND status = 'completed'
		  )
	`
	if err := r.pool.QueryRow(ctx, queryPending).Scan(&pendingCount); err != nil {
		return nil, SanitizeDBError(err)
	}

	// Consulta períodos diários pendentes com consciência de primeiro prazo agendado:
	// Em banco novo (antes do primeiro prazo agendado: 00:30 do dia seguinte à criação do banco), retorna 0.
	// Retorna 1 se há usuários com giro anterior não resetado ou se o reset de hoje está atrasado.
	var pendingDaily int64
	queryPendingDaily := `
		WITH config AS (
			SELECT COALESCE(MIN(created_at), CURRENT_TIMESTAMP) AS db_init
			FROM system_config
		),
		today_exec AS (
			SELECT 1 FROM cron_executions
			WHERE job_type = 'daily_roulette_reset'
			  AND period_key = to_char(CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife', 'YYYY-MM-DD')
			  AND status = 'completed'
		),
		past_exec AS (
			SELECT 1 FROM cron_executions
			WHERE job_type = 'daily_roulette_reset'
			  AND status = 'completed'
			LIMIT 1
		)
		SELECT CASE
			WHEN EXISTS (SELECT 1 FROM today_exec) THEN 0
			WHEN EXISTS (
				SELECT 1 FROM users
				WHERE already_spun_today = TRUE
				  AND (last_spin_date IS NOT NULL AND last_spin_date < to_char(CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife', 'YYYY-MM-DD'))
			) THEN 1
			WHEN EXISTS (SELECT 1 FROM past_exec)
			     AND CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife' > date_trunc('day', CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife') + INTERVAL '30 minutes'
			THEN 1
			WHEN CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife' > (
				SELECT date_trunc('day', db_init AT TIME ZONE 'America/Recife') + INTERVAL '1 day' + INTERVAL '30 minutes' FROM config
			) THEN 1
			ELSE 0
		END;
	`
	if err := r.pool.QueryRow(ctx, queryPendingDaily).Scan(&pendingDaily); err != nil {
		return nil, SanitizeDBError(err)
	}

	return &CronObservabilityData{
		Jobs:                 results,
		PendingWeeklyPeriods: pendingCount,
		PendingDailyPeriods:  pendingDaily,
	}, nil
}


// GetPendingWeeklyClosures consulta períodos semanais pendentes de fechamento strictly anteriores a currentWeekStart
func (r *CronRepository) GetPendingWeeklyClosures(ctx context.Context, currentWeekStart time.Time, limit int) ([]time.Time, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT DISTINCT period_start
		FROM mining_race_weekly
		WHERE period_start < $1
		  AND NOT EXISTS (
		      SELECT 1 FROM cron_executions
		      WHERE job_type = 'weekly_season_close'
		        AND period_key = to_char(period_start AT TIME ZONE 'America/Recife', 'IYYY-"W"IW')
		        AND status = 'completed'
		  )
		ORDER BY period_start ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, currentWeekStart, limit)
	if err != nil {
		return nil, fmt.Errorf("consultar semanas pendentes no banco: %w", err)
	}
	defer rows.Close()

	var pending []time.Time
	for rows.Next() {
		var p time.Time
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Garante que a semana imediatamente anterior seja verificada mesmo se a tabela estava vazia
	lastWeekStart := currentWeekStart.AddDate(0, 0, -7)
	hasLastWeek := false
	for _, p := range pending {
		if p.Equal(lastWeekStart) {
			hasLastWeek = true
			break
		}
	}
	if !hasLastWeek {
		var closed bool
		lastYear, lastWeek := lastWeekStart.ISOWeek()
		lastWeekKey := fmt.Sprintf("%04d-W%02d", lastYear, lastWeek)
		err := r.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM cron_executions
				WHERE job_type = 'weekly_season_close' AND period_key = $1 AND status = 'completed'
			)
		`, lastWeekKey).Scan(&closed)
		if err == nil && !closed {
			pending = append(pending, lastWeekStart)
		}
	}

	return pending, nil
}
