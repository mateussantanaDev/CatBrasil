package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func payoutRunID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// reservePayoutFor reserva um saque SuitPay com taxa de 5% para os testes de
// cash-out: valor bruto -60 gera líquido 57.
func reservePayoutFor(t *testing.T, repo *postgres.TransactionRepository, user, id, key string, valor float64) *domain.Transaction {
	t.Helper()
	pix := user + "@example.com"
	reserved, _, _, err := repo.ReservePayout(context.Background(), &domain.Transaction{
		ID: id, UserID: user, Tipo: "Saque PIX", Valor: valor, PixKey: &pix,
		IdempotencyKey: &key, GatewayProvider: "suitpay",
	}, nil)
	if err != nil {
		t.Fatalf("reserva de saque: %v", err)
	}
	return reserved
}

func payoutQueueKinds(t *testing.T, pool *pgxpool.Pool, externalID string) map[string]int {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT kind,count(*) FROM payout_event_queue WHERE external_id=$1 AND queue_status='pending' GROUP BY kind`, externalID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	kinds := make(map[string]int)
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			t.Fatal(err)
		}
		kinds[kind] += count
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return kinds
}

func userBalance(t *testing.T, pool *pgxpool.Pool, user string) float64 {
	t.Helper()
	var balance float64
	if err := pool.QueryRow(context.Background(), `SELECT balance FROM users WHERE id=$1`, user).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	return balance
}

// payoutQueueAmounts devolve os valores conflitantes ainda pendentes para o
// external_id, ordenados: evidências diferentes nunca podem se sobrescrever.
func payoutQueueAmounts(t *testing.T, pool *pgxpool.Pool, externalID string) []float64 {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT amount FROM payout_event_queue WHERE external_id=$1 AND kind='amount_conflict' AND queue_status='pending' ORDER BY amount`, externalID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	amounts := make([]float64, 0)
	for rows.Next() {
		var amount float64
		if err := rows.Scan(&amount); err != nil {
			t.Fatal(err)
		}
		amounts = append(amounts, amount)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return amounts
}

// resolveAllPayoutQueue encerra todos os itens pendentes da fila de revisão do
// external_id e devolve quantos foram resolvidos.
func resolveAllPayoutQueue(t *testing.T, pool *pgxpool.Pool, repo *postgres.TransactionRepository, externalID string) int {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT id FROM payout_event_queue WHERE external_id=$1 AND queue_status='pending'`, externalID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for i, id := range ids {
		if err := repo.ResolvePayoutQueueItem(ctx, id, fmt.Sprintf("operador-%d", i+1), "extrato_suitpay:conferido", "valor do gateway conferido manualmente"); err != nil {
			t.Fatal(err)
		}
	}
	return len(ids)
}

// PAID_OUT chegando depois do cancelamento local preserva a evidência na fila,
// trava novos saques da conta e não reenvia nem move dinheiro.
func TestPayoutPaidOutAfterLocalCancelKeepsEvidenceWithoutMoney(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-pac-" + run
	id, key, ext := "WDR-pac-"+run, "po-pac-key-"+run, "gw-pac-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
	canceled, err := repo.ApplyPayoutCallback(ctx, ext, "CANCELED", 57)
	if err != nil || canceled == nil || canceled.Status != domain.TxStatusCancelado {
		t.Fatalf("cancelamento: %+v %v", canceled, err)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("estorno inicial: %v", balance)
	}
	// PAID_OUT tardio: reconhecido (200), evidência durável, dinheiro intocado.
	got, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57)
	if err != nil || got != nil {
		t.Fatalf("PAID_OUT pós-cancelamento deveria enfileirar sem efeito: %+v %v", got, err)
	}
	var events int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_gateway_events WHERE external_id=$1 AND status='PAID_OUT'`, ext).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("evento autenticado perdido no rollback: %d", events)
	}
	final, err := repo.GetByID(ctx, id)
	if err != nil || final.Status != domain.TxStatusCancelado || final.PaidAt != nil {
		t.Fatalf("estado enganoso: %+v %v", final, err)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("PAID_OUT tardio moveu saldo: %v", balance)
	}
	var refunds int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='payout_cancel_refund'`, id).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 {
		t.Fatalf("estorno duplicado: %d", refunds)
	}
	var queueID int64
	var reason string
	if err = pool.QueryRow(ctx, `SELECT id,reason FROM payout_event_queue WHERE external_id=$1 AND kind='paid_after_cancel' AND queue_status='pending'`, ext).Scan(&queueID, &reason); err != nil || reason == "" {
		t.Fatalf("flag de revisão ausente: %v", err)
	}
	var locked bool
	if err = pool.QueryRow(ctx, `SELECT withdraw_locked FROM users WHERE id=$1`, user).Scan(&locked); err != nil || !locked {
		t.Fatalf("saques da conta não travados após conflito: %v %v", locked, err)
	}
	issues, err := repo.ListFinancialReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range issues {
		if issue.Kind == "paid_after_cancel" && issue.QueueID == queueID && issue.TransactionID == id && issue.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("conflito PAID_OUT pós-cancelamento invisível para operação: %+v", issues)
	}
	// Replay não duplica o item nem move dinheiro.
	if _, err = repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE external_id=$1 AND kind='paid_after_cancel'`, ext).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("fila duplicada no replay: %d (%v)", rows, err)
	}
}

