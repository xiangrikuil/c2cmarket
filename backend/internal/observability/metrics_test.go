package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
)

type metricsDatabaseSource struct{}

func (metricsDatabaseSource) DatabasePoolStats() database.PoolStats {
	return database.PoolStats{
		AcquireCount:    7,
		AcquiredConns:   2,
		IdleConns:       3,
		MaxConns:        20,
		NewConnsCount:   5,
		AcquireDuration: 2 * time.Second,
	}
}

func (metricsDatabaseSource) ContactCryptoStats() postgres.ContactCryptoStats {
	return postgres.ContactCryptoStats{
		DecryptSuccessTotal: 4,
		DecryptFailureTotal: 2,
		UnknownKeyTotal:     1,
	}
}

func (metricsDatabaseSource) Readiness(context.Context) health.Status {
	version := int64(64)
	dirty := false
	return health.Status{
		Configured:            true,
		OK:                    true,
		SchemaVersion:         &version,
		SchemaDirty:           &dirty,
		ExpectedSchemaVersion: 64,
	}
}

func (metricsDatabaseSource) SlowActiveQueryCount(context.Context, time.Duration) (int64, error) {
	return 3, nil
}

type metricsRateLimiterSource struct{}

func (metricsRateLimiterSource) Stats() middleware.RateLimiterStats {
	return middleware.RateLimiterStats{
		MaxKeys:              100,
		ActiveKeys:           8,
		AllowedTotal:         12,
		LimitedTotal:         4,
		CapacityLimitedTotal: 1,
		ExpiredTotal:         2,
	}
}

type metricsMaintenanceSource struct{}

func (metricsMaintenanceSource) Stats() maintenance.Stats {
	return maintenance.Stats{
		SuccessTotal:        2,
		FailureTotal:        1,
		SkippedTotal:        3,
		LastDurationSeconds: 0.25,
	}
}

type metricsAPIHealthRunnerSource struct{}

func (metricsAPIHealthRunnerSource) Stats() apihealthrunner.Stats {
	return apihealthrunner.Stats{
		RunSuccessTotal: 3, RunFailureTotal: 1, ProbeSuccessTotal: 11,
		ProbeFailureTotal: 2, Inflight: 1, LastDurationSeconds: 0.75,
		LastSuccessAt: time.Unix(1_785_817_600, 0).UTC(),
	}
}

type metricsOutboundSource struct{}

func (metricsOutboundSource) Stats() outboundhttp.PolicyStats {
	return outboundhttp.PolicyStats{
		InvalidTargetTotal:    1,
		HostNotAllowedTotal:   2,
		ResolutionFailedTotal: 3,
		UnsafeAddressTotal:    4,
		RedirectRejectedTotal: 5,
	}
}

type metricsRealtimeHubSource struct{}

func (metricsRealtimeHubSource) Stats() realtime.HubStats {
	return realtime.HubStats{ActiveConnections: 2, ConnectionsTotal: 7, DisconnectsTotal: 5}
}

type metricsRealtimeListenerSource struct{}

func (metricsRealtimeListenerSource) Stats() realtime.ListenerStats {
	return realtime.ListenerStats{FailuresTotal: 2, InvalidPayloadTotal: 1, CloseFailuresTotal: 1}
}

func TestMetricsExposeBoundedRuntimeSnapshots(t *testing.T) {
	metrics := New(Sources{
		Database:         metricsDatabaseSource{},
		RateLimiter:      metricsRateLimiterSource{},
		Maintenance:      metricsMaintenanceSource{},
		APIHealthRunner:  metricsAPIHealthRunnerSource{},
		OutboundPolicy:   metricsOutboundSource{},
		RealtimeHub:      metricsRealtimeHubSource{},
		RealtimeListener: metricsRealtimeListenerSource{},
		SlowQueryAfter:   1500 * time.Millisecond,
	})
	response := httptest.NewRecorder()

	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{
		`c2c_market_database_pool_connections{state="acquired"} 2`,
		`c2c_market_database_slow_active_queries 3`,
		`c2c_market_database_migration_version{kind="current"} 64`,
		`c2c_market_rate_limiter_decisions_total{decision="limited"} 4`,
		`c2c_market_contact_decrypt_total{result="unknown_key"} 1`,
		`c2c_market_outbound_rejections_total{reason="unsafe_address"} 4`,
		`c2c_market_maintenance_runs_total{result="skipped"} 3`,
		`c2c_market_api_health_runs_total{result="success"} 3`,
		`c2c_market_api_health_probes_total{result="failure"} 2`,
		`c2c_market_api_health_inflight 1`,
		`c2c_market_realtime_active_connections 2`,
		`c2c_market_realtime_listener_failures_total{reason="invalid_payload"} 1`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("metrics output is missing %q", expected)
		}
	}
}

