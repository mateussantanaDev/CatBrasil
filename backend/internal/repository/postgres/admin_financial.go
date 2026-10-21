package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"github.com/jackc/pgx/v5"
)

// AdjustAdminBalanceAtomic records the operator, reason and balance movement
// with the wallet update. A repeated request key cannot apply it twice.
func (r *TransactionRepository) AdjustAdminBalanceAtomic(ctx context.Context, userID, actorID, reason, key string, delta float64) (*domain.User, error) {
	if key == "" || len(key) > 128 || actorID == "" || len(reason) > 1000 || delta == 0 || math.IsNaN(delta) || math.IsInf(delta, 0) || !validMoney(math.Abs(delta)) {
		return nil, fmt.Errorf("ajuste financeiro inválido")
	}
	deltaCents, err := money.FromFloat64(delta)
	if err != nil {
		return nil, err
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), userID))
	if err != nil {
		return nil, err
	}
	var prior float64
	var previousActor, previousReason string
	err = dbtx.QueryRow(ctx, `SELECT t.valor,COALESCE(t.reference_id,''),COALESCE(e.reason,'') FROM transactions t JOIN financial_events e ON e.transaction_id=t.id AND e.kind='admin_adjustment' WHERE t.user_id=$1 AND t.idempotency_key=$2 AND t.gateway_provider='internal_admin'`, userID, key).Scan(&prior, &previousActor, &previousReason)
	if err == nil {
		if !money.EqualFloat64(prior, delta) || previousActor != actorID || previousReason != reason {
			return nil, fmt.Errorf("chave de idempotência reutilizada com outro ajuste")
		}
		return user, dbtx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	balanceCents, err := money.FromFloat64(user.Balance)
	if err != nil {
		return nil, err
	}
	if balanceCents+deltaCents < 0 {
		return nil, domain.ErrInsufficientBalance
	}
	id, err := financialID("ADJ")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	updated, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET balance=balance+$1,updated_at=$2 WHERE id=$3 RETURNING %s`, userSelectFields), deltaCents.String(), now, userID))
	if err != nil {
		return nil, err
	}
	_, err = dbtx.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider,idempotency_key,reference_id,paid_at,created_at,updated_at) VALUES($1,$2,'Ajuste Manual',$3,'sucesso','internal_admin',$4,$5,$6,$6,$6)`, id, userID, deltaCents.String(), key, actorID, now)
	if err != nil {
		return nil, err
	}
	beforeCents, err := money.FromFloat64(user.Balance)
	if err != nil {
		return nil, err
	}
	afterCents, err := money.FromFloat64(updated.Balance)
	if err != nil {
		return nil, err
	}
	_, err = dbtx.Exec(ctx, `INSERT INTO financial_events(transaction_id,user_id,kind,amount,balance_before,balance_after,actor_id,reason) VALUES($1,$2,'admin_adjustment',$3,$4,$5,$6,$7)`, id, userID, deltaCents.String(), beforeCents.String(), afterCents.String(), actorID, reason)
	if err != nil {
		return nil, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}