// CANCELED após PAID_OUT é estorno/chargeback: devolve o valor uma única vez e
// um replay de ambos os lados é idempotente.
func TestPayoutCancelAfterPaidOutRefundsExactlyOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-cap-" + run
	id, key, ext := "WDR-cap-"+run, "po-cap-key-"+run, "gw-cap-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
	paid, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57)
	if err != nil || paid == nil || paid.Status != domain.TxStatusSucesso {
		t.Fatalf("liquidação: %+v %v", paid, err)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo pós-liquidação: %v", balance)
	}
	canceled, err := repo.ApplyPayoutCallback(ctx, ext, "CANCELED", 57)
	if err != nil || canceled == nil || canceled.Status != domain.TxStatusCancelado {
		t.Fatalf("estorno: %+v %v", canceled, err)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("saldo pós-estorno: %v", balance)
	}
	var refunds int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='payout_cancel_refund'`, id).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 {
		t.Fatalf("estorno aplicado %d vezes", refunds)
	}
	// Duplicatas de ambos os status não alteram nada.
	dupCancel, err := repo.ApplyPayoutCallback(ctx, ext, "CANCELED", 57)
	if err != nil || dupCancel == nil || dupCancel.Status != domain.TxStatusCancelado {
		t.Fatalf("CANCELED duplicado: %+v %v", dupCancel, err)
	}
	dupPaid, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57)
	if err != nil || dupPaid == nil || dupPaid.Status != domain.TxStatusCancelado {
		t.Fatalf("replay de PAID_OUT pós-ciclo: %+v %v", dupPaid, err)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("duplicata alterou saldo: %v", balance)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='payout_cancel_refund'`, id).Scan(&refunds); err != nil || refunds != 1 {
		t.Fatalf("duplicata estornou duas vezes: %d (%v)", refunds, err)
	}
	if kinds := payoutQueueKinds(t, pool, ext); len(kinds) != 0 {
		t.Fatalf("replay idempotente gerou ruído na fila: %v", kinds)
	}
}

