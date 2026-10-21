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
	"catbrasil/backend/pkg/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

func financialDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("CATBRASIL_TEST_DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("defina TEST_DATABASE_URL (ou CATBRASIL_TEST_DATABASE_URL) para testes PostgreSQL")
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

func seedFinancialUser(t *testing.T, pool *pgxpool.Pool, id string, balance float64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO users (id,username,password_hash,balance,security_password) VALUES ($1,$1,'hash',$2,crypt('123456',gen_salt('bf',4)))`, id, balance)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentDepositCallbackCreditsOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "deposit-user", 0)
	_, err := pool.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,external_id,gateway_provider) VALUES ('PIX-test','deposit-user','Depósito PIX',100,'pendente','SIM-PIX-test','simulated')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	errors := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.ConfirmDeposit(ctx, "PIX-test", 100, "simulated", "SIM-PIX-test")
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var balance, rollover float64
	var events int
	if err = pool.QueryRow(ctx, `SELECT balance,required_rollover FROM users WHERE id='deposit-user'`).Scan(&balance, &rollover); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id='PIX-test'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if balance != 100 || rollover != 100 || events != 1 {
		t.Fatalf("crédito duplicado: balance=%v rollover=%v events=%d", balance, rollover, events)
	}
}

func TestDatabaseMaximumCentDepositPreservesExactLedger(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "max-cent-user", 0)
	if _, err := pool.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,external_id,gateway_provider)
		VALUES('PIX-max-cent','max-cent-user','Depósito PIX',9999999999999.99,'pendente','SIM-max-cent','simulated')`); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	if _, _, err := repo.ConfirmDeposit(ctx, "PIX-max-cent", 9_999_999_999_999.99, "simulated", "SIM-max-cent"); err != nil {
		t.Fatal(err)
	}
	var balance, amount, after string
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM users WHERE id='max-cent-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT amount::text,balance_after::text FROM financial_events WHERE transaction_id='PIX-max-cent' AND kind='deposit'`).Scan(&amount, &after); err != nil {
		t.Fatal(err)
	}
	if balance != "9999999999999.99" || amount != balance || after != balance {
		t.Fatalf("centavo máximo perdido: balance=%s amount=%s after=%s", balance, amount, after)
	}
}

