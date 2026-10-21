package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"catbrasil/backend/internal/repository/postgres"
)

func TestRateLimitStoreSharedAndAtomic(t *testing.T) {
	pool := financialDB(t)
	key := fmt.Sprintf("test-rate-%d", time.Now().UnixNano())
	one := postgres.NewRateLimitStore(pool)
	two := postgres.NewRateLimitStore(pool)
	const attempts, limit = 24, 5
	results := make(chan bool, attempts)
	errors := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			store := one
			if index%2 == 0 {
				store = two
			}
			allowed, err := store.Allow(context.Background(), key, time.Minute, limit)
			results <- allowed
			errors <- err
		}(i)
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}
	if allowed != limit {
		t.Fatalf("esperadas %d requisições entre réplicas, aceitas %d", limit, allowed)
	}
}