// Valor conflitante fica visível na fila segura sem alterar saldo nem estado,
// e a resolução operacional não silencia um novo conflito.
func TestPayoutConflictingAmountIsQueuedWithoutBalanceChange(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-amt-" + run
	id, key, ext := "WDR-amt-"+run, "po-amt-key-"+run, "gw-amt-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 99)
	if err != nil || got != nil {
		t.Fatalf("valor conflitante deveria enfileirar: %+v %v", got, err)
	}
	var events int
	var stored float64
	if err = pool.QueryRow(ctx, `SELECT count(*),COALESCE(max(amount),0) FROM payout_gateway_events WHERE external_id=$1 AND status='PAID_OUT'`, ext).Scan(&events, &stored); err != nil {
		t.Fatal(err)
	}
	if events != 1 || stored != 99 {
		t.Fatalf("evento conflitante perdido: %d/%v", events, stored)
	}
	withdrawal, err := repo.GetByID(ctx, id)
	if err != nil || withdrawal.Status != domain.TxStatusProcessando {
		t.Fatalf("estado alterado por conflito: %+v %v", withdrawal, err)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo alterado por conflito: %v", balance)
	}
	// O valor correto também conflita com o primeiro registrado: nada aplica.
	if got, err = repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57); err != nil || got != nil {
		t.Fatalf("segundo valor não pode aplicar silenciosamente: %+v %v", got, err)
	}
	if withdrawal, err = repo.GetByID(ctx, id); err != nil || withdrawal.Status != domain.TxStatusProcessando {
		t.Fatalf("liquidação indevida: %+v %v", withdrawal, err)
	}
	issues, err := repo.ListFinancialReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	visible := 0
	for _, issue := range issues {
		if issue.Kind == "amount_conflict" && issue.ExternalID == ext && issue.QueueID != 0 && issue.Reason != "" {
			visible++
		}
	}
	if visible != 2 {
		t.Fatalf("valores conflitantes distintos não ficaram visíveis como evidências separadas: %d (%+v)", visible, issues)
	}
	// 99 conflita com o líquido reservado e 57 conflita com o valor registrado no
	// evento: duas evidências coexistem, nenhuma sobrescreve a outra.
	if amounts := payoutQueueAmounts(t, pool, ext); len(amounts) != 2 || amounts[0] != 57 || amounts[1] != 99 {
		t.Fatalf("evidências de valor conflitante: %v", amounts)
	}
	if resolved := resolveAllPayoutQueue(t, pool, repo, ext); resolved != 2 {
		t.Fatalf("resolução operacional incompleta: %d", resolved)
	}
	if kinds := payoutQueueKinds(t, pool, ext); len(kinds) != 0 {
		t.Fatalf("item resolvido continua pendente: %v", kinds)
	}
	// Um novo conflito reabre a própria linha em vez de sumir com a evidência.
	if _, err = repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 99); err != nil {
		t.Fatal(err)
	}
	if kinds := payoutQueueKinds(t, pool, ext); kinds["amount_conflict"] != 1 {
		t.Fatalf("replay de conflito não reabriu a fila: %v", kinds)
	}
}

// Callback antecipado com valor divergente não pode bloquear o vínculo da
// submissão para sempre: o saque fica linkado e o conflito fica na fila.
func TestConflictingEarlyCallbackDoesNotBlockSubmissionLink(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-early-" + run
	id, key, ext := "WDR-early-"+run, "po-early-key-"+run, "gw-early-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 12); err != nil {
		t.Fatal(err)
	}
	linked, err := repo.RecordPayoutSubmission(ctx, id, ext)
	if err != nil {
		t.Fatalf("submissão bloqueada por evento conflitante: %v", err)
	}
	if linked.Status != domain.TxStatusProcessando || linked.ExternalID == nil || *linked.ExternalID != ext {
		t.Fatalf("vínculo ausente: %+v", linked)
	}
	if kinds := payoutQueueKinds(t, pool, ext); kinds["amount_conflict"] != 1 {
		t.Fatalf("conflito do drain ausente: %v", kinds)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("drain conflitante moveu saldo: %v", balance)
	}
	var internalRef, linkedExt string
	var linkedAt *time.Time
	if err = pool.QueryRow(ctx, `SELECT internal_ref,COALESCE(external_id,''),linked_at FROM payout_submissions WHERE transaction_id=$1`, id).Scan(&internalRef, &linkedExt, &linkedAt); err != nil {
		t.Fatal(err)
	}
	if internalRef != id || linkedExt != ext || linkedAt == nil {
		t.Fatalf("ID interno/retorno não associados: ref=%s ext=%s linked=%v", internalRef, linkedExt, linkedAt)
	}
}

// Submissões concorrentes de PAID_OUT e CANCELED convergem para um único
// estado final, com no máximo um estorno no razão.
func TestConcurrentPayoutCallbacksConvergeWithoutDoubleRefund(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-mixed-" + run
	id, key, ext := "WDR-mixed-"+run, "po-mixed-key-"+run, "gw-mixed-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 24)
	for i := 0; i < 12; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57); failures <- err }()
		go func() { defer wg.Done(); _, err := repo.ApplyPayoutCallback(ctx, ext, "CANCELED", 57); failures <- err }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatalf("callback concorrente falhou: %v", err)
		}
	}
	final, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// CANCELED é aplicável em qualquer ordem; PAID_OUT nunca re-debita.
	if final.Status != domain.TxStatusCancelado {
		t.Fatalf("estado final inesperado: %s", final.Status)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("saldo final enganoso: %v", balance)
	}
	var refunds int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='payout_cancel_refund'`, id).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 {
		t.Fatalf("estornos aplicados: %d", refunds)
	}
	var paidEvents, cancelEvents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='PAID_OUT'),count(*) FILTER (WHERE status='CANCELED') FROM payout_gateway_events WHERE external_id=$1`, ext).Scan(&paidEvents, &cancelEvents); err != nil {
		t.Fatal(err)
	}
	if paidEvents == 0 || cancelEvents == 0 {
		t.Fatalf("eventos perdidos: paid=%d cancel=%d", paidEvents, cancelEvents)
	}
	var conflicts int
	var locked bool
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE external_id=$1 AND kind='paid_after_cancel'`, ext).Scan(&conflicts); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT withdraw_locked FROM users WHERE id=$1`, user).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if conflicts > 1 || (conflicts == 1 && !locked) || (conflicts == 0 && locked) {
		t.Fatalf("flag de conflito inconsistente: conflicts=%d locked=%v", conflicts, locked)
	}
}

