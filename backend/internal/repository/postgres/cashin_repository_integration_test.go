package postgres_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func cashinDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CATBRASIL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina CATBRASIL_TEST_DATABASE_URL para testes PostgreSQL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.CheckCurrent(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func cashinRunID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func seedCashInUser(t *testing.T, pool *pgxpool.Pool, id string, balance float64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, username, password_hash, balance, required_rollover, security_password)
		VALUES ($1, $1, 'hash', $2, 0, crypt('123456',gen_salt('bf',4)))
		ON CONFLICT (id) DO UPDATE SET balance=EXCLUDED.balance, required_rollover=0, withdraw_locked=FALSE`,
		id, balance)
	if err != nil {
		t.Fatal(err)
	}
}

func seedCashInIntent(t *testing.T, pool *pgxpool.Pool, id, userID string, amount float64, externalID string) {
	t.Helper()
	var external any
	if externalID != "" {
		external = externalID
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO transactions (id, user_id, tipo, valor, status, external_id, gateway_provider)
		VALUES ($1, $2, 'Depósito PIX', $3, 'pendente', $4, 'suitpay')`,
		id, userID, amount, external)
	if err != nil {
		t.Fatal(err)
	}
}

func cashinBalance(t *testing.T, pool *pgxpool.Pool, userID string) (float64, float64, bool) {
	t.Helper()
	var balance, rollover float64
	var locked bool
	if err := pool.QueryRow(context.Background(), `SELECT balance, required_rollover, withdraw_locked FROM users WHERE id=$1`, userID).Scan(&balance, &rollover, &locked); err != nil {
		t.Fatal(err)
	}
	return balance, rollover, locked
}

func cashinQueueCount(t *testing.T, pool *pgxpool.Pool, kind, externalID string) int {
	t.Helper()
	var count int
	query := `SELECT count(*) FROM cashin_event_queue WHERE kind=$1 AND queue_status='pending'`
	args := []any{kind}
	if externalID != "" {
		query += ` AND external_id=$2`
		args = append(args, externalID)
	}
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func cashinQueueID(t *testing.T, pool *pgxpool.Pool, kind, requestNumber string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM cashin_event_queue WHERE kind=$1 AND request_number=$2 AND queue_status='pending' ORDER BY id DESC LIMIT 1`, kind, requestNumber).Scan(&id); err != nil {
		t.Fatalf("item %s de %s não encontrado: %v", kind, requestNumber, err)
	}
	return id
}

func TestCashInConcurrentPaidOutCreditsOnce(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-conc-" + run
	intent := "CI-conc-" + run
	external := "ext-conc-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	var wg sync.WaitGroup
	results := make(chan string, 24)
	errs := make(chan error, 24)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 100)
			if err != nil {
				errs <- err
				return
			}
			results <- outcome
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("callback concorrente falhou: %v", err)
	}
	applied, duplicates := 0, 0
	for outcome := range results {
		switch outcome {
		case postgres.CashInApplied:
			applied++
		case postgres.CashInDuplicate:
			duplicates++
		default:
			t.Fatalf("resultado inesperado: %s", outcome)
		}
	}
	if applied != 1 || duplicates != 19 {
		t.Fatalf("esperado 1 applied e 19 duplicate; applied=%d duplicate=%d", applied, duplicates)
	}
	balance, rollover, _ := cashinBalance(t, pool, user)
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='deposit'`, intent).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if balance != 100 || rollover != 100 || events != 1 {
		t.Fatalf("crédito duplicado: balance=%v rollover=%v events=%d", balance, rollover, events)
	}
}

