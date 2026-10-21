package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"

	"github.com/jackc/pgx/v5"
)

// Cash-in reconciliation outcomes returned by ApplyDepositEvent.
const (
	CashInApplied   = "applied"
	CashInDuplicate = "duplicate"
	CashInQueued    = "queued"
)

// Gateway statuses accepted for deposit events. Any other authenticated status
// is durably recorded and queued, never credited.
const (
	cashInStatusPaidOut    = "PAID_OUT"
	cashInStatusChargeback = "CHARGEBACK"
	cashInProvider         = "suitpay"
)

// Kinds stored in cashin_event_queue for operator-visible reconciliation.
const (
	cashInKindPremature            = "premature_callback"
	cashInKindAmountConflict       = "amount_conflict"
	cashInKindAmountInvalid        = "amount_invalid"
	cashInKindExternalConflict     = "external_conflict"
	cashInKindRequestConflict      = "request_conflict"
	cashInKindInvalidTarget        = "invalid_target"
	cashInKindInvalidState         = "invalid_state"
	cashInKindNonFinalStatus       = "non_final_status"
	cashInKindChargebackBeforePaid = "chargeback_before_credit"
	cashInKindChargebackNoBalance  = "chargeback_insufficient_balance"
	cashInKindChargebackConflict   = "chargeback_conflict"
	cashInKindCommissionNoBalance  = "commission_reversal_insufficient"
	cashInKindMissingExternalID    = "missing_external_id"
	cashInKindOrphanDeposit        = "orphan_deposit"
	cashInKindQRAbsent             = "qr_missing"
	cashInKindAmbiguousResult      = "ambiguous_result"
	cashInKindSubmissionIssue      = "submission_issue"
	cashInKindUnconfirmedIntent    = "intent_sent_unconfirmed"
)

// intentPendencyKinds are reconciled automatically when a callback settles an
// intent. Financial conflicts are intentionally excluded: they require
// reapplication or proven reconciliation.
var intentPendencyKinds = "('ambiguous_result','submission_issue','intent_sent_unconfirmed','qr_missing')"

// nonFinancialResolvableKinds are the only queue kinds an operator may close
// manually without touching money. It is deliberately limited to a proven-fixed
// qr_missing: an ambiguous or unconfirmed charge must never be discarded by
// assumption, only by a reconciled callback or official provider proof. Any
// other kind (ambiguous_result, intent_sent_unconfirmed, financial conflicts,
// chargeback/commission discrepancies) can only be auto-resolved by a
// reconciled callback.
var nonFinancialResolvableKinds = map[string]bool{
	cashInKindQRAbsent: true,
}

// CashInReconciliationItem is one row of the cash-in reconciliation worklist.
type CashInReconciliationItem struct {
	QueueID       int64
	Kind          string
	RequestNumber string
	ExternalID    string
	Amount        float64
	Reason        string
	Since         time.Time
}