// O claim grava o ID interno estável enviado ao provedor; uma resposta ausente
// permanece visível para triagem e o retorno posterior associa os dois IDs.
func TestClaimRecordsInternalRefAndAmbiguousDispatchIsVisible(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-claim-" + run
	id, key, ext := "WDR-claim-"+run, "po-claim-key-"+run, "gw-claim-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	claimed, err := repo.ClaimPayout(ctx, id)
	if err != nil || claimed.Status != domain.TxStatusProcessando {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	var internalRef, provider string
	var externalID *string
	var sentAt time.Time
	if err = pool.QueryRow(ctx, `SELECT internal_ref,provider,external_id,sent_at FROM payout_submissions WHERE transaction_id=$1`, id).Scan(&internalRef, &provider, &externalID, &sentAt); err != nil {
		t.Fatalf("submissão durável ausente: %v", err)
	}
	if internalRef != id || provider != "suitpay" || externalID != nil {
		t.Fatalf("triagem de submissão errada: ref=%s provider=%s ext=%v", internalRef, provider, externalID)
	}
	// Resposta ambígua (nunca associada): a operação envelhecida aparece.
	if _, err = pool.Exec(ctx, `UPDATE transactions SET payout_attempted_at=now()-interval '10 minutes' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	issues, err := repo.ListFinancialReconciliation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	visible := false
	for _, issue := range issues {
		if issue.Kind == "payout_processing" && issue.TransactionID == id {
			visible = true
		}
	}
	if !visible {
		t.Fatal("submissão envelhecida invisível para triagem")
	}
	linked, err := repo.RecordPayoutSubmission(ctx, id, ext)
	if err != nil || linked.ExternalID == nil || *linked.ExternalID != ext {
		t.Fatalf("retorno não associado: %+v %v", linked, err)
	}
	if err = pool.QueryRow(ctx, `SELECT external_id,linked_at FROM payout_submissions WHERE transaction_id=$1`, id).Scan(&externalID, &sentAt); err != nil || externalID == nil || *externalID != ext {
		t.Fatalf("ID de retorno não associado ao ID interno: %v %v", externalID, err)
	}
	got, err := repo.GetPayoutByExternalID(ctx, ext)
	if err != nil || got.ID != id {
		t.Fatalf("consulta por retorno: %+v %v", got, err)
	}
}

// Callbacks autenticados que não podem ser aplicados deixam rastro na fila sem
// nenhum efeito financeiro.
func TestPayoutCallbackBoundariesLeaveDurableQueueOnly(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	repo := postgres.NewTransactionRepository(pool)
	// Sem external_id: nada é inferido, nada move.
	if got, err := repo.ApplyPayoutCallback(ctx, "", "PAID_OUT", 50); err != nil || got != nil {
		t.Fatalf("callback sem external id: %+v %v", got, err)
	}
	var missing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE kind='missing_external_id' AND queue_status='pending' AND external_id IS NULL`).Scan(&missing); err != nil || missing == 0 {
		t.Fatalf("missing_external_id ausente: %d (%v)", missing, err)
	}
	// Status não suportado: enfileirado, nunca aplicado.
	unsupportedExt := "gw-unsupported-" + run
	if got, err := repo.ApplyPayoutCallback(ctx, unsupportedExt, "IN_ANALYSIS", 50); err != nil || got != nil {
		t.Fatalf("status não suportado: %+v %v", got, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT gateway_status FROM payout_event_queue WHERE external_id=$1 AND kind='unsupported_status'`, unsupportedExt).Scan(&status); err != nil || status != "IN_ANALYSIS" {
		t.Fatalf("status não suportado ausente: %q (%v)", status, err)
	}
	// Valor não transacionável: enfileirado sem efeito.
	invalidExt := "gw-invalid-amount-" + run
	if got, err := repo.ApplyPayoutCallback(ctx, invalidExt, "PAID_OUT", 0); err != nil || got != nil {
		t.Fatalf("valor inválido: %+v %v", got, err)
	}
	var invalid int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE external_id=$1 AND kind='amount_invalid'`, invalidExt).Scan(&invalid); err != nil || invalid != 1 {
		t.Fatalf("amount_invalid ausente: %d (%v)", invalid, err)
	}
	var registry int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payout_gateway_events WHERE external_id IN ($1,$2)`, unsupportedExt, invalidExt).Scan(&registry); err != nil || registry != 0 {
		t.Fatalf("status/valor inválidos não podem entrar no registro: %d (%v)", registry, err)
	}
}

// Um retorno para saque nunca enviado ao provedor não é inferido: erro + fila.
func TestSubmissionForNeverDispatchedPayoutIsQueuedNotLinked(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-nolink-" + run
	id, key, ext := "WDR-nolink-"+run, "po-nolink-key-"+run, "gw-nolink-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if got, err := repo.RecordPayoutSubmission(ctx, id, ext); err == nil || got != nil {
		t.Fatalf("retorno aceito sem claim: %+v", got)
	}
	withdrawal, err := repo.GetByID(ctx, id)
	if err != nil || withdrawal.ExternalID != nil || withdrawal.Status != domain.TxStatusPendente {
		t.Fatalf("vínculo inferido: %+v %v", withdrawal, err)
	}
	var queued int
	var queuedTx, queuedExt string
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE transaction_id=$1 AND kind='invalid_state' AND queue_status='pending'`, id).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("retorno sem envio invisível: %d (%v)", queued, err)
	}
	// A evidência do retorno recusado é preservada para o operador.
	if err = pool.QueryRow(ctx, `SELECT COALESCE(transaction_id,''),COALESCE(external_id,'') FROM payout_event_queue WHERE external_id=$1 AND kind='invalid_state'`, ext).Scan(&queuedTx, &queuedExt); err != nil || queuedTx != id || queuedExt != ext {
		t.Fatalf("evidência do retorno recusado incompleta: tx=%s ext=%s (%v)", queuedTx, queuedExt, err)
	}
}

