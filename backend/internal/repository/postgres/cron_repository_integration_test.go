package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
)

func TestCronRepository_SafeDailyReset_DoesNotResetTodaySpins(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	today := time.Now().In(postgres.RecifeLocation).Format("2006-01-02")
	yesterday := time.Now().In(postgres.RecifeLocation).AddDate(0, 0, -1).Format("2006-01-02")

	u1 := fmt.Sprintf("u-yesterday-%d", time.Now().UnixNano())
	u2 := fmt.Sprintf("u-today-%d", time.Now().UnixNano())
	periodKey := fmt.Sprintf("daily-test-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, u1, u2)
	})

	seedFinancialUser(t, pool, u1, 100)
	seedFinancialUser(t, pool, u2, 100)

	// u1 girou ontem
	_, err := pool.Exec(ctx, `UPDATE users SET already_spun_today = TRUE, last_spin_date = $1 WHERE id = $2`, yesterday, u1)
	if err != nil {
		t.Fatal(err)
	}

	// u2 girou hoje de manhã antes do cron rodar
	_, err = pool.Exec(ctx, `UPDATE users SET already_spun_today = TRUE, last_spin_date = $1 WHERE id = $2`, today, u2)
	if err != nil {
		t.Fatal(err)
	}

	cronRepo := postgres.NewCronRepository(pool)

	alreadyDone, affected, err := cronRepo.ResetDailySpinsAtomic(ctx, periodKey, today)
	if err != nil {
		t.Fatalf("erro ao executar reset diário atômico: %v", err)
	}
	if alreadyDone {
		t.Fatal("não deveria constar como alreadyDone na primeira execução")
	}
	if affected < 1 {
		t.Fatalf("esperava ao menos 1 usuário resetado, obteve %d", affected)
	}

	// Verifica que u1 foi resetado (já pode girar hoje)
	var u1Spun bool
	if err := pool.QueryRow(ctx, `SELECT already_spun_today FROM users WHERE id = $1`, u1).Scan(&u1Spun); err != nil {
		t.Fatal(err)
	}
	if u1Spun {
		t.Fatal("u1 (que girou ontem) deveria ter sido resetado para FALSE")
	}

	// Verifica que u2 NÃO foi resetado (continua bloqueado porque já girou hoje)
	var u2Spun bool
	if err := pool.QueryRow(ctx, `SELECT already_spun_today FROM users WHERE id = $1`, u2).Scan(&u2Spun); err != nil {
		t.Fatal(err)
	}
	if !u2Spun {
		t.Fatal("u2 (que girou hoje) NÃO deveria ter sido resetado (deve continuar TRUE)")
	}

	// Segunda execução no mesmo periodKey deve retornar alreadyDone = true
	alreadyDone2, _, err := cronRepo.ResetDailySpinsAtomic(ctx, periodKey, today)
	if err != nil {
		t.Fatalf("erro na segunda execução: %v", err)
	}
	if !alreadyDone2 {
		t.Fatal("segunda execução no mesmo período deveria constar como alreadyDone = true")
	}
}

