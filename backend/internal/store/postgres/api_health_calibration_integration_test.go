package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apihealth"

	"github.com/google/uuid"
)

func TestPostgresAPIProbeCalibrationEmptyDatasetIsZero(t *testing.T) {
	store := connectLifecycleTestStore(t)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	model := "calibration-empty-" + uuid.NewString()

	calibration, appErr := store.LoadProbeCalibration(
		context.Background(),
		model,
		apihealth.ProtocolResponsesV1,
		apihealth.ProbeEnvironmentUSWestV1,
		now,
	)
	if appErr != nil {
		t.Fatalf("load empty calibration: %v", appErr)
	}
	expectedBoundary := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	if !calibration.ObservationStartedAt.Equal(expectedBoundary) ||
		!calibration.ObservationEndedAt.Equal(expectedBoundary) ||
		calibration.CompleteCalendarDays != 0 || calibration.ConnectionCount != 0 ||
		calibration.SampleCount != 0 || calibration.Ready || calibration.P50TTFTMS != nil ||
		calibration.P90TTFTMS != nil || calibration.P95TTFTMS != nil || calibration.P99TTFTMS != nil {
		t.Fatalf("unexpected empty calibration: %+v", calibration)
	}
}

func TestPostgresAPIProbeCalibrationUsesOnlyCompleteUTCDaysAndPublishesImmutableVersions(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	ownerID := seedGrowthUser(t, ctx, store.pool, "probe-calibration-owner", now)
	adminID := seedGrowthUser(t, ctx, store.pool, "probe-calibration-admin", now)
	connectionIDs := make([]string, 0, 5)
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_latency_rules WHERE published_by_admin_id = $1`, adminID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, ownerID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, ownerID, adminID)
	})

	for index := 0; index < 5; index++ {
		connectionID := uuid.NewString()
		connectionIDs = append(connectionIDs, connectionID)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_probe_connections (
			  id, owner_user_id, name, base_url, normalized_base_url, enabled,
			  verification_status, probe_model, probe_protocol, probe_environment,
			  measurement_version, version, created_at, updated_at
			) VALUES ($1, $2, $3, 'https://api.example.test/v1', 'https://api.example.test/v1', false,
			          'unverified', $4, $5, $6, 1, 1, $7, $7)
		`, connectionID, ownerID, fmt.Sprintf("calibration-%d", index), apihealth.DefaultGPTProbeModel,
			apihealth.ProtocolResponsesV1, apihealth.ProbeEnvironmentUSWestV1, now.Add(-10*24*time.Hour)); err != nil {
			t.Fatalf("insert calibration connection %d: %v", index, err)
		}
		ttft := (index + 1) * 100
		insertCalibrationSample(t, store, connectionID, time.Date(2026, 8, 1, 0, index*5, 0, 0, time.UTC), ttft)
	}
	insertCalibrationSample(t, store, connectionIDs[0], time.Date(2026, 7, 31, 23, 55, 0, 0, time.UTC), 9000)

	calibration, appErr := store.LoadProbeCalibration(ctx, apihealth.DefaultGPTProbeModel, apihealth.ProtocolResponsesV1, apihealth.ProbeEnvironmentUSWestV1, now)
	if appErr != nil {
		t.Fatalf("load calibration: %v", appErr)
	}
	expectedStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !calibration.ObservationStartedAt.Equal(expectedStart) || calibration.CompleteCalendarDays != 7 ||
		calibration.ConnectionCount != 5 || calibration.SampleCount != 5 || !calibration.Ready ||
		calibration.P50TTFTMS == nil || *calibration.P50TTFTMS != 300 ||
		calibration.P90TTFTMS == nil || *calibration.P90TTFTMS != 500 {
		t.Fatalf("unexpected complete-day calibration: %+v", calibration)
	}

	preview, appErr := store.PreviewProbeLatencyRule(ctx, calibration, 250, 450)
	if appErr != nil || preview.SlowSampleCount != 2 || preview.SlowPercent != "40.0" ||
		preview.OverTimeoutCount != 1 || preview.OverTimeoutPercent != "20.0" {
		t.Fatalf("unexpected calibration preview: %+v error=%v", preview, appErr)
	}

	first, appErr := store.PublishProbeLatencyRule(ctx, apihealth.LatencyRule{
		Model: apihealth.DefaultGPTProbeModel, Protocol: apihealth.ProtocolResponsesV1,
		Environment: apihealth.ProbeEnvironmentUSWestV1, SlowTTFTMS: 250, HardTimeoutMS: 450,
		PublishedByAdminID: adminID, PublishedAt: now,
	})
	if appErr != nil || first.Version != 1 || first.Status != "active" || first.SampleCount != 5 {
		t.Fatalf("publish first latency rule: %+v error=%v", first, appErr)
	}
	second, appErr := store.PublishProbeLatencyRule(ctx, apihealth.LatencyRule{
		Model: apihealth.DefaultGPTProbeModel, Protocol: apihealth.ProtocolResponsesV1,
		Environment: apihealth.ProbeEnvironmentUSWestV1, SlowTTFTMS: 300, HardTimeoutMS: 500,
		PublishedByAdminID: adminID, PublishedAt: now.Add(time.Second),
	})
	if appErr != nil || second.Version != 2 || second.Status != "active" {
		t.Fatalf("publish replacement latency rule: %+v error=%v", second, appErr)
	}
	rules, appErr := store.ListProbeLatencyRules(ctx)
	if appErr != nil {
		t.Fatalf("list latency rules: %v", appErr)
	}
	var active, superseded int
	for _, rule := range rules {
		if rule.Model != apihealth.DefaultGPTProbeModel || rule.Protocol != apihealth.ProtocolResponsesV1 || rule.Environment != apihealth.ProbeEnvironmentUSWestV1 {
			continue
		}
		switch rule.Status {
		case "active":
			active++
		case "superseded":
			superseded++
		}
	}
	if active != 1 || superseded != 1 {
		t.Fatalf("unexpected immutable rule statuses: active=%d superseded=%d", active, superseded)
	}
}

func insertCalibrationSample(t *testing.T, store *Store, connectionID string, slot time.Time, ttft int) {
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO api_probe_connection_samples (
		  connection_id, measurement_version, slot_started_at, status, total_duration_ms,
		  started_at, finished_at, probe_model, probe_protocol, probe_environment,
		  outcome, attempt_count, first_attempt_ttft_ms, first_attempt_total_duration_ms
		) VALUES ($1, 1, $2, 'succeeded', $3, $2, $4, $5, $6, $7, 'first_success', 1, $3, $3)
	`, connectionID, slot, ttft, slot.Add(time.Duration(ttft)*time.Millisecond),
		apihealth.DefaultGPTProbeModel, apihealth.ProtocolResponsesV1, apihealth.ProbeEnvironmentUSWestV1); err != nil {
		t.Fatalf("insert calibration sample at %s: %v", slot, err)
	}
}