func TestCashInDuplicateCallbackHasNoEffect(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-dup-" + run
	intent := "CI-dup-" + run
	external := "ext-dup-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 42, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 42); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("primeiro callback: %s %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 42); err != nil || outcome != postgres.CashInDuplicate {
		t.Fatalf("callback duplicado: %s %v", outcome, err)
	}
	balance, _, _ := cashinBalance(t, pool, user)
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1`, intent).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if balance != 42 || events != 1 {
		t.Fatalf("duplicata gerou efeito: balance=%v events=%d", balance, events)
	}
}

func TestCashInChargebackReversesBalanceRolloverAndCommission(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	sponsor := "ci-sponsor-" + run
	guest := "ci-guest-" + run
	referral := "ci-ref-" + run
	intent := "CI-cb-" + run
	external := "ext-cb-" + run
	seedCashInUser(t, pool, sponsor, 0)
	seedCashInUser(t, pool, guest, 0)
	if _, err := pool.Exec(ctx, `INSERT INTO referrals (id, referrer_id, referred_user_id, nome, status) VALUES ($1,$2,$3,$3,'inativo')`, referral, sponsor, guest); err != nil {
		t.Fatal(err)
	}
	seedCashInIntent(t, pool, intent, guest, 1000, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 1000); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("crédito: %s %v", outcome, err)
	}
	if balance, rollover, _ := cashinBalance(t, pool, guest); balance != 1000 || rollover != 1000 {
		t.Fatalf("crédito incorreto: balance=%v rollover=%v", balance, rollover)
	}
	if balance, _, _ := cashinBalance(t, pool, sponsor); balance != 50 {
		t.Fatalf("comissão não paga: %v", balance)
	}

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 1000); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("chargeback: %s %v", outcome, err)
	}
	if balance, rollover, locked := cashinBalance(t, pool, guest); balance != 0 || rollover != 0 || locked {
		t.Fatalf("estorno incorreto: balance=%v rollover=%v locked=%v", balance, rollover, locked)
	}
	if balance, _, _ := cashinBalance(t, pool, sponsor); balance != 0 {
		t.Fatalf("comissão não estornada: %v", balance)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusCancelado {
		t.Fatalf("depósito não cancelado: %+v %v", deposit, err)
	}
	var commissionStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM transactions WHERE reference_id=$1`, intent).Scan(&commissionStatus); err != nil {
		t.Fatal(err)
	}
	if commissionStatus != string(domain.TxStatusCancelado) {
		t.Fatalf("transação de comissão ativa: %s", commissionStatus)
	}
	var reversals int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='deposit_chargeback'`, intent).Scan(&reversals); err != nil {
		t.Fatal(err)
	}
	if reversals != 1 {
		t.Fatalf("journal de chargeback ausente: %d", reversals)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 1000); err != nil || outcome != postgres.CashInDuplicate {
		t.Fatalf("chargeback duplicado: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, sponsor); balance != 0 {
		t.Fatalf("chargeback duplicado estornou de novo: %v", balance)
	}
}

func TestCashInOneCentActivatesReferralWithoutZeroCommission(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	sponsor, guest := "ci-tiny-sponsor-"+run, "ci-tiny-guest-"+run
	referral, intent, external := "ci-tiny-ref-"+run, "ci-tiny-intent-"+run, "ci-tiny-external-"+run
	seedCashInUser(t, pool, sponsor, 0)
	seedCashInUser(t, pool, guest, 0)
	if _, err := pool.Exec(ctx, `INSERT INTO referrals(id,referrer_id,referred_user_id,nome,status) VALUES($1,$2,$3,$3,'inativo')`, referral, sponsor, guest); err != nil {
		t.Fatal(err)
	}
	seedCashInIntent(t, pool, intent, guest, 0.01, "")
	repo := postgres.NewTransactionRepository(pool)
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 0.01); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback: %s %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 0.01); err != nil || outcome != postgres.CashInDuplicate {
		t.Fatalf("replay: %s %v", outcome, err)
	}
	var guestBalance, sponsorBalance, referralDeposit, status string
	var commissions, events int
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM users WHERE id=$1`, guest).Scan(&guestBalance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM users WHERE id=$1`, sponsor).Scan(&sponsorBalance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status,deposito::text FROM referrals WHERE id=$1`, referral).Scan(&status, &referralDeposit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE tipo='Comissão de primeiro depósito' AND reference_id=$1`, intent).Scan(&commissions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='deposit'`, intent).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if guestBalance != "0.01" || sponsorBalance != "0.00" || status != "ativo" || referralDeposit != "0.01" || commissions != 0 || events != 1 {
		t.Fatalf("cash-in mínimo inconsistente: guest=%s sponsor=%s referral=%s/%s commissions=%d events=%d", guestBalance, sponsorBalance, status, referralDeposit, commissions, events)
	}
}

func TestCashInChargebackInsufficientBalanceQueuesAndBlocks(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-poor-" + run
	intent := "CI-poor-" + run
	external := "ext-poor-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 100); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("crédito: %s %v", outcome, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET balance=10 WHERE id=$1`, user); err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 100); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("chargeback sem saldo: %s %v", outcome, err)
	}
	balance, _, locked := cashinBalance(t, pool, user)
	if balance != 10 || !locked {
		t.Fatalf("saldo negativo ou conta não bloqueada: balance=%v locked=%v", balance, locked)
	}
	if cashinQueueCount(t, pool, "chargeback_insufficient_balance", external) != 1 {
		t.Fatal("chargeback sem saldo não entrou na fila")
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusSucesso {
		t.Fatalf("depósito deveria seguir creditado: %+v %v", deposit, err)
	}
}

func TestCashInPrematureCallbackQueuedThenApplied(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-late-" + run
	intent := "CI-late-" + run
	external := "ext-late-" + run
	seedCashInUser(t, pool, user, 0)
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 30); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("callback antecipado: %s %v", outcome, err)
	}
	if cashinQueueCount(t, pool, "premature_callback", external) != 1 {
		t.Fatal("callback antecipado não entrou na fila")
	}
	seedCashInIntent(t, pool, intent, user, 30, "")
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 30); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("retomada pós-intenção: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 30 {
		t.Fatalf("crédito não aplicado após retomada: %v", balance)
	}
	if cashinQueueCount(t, pool, "premature_callback", external) != 0 {
		t.Fatal("fila não foi resolvida após aplicação")
	}
}

func TestCashInAmountMismatchNeverCredits(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-amt-" + run
	intent := "CI-amt-" + run
	externalA := "ext-amt-a-" + run
	externalB := "ext-amt-b-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, externalA, "PAID_OUT", 90); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("valor divergente da intenção: %s %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, externalA, "PAID_OUT", 100); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("valor conflitante com evento registrado: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("valor divergente creditou: %v", balance)
	}
	if cashinQueueCount(t, pool, "amount_conflict", "") == 0 {
		t.Fatal("conflito de valor não entrou na fila")
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, externalB, "PAID_OUT", 100); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("evento coerente: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 100 {
		t.Fatalf("evento coerente não creditou: %v", balance)
	}
}

func TestCashInExternalConflictQueued(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-ext-" + run
	intent := "CI-ext-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 50, "ext-known-"+run)
	repo := postgres.NewTransactionRepository(pool)

	external := "ext-unknown-" + run
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 50); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("external divergente: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("external divergente creditou: %v", balance)
	}
	if cashinQueueCount(t, pool, "external_conflict", external) != 1 {
		t.Fatal("conflito de external não entrou na fila")
	}
}

func TestCashInRequestConflictDoesNotRebind(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-reqconf-" + run
	intentA := "CI-reqa-" + run
	intentB := "CI-reqb-" + run
	external := "ext-reqconf-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intentA, user, 70, "")
	seedCashInIntent(t, pool, intentB, user, 70, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intentA, external, "PAID_OUT", 70); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("intenção A: %s %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intentB, external, "PAID_OUT", 70); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("conflito de request: %s %v", outcome, err)
	}
	if cashinQueueCount(t, pool, "request_conflict", external) != 1 {
		t.Fatal("conflito de request não entrou na fila")
	}
	b, err := repo.GetByID(ctx, intentB)
	if err != nil || b.Status != domain.TxStatusPendente || b.ExternalID != nil {
		t.Fatalf("intenção B foi rebindada: %+v %v", b, err)
	}
	var storedRequest string
	if err = pool.QueryRow(ctx, `SELECT request_number FROM deposit_gateway_events WHERE external_id=$1 AND status='PAID_OUT'`, external).Scan(&storedRequest); err != nil {
		t.Fatal(err)
	}
	if storedRequest != intentA {
		t.Fatalf("evento reatribuído silenciosamente: %s", storedRequest)
	}
}

