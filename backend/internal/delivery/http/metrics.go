package http

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

var requestDurationBuckets = [...]float64{0.1, 0.5, 1, 5}

// CronJobMetric representa os sinais consolidados de um job do cron para observabilidade
type CronJobMetric struct {
	JobType              string
	LastSuccessTimestamp int64
	RecentFailures       int64
}

// CronObservabilityData encapsula todos os dados do cron para o endpoint de métricas
type CronObservabilityData struct {
	Jobs                 []CronJobMetric
	PendingWeeklyPeriods int64
	PendingDailyPeriods  int64
}

// CronMetricsCallback define a função segura de consulta a métricas do cron
type CronMetricsCallback func(ctx context.Context) (*CronObservabilityData, error)

type requestMetricKey struct {
	method string
	route  string
	status int
}

type requestMetric struct {
	count   uint64
	sum     float64
	buckets [len(requestDurationBuckets)]uint64
}

type requestMetrics struct {
	mu           sync.Mutex
	values       map[requestMetricKey]*requestMetric
	token        string
	cronCallback CronMetricsCallback
	logger       *slog.Logger
}

func newRequestMetrics(token string, cronCallback ...CronMetricsCallback) *requestMetrics {
	var cb CronMetricsCallback
	if len(cronCallback) > 0 {
		cb = cronCallback[0]
	}
	return &requestMetrics{
		token:        token,
		values:       make(map[requestMetricKey]*requestMetric),
		cronCallback: cb,
		logger:       slog.Default(),
	}
}

// SetLogger configura o logger estruturado para o middleware de métricas
func (m *requestMetrics) SetLogger(l *slog.Logger) {
	if l != nil {
		m.logger = l
	}
}

func (m *requestMetrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(started).Seconds()
		method := r.Method
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		default:
			method = "OTHER"
		}
		key := requestMetricKey{method: method, route: route, status: status}
		m.mu.Lock()
		value := m.values[key]
		if value == nil {
			value = &requestMetric{}
			m.values[key] = value
		}
		value.count++
		value.sum += elapsed
		for index, boundary := range requestDurationBuckets {
			if elapsed <= boundary {
				value.buckets[index]++
			}
		}
		m.mu.Unlock()

		// Log estruturado com route pattern estático, método, status e duração.
		// NUNCA loga query string, raw URL, headers ou body para proteger credenciais e PII.
		m.logger.Info("http_request",
			slog.String("method", method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Float64("duration_sec", elapsed),
		)
	})
}

