package http

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMetricsRequireTokenAndUseRoutePattern(t *testing.T) {
	token := strings.Repeat("s", 32)
	metrics := newRequestMetrics(token)
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/users/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	router.Get("/internal/metrics", metrics.ServeHTTP)

	for _, id := range []string{"alice", "bob"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
		if response.Code != http.StatusAccepted {
			t.Fatalf("request %s: %d", id, response.Code)
		}
	}
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("metrics sem token: %d", unauthorized.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics com token: %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `catbrasil_http_requests_total{method="GET",route="/users/{id}",status="202"} 2`) {
		t.Fatalf("sem contador agregado por rota: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "alice") || strings.Contains(response.Body.String(), "bob") {
		t.Fatal("métrica expôs identificador de usuário")
	}
}

func TestMetricsEndpointProtection_BothPathsProtected(t *testing.T) {
	token := strings.Repeat("m", 32)
	metrics := newRequestMetrics(token)
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/internal/metrics", metrics.ServeHTTP)
	router.Get("/metrics", metrics.ServeHTTP)

	for _, path := range []string{"/internal/metrics", "/metrics"} {
		// Sem token -> 401
		reqUnauth := httptest.NewRequest(http.MethodGet, path, nil)
		rrUnauth := httptest.NewRecorder()
		router.ServeHTTP(rrUnauth, reqUnauth)
		if rrUnauth.Code != http.StatusUnauthorized {
			t.Fatalf("%s sem token: esperado 401, obteve %d", path, rrUnauth.Code)
		}

		// Token errado -> 401
		reqWrong := httptest.NewRequest(http.MethodGet, path, nil)
		reqWrong.Header.Set("Authorization", "Bearer invalid-token")
		rrWrong := httptest.NewRecorder()
		router.ServeHTTP(rrWrong, reqWrong)
		if rrWrong.Code != http.StatusUnauthorized {
			t.Fatalf("%s com token inválido: esperado 401, obteve %d", path, rrWrong.Code)
		}

		// Token correto -> 200
		reqAuth := httptest.NewRequest(http.MethodGet, path, nil)
		reqAuth.Header.Set("Authorization", "Bearer "+token)
		rrAuth := httptest.NewRecorder()
		router.ServeHTTP(rrAuth, reqAuth)
		if rrAuth.Code != http.StatusOK {
			t.Fatalf("%s com token válido: esperado 200, obteve %d", path, rrAuth.Code)
		}
	}
}

func TestMetricsMiddleware_StructuredLog_NeverLeaksQueryParams(t *testing.T) {
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	metrics := newRequestMetrics("test-token")
	metrics.SetLogger(testLogger)

	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/api/v1/check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	canary := "canary_secret_token_1234567890"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/check?token="+canary+"&foo=bar", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obteve %d", rr.Code)
	}

	logs := logBuf.String()

	// 1. Deve conter os campos estruturados seguros
	if !strings.Contains(logs, `"method":"GET"`) || !strings.Contains(logs, `"route":"/api/v1/check"`) || !strings.Contains(logs, `"status":200`) {
		t.Fatalf("log estruturado não contém os campos esperados: %s", logs)
	}

	// 2. CANÁRIO: O token da query string NUNCA pode aparecer no log
	if strings.Contains(logs, canary) {
		t.Fatalf("VIOLAÇÃO DE SEGURANÇA: query param sensível vazou no log: %s", logs)
	}
	if strings.Contains(logs, "foo=bar") {
		t.Fatalf("VIOLAÇÃO DE SEGURANÇA: query params vazaram no log: %s", logs)
	}
}