func TestCronRepository_WeeklyClosureAtomic_IdempotentRewardAndLateCollectionsPreserved(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	// Período isolado exclusivamente na semana histórica de 2011 (não interfere com dados vivos nem outros testes)
	testWeekStart := time.Date(2011, 1, 3, 0, 0, 0, 0, postgres.RecifeLocation)
	testNextWeekStart := testWeekStart.AddDate(0, 0, 7)
	periodKey := fmt.Sprintf("2011-W01-TEST-%d", time.Now().UnixNano())

	w1 := fmt.Sprintf("winner-%d", time.Now().UnixNano())
	w2 := fmt.Sprintf("second-%d", time.Now().UnixNano())
	w3 := fmt.Sprintf("late-%d", time.Now().UnixNano())

	expectedMachID := postgres.RewardMachineID(periodKey, w1)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE period_start IN ($1, $2)`, testWeekStart, testNextWeekStart)
		pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, expectedMachID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2, $3)`, w1, w2, w3)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, w1, 0)
	seedFinancialUser(t, pool, w2, 0)
	seedFinancialUser(t, pool, w3, 0)

	// Inserir pontuações da semana encerrada (testWeekStart)
	_, err := pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Campeão', 5000.0, $1),
		       ($1, $3, 'Operador Segundo', 3000.0, $1)
	`, testWeekStart, w1, w2)
	if err != nil {
		t.Fatal(err)
	}

	// Inserir coleta atrasada que já chegou na nova semana (testNextWeekStart)
	_, err = pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Madrugador', 1200.0, $1)
	`, testNextWeekStart, w3)
	if err != nil {
		t.Fatal(err)
	}

	cronRepo := postgres.NewCronRepository(pool)

	// Fecha semana encerrada
	res, err := cronRepo.CloseWeeklySeasonAtomic(ctx, periodKey, testWeekStart, testNextWeekStart)
	if err != nil {
		t.Fatalf("fechamento semanal falhou: %v", err)
	}
	if res.AlreadyClosed {
		t.Fatal("primeiro fechamento não deveria constar como AlreadyClosed")
	}
	if res.TopWinner == nil || res.TopWinner.UserID != w1 {
		t.Fatalf("vencedor incorreto: esperado %s, obteve %+v", w1, res.TopWinner)
	}
	if res.TopWinner.WeeklyTonnage != 5000.0 {
		t.Fatalf("tonelagem do vencedor incorreta: %f", res.TopWinner.WeeklyTonnage)
	}

	// Verifica que o prêmio foi concedido na tabela machines com ID <= 64 chars
	if len(expectedMachID) > 64 {
		t.Fatalf("expectedMachID ultrapassa limite de 64 caracteres: len=%d (%s)", len(expectedMachID), expectedMachID)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM machines WHERE id = $1 AND user_id = $2`, expectedMachID, w1).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("máquina prêmio com ID único não encontrada: count=%d (id=%s)", count, expectedMachID)
	}

	// Pontuação do w3 (coleta da nova semana) continua intacta em mining_race_weekly
	var w3Tonnage float64
	err = pool.QueryRow(ctx, `SELECT weekly_tonnage FROM mining_race_weekly WHERE period_start = $1 AND user_id = $2`, testNextWeekStart, w3).Scan(&w3Tonnage)
	if err != nil {
		t.Fatalf("coleta da nova semana foi perdida ou corrompida: %v", err)
	}
	if w3Tonnage != 1200.0 {
		t.Fatalf("tonelagem da nova semana alterada: esperado 1200, obteve %f", w3Tonnage)
	}

	// Re-execução para idempotência: não deve duplicar nem falhar
	res2, err := cronRepo.CloseWeeklySeasonAtomic(ctx, periodKey, testWeekStart, testNextWeekStart)
	if err != nil {
		t.Fatalf("re-execução falhou: %v", err)
	}
	if !res2.AlreadyClosed {
		t.Fatal("re-execução deveria retornar AlreadyClosed = true")
	}

	// Quantidade de máquinas continua sendo exatamente 1
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM machines WHERE id = $1`, expectedMachID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("idempotência falhou: número de máquinas prêmio é %d", count)
	}
}

func TestCronRepository_ConcurrentReplicasExclusion(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	// Semana isolada na janela de 2012 e periodKey longo para testar garantia de ID <= 64
	testWeekStart := time.Date(2012, 1, 2, 0, 0, 0, 0, postgres.RecifeLocation)
	testNextWeekStart := testWeekStart.AddDate(0, 0, 7)
	periodKey := fmt.Sprintf("CONCURRENT-W-VERY-LONG-KEY-FOR-TESTING-%d", time.Now().UnixNano())

	w1 := fmt.Sprintf("winner-conc-%d", time.Now().UnixNano())
	expectedMachID := postgres.RewardMachineID(periodKey, w1)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE period_start = $1`, testWeekStart)
		pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, expectedMachID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, w1)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, w1, 0)
	_, err := pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Concorrente', 9999.0, $1)
	`, testWeekStart, w1)
	if err != nil {
		t.Fatal(err)
	}

	const replicas = 8
	var wg sync.WaitGroup
	results := make(chan *postgres.CronRepository, replicas)
	errors := make(chan error, replicas)

	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo := postgres.NewCronRepository(pool)
			results <- repo
			_, err := repo.CloseWeeklySeasonAtomic(context.Background(), periodKey, testWeekStart, testNextWeekStart)
			if err != nil && err != postgres.ErrLockedByAnotherReplica {
				errors <- err
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatalf("erro inesperado entre réplicas: %v", err)
	}

	// Verifica se cron_executions foi marcado como completed
	exec, err := postgres.NewCronRepository(pool).GetExecution(ctx, "weekly_season_close", periodKey)
	if err != nil {
		t.Fatal(err)
	}
	if exec == nil || exec.Status != "completed" {
		t.Fatalf("execução deveria estar concluída, obteve %+v", exec)
	}

	// Verifica que apenas 1 máquina foi criada e seu ID respeitou <= 64 caracteres
	if len(expectedMachID) > 64 {
		t.Fatalf("expectedMachID ultrapassa 64 caracteres: len=%d (%s)", len(expectedMachID), expectedMachID)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM machines WHERE id = $1`, expectedMachID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("esperada exatamente 1 máquina criada entre réplicas concorrentes, obteve %d", count)
	}
}

func TestRoulette_ApplySpin_AllowedOnNewDayEvenIfAlreadySpunTodayFlagStale(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	yesterday := time.Now().In(postgres.RecifeLocation).AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().In(postgres.RecifeLocation).Format("2006-01-02")

	userID := fmt.Sprintf("u-stale-spin-%d", time.Now().UnixNano())

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM transactions WHERE user_id = $1`, userID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, userID, 50.0)

	// Simula que cron atrasou: flag already_spun_today ainda é TRUE de ontem, mas last_spin_date é ontem
	_, err := pool.Exec(ctx, `
		UPDATE users
		SET already_spun_today = TRUE, last_spin_date = $1, last_spin_at = now() - interval '1 day'
		WHERE id = $2
	`, yesterday, userID)
	if err != nil {
		t.Fatal(err)
	}

	txRepo := postgres.NewTransactionRepository(pool)

	// Usuário tenta girar hoje: DEVE ser permitido porque last_spin_date < today
	updatedUser, err := txRepo.ApplySpin(ctx, userID, "Bônus R$ 5,00", "", 5.00)
	if err != nil {
		t.Fatalf("ApplySpin falhou indevidamente quando cron atrasou: %v", err)
	}

	if updatedUser.Balance != 55.00 {
		t.Fatalf("saldo esperado 55.00, obteve %f", updatedUser.Balance)
	}
	if updatedUser.LastSpinDate != today {
		t.Fatalf("last_spin_date esperado %s, obteve %s", today, updatedUser.LastSpinDate)
	}
	if !updatedUser.AlreadySpunToday {
		t.Fatal("esperava already_spun_today = true após giro realizado hoje")
	}

	// Segundo giro no mesmo dia: DEVE ser bloqueado com ErrSpinAlreadyUsedToday
	_, err = txRepo.ApplySpin(ctx, userID, "Bônus R$ 5,00", "", 5.00)
	if err != domain.ErrSpinAlreadyUsedToday {
		t.Fatalf("segundo giro deveria ser bloqueado com ErrSpinAlreadyUsedToday, obteve: %v", err)
	}
}