// Um retorno já vinculado a outro saque nunca é reatribuído.
func TestSubmissionRebindToForeignExternalIDIsRejectedWithEvidence(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	userA, userB := "po-foreign-a-"+run, "po-foreign-b-"+run
	idA, keyA, ext := "WDR-foreign-a-"+run, "po-foreign-a-"+run, "gw-foreign-"+run
	idB, keyB := "WDR-foreign-b-"+run, "po-foreign-b-"+run
	seedFinancialUser(t, pool, userA, 100)
	seedFinancialUser(t, pool, userB, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, userA, idA, keyA, -60)
	if _, err := repo.ClaimPayout(ctx, idA); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, idA, ext); err != nil {
		t.Fatal(err)
	}
	reservePayoutFor(t, repo, userB, idB, keyB, -60)
	if _, err := repo.ClaimPayout(ctx, idB); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.RecordPayoutSubmission(ctx, idB, ext); err == nil || got != nil {
		t.Fatalf("retorno reatribuído: %+v", got)
	}
	stolen, err := repo.GetByID(ctx, idB)
	if err != nil || stolen.ExternalID != nil {
		t.Fatalf("saque alvo vinculado indevidamente: %+v %v", stolen, err)
	}
	owner, err := repo.GetByID(ctx, idA)
	if err != nil || owner.ExternalID == nil || *owner.ExternalID != ext {
		t.Fatalf("vínculo legítimo alterado: %+v %v", owner, err)
	}
	var conflicts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE transaction_id=$1 AND kind='external_conflict' AND queue_status='pending'`, idB).Scan(&conflicts); err != nil || conflicts != 1 {
		t.Fatalf("conflito de retorno invisível: %d (%v)", conflicts, err)
	}
	if balance := userBalance(t, pool, userB); balance != 40 {
		t.Fatalf("saldo do saque alvo alterado: %v", balance)
	}
}

// Um 23505 no índice único (gateway_provider, external_id) aborta a transação
// PostgreSQL: sem savepoint a evidência não poderia ser enfileirada nem
// commitada. Um depósito SuitPay já ocupando o retorno passa pela pré-checagem
// (que só olha saques) e prova que o savepoint devolveu a transação
// utilizável, sem vínculo inferido e sem movimentação.
func TestSubmissionUniqueViolationStillQueuesEvidenceAndCommits(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-uniq-" + run
	id, key, ext := "WDR-uniq-"+run, "po-uniq-key-"+run, "gw-uniq-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	depositID := "PIX-uniq-" + run
	if _, err := pool.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,external_id,gateway_provider) VALUES ($1,$2,'Depósito PIX',10,'sucesso',$3,'suitpay')`, depositID, user, ext); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.RecordPayoutSubmission(ctx, id, ext); err == nil || got != nil {
		t.Fatalf("retorno em uso por outro tipo aceito: %+v %v", got, err)
	}
	var conflicts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM payout_event_queue WHERE transaction_id=$1 AND kind='external_conflict' AND queue_status='pending'`, id).Scan(&conflicts); err != nil || conflicts != 1 {
		t.Fatalf("evidência do 23505 perdida: %d (%v)", conflicts, err)
	}
	var queuedExt, reason string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(external_id,''),reason FROM payout_event_queue WHERE transaction_id=$1 AND kind='external_conflict'`, id).Scan(&queuedExt, &reason); err != nil || queuedExt != ext || reason == "" {
		t.Fatalf("evidência incompleta: ext=%s reason=%s (%v)", queuedExt, reason, err)
	}
	var linkedExt *string
	if err := pool.QueryRow(ctx, `SELECT external_id FROM payout_submissions WHERE transaction_id=$1`, id).Scan(&linkedExt); err != nil || linkedExt != nil {
		t.Fatalf("submissão vinculada após conflito: %v (%v)", linkedExt, err)
	}
	withdrawal, err := repo.GetByID(ctx, id)
	if err != nil || withdrawal.ExternalID != nil || withdrawal.Status != domain.TxStatusProcessando {
		t.Fatalf("vínculo inferido após 23505: %+v %v", withdrawal, err)
	}
	var owner string
	if err = pool.QueryRow(ctx, `SELECT id FROM transactions WHERE gateway_provider='suitpay' AND external_id=$1`, ext).Scan(&owner); err != nil || owner != depositID {
		t.Fatalf("dono do retorno alterado: %s (%v)", owner, err)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo alterado: %v", balance)
	}
}