func TestCashInCallbackBeforeQRCodeAssociation(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "early-cashin-user", 0)
	_, err := pool.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider) VALUES ('PIX-early-cashin','early-cashin-user','Depósito PIX',25,'pendente','suitpay')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	var callbackErr, attachErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, callbackErr = repo.ConfirmDeposit(ctx, "PIX-early-cashin", 25, "suitpay", "gateway-early-1")
	}()
	go func() {
		defer wg.Done()
		_, attachErr = repo.AttachDepositQRCode(ctx, "PIX-early-cashin", "gateway-early-1", "STAGING-ONLY-code", "data:image/png;base64,AA")
	}()
	wg.Wait()
	if callbackErr != nil || attachErr != nil {
		t.Fatalf("callback=%v, attach=%v", callbackErr, attachErr)
	}
	deposit, err := repo.GetByID(ctx, "PIX-early-cashin")
	if err != nil || deposit.Status != domain.TxStatusSucesso || deposit.ExternalID == nil || *deposit.ExternalID != "gateway-early-1" {
		t.Fatalf("depósito inconciliado: %+v, %v", deposit, err)
	}
	var balance float64
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='early-cashin-user'`).Scan(&balance); err != nil || balance != 25 {
		t.Fatalf("saldo=%v, erro=%v", balance, err)
	}
	if _, err := repo.AttachDepositQRCode(ctx, "PIX-early-cashin", "gateway-wrong", "bad", "bad"); err == nil {
		t.Fatal("ID externo divergente aceito")
	}
}

func TestConcurrentPayoutReservationAndIdempotency(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "payout-user", 100)
	repo := postgres.NewTransactionRepository(pool)
	pix := "user@example.com"
	var wg sync.WaitGroup
	var success int
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: fmt.Sprintf("WDR-%d", i), UserID: "payout-user", Tipo: "Saque PIX", Valor: -80, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "simulated"}, nil)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			} else if err != domain.ErrInsufficientBalance {
				t.Errorf("erro inesperado: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("esperado 1 saque reservado; obtidos %d", success)
	}
	var balance float64
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='payout-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 20 {
		t.Fatalf("saldo após corrida=%v", balance)
	}
	seedFinancialUser(t, pool, "idempotent-user", 100)
	key := "same-key"
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: fmt.Sprintf("WDR-repeat-%d", i), UserID: "idempotent-user", Tipo: "Saque PIX", Valor: -60, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "simulated"}, nil)
			if err != nil {
				t.Errorf("repetição falhou: %v", err)
				return
			}
			ids <- got.ID
		}(i)
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatalf("IDs diferentes para mesma chave: %s vs %s", first, id)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='idempotent-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 40 {
		t.Fatalf("replay debitou duas vezes: %v", balance)
	}
}

func TestPayoutClaimRejectAndCallbackTransitions(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "state-user", 200)
	repo := postgres.NewTransactionRepository(pool)
	pix, key := "user@example.com", "state-key"
	_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: "WDR-state", UserID: "state-user", Tipo: "Saque PIX", Valor: -100, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "suitpay"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimPayout(ctx, "WDR-state")
	if err != nil || claimed.Status != domain.TxStatusProcessando {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if _, err := repo.ClaimPayout(ctx, "WDR-state"); err == nil {
		t.Fatal("envio duplicado aceito")
	}
	if _, err := repo.RejectWithdrawal(ctx, "WDR-state", "admin", false); err == nil {
		t.Fatal("estorno de saque enviado aceito")
	}
	if _, err := repo.RecordPayoutSubmission(ctx, "WDR-state", "provider-1"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = repo.ConfirmPayout(ctx, "WDR-state", "provider-1", 95) }()
	go func() { defer wg.Done(); _, _ = repo.CancelPayout(ctx, "WDR-state", "provider-1", 95) }()
	wg.Wait()
	final, err := repo.GetByID(ctx, "WDR-state")
	if err != nil {
		t.Fatal(err)
	}
	var balance float64
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='state-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if (final.Status == domain.TxStatusSucesso && balance != 100) || (final.Status == domain.TxStatusCancelado && balance != 200) {
		t.Fatalf("estado/saldo inconsistentes: %s %v", final.Status, balance)
	}
	if final.ReconciledAt == nil || time.Since(*final.ReconciledAt) > time.Minute {
		t.Fatal("saque não conciliado")
	}
}

func TestEarlyCallbackIsReconciledWhenSubmissionArrives(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "early-user", 100)
	repo := postgres.NewTransactionRepository(pool)
	pix, key := "user@example.com", "early-key"
	_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: "WDR-early", UserID: "early-user", Tipo: "Saque PIX", Valor: -60, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "suitpay"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimPayout(ctx, "WDR-early"); err != nil {
		t.Fatal(err)
	}
	matched, err := repo.ApplyPayoutCallback(ctx, "provider-early", "PAID_OUT", 57)
	if err != nil || matched != nil {
		t.Fatalf("callback antecipado: %+v %v", matched, err)
	}
	got, err := repo.RecordPayoutSubmission(ctx, "WDR-early", "provider-early")
	if err != nil || got.Status != domain.TxStatusSucesso {
		t.Fatalf("evento não conciliado: %+v %v", got, err)
	}
	got, err = repo.ApplyPayoutCallback(ctx, "provider-early", "CANCELED", 57)
	if err != nil || got.Status != domain.TxStatusCancelado {
		t.Fatalf("cancelamento não conciliado: %+v %v", got, err)
	}
	var balance float64
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='early-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 100 {
		t.Fatalf("estorno incorreto: %v", balance)
	}
	if _, err = repo.ApplyPayoutCallback(ctx, "provider-early", "CANCELED", 57); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='early-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 100 {
		t.Fatalf("callback duplicado estornou duas vezes: %v", balance)
	}
}

func TestClaimAndRejectRaceHasOneWinner(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "race-user", 100)
	repo := postgres.NewTransactionRepository(pool)
	pix, key := "race@example.com", "race-key"
	_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: "WDR-race", UserID: "race-user", Tipo: "Saque PIX", Valor: -60, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "suitpay"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = repo.ClaimPayout(ctx, "WDR-race") }()
	go func() { defer wg.Done(); _, _ = repo.RejectWithdrawal(ctx, "WDR-race", "manual", false) }()
	wg.Wait()
	result, err := repo.GetByID(ctx, "WDR-race")
	if err != nil {
		t.Fatal(err)
	}
	var balance float64
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='race-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if (result.Status != domain.TxStatusProcessando || balance != 40) && (result.Status != domain.TxStatusRejeitado || balance != 100) {
		t.Fatalf("corrida inconsistente: status=%s saldo=%v", result.Status, balance)
	}
}

func TestStaleUserUpdateCannotEraseFinancialCredit(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "stale-user", 0)
	users := postgres.NewUserRepository(pool)
	stale, err := users.GetByID(ctx, "stale-user")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,external_id,gateway_provider) VALUES ('PIX-stale','stale-user','Depósito PIX',50,'pendente','SIM-PIX-stale','simulated')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	if _, _, err = repo.ConfirmDeposit(ctx, "PIX-stale", 50, "simulated", "SIM-PIX-stale"); err != nil {
		t.Fatal(err)
	}
	stale.AlreadySpunToday = true
	if err = users.Update(ctx, stale); err != domain.ErrConcurrentModification {
		t.Fatalf("update obsoleto aceito: %v", err)
	}
	fresh, err := users.GetByID(ctx, "stale-user")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Balance != 50 {
		t.Fatalf("crédito perdido: %v", fresh.Balance)
	}
}

func TestRentAndPayoutCompeteForSameBalance(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "mixed-user", 100)
	_, err := pool.Exec(ctx, `INSERT INTO fleet_catalog (id,name,price,daily_yield,stock_quantity) VALUES ('CAT-test','Cat Teste',80,2,1)`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	pix, key := "mixed@example.com", "mixed-payout"
	var wg sync.WaitGroup
	var rentOK, payoutOK bool
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _, err := repo.RentMachineAtomic(ctx, "mixed-user", "CAT-test", "mixed-rent")
		rentOK = err == nil
	}()
	go func() {
		defer wg.Done()
		_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: "WDR-mixed", UserID: "mixed-user", Tipo: "Saque PIX", Valor: -80, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "simulated"}, nil)
		payoutOK = err == nil
	}()
	wg.Wait()
	if rentOK == payoutOK {
		t.Fatalf("esperado exatamente um débito: aluguel=%v saque=%v", rentOK, payoutOK)
	}
	var balance float64
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='mixed-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 20 {
		t.Fatalf("saldo incorreto: %v", balance)
	}
	if rentOK {
		first, _, _, err := repo.RentMachineAtomic(ctx, "mixed-user", "CAT-test", "mixed-rent")
		if err != nil {
			t.Fatal(err)
		}
		second, _, _, err := repo.RentMachineAtomic(ctx, "mixed-user", "CAT-test", "mixed-rent")
		if err != nil || first.ID != second.ID {
			t.Fatalf("aluguel idempotente falhou: %+v %+v %v", first, second, err)
		}
	}
}

func TestConcurrentYieldSpinAndInsuranceCreditOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "earn-user", 0)
	_, err := pool.Exec(ctx, `INSERT INTO machines(id,user_id,name,cost,yield_amount,last_collected_at) VALUES ('earn-machine','earn-user','Cat Teste',0,10,now()-interval '24 hours 5 minutes')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	var collected int
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.CollectYieldAtomic(ctx, "earn-user", "earn-machine")
			if err == nil {
				mu.Lock()
				collected++
				mu.Unlock()
			} else if err != domain.ErrYieldNotReady {
				t.Errorf("coleta: %v", err)
			}
		}()
	}
	wg.Wait()
	if collected != 1 {
		t.Fatalf("coletas creditadas: %d", collected)
	}
	var balance float64
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='earn-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("coleta duplicada: %v", balance)
	}
	var spun int
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.ApplySpin(ctx, "earn-user", "Prêmio R$ 5", "", 5)
			if err == nil {
				mu.Lock()
				spun++
				mu.Unlock()
			} else if err != domain.ErrSpinAlreadyUsedToday {
				t.Errorf("giro: %v", err)
			}
		}()
	}
	wg.Wait()
	if spun != 1 {
		t.Fatalf("giros creditados: %d", spun)
	}
	_, err = pool.Exec(ctx, `UPDATE users SET insurance_vouchers_count=1 WHERE id='earn-user'`)
	if err != nil {
		t.Fatal(err)
	}
	var recovered int
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.RecoverInsuranceAtomic(ctx, "earn-user")
			if err == nil {
				mu.Lock()
				recovered++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if recovered != 1 {
		t.Fatalf("resgates de seguro: %d", recovered)
	}
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='earn-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 35 {
		t.Fatalf("saldo após operações=%v", balance)
	}
}