func TestCashInChargebackBeforeCreditQueued(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-earlycb-" + run
	intent := "CI-earlycb-" + run
	external := "ext-earlycb-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 40, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 40); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("chargeback antes do crédito: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("chargeback antecipado alterou saldo: %v", balance)
	}
	if cashinQueueCount(t, pool, "chargeback_before_credit", external) != 1 {
		t.Fatal("chargeback antecipado não entrou na fila")
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 40); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("crédito após chargeback antecipado: %s %v", outcome, err)
	}
	dep, err := repo.GetByID(ctx, intent)
	if err != nil || dep.Status != domain.TxStatusCancelado {
		t.Fatalf("depósito após PAID_OUT com chargeback prévio deve ser cancelado: %+v %v", dep, err)
	}
	if balance, _, locked := cashinBalance(t, pool, user); balance != 0 || !locked {
		t.Fatalf("saldo deve ser 0 e saques devem continuar bloqueados: balance=%v locked=%v", balance, locked)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 40); err != nil || outcome != postgres.CashInDuplicate {
		t.Fatalf("replay de chargeback deve ser duplicate: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("saldo final incorreto: %v", balance)
	}
}

func TestCashInConcurrentPaidOutAndChargebackStaysConsistent(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-race-" + run
	intent := "CI-race-" + run
	external := "ext-race-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 20, "")
	repo := postgres.NewTransactionRepository(pool)

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	wg.Add(2)
	runEvent := func(status string) {
		defer wg.Done()
		_, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, status, 20)
		if err != nil {
			t.Errorf("%s: %v", status, err)
			return
		}
		mu.Lock()
		seen[status+"="+outcome] = true
		mu.Unlock()
	}
	go runEvent("PAID_OUT")
	go runEvent("CHARGEBACK")
	wg.Wait()
	if len(seen) == 0 {
		t.Fatal("nenhum resultado registrado")
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance < 0 {
		t.Fatalf("saldo negativo após corrida: %v", balance)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 20); err != nil ||
		(outcome != postgres.CashInApplied && outcome != postgres.CashInDuplicate) {
		t.Fatalf("replay do chargeback: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("saldo final não reconciliado: %v", balance)
	}
}

func TestCashInIntentQRAndAmbiguousReconciliation(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-intent-" + run
	intent := "CI-intent-" + run
	external := "ext-intent-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 10, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, err := repo.MarkDepositIntentSent(ctx, intent, external); err != nil {
		t.Fatalf("registro de intenção: %v", err)
	}
	var storedExternal string
	if err := pool.QueryRow(ctx, `SELECT external_id FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&storedExternal); err != nil || storedExternal != external {
		t.Fatalf("intenção não persistida: %q %v", storedExternal, err)
	}
	queued, err := repo.MarkDepositQRMissing(ctx, intent)
	if err != nil || !queued {
		t.Fatalf("QR ausente não registrado: %v %v", queued, err)
	}
	if err = repo.MarkDepositResultAmbiguous(ctx, intent, "timeout do provedor"); err != nil {
		t.Fatalf("resultado ambíguo: %v", err)
	}
	items, err := repo.ListCashInReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundQR, foundAmbiguous := false, false
	for _, item := range items {
		if item.RequestNumber != intent {
			continue
		}
		switch item.Kind {
		case "qr_missing":
			foundQR = true
		case "ambiguous_result":
			foundAmbiguous = true
		}
	}
	if !foundQR || !foundAmbiguous {
		t.Fatalf("reconciliação incompleta: qr=%v ambiguous=%v", foundQR, foundAmbiguous)
	}

	if _, err = repo.AttachDepositQRCode(ctx, intent, external, "CODE", "data:image/png;base64,AA"); err != nil {
		t.Fatalf("associação de QR: %v", err)
	}
	queued, err = repo.MarkDepositQRMissing(ctx, intent)
	if err != nil || queued {
		t.Fatalf("QR presente marcado como ausente: %v %v", queued, err)
	}
	items, err = repo.ListCashInReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.RequestNumber == intent && item.Kind == "qr_missing" {
			t.Fatal("item de QR não resolvido após associação")
		}
	}

	queueID := cashinQueueID(t, pool, "ambiguous_result", intent)
	if err = repo.ResolveCashInQueueItem(ctx, queueID, "operador-1", "ticket-123", "tratado manualmente"); err == nil {
		t.Fatal("resultado ambíguo encerrado manualmente sem prova do provedor")
	}
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM cashin_event_queue WHERE id=$1 AND queue_status='pending'`, queueID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatal("item ambíguo deveria permanecer pendente")
	}
}

func TestCashInResolvedIntentNotReported(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-resolved-" + run
	intent := "CI-resolved-" + run
	external := "ext-resolved-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 55, "")
	repo := postgres.NewTransactionRepository(pool)

	if err := repo.MarkDepositResultAmbiguous(ctx, intent, "timeout"); err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 55); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("crédito pós-ambiguidade: %s %v", outcome, err)
	}
	var ambiguous bool
	if err := pool.QueryRow(ctx, `SELECT result_ambiguous FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&ambiguous); err != nil {
		t.Fatal(err)
	}
	if ambiguous {
		t.Fatal("result_ambiguous não foi zerado após reconciliação")
	}
	var pendingIntent int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cashin_event_queue WHERE request_number=$1 AND queue_status='pending'`, intent).Scan(&pendingIntent); err != nil {
		t.Fatal(err)
	}
	if pendingIntent != 0 {
		t.Fatalf("pendências de intenção não resolvidas: %d", pendingIntent)
	}
	items, err := repo.ListCashInReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.RequestNumber == intent {
			t.Fatalf("intenção reconciliada ainda reportada: %+v", item)
		}
	}
}