// Transição direta (modo simulado) com valor divergente enfileira evidência e
// não move dinheiro.
func TestPayoutTransitionReviewKeepsBalanceUntouched(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-trans-" + run
	id, key, ext := "WDR-trans-"+run, "po-trans-key-"+run, "gw-trans-"+run
	seedFinancialUser(t, pool, user, 100)
	repo := postgres.NewTransactionRepository(pool)
	reservePayoutFor(t, repo, user, id, key, -60)
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConfirmPayout(ctx, id, ext, 90); err == nil {
		t.Fatal("liquidação com valor divergente aceita")
	}
	withdrawal, err := repo.GetByID(ctx, id)
	if err != nil || withdrawal.Status != domain.TxStatusProcessando {
		t.Fatalf("estado alterado: %+v %v", withdrawal, err)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo alterado: %v", balance)
	}
	if kinds := payoutQueueKinds(t, pool, ext); kinds["amount_conflict"] != 1 {
		t.Fatalf("evidência da transição ausente: %v", kinds)
	}
}

// setupLinkedPayout reserva, reivindica e vincula um saque SuitPay para os
// testes de desfecho de callback (saldo inicial 100, líquido 57).
func setupLinkedPayout(t *testing.T, pool *pgxpool.Pool, repo *postgres.TransactionRepository, user, id, key, ext string) {
	t.Helper()
	seedFinancialUser(t, pool, user, 100)
	reservePayoutFor(t, repo, user, id, key, -60)
	ctx := context.Background()
	if _, err := repo.ClaimPayout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordPayoutSubmission(ctx, id, ext); err != nil {
		t.Fatal(err)
	}
}