func TestRoulette_ResetUserSpin_ClearsLastSpinDateAndAllowsSpin(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	today := time.Now().In(postgres.RecifeLocation).Format("2006-01-02")
	userID := fmt.Sprintf("u-admin-reset-%d", time.Now().UnixNano())

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM transactions WHERE user_id = $1`, userID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, userID, 100.0)

	txRepo := postgres.NewTransactionRepository(pool)
	userRepo := postgres.NewUserRepository(pool)

	// 1. Usuário gira hoje pela primeira vez
	userAfterSpin, err := txRepo.ApplySpin(ctx, userID, "Bônus R$ 10,00", "", 10.00)
	if err != nil {
		t.Fatalf("primeiro giro falhou: %v", err)
	}
	if userAfterSpin.LastSpinDate != today || !userAfterSpin.AlreadySpunToday {
		t.Fatalf("giro de hoje não foi registrado adequadamente: date=%s flag=%v", userAfterSpin.LastSpinDate, userAfterSpin.AlreadySpunToday)
	}

	// 2. Novo giro no mesmo dia é bloqueado
	_, err = txRepo.ApplySpin(ctx, userID, "Bônus R$ 10,00", "", 10.00)
	if err != domain.ErrSpinAlreadyUsedToday {
		t.Fatalf("segundo giro deveria ser bloqueado, obteve: %v", err)
	}

	// 3. Admin executa ResetUserSpin (limpa flag e last_spin_date/at)
	u, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("falha ao carregar usuário para reset: %v", err)
	}
	u.AlreadySpunToday = false
	u.LastSpinDate = ""
	u.LastSpinAt = nil
	if err := userRepo.Update(ctx, u); err != nil {
		t.Fatalf("falha ao atualizar usuário no reset admin: %v", err)
	}

	// 4. Confirma diretamente no PostgreSQL que last_spin_date foi limpo para NULL
	var alreadySpun bool
	var dbLastSpinDate *string
	var dbLastSpinAt *time.Time
	err = pool.QueryRow(ctx, `SELECT already_spun_today, last_spin_date, last_spin_at FROM users WHERE id = $1`, userID).Scan(&alreadySpun, &dbLastSpinDate, &dbLastSpinAt)
	if err != nil {
		t.Fatalf("falha ao consultar banco após reset: %v", err)
	}
	if alreadySpun {
		t.Fatal("already_spun_today deveria ser FALSE após reset admin")
	}
	if dbLastSpinDate != nil && *dbLastSpinDate != "" {
		t.Fatalf("last_spin_date deveria ser NULL/vazio após reset admin, obteve: %s", *dbLastSpinDate)
	}
	if dbLastSpinAt != nil {
		t.Fatal("last_spin_at deveria ser NULL após reset admin")
	}

	// 5. Usuário tenta girar novamente no mesmo dia: DEVE SER PERMITIDO!
	userAfterResetSpin, err := txRepo.ApplySpin(ctx, userID, "Bônus R$ 10,00", "", 10.00)
	if err != nil {
		t.Fatalf("giro após reset administrativo foi indevidamente bloqueado: %v", err)
	}
	if userAfterResetSpin.Balance != 120.00 {
		t.Fatalf("saldo esperado 120.00 (100 + 10 + 10), obteve %f", userAfterResetSpin.Balance)
	}
	if userAfterResetSpin.LastSpinDate != today {
		t.Fatalf("last_spin_date esperado %s após novo giro, obteve %s", today, userAfterResetSpin.LastSpinDate)
	}
}

func TestCronRepository_GetPendingWeeklyClosures_MultiWeekOrder(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	// Janela histórica isolada no ano 2000 (onde nenhum outro registro de teste existe)
	wOldest := time.Date(2000, 1, 3, 0, 0, 0, 0, postgres.RecifeLocation)
	wNewer := time.Date(2000, 1, 10, 0, 0, 0, 0, postgres.RecifeLocation)
	testBound := time.Date(2000, 1, 17, 0, 0, 0, 0, postgres.RecifeLocation)

	uid1 := fmt.Sprintf("u-pw1-%d", time.Now().UnixNano())
	uid2 := fmt.Sprintf("u-pw2-%d", time.Now().UnixNano())

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE period_start < '2000-02-01'`)
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key LIKE '2000-W%'`)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, uid1, uid2)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, uid1, 0)
	seedFinancialUser(t, pool, uid2, 0)

	_, err := pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Antigo', 1000.0, $1),
		       ($3, $4, 'Operador Novo', 2000.0, $3)
	`, wOldest, uid1, wNewer, uid2)
	if err != nil {
		t.Fatal(err)
	}

	cronRepo := postgres.NewCronRepository(pool)
	pending, err := cronRepo.GetPendingWeeklyClosures(ctx, testBound, 10)
	if err != nil {
		t.Fatalf("falha ao consultar semanas pendentes: %v", err)
	}

	// Encontra os índices de wOldest e wNewer
	idxOldest := -1
	idxNewer := -1
	for i, p := range pending {
		if p.Equal(wOldest) {
			idxOldest = i
		}
		if p.Equal(wNewer) {
			idxNewer = i
		}
	}

	if idxOldest == -1 || idxNewer == -1 {
		t.Fatalf("esperava encontrar ambas as semanas pendentes na janela isolada, obteve: %v", pending)
	}
	if idxOldest >= idxNewer {
		t.Fatalf("ordem cronológica incorreta: wOldest (%d) deveria preceder wNewer (%d)", idxOldest, idxNewer)
	}
}