func TestCashInCallbackWithoutRequestNumberResolvesByExternal(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-noreq-" + run
	intent := "CI-noreq-" + run
	external := "ext-noreq-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 65, external)
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, "", external, "PAID_OUT", 65); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback sem requestNumber: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 65 {
		t.Fatalf("callback sem requestNumber não creditou: %v", balance)
	}
	ghost := "ext-ghost-" + run
	if _, outcome, err := repo.ApplyDepositEvent(ctx, "", ghost, "PAID_OUT", 10); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("external desconhecido deveria enfileirar: %s %v", outcome, err)
	}
	if cashinQueueCount(t, pool, "premature_callback", ghost) != 1 {
		t.Fatal("callback sem request e external desconhecido foi descartado sem trilha")
	}
}

func TestCashInNonFinalStatusQueued(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-nonfinal-" + run
	intent := "CI-nonfinal-" + run
	external := "ext-nonfinal-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	for _, status := range []string{"CANCELED", "EXPIRED", "UNPAID", "PENDING", "WEIRD_STATUS"} {
		if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, status, 0); err != nil || outcome != postgres.CashInQueued {
			t.Fatalf("status %s deveria enfileirar: %s %v", status, outcome, err)
		}
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("status não final creditou: %v", balance)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusPendente {
		t.Fatalf("status não final alterou depósito: %+v %v", deposit, err)
	}
	var events int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM deposit_gateway_events WHERE external_id=$1`, external).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 5 {
		t.Fatalf("eventos não finais sem trilha durável: %d", events)
	}
	if cashinQueueCount(t, pool, "non_final_status", external) != 5 {
		t.Fatalf("fila de status não final incompleta: %d", cashinQueueCount(t, pool, "non_final_status", external))
	}
}

func TestCashInInvalidAmountStillLeavesTrail(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	external := "ext-badamount-" + run
	intent := "CI-badamount-" + run
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 0); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("valor zero deveria enfileirar com trilha: %s %v", outcome, err)
	}
	if cashinQueueCount(t, pool, "amount_invalid", external) != 1 {
		t.Fatal("valor inválido não deixou trilha")
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external+".2", "PAID_OUT", 10.005); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("valor fracionado deveria enfileirar: %s %v", outcome, err)
	}
}

func TestCashInRejectsUnpersistableEvents(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, "", "", "PAID_OUT", 10); err == nil || outcome != "" {
		t.Fatalf("evento sem identificadores aceito: %q %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, "", "", "CANCELED", 0); err == nil || outcome != "" {
		t.Fatalf("evento sem identificadores aceito: %q %v", outcome, err)
	}
}

func TestCashInSubmissionIssueAndStarted(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-issue-" + run
	intent := "CI-issue-" + run
	external := "ext-issue-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 80, "")
	repo := postgres.NewTransactionRepository(pool)

	if err := repo.MarkDepositSubmissionStarted(ctx, intent); err != nil {
		t.Fatalf("submission started: %v", err)
	}
	var intentSent bool
	var ext *string
	if err := pool.QueryRow(ctx, `SELECT intent_sent_at IS NOT NULL, external_id FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&intentSent, &ext); err != nil {
		t.Fatal(err)
	}
	if !intentSent || ext != nil {
		t.Fatalf("intenção inicial mal registrada: sent=%v external=%v", intentSent, ext)
	}
	if err := repo.MarkDepositSubmissionIssue(ctx, intent, "timeout de conexão", external); err != nil {
		t.Fatalf("submission issue: %v", err)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusProcessando {
		t.Fatalf("issue não marcou processando: %+v %v", deposit, err)
	}
	if cashinQueueCount(t, pool, "submission_issue", external) != 1 {
		t.Fatal("issue não entrou na fila")
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 80); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback após issue: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 80 {
		t.Fatalf("crédito não aplicado: %v", balance)
	}
}

