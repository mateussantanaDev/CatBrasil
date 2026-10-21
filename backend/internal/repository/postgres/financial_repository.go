package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Estados de saque reconhecidos pelo gateway SuitPay. Qualquer outro status
// autenticado é enfileirado sem efeito financeiro, nunca descartado.
const (
	payoutStatusPaidOut  = "PAID_OUT"
	payoutStatusCanceled = "CANCELED"
	payoutStatusMaxLen   = 32
)

// Desfechos de ApplyPayoutCallbackWithOutcome, com o mesmo vocabulário do
// cash-in (CashInApplied/Duplicate/Queued): applied mudou o estado financeiro
// nesta chamada, duplicate é um evento válido já conciliado sem novo efeito e
// queued é a trilha durável sem movimentação (sem vínculo, valor conflitante,
// status inesperado ou PAID_OUT após cancelamento sem liquidação).
const (
	PayoutApplied   = "applied"
	PayoutDuplicate = "duplicate"
	PayoutQueued    = "queued"
)

// Kind da fila payout_event_queue (payout_event_queue.kind).
const (
	payoutKindPaidAfterCancel   = "paid_after_cancel"
	payoutKindAmountConflict    = "amount_conflict"
	payoutKindAmountInvalid     = "amount_invalid"
	payoutKindInvalidState      = "invalid_state"
	payoutKindExternalConflict  = "external_conflict"
	payoutKindMissingExternalID = "missing_external_id"
	payoutKindUnsupportedStatus = "unsupported_status"
)

// payoutRefundEventKind é o lançamento único do razão para o estorno de um
// saque cancelado. A UNIQUE (transaction_id,user_id,kind) impede duplicidade.
const payoutRefundEventKind = "payout_cancel_refund"

// ConfirmDeposit owns the row locks, balance update, referral credit and journal
// in one database transaction. A duplicate callback returns the prior result.
func (r *TransactionRepository) ConfirmDeposit(ctx context.Context, id string, amount float64, provider, externalID string) (*domain.Transaction, float64, error) {
	amountCents, amountErr := money.Positive(amount)
	if amountErr != nil {
		return nil, 0, fmt.Errorf("valor PIX inválido")
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer dbtx.Rollback(ctx)
	q := fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields)
	deposit, err := scanTransaction(dbtx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, 0, err
	}
	depositCents, moneyErr := money.FromFloat64(deposit.Valor)
	if moneyErr != nil || deposit.Tipo != "Depósito PIX" || deposit.GatewayProvider != provider || externalID == "" || depositCents != amountCents {
		return nil, 0, domain.ErrInvalidTransactionStatus
	}
	if deposit.ExternalID != nil && *deposit.ExternalID != externalID {
		return nil, 0, domain.ErrInvalidTransactionStatus
	}
	if deposit.ExternalID == nil {
		if provider != "suitpay" {
			return nil, 0, domain.ErrInvalidTransactionStatus
		}
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET external_id=$2,updated_at=now() WHERE id=$1`, id, externalID); err != nil {
			return nil, 0, err
		}
		e := externalID
		deposit.ExternalID = &e
	}
	if deposit.Status != domain.TxStatusPendente && deposit.Status != domain.TxStatusSucesso {
		return nil, 0, domain.ErrInvalidTransactionStatus
	}
	var before float64
	if err = dbtx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, deposit.UserID).Scan(&before); err != nil {
		return nil, 0, err
	}
	if deposit.Status == domain.TxStatusSucesso {
		return deposit, before, dbtx.Commit(ctx)
	}
	var after float64
	if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance+$1, required_rollover=required_rollover+$1, updated_at=now() WHERE id=$2 RETURNING balance`, amountCents.String(), deposit.UserID).Scan(&after); err != nil {
		return nil, 0, err
	}
	if err = journal(ctx, dbtx, deposit.ID, deposit.UserID, "deposit", amount, before, after); err != nil {
		return nil, 0, err
	}
	now := time.Now()
	deposit, err = scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET status='sucesso', valor_efetivo_pago=$2, paid_at=$3, reconciled_at=$3, updated_at=$3 WHERE id=$1 RETURNING %s`, transactionSelectFields), id, amountCents.String(), now))
	if err != nil {
		return nil, 0, err
	}
	// Activate the referral at most once, under its own row lock. A missing
	// sponsor or failed commission rolls the entire deposit back.
	var referralID, sponsorID string
	refErr := dbtx.QueryRow(ctx, `SELECT r.id,r.referrer_id FROM referrals r JOIN users u ON u.id=$1 WHERE (r.referred_user_id=$1 OR (r.referred_user_id IS NULL AND r.nome=u.username)) AND r.status<>'ativo' ORDER BY r.id LIMIT 1 FOR UPDATE OF r`, deposit.UserID).Scan(&referralID, &sponsorID)
	if refErr != nil && !errors.Is(refErr, pgx.ErrNoRows) {
		return nil, 0, refErr
	}
	if refErr == nil {
		if _, err = dbtx.Exec(ctx, `UPDATE referrals SET status='ativo', deposito=$2, updated_at=now() WHERE id=$1`, referralID, amountCents.String()); err != nil {
			return nil, 0, err
		}
		var sponsorBefore, percent float64
		var suspended, multiAccount, level12Suspended bool
		var level int
		err = dbtx.QueryRow(ctx, `SELECT balance, custom_referral_commission_percent, referral_bonus_suspended, multi_account_flag, nivel, level12_bonus_suspended FROM users WHERE id=$1 FOR UPDATE`, sponsorID).Scan(&sponsorBefore, &percent, &suspended, &multiAccount, &level, &level12Suspended)
		if err != nil {
			return nil, 0, err
		}
		if !suspended && !multiAccount {
			if percent <= 0 || (level >= 12 && level12Suspended) {
				percent = 5
			}
			basisPoints, percentErr := money.BasisPoints(percent)
			if percentErr != nil {
				return nil, 0, percentErr
			}
			commissionCents, percentErr := amountCents.Percent(basisPoints)
			if percentErr != nil {
				return nil, 0, percentErr
			}
			// A small deposit can legitimately round a percentage commission
			// down to zero cents. The deposit and referral activation still settle.
			if commissionCents == 0 {
				return deposit, after, dbtx.Commit(ctx)
			}
			commission := commissionCents.Float64()
			if !validMoney(commission) {
				return nil, 0, fmt.Errorf("comissão inválida")
			}
			commissionID, idErr := financialID("COM")
			if idErr != nil {
				return nil, 0, idErr
			}
			var sponsorAfter float64
			if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance+$1,updated_at=now() WHERE id=$2 RETURNING balance`, commissionCents.String(), sponsorID).Scan(&sponsorAfter); err != nil {
				return nil, 0, err
			}
			_, err = dbtx.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,gateway_provider,reference_id,paid_at) VALUES ($1,$2,'Comissão de primeiro depósito',$3,'sucesso',$4,$5,$6)`, commissionID, sponsorID, commissionCents.String(), provider, deposit.ID, now)
			if err != nil {
				return nil, 0, err
			}
			if err = journal(ctx, dbtx, commissionID, sponsorID, "referral_commission", commission, sponsorBefore, sponsorAfter); err != nil {
				return nil, 0, err
			}
		}
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return deposit, after, nil
}

func validMoney(value float64) bool {
	_, err := money.Positive(value)
	return err == nil
}

func journal(ctx context.Context, tx pgx.Tx, id, userID, kind string, amount, before, after float64) error {
	amountCents, err := money.FromFloat64(amount)
	if err != nil {
		return err
	}
	beforeCents, err := money.FromFloat64(before)
	if err != nil {
		return err
	}
	afterCents, err := money.FromFloat64(after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO financial_events (transaction_id,user_id,kind,amount,balance_before,balance_after) VALUES ($1,$2,$3,$4,$5,$6)`, id, userID, kind, amountCents.String(), beforeCents.String(), afterCents.String())
	return err
}