func TestAdminAdjustmentIsIdempotentAndReconciliationFindsDrift(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "adjust-user", 10)
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.AdjustAdminBalanceAtomic(ctx, "adjust-user", "admin-1", "correção", "adjust-key", 5); err != nil {
				t.Errorf("ajuste: %v", err)
			}
		}()
	}
	wg.Wait()
	var balance float64
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='adjust-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 15 {
		t.Fatalf("ajuste duplicado: %v", balance)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET balance=balance+1 WHERE id='adjust-user'`); err != nil {
		t.Fatal(err)
	}
	issues, err := repo.ListFinancialReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range issues {
		if issue.Kind == "balance_mismatch" && issue.UserID == "adjust-user" {
			found = true
		}
	}
	if !found {
		t.Fatal("divergência de saldo não detectada")
	}
}

func TestConcurrentAssemblyConsumesPartsOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "parts-user", 0)
	_, err := pool.Exec(ctx, `UPDATE users SET motor_c15_count=1,esteira_reforcada_count=1,filtro_oleo_count=1,turbo_garrett_count=1 WHERE id='parts-user'`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	var count int
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.AssemblePartsAtomic(ctx, "parts-user")
			if err == nil {
				mu.Lock()
				count++
				mu.Unlock()
			} else if err != domain.ErrIncompletePartsAssembly {
				t.Errorf("montagem: %v", err)
			}
		}()
	}
	wg.Wait()
	if count != 1 {
		t.Fatalf("montagens: %d", count)
	}
	var machines int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM machines WHERE user_id='parts-user'`).Scan(&machines); err != nil {
		t.Fatal(err)
	}
	if machines != 1 {
		t.Fatalf("máquinas criadas: %d", machines)
	}
}

