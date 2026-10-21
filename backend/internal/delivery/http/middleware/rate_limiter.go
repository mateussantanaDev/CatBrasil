package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type RateStore interface {
	Allow(context.Context, string, time.Duration, int) (bool, error)
}

type rateRecord struct {
	count     int
	resetTime time.Time
}

// RateLimiter aplica uma janela fixa, local em testes ou compartilhada via RateStore.
type RateLimiter struct {
	mu           sync.Mutex
	records      map[string]*rateRecord
	window       time.Duration
	maxRequests  int
	errorMessage string
	store        RateStore
}

// NewRateLimiter inicializa um novo limitador de taxa
func NewRateLimiter(window time.Duration, maxRequests int, errorMessage string) *RateLimiter {
	limiter := &RateLimiter{
		records:      make(map[string]*rateRecord),
		window:       window,
		maxRequests:  maxRequests,
		errorMessage: errorMessage,
	}

	// Limpeza periódica de chaves expiradas para evitar vazamento de memória
	go func() {
		ticker := time.NewTicker(window * 2)
		for range ticker.C {
			limiter.mu.Lock()
			now := time.Now()
			for k, v := range limiter.records {
				if now.After(v.resetTime) {
					delete(limiter.records, k)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

func NewRateLimiterWithStore(window time.Duration, maxRequests int, errorMessage string, store RateStore) *RateLimiter {
	if store != nil {
		return &RateLimiter{window: window, maxRequests: maxRequests, errorMessage: errorMessage, store: store}
	}
	limiter := NewRateLimiter(window, maxRequests, errorMessage)
	return limiter
}

// Handler retorna o middleware HTTP para ser usado nas rotas
func (rl *RateLimiter) Handler() func(http.Handler) http.Handler {
	return rl.handlerWithKey(func(r *http.Request) string {
		return "ip:" + ClientIPFromRequest(r)
	})
}

// HandlerByUser limits sensitive authenticated operations per account even
// when requests arrive through different IP addresses or proxies.
func (rl *RateLimiter) HandlerByUser() func(http.Handler) http.Handler {
	return rl.handlerWithKey(func(r *http.Request) string {
		return "user:" + GetUserID(r.Context())
	})
}

func (rl *RateLimiter) handlerWithKey(keyForRequest func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}
			key := fmt.Sprintf("%d:%d:%s:%s", rl.window.Milliseconds(), rl.maxRequests, r.URL.Path, keyForRequest(r))
			if rl.store != nil {
				allowed, err := rl.store.Allow(r.Context(), key, rl.window, rl.maxRequests)
				if err != nil {
					RespondError(w, http.StatusServiceUnavailable, "Limitador de requisições indisponível", "RATE_LIMIT_UNAVAILABLE")
					return
				}
				if !allowed {
					rl.reject(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			now := time.Now()

			rl.mu.Lock()
			rec, exists := rl.records[key]
			if !exists || now.After(rec.resetTime) {
				rl.records[key] = &rateRecord{
					count:     1,
					resetTime: now.Add(rl.window),
				}
				rl.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			rec.count++
			if rec.count > rl.maxRequests {
				rl.mu.Unlock()
				rl.reject(w)
				return
			}
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) reject(w http.ResponseWriter) {
	msg := rl.errorMessage
	if msg == "" {
		msg = "Muitas requisições. Aguarde um momento e tente novamente."
	}
	RespondError(w, http.StatusTooManyRequests, msg, "RATE_LIMIT_EXCEEDED")
}
