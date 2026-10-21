package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"
)

// ApplySpin commits the daily guard, prize and inventory together.
func (r *TransactionRepository) ApplySpin(ctx context.Context, userID, label, part string, amount float64) (*domain.User, error) {
	if amount < 0 || (amount > 0 && !validMoney(amount)) {
		return nil, domain.ErrInvalidRouletteSlice
	}
	amountCents, moneyErr := money.FromFloat64(amount)
	if moneyErr != nil {
		return nil, domain.ErrInvalidRouletteSlice
	}
	column := ""
	switch {
	case strings.HasPrefix(part, "Motor"):
		column = "motor_c15_count"
	case strings.HasPrefix(part, "Esteira"):
		column = "esteira_reforcada_count"
	case strings.HasPrefix(part, "Filtro"):
		column = "filtro_oleo_count"
	case strings.HasPrefix(part, "Turbo"):
		column = "turbo_garrett_count"
	case part != "":
		return nil, domain.ErrInvalidRouletteSlice
	}
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	todayRecife := time.Now().In(RecifeLocation).Format("2006-01-02")
	var before float64
	var already bool
	var lastSpinDate *string
	if err = dbtx.QueryRow(ctx, `SELECT balance,already_spun_today,last_spin_date FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&before, &already, &lastSpinDate); err != nil {
		return nil, err
	}
	hasSpunToday := false
	if lastSpinDate != nil && *lastSpinDate != "" {
		hasSpunToday = (*lastSpinDate == todayRecife)
	} else {
		hasSpunToday = already
	}
	if hasSpunToday {
		return nil, domain.ErrSpinAlreadyUsedToday
	}
	partSQL := ""
	if column != "" {
		partSQL = fmt.Sprintf(",%s=%s+1", column, column)
	}
	user, err := scanUser(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE users SET balance=balance+$1,already_spun_today=true,last_spin_date=$3,last_spin_at=now()%s,updated_at=now() WHERE id=$2 RETURNING %s`, partSQL, userSelectFields), amountCents.String(), userID, todayRecife))
	if err != nil {
		return nil, err
	}
	if amount > 0 || part != "" {
		id, err := financialID("SPIN")
		if err != nil {
			return nil, err
		}
		now := time.Now()
		_, err = dbtx.Exec(ctx, `INSERT INTO transactions(id,user_id,tipo,valor,status,gateway_provider,paid_at,created_at,updated_at) VALUES($1,$2,$3,$4,'sucesso','internal_spin',$5,$5,$5)`, id, userID, "Giro Roleta: "+label, amountCents.String(), now)
		if err != nil {
			return nil, err
		}
		if amount > 0 {
			if err = journal(ctx, dbtx, id, userID, "roulette_prize", amount, before, user.Balance); err != nil {
				return nil, err
			}
		}
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, err
	}
	return user, nil
}
