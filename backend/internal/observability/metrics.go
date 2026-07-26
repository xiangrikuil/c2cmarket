package observability

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/database"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/health"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/platform/outboundhttp"
	"c2c-market/backend/internal/realtime"
	"c2c-market/backend/internal/store/postgres"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "c2c_market"

type DatabaseSource interface {
	DatabasePoolStats() database.PoolStats
	ContactCryptoStats() postgres.ContactCryptoStats
	Readiness(context.Context) health.Status
	SlowActiveQueryCount(context.Context, time.Duration) (int64, error)
}

type RateLimiterSource interface {
	Stats() middleware.RateLimiterStats
}

type MaintenanceSource interface {
	Stats() maintenance.Stats
}

type OutboundPolicySource interface {
	Stats() outboundhttp.PolicyStats
}

type RealtimeHubSource interface {
	Stats() realtime.HubStats
}

type RealtimeListenerSource interface {
	Stats() realtime.ListenerStats
}

type Sources struct {
	Database         DatabaseSource
	RateLimiter      RateLimiterSource
	Maintenance      MaintenanceSource
	OutboundPolicy   OutboundPolicySource
	RealtimeHub      RealtimeHubSource
	RealtimeListener RealtimeListenerSource
	SlowQueryAfter   time.Duration
}

type Metrics struct {
	registry             *prometheus.Registry
	requestsTotal        *prometheus.CounterVec
	requestDuration      *prometheus.HistogramVec
	operationFailures    *prometheus.CounterVec
	idempotencyConflicts prometheus.Counter
}

type recorderContextKey struct{}

func New(sources Sources) *Metrics {
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "HTTP requests by method, chi route pattern, and status.",
	}, []string{"method", "route", "status"})
	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration by method and chi route pattern.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})
	operationFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "operations",
		Name:      "failures_total",
		Help:      "Failures at selected operational boundaries.",
	}, []string{"operation", "status_class"})
	idempotencyConflicts := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "idempotency",
		Name:      "conflicts_total",
		Help:      "Idempotency conflicts returned to clients.",
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		requestsTotal,
		requestDuration,
		operationFailures,
		idempotencyConflicts,
		newRuntimeCollector(sources),
	)
	return &Metrics{
		registry:             registry,
		requestsTotal:        requestsTotal,
		requestDuration:      requestDuration,
		operationFailures:    operationFailures,
		idempotencyConflicts: idempotencyConflicts,
	}
}

func (m *Metrics) Handler() http.Handler {
	if m == nil || m.registry == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		ErrorHandling:     promhttp.ContinueOnError,
	})
}

func (m *Metrics) Instrument(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		ctx := context.WithValue(r.Context(), recorderContextKey{}, m)
		next.ServeHTTP(recorder, r.WithContext(ctx))

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		m.requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		m.requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(startedAt).Seconds())
		if status >= http.StatusBadRequest {
			if operation := failureOperation(route); operation != "" {
				m.operationFailures.WithLabelValues(operation, statusClass(status)).Inc()
			}
		}
	})
}

func RecordProblem(ctx context.Context, err error) {
	if ctx == nil || err == nil {
		return
	}
	metrics, _ := ctx.Value(recorderContextKey{}).(*Metrics)
	if metrics == nil {
		return
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		return
	}
	switch appErr.Code {
	case domain.CodeIdempotencyInProgress,
		domain.CodeIdempotencyKeyReused,
		domain.CodeIdempotencyResultNotReplayable:
		metrics.idempotencyConflicts.Inc()
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func failureOperation(route string) string {
	switch route {
	case "/api/v1/auth/oauth/callback":
		return "oauth_callback"
	case "/api/v1/me/email-verification/start":
		return "email_verification_send"
	case "/api/v1/me/email-verification/confirm":
		return "email_verification_verify"
	default:
		return ""
	}
}

func statusClass(status int) string {
	return strconv.Itoa(status/100) + "xx"
}