// ApplyDepositEvent applies an authenticated PIX deposit callback exactly once
// and returns applied, duplicate or queued. Everything (durable event registry,
// balance/journal/commission effects and queue entries) happens in one
// PostgreSQL transaction, so an event is never silently lost and a duplicate
// never creates a second effect. Only PAID_OUT credits and CHARGEBACK reverses;
// every other authenticated status is registered and queued without any money
// movement. Lock order is always gateway event -> deposit row -> user row ->
// referral -> sponsor row, matching ConfirmDeposit.
func (r *TransactionRepository) ApplyDepositEvent(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
	requestNumber = strings.TrimSpace(requestNumber)
	externalID = strings.TrimSpace(externalID)
	status = strings.ToUpper(strings.TrimSpace(status))
	// PAID is deliberately NOT treated as PAID_OUT: the provider's "paid"
	// notification does not prove settlement (money out), so it is registered
	// and queued as a non-final status and never credits on its own.
	if externalID == "" && requestNumber == "" {
		// Nothing reliable to persist a trail with.
		return nil, "", domain.ErrInvalidTransactionStatus
	}
	if status == "" {
		status = "UNKNOWN"
	}
	moneyStatus := status == cashInStatusPaidOut || status == cashInStatusChargeback

	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer dbtx.Rollback(ctx)

	lockKey := externalID
	if lockKey == "" {
		lockKey = requestNumber
	}
	if _, err = dbtx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 8127731))`, lockKey); err != nil {
		return nil, "", err
	}

	baseKey := "cashin-event:" + lockKey + ":" + status

	// A money-moving status without the provider's external id cannot be bound
	// to a gateway event nor deduplicated, so it must never move money. The
	// authenticated callback is still durably queued for reconciliation.
	if moneyStatus && externalID == "" {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindMissingExternalID, requestNumber, "", status, amount,
			fmt.Sprintf("status %s sem external_id do provedor; nenhum efeito financeiro", status))
	}

	// A money-moving status without a valid amount cannot be applied, but the
	// authenticated callback still leaves a durable trail when identifiable.
	if moneyStatus && !validMoney(amount) {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindAmountInvalid, requestNumber, externalID, status, 0,
			fmt.Sprintf("valor inválido para status %s: %.4f", status, amount))
	}
	var storedAmountArg any = amount
	if moneyStatus {
		cents, _ := money.Positive(amount)
		storedAmountArg = cents.String()
	}

	var storedAmount float64
	var storedRequest *string
	if externalID != "" {
		var inserted bool
		err = dbtx.QueryRow(ctx, `
			INSERT INTO deposit_gateway_events (external_id, status, amount, request_number)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (external_id, status) DO UPDATE SET received_at = deposit_gateway_events.received_at
			RETURNING amount, request_number, (xmax = 0) AS inserted`,
			externalID, status, storedAmountArg, nullableString(requestNumber)).Scan(&storedAmount, &storedRequest, &inserted)
		if err != nil {
			return nil, "", err
		}
		if !inserted {
			if moneyStatus && storedAmount > 0 && !money.EqualFloat64(storedAmount, amount) {
				reason := fmt.Sprintf("valor conflitante no evento %s/%s: registrado %.2f, recebido %.2f", externalID, status, storedAmount, amount)
				return queuedCashIn(ctx, dbtx, baseKey, cashInKindAmountConflict, derefString(storedRequest, requestNumber), externalID, status, amount, reason)
			}
			// The same gateway event must never be silently rebound to another
			// local intent: a differing request_number is a durable conflict.
			if requestNumber != "" && storedRequest != nil && *storedRequest != "" && *storedRequest != requestNumber {
				reason := fmt.Sprintf("evento %s/%s já vinculado à intenção %s; recebido request %s", externalID, status, *storedRequest, requestNumber)
				return queuedCashIn(ctx, dbtx, baseKey, cashInKindRequestConflict, requestNumber, externalID, status, amount, reason)
			}
		}
	}

	// Resolve the target intent. A callback without requestNumber but with a
	// valid externalID is matched by the external id; if unmatched it is queued,
	// never discarded.
	target := requestNumber
	if target == "" && storedRequest != nil {
		target = *storedRequest
	}
	if target == "" && externalID != "" {
		var found string
		err = dbtx.QueryRow(ctx, `SELECT id FROM transactions WHERE tipo='Depósito PIX' AND gateway_provider=$1 AND external_id=$2`, cashInProvider, externalID).Scan(&found)
		if err == nil {
			target = found
		} else if !errors.Is(err, domain.ErrTransactionNotFound) && !errors.Is(err, pgx.ErrNoRows) {
			return nil, "", err
		}
	}
	if externalID != "" && (storedRequest == nil || *storedRequest == "") && target != "" {
		if _, err = dbtx.Exec(ctx, `UPDATE deposit_gateway_events SET request_number=$3 WHERE external_id=$1 AND status=$2 AND request_number IS NULL`, externalID, status, target); err != nil {
			return nil, "", err
		}
	}
	if target == "" {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindPremature, requestNumber, externalID, status, amount, "callback autenticado sem intenção local correspondente")
	}

	deposit, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields), target))
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			return queuedCashIn(ctx, dbtx, baseKey, cashInKindPremature, target, externalID, status, amount, "callback autenticado sem intenção local correspondente")
		}
		return nil, "", err
	}
	if deposit.Tipo != "Depósito PIX" || deposit.GatewayProvider != cashInProvider {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindInvalidTarget, target, externalID, status, amount, "evento não corresponde a uma intenção de depósito PIX SuitPay")
	}
	if externalID != "" && deposit.ExternalID != nil && *deposit.ExternalID != externalID {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindExternalConflict, target, externalID, status, amount, fmt.Sprintf("external_id divergente: intenção %s, evento %s", *deposit.ExternalID, externalID))
	}
	if moneyStatus && !money.EqualFloat64(deposit.Valor, amount) {
		kind := cashInKindAmountConflict
		if status == cashInStatusChargeback {
			kind = cashInKindChargebackConflict
		}
		return queuedCashIn(ctx, dbtx, baseKey, kind, target, externalID, status, amount, fmt.Sprintf("valor divergente: intenção %.2f, evento %.2f", deposit.Valor, amount))
	}
	if externalID != "" && deposit.ExternalID == nil {
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET external_id=$2, updated_at=now() WHERE id=$1`, deposit.ID, externalID); err != nil {
			return nil, "", err
		}
		external := externalID
		deposit.ExternalID = &external
	}

	// Authenticated but non-final statuses never credit. They are durably queued
	// so operators can reconcile them; a later PAID_OUT can still be applied.
	if !moneyStatus {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindNonFinalStatus, target, externalID, status, amount, fmt.Sprintf("status autenticado não final (%s); nenhum efeito financeiro", status))
	}

	if status == cashInStatusPaidOut {
		if deposit.Status == domain.TxStatusSucesso {
			return r.finishCashIn(ctx, dbtx, deposit, baseKey, CashInDuplicate)
		}
		if deposit.Status == domain.TxStatusCancelado {
			// This is safe ONLY if the same externalID had both PAID_OUT and CHARGEBACK actually applied
			// (ledger deposit and deposit_chargeback). A local cancellation for another reason followed by a real
			// PAID_OUT must be durably queued as invalid_state/financial mismatch, NEVER silently acknowledged as duplicate.
			var depositLedgerCount, cbLedgerCount int
			err := dbtx.QueryRow(ctx, `
				SELECT
					COUNT(*) FILTER (WHERE kind = 'deposit'),
					COUNT(*) FILTER (WHERE kind = 'deposit_chargeback')
				FROM financial_events
				WHERE transaction_id = $1`, deposit.ID).Scan(&depositLedgerCount, &cbLedgerCount)
			if err != nil {
				return nil, "", err
			}

			var hasGwCycle bool
			if externalID != "" {
				var countGw int
				err := dbtx.QueryRow(ctx, `
					SELECT COUNT(DISTINCT status)
					FROM deposit_gateway_events
					WHERE external_id = $1 AND status IN ($2, $3)`,
					externalID, cashInStatusPaidOut, cashInStatusChargeback).Scan(&countGw)
				if err != nil {
					return nil, "", err
				}
				hasGwCycle = countGw >= 2
			}

			if depositLedgerCount > 0 && cbLedgerCount > 0 && hasGwCycle {
				return r.finishCashIn(ctx, dbtx, deposit, baseKey, CashInDuplicate)
			}

			return queuedCashIn(ctx, dbtx, baseKey, cashInKindInvalidState, target, externalID, status, amount,
				fmt.Sprintf("PAID_OUT recebido para transação cancelada sem ciclo prévio de crédito e estorno (depositLedger=%d, cbLedger=%d, gwCycle=%v)",
					depositLedgerCount, cbLedgerCount, hasGwCycle))
		}
		if deposit.Status != domain.TxStatusPendente && deposit.Status != domain.TxStatusProcessando {
			return queuedCashIn(ctx, dbtx, baseKey, cashInKindInvalidState, target, externalID, status, amount, fmt.Sprintf("PAID_OUT em estado incompatível (%s)", deposit.Status))
		}

		// Detect matching pending early chargeback for this intent.
		// Must match only chargeback_before_credit for the SAME externalID (and request if present),
		// excluding chargeback_conflict from automatic reversal.
		var pendingCBID int64
		var pendingCBAmount float64
		var hasPendingCB bool

		if externalID != "" {
			err := dbtx.QueryRow(ctx, `
				SELECT id, COALESCE(amount, 0)
				FROM cashin_event_queue
				WHERE queue_status='pending'
				  AND kind = 'chargeback_before_credit'
				  AND external_id = $1
				  AND (request_number = $2 OR request_number = '' OR request_number IS NULL)
				ORDER BY id ASC LIMIT 1 FOR UPDATE`,
				externalID, deposit.ID).Scan(&pendingCBID, &pendingCBAmount)
			if err == nil {
				hasPendingCB = true
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, "", err
			}

			if !hasPendingCB {
				var cbAmount float64
				var reqNum string
				err := dbtx.QueryRow(ctx, `
					SELECT amount, COALESCE(request_number, '')
					FROM deposit_gateway_events
					WHERE external_id=$1 AND status=$2`,
					externalID, cashInStatusChargeback).Scan(&cbAmount, &reqNum)
				if err == nil && (reqNum == "" || reqNum == deposit.ID) {
					hasPendingCB = true
					pendingCBAmount = cbAmount
				} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return nil, "", err
				}
			}
		}

		if hasPendingCB {
			// Early CHARGEBACK arrived before PAID_OUT for this intent.
			// Keep user's withdrawals locked; never expose credited balance for withdrawal without review.
			if _, err := dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, deposit.UserID); err != nil {
				return nil, "", err
			}

			if pendingCBAmount > 0 && !money.EqualFloat64(pendingCBAmount, amount) {
				return queuedCashIn(ctx, dbtx, baseKey, cashInKindChargebackConflict, target, externalID, status, amount,
					fmt.Sprintf("conflito de valor entre PAID_OUT (%.2f) e CHARGEBACK antecipado (%.2f); saques bloqueados", amount, pendingCBAmount))
			}

			// 1. Credit via applyPaidOutCredit
			deposit, err = applyPaidOutCredit(ctx, dbtx, deposit, amount)
			if err != nil {
				return nil, "", err
			}

			// 2. Immediately reverse via applyChargeback inside the same transaction
			applied, reversed, _, err := applyChargeback(ctx, dbtx, deposit, amount)
			if err != nil {
				return nil, "", err
			}
			if !applied {
				return queuedCashIn(ctx, dbtx, baseKey, cashInKindChargebackNoBalance, target, externalID, status, amount,
					"falha ao estornar chargeback após crédito de PAID_OUT; saques bloqueados")
			}

			// Close pending chargeback queue item only after successful reversal for the exact matching externalID
			if err := resolveChargebackPendencies(ctx, dbtx, deposit.ID, externalID, "estornado automaticamente após liquidação de PAID_OUT"); err != nil {
				return nil, "", err
			}

			// User withdrawals remain locked (never automatically cleared)
			return r.finishCashIn(ctx, dbtx, reversed, baseKey, CashInApplied)
		}

		deposit, err = applyPaidOutCredit(ctx, dbtx, deposit, amount)
		if err != nil {
			return nil, "", err
		}
		return r.finishCashIn(ctx, dbtx, deposit, baseKey, CashInApplied)
	}

	// CHARGEBACK
	if deposit.Status == domain.TxStatusCancelado {
		// Treat duplicate only when the SAME externalID already has applied deposit + deposit_chargeback
		// journal cycle (beware current event is inserted before this check, so require ledger proof).
		// Otherwise queue invalid_state for human reconciliation with no money effect.
		var depositLedgerCount, cbLedgerCount int
		err := dbtx.QueryRow(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE kind = 'deposit'),
				COUNT(*) FILTER (WHERE kind = 'deposit_chargeback')
			FROM financial_events
			WHERE transaction_id = $1`, deposit.ID).Scan(&depositLedgerCount, &cbLedgerCount)
		if err != nil {
			return nil, "", err
		}

		sameExternal := externalID == "" || (deposit.ExternalID != nil && *deposit.ExternalID == externalID)
		if depositLedgerCount > 0 && cbLedgerCount > 0 && sameExternal {
			return r.finishCashIn(ctx, dbtx, deposit, baseKey, CashInDuplicate)
		}

		if _, err := dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, deposit.UserID); err != nil {
			return nil, "", err
		}
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindInvalidState, target, externalID, status, amount,
			fmt.Sprintf("CHARGEBACK em transação cancelada sem ciclo prévio de crédito e chargeback no razão (depositLedger=%d, cbLedger=%d, sameExternal=%v); saques bloqueados",
				depositLedgerCount, cbLedgerCount, sameExternal))
	}
	if deposit.Status != domain.TxStatusSucesso {
		if _, err := dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, deposit.UserID); err != nil {
			return nil, "", err
		}
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindChargebackBeforePaid, target, externalID, status, amount, fmt.Sprintf("CHARGEBACK antes do crédito (estado %s)", deposit.Status))
	}
	applied, reversed, available, err := applyChargeback(ctx, dbtx, deposit, amount)
	if err != nil {
		return nil, "", err
	}
	if !applied {
		return queuedCashIn(ctx, dbtx, baseKey, cashInKindChargebackNoBalance, target, externalID, status, amount, fmt.Sprintf("saldo insuficiente para estorno (saldo %.2f, chargeback %.2f); saques bloqueados", available, amount))
	}
	return r.finishCashIn(ctx, dbtx, reversed, baseKey, CashInApplied)
}

// finishCashIn resolves any pending queue entries for the event, clears the
// intent ambiguity, marks it reconciled and commits, so ListCashInReconciliation
// no longer reports the settled intent.
func (r *TransactionRepository) finishCashIn(ctx context.Context, dbtx pgx.Tx, deposit *domain.Transaction, baseKey, outcome string) (*domain.Transaction, string, error) {
	if _, err := dbtx.Exec(ctx, `UPDATE cashin_event_queue SET queue_status='resolved', resolved_at=now(), resolved_by='system', evidence_ref=$2, resolution_note='reconciliado por callback'
		WHERE queue_status='pending' AND (dedup_key=$1 OR dedup_key LIKE $1 || ':%')`, baseKey, "auto:"+baseKey); err != nil {
		return nil, "", err
	}
	if err := resolveIntentPendencies(ctx, dbtx, deposit.ID, "reconciliado por callback"); err != nil {
		return nil, "", err
	}
	if deposit.Status == domain.TxStatusCancelado {
		ext := ""
		if deposit.ExternalID != nil {
			ext = *deposit.ExternalID
		}
		if err := resolveChargebackPendencies(ctx, dbtx, deposit.ID, ext, "reconciliado por estorno de chargeback"); err != nil {
			return nil, "", err
		}
	}
	if _, err := dbtx.Exec(ctx, `UPDATE cashin_deposit_intents
		SET result_ambiguous=FALSE, ambiguity_reason=NULL, reconciled_at=now(), updated_at=now(), external_id=COALESCE(external_id,$2)
		WHERE request_number=$1`, deposit.ID, derefString(deposit.ExternalID, "")); err != nil {
		return nil, "", err
	}
	if err := dbtx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return deposit, outcome, nil
}

// resolveIntentPendencies auto-closes only intent-level items of one request.
// Financial conflicts are left untouched for reapplication/reconciliation.
func resolveIntentPendencies(ctx context.Context, dbtx pgx.Tx, requestNumber, note string) error {
	_, err := dbtx.Exec(ctx, `UPDATE cashin_event_queue SET queue_status='resolved', resolved_at=now(), resolved_by='system', evidence_ref=$2, resolution_note=$3
		WHERE queue_status='pending' AND request_number=$1 AND kind IN `+intentPendencyKinds,
		requestNumber, "auto:"+requestNumber, note)
	return err
}

func resolveChargebackPendencies(ctx context.Context, dbtx pgx.Tx, requestNumber, externalID, note string) error {
	if externalID == "" {
		return nil
	}
	_, err := dbtx.Exec(ctx, `UPDATE cashin_event_queue SET queue_status='resolved', resolved_at=now(), resolved_by='system', evidence_ref=$3, resolution_note=$4
		WHERE queue_status='pending' AND external_id=$1 AND (request_number=$2 OR request_number='' OR request_number IS NULL) AND kind='chargeback_before_credit'`,
		externalID, requestNumber, "auto:"+externalID, note)
	return err
}

// queuedCashIn persists a pending, operator-visible queue entry and commits the
// durable event without any balance effect.
func queuedCashIn(ctx context.Context, dbtx pgx.Tx, baseKey, kind, requestNumber, externalID, status string, amount float64, reason string) (*domain.Transaction, string, error) {
	if err := enqueueCashInEvent(ctx, dbtx, baseKey, kind, requestNumber, externalID, status, amount, reason); err != nil {
		return nil, "", err
	}
	if err := dbtx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return nil, CashInQueued, nil
}

// enqueueCashInEvent writes a deduplicated pending item. A repeated occurrence
// bumps the attempt counter instead of creating a new row.
func enqueueCashInEvent(ctx context.Context, dbtx pgx.Tx, baseKey, kind, requestNumber, externalID, status string, amount float64, reason string) error {
	dedupKey := baseKey + ":" + kind
	var amountArg any
	if amount > 0 {
		if cents, err := money.Positive(amount); err == nil {
			amountArg = cents.String()
		}
	}
	_, err := dbtx.Exec(ctx, `
		INSERT INTO cashin_event_queue (dedup_key, kind, request_number, external_id, gateway_status, amount, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (dedup_key) DO UPDATE SET
			attempts = cashin_event_queue.attempts + 1,
			reason = EXCLUDED.reason,
			queue_status = 'pending',
			resolved_at = NULL,
			resolved_by = NULL,
			evidence_ref = NULL,
			resolution_note = NULL`,
		dedupKey, kind, nullableString(requestNumber), nullableString(externalID), nullableString(status), amountArg, reason)
	return err
}

// applyPaidOutCredit credits a pending deposit atomically, mirrors the journal
// and referral commission of ConfirmDeposit and returns the updated row. It
// deliberately cannot call ConfirmDeposit because this call already holds the
// deposit row lock; reimplementing it here keeps the event registry, credit and
// commission in one ACID transaction without editing financial_repository.go.
func applyPaidOutCredit(ctx context.Context, dbtx pgx.Tx, deposit *domain.Transaction, amount float64) (*domain.Transaction, error) {
	amountCents, err := money.Positive(amount)
	if err != nil {
		return nil, err
	}
	var before float64
	if err := dbtx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, deposit.UserID).Scan(&before); err != nil {
		return nil, err
	}
	var after float64
	if err := dbtx.QueryRow(ctx, `UPDATE users SET balance=balance+$1, required_rollover=required_rollover+$1, updated_at=now() WHERE id=$2 RETURNING balance`, amountCents.String(), deposit.UserID).Scan(&after); err != nil {
		return nil, err
	}
	if err := journal(ctx, dbtx, deposit.ID, deposit.UserID, "deposit", amount, before, after); err != nil {
		return nil, err
	}
	now := time.Now()
	updated, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET status='sucesso', valor_efetivo_pago=$2, paid_at=$3, reconciled_at=$3, updated_at=$3 WHERE id=$1 RETURNING %s`, transactionSelectFields), deposit.ID, amountCents.String(), now))
	if err != nil {
		return nil, err
	}
	if err := payReferralCommission(ctx, dbtx, updated, amount); err != nil {
		return nil, err
	}
	return updated, nil
}