func TestCashInSubmissionIssueAfterCallbackDoesNotReopen(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-lateissue-" + run
	intent := "CI-lateissue-" + run
	external := "ext-lateissue-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 90, "")
	repo := postgres.NewTransactionRepository(pool)
	if err := repo.MarkDepositSubmissionStarted(ctx, intent); err != nil {
		t.Fatalf("submission started: %v", err)
	}

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 90); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("crédito: %s %v", outcome, err)
	}
	if err := repo.MarkDepositSubmissionIssue(ctx, intent, "timeout tardio", external); err != nil {
		t.Fatalf("issue tardio: %v", err)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusSucesso {
		t.Fatalf("issue tardio reverteu o sucesso: %+v %v", deposit, err)
	}
	var ambiguous bool
	if err = pool.QueryRow(ctx, `SELECT result_ambiguous FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&ambiguous); err != nil {
		t.Fatal(err)
	}
	if ambiguous {
		t.Fatal("issue tardio reabriu ambiguidade após sucesso")
	}
	if cashinQueueCount(t, pool, "submission_issue", external) != 0 {
		t.Fatal("issue tardio criou pendência após sucesso")
	}
	if err = repo.MarkDepositSubmissionStarted(ctx, intent); err != nil {
		t.Fatalf("started tardio: %v", err)
	}
	if cashinQueueCount(t, pool, "submission_issue", external) != 0 {
		t.Fatal("started tardio criou pendência após sucesso")
	}
}

func TestCashInValidPixCodeIsNotMissingQR(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-pixcode-" + run
	intent := "CI-pixcode-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 15, "")
	if _, err := pool.Exec(ctx, `UPDATE transactions SET pix_code='000201BR.GOV.BCB.PIX...' WHERE id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	queued, err := repo.MarkDepositQRMissing(ctx, intent)
	if err != nil || queued {
		t.Fatalf("código PIX válido foi tratado como QR ausente: %v %v", queued, err)
	}
}

func TestCashInResolveRequiresEvidenceAndRejectsFinancial(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-resolve-" + run
	qrIntent := "CI-resolveqr-" + run
	amtIntent := "CI-resolveamt-" + run
	external := "ext-resolve-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, qrIntent, user, 10, "")
	seedCashInIntent(t, pool, amtIntent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, err := repo.MarkDepositQRMissing(ctx, qrIntent); err != nil {
		t.Fatal(err)
	}
	qrQueue := cashinQueueID(t, pool, "qr_missing", qrIntent)
	if err := repo.ResolveCashInQueueItem(ctx, qrQueue, "", "ticket-1", "ok"); err == nil {
		t.Fatal("resolução sem operador aceita")
	}
	if err := repo.ResolveCashInQueueItem(ctx, qrQueue, "operador-1", "", "ok"); err == nil {
		t.Fatal("resolução sem evidência aceita")
	}
	if err := repo.ResolveCashInQueueItem(ctx, qrQueue, "operador-1", "ticket-1", ""); err == nil {
		t.Fatal("resolução sem razão aceita")
	}
	if err := repo.ResolveCashInQueueItem(ctx, qrQueue, "operador-1", "ticket-1", "confirmado no extrato"); err != nil {
		t.Fatalf("resolução válida rejeitada: %v", err)
	}

	if _, outcome, err := repo.ApplyDepositEvent(ctx, amtIntent, external, "PAID_OUT", 90); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("conflito de valor: %s %v", outcome, err)
	}
	amtQueue := cashinQueueID(t, pool, "amount_conflict", amtIntent)
	if err := repo.ResolveCashInQueueItem(ctx, amtQueue, "operador-1", "ticket-2", "fechado"); err == nil {
		t.Fatal("item financeiro foi encerrado por botão")
	}

	// Ambiguous and unconfirmed intents must never be closable by assumption.
	ambIntent := "CI-resolveamb-" + run
	seedCashInIntent(t, pool, ambIntent, user, 20, "")
	if err := repo.MarkDepositResultAmbiguous(ctx, ambIntent, "timeout"); err != nil {
		t.Fatal(err)
	}
	ambQueue := cashinQueueID(t, pool, "ambiguous_result", ambIntent)
	if err := repo.ResolveCashInQueueItem(ctx, ambQueue, "operador-1", "ticket-3", "achei que cancelou"); err == nil {
		t.Fatal("resultado ambíguo encerrado manualmente sem prova oficial")
	}
	for _, nonResolvable := range []string{"intent_sent_unconfirmed", "orphan_deposit", "missing_external_id", "submission_issue"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO cashin_event_queue (dedup_key, kind, reason) VALUES ($1, $2, 'teste') RETURNING id`, "test-"+nonResolvable+"-"+run, nonResolvable).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := repo.ResolveCashInQueueItem(ctx, id, "operador-1", "ticket-x", "por suposição"); err == nil {
			t.Fatalf("item %s não deveria ser resolvível manualmente", nonResolvable)
		}
	}
}

func TestCashInPaidStatusNeverCredits(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-paid-" + run
	intent := "CI-paid-" + run
	external := "ext-paid-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID", 100); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("status PAID deveria enfileirar sem creditar: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("status PAID moveu saldo: %v", balance)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusPendente {
		t.Fatalf("status PAID alterou o depósito: %+v %v", deposit, err)
	}
	if cashinQueueCount(t, pool, "non_final_status", external) != 1 {
		t.Fatal("status PAID não foi enfileirado como não final")
	}
	var storedStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM deposit_gateway_events WHERE external_id=$1`, external).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != "PAID" {
		t.Fatalf("evento PAID não preservado como recebido: %s", storedStatus)
	}
}

func TestCashInMoneyStatusWithoutExternalIDDoesNotMoveMoney(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-noext-" + run
	intent := "CI-noext-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, "", "PAID_OUT", 100); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("PAID_OUT sem external deveria enfileirar: %s %v", outcome, err)
	}
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, "", "CHARGEBACK", 100); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("CHARGEBACK sem external deveria enfileirar: %s %v", outcome, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 0 {
		t.Fatalf("status financeiro sem external moveu saldo: %v", balance)
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusPendente {
		t.Fatalf("depósito alterado sem external: %+v %v", deposit, err)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cashin_event_queue WHERE kind='missing_external_id' AND request_number=$1 AND queue_status='pending'`, intent).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 2 {
		t.Fatalf("esperado 2 itens missing_external_id, obtido %d", stored)
	}
}

func TestCashInQueuedEventVisibleThroughAdminReconciliation(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-adminrecon-" + run
	ghost := "ext-adminrecon-" + run
	seedCashInUser(t, pool, user, 0)
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, "CI-unknown-"+run, ghost, "PAID_OUT", 12); err != nil || outcome != postgres.CashInQueued {
		t.Fatalf("callback sem intenção: %s %v", outcome, err)
	}

	issues, err := repo.ListFinancialReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range issues {
		if issue.Kind == "cashin_premature_callback" && issue.ExternalID == ghost {
			found = true
			if issue.QueueID == 0 {
				t.Fatal("item de cash-in reportado sem queue id")
			}
			if issue.Reason == "" {
				t.Fatal("item de cash-in reportado sem razão")
			}
		}
	}
	if !found {
		t.Fatalf("evento enfileirado não apareceu no método administrativo: %+v", issues)
	}
}

func TestCashInOrphanPendingDepositAppearsAfterThreshold(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-orphan-" + run
	orphan := "CI-orphan-" + run
	fresh := "CI-orphan-fresh-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, orphan, user, 25, "")
	seedCashInIntent(t, pool, fresh, user, 25, "")
	if _, err := pool.Exec(ctx, `UPDATE transactions SET created_at=now()-interval '6 minutes' WHERE id=$1`, orphan); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)

	items, err := repo.ListCashInReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundOrphan, foundFresh := false, false
	for _, item := range items {
		if item.RequestNumber == orphan && item.Kind == "orphan_deposit" {
			foundOrphan = true
		}
		if item.RequestNumber == fresh {
			foundFresh = true
		}
	}
	if !foundOrphan {
		t.Fatal("depósito pendente órfão (>5min, sem QR, sem tracker) não apareceu na conciliação")
	}
	if foundFresh {
		t.Fatal("depósito recente reportado antes do limiar de 5 minutos")
	}
	deposit, err := repo.GetByID(ctx, orphan)
	if err != nil || deposit.Status != domain.TxStatusPendente {
		t.Fatalf("listagem cancelou o depósito órfão: %+v %v", deposit, err)
	}
}

func TestCashInLateIntentSentAfterSettlementReconciles(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-lateintent-" + run
	intent := "CI-lateintent-" + run
	external := "ext-lateintent-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 70, "")
	repo := postgres.NewTransactionRepository(pool)

	// The HTTP charge response arrives only after the callback credited the
	// transaction. The intent must be recorded as reconciled, never reopened.
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 70); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback antes da resposta HTTP: %s %v", outcome, err)
	}
	if _, err := repo.MarkDepositIntentSent(ctx, intent, external); err != nil {
		t.Fatalf("MarkDepositIntentSent após liquidação: %v", err)
	}

	var reconciled bool
	var storedExternal string
	if err := pool.QueryRow(ctx, `SELECT reconciled_at IS NOT NULL, COALESCE(external_id,'') FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&reconciled, &storedExternal); err != nil {
		t.Fatalf("intenção não registrada: %v", err)
	}
	if !reconciled {
		t.Fatal("intenção pós-liquidação não ficou reconciliada")
	}
	if storedExternal != external {
		t.Fatalf("externalID creditado não preservado: %s", storedExternal)
	}
	var pending int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cashin_event_queue WHERE request_number=$1 AND queue_status='pending' AND kind IN ('qr_missing','ambiguous_result','intent_sent_unconfirmed')`, intent).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("resposta tardia reabriu pendência: %d", pending)
	}
	items, err := repo.ListCashInReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.RequestNumber == intent {
			t.Fatalf("intenção reconciliada ainda reportada: %+v", item)
		}
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusSucesso || deref(deposit.ExternalID) != external {
		t.Fatalf("depósito creditado alterado: %+v %v", deposit, err)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 70 {
		t.Fatalf("saldo alterado por resposta tardia: %v", balance)
	}
}

func TestCashInLateIntentSentConflictingExternalIsDurable(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-lateconflict-" + run
	intent := "CI-lateconflict-" + run
	credited := "ext-late-credited-" + run
	other := "ext-late-other-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 40, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, credited, "PAID_OUT", 40); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback antes da resposta HTTP: %s %v", outcome, err)
	}
	// The late HTTP response carries a different external id: it must be queued
	// as a durable conflict and surfaced as an error, never overwritten.
	if _, err := repo.MarkDepositIntentSent(ctx, intent, other); err == nil {
		t.Fatal("externalID conflitante aceito silenciosamente")
	}
	if cashinQueueCount(t, pool, "external_conflict", credited) != 1 {
		t.Fatal("conflito de external não registrado na fila")
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if deposit.ExternalID == nil || *deposit.ExternalID != credited {
		t.Fatalf("externalID creditado foi substituído: %v", deposit.ExternalID)
	}
	var storedExternal string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(external_id,'') FROM cashin_deposit_intents WHERE request_number=$1`, intent).Scan(&storedExternal); err != nil {
		t.Fatal(err)
	}
	if storedExternal != credited {
		t.Fatalf("intent externalID substituído: %s", storedExternal)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo alterado: %v", balance)
	}
}

