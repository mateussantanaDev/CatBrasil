package cron_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	deliverycron "catbrasil/backend/internal/delivery/cron"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
)

type mockUserRepoCron struct {
	spinsReset bool
}

func (m *mockUserRepoCron) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) GetByFingerprintOrIP(ctx context.Context, fp, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) Create(ctx context.Context, u *domain.User) error { return nil }
func (m *mockUserRepoCron) Update(ctx context.Context, u *domain.User) error { return nil }
func (m *mockUserRepoCron) AdjustBalance(ctx context.Context, id string, db, dy float64) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoCron) List(ctx context.Context, q, s string, l, o int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepoCron) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *mockUserRepoCron) ResetDailySpins(ctx context.Context) error {
	m.spinsReset = true
	return nil
}

type mockRankingRepoCron struct {
	boardReset bool
}

func (m *mockRankingRepoCron) AddOrUpdateWeeklyTonnage(ctx context.Context, uid, uName string, t float64) error {
	return nil
}
func (m *mockRankingRepoCron) GetWeeklyLeaderboard(ctx context.Context, l int) ([]*domain.LeaderboardEntry, error) {
	return []*domain.LeaderboardEntry{
		{
			Rank:          1,
			UserID:        "USR-WINNER",
			Username:      "campeao_minerador",
			WeeklyTonnage: 15400.0,
			Prize:         "Escavadeira CAT 320 TURBO",
			Badge:         "👑 LENDÁRIO",
		},
	}, nil
}
func (m *mockRankingRepoCron) GetUserRank(ctx context.Context, uid string) (*domain.LeaderboardEntry, error) {
	return nil, nil
}
func (m *mockRankingRepoCron) ResetWeeklyLeaderboard(ctx context.Context) error {
	m.boardReset = true
	return nil
}

type mockMachineRepoCron struct {
	createdReward *domain.Machine
}

func (m *mockMachineRepoCron) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepoCron) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepoCron) GetByUserID(ctx context.Context, uid string) ([]*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepoCron) Create(ctx context.Context, mach *domain.Machine) error {
	m.createdReward = mach
	return nil
}
func (m *mockMachineRepoCron) Update(ctx context.Context, mach *domain.Machine) error { return nil }
func (m *mockMachineRepoCron) CollectYield(ctx context.Context, mid, uid string, b float64) (*domain.Machine, float64, error) {
	return nil, 0, nil
}
func (m *mockMachineRepoCron) ResetDailyMachines(ctx context.Context) error { return nil }
func (m *mockMachineRepoCron) Delete(ctx context.Context, id string) error  { return nil }

type mockCronRepoStateful struct {
	dailyAttempts  int
	dailyFails     int
	dailyDone      bool
	weeklyAttempts int
	weeklyFails    int
	weeklyDone     bool
	lastClosedKey  string
	pendingWeeks   []time.Time
	closedKeys     []string
}

func (m *mockCronRepoStateful) ResetDailySpinsAtomic(ctx context.Context, periodKey string, spinDate string) (bool, int64, error) {
	m.dailyAttempts++
	if m.dailyFails > 0 {
		m.dailyFails--
		return false, 0, errors.New("db connection failure simulation")
	}
	if m.dailyDone {
		return true, 0, nil
	}
	m.dailyDone = true
	return false, 10, nil
}

func (m *mockCronRepoStateful) CloseWeeklySeasonAtomic(ctx context.Context, periodKey string, start, end time.Time) (*domain.WeeklyClosureResult, error) {
	m.weeklyAttempts++
	if m.weeklyFails > 0 {
		m.weeklyFails--
		return nil, errors.New("db deadlock simulation")
	}
	for _, k := range m.closedKeys {
		if k == periodKey {
			return &domain.WeeklyClosureResult{
				PeriodKey:     periodKey,
				PeriodStart:   start,
				PeriodEnd:     end,
				AlreadyClosed: true,
			}, nil
		}
	}
	m.closedKeys = append(m.closedKeys, periodKey)
	m.weeklyDone = true
	m.lastClosedKey = periodKey
	return &domain.WeeklyClosureResult{
		PeriodKey:   periodKey,
		PeriodStart: start,
		PeriodEnd:   end,
		TopWinner: &domain.LeaderboardEntry{
			UserID:        "USR-WINNER",
			Username:      "campeao_minerador",
			WeeklyTonnage: 15400.0,
		},
		RewardMachine: &domain.Machine{
			ID:     "mach-reward-" + periodKey + "-USR-WINNER",
			UserID: "USR-WINNER",
		},
		AlreadyClosed: false,
	}, nil
}

