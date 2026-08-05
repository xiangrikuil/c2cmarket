package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/apihealthrunner"
	"c2c-market/backend/internal/database"
	"c2c-market/backend/internal/health"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/platform/outboundhttp"
	"c2c-market/backend/internal/realtime"
	"c2c-market/backend/internal/store/postgres"
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