func TestCronRepository_Auditability_WeeklyCloseInducedFailureAndRetryTracksAttempts(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	// Janela histórica isolada no ano de 2013
	testWeekStart := time.Date(2013, 1, 7, 0, 0, 0, 0, postgres.RecifeLocation)
	testNextWeekStart := testWeekStart.AddDate(0, 0, 7)
	periodKey := fmt.Sprintf("AUDIT-FAIL-W13-%d", time.Now().UnixNano())

	w1 := fmt.Sprintf("winner-audit-%d", time.Now().UnixNano())
	expectedMachID := postgres.RewardMachineID(periodKey, w1)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE period_start = $1`, testWeekStart)
		pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, expectedMachID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, w1)
	}
	cleanup()
	t.Cleanup(cleanup)

	cronRepo := postgres.NewCronRepository(pool)

	// 1. Falha induzida na 1ª tentativa: contexto cancelado antes da execução
	failCtx, cancel := context.WithCancel(ctx)
	cancel() // cancelado previamente

	_, err := cronRepo.CloseWeeklySeasonAtomic(failCtx, periodKey, testWeekStart, testNextWeekStart)
	if err == nil {
		t.Fatal("esperava erro de execução com contexto cancelado")
	}

	// 2. Verifica que RecordFailure gravou 'failed', attempt_count=1 e last_error no banco
	exec1, err := cronRepo.GetExecution(ctx, "weekly_season_close", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução após erro: %v", err)
	}
	if exec1 == nil {
		t.Fatal("esperava registro em cron_executions após falha")
	}
	if exec1.Status != "failed" {
		t.Fatalf("esperado status 'failed', obteve %q", exec1.Status)
	}
	if exec1.AttemptCount != 1 {
		t.Fatalf("esperado attempt_count = 1, obteve %d", exec1.AttemptCount)
	}
	if exec1.LastError != "timeout" {
		t.Fatalf("esperado categoria fixa 'timeout' em last_error, obteve: %q", exec1.LastError)
	}
	// Canário: last_error NUNCA pode vazar string bruta ou segredo
	canarySecret := "SUPER_SECRET_CANARY_DSN_PASSWORD"
	if strings.Contains(exec1.LastError, "canceled") || strings.Contains(exec1.LastError, canarySecret) {
		t.Fatalf("last_error vazou texto bruto ou segredo: %q", exec1.LastError)
	}

	// 3. Retry com contexto válido: deve recuperar, incrementar attempt_count e concluir
	seedFinancialUser(t, pool, w1, 0)
	_, err = pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Auditado', 8888.0, $1)
	`, testWeekStart, w1)
	if err != nil {
		t.Fatal(err)
	}

	res, err := cronRepo.CloseWeeklySeasonAtomic(ctx, periodKey, testWeekStart, testNextWeekStart)
	if err != nil {
		t.Fatalf("retry de fechamento semanal falhou: %v", err)
	}
	if res.AlreadyClosed {
		t.Fatal("retry não deveria constar como AlreadyClosed")
	}
	if res.TopWinner == nil || res.TopWinner.UserID != w1 {
		t.Fatalf("vencedor incorreto no retry: %+v", res.TopWinner)
	}

	// 4. Verifica que cron_executions evoluiu para 'completed' com attempt_count = 2
	exec2, err := cronRepo.GetExecution(ctx, "weekly_season_close", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução após retry: %v", err)
	}
	if exec2 == nil {
		t.Fatal("registro não encontrado após retry")
	}
	if exec2.Status != "completed" {
		t.Fatalf("esperado status 'completed', obteve %q", exec2.Status)
	}
	if exec2.AttemptCount != 2 {
		t.Fatalf("esperado attempt_count incrementado para 2 no retry, obteve %d", exec2.AttemptCount)
	}
	if exec2.FinishedAt == nil {
		t.Fatal("esperava finished_at preenchido após conclusão com sucesso")
	}
}