// ReservePayout validates a locked account and reserves funds before any
// request is sent to the gateway. A repeated key returns the same withdrawal.
func (r *TransactionRepository) ReservePayout(ctx context.Context, withdrawal *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
	if withdrawal.IdempotencyKey == nil || strings.TrimSpace(*withdrawal.IdempotencyKey) == "" || !validMoney(-withdrawal.Valor) {
		return nil, 0, false, fmt.Errorf("saque ou chave de idempotência inválidos")
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, false, err
	}
	defer dbtx.Rollback(ctx)
	var globalLocked bool
	if err = dbtx.QueryRow(ctx, `SELECT global_withdrawals_locked FROM system_config WHERE id=1 FOR SHARE`).Scan(&globalLocked); err != nil {
		return nil, 0, false, err
	}
	if globalLocked {
		return nil, 0, false, domain.ErrGlobalWithdrawLocked
	}
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), withdrawal.UserID))
	if err != nil {
		return nil, 0, false, err
	}
	q := fmt.Sprintf("SELECT %s FROM transactions WHERE user_id=$1 AND idempotency_key=$2 AND tipo='Saque PIX' FOR UPDATE", transactionSelectFields)
	existing, err := scanTransaction(dbtx.QueryRow(ctx, q, user.ID, *withdrawal.IdempotencyKey))
	if err == nil {
		if !money.EqualFloat64(existing.Valor, withdrawal.Valor) || existing.PixKey == nil || withdrawal.PixKey == nil || *existing.PixKey != *withdrawal.PixKey {
			return nil, 0, false, fmt.Errorf("chave de idempotência reutilizada com dados diferentes")
		}
		return existing, user.Balance, true, dbtx.Commit(ctx)
	}
	if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, 0, false, err
	}
	var failures int
	var lockedUntil *time.Time
	if err = dbtx.QueryRow(ctx, `SELECT withdraw_pin_failures,withdraw_pin_locked_until FROM users WHERE id=$1`, user.ID).Scan(&failures, &lockedUntil); err != nil {
		return nil, 0, false, err
	}
	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		return nil, 0, false, domain.ErrFinancialAuthLocked
	}
	if lockedUntil != nil {
		failures = 0
	}
	if validate != nil {
		if err = validate(user); err != nil {
			if errors.Is(err, domain.ErrInvalidSecurityPassword) || errors.Is(err, domain.ErrInvalid2FACode) {
				failures++
				if _, updateErr := dbtx.Exec(ctx, `UPDATE users SET withdraw_pin_failures=$2,withdraw_pin_locked_until=CASE WHEN $2>=5 THEN now()+interval '15 minutes' ELSE NULL END,updated_at=now() WHERE id=$1`, user.ID, failures); updateErr != nil {
					return nil, 0, false, updateErr
				}
				if commitErr := dbtx.Commit(ctx); commitErr != nil {
					return nil, 0, false, commitErr
				}
			}
			return nil, 0, false, err
		}
	}
	amount := -withdrawal.Valor
	amountCents, amountErr := money.Positive(amount)
	if amountErr != nil {
		return nil, 0, false, amountErr
	}
	balanceCents, balanceErr := money.FromFloat64(user.Balance)
	if balanceErr != nil {
		return nil, 0, false, balanceErr
	}
	if balanceCents < amountCents {
		return nil, 0, false, domain.ErrInsufficientBalance
	}
	feeCents, feeErr := amountCents.Percent(500)
	if feeErr != nil {
		return nil, 0, false, feeErr
	}
	if user.FreeWithdrawActive {
		feeCents = 0
	}
	netCents := amountCents - feeCents
	fee, net := feeCents.Float64(), netCents.Float64()
	withdrawal.NetAmount, withdrawal.FeeAmount = &net, &fee
	withdrawal.Status = domain.TxStatusPendente
	if withdrawal.CreatedAt.IsZero() {
		withdrawal.CreatedAt = time.Now()
	}
	withdrawal.UpdatedAt = withdrawal.CreatedAt
	_, err = dbtx.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,pix_key,net_amount,fee_amount,gateway_provider,idempotency_key,created_at,updated_at) VALUES ($1,$2,'Saque PIX',$3,'pendente',$4,$5,$6,$7,$8,$9,$9)`, withdrawal.ID, user.ID, (-amountCents).String(), withdrawal.PixKey, netCents.String(), feeCents.String(), withdrawal.GatewayProvider, withdrawal.IdempotencyKey, withdrawal.CreatedAt)
	if err != nil {
		return nil, 0, false, err
	}
	var after float64
	if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance-$1,free_withdraw_active=false,withdraw_pin_failures=0,withdraw_pin_locked_until=NULL,updated_at=now() WHERE id=$2 RETURNING balance`, amountCents.String(), user.ID).Scan(&after); err != nil {
		return nil, 0, false, err
	}
	if err = journal(ctx, dbtx, withdrawal.ID, user.ID, "payout_reserve", -amount, user.Balance, after); err != nil {
		return nil, 0, false, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, 0, false, err
	}
	return withdrawal, after, false, nil
}

