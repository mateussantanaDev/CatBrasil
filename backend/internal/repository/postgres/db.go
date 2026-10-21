package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDBConfigInvalid  = errors.New("falha ao analisar configuração de conexão com PostgreSQL")
	ErrDBPoolInitFailed = errors.New("falha ao inicializar pool de conexões com PostgreSQL")
	ErrDBPingFailed     = errors.New("falha de conectividade (ping) com PostgreSQL")
	ErrDBConnection     = errors.New("falha de conexão com PostgreSQL")
	ErrDBSchemaCheck    = errors.New("falha na validação do schema do banco de dados")
	ErrDBMigration      = errors.New("falha na execução de migrações do banco de dados")
)

// SanitizeDBError classifica um erro e devolve uma categoria estática segura,
// sem jamais interpolar ou inspecionar err.Error(), garantindo ausência total de vazamento de DSNs.
func SanitizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrDBConfigInvalid) {
		return ErrDBConfigInvalid
	}
	if errors.Is(err, ErrDBPoolInitFailed) {
		return ErrDBPoolInitFailed
	}
	if errors.Is(err, ErrDBPingFailed) {
		return ErrDBPingFailed
	}
	if errors.Is(err, ErrDBSchemaCheck) {
		return ErrDBSchemaCheck
	}
	if errors.Is(err, ErrDBMigration) {
		return ErrDBMigration
	}
	return ErrDBConnection
}

// DB encapsulates the pgx connection pool and helper methods
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB initializes a new PostgreSQL connection pool configured for high concurrency
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, ErrDBConfigInvalid
	}

	// Performance Tuning para VPS 8GB RAM / 10 vCPUs
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, ErrDBPoolInitFailed
	}

	// Testa conectividade imediata
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, ErrDBPingFailed
	}

	log.Println("🐘 Conexão com PostgreSQL 16 (pgxpool) estabelecida com sucesso!")
	return &DB{Pool: pool}, nil
}

// Close gracefully terminates all connections in the pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// WithTransaction executes a function within an ACID transaction with automatic Rollback on error
func (db *DB) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("erro na transação (%v) e falha no rollback (%v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("falha ao comitar transação: %w", err)
	}

	return nil
}
