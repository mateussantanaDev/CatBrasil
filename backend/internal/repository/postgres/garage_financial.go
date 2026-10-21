package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
	"github.com/jackc/pgx/v5"
)

func (r *TransactionRepository) CollectYieldAtomic(ctx context.Context, userID, machineID string) (*domain.User, float64, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer dbtx.Rollback(ctx)
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), userID))
	if err != nil {
		return nil, 0, err
	}
	machine, err := scanMachine(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM machines WHERE id=$1 AND user_id=$2 FOR UPDATE", machineSelectFields), machineID, userID))
	if err != nil {
		return nil, 0, err
	}
	now := time.Now()
	state, _ := machine.CalculateState(now, 10)
	if state == domain.MachineStateAguardando {
		return nil, 0, domain.ErrYieldNotReady
	}
	if state == domain.MachineStateExpirado {
		if _, err = dbtx.Exec(ctx, `UPDATE machines SET last_collected_at=$2,state='aguardando',override_state=NULL,override_seconds_remaining=NULL,updated_at=$2 WHERE id=$1`, machineID, now); err != nil {
			return nil, 0, err
		}
		if err = dbtx.Commit(ctx); err != nil {
			return nil, 0, err
		}
		return nil, 0, domain.ErrYieldExpired
	}
	bonus := float64(user.Nivel-1) * 5
	err = dbtx.QueryRow(ctx, `SELECT bonus_yield_percent FROM level_configs WHERE level=$1`, user.Nivel).Scan(&bonus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, err
	}
	if user.Nivel >= 12 && user.Level12BonusSuspended {
		bonus = 0
	}
	baseCents, moneyErr := money.Positive(machine.YieldAmount)
	if moneyErr != nil {
		return nil, 0, fmt.Errorf("rendimento base inválido: %w", moneyErr)
	}
	basisPoints, moneyErr := money.BasisPoints(bonus)
	if moneyErr != nil {
		return nil, 0, fmt.Errorf("bônus de rendimento inválido: %w", moneyErr)
	}
	bonusCents, moneyErr := baseCents.Percent(basisPoints)
	if moneyErr != nil {
		return nil, 0, moneyErr
	}
	amountCents := baseCents + bonusCents
	amount := amountCents.Float64()
	if !validMoney(amount) {
		return nil, 0, fmt.Errorf("rendimento inválido")
	}
	updated, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET balance=balance+$1,yield_balance_available=yield_balance_available+$1,coletas_completed=coletas_completed+1,xp=xp+25,updated_at=$2 WHERE id=$3 RETURNING %s`, userSelectFields), amountCents.String(), now, userID))
	if err != nil {
		return nil, 0, err
	}
	if _, err = dbtx.Exec(ctx, `UPDATE machines SET last_collected_at=$2,state='aguardando',override_state=NULL,override_seconds_remaining=NULL,updated_at=$2 WHERE id=$1`, machineID, now); err != nil {
		return nil, 0, err
	}
	id, err := financialID("YIELD")
	if err != nil {
		return nil, 0, err
	}
	_, err = dbtx.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider,reference_id,paid_at,created_at,updated_at) VALUES($1,$2,$3,$4,'sucesso','internal_yield',$5,$6,$6,$6)`, id, userID, "Rendimento ["+machine.Name+"]", amountCents.String(), machineID, now)
	if err != nil {
		return nil, 0, err
	}
	if err = journal(ctx, dbtx, id, userID, "machine_yield", amount, user.Balance, updated.Balance); err != nil {
		return nil, 0, err
	}
	periodStart := CurrentWeekPeriodStart(now)
	_, err = dbtx.Exec(ctx, `INSERT INTO mining_race_weekly (period_start,user_id,username,weekly_tonnage,updated_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(period_start,user_id) DO UPDATE SET username=EXCLUDED.username,weekly_tonnage=mining_race_weekly.weekly_tonnage+EXCLUDED.weekly_tonnage,updated_at=EXCLUDED.updated_at`, periodStart, userID, user.Username, amount*10, now)
	if err != nil {
		return nil, 0, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return updated, amount, nil
}

func (r *TransactionRepository) RecoverInsuranceAtomic(ctx context.Context, userID string) (*domain.User, float64, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer dbtx.Rollback(ctx)
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), userID))
	if err != nil {
		return nil, 0, err
	}
	if user.InsuranceVouchersCount <= 0 {
		return nil, 0, fmt.Errorf("você não possui Vale Seguro disponível")
	}
	var total float64
	if err = dbtx.QueryRow(ctx, `SELECT COALESCE(sum(yield_amount),0) FROM machines WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	totalCents, moneyErr := money.FromFloat64(total)
	if moneyErr != nil {
		return nil, 0, moneyErr
	}
	amountCents := totalCents * 2
	amount := amountCents.Float64()
	if !validMoney(amount) {
		return nil, 0, fmt.Errorf("nenhum rendimento elegível para recuperação")
	}
	now := time.Now()
	updated, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET balance=balance+$1,yield_balance_available=yield_balance_available+$1,insurance_vouchers_count=insurance_vouchers_count-1,updated_at=$2 WHERE id=$3 RETURNING %s`, userSelectFields), amountCents.String(), now, userID))
	if err != nil {
		return nil, 0, err
	}
	id, err := financialID("INS")
	if err != nil {
		return nil, 0, err
	}
	_, err = dbtx.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider,paid_at,created_at,updated_at) VALUES($1,$2,'Resgate Vale Seguro CAT (2 Dias)',$3,'sucesso','internal_insurance',$4,$4,$4)`, id, userID, amountCents.String(), now)
	if err != nil {
		return nil, 0, err
	}
	if err = journal(ctx, dbtx, id, userID, "insurance_recovery", amount, user.Balance, updated.Balance); err != nil {
		return nil, 0, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return updated, amount, nil
}

func (r *TransactionRepository) AssemblePartsAtomic(ctx context.Context, userID string) (*domain.User, *domain.Machine, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer dbtx.Rollback(ctx)
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM users WHERE id=$1 FOR UPDATE", userSelectFields), userID))
	if err != nil {
		return nil, nil, err
	}
	if !user.HasCompleteParts() {
		return nil, nil, domain.ErrIncompletePartsAssembly
	}
	updated, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET motor_c15_count=motor_c15_count-1,esteira_reforcada_count=esteira_reforcada_count-1,filtro_oleo_count=filtro_oleo_count-1,turbo_garrett_count=turbo_garrett_count-1,updated_at=now() WHERE id=$1 RETURNING %s`, userSelectFields), userID))
	if err != nil {
		return nil, nil, err
	}
	id, err := financialID("MACH-PARTS")
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	machine, err := scanMachine(dbtx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO machines(id,user_id,name,cost,yield_amount,last_collected_at,state,created_at,updated_at) VALUES($1,$2,'Escavadeira CAT 320 (Oficina de Peças) 🧩',2500,110,$3,'aguardando',$3,$3) RETURNING %s`, machineSelectFields), id, userID, now))
	if err != nil {
		return nil, nil, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return updated, machine, nil
}