// ClaimPayout is a one-way transition. Unknown gateway outcomes remain in
// processando for reconciliation; they are never sent a second time blindly.
// The claim durably records the internal stable id that is about to be handed
// to the gateway (payout_submissions), so an ambiguous or missing response
// keeps triage evidence instead of leaving an anonymous processando row.
func (r *TransactionRepository) ClaimPayout(ctx context.Context, id string) (*domain.Transaction, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	q := fmt.Sprintf(`UPDATE transactions SET status='processando', payout_attempted_at=now(),updated_at=now() WHERE id=$1 AND tipo='Saque PIX' AND status='pendente' AND payout_attempted_at IS NULL RETURNING %s`, transactionSelectFields)
	withdrawal, err := scanTransaction(dbtx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	// internal_ref = ID interno estável enviado ao provedor como ExternalID.
	if _, err = dbtx.Exec(ctx, `INSERT INTO payout_submissions (transaction_id, provider, internal_ref) VALUES ($1,$2,$3) ON CONFLICT (transaction_id) DO NOTHING`, withdrawal.ID, withdrawal.GatewayProvider, withdrawal.ID); err != nil {
		return nil, err
	}
	return withdrawal, dbtx.Commit(ctx)
}

// RecordPayoutSubmission durably associates the gateway's returned idTransaction
// with the internal stable id sent in the request (payout_submissions), then
// drains authenticated events that arrived before the response. Conflicting
// events are queued for manual review instead of blocking the link forever or
// moving money; a return without an external id is never linked by inference.
func (r *TransactionRepository) RecordPayoutSubmission(ctx context.Context, id, externalID string) (*domain.Transaction, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		// Retorno incompleto: nenhum vínculo é inferido.
		return nil, fmt.Errorf("ID do gateway ausente")
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	if err = lockPayoutExternalID(ctx, dbtx, externalID); err != nil {
		return nil, err
	}
	withdrawal, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields), id))
	if err != nil {
		return nil, err
	}
	if withdrawal.Tipo != "Saque PIX" {
		return nil, domain.ErrInvalidTransactionStatus
	}
	if withdrawal.Status != domain.TxStatusProcessando && withdrawal.Status != domain.TxStatusSucesso && withdrawal.Status != domain.TxStatusCancelado {
		// pendente/rejeitado nunca foi enviado ao provedor: vincular seria
		// inferir um payout a partir de um retorno incompleto.
		if err = r.queuePayoutReview(ctx, dbtx, id, externalID, "", 0, payoutKindInvalidState, fmt.Sprintf("retorno do gateway para saque em estado %q (nunca enviado ao provedor)", withdrawal.Status)); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidTransactionStatus
	}
	if withdrawal.ExternalID != nil && *withdrawal.ExternalID != externalID {
		if err = r.queuePayoutReview(ctx, dbtx, id, externalID, "", 0, payoutKindExternalConflict, fmt.Sprintf("saque %s já vinculado ao retorno %s; resposta divergente %s", id, *withdrawal.ExternalID, externalID)); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidTransactionStatus
	}
	// Um retorno só pode pertencer a um único saque local.
	var bound string
	err = dbtx.QueryRow(ctx, `SELECT id FROM transactions WHERE tipo='Saque PIX' AND external_id=$1 AND id<>$2`, externalID, id).Scan(&bound)
	if err == nil {
		if err = r.queuePayoutReview(ctx, dbtx, id, externalID, "", 0, payoutKindExternalConflict, fmt.Sprintf("retorno %s já vinculado ao saque %s", externalID, bound)); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidTransactionStatus
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	err = dbtx.QueryRow(ctx, `SELECT transaction_id FROM payout_submissions WHERE external_id=$1 AND transaction_id<>$2`, externalID, id).Scan(&bound)
	if err == nil {
		if err = r.queuePayoutReview(ctx, dbtx, id, externalID, "", 0, payoutKindExternalConflict, fmt.Sprintf("retorno %s já registrado na submissão do saque %s", externalID, bound)); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidTransactionStatus
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Vincular exige duas escritas com índice único (transactions.external_id e
	// payout_submissions). Depois de um 23505 a transação PostgreSQL fica
	// abortada, então as escritas correm sob savepoint: o rollback devolve a
	// capacidade de enfileirar a evidência e commitar no mesmo ciclo.
	if _, err = dbtx.Exec(ctx, `SAVEPOINT payout_link`); err != nil {
		return nil, err
	}
	if withdrawal.ExternalID == nil {
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET external_id=$2,updated_at=now() WHERE id=$1`, id, externalID); err != nil {
			if isUniqueViolation(err) {
				return r.abortPayoutLink(ctx, dbtx, id, externalID, fmt.Sprintf("retorno %s já pertence a outro saque local", externalID))
			}
			return nil, err
		}
		external := externalID
		withdrawal.ExternalID = &external
	}
	if _, err = dbtx.Exec(ctx, `
		INSERT INTO payout_submissions (transaction_id, provider, internal_ref, external_id, linked_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (transaction_id) DO UPDATE SET
			external_id = COALESCE(payout_submissions.external_id, EXCLUDED.external_id),
			linked_at = COALESCE(payout_submissions.linked_at, EXCLUDED.linked_at),
			updated_at = now()`,
		id, withdrawal.GatewayProvider, id, externalID); err != nil {
		if isUniqueViolation(err) {
			return r.abortPayoutLink(ctx, dbtx, id, externalID, fmt.Sprintf("retorno %s já registrado em outra submissão", externalID))
		}
		return nil, err
	}
	if _, err = dbtx.Exec(ctx, `RELEASE SAVEPOINT payout_link`); err != nil {
		return nil, err
	}
	rows, err := dbtx.Query(ctx, `SELECT status,amount FROM payout_gateway_events WHERE external_id=$1 ORDER BY CASE status WHEN 'PAID_OUT' THEN 0 ELSE 1 END`, externalID)
	if err != nil {
		return nil, err
	}
	type event struct {
		status string
		amount float64
	}
	var pending []event
	for rows.Next() {
		var e event
		if err = rows.Scan(&e.status, &e.amount); err != nil {
			rows.Close()
			return nil, err
		}
		pending = append(pending, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, e := range pending {
		// Eventos conflitantes vão para a fila em vez de desfazer o vínculo:
		// submissão e eventos autenticados nunca se perdem por conflito.
		applied, review, _, applyErr := applyPayoutState(ctx, dbtx, withdrawal, externalID, e.status, e.amount)
		if applyErr != nil {
			return nil, applyErr
		}
		if applied != nil {
			withdrawal = applied
		}
		if review != nil {
			if err = r.queuePayoutReview(ctx, dbtx, withdrawal.ID, externalID, e.status, e.amount, review.kind, review.reason); err != nil {
				return nil, err
			}
		}
	}
	return withdrawal, dbtx.Commit(ctx)
}

// abortPayoutLink desfaz as escritas do vínculo até o savepoint e grava a
// evidência em fila: um 23505 aborta a transação PostgreSQL, e somente o
// rollback ao savepoint devolve a capacidade de enfileirar e commitar sem
// perder o retorno autenticado.
func (r *TransactionRepository) abortPayoutLink(ctx context.Context, dbtx pgx.Tx, id, externalID, reason string) (*domain.Transaction, error) {
	if _, err := dbtx.Exec(ctx, `ROLLBACK TO SAVEPOINT payout_link`); err != nil {
		return nil, err
	}
	if err := r.queuePayoutReview(ctx, dbtx, id, externalID, "", 0, payoutKindExternalConflict, reason); err != nil {
		return nil, err
	}
	if err := dbtx.Commit(ctx); err != nil {
		return nil, err
	}
	return nil, domain.ErrInvalidTransactionStatus
}

func (r *TransactionRepository) ConfirmPayout(ctx context.Context, id, externalID string, amount float64) (*domain.Transaction, error) {
	return r.payoutTransition(ctx, id, externalID, payoutStatusPaidOut, amount)
}

func (r *TransactionRepository) CancelPayout(ctx context.Context, id, externalID string, amount float64) (*domain.Transaction, error) {
	return r.payoutTransition(ctx, id, externalID, payoutStatusCanceled, amount)
}

func (r *TransactionRepository) payoutTransition(ctx context.Context, id, externalID, status string, amount float64) (*domain.Transaction, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	q := fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields)
	withdrawal, err := scanTransaction(dbtx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	applied, review, _, err := applyPayoutState(ctx, dbtx, withdrawal, externalID, status, amount)
	if err != nil {
		return nil, err
	}
	if review != nil {
		// A evidência autenticada é commitada antes do erro: o chamador vê a
		// falha e a operação permanece visível para revisão manual.
		if err = r.queuePayoutReview(ctx, dbtx, id, externalID, status, amount, review.kind, review.reason); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("saque %s exige revisão manual: %s", id, review.reason)
	}
	return applied, dbtx.Commit(ctx)
}

func (r *TransactionRepository) GetPayoutByExternalID(ctx context.Context, externalID string) (*domain.Transaction, error) {
	q := fmt.Sprintf("SELECT %s FROM transactions WHERE tipo='Saque PIX' AND gateway_provider='suitpay' AND external_id=$1", transactionSelectFields)
	return scanTransaction(r.pool.QueryRow(ctx, q, externalID))
}

// ApplyPayoutCallback persists a verified event even if it beats the gateway
// response. RecordPayoutSubmission drains it under the same advisory lock.
// Every authenticated callback leaves a durable trail: it is applied, or — when
// the amount conflicts, the status is unsupported, the return cannot be bound
// or PAID_OUT arrives after a local cancellation — it is committed to
// payout_event_queue for manual review. An authenticated event is never lost to
// a rollback, money never moves twice and queued outcomes return (nil, nil):
// acknowledged for reconciliation without a state or balance change.
// Compatibility wrapper over ApplyPayoutCallbackWithOutcome that discards the
// outcome.
func (r *TransactionRepository) ApplyPayoutCallback(ctx context.Context, externalID, status string, amount float64) (*domain.Transaction, error) {
	withdrawal, _, err := r.ApplyPayoutCallbackWithOutcome(ctx, externalID, status, amount)
	return withdrawal, err
}

// ApplyPayoutCallbackWithOutcome applies an authenticated cash-out callback and
// reports what this call did, so the WebSocket layer can acknowledge without
// duplicating notices:
//
//   - PayoutApplied: the withdrawal changed financial state for the first time
//     in this call (settlement or refund); the updated transaction is returned.
//   - PayoutDuplicate: a valid event already reconciled; the current
//     transaction is returned with no new effect.
//   - PayoutQueued: no usable link, a conflicting amount, an unsupported
//     status or PAID_OUT after a local cancellation without settlement. The
//     durable trail is committed and (nil, PayoutQueued, nil) is returned.
//
// The insert into payout_gateway_events never proves application by itself: an
// event can precede the gateway response that carries the external id, so the
// outcome is decided by the state machine under the same advisory lock and row
// lock used by RecordPayoutSubmission.
func (r *TransactionRepository) ApplyPayoutCallbackWithOutcome(ctx context.Context, externalID, status string, amount float64) (*domain.Transaction, string, error) {
	externalID = strings.TrimSpace(externalID)
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "" {
		status = "UNKNOWN"
	}
	if len(status) > payoutStatusMaxLen {
		return nil, "", domain.ErrInvalidTransactionStatus
	}
	// Sem external_id não há como deduplicar nem vincular: nenhum payout é
	// inferido, nenhum efeito financeiro ocorre e a evidência fica na fila.
	if externalID == "" {
		return nil, PayoutQueued, r.queuePayoutReviewAndCommit(ctx, "", "", status, amount, payoutKindMissingExternalID, "callback de saque sem external_id do provedor; nenhum payout inferido")
	}
	if !validMoney(amount) {
		return nil, PayoutQueued, r.queuePayoutReviewAndCommit(ctx, "", externalID, status, amount, payoutKindAmountInvalid, "valor de callback não transacionável; nenhum efeito financeiro")
	}
	amountCents, _ := money.Positive(amount)
	if status != payoutStatusPaidOut && status != payoutStatusCanceled {
		return nil, PayoutQueued, r.queuePayoutReviewAndCommit(ctx, "", externalID, status, amount, payoutKindUnsupportedStatus, fmt.Sprintf("status de saque não suportado (%s); nenhum efeito financeiro", status))
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer dbtx.Rollback(ctx)
	if err = lockPayoutExternalID(ctx, dbtx, externalID); err != nil {
		return nil, "", err
	}
	var storedAmount float64
	var inserted bool
	if err = dbtx.QueryRow(ctx, `INSERT INTO payout_gateway_events (external_id,status,amount) VALUES ($1,$2,$3) ON CONFLICT (external_id,status) DO UPDATE SET received_at=payout_gateway_events.received_at RETURNING amount,(xmax=0) AS inserted`, externalID, status, amountCents.String()).Scan(&storedAmount, &inserted); err != nil {
		return nil, "", err
	}
	if !inserted && !money.EqualFloat64(storedAmount, amount) {
		// Valor conflitante: o registro mantém o primeiro valor autenticado e o
		// segundo fica visível na fila segura, sem tocar o saldo.
		if err = r.queuePayoutReview(ctx, dbtx, "", externalID, status, amount, payoutKindAmountConflict, fmt.Sprintf("valor conflitante no evento %s/%s: registrado %.2f, recebido %.2f", externalID, status, storedAmount, amount)); err != nil {
			return nil, "", err
		}
		return nil, PayoutQueued, dbtx.Commit(ctx)
	}
	withdrawal, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE tipo='Saque PIX' AND gateway_provider='suitpay' AND external_id=$1 FOR UPDATE", transactionSelectFields), externalID))
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			// Sem payout local o evento permanece durável para o vínculo tardio
			// (RecordPayoutSubmission) ou para triagem (unmatched_callback). O
			// INSERT do evento nunca é prova de aplicação.
			return nil, PayoutQueued, dbtx.Commit(ctx)
		}
		return nil, "", err
	}
	applied, review, changed, err := applyPayoutState(ctx, dbtx, withdrawal, externalID, status, amount)
	if err != nil {
		return nil, "", err
	}
	if review != nil {
		if err = r.queuePayoutReview(ctx, dbtx, withdrawal.ID, externalID, status, amount, review.kind, review.reason); err != nil {
			return nil, "", err
		}
		return nil, PayoutQueued, dbtx.Commit(ctx)
	}
	if !changed {
		return applied, PayoutDuplicate, dbtx.Commit(ctx)
	}
	return applied, PayoutApplied, dbtx.Commit(ctx)
}

// payoutReview descreve um evento autenticado que exige revisão manual. O
// chamador deve enfileirá-lo e commitar; nenhum estado ou saldo é alterado.
type payoutReview struct {
	kind   string
	reason string
}

// lockPayoutExternalID serializa submissão e callbacks pelo mesmo external_id.
func lockPayoutExternalID(ctx context.Context, dbtx pgx.Tx, externalID string) error {
	_, err := dbtx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 718819))`, externalID)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// payoutReviewDedupKey monta a identidade estável de um item da fila. Valores
// conflitantes diferentes são evidências diferentes e nunca se sobrescrevem;
// um replay idêntico (mesmo valor) colapsa na mesma linha.
func payoutReviewDedupKey(transactionID, externalID, status string, amount float64, kind string) string {
	ref := strings.TrimSpace(externalID)
	if ref == "" {
		ref = strings.TrimSpace(transactionID)
	}
	if ref == "" {
		ref = "sem-ref"
	}
	return fmt.Sprintf("payout:%s:%s:%s:%.2f", ref, status, kind, amount)
}

// queuePayoutReview grava um item pendente deduplicado na fila de revisão.
func (r *TransactionRepository) queuePayoutReview(ctx context.Context, dbtx pgx.Tx, transactionID, externalID, status string, amount float64, kind, reason string) error {
	var amountArg any
	if validMoney(amount) {
		cents, _ := money.Positive(amount)
		amountArg = cents.String()
	}
	_, err := dbtx.Exec(ctx, `
		INSERT INTO payout_event_queue (dedup_key, kind, transaction_id, external_id, gateway_status, amount, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (dedup_key) DO UPDATE SET
			attempts = payout_event_queue.attempts + 1,
			reason = EXCLUDED.reason,
			queue_status = 'pending',
			resolved_at = NULL,
			resolved_by = NULL,
			evidence_ref = NULL,
			resolution_note = NULL`,
		payoutReviewDedupKey(transactionID, externalID, status, amount, kind), kind,
		nullableString(transactionID), nullableString(externalID), nullableString(status), amountArg, reason)
	return err
}

// queuePayoutReviewAndCommit é a variante para caminhos sem transação aberta.
func (r *TransactionRepository) queuePayoutReviewAndCommit(ctx context.Context, transactionID, externalID, status string, amount float64, kind, reason string) error {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbtx.Rollback(ctx)
	if err = r.queuePayoutReview(ctx, dbtx, transactionID, externalID, status, amount, kind, reason); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}

// applyPayoutState executa a máquina de estados do cash-out dentro da
// transação do chamador. Uma revisão não nula significa que o evento
// autenticado deve ser enfileirado sem alterar estado ou saldo. Dinheiro só se
// move em dois casos: liquidação (PAID_OUT) e estorno/chargeback único do
// gateway (CANCELED), documentado e idempotente pelo razão. changed relata se
// esta chamada moveu o estado financeiro pela primeira vez; um replay
// idempotente devolve changed=false sem revisão.
func applyPayoutState(ctx context.Context, dbtx pgx.Tx, withdrawal *domain.Transaction, externalID, status string, amount float64) (*domain.Transaction, *payoutReview, bool, error) {
	if withdrawal.Tipo != "Saque PIX" {
		return nil, &payoutReview{kind: payoutKindInvalidState, reason: fmt.Sprintf("evento %s para transação de tipo %q", status, withdrawal.Tipo)}, false, nil
	}
	if withdrawal.ExternalID == nil || *withdrawal.ExternalID != externalID {
		return nil, &payoutReview{kind: payoutKindExternalConflict, reason: fmt.Sprintf("evento %s/%s sem correspondência com o vínculo %q do saque %s", status, externalID, derefString(withdrawal.ExternalID, ""), withdrawal.ID)}, false, nil
	}
	if withdrawal.NetAmount == nil || !money.EqualFloat64(*withdrawal.NetAmount, amount) {
		return nil, &payoutReview{kind: payoutKindAmountConflict, reason: fmt.Sprintf("valor do evento %.2f diverge do líquido reservado (%s) no saque %s", amount, formatNullableMoney(withdrawal.NetAmount), withdrawal.ID)}, false, nil
	}
	switch status {
	case payoutStatusPaidOut:
		switch withdrawal.Status {
		case domain.TxStatusSucesso:
			// Callback duplicado: liquidação já aplicada, nenhum novo efeito.
			return withdrawal, nil, false, nil
		case domain.TxStatusProcessando:
			settled, settleErr := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET status='sucesso',paid_at=now(),reconciled_at=now(),updated_at=now() WHERE id=$1 RETURNING %s`, transactionSelectFields), withdrawal.ID))
			if settleErr != nil {
				return nil, nil, false, settleErr
			}
			return settled, nil, true, nil
		case domain.TxStatusCancelado:
			// paid_at só é gravado pela liquidação (PAID_OUT). Se está presente,
			// o ciclo liquidação→estorno já foi concluído e este callback é um
			// replay idempotente. Sem paid_at, o cancelamento local nunca teve
			// liquidação e o PAID_OUT é um conflito: sem reenvio e sem dinheiro.
			if withdrawal.PaidAt != nil {
				return withdrawal, nil, false, nil
			}
			if _, err := dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, withdrawal.UserID); err != nil {
				return nil, nil, false, err
			}
			return nil, &payoutReview{kind: payoutKindPaidAfterCancel, reason: fmt.Sprintf("PAID_OUT recebido após cancelamento local do saque %s (saldo já estornado); sem reenvio e sem movimentação automática", withdrawal.ID)}, false, nil
		default:
			return nil, &payoutReview{kind: payoutKindInvalidState, reason: fmt.Sprintf("PAID_OUT em estado incompatível (%s) para o saque %s", withdrawal.Status, withdrawal.ID)}, false, nil
		}
	case payoutStatusCanceled:
		switch withdrawal.Status {
		case domain.TxStatusCancelado:
			// Duplicado: o estorno já foi creditado uma única vez.
			return withdrawal, nil, false, nil
		case domain.TxStatusProcessando, domain.TxStatusSucesso:
			// Estorno/chargeback (docs SuitPay): devolve o valor bruto do
			// saque uma única vez. A UNIQUE (transaction_id,user_id,kind) do
			// razão impede um segundo estorno mesmo sob bug de lógica.
			refund := -withdrawal.Valor
			refundCents, refundErr := money.Positive(refund)
			if refundErr != nil {
				return nil, &payoutReview{kind: payoutKindInvalidState, reason: fmt.Sprintf("valor de estorno não transacionável (%v) no saque %s", refund, withdrawal.ID)}, false, nil
			}
			var before, after float64
			if err := dbtx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, withdrawal.UserID).Scan(&before); err != nil {
				return nil, nil, false, err
			}
			if err := dbtx.QueryRow(ctx, `UPDATE users SET balance=balance+$1,updated_at=now() WHERE id=$2 RETURNING balance`, refundCents.String(), withdrawal.UserID).Scan(&after); err != nil {
				return nil, nil, false, err
			}
			if err := journal(ctx, dbtx, withdrawal.ID, withdrawal.UserID, payoutRefundEventKind, refund, before, after); err != nil {
				return nil, nil, false, err
			}
			canceled, cancelErr := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET status='cancelado',rejection_reason='Cancelado pela SuitPay',reconciled_at=now(),updated_at=now() WHERE id=$1 RETURNING %s`, transactionSelectFields), withdrawal.ID))
			if cancelErr != nil {
				return nil, nil, false, cancelErr
			}
			return canceled, nil, true, nil
		default:
			return nil, &payoutReview{kind: payoutKindInvalidState, reason: fmt.Sprintf("CANCELED em estado incompatível (%s) para o saque %s", withdrawal.Status, withdrawal.ID)}, false, nil
		}
	default:
		return nil, &payoutReview{kind: payoutKindUnsupportedStatus, reason: fmt.Sprintf("status de saque não suportado (%s) para o saque %s", status, withdrawal.ID)}, false, nil
	}
}

func formatNullableMoney(value *float64) string {
	if value == nil {
		return "ausente"
	}
	return fmt.Sprintf("%.2f", *value)
}

// ListFinancialReconciliation gives operators a bounded queue of uncertain
// gateway outcomes and callbacks that could not yet be matched to a payout. It
// also surfaces the durable cash-in reconciliation worklist (unmatched deposit
// callbacks, ambiguous intents, missing QR/instructions and orphan deposits) so
// the administrative endpoint reports cash-in events too, plus the cash-out
// review queue (conflicting amounts, PAID_OUT after local cancellation,
// unsupported statuses and divergent returns). Results are de-duplicated by
// (kind, transaction, external id, queue id) and capped at 200.
func (r *TransactionRepository) ListFinancialReconciliation(ctx context.Context) ([]domain.ReconciliationIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id,user_id,COALESCE(external_id,''),'payout_processing',-valor,payout_attempted_at
		FROM transactions
		WHERE tipo='Saque PIX' AND status='processando' AND payout_attempted_at < now()-interval '5 minutes'
		UNION ALL
		SELECT '','',e.external_id,'unmatched_callback',e.amount,e.received_at
		FROM payout_gateway_events e
		WHERE e.received_at < now()-interval '5 minutes'
		AND NOT EXISTS (SELECT 1 FROM transactions t WHERE t.tipo='Saque PIX' AND t.gateway_provider='suitpay' AND t.external_id=e.external_id)
		UNION ALL
		SELECT '',u.id,'','balance_mismatch',u.balance-latest.balance_after,u.updated_at
		FROM users u
		JOIN LATERAL (SELECT balance_after FROM financial_events WHERE user_id=u.id ORDER BY id DESC LIMIT 1) latest ON true
		WHERE u.balance<>latest.balance_after
		ORDER BY 6 ASC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	issues := make([]domain.ReconciliationIssue, 0)
	seen := make(map[string]bool)
	for rows.Next() {
		var item domain.ReconciliationIssue
		if err := rows.Scan(&item.TransactionID, &item.UserID, &item.ExternalID, &item.Kind, &item.Amount, &item.Since); err != nil {
			rows.Close()
			return nil, err
		}
		key := reconciliationIssueKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		issues = append(issues, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}

	cashInItems, err := r.ListCashInReconciliation(ctx)
	if err != nil {
		return nil, err
	}
	for _, ci := range cashInItems {
		issue := domain.ReconciliationIssue{
			TransactionID: ci.RequestNumber,
			ExternalID:    ci.ExternalID,
			Kind:          "cashin_" + ci.Kind,
			Amount:        ci.Amount,
			Since:         ci.Since,
			QueueID:       ci.QueueID,
			Reason:        ci.Reason,
		}
		key := reconciliationIssueKey(issue)
		if seen[key] {
			continue
		}
		seen[key] = true
		issues = append(issues, issue)
	}

	payoutQueueItems, err := r.listPayoutQueueIssues(ctx)
	if err != nil {
		return nil, err
	}
	for _, pi := range payoutQueueItems {
		key := reconciliationIssueKey(pi)
		if seen[key] {
			continue
		}
		seen[key] = true
		issues = append(issues, pi)
	}

	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Since.Before(issues[j].Since) })
	if len(issues) > 200 {
		issues = issues[:200]
	}
	return issues, nil
}

