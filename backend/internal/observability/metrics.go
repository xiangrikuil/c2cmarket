package observability

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/apihealthrunner"
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

type APIHealthRunnerSource interface {
	Stats() apihealthrunner.Stats
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
	APIHealthRunner  APIHealthRunnerSource
	OutboundPolicy   OutboundPolicySource
	RealtimeHub      RealtimeHubSource
	RealtimeListener RealtimeListenerSource
	SlowQueryAfter   time.Duration
	FailureLogger    *log.Logger
}

type Metrics struct {
	registry              *prometheus.Registry
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
	operationFailures     *prometheus.CounterVec
	securityFailures      *prometheus.CounterVec
	passwordResetDelivery *prometheus.CounterVec
	idempotencyConflicts  prometheus.Counter
	failureLogger         *log.Logger
	failureLogMu          sync.Mutex
}

type recorderContextKey struct{}

type requestProblemRecorder struct {
	metrics *Metrics
	mu      sync.Mutex
	seen    map[string]struct{}
}

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
	securityFailures := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "operations",
		Name:      "security_failures_total",
		Help:      "Selected authentication, request-integrity, authorization, and rate-limit failures using bounded labels.",
	}, []string{"category", "result", "route"})
	passwordResetDelivery := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "auth",
		Name:      "password_reset_email_total",
		Help:      "Password reset email delivery attempts by bounded outcome.",
	}, []string{"outcome"})
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
		securityFailures,
		passwordResetDelivery,
		idempotencyConflicts,
		newRuntimeCollector(sources),
	)
	failureLogger := sources.FailureLogger
	if failureLogger == nil {
		failureLogger = log.Default()
	}
	return &Metrics{
		registry:              registry,
		requestsTotal:         requestsTotal,
		requestDuration:       requestDuration,
		operationFailures:     operationFailures,
		securityFailures:      securityFailures,
		passwordResetDelivery: passwordResetDelivery,
		idempotencyConflicts:  idempotencyConflicts,
		failureLogger:         failureLogger,
	}
}

func (m *Metrics) RecordPasswordResetDelivery(outcome string) {
	if m == nil || m.passwordResetDelivery == nil {
		return
	}
	if outcome != "sent" && outcome != "failed" {
		outcome = "failed"
	}
	m.passwordResetDelivery.WithLabelValues(outcome).Inc()
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
		ctx := context.WithValue(r.Context(), recorderContextKey{}, &requestProblemRecorder{
			metrics: m,
			seen:    make(map[string]struct{}),
		})
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

func RecordProblem(r *http.Request, err error) {
	if r == nil || err == nil {
		return
	}
	recorder, _ := r.Context().Value(recorderContextKey{}).(*requestProblemRecorder)
	if recorder == nil || recorder.metrics == nil {
		return
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		return
	}
	if !recorder.markFirst(appErr.Code) {
		return
	}
	switch appErr.Code {
	case domain.CodeIdempotencyInProgress,
		domain.CodeIdempotencyKeyReused,
		domain.CodeIdempotencyResultNotReplayable:
		recorder.metrics.idempotencyConflicts.Inc()
	}

	category, ok := securityFailureCategory(appErr.Code)
	if !ok {
		return
	}
	routeKey := securityFailureRouteKey(r)
	actorKind := securityFailureActorKind(r)
	recorder.metrics.securityFailures.WithLabelValues(category, appErr.Code, routeKey).Inc()
	entry, marshalErr := json.Marshal(struct {
		RequestID string `json:"request_id"`
		RouteKey  string `json:"route_key"`
		Result    string `json:"result_code"`
		Status    int    `json:"status"`
		ActorKind string `json:"actor_kind"`
	}{
		RequestID: middleware.RequestIDFromRequest(r),
		RouteKey:  routeKey,
		Result:    appErr.Code,
		Status:    appErr.Status,
		ActorKind: actorKind,
	})
	if marshalErr == nil && recorder.metrics.failureLogger != nil {
		recorder.metrics.failureLogMu.Lock()
		_, _ = recorder.metrics.failureLogger.Writer().Write(append(entry, '\n'))
		recorder.metrics.failureLogMu.Unlock()
	}
}

func (r *requestProblemRecorder) markFirst(code string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.seen[code]; exists {
		return false
	}
	r.seen[code] = struct{}{}
	return true
}

func securityFailureCategory(code string) (string, bool) {
	switch code {
	case domain.CodeInvalidCredentials:
		return "authentication", true
	case domain.CodeTurnstileVerificationFailed:
		return "human_verification", true
	case domain.CodeCSRFTokenInvalid:
		return "request_integrity", true
	case domain.CodeCapabilityRequired:
		return "authorization", true
	case domain.CodeRateLimited:
		return "rate_limit", true
	default:
		return "", false
	}
}

func securityFailureRouteKey(r *http.Request) string {
	if r == nil {
		return "other"
	}
	route := chi.RouteContext(r.Context()).RoutePattern()
	if route == "" && r.URL != nil {
		route = r.URL.Path
	}
	switch route {
	case "/api/v1/auth/password/login":
		return "auth_password_login"
	case "/api/v1/auth/password/reauthenticate", "/api/v1/auth/password":
		return "auth_password_management"
	case "/api/v1/auth/email-registration/start", "/api/v1/auth/email-registration/confirm":
		return "auth_student_registration"
	case "/api/v1/auth/password-reset/start", "/api/v1/auth/password-reset/confirm":
		return "auth_password_reset"
	case "/api/v1/auth/oauth/start", "/api/v1/auth/oauth/callback":
		return "auth_oauth"
	}
	switch {
	case strings.HasPrefix(route, "/api/v1/account-appeal/"):
		return "account_appeal"
	case strings.HasPrefix(route, "/api/v1/admin/"):
		return "admin_api"
	case strings.HasPrefix(route, "/api/v1/owner/"):
		return "owner_api"
	case strings.HasPrefix(route, "/api/v1/me/"):
		return "member_api"
	case strings.HasPrefix(route, "/api/v1/auth/"):
		return "auth_other"
	case strings.HasPrefix(route, "/api/v1/"):
		return "public_api"
	default:
		return "other"
	}
}

func securityFailureActorKind(r *http.Request) string {
	if _, ok := middleware.SessionToken(r); ok {
		return "session"
	}
	return "anonymous"
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
