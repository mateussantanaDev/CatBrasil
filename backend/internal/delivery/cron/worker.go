package cron

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
)

var RecifeLocation *time.Location

func init() {
	var err error
	RecifeLocation, err = time.LoadLocation("America/Recife")
	if err != nil {
		RecifeLocation = time.FixedZone("America/Recife", -3*3600)
	}
}

// Categorias fixas para observabilidade sem vazamento de PII ou DSN
const (
	CategoryNone         = "none"
	CategoryTimeout      = "timeout"
	CategoryDBLocked     = "db_locked_by_replica"
	CategoryDBConnection = "db_connection_failed"
	CategoryDBGeneric    = "db_error"
)

// ClassifyError mapeia erros para categorias fixas conhecidas sem vazar mensagens brutas de banco de dados
func ClassifyError(err error) string {
	if err == nil {
		return CategoryNone
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CategoryTimeout
	}
	if errors.Is(err, postgres.ErrLockedByAnotherReplica) {
		return CategoryDBLocked
	}
	sanitized := postgres.SanitizeDBError(err)
	if sanitized != nil {
		switch {
		case errors.Is(sanitized, postgres.ErrDBPingFailed),
			errors.Is(sanitized, postgres.ErrDBPoolInitFailed),
			errors.Is(sanitized, postgres.ErrDBConnection):
			return CategoryDBConnection
		default:
			return CategoryDBGeneric
		}
	}
	return CategoryDBGeneric
}

// CurrentWeekPeriodStart calcula a segunda-feira 00:00:00 da semana em America/Recife
func CurrentWeekPeriodStart(t time.Time) time.Time {
	tRecife := t.In(RecifeLocation)
	weekday := int(tRecife.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysSinceMonday := weekday - 1
	return time.Date(tRecife.Year(), tRecife.Month(), tRecife.Day()-daysSinceMonday, 0, 0, 0, 0, RecifeLocation)
}

type CronWorker struct {
	rouletteUC            *roulette.RouletteUseCase
	rankingUC             *ranking.RankingUseCase
	rankingRepo           domain.RankingRepository
	machineRepo           domain.MachineRepository
	logRepo               domain.LogRepository
	hub                   *ws.Hub
	cronRepo              domain.CronRepository
	checkInterval         time.Duration
	completedDailyPeriod  string
	completedWeeklyPeriod string
	logger                *slog.Logger
}

func NewCronWorker(
	rouletteUC *roulette.RouletteUseCase,
	rankingUC *ranking.RankingUseCase,
	rankingRepo domain.RankingRepository,
	machineRepo domain.MachineRepository,
	logRepo domain.LogRepository,
	hub *ws.Hub,
	cronRepo ...domain.CronRepository,
) *CronWorker {
	var cr domain.CronRepository
	if len(cronRepo) > 0 {
		cr = cronRepo[0]
	}

	return &CronWorker{
		rouletteUC:    rouletteUC,
		rankingUC:     rankingUC,
		rankingRepo:   rankingRepo,
		machineRepo:   machineRepo,
		logRepo:       logRepo,
		hub:           hub,
		cronRepo:      cr,
		checkInterval: 30 * time.Second,
		logger:        slog.Default(),
	}
}

// SetLogger configura um logger estruturado (útil para auditoria, testes e observabilidade)
func (w *CronWorker) SetLogger(l *slog.Logger) {
	if l != nil {
		w.logger = l
	}
}

// SetCheckInterval configura a frequência de checagem do loop
func (w *CronWorker) SetCheckInterval(d time.Duration) {
	if d > 0 {
		w.checkInterval = d
	}
}

// Start inicia o loop com catch-up imediato ao inicializar e checagens periódicas.
// Falha rápido se cronRepo não estiver configurado.
func (w *CronWorker) Start(ctx context.Context) {
	if w.cronRepo == nil {
		w.logger.Error("cron_worker_fatal", slog.String("reason", "cron_repo_missing"))
		log.Fatal("❌ [CRON WORKER] cronRepo é obrigatório; fallback em memória desabilitado em produção")
	}

	w.logger.Info("cron_worker_started", slog.String("timezone", "America/Recife"))

	// Catch-up imediato ao iniciar sem aguardar o primeiro tick
	w.CheckJobs(ctx, time.Now().In(RecifeLocation))

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("cron_worker_stopped")
			return

		case now := <-ticker.C:
			w.CheckJobs(ctx, now.In(RecifeLocation))
		}
	}
}

// CheckJobs executa as tarefas agendadas para o instante informado
func (w *CronWorker) CheckJobs(ctx context.Context, now time.Time) {
	if w.cronRepo == nil {
		w.logger.Error("cron_worker_fatal", slog.String("reason", "cron_repo_missing"))
		log.Fatal("❌ [CRON WORKER] cronRepo é obrigatório; fallback em memória desabilitado em produção")
	}

	w.checkDailyReset(ctx, now)
	w.checkWeeklySeasonClose(ctx, now)
}