// listPayoutQueueIssues devolve os itens pendentes da fila de revisão de
// saques (valores conflitantes, PAID_OUT após cancelamento local, status não
// suportado e retornos sem/divergentes de vínculo) para o quadro operacional.
func (r *TransactionRepository) listPayoutQueueIssues(ctx context.Context) ([]domain.ReconciliationIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT q.id,COALESCE(q.transaction_id,''),COALESCE(t.user_id,''),COALESCE(q.external_id,''),q.kind,COALESCE(q.amount,0),q.created_at,q.reason
		FROM payout_event_queue q
		LEFT JOIN transactions t ON t.id=q.transaction_id
		WHERE q.queue_status='pending'
		ORDER BY q.created_at ASC
		LIMIT 200`)
	if err != nil {
		return nil, err
	}
	issues := make([]domain.ReconciliationIssue, 0)
	for rows.Next() {
		var item domain.ReconciliationIssue
		if err := rows.Scan(&item.QueueID, &item.TransactionID, &item.UserID, &item.ExternalID, &item.Kind, &item.Amount, &item.Since, &item.Reason); err != nil {
			rows.Close()
			return nil, err
		}
		issues = append(issues, item)
	}
	err = rows.Err()
	rows.Close()
	return issues, err
}

// ResolvePayoutQueueItem encerra um item pendente da fila de revisão de
// saques. Como na fila de cash-in, a resolução exige operador, evidência
// objetiva e razão; por si só ela nunca movimenta saldo nem reenvia transferência.
func (r *TransactionRepository) ResolvePayoutQueueItem(ctx context.Context, id int64, operatorID, evidence, reason string) error {
	if strings.TrimSpace(operatorID) == "" || strings.TrimSpace(evidence) == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("resolução da fila de saque exige operador, evidência e razão")
	}
	tag, err := r.pool.Exec(ctx, `UPDATE payout_event_queue SET queue_status='resolved',resolved_at=now(),resolved_by=$2,evidence_ref=$3,resolution_note=$4 WHERE id=$1 AND queue_status='pending'`, id, operatorID, evidence, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("item %d da fila de saque não está pendente", id)
	}
	return nil
}

func reconciliationIssueKey(issue domain.ReconciliationIssue) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d", issue.Kind, issue.TransactionID, issue.UserID, issue.ExternalID, issue.QueueID)
}