// payReferralCommission mirrors the first-deposit referral policy of
// ConfirmDeposit: the referral is activated at most once under its own lock and
// a failure rolls the whole deposit back.
func payReferralCommission(ctx context.Context, dbtx pgx.Tx, deposit *domain.Transaction, amount float64) error {
	amountCents, amountErr := money.Positive(amount)
	if amountErr != nil {
		return amountErr
	}
	var referralID, sponsorID string
	refErr := dbtx.QueryRow(ctx, `SELECT r.id,r.referrer_id FROM referrals r JOIN users u ON u.id=$1 WHERE (r.referred_user_id=$1 OR (r.referred_user_id IS NULL AND r.nome=u.username)) AND r.status<>'ativo' ORDER BY r.id LIMIT 1 FOR UPDATE OF r`, deposit.UserID).Scan(&referralID, &sponsorID)
	if refErr != nil {
		if errors.Is(refErr, pgx.ErrNoRows) {
			return nil
		}
		return refErr
	}
	if _, err := dbtx.Exec(ctx, `UPDATE referrals SET status='ativo', deposito=$2, updated_at=now() WHERE id=$1`, referralID, amountCents.String()); err != nil {
		return err
	}
	var sponsorBefore, percent float64
	var suspended, multiAccount, level12Suspended bool
	var level int
	if err := dbtx.QueryRow(ctx, `SELECT balance, custom_referral_commission_percent, referral_bonus_suspended, multi_account_flag, nivel, level12_bonus_suspended FROM users WHERE id=$1 FOR UPDATE`, sponsorID).Scan(&sponsorBefore, &percent, &suspended, &multiAccount, &level, &level12Suspended); err != nil {
		return err
	}
	if suspended || multiAccount {
		return nil
	}
	if percent <= 0 || (level >= 12 && level12Suspended) {
		percent = 5
	}
	basisPoints, percentErr := money.BasisPoints(percent)
	if percentErr != nil {
		return percentErr
	}
	commissionCents, percentErr := amountCents.Percent(basisPoints)
	if percentErr != nil {
		return percentErr
	}
	if commissionCents == 0 {
		return nil
	}
	commission := commissionCents.Float64()
	if !validMoney(commission) {
		return fmt.Errorf("comissão inválida")
	}
	commissionID, err := financialID("COM")
	if err != nil {
		return err
	}
	var sponsorAfter float64
	if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance+$1, updated_at=now() WHERE id=$2 RETURNING balance`, commissionCents.String(), sponsorID).Scan(&sponsorAfter); err != nil {
		return err
	}
	if _, err = dbtx.Exec(ctx, `INSERT INTO transactions (id,user_id,tipo,valor,status,gateway_provider,reference_id,paid_at) VALUES ($1,$2,'Comissão de primeiro depósito',$3,'sucesso',$4,$5,now())`, commissionID, sponsorID, commissionCents.String(), cashInProvider, deposit.ID); err != nil {
		return err
	}
	return journal(ctx, dbtx, commissionID, sponsorID, "referral_commission", commission, sponsorBefore, sponsorAfter)
}

// applyChargeback reverses a credited deposit: balance, rollover and the
// first-deposit commission. It never lets a balance go negative; when the
// customer cannot cover the reversal it reports applied=false together with the
// available balance, and the caller queues the event and blocks the account.
func applyChargeback(ctx context.Context, dbtx pgx.Tx, deposit *domain.Transaction, amount float64) (applied bool, result *domain.Transaction, available float64, err error) {
	amountCents, amountErr := money.Positive(amount)
	if amountErr != nil {
		return false, nil, 0, amountErr
	}
	var before float64
	if err = dbtx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, deposit.UserID).Scan(&before); err != nil {
		return false, nil, 0, err
	}
	beforeCents, moneyErr := money.FromFloat64(before)
	if moneyErr != nil {
		return false, nil, 0, moneyErr
	}
	if beforeCents < amountCents {
		if _, err = dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, deposit.UserID); err != nil {
			return false, nil, 0, err
		}
		return false, deposit, before, nil
	}
	var after float64
	if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance-$1, required_rollover=GREATEST(required_rollover-$1,0), updated_at=now() WHERE id=$2 RETURNING balance`, amountCents.String(), deposit.UserID).Scan(&after); err != nil {
		return false, nil, 0, err
	}
	if err = journal(ctx, dbtx, deposit.ID, deposit.UserID, "deposit_chargeback", -amount, before, after); err != nil {
		return false, nil, 0, err
	}
	if err = reverseReferralCommission(ctx, dbtx, deposit); err != nil {
		return false, nil, 0, err
	}
	now := time.Now()
	updated, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET status='cancelado', rejection_reason='Chargeback SuitPay', reconciled_at=$2, updated_at=$2 WHERE id=$1 RETURNING %s`, transactionSelectFields), deposit.ID, now))
	if err != nil {
		return false, nil, 0, err
	}
	return true, updated, after, nil
}

// reverseReferralCommission refunds the first-deposit commission. When the
// sponsor lacks balance the commission stays pending in the queue and the
// sponsor is blocked instead of forcing a negative balance or hiding it.
func reverseReferralCommission(ctx context.Context, dbtx pgx.Tx, deposit *domain.Transaction) error {
	var commissionID, sponsorID string
	var commissionAmount float64
	err := dbtx.QueryRow(ctx, `SELECT id, user_id, valor FROM transactions WHERE reference_id=$1 AND tipo='Comissão de primeiro depósito' AND status='sucesso' ORDER BY created_at LIMIT 1 FOR UPDATE`, deposit.ID).Scan(&commissionID, &sponsorID, &commissionAmount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	var sponsorBefore float64
	if err = dbtx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, sponsorID).Scan(&sponsorBefore); err != nil {
		return err
	}
	sponsorCents, moneyErr := money.FromFloat64(sponsorBefore)
	if moneyErr != nil {
		return moneyErr
	}
	commissionCents, moneyErr := money.Positive(commissionAmount)
	if moneyErr != nil {
		return moneyErr
	}
	if sponsorCents < commissionCents {
		if _, err = dbtx.Exec(ctx, `UPDATE users SET withdraw_locked=TRUE, updated_at=now() WHERE id=$1`, sponsorID); err != nil {
			return err
		}
		return enqueueCashInEvent(ctx, dbtx, "cashin-commission:"+deposit.ID, cashInKindCommissionNoBalance, deposit.ID, derefString(deposit.ExternalID, ""), "", commissionAmount, "saldo insuficiente do indicador para estornar comissão; saque bloqueado")
	}
	var sponsorAfter float64
	if err = dbtx.QueryRow(ctx, `UPDATE users SET balance=balance-$1, updated_at=now() WHERE id=$2 RETURNING balance`, commissionCents.String(), sponsorID).Scan(&sponsorAfter); err != nil {
		return err
	}
	if err = journal(ctx, dbtx, commissionID, sponsorID, "referral_commission_reversal", -commissionAmount, sponsorBefore, sponsorAfter); err != nil {
		return err
	}
	_, err = dbtx.Exec(ctx, `UPDATE transactions SET status='cancelado', rejection_reason='Estornada por chargeback', updated_at=now() WHERE id=$1`, commissionID)
	return err
}

// MarkDepositSubmissionStarted durably records that a deposit intent entered
// submission, before the external gateway call and without an external id yet,
// so a crash or timeout can be detected later by reconciliation.
func (r *TransactionRepository) MarkDepositSubmissionStarted(ctx context.Context, txID string) error {
	txID = strings.TrimSpace(txID)
	if txID == "" {
		return domain.ErrInvalidTransactionStatus
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbtx.Rollback(ctx)
	deposit, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields), txID))
	if err != nil {
		return err
	}
	if deposit.Tipo != "Depósito PIX" {
		return domain.ErrInvalidTransactionStatus
	}
	// Never clears reconciliation/ambiguity: a late start message must not
	// reopen a settled intent.
	if _, err = dbtx.Exec(ctx, `
		INSERT INTO cashin_deposit_intents (request_number, provider, intent_sent_at)
		VALUES ($1, $2, now())
		ON CONFLICT (request_number) DO UPDATE SET
			intent_sent_at = COALESCE(cashin_deposit_intents.intent_sent_at, now()),
			updated_at = now()`,
		txID, cashInProvider); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}

// MarkDepositSubmissionIssue transactionally records an ambiguous submission
// (timeout, incomplete response, error). It is safe against an early callback:
// when the intent is already settled (sucesso/cancelado) it never reopens a
// pending item, it only preserves identifiers.
func (r *TransactionRepository) MarkDepositSubmissionIssue(ctx context.Context, txID, reason, externalID string) error {
	txID = strings.TrimSpace(txID)
	reason = strings.TrimSpace(reason)
	externalID = strings.TrimSpace(externalID)
	if txID == "" {
		return domain.ErrInvalidTransactionStatus
	}
	if reason == "" {
		reason = "falha de envio ao provedor"
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbtx.Rollback(ctx)
	deposit, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields), txID))
	if err != nil {
		return err
	}
	settled := deposit.Status == domain.TxStatusSucesso || deposit.Status == domain.TxStatusCancelado
	creditedExternal := derefString(deposit.ExternalID, "")
	// A conflicting external id on a late submission failure must never be
	// swallowed: record the discrepancy durably before any other handling.
	if externalID != "" && creditedExternal != "" && creditedExternal != externalID {
		reasonConflict := fmt.Sprintf("ocorrência de envio com external_id %s divergente do vinculado %s; identidade preservada", externalID, creditedExternal)
		if err = enqueueCashInEvent(ctx, dbtx, "cashin-intent:"+txID, cashInKindExternalConflict, txID, creditedExternal, "", 0, reasonConflict); err != nil {
			return err
		}
	}
	if !settled && externalID != "" && deposit.ExternalID == nil {
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET external_id=$2, updated_at=now() WHERE id=$1`, txID, externalID); err != nil {
			return err
		}
	}
	if settled {
		// Early callback already won: clear any stale intent pendency and keep
		// the record as reconciled, but never enqueue a new pending item.
		if err = resolveIntentPendencies(ctx, dbtx, txID, "resolvido por callback antecipado"); err != nil {
			return err
		}
		if err = upsertReconciledIntent(ctx, dbtx, txID, creditedExternal); err != nil {
			return err
		}
		return dbtx.Commit(ctx)
	}
	if deposit.Status == domain.TxStatusPendente {
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET status='processando', rejection_reason=$2, updated_at=now() WHERE id=$1`, txID, reason); err != nil {
			return err
		}
	}
	if _, err = dbtx.Exec(ctx, `
		INSERT INTO cashin_deposit_intents (request_number, provider, external_id, intent_sent_at, result_ambiguous, ambiguity_reason)
		VALUES ($1, $2, $3, now(), TRUE, $4)
		ON CONFLICT (request_number) DO UPDATE SET
			external_id = COALESCE(cashin_deposit_intents.external_id, EXCLUDED.external_id),
			intent_sent_at = COALESCE(cashin_deposit_intents.intent_sent_at, now()),
			result_ambiguous = TRUE,
			ambiguity_reason = EXCLUDED.ambiguity_reason,
			updated_at = now()`,
		txID, cashInProvider, nullableString(externalID), reason); err != nil {
		return err
	}
	if err = enqueueCashInEvent(ctx, dbtx, "cashin-intent:"+txID, cashInKindSubmissionIssue, txID, externalID, "", 0, reason); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}

// MarkDepositIntentSent durably records that a deposit intent was sent to the
// provider and reserves its external id, even when the local QR association is
// still pending. It is safe against an out-of-order callback: if the charge was
// already settled (sucesso/cancelado) before the HTTP response arrived, the
// intent is recorded as reconciled, the external id already used to credit is
// preserved and no qr_missing/ambiguous/unconfirmed pendency is reopened. A late
// response carrying a different external id is a durable conflict: it is queued
// and surfaced as an error, never silently overwritten.
func (r *TransactionRepository) MarkDepositIntentSent(ctx context.Context, requestNumber, externalID string) (*domain.Transaction, error) {
	requestNumber = strings.TrimSpace(requestNumber)
	externalID = strings.TrimSpace(externalID)
	if requestNumber == "" || externalID == "" {
		return nil, domain.ErrInvalidTransactionStatus
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	deposit, err := scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM transactions WHERE id=$1 FOR UPDATE", transactionSelectFields), requestNumber))
	if err != nil {
		return nil, err
	}
	if deposit.Tipo != "Depósito PIX" || deposit.GatewayProvider != cashInProvider {
		return nil, domain.ErrInvalidTransactionStatus
	}
	settled := deposit.Status == domain.TxStatusSucesso || deposit.Status == domain.TxStatusCancelado
	creditedExternal := derefString(deposit.ExternalID, "")
	conflict := externalID != "" && creditedExternal != "" && creditedExternal != externalID

	if conflict {
		// Never substitute or hide the credited external id: record the
		// discrepancy durably and keep the winning identity.
		reason := fmt.Sprintf("resposta tardia com external_id %s divergente do creditado %s; identidade preservada", externalID, creditedExternal)
		if err = enqueueCashInEvent(ctx, dbtx, "cashin-intent:"+requestNumber, cashInKindExternalConflict, requestNumber, creditedExternal, "", 0, reason); err != nil {
			return nil, err
		}
		if settled {
			if err = resolveIntentPendencies(ctx, dbtx, requestNumber, "reconciliado por callback"); err != nil {
				return nil, err
			}
			if err = upsertReconciledIntent(ctx, dbtx, requestNumber, creditedExternal); err != nil {
				return nil, err
			}
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return deposit, fmt.Errorf("external_id divergente para a intenção %s: creditado %s, recebido %s", requestNumber, creditedExternal, externalID)
	}

	if settled {
		// Early callback already won. Mark the intent reconciled, preserve the
		// credited external id and never reopen pendencies.
		if err = resolveIntentPendencies(ctx, dbtx, requestNumber, "reconciliado por callback"); err != nil {
			return nil, err
		}
		if err = upsertReconciledIntent(ctx, dbtx, requestNumber, creditedExternal); err != nil {
			return nil, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, err
		}
		return deposit, nil
	}

	if deposit.ExternalID == nil {
		if _, err = dbtx.Exec(ctx, `UPDATE transactions SET external_id=$2, updated_at=now() WHERE id=$1`, requestNumber, externalID); err != nil {
			return nil, err
		}
	}
	if _, err = dbtx.Exec(ctx, `
		INSERT INTO cashin_deposit_intents (request_number, provider, external_id, intent_sent_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (request_number) DO UPDATE SET external_id=EXCLUDED.external_id, intent_sent_at=now(), updated_at=now()`,
		requestNumber, cashInProvider, externalID); err != nil {
		return nil, err
	}
	deposit.ExternalID = &externalID
	if err = dbtx.Commit(ctx); err != nil {
		return nil, err
	}
	return deposit, nil
}

// upsertReconciledIntent records an intent as settled without reopening any
// pendency. The external id already stored is preserved (COALESCE), so a later
// message can never overwrite the identity that was actually credited.
func upsertReconciledIntent(ctx context.Context, dbtx pgx.Tx, requestNumber, externalID string) error {
	_, err := dbtx.Exec(ctx, `
		INSERT INTO cashin_deposit_intents (request_number, provider, external_id, intent_sent_at, reconciled_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (request_number) DO UPDATE SET
			external_id = COALESCE(cashin_deposit_intents.external_id, EXCLUDED.external_id),
			intent_sent_at = COALESCE(cashin_deposit_intents.intent_sent_at, now()),
			result_ambiguous = FALSE,
			ambiguity_reason = NULL,
			reconciled_at = COALESCE(cashin_deposit_intents.reconciled_at, now()),
			updated_at = now()`,
		requestNumber, cashInProvider, nullableString(externalID))
	return err
}

// MarkDepositResultAmbiguous records an ambiguous provider response (timeout or
// incomplete body) so the intent is never lost and stays reconciliable. Local
// expiration never proves provider cancellation, so nothing is cancelled here.
// A settled intent is never reopened.
func (r *TransactionRepository) MarkDepositResultAmbiguous(ctx context.Context, requestNumber, reason string) error {
	requestNumber = strings.TrimSpace(requestNumber)
	if requestNumber == "" {
		return domain.ErrInvalidTransactionStatus
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "resultado ambíguo do provedor"
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbtx.Rollback(ctx)
	var provider string
	var status string
	if err = dbtx.QueryRow(ctx, `SELECT COALESCE(gateway_provider,''), status FROM transactions WHERE id=$1 FOR UPDATE`, requestNumber).Scan(&provider, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTransactionNotFound
		}
		return err
	}
	if status == string(domain.TxStatusSucesso) || status == string(domain.TxStatusCancelado) {
		if err = resolveIntentPendencies(ctx, dbtx, requestNumber, "resolvido por callback"); err != nil {
			return err
		}
		return dbtx.Commit(ctx)
	}
	if provider == "" {
		provider = cashInProvider
	}
	if _, err = dbtx.Exec(ctx, `
		INSERT INTO cashin_deposit_intents (request_number, provider, intent_sent_at, result_ambiguous, ambiguity_reason)
		VALUES ($1, $2, now(), TRUE, $3)
		ON CONFLICT (request_number) DO UPDATE SET result_ambiguous=TRUE, ambiguity_reason=EXCLUDED.ambiguity_reason, intent_sent_at=COALESCE(cashin_deposit_intents.intent_sent_at, now()), updated_at=now()`,
		requestNumber, provider, reason); err != nil {
		return err
	}
	if err = enqueueCashInEvent(ctx, dbtx, "cashin-intent:"+requestNumber, cashInKindAmbiguousResult, requestNumber, "", "", 0, reason); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}

// MarkDepositQRMissing records a missing PIX charge as an operator-visible
// queue item, or resolves it once a payment instruction exists. A valid
// copy-and-paste PIX code (pix_code) is enough: a missing base64 image alone
// does not mean the charge has no QR.
func (r *TransactionRepository) MarkDepositQRMissing(ctx context.Context, requestNumber string) (bool, error) {
	requestNumber = strings.TrimSpace(requestNumber)
	if requestNumber == "" {
		return false, domain.ErrInvalidTransactionStatus
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer dbtx.Rollback(ctx)
	var pixCode, qrURL *string
	if err = dbtx.QueryRow(ctx, `SELECT pix_code, qr_code_url FROM transactions WHERE id=$1 FOR UPDATE`, requestNumber).Scan(&pixCode, &qrURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domain.ErrTransactionNotFound
		}
		return false, err
	}
	hasInstruction := (pixCode != nil && *pixCode != "") || (qrURL != nil && *qrURL != "")
	baseKey := "cashin-intent:" + requestNumber
	if hasInstruction {
		if _, err = dbtx.Exec(ctx, `UPDATE cashin_deposit_intents SET qr_attached_at=now(), updated_at=now() WHERE request_number=$1`, requestNumber); err != nil {
			return false, err
		}
		if _, err = dbtx.Exec(ctx, `UPDATE cashin_event_queue SET queue_status='resolved', resolved_at=now(), resolved_by='system', evidence_ref=$2, resolution_note='instrução PIX presente'
			WHERE queue_status='pending' AND dedup_key=$1`, baseKey+":"+cashInKindQRAbsent, "auto:"+baseKey); err != nil {
			return false, err
		}
		return false, dbtx.Commit(ctx)
	}
	if err = enqueueCashInEvent(ctx, dbtx, baseKey, cashInKindQRAbsent, requestNumber, "", "", 0, "sem código PIX nem QR após intenção de depósito"); err != nil {
		return false, err
	}
	return true, dbtx.Commit(ctx)
}

// ListCashInReconciliation returns pending queue items plus intents with
// ambiguous results, missing payment instruction or an unconfirmed send that
// aged past the operator threshold. Reconciled intents are never reported. It
// is read-only and never mutates provider state.
func (r *TransactionRepository) ListCashInReconciliation(ctx context.Context) ([]CashInReconciliationItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, kind, COALESCE(request_number,''), COALESCE(external_id,''), COALESCE(amount,0), reason, created_at
		FROM cashin_event_queue
		WHERE queue_status='pending'
		UNION ALL
		SELECT 0, 'ambiguous_result', i.request_number, COALESCE(i.external_id,''), 0, COALESCE(i.ambiguity_reason,'resultado ambíguo'), i.updated_at
		FROM cashin_deposit_intents i
		WHERE i.result_ambiguous = TRUE AND i.reconciled_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM cashin_event_queue q WHERE q.request_number=i.request_number AND q.queue_status='pending')
		UNION ALL
		SELECT 0, 'qr_missing', i.request_number, COALESCE(i.external_id,''), 0, 'sem código PIX nem QR', i.created_at
		FROM cashin_deposit_intents i
		JOIN transactions t ON t.id = i.request_number
		WHERE i.qr_attached_at IS NULL AND i.reconciled_at IS NULL
		  AND COALESCE(t.pix_code,'')='' AND COALESCE(t.qr_code_url,'')=''
		  AND NOT EXISTS (SELECT 1 FROM cashin_event_queue q WHERE q.request_number=i.request_number AND q.queue_status='pending')
		UNION ALL
		SELECT 0, 'intent_sent_unconfirmed', i.request_number, COALESCE(i.external_id,''), 0, 'intenção enviada sem confirmação local', i.intent_sent_at
		FROM cashin_deposit_intents i
		JOIN transactions t ON t.id = i.request_number
		WHERE i.intent_sent_at IS NOT NULL AND i.reconciled_at IS NULL
		  AND t.status IN ('pendente','processando')
		  AND i.intent_sent_at < now()-interval '5 minutes'
		  AND NOT EXISTS (SELECT 1 FROM cashin_event_queue q WHERE q.request_number=i.request_number AND q.queue_status='pending')
		UNION ALL
		SELECT 0, 'orphan_deposit', t.id, COALESCE(t.external_id,''), t.valor, 'depósito PIX pendente sem instrução de pagamento há mais de 5 minutos (possível órfão de envio)', t.created_at
		FROM transactions t
		WHERE t.tipo='Depósito PIX' AND t.gateway_provider=$1
		  AND t.status='pendente'
		  AND COALESCE(t.pix_code,'')='' AND COALESCE(t.qr_code_url,'')=''
		  AND t.created_at < now()-interval '5 minutes'
		  AND NOT EXISTS (SELECT 1 FROM cashin_deposit_intents i WHERE i.request_number=t.id)
		  AND NOT EXISTS (SELECT 1 FROM cashin_event_queue q WHERE q.request_number=t.id AND q.queue_status='pending')
		ORDER BY 7 ASC
		LIMIT 200`, cashInProvider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CashInReconciliationItem, 0)
	for rows.Next() {
		var item CashInReconciliationItem
		if err = rows.Scan(&item.QueueID, &item.Kind, &item.RequestNumber, &item.ExternalID, &item.Amount, &item.Reason, &item.Since); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ResolveCashInQueueItem closes a queue item an operator proved fixed. Only
// qr_missing (a payment instruction that was genuinely attached) may be closed
// manually, and it still requires the operator id, an objective evidence
// reference and a reason. Every other kind — including ambiguous_result and
// intent_sent_unconfirmed — is refused: a real charge must never be discarded by
// assumption and is only settled by a reconciled callback or official proof.
func (r *TransactionRepository) ResolveCashInQueueItem(ctx context.Context, id int64, operatorID, evidence, reason string) error {
	operatorID = strings.TrimSpace(operatorID)
	evidence = strings.TrimSpace(evidence)
	reason = strings.TrimSpace(reason)
	if operatorID == "" || evidence == "" || reason == "" {
		return domain.ErrInvalidTransactionStatus
	}
	var kind string
	if err := r.pool.QueryRow(ctx, `SELECT kind FROM cashin_event_queue WHERE id=$1`, id).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTransactionNotFound
		}
		return err
	}
	if !nonFinancialResolvableKinds[kind] {
		return fmt.Errorf("item %s não pode ser encerrado manualmente: exige callback conciliado ou prova oficial do provedor", kind)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE cashin_event_queue
		SET queue_status='resolved', resolved_at=now(), resolved_by=$2, evidence_ref=$3, resolution_note=$4
		WHERE id=$1 AND queue_status='pending'`, id, operatorID, evidence, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTransactionNotFound
	}
	return nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func derefString(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
}