// Sob o mesmo advisory lock/row lock, N callbacks concorrentes de PAID_OUT
// produzem exatamente 1 "applied" e o restante "duplicate": o INSERT do evento
// autenticado nunca é tratado como prova de aplicação.
func TestApplyPayoutCallbackOutcomeConcurrentPaidOutAppliesExactlyOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-out-" + run
	id, key, ext := "WDR-out-"+run, "po-out-key-"+run, "gw-out-"+run
	repo := postgres.NewTransactionRepository(pool)
	setupLinkedPayout(t, pool, repo, user, id, key, ext)
	const workers = 12
	outcomes := make(chan string, workers)
	failures := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, ext, "PAID_OUT", 57)
			if err != nil {
				failures <- err
				return
			}
			switch outcome {
			case postgres.PayoutApplied:
				if tx == nil || tx.Status != domain.TxStatusSucesso {
					failures <- fmt.Errorf("applied sem liquidação: %+v", tx)
					return
				}
			case postgres.PayoutDuplicate:
				if tx == nil {
					failures <- fmt.Errorf("duplicate sem a transação atual")
					return
				}
			default:
				failures <- fmt.Errorf("outcome inesperado para PAID_OUT já vinculado: %s", outcome)
				return
			}
			outcomes <- outcome
		}()
	}
	wg.Wait()
	close(outcomes)
	close(failures)
	for err := range failures {
		t.Fatalf("callback concorrente: %v", err)
	}
	counts := make(map[string]int)
	for outcome := range outcomes {
		counts[outcome]++
	}
	if counts[postgres.PayoutApplied] != 1 || counts[postgres.PayoutDuplicate] != workers-1 {
		t.Fatalf("outcomes: applied=%d duplicate=%d; esperado 1/%d", counts[postgres.PayoutApplied], counts[postgres.PayoutDuplicate], workers-1)
	}
	final, err := repo.GetByID(ctx, id)
	if err != nil || final.Status != domain.TxStatusSucesso || final.PaidAt == nil {
		t.Fatalf("liquidação final: %+v %v", final, err)
	}
	if balance := userBalance(t, pool, user); balance != 40 {
		t.Fatalf("saldo: %v", balance)
	}
	if kinds := payoutQueueKinds(t, pool, ext); len(kinds) != 0 {
		t.Fatalf("fila com ruído após replays: %v", kinds)
	}
}