func TestPayoutPINLockoutPersistsAcrossRequests(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "pin-user", 100)
	repo := postgres.NewTransactionRepository(pool)
	pix := "pin@example.com"
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wrong-%d", i)
		_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: fmt.Sprintf("WDR-wrong-%d", i), UserID: "pin-user", Tipo: "Saque PIX", Valor: -50, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "simulated"}, func(*domain.User) error { return domain.ErrInvalidSecurityPassword })
		if err != domain.ErrInvalidSecurityPassword {
			t.Fatalf("tentativa %d: %v", i, err)
		}
	}
	key := "correct"
	_, _, _, err := repo.ReservePayout(ctx, &domain.Transaction{ID: "WDR-correct", UserID: "pin-user", Tipo: "Saque PIX", Valor: -50, PixKey: &pix, IdempotencyKey: &key, GatewayProvider: "simulated"}, func(*domain.User) error { return nil })
	if err != domain.ErrFinancialAuthLocked {
		t.Fatalf("bloqueio não persistiu: %v", err)
	}
	var balance float64
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='pin-user'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 100 {
		t.Fatalf("saque indevido: %v", balance)
	}
}

func TestConcurrentFirstDepositPaysOneReferralCommission(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "sponsor", 0)
	seedFinancialUser(t, pool, "guest", 0)
	_, err := pool.Exec(ctx, `INSERT INTO referrals(id,referrer_id,referred_user_id,nome,status) VALUES('ref-financial','sponsor','guest','guest','inativo')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,external_id,gateway_provider) VALUES('PIX-ref','guest','Depósito PIX',1000,'pendente','SIM-PIX-ref','simulated')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.ConfirmDeposit(ctx, "PIX-ref", 1000, "simulated", "SIM-PIX-ref")
			if err != nil {
				t.Errorf("callback: %v", err)
			}
		}()
	}
	wg.Wait()
	var balance float64
	var count int
	if err = pool.QueryRow(ctx, `SELECT balance FROM users WHERE id='sponsor'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE user_id='sponsor' AND kind='referral_commission'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if balance != 50 || count != 1 {
		t.Fatalf("comissão duplicada ou ausente: saldo=%v eventos=%d", balance, count)
	}
}