func TestCashInLateSubmissionIssueConflictingExternalIsDurable(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-lateissueconf-" + run
	intent := "CI-lateissueconf-" + run
	credited := "ext-lic-credited-" + run
	other := "ext-lic-other-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 55, "")
	repo := postgres.NewTransactionRepository(pool)

	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, credited, "PAID_OUT", 55); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("callback antes do erro HTTP: %s %v", outcome, err)
	}
	if err := repo.MarkDepositSubmissionIssue(ctx, intent, "timeout tardio", other); err != nil {
		t.Fatalf("ocorrência tardia deveria registrar sem erro: %v", err)
	}
	if cashinQueueCount(t, pool, "external_conflict", credited) != 1 {
		t.Fatal("discrepância de external não registrada na fila")
	}
	if cashinQueueCount(t, pool, "submission_issue", credited) != 0 {
		t.Fatal("ocorrência tardia não deveria reabrir submission_issue após liquidação")
	}
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil || deposit.Status != domain.TxStatusSucesso {
		t.Fatalf("depósito liquidado alterado: %+v %v", deposit, err)
	}
	if deposit.ExternalID == nil || *deposit.ExternalID != credited {
		t.Fatalf("externalID creditado foi substituído: %v", deposit.ExternalID)
	}
	if balance, _, _ := cashinBalance(t, pool, user); balance != 55 {
		t.Fatalf("saldo alterado: %v", balance)
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestCashInEarlyChargebackThenPaidOutACID(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-ecb-" + run
	intent := "CI-ecb-" + run
	external := "ext-ecb-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 100, external)
	repo := postgres.NewTransactionRepository(pool)

	// 1. CHARGEBACK arrives first for intent in 'pendente' status
	_, outcomeCB, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 100)
	if err != nil {
		t.Fatalf("erro no chargeback antecipado: %v", err)
	}
	if outcomeCB != postgres.CashInQueued {
		t.Fatalf("esperado outcome queued para chargeback antecipado, obteve %s", outcomeCB)
	}

	// Assert user withdrawals are locked
	balance, rollover, locked := cashinBalance(t, pool, user)
	if !locked {
		t.Fatal("chargeback antecipado DEVE bloquear saques do usuário")
	}
	if balance != 0 || rollover != 0 {
		t.Fatalf("saldo ou rollover alterado prematuramente: balance=%.2f rollover=%.2f", balance, rollover)
	}
	if cashinQueueCount(t, pool, "chargeback_before_credit", external) != 1 {
		t.Fatal("esperado 1 item em cashin_event_queue kind chargeback_before_credit")
	}

	// 2. PAID_OUT arrives for the same intent
	txPaid, outcomePaid, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 100)
	if err != nil {
		t.Fatalf("erro no PAID_OUT subsequente: %v", err)
	}
	if outcomePaid != postgres.CashInApplied {
		t.Fatalf("esperado outcome applied, obteve %s", outcomePaid)
	}
	if txPaid == nil || txPaid.Status != domain.TxStatusCancelado {
		t.Fatalf("status final deve ser cancelado, obteve: %+v", txPaid)
	}

	// Assert net balance and rollover are zero
	balance, rollover, locked = cashinBalance(t, pool, user)
	if balance != 0 || rollover != 0 {
		t.Fatalf("saldo líquido ou rollover diferente de zero: balance=%.2f rollover=%.2f", balance, rollover)
	}

	// Assert user withdrawals remain locked (never automatically cleared)
	if !locked {
		t.Fatal("saques do usuário NÃO devem ser desbloqueados automaticamente")
	}

	// Assert final transaction status in DB is 'cancelado'
	deposit, err := repo.GetByID(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if deposit.Status != domain.TxStatusCancelado {
		t.Fatalf("status da transação no banco deve ser cancelado, obteve %s", deposit.Status)
	}

	// Assert 2 ledger rows in financial_events: 1 deposit (+100), 1 deposit_chargeback (-100)
	var depositCount, cbCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='deposit'`, intent).Scan(&depositCount)
	if err != nil || depositCount != 1 {
		t.Fatalf("esperada 1 linha deposit no livro-razão: count=%d err=%v", depositCount, err)
	}
	err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='deposit_chargeback'`, intent).Scan(&cbCount)
	if err != nil || cbCount != 1 {
		t.Fatalf("esperada 1 linha deposit_chargeback no livro-razão: count=%d err=%v", cbCount, err)
	}

	// Assert pending chargeback queue item was resolved
	if cashinQueueCount(t, pool, "chargeback_before_credit", external) != 0 {
		t.Fatal("fila de chargeback_before_credit deveria ter sido resolvida após a reversão")
	}
}