func (m *mockCronRepoStateful) GetPendingWeeklyClosures(ctx context.Context, currentWeekStart time.Time, limit int) ([]time.Time, error) {
	if len(m.pendingWeeks) > 0 {
		return m.pendingWeeks, nil
	}
	if m.weeklyDone {
		return nil, nil
	}
	return []time.Time{currentWeekStart.AddDate(0, 0, -7)}, nil
}

func (m *mockCronRepoStateful) GetExecution(ctx context.Context, jobType, periodKey string) (*domain.CronExecution, error) {
	return nil, nil
}

func (m *mockCronRepoStateful) RecordFailure(ctx context.Context, jobType, periodKey string, failureErr error) error {
	return nil
}

func TestCronWorker_DailyAndWeeklyCycles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userRepo := &mockUserRepoCron{}
	rankingRepo := &mockRankingRepoCron{}
	machineRepo := &mockMachineRepoCron{}
	cronRepo := &mockCronRepoStateful{}

	hub := ws.NewHub()
	go hub.Run()

	rouletteUC := roulette.NewRouletteUseCase(userRepo, nil, machineRepo, nil, nil)
	rankingUC := ranking.NewRankingUseCase(rankingRepo)

	worker := deliverycron.NewCronWorker(rouletteUC, rankingUC, rankingRepo, machineRepo, nil, hub, cronRepo)
	if worker == nil {
		t.Fatal("falha ao inicializar CronWorker")
	}

	go worker.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
}

func TestCronWorker_CatchupAtStartup(t *testing.T) {
	ctx := context.Background()
	cronRepo := &mockCronRepoStateful{}

	worker := deliverycron.NewCronWorker(nil, nil, nil, nil, nil, nil, cronRepo)
	now := time.Now().In(deliverycron.RecifeLocation)

	// Catch-up síncrono no início
	worker.CheckJobs(ctx, now)

	if cronRepo.dailyAttempts != 1 || !cronRepo.dailyDone {
		t.Fatalf("esperado reset diário executado no catchup: tentativas=%d feito=%v", cronRepo.dailyAttempts, cronRepo.dailyDone)
	}
	if cronRepo.weeklyAttempts != 1 || !cronRepo.weeklyDone {
		t.Fatalf("esperado fechamento semanal executado no catchup: tentativas=%d feito=%v", cronRepo.weeklyAttempts, cronRepo.weeklyDone)
	}
}

func TestCronWorker_RetryAfterFailure(t *testing.T) {
	ctx := context.Background()
	cronRepo := &mockCronRepoStateful{
		dailyFails:  1,
		weeklyFails: 1,
	}

	worker := deliverycron.NewCronWorker(nil, nil, nil, nil, nil, nil, cronRepo)
	now := time.Now().In(deliverycron.RecifeLocation)

	// Primeira tentativa: falha simulada do banco
	worker.CheckJobs(ctx, now)
	if cronRepo.dailyDone || cronRepo.weeklyDone {
		t.Fatal("operações não deveriam estar concluídas após falha")
	}

	// Segunda tentativa (próximo tick): deve retentar e concluir com sucesso
	worker.CheckJobs(ctx, now)
	if !cronRepo.dailyDone || cronRepo.dailyAttempts != 2 {
		t.Fatalf("esperado retry com sucesso no diário: tentativas=%d feito=%v", cronRepo.dailyAttempts, cronRepo.dailyDone)
	}
	if !cronRepo.weeklyDone || cronRepo.weeklyAttempts != 2 {
		t.Fatalf("esperado retry com sucesso no semanal: tentativas=%d feito=%v", cronRepo.weeklyAttempts, cronRepo.weeklyDone)
	}
}

func TestCronWorker_IdempotencyNoDoubleExecution(t *testing.T) {
	ctx := context.Background()
	cronRepo := &mockCronRepoStateful{}

	worker := deliverycron.NewCronWorker(nil, nil, nil, nil, nil, nil, cronRepo)
	now := time.Now().In(deliverycron.RecifeLocation)

	// Primeiro ciclo executa
	worker.CheckJobs(ctx, now)
	// Segundo ciclo no mesmo instante deve ser noop via cache em memória
	worker.CheckJobs(ctx, now)

	if cronRepo.dailyAttempts != 1 {
		t.Fatalf("esperada apenas 1 tentativa diária no mesmo período, obteve %d", cronRepo.dailyAttempts)
	}
	if cronRepo.weeklyAttempts != 1 {
		t.Fatalf("esperada apenas 1 tentativa semanal no mesmo período, obteve %d", cronRepo.weeklyAttempts)
	}
}

