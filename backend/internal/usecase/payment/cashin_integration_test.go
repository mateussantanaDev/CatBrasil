package payment_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CATBRASIL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CATBRASIL_TEST_DATABASE_URL não configurado; pulando teste de integração PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("falha ao conectar no PostgreSQL de teste: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err = migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("falha ao aplicar migrações: %v", err)
	}
	if err = migrations.CheckCurrent(ctx, pool); err != nil {
		t.Fatalf("banco de teste não está na versão corrente da migração: %v", err)
	}
	return pool
}

func seedCashInUser(t *testing.T, pool *pgxpool.Pool, id string, balance float64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, username, password_hash, balance, required_rollover, withdraw_locked)
		VALUES ($1, $1, 'hashed_pwd', $2, 0, FALSE)
		ON CONFLICT (id) DO UPDATE SET balance=$2, required_rollover=0, withdraw_locked=FALSE`,
		id, balance)
	if err != nil {
		t.Fatalf("falha ao criar usuário de teste no PostgreSQL: %v", err)
	}
}

func seedCashInIntent(t *testing.T, pool *pgxpool.Pool, id, userID string, amount float64, externalID string) {
	t.Helper()
	var ext any
	if externalID != "" {
		ext = externalID
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO transactions (id, user_id, tipo, valor, status, external_id, gateway_provider)
		VALUES ($1, $2, 'Depósito PIX', $3, 'pendente', $4, 'suitpay')
		ON CONFLICT (id) DO UPDATE SET status='pendente', external_id=EXCLUDED.external_id, valor=$3`,
		id, userID, amount, ext)
	if err != nil {
		t.Fatalf("falha ao criar transação de depósito no PostgreSQL: %v", err)
	}
}