func TestCashInConcurrentPaidOutAndChargebackACID(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-concord-" + run
	intent := "CI-concord-" + run
	external := "ext-concord-" + run
	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, 120, external)
	repo := postgres.NewTransactionRepository(pool)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _ = repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 120)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 120)
	}()
	wg.Wait()

	// In either order: both events serialize under advisory lock.
	// Net balance must be 0, net rollover must be 0, final status 'cancelado'.
	balance, rollover, _ := cashinBalance(t, pool, user)
	if balance != 0 || rollover != 0 {
		t.Fatalf("saldo ou rollover corrompido sob concorrência: balance=%.2f rollover=%.2f", balance, rollover)
	}

	deposit, err := repo.GetByID(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if deposit.Status != domain.TxStatusCancelado {
		t.Fatalf("status final sob concorrência deve ser cancelado, obteve: %s", deposit.Status)
	}

	// Exactly 2 ledger rows in financial_events
	var totalLedger int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1`, intent).Scan(&totalLedger); err != nil || totalLedger != 2 {
		t.Fatalf("esperadas 2 linhas no razão sob concorrência, obteve %d (err: %v)", totalLedger, err)
	}
}

func TestCashInPreexistingWithdrawLockedPreserved(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-lockpres-" + run
	intent := "CI-lockpres-" + run
	external := "ext-lockpres-" + run
	seedCashInUser(t, pool, user, 100)
	// Manually set withdraw_locked = TRUE (e.g. by security / admin / KYC)
	if _, err := pool.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE WHERE id=$1`, user); err != nil {
		t.Fatal(err)
	}
	seedCashInIntent(t, pool, intent, user, 50, external)
	repo := postgres.NewTransactionRepository(pool)

	// 1. Normal PAID_OUT
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", 50); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("falha no PAID_OUT: %s %v", outcome, err)
	}
	// Verify withdraw_locked is still TRUE
	_, _, locked := cashinBalance(t, pool, user)
	if !locked {
		t.Fatal("PAID_OUT não pode limpar withdraw_locked pré-existente!")
	}

	// 2. Normal CHARGEBACK
	if _, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", 50); err != nil || outcome != postgres.CashInApplied {
		t.Fatalf("falha no CHARGEBACK: %s %v", outcome, err)
	}
	// Verify withdraw_locked is STILL TRUE
	_, _, locked = cashinBalance(t, pool, user)
	if !locked {
		t.Fatal("CHARGEBACK não pode limpar withdraw_locked pré-existente!")
	}
}