func (m *requestMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	provided := r.Header.Get("Authorization")
	expected := "Bearer " + m.token
	if m.token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Consulta executada estritamente fora de qualquer lock de mutex. Erros de banco são silenciados sem vazar no corpo.
	var cronData *CronObservabilityData
	cronAvailable := 0
	if m.cronCallback != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		data, err := m.cronCallback(ctx)
		cancel()
		if err == nil && data != nil {
			cronData = data
			cronAvailable = 1
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprintln(w, "# HELP catbrasil_up API process is serving requests.")
	_, _ = fmt.Fprintln(w, "# TYPE catbrasil_up gauge")
	_, _ = fmt.Fprintln(w, "catbrasil_up 1")
	_, _ = fmt.Fprintln(w, "# HELP catbrasil_http_requests_total HTTP requests by method, route and status.")
	_, _ = fmt.Fprintln(w, "# TYPE catbrasil_http_requests_total counter")
	_, _ = fmt.Fprintln(w, "# HELP catbrasil_http_request_duration_seconds HTTP request duration.")
	_, _ = fmt.Fprintln(w, "# TYPE catbrasil_http_request_duration_seconds histogram")

	m.mu.Lock()
	keys := make([]requestMetricKey, 0, len(m.values))
	snapshot := make(map[requestMetricKey]requestMetric, len(m.values))
	for key := range m.values {
		keys = append(keys, key)
		snapshot[key] = *m.values[key]
	}
	m.mu.Unlock()
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].status < keys[j].status
	})
	for _, key := range keys {
		value := snapshot[key]
		labels := fmt.Sprintf("method=%s,route=%s,status=%s", strconv.Quote(key.method), strconv.Quote(key.route), strconv.Quote(strconv.Itoa(key.status)))
		_, _ = fmt.Fprintf(w, "catbrasil_http_requests_total{%s} %d\n", labels, value.count)
		for index, boundary := range requestDurationBuckets {
			_, _ = fmt.Fprintf(w, "catbrasil_http_request_duration_seconds_bucket{%s,le=%s} %d\n", labels, strconv.Quote(strconv.FormatFloat(boundary, 'f', -1, 64)), value.buckets[index])
		}
		_, _ = fmt.Fprintf(w, "catbrasil_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, value.count)
		_, _ = fmt.Fprintf(w, "catbrasil_http_request_duration_seconds_sum{%s} %g\n", labels, value.sum)
		_, _ = fmt.Fprintf(w, "catbrasil_http_request_duration_seconds_count{%s} %d\n", labels, value.count)
	}

	allowlist := map[string]bool{
		"daily_roulette_reset": true,
		"weekly_season_close":  true,
	}

	if m.cronCallback != nil {
		_, _ = fmt.Fprintln(w, "# HELP catbrasil_cron_metrics_available 1 if cron metrics were collected successfully, 0 on database or collector failure.")
		_, _ = fmt.Fprintln(w, "# TYPE catbrasil_cron_metrics_available gauge")
		_, _ = fmt.Fprintf(w, "catbrasil_cron_metrics_available %d\n", cronAvailable)
	}

	if cronData != nil {
		_, _ = fmt.Fprintln(w, "# HELP catbrasil_cron_pending_weekly_periods Number of past weekly mining race seasons awaiting closure.")
		_, _ = fmt.Fprintln(w, "# TYPE catbrasil_cron_pending_weekly_periods gauge")
		_, _ = fmt.Fprintf(w, "catbrasil_cron_pending_weekly_periods %d\n", cronData.PendingWeeklyPeriods)

		_, _ = fmt.Fprintln(w, "# HELP catbrasil_cron_pending_daily_periods Number of past or overdue daily roulette reset periods awaiting execution.")
		_, _ = fmt.Fprintln(w, "# TYPE catbrasil_cron_pending_daily_periods gauge")
		_, _ = fmt.Fprintf(w, "catbrasil_cron_pending_daily_periods %d\n", cronData.PendingDailyPeriods)

		if len(cronData.Jobs) > 0 {
			_, _ = fmt.Fprintln(w, "# HELP catbrasil_cron_last_success_timestamp_seconds Unix timestamp of last completed cron execution.")
			_, _ = fmt.Fprintln(w, "# TYPE catbrasil_cron_last_success_timestamp_seconds gauge")
			_, _ = fmt.Fprintln(w, "# HELP catbrasil_cron_recent_failures_total Count of cron failures in the last 24 hours.")
			_, _ = fmt.Fprintln(w, "# TYPE catbrasil_cron_recent_failures_total gauge")
			sort.Slice(cronData.Jobs, func(i, j int) bool {
				return cronData.Jobs[i].JobType < cronData.Jobs[j].JobType
			})
			for _, cm := range cronData.Jobs {
				if !allowlist[cm.JobType] {
					continue
				}
				jobLabel := fmt.Sprintf("job=%s", strconv.Quote(cm.JobType))
				_, _ = fmt.Fprintf(w, "catbrasil_cron_last_success_timestamp_seconds{%s} %d\n", jobLabel, cm.LastSuccessTimestamp)
				_, _ = fmt.Fprintf(w, "catbrasil_cron_recent_failures_total{%s} %d\n", jobLabel, cm.RecentFailures)
			}
		}
	}

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	_, _ = fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
	_, _ = fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", memory.Alloc)
}