func TestCronWorker_MultiWeekCatchup(t *testing.T) {
	ctx := context.Background()
	now := time.Now().In(deliverycron.RecifeLocation)
	currentWeek := deliverycron.CurrentWeekPeriodStart(now)

	// Simula 3 semanas antigas pendentes (ex: servidor ficou 3 semanas fora)
	w1 := currentWeek.AddDate(0, 0, -21)
	w2 := currentWeek.AddDate(0, 0, -14)
	w3 := currentWeek.AddDate(0, 0, -7)

	cronRepo := &mockCronRepoStateful{
		pendingWeeks: []time.Time{w1, w2, w3},
	}

	worker := deliverycron.NewCronWorker(nil, nil, nil, nil, nil, nil, cronRepo)
	worker.CheckJobs(ctx, now)

	if len(cronRepo.closedKeys) != 3 {
		t.Fatalf("esperava 3 temporadas fechadas no catch-up, obteve %d (%v)", len(cronRepo.closedKeys), cronRepo.closedKeys)
	}

	// Verifica se foram fechadas em ordem cronológica crescente
	y1, wk1 := w1.ISOWeek()
	y2, wk2 := w2.ISOWeek()
	y3, wk3 := w3.ISOWeek()
	expectedKey1 := fmt.Sprintf("%04d-W%02d", y1, wk1)
	expectedKey2 := fmt.Sprintf("%04d-W%02d", y2, wk2)
	expectedKey3 := fmt.Sprintf("%04d-W%02d", y3, wk3)

	if cronRepo.closedKeys[0] != expectedKey1 || cronRepo.closedKeys[1] != expectedKey2 || cronRepo.closedKeys[2] != expectedKey3 {
		t.Fatalf("ordem de fechamento incorreta: esperado [%s %s %s], obteve %v", expectedKey1, expectedKey2, expectedKey3, cronRepo.closedKeys)
	}
}

func TestCronWorker_StructuredLogs_NoPII_NoRawDBErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Now().In(deliverycron.RecifeLocation)

	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cronRepo := &mockCronRepoStateful{
		dailyFails:  1,
		weeklyFails: 1,
	}

	worker := deliverycron.NewCronWorker(nil, nil, nil, nil, nil, nil, cronRepo)
	worker.SetLogger(testLogger)

	// Primeira execução: falhas simuladas
	worker.CheckJobs(ctx, now)

	// Segunda execução: sucessos
	worker.CheckJobs(ctx, now)

	logs := logBuf.String()

	// 1. Deve conter campos estruturados obrigatórios
	for _, expectedKey := range []string{`"job"`, `"period"`, `"resultado"`, `"categoria"`} {
		if !strings.Contains(logs, expectedKey) {
			t.Fatalf("logs estruturados não contêm chave esperada %s: %s", expectedKey, logs)
		}
	}

	// 2. Deve registrar resultados padronizados
	if !strings.Contains(logs, `"resultado":"falha"`) {
		t.Fatalf("esperado resultado:falha nos logs: %s", logs)
	}
	if !strings.Contains(logs, `"resultado":"sucesso"`) {
		t.Fatalf("esperado resultado:sucesso nos logs: %s", logs)
	}

	// 3. Deve usar categorias fixas de erro
	if !strings.Contains(logs, `"categoria":"db_connection_failed"`) {
		t.Fatalf("esperada categoria de erro fixa db_connection_failed: %s", logs)
	}

	// 4. NÃO PODE CONTER PII: username do vencedor
	if strings.Contains(logs, "campeao_minerador") {
		t.Fatalf("VIOLAÇÃO DE PII: log vazou username: %s", logs)
	}

	// 5. NÃO PODE CONTER identificador de máquina/recompensa
	if strings.Contains(logs, "mach-reward") {
		t.Fatalf("VIOLAÇÃO DE DADOS: log vazou ID de máquina: %s", logs)
	}

	// 6. NÃO PODE CONTER mensagem de erro bruto do banco de dados
	for _, rawErr := range []string{"db connection failure simulation", "db deadlock simulation"} {
		if strings.Contains(logs, rawErr) {
			t.Fatalf("VIOLAÇÃO DE SEGURANÇA: log vazou erro bruto de banco (%s): %s", rawErr, logs)
		}
	}
}

