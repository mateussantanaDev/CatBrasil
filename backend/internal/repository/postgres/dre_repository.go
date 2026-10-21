package postgres

import (
	"context"
	"fmt"
	"math"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DRERepository struct {
	pool *pgxpool.Pool
}

func NewDRERepository(pool *pgxpool.Pool) *DRERepository {
	return &DRERepository{pool: pool}
}

func (r *DRERepository) GetMetrics(ctx context.Context) (*domain.DREMetrics, error) {
	// 1. Obtém taxa do gateway da tabela de configuração
	var depositFeePercent float64 = 1.50
	_ = r.pool.QueryRow(ctx, "SELECT deposit_fee_percent FROM system_config WHERE id = 1").Scan(&depositFeePercent)

	// 2. Consolida transações confirmadas (Depósitos, Saques e Prêmios de Roleta)
	txSummaryQuery := `
		SELECT
			COALESCE(SUM(CASE WHEN LOWER(tipo) LIKE '%depósito%' OR LOWER(tipo) LIKE '%deposito%' THEN ABS(valor) ELSE 0 END), 0) AS total_deposits,
			COALESCE(SUM(CASE WHEN LOWER(tipo) LIKE '%saque%' THEN ABS(valor) ELSE 0 END), 0) AS total_withdrawals,
			COALESCE(SUM(CASE WHEN LOWER(tipo) LIKE '%roleta%' OR LOWER(tipo) LIKE '%giro%' THEN ABS(valor) ELSE 0 END), 0) AS total_roulette_prizes
		FROM transactions
		WHERE status = 'sucesso'
	`
	var totalDeposits, totalWithdrawals, totalRoulettePrizes float64
	err := r.pool.QueryRow(ctx, txSummaryQuery).Scan(&totalDeposits, &totalWithdrawals, &totalRoulettePrizes)
	if err != nil {
		return nil, fmt.Errorf("falha ao consolidar totais de transações: %w", err)
	}

	// 3. Saldo total custodiado em carteiras dos operadores
	var totalUserBalances float64
	err = r.pool.QueryRow(ctx, "SELECT COALESCE(SUM(balance), 0) FROM users WHERE role = 'user'").Scan(&totalUserBalances)
	if err != nil {
		return nil, fmt.Errorf("falha ao calcular saldo total dos operadores: %w", err)
	}

	// 4. Métricas de coletas (Sucesso vs Expiradas retidas para a banca)
	usageSummaryQuery := `
		SELECT
			COALESCE(COUNT(CASE WHEN type = 'collect_success' THEN 1 END), 0) AS success_count,
			COALESCE(COUNT(CASE WHEN type = 'collect_expired' THEN 1 END), 0) AS expired_count,
			COALESCE(SUM(CASE WHEN type = 'collect_expired' THEN amount ELSE 0 END), 0) AS retained_expired_yields
		FROM usage_logs
	`
	var successCollectCount, expiredCollectCount int
	var retainedExpiredYields float64
	_ = r.pool.QueryRow(ctx, usageSummaryQuery).Scan(&successCollectCount, &expiredCollectCount, &retainedExpiredYields)

	// 5. Prêmios de giros não realizados retidos (estimativa da banca)
	var retainedUnspunPrizes float64
	var unspunCount int
	_ = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE already_spun_today = FALSE AND role = 'user'").Scan(&unspunCount)
	retainedUnspunPrizes = math.Round(float64(unspunCount)*5.00*100) / 100

	// 6. Aplica a formulação matemática oficial do DRE CatBrasil
	metrics := domain.CalculateDREMetrics(
		totalDeposits,
		depositFeePercent,
		totalWithdrawals,
		totalUserBalances,
		retainedExpiredYields,
		retainedUnspunPrizes,
		totalRoulettePrizes,
		expiredCollectCount,
		successCollectCount,
	)

	return metrics, nil
}