func TestOneCentDepositActivatesReferralWithoutZeroCommission(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "tiny-sponsor-fin", 0)
	seedFinancialUser(t, pool, "tiny-guest-fin", 0)
	if _, err := pool.Exec(ctx, `INSERT INTO referrals(id,referrer_id,referred_user_id,nome,status) VALUES('tiny-ref-fin','tiny-sponsor-fin','tiny-guest-fin','tiny-guest-fin','inativo')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,external_id,gateway_provider) VALUES('PIX-tiny-fin','tiny-guest-fin','Depósito PIX',0.01,'pendente','SIM-tiny-fin','simulated')`); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	if _, _, err := repo.ConfirmDeposit(ctx, "PIX-tiny-fin", 0.01, "simulated", "SIM-tiny-fin"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.ConfirmDeposit(ctx, "PIX-tiny-fin", 0.01, "simulated", "SIM-tiny-fin"); err != nil {
		t.Fatal(err)
	}
	var balance, sponsorBalance, referralDeposit string
	var status string
	var commissionCount, depositEvents int
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM users WHERE id='tiny-guest-fin'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM users WHERE id='tiny-sponsor-fin'`).Scan(&sponsorBalance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status,deposito::text FROM referrals WHERE id='tiny-ref-fin'`).Scan(&status, &referralDeposit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE tipo='Comissão de primeiro depósito' AND reference_id='PIX-tiny-fin'`).Scan(&commissionCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id='PIX-tiny-fin' AND kind='deposit'`).Scan(&depositEvents); err != nil {
		t.Fatal(err)
	}
	if balance != "0.01" || sponsorBalance != "0.00" || status != "ativo" || referralDeposit != "0.01" || commissionCount != 0 || depositEvents != 1 {
		t.Fatalf("depósito mínimo inconsistente: guest=%s sponsor=%s referral=%s/%s commissions=%d events=%d", balance, sponsorBalance, status, referralDeposit, commissionCount, depositEvents)
	}
}

func TestPostgresBcryptPINCanBeVerifiedByGo(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "legacy-pin", 0)
	var stored string
	if err := pool.QueryRow(ctx, `SELECT security_password FROM users WHERE id='legacy-pin'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "123456" || !crypto.CheckPassword("123456", stored) {
		t.Fatal("PIN legado não migrado para hash compatível")
	}
}

func TestWithdrawalQueueExcludesDeposits(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	seedFinancialUser(t, pool, "queue-user", 100)
	_, err := pool.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider) VALUES('PIX-queue','queue-user','Depósito PIX',50,'pendente','simulated'),('WDR-queue','queue-user','Saque PIX',-50,'pendente','simulated')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewTransactionRepository(pool)
	items, total, err := repo.ListWithdrawals(ctx, "pendente", 10, 0)
	if err != nil || total < 1 || len(items) < 1 {
		t.Fatalf("fila incorreta: %d %+v %v", total, items, err)
	}
	found := false
	for _, item := range items {
		if item.Tipo != "Saque PIX" {
			t.Fatalf("depósito entrou na fila: %+v", item)
		}
		if item.ID == "WDR-queue" {
			found = true
		}
	}
	if !found {
		t.Fatal("saque pendente não listado")
	}
}
