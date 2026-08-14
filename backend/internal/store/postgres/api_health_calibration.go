package postgres

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ apihealth.CalibrationRepository = (*Store)(nil)

func (store *Store) LoadProbeCalibration(ctx context.Context, model, protocol, environment string, now time.Time) (apihealth.Calibration, *domain.AppError) {
	if store == nil || store.pool == nil {
		return apihealth.Calibration{}, internalStoreError()
	}
	return loadProbeCalibration(ctx, store.pool, model, protocol, environment, now)
}

type calibrationQueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadProbeCalibration(ctx context.Context, query calibrationQueryRower, model, protocol, environment string, now time.Time) (apihealth.Calibration, *domain.AppError) {
	calibration := apihealth.Calibration{Model: model, Protocol: protocol, Environment: environment}
	err := query.QueryRow(ctx, `
		WITH bounds AS (
		  SELECT date_trunc('day', $4::timestamptz AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS today
		), first_sample AS (
		  SELECT min(sample.slot_started_at) AS first_slot
		  FROM api_probe_connection_samples sample, bounds
		  WHERE sample.probe_model = $1 AND sample.probe_protocol = $2 AND sample.probe_environment = $3
		    AND sample.outcome IN ('first_success', 'first_success_slow')
		    AND sample.first_attempt_ttft_ms IS NOT NULL AND sample.slot_started_at < bounds.today
		), calibration_bounds AS (
		  SELECT bounds.today,
		         CASE
		           WHEN first_sample.first_slot IS NULL THEN bounds.today
		           WHEN first_sample.first_slot = date_trunc('day', first_sample.first_slot AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
		             THEN first_sample.first_slot
		           ELSE (date_trunc('day', first_sample.first_slot AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') + interval '1 day'
		         END AS observation_start
		  FROM bounds CROSS JOIN first_sample
		), eligible AS (
		  SELECT sample.connection_id, sample.first_attempt_ttft_ms
		  FROM api_probe_connection_samples sample, calibration_bounds
		  WHERE sample.probe_model = $1 AND sample.probe_protocol = $2 AND sample.probe_environment = $3
		    AND sample.outcome IN ('first_success', 'first_success_slow')
		    AND sample.first_attempt_ttft_ms IS NOT NULL
		    AND sample.slot_started_at >= calibration_bounds.observation_start
		    AND sample.slot_started_at < calibration_bounds.today
		)
		SELECT calibration_bounds.observation_start, calibration_bounds.today,
		       GREATEST(0, floor(extract(epoch FROM (calibration_bounds.today - calibration_bounds.observation_start)) / 86400))::int,
		       count(DISTINCT connection_id)::int, count(eligible.first_attempt_ttft_ms)::bigint,
		       percentile_disc(0.50) WITHIN GROUP (ORDER BY first_attempt_ttft_ms)::int,
		       percentile_disc(0.90) WITHIN GROUP (ORDER BY first_attempt_ttft_ms)::int,
		       percentile_disc(0.95) WITHIN GROUP (ORDER BY first_attempt_ttft_ms)::int,
		       percentile_disc(0.99) WITHIN GROUP (ORDER BY first_attempt_ttft_ms)::int
		FROM calibration_bounds LEFT JOIN eligible ON true
		GROUP BY calibration_bounds.observation_start, calibration_bounds.today
	`, model, protocol, environment, now.UTC()).Scan(
		&calibration.ObservationStartedAt, &calibration.ObservationEndedAt, &calibration.CompleteCalendarDays,
		&calibration.ConnectionCount, &calibration.SampleCount, &calibration.P50TTFTMS,
		&calibration.P90TTFTMS, &calibration.P95TTFTMS, &calibration.P99TTFTMS,
	)
	if err != nil {
		return apihealth.Calibration{}, internalStoreError()
	}
	calibration.Ready = calibration.CompleteCalendarDays >= 7 && calibration.ConnectionCount >= 5
	return calibration, nil
}