// checkDailyReset executa o reset diário de giros com persistência por período e retry em caso de falha
func (w *CronWorker) checkDailyReset(ctx context.Context, now time.Time) {
	nowRecife := now.In(RecifeLocation)
	todayPeriodKey := nowRecife.Format("2006-01-02")

	// Se a memória local já confirma a conclusão do período de hoje, evita chamadas repetidas
	if w.completedDailyPeriod == todayPeriodKey {
		return
	}

	alreadyDone, affected, err := w.cronRepo.ResetDailySpinsAtomic(ctx, todayPeriodKey, todayPeriodKey)
	if err != nil {
		w.logger.Error("cron_job_finished",
			slog.String("job", "daily_roulette_reset"),
			slog.String("period", todayPeriodKey),
			slog.String("resultado", "falha"),
			slog.String("categoria", ClassifyError(err)),
		)
		return // Mantém sem marcar como concluído para que o próximo tick tente novamente
	}

	w.completedDailyPeriod = todayPeriodKey

	if !alreadyDone {
		w.logger.Info("cron_job_finished",
			slog.String("job", "daily_roulette_reset"),
			slog.String("period", todayPeriodKey),
			slog.String("resultado", "sucesso"),
			slog.String("categoria", CategoryNone),
			slog.Int64("affected_count", affected),
		)
		if w.hub != nil {
			w.hub.Broadcast("daily_reset_completed", map[string]interface{}{
				"timestamp": nowRecife.Format(time.RFC3339),
				"period":    todayPeriodKey,
				"message":   "Novo ciclo diário liberado! Seu giro da Roleta da Sorte está disponível.",
			})
		}
	} else {
		w.logger.Info("cron_job_finished",
			slog.String("job", "daily_roulette_reset"),
			slog.String("period", todayPeriodKey),
			slog.String("resultado", "ja_executado"),
			slog.String("categoria", CategoryNone),
		)
	}
}

// checkWeeklySeasonClose busca todos os períodos pendentes desde o mais antigo e fecha em ordem cronológica
func (w *CronWorker) checkWeeklySeasonClose(ctx context.Context, now time.Time) {
	nowRecife := now.In(RecifeLocation)
	currentWeekStart := CurrentWeekPeriodStart(nowRecife)

	pendingWeeks, err := w.cronRepo.GetPendingWeeklyClosures(ctx, currentWeekStart, 10)
	if err != nil {
		w.logger.Error("cron_job_finished",
			slog.String("job", "weekly_season_close"),
			slog.String("period", currentWeekStart.Format("2006-01-02")),
			slog.String("resultado", "falha"),
			slog.String("categoria", ClassifyError(err)),
		)
		return
	}

	for _, closedWeekStart := range pendingWeeks {
		closedWeekEnd := closedWeekStart.AddDate(0, 0, 7)
		year, week := closedWeekStart.ISOWeek()
		closedPeriodKey := fmt.Sprintf("%04d-W%02d", year, week)

		res, err := w.cronRepo.CloseWeeklySeasonAtomic(ctx, closedPeriodKey, closedWeekStart, closedWeekEnd)
		if err != nil {
			w.logger.Error("cron_job_finished",
				slog.String("job", "weekly_season_close"),
				slog.String("period", closedPeriodKey),
				slog.String("resultado", "falha"),
				slog.String("categoria", ClassifyError(err)),
			)
			break // Interrompe para retentar no próximo tick respeitando a ordem cronológica
		}

		w.completedWeeklyPeriod = closedPeriodKey

		if !res.AlreadyClosed {
			w.logger.Info("cron_job_finished",
				slog.String("job", "weekly_season_close"),
				slog.String("period", closedPeriodKey),
				slog.String("resultado", "sucesso"),
				slog.String("categoria", CategoryNone),
				slog.Bool("winner_awarded", res.TopWinner != nil),
				slog.Bool("machine_awarded", res.RewardMachine != nil),
			)

			if w.hub != nil {
				w.hub.Broadcast("season_closed", map[string]interface{}{
					"timestamp": nowRecife.Format(time.RFC3339),
					"period":    closedPeriodKey,
					"message":   "Nova Temporada da Corrida de Mineração iniciada! Colete rendimentos para pontuar.",
				})
			}
		} else {
			w.logger.Info("cron_job_finished",
				slog.String("job", "weekly_season_close"),
				slog.String("period", closedPeriodKey),
				slog.String("resultado", "ja_executado"),
				slog.String("categoria", CategoryNone),
			)
		}
	}
}

