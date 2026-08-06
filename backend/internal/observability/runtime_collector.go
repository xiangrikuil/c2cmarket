package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type runtimeCollector struct {
	sources Sources
	descs   runtimeDescriptions
}

type runtimeDescriptions struct {
	poolConnections       *prometheus.Desc
	poolAcquires          *prometheus.Desc
	poolAcquireSeconds    *prometheus.Desc
	poolAcquireWait       *prometheus.Desc
	poolNewConnections    *prometheus.Desc
	poolDestroyed         *prometheus.Desc
	slowQueries           *prometheus.Desc
	databaseMetricsUp     *prometheus.Desc
	migrationVersion      *prometheus.Desc
	migrationDirty        *prometheus.Desc
	databaseReady         *prometheus.Desc
	rateLimiterKeys       *prometheus.Desc
	rateLimiterDecisions  *prometheus.Desc
	rateLimiterExpired    *prometheus.Desc
	contactDecrypt        *prometheus.Desc
	outboundRejections    *prometheus.Desc
	maintenanceRuns       *prometheus.Desc
	maintenanceDuration   *prometheus.Desc
	apiHealthRuns         *prometheus.Desc
	apiHealthProbes       *prometheus.Desc
	apiHealthInflight     *prometheus.Desc
	apiHealthDuration     *prometheus.Desc
	apiHealthLastSuccess  *prometheus.Desc
	realtimeConnections   *prometheus.Desc
	realtimeActive        *prometheus.Desc
	realtimeListenerError *prometheus.Desc
}

func newRuntimeCollector(sources Sources) prometheus.Collector {
	if sources.SlowQueryAfter <= 0 {
		sources.SlowQueryAfter = time.Second
	}
	return &runtimeCollector{
		sources: sources,
		descs: runtimeDescriptions{
			poolConnections:       desc("database_pool_connections", "PostgreSQL pool connections by state.", []string{"state"}),
			poolAcquires:          desc("database_pool_acquires_total", "PostgreSQL pool acquire outcomes.", []string{"result"}),
			poolAcquireSeconds:    desc("database_pool_acquire_seconds_total", "Cumulative PostgreSQL pool acquire duration.", nil),
			poolAcquireWait:       desc("database_pool_empty_acquire_wait_seconds_total", "Cumulative wait duration when no connection was immediately available.", nil),
			poolNewConnections:    desc("database_pool_new_connections_total", "PostgreSQL connections created by the pool.", nil),
			poolDestroyed:         desc("database_pool_destroyed_connections_total", "PostgreSQL pool connections destroyed by reason.", []string{"reason"}),
			slowQueries:           desc("database_slow_active_queries", "Active PostgreSQL queries older than the configured threshold.", nil),
			databaseMetricsUp:     desc("database_observability_up", "Whether the bounded database observability query succeeded.", nil),
			migrationVersion:      desc("database_migration_version", "Current and expected PostgreSQL migration versions.", []string{"kind"}),
			migrationDirty:        desc("database_migration_dirty", "Whether schema_migrations is dirty.", nil),
			databaseReady:         desc("database_ready", "Whether PostgreSQL and its migration are ready.", nil),
			rateLimiterKeys:       desc("rate_limiter_keys", "In-memory rate limiter keys by state.", []string{"state"}),
			rateLimiterDecisions:  desc("rate_limiter_decisions_total", "Rate limiter decisions by outcome.", []string{"decision"}),
			rateLimiterExpired:    desc("rate_limiter_expired_keys_total", "Expired rate limiter keys removed.", nil),
			contactDecrypt:        desc("contact_decrypt_total", "Contact decrypt outcomes.", []string{"result"}),
			outboundRejections:    desc("outbound_rejections_total", "Safe outbound HTTP rejections by fixed reason.", []string{"reason"}),
			maintenanceRuns:       desc("maintenance_runs_total", "Data lifecycle maintenance runs by result.", []string{"result"}),
			maintenanceDuration:   desc("maintenance_last_duration_seconds", "Duration of the latest data lifecycle maintenance run.", nil),
			apiHealthRuns:         desc("api_health_runs_total", "API health runner scans by result.", []string{"result"}),
			apiHealthProbes:       desc("api_health_probes_total", "Final API health probes by result.", []string{"result"}),
			apiHealthInflight:     desc("api_health_inflight", "API health probes currently in flight.", nil),
			apiHealthDuration:     desc("api_health_last_run_duration_seconds", "Duration of the latest API health runner scan.", nil),
			apiHealthLastSuccess:  desc("api_health_last_success_timestamp_seconds", "Unix timestamp of the latest successful API health runner scan.", nil),
			realtimeConnections:   desc("realtime_connections_total", "Realtime SSE connections and disconnects.", []string{"event"}),
			realtimeActive:        desc("realtime_active_connections", "Active realtime SSE connections.", nil),
			realtimeListenerError: desc("realtime_listener_failures_total", "Realtime PostgreSQL listener failures by reason.", []string{"reason"}),
		},
	}
}

