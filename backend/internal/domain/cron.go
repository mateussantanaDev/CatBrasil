package domain

import (
	"context"
	"time"
)

// CronExecution representa o registro auditável e durável de execução de uma tarefa agendada
type CronExecution struct {
	ID           int64                  `json:"id"`
	JobType      string                 `json:"jobType"`
	PeriodKey    string                 `json:"periodKey"`
	Status       string                 `json:"status"` // "running", "completed", "failed"
	StartedAt    time.Time              `json:"startedAt"`
	FinishedAt   *time.Time             `json:"finishedAt,omitempty"`
	AttemptCount int                    `json:"attemptCount"`
	LastError    string                 `json:"lastError,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// WeeklyClosureResult encapsula o resultado do fechamento atômico semanal da Corrida de Mineração
type WeeklyClosureResult struct {
	PeriodKey     string              `json:"periodKey"`
	PeriodStart   time.Time           `json:"periodStart"`
	PeriodEnd     time.Time           `json:"periodEnd"`
	TopWinner     *LeaderboardEntry   `json:"topWinner,omitempty"`
	RewardMachine *Machine            `json:"rewardMachine,omitempty"`
	AlreadyClosed bool                `json:"alreadyClosed"`
	Leaderboard   []*LeaderboardEntry `json:"leaderboard"`
}

// CronRepository define o contrato da camada de persistência para jobs por período
type CronRepository interface {
	ResetDailySpinsAtomic(ctx context.Context, periodKey string, spinDate string) (alreadyDone bool, affected int64, err error)
	CloseWeeklySeasonAtomic(ctx context.Context, periodKey string, periodStart time.Time, periodEnd time.Time) (*WeeklyClosureResult, error)
	GetPendingWeeklyClosures(ctx context.Context, currentWeekStart time.Time, limit int) ([]time.Time, error)
	GetExecution(ctx context.Context, jobType, periodKey string) (*CronExecution, error)
	RecordFailure(ctx context.Context, jobType, periodKey string, failureErr error) error
}