func TestCashInPaidOutOnLocallyCancelledIntentQueuedAsInvalidState(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-canc-" + run
	intent := "CI-canc-" + run
	external := "ext-canc-" + run
	amount := 75.00

	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, amount, external)
	// Cancel intent locally without any financial events having occurred
	if _, err := pool.Exec(ctx, `UPDATE transactions SET status='cancelado' WHERE id=$1`, intent); err != nil {
		t.Fatal(err)
	}

	repo := postgres.NewTransactionRepository(pool)
	// Receive real PAID_OUT for locally cancelled deposit
	_, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "PAID_OUT", amount)
	if err != nil {
		t.Fatalf("ApplyDepositEvent retornou erro inesperado: %v", err)
	}
	if outcome != postgres.CashInQueued {
		t.Fatalf("PAID_OUT em transação cancelada sem razão deve ser 'queued', obteve: %s", outcome)
	}

	// User balance must NOT be credited
	balance, _, _ := cashinBalance(t, pool, user)
	if balance != 0 {
		t.Fatalf("saldo creditado indevidamente para transação cancelada: %v", balance)
	}

	// Must be durably queued as invalid_state with queue_status='pending'
	var queueCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM cashin_event_queue
		WHERE request_number=$1 AND external_id=$2 AND kind='invalid_state' AND queue_status='pending'`,
		intent, external).Scan(&queueCount)
	if err != nil || queueCount != 1 {
		t.Fatalf("evento não foi enfileirado duravelmente como invalid_state: count=%d err=%v", queueCount, err)
	}
}

func TestCashInEarlyChargebackDifferentExternalMismatchDoesNotReverse(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-mismatch-" + run
	intent := "CI-mismatch-" + run
	extLegit := "ext-legit-" + run
	extMismatch := "ext-mismatch-" + run
	amount := 90.00

	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, amount, extLegit)

	// Simulate an early chargeback queued with a DIFFERENT externalID on this request
	repo := postgres.NewTransactionRepository(pool)
	_, outcomeCB, err := repo.ApplyDepositEvent(ctx, intent, extMismatch, "CHARGEBACK", amount)
	if err != nil || outcomeCB != postgres.CashInQueued {
		t.Fatalf("falha ao enfileirar chargeback com external divergente: outcome=%s err=%v", outcomeCB, err)
	}

	// Verify extMismatch is pending in cashin_event_queue
	var mismatchPending int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM cashin_event_queue WHERE external_id=$1 AND queue_status='pending'`, extMismatch).Scan(&mismatchPending); err != nil || mismatchPending != 1 {
		t.Fatalf("chargeback com external divergente deveria estar pending: %d (err: %v)", mismatchPending, err)
	}

	// Now legitimate PAID_OUT arrives with extLegit
	_, outcomePaid, err := repo.ApplyDepositEvent(ctx, intent, extLegit, "PAID_OUT", amount)
	if err != nil || outcomePaid != postgres.CashInApplied {
		t.Fatalf("PAID_OUT legítimo deveria ser applied: outcome=%s err=%v", outcomePaid, err)
	}

	// User should have received legitimate credit (not reversed by extMismatch!)
	balance, _, _ := cashinBalance(t, pool, user)
	if balance != amount {
		t.Fatalf("crédito legítimo não foi aplicado ou foi revertido erroneamente por external divergente: balance=%.2f esperado=%.2f", balance, amount)
	}

	// Transaction status should be 'sucesso'
	dep, err := repo.GetByID(ctx, intent)
	if err != nil || dep.Status != domain.TxStatusSucesso {
		t.Fatalf("transação legítima deveria estar em sucesso, obteve: %+v (err: %v)", dep, err)
	}

	// Queued item for extMismatch MUST STILL BE PENDING (not closed by resolveChargebackPendencies!)
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM cashin_event_queue WHERE external_id=$1 AND queue_status='pending'`, extMismatch).Scan(&mismatchPending); err != nil || mismatchPending != 1 {
		t.Fatalf("item divergente em cashin_event_queue não deve ser fechado por PAID_OUT de outro external_id! count=%d", mismatchPending)
	}
}

func TestCashInChargebackOnLocallyCancelledIntentQueuedAsInvalidState(t *testing.T) {
	pool := cashinDB(t)
	ctx := context.Background()
	run := cashinRunID()
	user := "ci-canccb-" + run
	intent := "CI-canccb-" + run
	external := "ext-canccb-" + run
	amount := 60.00

	seedCashInUser(t, pool, user, 0)
	seedCashInIntent(t, pool, intent, user, amount, external)
	// Cancel intent locally without any financial events having occurred
	if _, err := pool.Exec(ctx, `UPDATE transactions SET status='cancelado' WHERE id=$1`, intent); err != nil {
		t.Fatal(err)
	}

	repo := postgres.NewTransactionRepository(pool)
	// Receive CHARGEBACK for locally cancelled deposit
	_, outcome, err := repo.ApplyDepositEvent(ctx, intent, external, "CHARGEBACK", amount)
	if err != nil {
		t.Fatalf("ApplyDepositEvent retornou erro inesperado: %v", err)
	}
	if outcome != postgres.CashInQueued {
		t.Fatalf("CHARGEBACK em transação cancelada sem razão deve ser 'queued', obteve: %s", outcome)
	}

	// User balance must remain untouched (0) and withdrawals must be locked
	balance, _, locked := cashinBalance(t, pool, user)
	if balance != 0 {
		t.Fatalf("saldo alterado indevidamente: %v", balance)
	}
	if !locked {
		t.Fatal("saques do usuário devem ser bloqueados sob chargeback de transação cancelada")
	}

	// Must be durably queued as invalid_state with queue_status='pending'
	var queueCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM cashin_event_queue
		WHERE request_number=$1 AND external_id=$2 AND kind='invalid_state' AND queue_status='pending'`,
		intent, external).Scan(&queueCount)
	if err != nil || queueCount != 1 {
		t.Fatalf("evento não foi enfileirado duravelmente como invalid_state pendente: count=%d err=%v", queueCount, err)
	}

	// Zero entries in financial_events
	var ledgerCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM financial_events WHERE transaction_id=$1`, intent).Scan(&ledgerCount); err != nil || ledgerCount != 0 {
		t.Fatalf("razão financeiro não pode conter entradas: count=%d err=%v", ledgerCount, err)
	}
}