func TestCronRepository_Auditability_DailyResetInducedFailureAndRetryTracksAttempts(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	periodKey := fmt.Sprintf("AUDIT-DAILY-FAIL-%d", time.Now().UnixNano())
	today := time.Now().In(postgres.RecifeLocation).Format("2006-01-02")

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
	}
	cleanup()
	t.Cleanup(cleanup)

	cronRepo := postgres.NewCronRepository(pool)

	// 1. Falha induzida na 1ª tentativa
	failCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, _, err := cronRepo.ResetDailySpinsAtomic(failCtx, periodKey, today)
	if err == nil {
		t.Fatal("esperava erro com contexto cancelado")
	}

	// 2. Confirma gravação de falha durável em cron_executions
	exec1, err := cronRepo.GetExecution(ctx, "daily_roulette_reset", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução diária: %v", err)
	}
	if exec1 == nil || exec1.Status != "failed" || exec1.AttemptCount != 1 {
		t.Fatalf("esperava execução failed com 1 tentativa, obteve: %+v", exec1)
	}

	// 3. Retry com sucesso
	alreadyDone, _, err := cronRepo.ResetDailySpinsAtomic(ctx, periodKey, today)
	if err != nil {
		t.Fatalf("retry diário falhou: %v", err)
	}
	if alreadyDone {
		t.Fatal("retry diário não deveria constar como alreadyDone")
	}

	// 4. Confirma transição para completed com attempt_count = 2
	exec2, err := cronRepo.GetExecution(ctx, "daily_roulette_reset", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução pós-retry: %v", err)
	}
	if exec2 == nil || exec2.Status != "completed" || exec2.AttemptCount != 2 {
		t.Fatalf("esperava execução completed com 2 tentativas, obteve: %+v", exec2)
	}
}

func TestCronRepository_Auditability_LockExclusionDoesNotMarkFailed(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	periodKey := fmt.Sprintf("LOCK-EXCL-TEST-%d", time.Now().UnixNano())

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
	}
	cleanup()
	t.Cleanup(cleanup)

	cronRepo := postgres.NewCronRepository(pool)

	// Segura lock consultivo diário em transação manual
	txHold, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txHold.Rollback(ctx)

	var locked bool
	if err := txHold.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", postgres.LockCronDailyReset).Scan(&locked); err != nil || !locked {
		t.Fatalf("falha ao travar lock manualmente: locked=%v err=%v", locked, err)
	}

	// Tenta executar reset na mesma chave enquanto bloqueado
	_, _, err = cronRepo.ResetDailySpinsAtomic(ctx, periodKey, "2026-10-19")
	if !errors.Is(err, postgres.ErrLockedByAnotherReplica) {
		t.Fatalf("esperado ErrLockedByAnotherReplica, obteve: %v", err)
	}

	// Confirma que NÃO gravou status 'failed' para exclusão mútua normal
	exec, err := cronRepo.GetExecution(ctx, "daily_roulette_reset", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução: %v", err)
	}
	if exec != nil {
		t.Fatalf("execução NÃO deve ser criada/marcada como falha para ErrLockedByAnotherReplica, obteve: %+v", exec)
	}
}