func desc(name, help string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(namespace+"_"+name, help, labels, nil)
}

func (c *runtimeCollector) Describe(ch chan<- *prometheus.Desc) {
	values := c.descs
	for _, item := range []*prometheus.Desc{
		values.poolConnections, values.poolAcquires, values.poolAcquireSeconds,
		values.poolAcquireWait, values.poolNewConnections, values.poolDestroyed,
		values.slowQueries, values.databaseMetricsUp, values.migrationVersion,
		values.migrationDirty, values.databaseReady, values.rateLimiterKeys,
		values.rateLimiterDecisions, values.rateLimiterExpired, values.contactDecrypt,
		values.outboundRejections, values.maintenanceRuns, values.maintenanceDuration,
		values.apiHealthRuns, values.apiHealthProbes, values.apiHealthInflight,
		values.apiHealthDuration, values.apiHealthLastSuccess,
		values.realtimeConnections, values.realtimeActive, values.realtimeListenerError,
	} {
		ch <- item
	}
}

func (c *runtimeCollector) Collect(ch chan<- prometheus.Metric) {
	c.collectDatabase(ch)
	c.collectRateLimiter(ch)
	c.collectContactCrypto(ch)
	c.collectOutbound(ch)
	c.collectMaintenance(ch)
	c.collectAPIHealth(ch)
	c.collectRealtime(ch)
}

func (c *runtimeCollector) collectAPIHealth(ch chan<- prometheus.Metric) {
	if c.sources.APIHealthRunner == nil {
		return
	}
	stats := c.sources.APIHealthRunner.Stats()
	counter(ch, c.descs.apiHealthRuns, float64(stats.RunSuccessTotal), "success")
	counter(ch, c.descs.apiHealthRuns, float64(stats.RunFailureTotal), "failure")
	counter(ch, c.descs.apiHealthProbes, float64(stats.ProbeSuccessTotal), "success")
	counter(ch, c.descs.apiHealthProbes, float64(stats.ProbeFailureTotal), "failure")
	gauge(ch, c.descs.apiHealthInflight, float64(stats.Inflight))
	gauge(ch, c.descs.apiHealthDuration, stats.LastDurationSeconds)
	if !stats.LastSuccessAt.IsZero() {
		gauge(ch, c.descs.apiHealthLastSuccess, float64(stats.LastSuccessAt.Unix()))
	}
}

