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
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

const transactionSelectFields = `
	id, user_id, tipo, valor, status, external_id, pix_key,
	pix_code, qr_code_url, net_amount, fee_amount, valor_efetivo_pago,
	paid_at, rejection_reason, COALESCE(gateway_provider, 'simulated'), created_at, updated_at,
	idempotency_key, payout_attempted_at, reconciled_at, reference_id
`

func scanTransaction(row pgx.Row) (*domain.Transaction, error) {
	var tx domain.Transaction
	var statusStr string

	err := row.Scan(
		&tx.ID, &tx.UserID, &tx.Tipo, &tx.Valor, &statusStr, &tx.ExternalID, &tx.PixKey,
		&tx.PixCode, &tx.QRCodeURL, &tx.NetAmount, &tx.FeeAmount, &tx.ValorEfetivoPago,
		&tx.PaidAt, &tx.RejectionReason, &tx.GatewayProvider, &tx.CreatedAt, &tx.UpdatedAt,
		&tx.IdempotencyKey, &tx.PayoutAttemptedAt, &tx.ReconciledAt, &tx.ReferenceID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("erro ao mapear transação: %w", err)
	}

	tx.Status = domain.TxStatus(statusStr)
	return &tx, nil
}

func (r *TransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	value, err := money.FromFloat64(tx.Valor)
	if err != nil {
		return err
	}
	net, err := nullableDecimal(tx.NetAmount)
	if err != nil {
		return err
	}
	fee, err := nullableDecimal(tx.FeeAmount)
	if err != nil {
		return err
	}
	paid, err := nullableDecimal(tx.ValorEfetivoPago)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO transactions (
			id, user_id, tipo, valor, status, external_id, pix_key,
			pix_code, qr_code_url, net_amount, fee_amount, valor_efetivo_pago,
			paid_at, rejection_reason, gateway_provider, created_at, updated_at, idempotency_key, reference_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19
		)
	`

	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now()
	}
	tx.UpdatedAt = time.Now()

	status := string(tx.Status)
	if status == "" {
		status = string(domain.TxStatusPendente)
	}

	provider := tx.GatewayProvider
	if provider == "" {
		provider = "simulated"
	}

	_, err = r.pool.Exec(ctx, query,
		tx.ID, tx.UserID, tx.Tipo, value.String(), status, tx.ExternalID, tx.PixKey,
		tx.PixCode, tx.QRCodeURL, net, fee, paid,
		tx.PaidAt, tx.RejectionReason, provider, tx.CreatedAt, tx.UpdatedAt, tx.IdempotencyKey, tx.ReferenceID,
	)
	if err != nil {
		return fmt.Errorf("falha ao inserir transação: %w", err)
	}

	return nil
}

func nullableDecimal(value *float64) (any, error) {
	if value == nil {
		return nil, nil
	}
	cents, err := money.FromFloat64(*value)
	if err != nil {
		return nil, err
	}
	return cents.String(), nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := fmt.Sprintf("SELECT %s FROM transactions WHERE id = $1", transactionSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanTransaction(row)
}

func (r *TransactionRepository) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	// Consulta legada: o lock não cobre escritas subsequentes fora desta chamada.
	query := fmt.Sprintf("SELECT %s FROM transactions WHERE id = $1 FOR UPDATE", transactionSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanTransaction(row)
}

func (r *TransactionRepository) GetByUserID(ctx context.Context, userID string, filter string) ([]*domain.Transaction, error) {
	var query string
	var rows pgx.Rows
	var err error

	filter = strings.ToLower(filter)
	switch filter {
	case "depositos":
		query = fmt.Sprintf("SELECT %s FROM transactions WHERE user_id = $1 AND (LOWER(tipo) LIKE '%%depósito%%' OR LOWER(tipo) LIKE '%%deposito%%') ORDER BY created_at DESC", transactionSelectFields)
		rows, err = r.pool.Query(ctx, query, userID)
	case "saques":
		query = fmt.Sprintf("SELECT %s FROM transactions WHERE user_id = $1 AND LOWER(tipo) LIKE '%%saque%%' ORDER BY created_at DESC", transactionSelectFields)
		rows, err = r.pool.Query(ctx, query, userID)
	case "rendimentos":
		query = fmt.Sprintf("SELECT %s FROM transactions WHERE user_id = $1 AND LOWER(tipo) LIKE '%%rendimento%%' ORDER BY created_at DESC", transactionSelectFields)
		rows, err = r.pool.Query(ctx, query, userID)
	default:
		query = fmt.Sprintf("SELECT %s FROM transactions WHERE user_id = $1 ORDER BY created_at DESC", transactionSelectFields)
		rows, err = r.pool.Query(ctx, query, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao consultar transações do usuário: %w", err)
	}
	defer rows.Close()

	var list []*domain.Transaction
	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, tx)
	}

	return list, nil
}

func (r *TransactionRepository) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if status != "" && status != "todos" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("falha ao contar transações: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	dataQuery := fmt.Sprintf(`
		SELECT %s FROM transactions
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, transactionSelectFields, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao listar transações: %w", err)
	}
	defer rows.Close()

	var list []*domain.Transaction
	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, tx)
	}

	return list, total, nil
}

