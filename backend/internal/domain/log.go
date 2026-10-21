package domain

import (
	"context"
	"time"
)

// UsageLog registra eventos de coletas, expirações de rendimento e giros de roleta
type UsageLog struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"createdAt"`
}

// SecurityLog registra eventos de segurança, login, multi-contas e alterações administrativas
type SecurityLog struct {
	ID          int64     `json:"id"`
	UserID      *string   `json:"userId,omitempty"`
	Event       string    `json:"event"`
	IPAddress   string    `json:"ipAddress"`
	Fingerprint string    `json:"fingerprint"`
	Details     string    `json:"details"`
	CreatedAt   time.Time `json:"createdAt"`
}

// LogRepository define o contrato da camada de persistência para Logs e Auditoria
type LogRepository interface {
	LogUsage(ctx context.Context, logType string, amount float64) error
	LogSecurity(ctx context.Context, userID *string, event, ip, fingerprint, details string) error
	GetUsageLogs(ctx context.Context, limit, offset int) ([]*UsageLog, error)
	GetSecurityLogs(ctx context.Context, userID *string, limit, offset int) ([]*SecurityLog, error)
}