func TestCronRepository_Auditability_RecordFailureAfterCompletedPreservesSuccess(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	// Janela histórica isolada no ano de 2014
	testWeekStart := time.Date(2014, 1, 6, 0, 0, 0, 0, postgres.RecifeLocation)
	testNextWeekStart := testWeekStart.AddDate(0, 0, 7)
	periodKey := fmt.Sprintf("AUDIT-PRESERVE-%d", time.Now().UnixNano())

	w1 := fmt.Sprintf("winner-pres-%d", time.Now().UnixNano())
	expectedMachID := postgres.RewardMachineID(periodKey, w1)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key = $1`, periodKey)
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE period_start = $1`, testWeekStart)
		pool.Exec(context.Background(), `DELETE FROM machines WHERE id = $1`, expectedMachID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, w1)
	}
	cleanup()
	t.Cleanup(cleanup)

	seedFinancialUser(t, pool, w1, 0)
	_, err := pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Vencedor', 9500.0, $1)
	`, testWeekStart, w1)
	if err != nil {
		t.Fatal(err)
	}

	cronRepo := postgres.NewCronRepository(pool)

	// 1. Replica A conclui com sucesso primeiro
	res, err := cronRepo.CloseWeeklySeasonAtomic(ctx, periodKey, testWeekStart, testNextWeekStart)
	if err != nil {
		t.Fatalf("fechamento semanal falhou: %v", err)
	}
	if res.AlreadyClosed || res.TopWinner == nil || res.TopWinner.UserID != w1 {
		t.Fatalf("fechamento não obteve resultado esperado: %+v", res)
	}

	// 2. Confirma status 'completed' com attempt_count = 1
	execBefore, err := cronRepo.GetExecution(ctx, "weekly_season_close", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução: %v", err)
	}
	if execBefore == nil || execBefore.Status != "completed" || execBefore.AttemptCount != 1 {
		t.Fatalf("esperava completed com 1 tentativa, obteve: %+v", execBefore)
	}

	// 3. Replica B tenta gravar RecordFailure tardio para o mesmo período
	err = cronRepo.RecordFailure(ctx, "weekly_season_close", periodKey, errors.New("falha tardia simulada de réplica atrasada"))
	if err != nil {
		t.Fatalf("RecordFailure não deveria retornar erro não tratado: %v", err)
	}

	// 4. Confirma que o status 'completed', attempt_count e finished_at continuam intactos
	execAfter, err := cronRepo.GetExecution(ctx, "weekly_season_close", periodKey)
	if err != nil {
		t.Fatalf("falha ao consultar execução pós-falha tardia: %v", err)
	}
	if execAfter == nil {
		t.Fatal("execução desapareceu")
	}
	if execAfter.Status != "completed" {
		t.Fatalf("RecordFailure tardio SOBRESCREVEU status completed para %q!", execAfter.Status)
	}
	if execAfter.AttemptCount != 1 {
		t.Fatalf("attempt_count foi indevidamente alterado: esperado 1, obteve %d", execAfter.AttemptCount)
	}
	if execAfter.LastError != "" {
		t.Fatalf("last_error foi indevidamente sobrescrito: %q", execAfter.LastError)
	}

	// 5. Confirma que a máquina prêmio continua existindo no banco
	var machCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM machines WHERE id = $1`, expectedMachID).Scan(&machCount); err != nil {
		t.Fatal(err)
	}
	if machCount != 1 {
		t.Fatalf("máquina prêmio foi perdida: count=%d", machCount)
	}
}