func (r *TransactionRepository) ListWithdrawals(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := "tipo='Saque PIX'"
	args := []any{}
	if status != "" && status != "todos" {
		where += " AND status=$1"
		args = append(args, status)
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM transactions WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM transactions WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", transactionSelectFields, where, len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*domain.Transaction, 0)
	for rows.Next() {
		item, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

func (r *TransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	net, err := nullableDecimal(tx.NetAmount)
	if err != nil {
		return err
	}
	fee, err := nullableDecimal(tx.FeeAmount)
	if err != nil {
		return err
	}
	paid, err := nullableDecimal(tx.ValorEfetivoPago)
	if err != nil {
		return err
	}
	query := `
		UPDATE transactions SET
			status = $1, external_id = $2, pix_key = $3, pix_code = $4,
			qr_code_url = $5, net_amount = $6, fee_amount = $7, valor_efetivo_pago = $8,
			paid_at = $9, rejection_reason = $10, updated_at = $11
		WHERE id = $12
	`

	tx.UpdatedAt = time.Now()

	cmdTag, err := r.pool.Exec(ctx, query,
		string(tx.Status), tx.ExternalID, tx.PixKey, tx.PixCode,
		tx.QRCodeURL, net, fee, paid,
		tx.PaidAt, tx.RejectionReason, tx.UpdatedAt, tx.ID,
	)
	if err != nil {
		return fmt.Errorf("falha ao atualizar transação: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTransactionNotFound
	}

	return nil
}

// AttachDepositQRCode associa a resposta do gateway sem reverter um callback já confirmado.
func (r *TransactionRepository) AttachDepositQRCode(ctx context.Context, id, externalID, pixCode, qrURL string) (*domain.Transaction, error) {
	dbtx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback(ctx)
	q := fmt.Sprintf(`SELECT %s FROM transactions WHERE id=$1 FOR UPDATE`, transactionSelectFields)
	deposit, err := scanTransaction(dbtx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	if deposit.Tipo != "Depósito PIX" || deposit.GatewayProvider != "suitpay" || externalID == "" || (deposit.ExternalID != nil && *deposit.ExternalID != externalID) || (deposit.Status != domain.TxStatusPendente && deposit.Status != domain.TxStatusSucesso) {
		return nil, domain.ErrInvalidTransactionStatus
	}
	deposit, err = scanTransaction(dbtx.QueryRow(ctx, fmt.Sprintf(`UPDATE transactions SET external_id=$2,pix_code=$3,qr_code_url=$4,updated_at=now() WHERE id=$1 RETURNING %s`, transactionSelectFields), id, externalID, pixCode, qrURL))
	if err != nil {
		return nil, err
	}
	if err = dbtx.Commit(ctx); err != nil {
		return nil, err
	}
	return deposit, nil
}

func (r *TransactionRepository) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	return nil, fmt.Errorf("aprovação direta não confirma transferência PIX; use o serviço de pagamento")
}

// RejectWithdrawal executa rejeição com estorno atômico de saldo e trava opcional de saque via Transação ACID
func (r *TransactionRepository) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("falha ao iniciar transação de rejeição: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Busca e bloqueia a transação de saque (SELECT ... FOR UPDATE)
	queryTx := fmt.Sprintf("SELECT %s FROM transactions WHERE id = $1 FOR UPDATE", transactionSelectFields)
	targetTx, err := scanTransaction(tx.QueryRow(ctx, queryTx, txID))
	if err != nil {
		return nil, err
	}
	if targetTx.Tipo != "Saque PIX" || targetTx.Status != domain.TxStatusPendente || targetTx.PayoutAttemptedAt != nil {
		return nil, fmt.Errorf("apenas transações pendentes podem ser rejeitadas (status atual: %s)", targetTx.Status)
	}

	// 2. Estorna o valor absoluto do saque de volta para o saldo do usuário (SELECT ... FOR UPDATE)
	refundAmount := -targetTx.Valor
	refundCents, moneyErr := money.Positive(refundAmount)
	if moneyErr != nil {
		return nil, domain.ErrInvalidTransactionStatus
	}
	var before, after float64
	if err = tx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, targetTx.UserID).Scan(&before); err != nil {
		return nil, err
	}
	updateUserQuery := `
		UPDATE users SET
			balance = balance + $1,
			withdraw_locked = CASE WHEN $2 = TRUE THEN TRUE ELSE withdraw_locked END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING balance
	`
	err = tx.QueryRow(ctx, updateUserQuery, refundCents.String(), lockAccount, targetTx.UserID).Scan(&after)
	if err != nil {
		return nil, fmt.Errorf("falha ao estornar saldo do usuário: %w", err)
	}
	if err = journal(ctx, tx, txID, targetTx.UserID, "payout_refund", refundAmount, before, after); err != nil {
		return nil, err
	}

	// 3. Atualiza a transação com status rejeitado e motivo
	now := time.Now()
	updateTxQuery := fmt.Sprintf(`
		UPDATE transactions SET
			status = $1,
			rejection_reason = $2,
			updated_at = $3
		WHERE id = $4
		RETURNING %s
	`, transactionSelectFields)

	row := tx.QueryRow(ctx, updateTxQuery, string(domain.TxStatusRejeitado), reason, now, txID)
	updatedTx, err := scanTransaction(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("falha ao comitar rejeição de saque: %w", err)
	}

	return updatedTx, nil
}