// PAID_OUT seguido de CANCELED: cada transição é applied uma única vez e os
// replays de ambos os lados reportam duplicate sem novo efeito no razão.
func TestApplyPayoutCallbackOutcomeCancelAfterPaidOutAppliesOnce(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	user := "po-capo-" + run
	id, key, ext := "WDR-capo-"+run, "po-capo-key-"+run, "gw-capo-"+run
	repo := postgres.NewTransactionRepository(pool)
	setupLinkedPayout(t, pool, repo, user, id, key, ext)
	paid, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, ext, "PAID_OUT", 57)
	if err != nil || outcome != postgres.PayoutApplied || paid == nil || paid.Status != domain.TxStatusSucesso {
		t.Fatalf("liquidação: %+v %s %v", paid, outcome, err)
	}
	canceled, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, ext, "CANCELED", 57)
	if err != nil || outcome != postgres.PayoutApplied || canceled == nil || canceled.Status != domain.TxStatusCancelado {
		t.Fatalf("estorno: %+v %s %v", canceled, outcome, err)
	}
	for _, replay := range []string{"PAID_OUT", "CANCELED"} {
		tx, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, ext, replay, 57)
		if err != nil || outcome != postgres.PayoutDuplicate || tx == nil || tx.Status != domain.TxStatusCancelado {
			t.Fatalf("replay de %s: %+v %s %v", replay, tx, outcome, err)
		}
	}
	// O wrapper continua compatível: descarta o outcome e devolve a transação.
	if tx, err := repo.ApplyPayoutCallback(ctx, ext, "PAID_OUT", 57); err != nil || tx == nil || tx.Status != domain.TxStatusCancelado {
		t.Fatalf("wrapper incompatível: %+v %v", tx, err)
	}
	if balance := userBalance(t, pool, user); balance != 100 {
		t.Fatalf("saldo: %v", balance)
	}
	var refunds int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM financial_events WHERE transaction_id=$1 AND kind='payout_cancel_refund'`, id).Scan(&refunds); err != nil || refunds != 1 {
		t.Fatalf("estorno aplicado %d vezes (%v)", refunds, err)
	}
	if kinds := payoutQueueKinds(t, pool, ext); len(kinds) != 0 {
		t.Fatalf("fila com ruído em replays idempotentes: %v", kinds)
	}
}

// Sem vínculo, valor conflitante, status inesperado e PAID_OUT após
// cancelamento sem liquidação reportam queued, com trilha durável e nenhum
// efeito financeiro automático.
func TestApplyPayoutCallbackOutcomeConflictsAreQueued(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	run := payoutRunID()
	repo := postgres.NewTransactionRepository(pool)

	// Sem vínculo local: o INSERT do evento não prova aplicação (o evento pode
	// preceder a resposta de envio); ele fica durável e só o vínculo tardio o
	// aplica.
	userEarly := "po-qearly-" + run
	idEarly, keyEarly, extEarly := "WDR-qearly-"+run, "po-qearly-key-"+run, "gw-qearly-"+run
	seedFinancialUser(t, pool, userEarly, 100)
	reservePayoutFor(t, repo, userEarly, idEarly, keyEarly, -60)
	if _, err := repo.ClaimPayout(ctx, idEarly); err != nil {
		t.Fatal(err)
	}
	tx, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, extEarly, "PAID_OUT", 57)
	if err != nil || tx != nil || outcome != postgres.PayoutQueued {
		t.Fatalf("callback sem vínculo: %+v %s %v", tx, outcome, err)
	}
	var earlyEvents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payout_gateway_events WHERE external_id=$1 AND status='PAID_OUT'`, extEarly).Scan(&earlyEvents); err != nil || earlyEvents != 1 {
		t.Fatalf("evento sem vínculo não durável: %d (%v)", earlyEvents, err)
	}
	linked, err := repo.RecordPayoutSubmission(ctx, idEarly, extEarly)
	if err != nil || linked.Status != domain.TxStatusSucesso {
		t.Fatalf("drain do evento antecipado: %+v %v", linked, err)
	}

	// Valor conflitante com o líquido reservado.
	user := "po-queued-" + run
	id, key, ext := "WDR-queued-"+run, "po-queued-key-"+run, "gw-queued-"+run
	setupLinkedPayout(t, pool, repo, user, id, key, ext)
	tx, outcome, err = repo.ApplyPayoutCallbackWithOutcome(ctx, ext, "PAID_OUT", 99)
	if err != nil || tx != nil || outcome != postgres.PayoutQueued {
		t.Fatalf("valor conflitante: %+v %s %v", tx, outcome, err)
	}
	if kinds := payoutQueueKinds(t, pool, ext); kinds["amount_conflict"] != 1 {
		t.Fatalf("conflito de valor invisível: %v", kinds)
	}

	// Status inesperado.
	tx, outcome, err = repo.ApplyPayoutCallbackWithOutcome(ctx, ext, "IN_ANALYSIS", 57)
	if err != nil || tx != nil || outcome != postgres.PayoutQueued {
		t.Fatalf("status inesperado: %+v %s %v", tx, outcome, err)
	}
	if kinds := payoutQueueKinds(t, pool, ext); kinds["unsupported_status"] != 1 {
		t.Fatalf("status não suportado invisível: %v", kinds)
	}

	// PAID_OUT após cancelamento local sem liquidação (outro saque, para o
	// valor do evento não conflitar com o registrado).
	userAfter := "po-qafter-" + run
	idAfter, keyAfter, extAfter := "WDR-qafter-"+run, "po-qafter-key-"+run, "gw-qafter-"+run
	setupLinkedPayout(t, pool, repo, userAfter, idAfter, keyAfter, extAfter)
	canceled, outcome, err := repo.ApplyPayoutCallbackWithOutcome(ctx, extAfter, "CANCELED", 57)
	if err != nil || outcome != postgres.PayoutApplied || canceled == nil || canceled.Status != domain.TxStatusCancelado {
		t.Fatalf("estorno local: %+v %s %v", canceled, outcome, err)
	}
	tx, outcome, err = repo.ApplyPayoutCallbackWithOutcome(ctx, extAfter, "PAID_OUT", 57)
	if err != nil || tx != nil || outcome != postgres.PayoutQueued {
		t.Fatalf("PAID_OUT pós-cancelamento: %+v %s %v", tx, outcome, err)
	}
	if kinds := payoutQueueKinds(t, pool, extAfter); kinds["paid_after_cancel"] != 1 {
		t.Fatalf("conflito pós-cancelamento invisível: %v", kinds)
	}
	final, err := repo.GetByID(ctx, idAfter)
	if err != nil || final.Status != domain.TxStatusCancelado || final.PaidAt != nil {
		t.Fatalf("estado enganoso: %+v %v", final, err)
	}
	if balance := userBalance(t, pool, userAfter); balance != 100 {
		t.Fatalf("PAID_OUT tardio moveu saldo: %v", balance)
	}
}
