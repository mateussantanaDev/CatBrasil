package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"github.com/jackc/pgx/v5"
)

func financialID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(b[:]), nil
}

// RentMachineAtomic keeps price, stock, wallet, rollover, machine and receipt
// in one PostgreSQL transaction. The request key points to the created machine.
func (r *TransactionRepository) RentMachineAtomic(ctx context.Context, userID, catalogID, key string) (*domain.Machine, *domain.User, int, error) {
	if key == "" || len(key) > 128 {
		return nil, nil, 0, fmt.Errorf("Idempotency-Key obrigatório e limitado a 128 caracteres")
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	defer dbtx.Rollback(ctx)
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), userID))
	if err != nil {
		return nil, nil, 0, err
	}
	var reference, machineID string
	var priorAmount float64
	err = dbtx.QueryRow(ctx, `SELECT reference_id,external_id,valor FROM transactions WHERE user_id=$1 AND idempotency_key=$2 AND gateway_provider='internal_rent'`, userID, key).Scan(&reference, &machineID, &priorAmount)
	if err == nil {
		if reference != catalogID {
			return nil, nil, 0, fmt.Errorf("chave de idempotência reutilizada com outra frota")
		}
		machine, err := scanMachine(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM machines WHERE id=$1", machineSelectFields), machineID))
		if err != nil {
			return nil, nil, 0, err
		}
		return machine, user, int(-priorAmount * 0.02), dbtx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, 0, err
	}
	item, err := scanCatalogItem(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM fleet_catalog WHERE id=$1 FOR UPDATE", catalogSelectFields), catalogID))
	if err != nil {
		return nil, nil, 0, err
	}
	if !item.Active || item.StockQuantity <= 0 {
		return nil, nil, 0, domain.ErrCatalogItemInactive
	}
	if !validMoney(item.Price) {
		return nil, nil, 0, fmt.Errorf("preço de frota inválido")
	}
	priceCents, _ := money.Positive(item.Price)
	balanceCents, err := money.FromFloat64(user.Balance)
	if err != nil {
		return nil, nil, 0, err
	}
	if balanceCents < priceCents {
		return nil, nil, 0, domain.ErrInsufficientBalance
	}
	yieldCents, err := money.FromFloat64(user.YieldBalanceAvailable)
	if err != nil {
		return nil, nil, 0, err
	}
	if item.ReinvestOnly && yieldCents < priceCents {
		return nil, nil, 0, domain.ErrInsufficientYieldBalance
	}
	var existing int
	if err = dbtx.QueryRow(ctx, `SELECT count(*) FROM machines WHERE user_id=$1 AND name=$2`, userID, item.Name).Scan(&existing); err != nil {
		return nil, nil, 0, err
	}
	if item.MaxPurchasesPerUser > 0 && existing >= item.MaxPurchasesPerUser {
		return nil, nil, 0, domain.ErrMaxPurchasesExceeded
	}
	xp := int(item.Price * 0.02)
	yieldDebit := money.Cents(0)
	if item.ReinvestOnly {
		yieldDebit = priceCents
	}
	trator := catalogID == "trator_d8t" || strings.EqualFold(item.Category, "PESADA")
	updated, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET balance=balance-$1,current_rollover=current_rollover+$1,yield_balance_available=yield_balance_available-$2,xp=xp+$3,trator_rented=trator_rented OR $4,updated_at=now() WHERE id=$5 RETURNING %s`, userSelectFields), priceCents.String(), yieldDebit.String(), xp, trator, userID))
	if err != nil {
		return nil, nil, 0, err
	}
	if _, err = dbtx.Exec(ctx, `UPDATE fleet_catalog SET stock_quantity=stock_quantity-1,updated_at=now() WHERE id=$1`, catalogID); err != nil {
		return nil, nil, 0, err
	}
	machineID, err = financialID("MACH")
	if err != nil {
		return nil, nil, 0, err
	}
	txID, err := financialID("RENT")
	if err != nil {
		return nil, nil, 0, err
	}
	now := time.Now()
	machine, err := scanMachine(dbtx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO machines(id,user_id,name,cost,yield_amount,last_collected_at,state,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,'aguardando',$6,$6) RETURNING %s`, machineSelectFields), machineID, userID, item.Name, item.Price, item.DailyYield, now))
	if err != nil {
		return nil, nil, 0, err
	}
	_, err = dbtx.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,external_id,gateway_provider,idempotency_key,reference_id,paid_at,created_at,updated_at) VALUES($1,$2,$3,$4,'sucesso',$5,'internal_rent',$6,$7,$8,$8,$8)`, txID, userID, "Aluguel "+item.Name, (-priceCents).String(), machineID, key, catalogID, now)
	if err != nil {
		return nil, nil, 0, err
	}
	if err = journal(ctx, dbtx, txID, userID, "rent", -item.Price, user.Balance, updated.Balance); err != nil {
		return nil, nil, 0, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, nil, 0, err
	}
	return machine, updated, xp, nil
}
