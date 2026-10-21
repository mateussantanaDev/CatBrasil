package postgres

import (
	"context"
	"fmt"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LogRepository struct {
	pool *pgxpool.Pool
}

func NewLogRepository(pool *pgxpool.Pool) *LogRepository {
	return &LogRepository{pool: pool}
}

func (r *LogRepository) LogUsage(ctx context.Context, logType string, amount float64) error {
	cents, err := money.FromFloat64(amount)
	if err != nil {
		return err
	}
	query := "INSERT INTO usage_logs (type, amount, created_at) VALUES ($1, $2, CURRENT_TIMESTAMP)"
	_, err = r.pool.Exec(ctx, query, logType, cents.String())
	return err
}

func (r *LogRepository) LogSecurity(ctx context.Context, userID *string, event, ip, fingerprint, details string) error {
	query := "INSERT INTO security_logs (user_id, event, ip_address, fingerprint, details, created_at) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)"
	_, err := r.pool.Exec(ctx, query, userID, event, ip, fingerprint, details)
	return err
}

func (r *LogRepository) GetUsageLogs(ctx context.Context, limit, offset int) ([]*domain.UsageLog, error) {
	if limit <= 0 {
		limit = 100
	}
	query := "SELECT id, type, amount, created_at FROM usage_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar logs de uso: %w", err)
	}
	defer rows.Close()

	var logs []*domain.UsageLog
	for rows.Next() {
		var l domain.UsageLog
		if err := rows.Scan(&l.ID, &l.Type, &l.Amount, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, nil
}

func (r *LogRepository) GetSecurityLogs(ctx context.Context, userID *string, limit, offset int) ([]*domain.SecurityLog, error) {
	if limit <= 0 {
		limit = 100
	}

	var query string
	var args []interface{}

	if userID != nil && *userID != "" {
		query = "SELECT id, user_id, event, ip_address, fingerprint, details, created_at FROM security_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = []interface{}{*userID, limit, offset}
	} else {
		query = "SELECT id, user_id, event, ip_address, fingerprint, details, created_at FROM security_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2"
		args = []interface{}{limit, offset}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar logs de auditoria: %w", err)
	}
	defer rows.Close()

	var logs []*domain.SecurityLog
	for rows.Next() {
		var l domain.SecurityLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Event, &l.IPAddress, &l.Fingerprint, &l.Details, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, nil
}