func TestRecordProblemEmitsBoundedRedactedSecurityFailureTelemetryOnce(t *testing.T) {
	var failureLogs bytes.Buffer
	metrics := New(Sources{FailureLogger: log.New(&failureLogs, "", 0)})
	router := chi.NewRouter()
	router.Use(metrics.Instrument)

	tests := []struct {
		method      string
		pattern     string
		path        string
		requestID   string
		withSession bool
		err         *domain.AppError
		metric      string
	}{
		{
			method: http.MethodPost, pattern: "/api/v1/auth/password/login", path: "/api/v1/auth/password/login", requestID: "req-invalid-credentials",
			err:    domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "secret-user@example.test must not be logged"),
			metric: `c2c_market_operations_security_failures_total{category="authentication",result="INVALID_CREDENTIALS",route="auth_password_login"} 1`,
		},
		{
			method: http.MethodPost, pattern: "/api/v1/auth/email-registration/start", path: "/api/v1/auth/email-registration/start", requestID: "req-turnstile",
			err:    domain.NewError(http.StatusForbidden, domain.CodeTurnstileVerificationFailed, "Turnstile failed", "provider-detail-secret"),
			metric: `c2c_market_operations_security_failures_total{category="human_verification",result="TURNSTILE_VERIFICATION_FAILED",route="auth_student_registration"} 1`,
		},
		{
			method: http.MethodPatch, pattern: "/api/v1/owner/api-services/{id}", path: "/api/v1/owner/api-services/secret-resource-id", requestID: "req-csrf", withSession: true,
			err:    domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF invalid", "secret-csrf-token"),
			metric: `c2c_market_operations_security_failures_total{category="request_integrity",result="CSRF_TOKEN_INVALID",route="owner_api"} 1`,
		},
		{
			method: http.MethodGet, pattern: "/api/v1/admin/audit-logs", path: "/api/v1/admin/audit-logs", requestID: "req-capability", withSession: true,
			err:    domain.NewError(http.StatusForbidden, domain.CodeCapabilityRequired, "Capability required", "api_probe.manage"),
			metric: `c2c_market_operations_security_failures_total{category="authorization",result="CAPABILITY_REQUIRED",route="admin_api"} 1`,
		},
		{
			method: http.MethodPost, pattern: "/api/v1/api-services/{id}/purchase-intents", path: "/api/v1/api-services/secret-service-id/purchase-intents", requestID: "req-rate-limit",
			err:    domain.NewError(http.StatusTooManyRequests, domain.CodeRateLimited, "Rate limited", "secret-rate-key"),
			metric: `c2c_market_operations_security_failures_total{category="rate_limit",result="RATE_LIMITED",route="public_api"} 1`,
		},
	}
	for _, test := range tests {
		test := test
		router.Method(test.method, test.pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			RecordProblem(r, test.err)
			RecordProblem(r, test.err)
			w.WriteHeader(test.err.Status)
		}))
	}
	handler := middleware.WithRequestID(router)
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"username":"secret-user","email":"secret-user@example.test","password":"secret-password","turnstileToken":"secret-turnstile-token"}`))
		request.Header.Set(middleware.RequestIDHeader, test.requestID)
		request.Header.Set("Authorization", "Bearer secret-authorization-token")
		if test.withSession {
			request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "secret-session-cookie"})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.err.Status {
			t.Fatalf("%s status=%d body=%s", test.requestID, response.Code, response.Body.String())
		}
	}

	metricsResponse := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, test := range tests {
		if !strings.Contains(metricsResponse.Body.String(), test.metric) {
			t.Errorf("metrics output is missing %q", test.metric)
		}
	}

	lines := strings.Split(strings.TrimSpace(failureLogs.String()), "\n")
	if len(lines) != len(tests) {
		t.Fatalf("duplicate or missing failure logs: got %d lines\n%s", len(lines), failureLogs.String())
	}
	allowedKeys := map[string]struct{}{
		"request_id": {}, "route_key": {}, "result_code": {}, "status": {}, "actor_kind": {},
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failure log is not structured JSON: %q: %v", line, err)
		}
		if len(entry) != len(allowedKeys) {
			t.Fatalf("failure log contains unexpected fields: %+v", entry)
		}
		for key := range entry {
			if _, ok := allowedKeys[key]; !ok {
				t.Fatalf("failure log contains forbidden field %q: %+v", key, entry)
			}
		}
	}
	for _, forbidden := range []string{
		"secret-user", "secret-user@example.test", "secret-password", "secret-turnstile-token",
		"secret-authorization-token", "secret-session-cookie", "provider-detail-secret", "secret-resource-id",
		"secret-service-id", "secret-csrf-token", "api_probe.manage", "secret-rate-key",
	} {
		if strings.Contains(failureLogs.String(), forbidden) || strings.Contains(metricsResponse.Body.String(), forbidden) {
			t.Fatalf("failure telemetry leaked sensitive value %q", forbidden)
		}
	}
}

func TestSecurityFailureRouteKeyNeverUsesRawResourcePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/api/v1/admin/users/private-user-id/status", want: "admin_api"},
		{path: "/api/v1/owner/api-services/private-service-id", want: "owner_api"},
		{path: "/api/v1/me/disputes/private-dispute-id", want: "member_api"},
		{path: "/api/v1/api-services/private-service-id", want: "public_api"},
		{path: "/outside/private-value", want: "other"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		if got := securityFailureRouteKey(request); got != test.want {
			t.Errorf("securityFailureRouteKey(%q)=%q want %q", test.path, got, test.want)
		}
		if strings.Contains(securityFailureRouteKey(request), "private") {
			t.Fatalf("route key leaked raw path: %q", securityFailureRouteKey(request))
		}
	}
}