func (c *runtimeCollector) collectDatabase(ch chan<- prometheus.Metric) {
	if c.sources.Database == nil {
		return
	}
	stats := c.sources.Database.DatabasePoolStats()
	gauge(ch, c.descs.poolConnections, float64(stats.AcquiredConns), "acquired")
	gauge(ch, c.descs.poolConnections, float64(stats.IdleConns), "idle")
	gauge(ch, c.descs.poolConnections, float64(stats.ConstructingConns), "constructing")
	gauge(ch, c.descs.poolConnections, float64(stats.TotalConns), "total")
	gauge(ch, c.descs.poolConnections, float64(stats.MaxConns), "max")
	counter(ch, c.descs.poolAcquires, float64(stats.AcquireCount), "success")
	counter(ch, c.descs.poolAcquires, float64(stats.CanceledAcquireCount), "canceled")
	counter(ch, c.descs.poolAcquires, float64(stats.EmptyAcquireCount), "waited")
	counter(ch, c.descs.poolAcquireSeconds, stats.AcquireDuration.Seconds())
	counter(ch, c.descs.poolAcquireWait, stats.EmptyAcquireWaitTime.Seconds())
	counter(ch, c.descs.poolNewConnections, float64(stats.NewConnsCount))
	counter(ch, c.descs.poolDestroyed, float64(stats.MaxIdleDestroyCount), "idle")
	counter(ch, c.descs.poolDestroyed, float64(stats.MaxLifetimeDestroyCount), "lifetime")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	slowQueries, err := c.sources.Database.SlowActiveQueryCount(ctx, c.sources.SlowQueryAfter)
	cancel()
	if err == nil {
		gauge(ch, c.descs.slowQueries, float64(slowQueries))
		gauge(ch, c.descs.databaseMetricsUp, 1)
	} else {
		gauge(ch, c.descs.databaseMetricsUp, 0)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	readiness := c.sources.Database.Readiness(ctx)
	cancel()
	if readiness.SchemaVersion != nil {
		gauge(ch, c.descs.migrationVersion, float64(*readiness.SchemaVersion), "current")
	}
	gauge(ch, c.descs.migrationVersion, float64(readiness.ExpectedSchemaVersion), "expected")
	if readiness.SchemaDirty != nil && *readiness.SchemaDirty {
		gauge(ch, c.descs.migrationDirty, 1)
	} else {
		gauge(ch, c.descs.migrationDirty, 0)
	}
	if readiness.OK {
		gauge(ch, c.descs.databaseReady, 1)
	} else {
		gauge(ch, c.descs.databaseReady, 0)
	}
}

func (c *runtimeCollector) collectRateLimiter(ch chan<- prometheus.Metric) {
	if c.sources.RateLimiter == nil {
		return
	}
	stats := c.sources.RateLimiter.Stats()
	gauge(ch, c.descs.rateLimiterKeys, float64(stats.ActiveKeys), "active")
	gauge(ch, c.descs.rateLimiterKeys, float64(stats.MaxKeys), "max")
	counter(ch, c.descs.rateLimiterDecisions, float64(stats.AllowedTotal), "allowed")
	counter(ch, c.descs.rateLimiterDecisions, float64(stats.LimitedTotal), "limited")
	counter(ch, c.descs.rateLimiterDecisions, float64(stats.CapacityLimitedTotal), "capacity_limited")
	counter(ch, c.descs.rateLimiterExpired, float64(stats.ExpiredTotal))
}

func (c *runtimeCollector) collectContactCrypto(ch chan<- prometheus.Metric) {
	if c.sources.Database == nil {
		return
	}
	stats := c.sources.Database.ContactCryptoStats()
	counter(ch, c.descs.contactDecrypt, float64(stats.DecryptSuccessTotal), "success")
	counter(ch, c.descs.contactDecrypt, float64(stats.DecryptFailureTotal), "failure")
	counter(ch, c.descs.contactDecrypt, float64(stats.UnknownKeyTotal), "unknown_key")
}

func (c *runtimeCollector) collectOutbound(ch chan<- prometheus.Metric) {
	if c.sources.OutboundPolicy == nil {
		return
	}
	stats := c.sources.OutboundPolicy.Stats()
	counter(ch, c.descs.outboundRejections, float64(stats.InvalidTargetTotal), "invalid_target")
	counter(ch, c.descs.outboundRejections, float64(stats.HostNotAllowedTotal), "host_not_allowed")
	counter(ch, c.descs.outboundRejections, float64(stats.ResolutionFailedTotal), "resolution_failed")
	counter(ch, c.descs.outboundRejections, float64(stats.UnsafeAddressTotal), "unsafe_address")
	counter(ch, c.descs.outboundRejections, float64(stats.RedirectRejectedTotal), "redirect")
}

func (c *runtimeCollector) collectMaintenance(ch chan<- prometheus.Metric) {
	if c.sources.Maintenance == nil {
		return
	}
	stats := c.sources.Maintenance.Stats()
	counter(ch, c.descs.maintenanceRuns, float64(stats.SuccessTotal), "success")
	counter(ch, c.descs.maintenanceRuns, float64(stats.FailureTotal), "failure")
	counter(ch, c.descs.maintenanceRuns, float64(stats.SkippedTotal), "skipped")
	gauge(ch, c.descs.maintenanceDuration, stats.LastDurationSeconds)
}

func (c *runtimeCollector) collectRealtime(ch chan<- prometheus.Metric) {
	if c.sources.RealtimeHub != nil {
		stats := c.sources.RealtimeHub.Stats()
		counter(ch, c.descs.realtimeConnections, float64(stats.ConnectionsTotal), "connected")
		counter(ch, c.descs.realtimeConnections, float64(stats.DisconnectsTotal), "disconnected")
		gauge(ch, c.descs.realtimeActive, float64(stats.ActiveConnections))
	}
	if c.sources.RealtimeListener != nil {
		stats := c.sources.RealtimeListener.Stats()
		counter(ch, c.descs.realtimeListenerError, float64(stats.FailuresTotal), "connection")
		counter(ch, c.descs.realtimeListenerError, float64(stats.InvalidPayloadTotal), "invalid_payload")
		counter(ch, c.descs.realtimeListenerError, float64(stats.CloseFailuresTotal), "close")
	}
}

func gauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
}

func counter(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value, labels...)
}