func TestMetricsWithFakeCronCallback_AllowlistAndGauges(t *testing.T) {
	token := strings.Repeat("k", 32)
	fakeCallback := func(ctx context.Context) (*CronObservabilityData, error) {
		return &CronObservabilityData{
			Jobs: []CronJobMetric{
				{
					JobType:              "daily_roulette_reset",
					LastSuccessTimestamp: 1718000000,
					RecentFailures:       0,
				},
				{
					JobType:              "weekly_season_close",
					LastSuccessTimestamp: 1717500000,
					RecentFailures:       2,
				},
				{
					JobType:              "injected_unauthorized_job",
					LastSuccessTimestamp: 9999999999,
					RecentFailures:       99,
				},
			},
			PendingWeeklyPeriods: 3,
			PendingDailyPeriods:  1,
		}, nil
	}

	metrics := newRequestMetrics(token, fakeCallback)
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/internal/metrics", metrics.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obteve %d", rr.Code)
	}

	body := rr.Body.String()

	// 1. Deve conter catbrasil_cron_metrics_available 1 e pending weekly e daily periods
	if !strings.Contains(body, "catbrasil_cron_metrics_available 1") {
		t.Fatalf("ausente gauge catbrasil_cron_metrics_available: %s", body)
	}
	if !strings.Contains(body, "catbrasil_cron_pending_weekly_periods 3") {
		t.Fatalf("ausente gauge catbrasil_cron_pending_weekly_periods: %s", body)
	}
	if !strings.Contains(body, "catbrasil_cron_pending_daily_periods 1") {
		t.Fatalf("ausente gauge catbrasil_cron_pending_daily_periods: %s", body)
	}

	// 2. Deve conter os gauges para jobs na allowlist
	if !strings.Contains(body, `catbrasil_cron_last_success_timestamp_seconds{job="daily_roulette_reset"} 1718000000`) {
		t.Fatalf("ausente gauge de last_success para daily_roulette_reset: %s", body)
	}
	if !strings.Contains(body, `catbrasil_cron_recent_failures_total{job="daily_roulette_reset"} 0`) {
		t.Fatalf("ausente gauge de recent_failures para daily_roulette_reset: %s", body)
	}
	if !strings.Contains(body, `catbrasil_cron_last_success_timestamp_seconds{job="weekly_season_close"} 1717500000`) {
		t.Fatalf("ausente gauge de last_success para weekly_season_close: %s", body)
	}
	if !strings.Contains(body, `catbrasil_cron_recent_failures_total{job="weekly_season_close"} 2`) {
		t.Fatalf("ausente gauge de recent_failures para weekly_season_close: %s", body)
	}

	// 3. Deve barrar jobs fora da allowlist
	if strings.Contains(body, "injected_unauthorized_job") {
		t.Fatalf("VIOLAÇÃO: métrica expôs job fora da allowlist: %s", body)
	}

	// 4. Não deve expor IDs ou period_key
	if strings.Contains(body, "period_key") || strings.Contains(body, "user_id") {
		t.Fatalf("VIOLAÇÃO: métrica expôs campos de alta cardinalidade: %s", body)
	}
}

func TestMetricsWithFakeCronCallback_DBErrorSilenced(t *testing.T) {
	token := strings.Repeat("x", 32)
	fakeErrCallback := func(ctx context.Context) (*CronObservabilityData, error) {
		return nil, errors.New("fatal postgres dsn=postgres://catadmin:secret123@db:5432 query error")
	}

	metrics := newRequestMetrics(token, fakeErrCallback)
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/internal/metrics", metrics.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Mesmo com erro de banco no callback, o endpoint responde 200 servindo as demais métricas do processo
	if rr.Code != http.StatusOK {
		t.Fatalf("esperado status 200 mesmo com erro no callback do banco, obteve %d", rr.Code)
	}

	body := rr.Body.String()

	// Garante que catbrasil_up e go_goroutines continuam sendo expostas
	if !strings.Contains(body, "catbrasil_up 1") {
		t.Fatalf("esperado catbrasil_up 1 no corpo: %s", body)
	}

	// Garante que catbrasil_cron_metrics_available é emitido como 0 em caso de erro na coleta do DB
	if !strings.Contains(body, "catbrasil_cron_metrics_available 0") {
		t.Fatalf("esperado catbrasil_cron_metrics_available 0 em caso de falha no DB: %s", body)
	}

	// NUNCA DEVE VAZAR erro SQL bruto ou DSN no corpo da resposta
	for _, leak := range []string{"secret123", "postgres://", "fatal postgres"} {
		if strings.Contains(body, leak) {
			t.Fatalf("VIOLAÇÃO DE SEGURANÇA: corpo expôs string bruta de banco (%s): %s", leak, body)
		}
	}
}




