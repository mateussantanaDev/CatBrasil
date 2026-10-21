package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catbrasil/backend/internal/delivery/http/middleware"
)

type sharedTestStore struct {
	count map[string]int
	fail  bool
}

func (s *sharedTestStore) Allow(_ context.Context, key string, _ time.Duration, limit int) (bool, error) {
	if s.fail {
		return false, errors.New("database unavailable")
	}
	s.count[key]++
	return s.count[key] <= limit, nil
}

func TestRateLimiterSharedStoreAcrossInstancesAndFailClosed(t *testing.T) {
	store := &sharedTestStore{count: map[string]int{}}
	one := middleware.NewRateLimiterWithStore(time.Minute, 2, "", store)
	two := middleware.NewRateLimiterWithStore(time.Minute, 2, "", store)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	request := func(limiter *middleware.RateLimiter, forwarded string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		req.Header.Set("X-Forwarded-For", forwarded)
		rec := httptest.NewRecorder()
		limiter.Handler()(next).ServeHTTP(rec, req)
		return rec.Code
	}
	if request(one, "198.51.100.1") != http.StatusOK || request(two, "198.51.100.2") != http.StatusOK {
		t.Fatal("primeiras duas tentativas deveriam passar")
	}
	if got := request(two, "203.0.113.99"); got != http.StatusTooManyRequests {
		t.Fatalf("cabeçalho forjado ou segunda réplica contornou o limite: HTTP %d", got)
	}
	store.fail = true
	if got := request(one, "198.51.100.1"); got != http.StatusServiceUnavailable {
		t.Fatalf("falha do limite compartilhado deve fechar acesso: HTTP %d", got)
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := middleware.NewRateLimiter(500*time.Millisecond, 2, "Limite atingido")
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := limiter.Handler()(dummyHandler)

	req1 := httptest.NewRequest("GET", "/test-rate", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("req 1: expected 200, got %d", rr1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test-rate", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("req 2: expected 200, got %d", rr2.Code)
	}

	req3 := httptest.NewRequest("GET", "/test-rate", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("req 3: expected 429 Too Many Requests, got %d", rr3.Code)
	}
}

func TestRateLimiterByUserAcrossIPs(t *testing.T) {
	limiter := middleware.NewRateLimiter(time.Minute, 2, "Limite atingido")
	handler := limiter.HandlerByUser()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := func(userID, ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/user/security-password", nil)
		req.RemoteAddr = ip
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}
	if request("user-1", "10.0.0.1") != http.StatusOK || request("user-1", "10.0.0.2") != http.StatusOK {
		t.Fatal("primeiras tentativas do usuário deveriam passar")
	}
	if status := request("user-1", "10.0.0.3"); status != http.StatusTooManyRequests {
		t.Fatalf("mudança de IP contornou o limite: HTTP %d", status)
	}
	if status := request("user-2", "10.0.0.3"); status != http.StatusOK {
		t.Fatalf("limite afetou outro usuário: HTTP %d", status)
	}
}
