package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RateLimitStore shares counters across every API replica. PostgreSQL serializes
// competing UPSERTs on the key, so concurrent attempts cannot exceed the limit.
type RateLimitStore struct{ pool *pgxpool.Pool }

func NewRateLimitStore(pool *pgxpool.Pool) *RateLimitStore { return &RateLimitStore{pool: pool} }

func (s *RateLimitStore) Allow(ctx context.Context, key string, window time.Duration, maxRequests int) (bool, error) {
	digest := sha256.Sum256([]byte(key))
	var count int
	err := s.pool.QueryRow(ctx, `INSERT INTO http_rate_limits (key_hash, hits, reset_at)
		VALUES ($1, 1, now() + $2::bigint * interval '1 millisecond')
		ON CONFLICT (key_hash) DO UPDATE SET
			hits = CASE WHEN http_rate_limits.reset_at <= now() THEN 1 ELSE LEAST(http_rate_limits.hits + 1, $3) END,
			reset_at = CASE WHEN http_rate_limits.reset_at <= now() THEN now() + $2::bigint * interval '1 millisecond' ELSE http_rate_limits.reset_at END
		RETURNING hits`, hex.EncodeToString(digest[:]), window.Milliseconds(), maxRequests+1).Scan(&count)
	if err != nil {
		return false, err
	}
	return count <= maxRequests, nil
}

func (s *RateLimitStore) Cleanup(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM http_rate_limits WHERE reset_at < now() - interval '1 hour'`)
	return err
}