func TestCronRepository_GetCronObservabilityMetrics_And_RecordFailureSanitized(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()

	cronRepo := postgres.NewCronRepository(pool)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM cron_executions WHERE period_key LIKE 'test-obs-%'`)
		pool.Exec(context.Background(), `DELETE FROM mining_race_weekly WHERE user_id LIKE 'obs-user-%'`)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id LIKE 'obs-user-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	// 1. Teste de RecordFailure com Canary Secret
	canarySecret := "SUPER_SECRET_CANARY_PASSWORD_12345"
	canaryErr := fmt.Errorf("FATAL: password authentication failed for user 'postgres' password '%s' dbname 'prod'", canarySecret)
	canaryPeriod := fmt.Sprintf("test-obs-canary-%d", time.Now().UnixNano())

	// Prior semantic: RecordFailure returns nil upon successful durable audit record
	recErr := cronRepo.RecordFailure(ctx, "daily_roulette_reset", canaryPeriod, canaryErr)
	if recErr != nil {
		t.Fatalf("esperava nil de RecordFailure em gravação com sucesso, obteve: %v", recErr)
	}

	// Verifica o valor gravado em cron_executions.last_error
	var savedLastError string
	var savedStatus string
	err := pool.QueryRow(ctx, `SELECT status, last_error FROM cron_executions WHERE job_type = 'daily_roulette_reset' AND period_key = $1`, canaryPeriod).Scan(&savedStatus, &savedLastError)
	if err != nil {
		t.Fatalf("falha ao consultar gravação de falha do canary: %v", err)
	}
	if savedStatus != "failed" {
		t.Fatalf("esperava status 'failed', obteve %q", savedStatus)
	}
	if strings.Contains(savedLastError, canarySecret) {
		t.Fatalf("CRÍTICO: last_error no PostgreSQL contém segredo canary! Valor=%q", savedLastError)
	}
	if savedLastError != "db_connection_failed" && savedLastError != "db_error" {
		t.Fatalf("esperava categoria estática ('db_connection_failed' ou 'db_error'), obteve %q", savedLastError)
	}

	// 2. Completed Daily e Weekly + Exclusão de Job Desconhecido
	dailyPeriod := fmt.Sprintf("test-obs-daily-%d", time.Now().UnixNano())
	weeklyPeriod := fmt.Sprintf("test-obs-weekly-%d", time.Now().UnixNano())
	unknownPeriod := fmt.Sprintf("test-obs-unknown-%d", time.Now().UnixNano())

	// Limpa tabela cron_executions para isolar contagens das métricas
	pool.Exec(ctx, `DELETE FROM cron_executions`)

	// Insere completed daily
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('daily_roulette_reset', $1, 'completed', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 1, '')
	`, dailyPeriod)
	if err != nil {
		t.Fatal(err)
	}

	// Insere completed weekly
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('weekly_season_close', $1, 'completed', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', 1, '')
	`, weeklyPeriod)
	if err != nil {
		t.Fatal(err)
	}

	// Insere job_type desconhecido (deve ser ignorado pela allowlist)
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('unauthorized_custom_job', $1, 'completed', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', 1, '')
	`, unknownPeriod)
	if err != nil {
		t.Fatal(err)
	}

	// Insere falha recente (< 24h) para daily
	recentFailPeriod := fmt.Sprintf("test-obs-fail-rec-%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('daily_roulette_reset', $1, 'failed', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes', 1, 'timeout')
	`, recentFailPeriod)
	if err != nil {
		t.Fatal(err)
	}

	// Insere falha antiga (> 24h) para daily (não deve contar em recent_failures)
	oldFailPeriod := fmt.Sprintf("test-obs-fail-old-%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('daily_roulette_reset', $1, 'failed', NOW() - INTERVAL '26 hours', NOW() - INTERVAL '26 hours', 1, 'db_error')
	`, oldFailPeriod)
	if err != nil {
		t.Fatal(err)
	}

	// Consulta métricas
	metricsData, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("falha ao consultar métricas do cron: %v", err)
	}

	// Verifica se job desconhecido foi excluído
	jobMap := make(map[string]postgres.CronObservabilityMetric)
	for _, j := range metricsData.Jobs {
		jobMap[j.JobType] = j
		if j.JobType != "daily_roulette_reset" && j.JobType != "weekly_season_close" {
			t.Fatalf("job_type desconhecido %q não foi excluído da allowlist", j.JobType)
		}
	}

	// Valida daily_roulette_reset
	dailyMetric, ok := jobMap["daily_roulette_reset"]
	if !ok {
		t.Fatal("esperava métrica para daily_roulette_reset")
	}
	if dailyMetric.LastSuccessTimestamp <= 0 {
		t.Fatalf("esperava LastSuccessTimestamp > 0, obteve %d", dailyMetric.LastSuccessTimestamp)
	}
	if dailyMetric.RecentFailures != 1 {
		t.Fatalf("esperava RecentFailures = 1 (apenas falha recente < 24h), obteve %d", dailyMetric.RecentFailures)
	}

	// Valida weekly_season_close
	weeklyMetric, ok := jobMap["weekly_season_close"]
	if !ok {
		t.Fatal("esperava métrica para weekly_season_close")
	}
	if weeklyMetric.LastSuccessTimestamp <= 0 {
		t.Fatalf("esperava LastSuccessTimestamp > 0, obteve %d", weeklyMetric.LastSuccessTimestamp)
	}
	if weeklyMetric.RecentFailures != 0 {
		t.Fatalf("esperava RecentFailures = 0 para weekly, obteve %d", weeklyMetric.RecentFailures)
	}

	// 3. Pending Weekly Periods
	// Limpa mining_race_weekly para testar pendência
	pool.Exec(ctx, `DELETE FROM mining_race_weekly`)

	// Com mining_race_weekly vazia, pendingCount deve ser 0
	metricsEmptyWeekly, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("GetCronObservabilityMetrics falhou: %v", err)
	}
	if metricsEmptyWeekly.PendingWeeklyPeriods != 0 {
		t.Fatalf("esperava PendingWeeklyPeriods = 0 com tabela vazia, obteve %d", metricsEmptyWeekly.PendingWeeklyPeriods)
	}

	// Insere semana passada em mining_race_weekly sem execução completed correspondente
	pastWeekStart := time.Now().In(postgres.RecifeLocation).AddDate(0, 0, -14).Truncate(24 * time.Hour)
	testUser := fmt.Sprintf("obs-user-%d", time.Now().UnixNano())
	seedFinancialUser(t, pool, testUser, 0)

	_, err = pool.Exec(ctx, `
		INSERT INTO mining_race_weekly (period_start, user_id, username, weekly_tonnage, updated_at)
		VALUES ($1, $2, 'Operador Teste', 100.0, NOW())
	`, pastWeekStart, testUser)
	if err != nil {
		t.Fatal(err)
	}

	// Agora deve acusar 1 semana pendente
	metricsPending, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("GetCronObservabilityMetrics falhou: %v", err)
	}
	if metricsPending.PendingWeeklyPeriods < 1 {
		t.Fatalf("esperava ao menos 1 período semanal pendente, obteve %d", metricsPending.PendingWeeklyPeriods)
	}

	// Marca a semana como concluída em cron_executions com o period_key correto
	var expectedPeriodKey string
	err = pool.QueryRow(ctx, `SELECT to_char($1::timestamptz AT TIME ZONE 'America/Recife', 'IYYY-"W"IW')`, pastWeekStart).Scan(&expectedPeriodKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('weekly_season_close', $1, 'completed', NOW(), NOW(), 1, '')
	`, expectedPeriodKey)
	if err != nil {
		t.Fatal(err)
	}

	// Após conclusão, PendingWeeklyPeriods volta a 0
	metricsResolved, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("GetCronObservabilityMetrics falhou: %v", err)
	}
	if metricsResolved.PendingWeeklyPeriods != 0 {
		t.Fatalf("esperava PendingWeeklyPeriods = 0 após conclusão da semana, obteve %d", metricsResolved.PendingWeeklyPeriods)
	}

	// 4. Pending Daily Periods
	// Caso A: Usuário com giro de ontem aguardando reset acusa pendência
	yesterdayStr := time.Now().In(postgres.RecifeLocation).AddDate(0, 0, -1).Format("2006-01-02")
	todayStr := time.Now().In(postgres.RecifeLocation).Format("2006-01-02")
	spinUser := fmt.Sprintf("obs-user-spin-%d", time.Now().UnixNano())
	seedFinancialUser(t, pool, spinUser, 0)
	_, err = pool.Exec(ctx, `UPDATE users SET already_spun_today = TRUE, last_spin_date = $1 WHERE id = $2`, yesterdayStr, spinUser)
	if err != nil {
		t.Fatal(err)
	}

	// Garante que não há completed para hoje ainda
	pool.Exec(ctx, `DELETE FROM cron_executions WHERE job_type = 'daily_roulette_reset' AND period_key = $1`, todayStr)

	metricsDailyPending, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("GetCronObservabilityMetrics falhou: %v", err)
	}
	if metricsDailyPending.PendingDailyPeriods != 1 {
		t.Fatalf("esperava PendingDailyPeriods = 1 para usuário aguardando reset de ontem, obteve %d", metricsDailyPending.PendingDailyPeriods)
	}

	// Caso B: Execução de hoje concluída zera pendência diária
	_, err = pool.Exec(ctx, `
		INSERT INTO cron_executions (job_type, period_key, status, started_at, finished_at, attempt_count, last_error)
		VALUES ('daily_roulette_reset', $1, 'completed', NOW(), NOW(), 1, '')
		ON CONFLICT (job_type, period_key) DO UPDATE SET status = 'completed'
	`, todayStr)
	if err != nil {
		t.Fatal(err)
	}

	metricsDailyResolved, err := cronRepo.GetCronObservabilityMetrics(ctx)
	if err != nil {
		t.Fatalf("GetCronObservabilityMetrics falhou: %v", err)
	}
	if metricsDailyResolved.PendingDailyPeriods != 0 {
		t.Fatalf("esperava PendingDailyPeriods = 0 após execução de hoje concluída, obteve %d", metricsDailyResolved.PendingDailyPeriods)
	}
}