func (store *Store) PreviewProbeLatencyRule(ctx context.Context, calibration apihealth.Calibration, slowTTFTMS, hardTimeoutMS int) (apihealth.LatencyRulePreview, *domain.AppError) {
	if slowTTFTMS <= 0 || hardTimeoutMS <= slowTTFTMS || hardTimeoutMS > 30000 {
		return apihealth.LatencyRulePreview{}, probeLatencyValidationError("阈值必须满足 0 < X < Y <= 30000 毫秒。")
	}
	preview := apihealth.LatencyRulePreview{Calibration: calibration, SlowTTFTMS: slowTTFTMS, HardTimeoutMS: hardTimeoutMS}
	var total int64
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)::bigint,
		       count(*) FILTER (WHERE first_attempt_ttft_ms > $6 AND first_attempt_ttft_ms <= $7)::bigint,
		       count(*) FILTER (WHERE first_attempt_ttft_ms > $7)::bigint
		FROM api_probe_connection_samples
		WHERE probe_model = $1 AND probe_protocol = $2 AND probe_environment = $3
		  AND slot_started_at >= $4 AND slot_started_at < $5
		  AND outcome IN ('first_success', 'first_success_slow') AND first_attempt_ttft_ms IS NOT NULL
	`, calibration.Model, calibration.Protocol, calibration.Environment,
		calibration.ObservationStartedAt, calibration.ObservationEndedAt,
		slowTTFTMS, hardTimeoutMS).Scan(&total, &preview.SlowSampleCount, &preview.OverTimeoutCount)
	if err != nil {
		return apihealth.LatencyRulePreview{}, internalStoreError()
	}
	preview.SlowPercent = calibrationPercent(preview.SlowSampleCount, total)
	preview.OverTimeoutPercent = calibrationPercent(preview.OverTimeoutCount, total)
	return preview, nil
}

func (store *Store) PublishProbeLatencyRule(ctx context.Context, rule apihealth.LatencyRule) (apihealth.LatencyRule, *domain.AppError) {
	if store == nil || store.pool == nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	if rule.SlowTTFTMS <= 0 || rule.HardTimeoutMS <= rule.SlowTTFTMS || rule.HardTimeoutMS > 30000 {
		return apihealth.LatencyRule{}, probeLatencyValidationError("阈值必须满足 0 < X < Y <= 30000 毫秒。")
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(
		  hashtextextended(concat_ws(chr(31), $1::text, $2::text, $3::text), 0)
		)
	`, rule.Model, rule.Protocol, rule.Environment); err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	calibration, appErr := loadProbeCalibration(ctx, tx, rule.Model, rule.Protocol, rule.Environment, rule.PublishedAt)
	if appErr != nil {
		return apihealth.LatencyRule{}, appErr
	}
	if !calibration.Ready {
		return apihealth.LatencyRule{}, probeLatencyValidationError("校准至少需要 7 个完整自然日和 5 个独立连接。")
	}
	rule.ObservationStartedAt = calibration.ObservationStartedAt
	rule.ObservationEndedAt = calibration.ObservationEndedAt
	rule.CompleteCalendarDays = calibration.CompleteCalendarDays
	rule.ConnectionCount = calibration.ConnectionCount
	rule.SampleCount = calibration.SampleCount
	rule.P50TTFTMS = calibration.P50TTFTMS
	rule.P90TTFTMS = calibration.P90TTFTMS
	rule.P95TTFTMS = calibration.P95TTFTMS
	rule.P99TTFTMS = calibration.P99TTFTMS
	if _, err := tx.Exec(ctx, `
		UPDATE api_probe_latency_rules SET status = 'superseded', superseded_at = $4
		WHERE model = $1 AND protocol = $2 AND environment = $3 AND status = 'active'
	`, rule.Model, rule.Protocol, rule.Environment, rule.PublishedAt); err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max(version), 0) + 1 FROM api_probe_latency_rules
		WHERE model = $1 AND protocol = $2 AND environment = $3
	`, rule.Model, rule.Protocol, rule.Environment).Scan(&rule.Version); err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	rule.ID = uuid.NewString()
	rule.Status = "active"
	_, err = tx.Exec(ctx, `
		INSERT INTO api_probe_latency_rules (
		  id, model, protocol, environment, version, slow_ttft_ms, hard_timeout_ms,
		  observation_started_at, observation_ended_at, complete_calendar_days,
		  connection_count, sample_count, p50_ttft_ms, p90_ttft_ms, p95_ttft_ms, p99_ttft_ms,
		  status, published_by_admin_id, published_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 'active', $17, $18)
	`, rule.ID, rule.Model, rule.Protocol, rule.Environment, rule.Version, rule.SlowTTFTMS, rule.HardTimeoutMS,
		rule.ObservationStartedAt, rule.ObservationEndedAt, rule.CompleteCalendarDays,
		rule.ConnectionCount, rule.SampleCount, rule.P50TTFTMS, rule.P90TTFTMS, rule.P95TTFTMS, rule.P99TTFTMS,
		rule.PublishedByAdminID, rule.PublishedAt)
	if err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.LatencyRule{}, internalStoreError()
	}
	return rule, nil
}

func (store *Store) ListProbeLatencyRules(ctx context.Context) ([]apihealth.LatencyRule, *domain.AppError) {
	if store == nil || store.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, model, protocol, environment, version, slow_ttft_ms, hard_timeout_ms,
		       observation_started_at, observation_ended_at, complete_calendar_days,
		       connection_count, sample_count, p50_ttft_ms, p90_ttft_ms, p95_ttft_ms, p99_ttft_ms,
		       status, published_by_admin_id::text, published_at, superseded_at
		FROM api_probe_latency_rules ORDER BY model, protocol, environment, version DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	result := make([]apihealth.LatencyRule, 0)
	for rows.Next() {
		var rule apihealth.LatencyRule
		if err := rows.Scan(&rule.ID, &rule.Model, &rule.Protocol, &rule.Environment, &rule.Version,
			&rule.SlowTTFTMS, &rule.HardTimeoutMS, &rule.ObservationStartedAt, &rule.ObservationEndedAt,
			&rule.CompleteCalendarDays, &rule.ConnectionCount, &rule.SampleCount, &rule.P50TTFTMS,
			&rule.P90TTFTMS, &rule.P95TTFTMS, &rule.P99TTFTMS, &rule.Status,
			&rule.PublishedByAdminID, &rule.PublishedAt, &rule.SupersededAt); err != nil {
			return nil, internalStoreError()
		}
		result = append(result, rule)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func probeLatencyValidationError(detail string) *domain.AppError {
	return domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe latency rule invalid", detail)
}

func calibrationPercent(numerator, denominator int64) string {
	if denominator <= 0 {
		return "0.0"
	}
	tenths := (numerator*1000 + denominator/2) / denominator
	return strconv.FormatInt(tenths/10, 10) + "." + strconv.FormatInt(tenths%10, 10)
}
