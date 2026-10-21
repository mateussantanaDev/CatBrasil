package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/money"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepository struct {
	pool *pgxpool.Pool
}

func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{pool: pool}
}

const catalogSelectFields = `
	id, name, category, price, daily_yield, return_percent_per_day,
	duration_days, total_contract_return, stock_quantity, max_purchases_per_user,
	reinvest_only, image_url, active, created_at, updated_at
`

func scanCatalogItem(row pgx.Row) (*domain.FleetCatalogItem, error) {
	var item domain.FleetCatalogItem
	err := row.Scan(
		&item.ID, &item.Name, &item.Category, &item.Price, &item.DailyYield, &item.ReturnPercentPerDay,
		&item.DurationDays, &item.TotalContractReturn, &item.StockQuantity, &item.MaxPurchasesPerUser,
		&item.ReinvestOnly, &item.ImageURL, &item.Active, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMachineCatalogNotFound
		}
		return nil, fmt.Errorf("erro ao mapear item de catálogo: %w", err)
	}
	return &item, nil
}

func (r *CatalogRepository) GetAll(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	query := fmt.Sprintf("SELECT %s FROM fleet_catalog ORDER BY price ASC", catalogSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar catálogo: %w", err)
	}
	defer rows.Close()

	var items []*domain.FleetCatalogItem
	for rows.Next() {
		item, err := scanCatalogItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *CatalogRepository) GetActive(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	query := fmt.Sprintf("SELECT %s FROM fleet_catalog WHERE active = TRUE ORDER BY price ASC", catalogSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar catálogo ativo: %w", err)
	}
	defer rows.Close()

	var items []*domain.FleetCatalogItem
	for rows.Next() {
		item, err := scanCatalogItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *CatalogRepository) GetByID(ctx context.Context, id string) (*domain.FleetCatalogItem, error) {
	query := fmt.Sprintf("SELECT %s FROM fleet_catalog WHERE id = $1", catalogSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanCatalogItem(row)
}

func (r *CatalogRepository) Upsert(ctx context.Context, item *domain.FleetCatalogItem) error {
	price, err := money.FromFloat64(item.Price)
	if err != nil {
		return err
	}
	dailyYield, err := money.FromFloat64(item.DailyYield)
	if err != nil {
		return err
	}
	totalReturn, err := money.FromFloat64(item.TotalContractReturn)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO fleet_catalog (
			id, name, category, price, daily_yield, return_percent_per_day,
			duration_days, total_contract_return, stock_quantity, max_purchases_per_user,
			reinvest_only, image_url, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			price = EXCLUDED.price,
			daily_yield = EXCLUDED.daily_yield,
			return_percent_per_day = EXCLUDED.return_percent_per_day,
			duration_days = EXCLUDED.duration_days,
			total_contract_return = EXCLUDED.total_contract_return,
			stock_quantity = EXCLUDED.stock_quantity,
			max_purchases_per_user = EXCLUDED.max_purchases_per_user,
			reinvest_only = EXCLUDED.reinvest_only,
			image_url = EXCLUDED.image_url,
			active = EXCLUDED.active,
			updated_at = CURRENT_TIMESTAMP
	`

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	item.UpdatedAt = time.Now()

	_, err = r.pool.Exec(ctx, query,
		item.ID, item.Name, item.Category, price.String(), dailyYield.String(), item.ReturnPercentPerDay,
		item.DurationDays, totalReturn.String(), item.StockQuantity, item.MaxPurchasesPerUser,
		item.ReinvestOnly, item.ImageURL, item.Active, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (r *CatalogRepository) SaveCatalog(ctx context.Context, items []*domain.FleetCatalogItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO fleet_catalog (
			id, name, category, price, daily_yield, return_percent_per_day,
			duration_days, total_contract_return, stock_quantity, max_purchases_per_user,
			reinvest_only, image_url, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			price = EXCLUDED.price,
			daily_yield = EXCLUDED.daily_yield,
			return_percent_per_day = EXCLUDED.return_percent_per_day,
			duration_days = EXCLUDED.duration_days,
			total_contract_return = EXCLUDED.total_contract_return,
			stock_quantity = EXCLUDED.stock_quantity,
			max_purchases_per_user = EXCLUDED.max_purchases_per_user,
			reinvest_only = EXCLUDED.reinvest_only,
			image_url = EXCLUDED.image_url,
			active = EXCLUDED.active,
			updated_at = CURRENT_TIMESTAMP
	`

	for _, item := range items {
		price, err := money.FromFloat64(item.Price)
		if err != nil {
			return err
		}
		dailyYield, err := money.FromFloat64(item.DailyYield)
		if err != nil {
			return err
		}
		totalReturn, err := money.FromFloat64(item.TotalContractReturn)
		if err != nil {
			return err
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}
		item.UpdatedAt = time.Now()

		_, err = tx.Exec(ctx, query,
			item.ID, item.Name, item.Category, price.String(), dailyYield.String(), item.ReturnPercentPerDay,
			item.DurationDays, totalReturn.String(), item.StockQuantity, item.MaxPurchasesPerUser,
			item.ReinvestOnly, item.ImageURL, item.Active, item.CreatedAt, item.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("falha ao salvar item %s no catálogo: %w", item.ID, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *CatalogRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM fleet_catalog WHERE id = $1", id)
	return err
}