func TestPostgresCashIn_RealRepositoryWebhookFlows(t *testing.T) {
	pool := getTestPostgresPool(t)
	clientSecret := "suitpay-real-repo-secret"

	cfgRepo := &mockConfigRepo{
		cfg: &domain.SystemConfig{
			Gateway: domain.GatewayConfig{
				SuitPayEnabled:      true,
				SuitPayClientSecret: clientSecret,
			},
		},
	}

	// Uses production postgres.TransactionRepository directly — NO duplicate business logic or copied SQL applier
	txRepo := postgres.NewTransactionRepository(pool)
	userRepo := postgres.NewUserRepository(pool)

	uc := payment.NewPaymentUseCase(userRepo, txRepo, nil, cfgRepo, nil, "", payment.Options{
		Mode:           payment.ModeSuitPay,
		Environment:    "staging",
		DepositApplier: txRepo,
		DepositTracker: txRepo,
	})

	t.Run("Real Repo: Signed PAID_OUT webhook applies credit, rollover, and ledger entry", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 100000000)
		uid := fmt.Sprintf("usr_prod_%d", seq)
		txID := fmt.Sprintf("PIX_PROD_%d", seq)
		extID := fmt.Sprintf("%d", seq)
		amount := 150.00

		seedCashInUser(t, pool, uid, 50.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		raw := makeSignedSuitPayWebhook(seq, "PIX", "PAID_OUT", amount, txID, clientSecret)
		res := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !res.Success {
			t.Fatalf("webhook deveria ser processado com sucesso: %s", res.Error)
		}
		if res.Outcome != "applied" {
			t.Fatalf("esperado outcome applied pelo repositório real, obteve: %s", res.Outcome)
		}

		// 1. Assert balance and rollover in users table
		var balance, rollover float64
		err := pool.QueryRow(context.Background(), `SELECT balance, required_rollover FROM users WHERE id=$1`, uid).Scan(&balance, &rollover)
		if err != nil {
			t.Fatal(err)
		}
		if balance != 200.00 { // 50 initial + 150 deposit
			t.Fatalf("esperado saldo R$ 200.00, obteve R$ %.2f", balance)
		}
		if rollover != 150.00 {
			t.Fatalf("esperado rollover R$ 150.00, obteve R$ %.2f", rollover)
		}

		// 2. Assert transaction status in transactions table
		var status string
		var actualExtID *string
		err = pool.QueryRow(context.Background(), `SELECT status, external_id FROM transactions WHERE id=$1`, txID).Scan(&status, &actualExtID)
		if err != nil {
			t.Fatal(err)
		}
		if status != "sucesso" {
			t.Fatalf("status esperado 'sucesso', obteve: %s", status)
		}
		if actualExtID == nil || *actualExtID != extID {
			t.Fatalf("external_id incorreto no banco: %v", actualExtID)
		}

		// 3. Assert real ledger row in financial_events
		var eventKind string
		var eventAmt, bBefore, bAfter float64
		err = pool.QueryRow(context.Background(), `
			SELECT kind, amount, balance_before, balance_after
			FROM financial_events
			WHERE transaction_id=$1 AND user_id=$2`, txID, uid).Scan(&eventKind, &eventAmt, &bBefore, &bAfter)
		if err != nil {
			t.Fatalf("falha ao consultar financial_events: %v", err)
		}
		if eventKind != "deposit" || eventAmt != 150.00 || bBefore != 50.00 || bAfter != 200.00 {
			t.Fatalf("registro contábil inválido: kind=%s, amt=%.2f, before=%.2f, after=%.2f", eventKind, eventAmt, bBefore, bAfter)
		}

		// 4. Assert durable event in deposit_gateway_events
		var gwCount int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM deposit_gateway_events WHERE external_id=$1 AND status='PAID_OUT'`, extID).Scan(&gwCount)
		if err != nil {
			t.Fatal(err)
		}
		if gwCount != 1 {
			t.Fatalf("esperado 1 registro em deposit_gateway_events, obteve: %d", gwCount)
		}
	})

	t.Run("Real Repo: Duplicate and Concurrent Callbacks result in single credit and single journal entry", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 200000000)
		uid := fmt.Sprintf("usr_concur_%d", seq)
		txID := fmt.Sprintf("PIX_CONCUR_%d", seq)
		extID := fmt.Sprintf("%d", seq)
		amount := 100.00

		seedCashInUser(t, pool, uid, 0.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)
		raw := makeSignedSuitPayWebhook(seq, "PIX", "PAID_OUT", amount, txID, clientSecret)

		const concurrency = 8
		var appliedCount int32
		var duplicateCount int32
		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := uc.ProcessSuitPayWebhook(context.Background(), raw)
				if res.Success {
					if res.Outcome == "applied" {
						atomic.AddInt32(&appliedCount, 1)
					} else if res.Outcome == "duplicate" {
						atomic.AddInt32(&duplicateCount, 1)
					}
				}
			}()
		}
		wg.Wait()

		if appliedCount != 1 {
			t.Fatalf("esperado exatamente 1 callback aplicado sob concorrência real, obteve: %d (duplicados: %d)", appliedCount, duplicateCount)
		}
		if duplicateCount != concurrency-1 {
			t.Fatalf("esperado %d duplicados, obteve: %d", concurrency-1, duplicateCount)
		}

		// Saldo no PostgreSQL deve ter sido creditado exatamente uma vez
		var balance float64
		err := pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance)
		if err != nil {
			t.Fatal(err)
		}
		if balance != amount {
			t.Fatalf("saldo concorrente corrompido! Esperado R$ %.2f, obteve R$ %.2f", amount, balance)
		}

		// Linha única no journal
		var journalRows int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM financial_events WHERE transaction_id=$1`, txID).Scan(&journalRows)
		if err != nil {
			t.Fatal(err)
		}
		if journalRows != 1 {
			t.Fatalf("esperada exatamente 1 linha em financial_events para corrida, obteve: %d", journalRows)
		}
	})

	t.Run("Real Repo: CHARGEBACK reverses credit, rollover, and sets cancelado status", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 300000000)
		uid := fmt.Sprintf("usr_cb_%d", seq)
		txID := fmt.Sprintf("PIX_CB_%d", seq)
		extID := fmt.Sprintf("%d", seq)
		amount := 80.00

		seedCashInUser(t, pool, uid, 0.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		// 1. Primeiramente aplica PAID_OUT
		rawPaid := makeSignedSuitPayWebhook(seq, "PIX", "PAID_OUT", amount, txID, clientSecret)
		resPaid := uc.ProcessSuitPayWebhook(context.Background(), rawPaid)
		if !resPaid.Success || resPaid.Outcome != "applied" {
			t.Fatalf("falha ao creditar depósito inicial: %+v", resPaid)
		}

		// 2. Agora aplica CHARGEBACK assinado com idTransaction correspondente
		rawCB := makeSignedSuitPayWebhook(seq, "PIX", "CHARGEBACK", amount, txID, clientSecret)
		resCB := uc.ProcessSuitPayWebhook(context.Background(), rawCB)
		if !resCB.Success || resCB.Outcome != "applied" || resCB.Kind != "chargeback" {
			t.Fatalf("falha ao aplicar chargeback no repositório real: %+v", resCB)
		}

		// 3. Saldo deve ser estornado para 0.00
		var balance, rollover float64
		err := pool.QueryRow(context.Background(), `SELECT balance, required_rollover FROM users WHERE id=$1`, uid).Scan(&balance, &rollover)
		if err != nil {
			t.Fatal(err)
		}
		if balance != 0.00 {
			t.Fatalf("saldo após chargeback deveria ser 0.00, obteve: R$ %.2f", balance)
		}
		if rollover != 0.00 {
			t.Fatalf("rollover após chargeback deveria ser 0.00, obteve: R$ %.2f", rollover)
		}

		// 4. Status da transação no banco deve ser 'cancelado'
		var status string
		err = pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE id=$1`, txID).Scan(&status)
		if err != nil {
			t.Fatal(err)
		}
		if status != "cancelado" {
			t.Fatalf("status após chargeback deveria ser 'cancelado', obteve: %s", status)
		}

		// 5. Dois eventos em financial_events (deposit e deposit_chargeback)
		var totalEvents int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM financial_events WHERE transaction_id=$1`, txID).Scan(&totalEvents)
		if err != nil {
			t.Fatal(err)
		}
		if totalEvents != 2 {
			t.Fatalf("esperados 2 registros em financial_events (deposit + deposit_chargeback), obteve: %d", totalEvents)
		}
	})

	t.Run("Real Repo: Unrecognized or non-PIX status safely enqueues without crediting", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 400000000)
		uid := fmt.Sprintf("usr_q_%d", seq)
		txID := fmt.Sprintf("PIX_Q_%d", seq)
		extID := fmt.Sprintf("%d", seq)
		amount := 110.00

		seedCashInUser(t, pool, uid, 10.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		// Envia tipo BOLETO com PAID_OUT -> deve gerar UNSUPPORTED_TYPE:BOLETO e enfileirar
		rawBoleto := makeSignedSuitPayWebhook(seq, "BOLETO", "PAID_OUT", amount, txID, clientSecret)
		resBoleto := uc.ProcessSuitPayWebhook(context.Background(), rawBoleto)

		if !resBoleto.Success {
			t.Fatalf("webhook autenticado deve ser aceito: %s", resBoleto.Error)
		}
		if resBoleto.Outcome != "queued" {
			t.Fatalf("esperado outcome queued, obteve: %s", resBoleto.Outcome)
		}

		// Saldo no banco deve continuar intacto (10.00)
		var balance float64
		err := pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance)
		if err != nil {
			t.Fatal(err)
		}
		if balance != 10.00 {
			t.Fatalf("saldo não pode ser alterado para tipo não suportado! Obteve: R$ %.2f", balance)
		}

		// Verifica que foi gravado em cashin_event_queue
		var queueCount int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM cashin_event_queue WHERE request_number=$1`, txID).Scan(&queueCount)
		if err != nil {
			t.Fatal(err)
		}
		if queueCount != 1 {
			t.Fatalf("esperado 1 item em cashin_event_queue, obteve: %d", queueCount)
		}
	})

	t.Run("Real Repo: Early CHARGEBACK then PAID_OUT applies and immediately reverses", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 500000000)
		uid := fmt.Sprintf("usr_early_%d", seq)
		txID := fmt.Sprintf("PIX_EARLY_%d", seq)
		// SAME external id for CHARGEBACK and PAID_OUT (out-of-order delivery).
		extID := fmt.Sprintf("%d", seq)
		amount := 95.00

		seedCashInUser(t, pool, uid, 0.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		// 1. CHARGEBACK arrives BEFORE any credit: durably queued, no money moved.
		rawCB := makeSignedSuitPayWebhook(seq, "PIX", "CHARGEBACK", amount, txID, clientSecret)
		resCB := uc.ProcessSuitPayWebhook(context.Background(), rawCB)
		if !resCB.Success {
			t.Fatalf("CHARGEBACK antecipado autenticado deve ser aceito: %s", resCB.Error)
		}
		if resCB.Outcome != "queued" {
			t.Fatalf("CHARGEBACK antes do crédito deve ser enfileirado, obteve: %s", resCB.Outcome)
		}

		// 2. Late PAID_OUT with the SAME external id: repo applies credit and
		// immediately reverses it, returning applied with status cancelado.
		rawPaid := makeSignedSuitPayWebhook(seq, "PIX", "PAID_OUT", amount, txID, clientSecret)
		resPaid := uc.ProcessSuitPayWebhook(context.Background(), rawPaid)
		if !resPaid.Success {
			t.Fatalf("PAID_OUT tardio deve ter desfecho durável: %s", resPaid.Error)
		}
		if resPaid.Outcome != "applied" {
			t.Fatalf("esperado outcome applied do repositório real, obteve: %s", resPaid.Outcome)
		}
		if resPaid.Kind != "chargeback" {
			t.Fatalf("PAID_OUT revertido deve ser classificado como chargeback pelo usecase, obteve kind: %s", resPaid.Kind)
		}
		if resPaid.Status != "cancelado" {
			t.Fatalf("status cancelado deve ser preservado, obteve: %s", resPaid.Status)
		}

		// 3. Net balance effect must be zero (credited then reversed in one tx).
		var balance, rollover float64
		err := pool.QueryRow(context.Background(), `SELECT balance, required_rollover FROM users WHERE id=$1`, uid).Scan(&balance, &rollover)
		if err != nil {
			t.Fatal(err)
		}
		if balance != 0.00 || rollover != 0.00 {
			t.Fatalf("saldo/líquido após estorno automático deveria ser 0.00/0.00, obteve: %.2f/%.2f", balance, rollover)
		}

		// 4. Withdrawals stay locked after automatic reversal.
		var withdrawLocked bool
		err = pool.QueryRow(context.Background(), `SELECT withdraw_locked FROM users WHERE id=$1`, uid).Scan(&withdrawLocked)
		if err != nil {
			t.Fatal(err)
		}
		if !withdrawLocked {
			t.Fatal("saques devem permanecer bloqueados após estorno automático")
		}

		// 5. Transaction ends cancelado with both ledger entries.
		var status string
		err = pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE id=$1`, txID).Scan(&status)
		if err != nil {
			t.Fatal(err)
		}
		if status != "cancelado" {
			t.Fatalf("status final deveria ser cancelado, obteve: %s", status)
		}
		var totalEvents int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM financial_events WHERE transaction_id=$1`, txID).Scan(&totalEvents)
		if err != nil {
			t.Fatal(err)
		}
		if totalEvents != 2 {
			t.Fatalf("esperados 2 lançamentos (deposit + deposit_chargeback), obteve: %d", totalEvents)
		}

		// 6. Replayed PAID_OUT is a durable duplicate classified as chargeback —
		// never a new credit.
		resDup := uc.ProcessSuitPayWebhook(context.Background(), rawPaid)
		if !resDup.Success {
			t.Fatalf("replay de PAID_OUT deve ser aceito: %s", resDup.Error)
		}
		if resDup.Outcome != "duplicate" {
			t.Fatalf("replay após ciclo completo deve ser duplicate, obteve: %s", resDup.Outcome)
		}
		if resDup.Kind != "chargeback" {
			t.Fatalf("replay de transação estornada deve permanecer chargeback, obteve: %s", resDup.Kind)
		}
		err = pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance)
		if err != nil {
			t.Fatal(err)
		}
		if balance != 0.00 {
			t.Fatalf("replay não pode creditar saldo, obteve: R$ %.2f", balance)
		}
	})

	t.Run("Real Repo: Large numeric external ID above 2^53 preserves exact match", func(t *testing.T) {
		// 2^53+1 cannot survive a float64 round trip; this is the precision-loss
		// regression that previously turned the id into scientific notation.
		largeID := int64(9007199254740993) + int64(time.Now().UnixNano()%100000)
		uid := fmt.Sprintf("usr_big_%d", largeID)
		txID := fmt.Sprintf("PIX_BIG_%d", largeID)
		extID := fmt.Sprintf("%d", largeID)
		amount := 120.00

		seedCashInUser(t, pool, uid, 0.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		raw := makeSignedSuitPayWebhook(largeID, "PIX", "PAID_OUT", amount, txID, clientSecret)
		res := uc.ProcessSuitPayWebhook(context.Background(), raw)
		if !res.Success {
			t.Fatalf("webhook com id numérico grande deveria ser processado: %s", res.Error)
		}
		if res.Outcome != "applied" {
			t.Fatalf("esperado outcome applied, obteve: %s", res.Outcome)
		}

		var actualExtID *string
		var status string
		err := pool.QueryRow(context.Background(), `SELECT status, external_id FROM transactions WHERE id=$1`, txID).Scan(&status, &actualExtID)
		if err != nil {
			t.Fatal(err)
		}
		if status != "sucesso" {
			t.Fatalf("status esperado sucesso, obteve: %s", status)
		}
		if actualExtID == nil || *actualExtID != extID {
			t.Fatalf("external_id numérico grande perdeu precisão no banco: esperado %s, obteve %v", extID, actualExtID)
		}

		var balance float64
		err = pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance)
		if err != nil {
			t.Fatal(err)
		}
		if balance != amount {
			t.Fatalf("esperado saldo R$ %.2f, obteve R$ %.2f", amount, balance)
		}
	})

	t.Run("Real Repo: String idTransaction webhook resolves and credits exactly", func(t *testing.T) {
		seq := int((time.Now().UnixNano()/1000)%800000000 + 600000000)
		uid := fmt.Sprintf("usr_str_%d", seq)
		txID := fmt.Sprintf("PIX_STR_%d", seq)
		extID := fmt.Sprintf("prov-str-%d", seq)
		amount := 75.00

		seedCashInUser(t, pool, uid, 0.00)
		seedCashInIntent(t, pool, txID, uid, amount, extID)

		raw := makeSignedSuitPayWebhookWithStringID(extID, "PIX", "PAID_OUT", amount, txID, clientSecret)
		res := uc.ProcessSuitPayWebhook(context.Background(), raw)
		if !res.Success {
			t.Fatalf("webhook com id string deveria ser processado: %s", res.Error)
		}
		if res.Outcome != "applied" {
			t.Fatalf("esperado outcome applied, obteve: %s", res.Outcome)
		}

		var actualExtID *string
		var status string
		err := pool.QueryRow(context.Background(), `SELECT status, external_id FROM transactions WHERE id=$1`, txID).Scan(&status, &actualExtID)
		if err != nil {
			t.Fatal(err)
		}
		if status != "sucesso" {
			t.Fatalf("status esperado sucesso, obteve: %s", status)
		}
		if actualExtID == nil || *actualExtID != extID {
			t.Fatalf("external_id string alterado no banco: esperado %s, obteve %v", extID, actualExtID)
		}

		var balance float64
		err = pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, uid).Scan(&balance)
		if err != nil {
			t.Fatal(err)
		}
		if balance != amount {
			t.Fatalf("esperado saldo R$ %.2f, obteve R$ %.2f", amount, balance)
		}
	})
}
