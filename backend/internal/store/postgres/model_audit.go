package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/modelaudit"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateModelAuditTarget(ctx context.Context, target modelaudit.Target, apiKey string) (modelaudit.Target, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return modelaudit.Target{}, internalStoreError()
	}
	encoded, err := s.contactCodec.encode(apiKey)
	if err != nil {
		return modelaudit.Target{}, internalStoreError()
	}
	if target.ID == "" {
		target.ID = uuid.NewString()
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO model_audit_targets (
		  id, name, base_url, provider_type, api_key_ciphertext, api_key_nonce,
		  api_key_fingerprint, api_key_key_version, claimed_model, enabled,
		  api_service_id, api_service_model_id, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, name, base_url, provider_type, claimed_model, enabled,
		  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at
	`, target.ID, target.Name, target.BaseURL, target.ProviderType, encoded.Ciphertext, encoded.Nonce, encoded.Fingerprint,
		encoded.EncryptionKeyVersion, target.ClaimedModel, target.Enabled, nullUUID(target.APIServiceID), nullUUID(target.APIServiceModelID),
		target.CreatedAt, target.UpdatedAt)
	target, err = scanModelAuditTarget(row)
	if err != nil {
		return modelaudit.Target{}, internalStoreError()
	}
	return target, nil
}

func (s *Store) ListModelAuditTargets(ctx context.Context) ([]modelaudit.Target, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, base_url, provider_type, claimed_model, enabled,
		  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at
		FROM model_audit_targets
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	targets := []modelaudit.Target{}
	for rows.Next() {
		target, err := scanModelAuditTarget(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return targets, nil
}

func (s *Store) GetModelAuditTarget(ctx context.Context, targetID string) (modelaudit.Target, *domain.AppError) {
	target, err := scanModelAuditTarget(s.pool.QueryRow(ctx, `
		SELECT id, name, base_url, provider_type, claimed_model, enabled,
		  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at
		FROM model_audit_targets
		WHERE id = $1
	`, targetID))
	if errors.Is(err, pgx.ErrNoRows) {
		return modelaudit.Target{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit target not found", "审计目标不存在。")
	}
	if err != nil {
		return modelaudit.Target{}, internalStoreError()
	}
	return target, nil
}

func (s *Store) GetModelAuditTargetSecret(ctx context.Context, targetID string) (modelaudit.Target, string, *domain.AppError) {
	var ciphertext, nonce []byte
	target, err := scanModelAuditTargetSecret(s.pool.QueryRow(ctx, `
		SELECT id, name, base_url, provider_type, claimed_model, enabled,
		  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at,
		  api_key_ciphertext, api_key_nonce
		FROM model_audit_targets
		WHERE id = $1
	`, targetID), &ciphertext, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return modelaudit.Target{}, "", domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit target not found", "审计目标不存在。")
	}
	if err != nil {
		return modelaudit.Target{}, "", internalStoreError()
	}
	apiKey, err := s.contactCodec.decode(ciphertext, nonce)
	if err != nil {
		return modelaudit.Target{}, "", internalStoreError()
	}
	return target, apiKey, nil
}

func (s *Store) UpdateModelAuditTarget(ctx context.Context, target modelaudit.Target, apiKey *string) (modelaudit.Target, *domain.AppError) {
	if apiKey != nil {
		encoded, err := s.contactCodec.encode(*apiKey)
		if err != nil {
			return modelaudit.Target{}, internalStoreError()
		}
		row := s.pool.QueryRow(ctx, `
			UPDATE model_audit_targets
			SET name=$2, base_url=$3, provider_type=$4, api_key_ciphertext=$5, api_key_nonce=$6,
			    api_key_fingerprint=$7, api_key_key_version=$8, claimed_model=$9, enabled=$10,
			    api_service_id=$11, api_service_model_id=$12, updated_at=$13
			WHERE id=$1
			RETURNING id, name, base_url, provider_type, claimed_model, enabled,
			  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at
		`, target.ID, target.Name, target.BaseURL, target.ProviderType, encoded.Ciphertext, encoded.Nonce, encoded.Fingerprint,
			encoded.EncryptionKeyVersion, target.ClaimedModel, target.Enabled, nullUUID(target.APIServiceID), nullUUID(target.APIServiceModelID),
			target.UpdatedAt)
		target, err = scanModelAuditTarget(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return modelaudit.Target{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit target not found", "审计目标不存在。")
		}
		if err != nil {
			return modelaudit.Target{}, internalStoreError()
		}
		return target, nil
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE model_audit_targets
		SET name=$2, base_url=$3, provider_type=$4, claimed_model=$5, enabled=$6,
		    api_service_id=$7, api_service_model_id=$8, updated_at=$9
		WHERE id=$1
		RETURNING id, name, base_url, provider_type, claimed_model, enabled,
		  api_service_id, api_service_model_id, last_risk_level, last_run_id, created_at, updated_at
	`, target.ID, target.Name, target.BaseURL, target.ProviderType, target.ClaimedModel, target.Enabled,
		nullUUID(target.APIServiceID), nullUUID(target.APIServiceModelID), target.UpdatedAt)
	target, err := scanModelAuditTarget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return modelaudit.Target{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit target not found", "审计目标不存在。")
	}
	if err != nil {
		return modelaudit.Target{}, internalStoreError()
	}
	return target, nil
}

func (s *Store) DeleteModelAuditTarget(ctx context.Context, targetID string) *domain.AppError {
	tag, err := s.pool.Exec(ctx, `UPDATE model_audit_targets SET enabled=false, updated_at=now() WHERE id=$1`, targetID)
	if err != nil {
		return internalStoreError()
	}
	if tag.RowsAffected() == 0 {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit target not found", "审计目标不存在。")
	}
	return nil
}

func (s *Store) CreateModelAuditBaseline(ctx context.Context, baseline modelaudit.Baseline) (modelaudit.Baseline, *domain.AppError) {
	if baseline.ID == "" {
		baseline.ID = uuid.NewString()
	}
	params, features := jsonMapBytes(baseline.ParamsJSON), jsonMapBytes(baseline.FeatureJSON)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO model_audit_baselines (
		  id, baseline_name, source_target_id, model, source_type, probe_set_version,
		  params_json, feature_json, sample_count, valid_from, valid_to, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, baseline_name, source_target_id, model, source_type, probe_set_version,
		  params_json, feature_json, sample_count, valid_from, valid_to, created_at
	`, baseline.ID, baseline.BaselineName, nullUUID(baseline.SourceTargetID), baseline.Model, baseline.SourceType,
		baseline.ProbeSetVersion, params, features, baseline.SampleCount, baseline.ValidFrom, baseline.ValidTo, baseline.CreatedAt)
	baseline, err = scanModelAuditBaseline(row)
	if err != nil {
		return modelaudit.Baseline{}, internalStoreError()
	}
	return baseline, nil
}

func (s *Store) ListModelAuditBaselines(ctx context.Context) ([]modelaudit.Baseline, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, baseline_name, source_target_id, model, source_type, probe_set_version,
		  params_json, feature_json, sample_count, valid_from, valid_to, created_at
		FROM model_audit_baselines
		ORDER BY valid_from DESC, id DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []modelaudit.Baseline{}
	for rows.Next() {
		item, err := scanModelAuditBaseline(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) GetModelAuditBaseline(ctx context.Context, baselineID string) (modelaudit.Baseline, *domain.AppError) {
	baseline, err := scanModelAuditBaseline(s.pool.QueryRow(ctx, `
		SELECT id, baseline_name, source_target_id, model, source_type, probe_set_version,
		  params_json, feature_json, sample_count, valid_from, valid_to, created_at
		FROM model_audit_baselines
		WHERE id=$1
	`, baselineID))
	if errors.Is(err, pgx.ErrNoRows) {
		return modelaudit.Baseline{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit baseline not found", "基线不存在。")
	}
	if err != nil {
		return modelaudit.Baseline{}, internalStoreError()
	}
	return baseline, nil
}

func (s *Store) CreateModelAuditRun(ctx context.Context, run modelaudit.Run) (modelaudit.Run, *domain.AppError) {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO model_audit_runs (
		  id, target_id, claimed_model, baseline_id, status, mode, risk_level, confidence,
		  overall_score, score_json, report_json, report_markdown, error_message, started_at, finished_at, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, target_id, claimed_model, baseline_id, status, mode, risk_level, confidence,
		  overall_score, score_json, report_json, report_markdown, error_message, started_at, finished_at, created_at
	`, run.ID, run.TargetID, run.ClaimedModel, nullUUID(run.BaselineID), run.Status, run.Mode, nullText(string(run.RiskLevel)),
		run.Confidence, run.OverallScore, jsonMapBytes(run.ScoreJSON), jsonMapBytes(run.ReportJSON), nullText(run.ReportMarkdown),
		nullText(run.ErrorMessage), run.StartedAt, run.FinishedAt, run.CreatedAt)
	run, err := scanModelAuditRun(row)
	if err != nil {
		return modelaudit.Run{}, internalStoreError()
	}
	return run, nil
}

func (s *Store) ListModelAuditRuns(ctx context.Context) ([]modelaudit.Run, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.target_id, r.claimed_model, r.baseline_id, r.status, r.mode, r.risk_level, r.confidence,
		  r.overall_score, r.score_json, r.report_json, r.report_markdown, r.error_message, r.started_at, r.finished_at, r.created_at,
		  t.name
		FROM model_audit_runs r
		JOIN model_audit_targets t ON t.id = r.target_id
		ORDER BY r.created_at DESC, r.id DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	runs := []modelaudit.Run{}
	for rows.Next() {
		run, err := scanModelAuditRunWithTargetName(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (s *Store) GetModelAuditRun(ctx context.Context, runID string) (modelaudit.Run, *domain.AppError) {
	run, err := scanModelAuditRunWithTargetName(s.pool.QueryRow(ctx, `
		SELECT r.id, r.target_id, r.claimed_model, r.baseline_id, r.status, r.mode, r.risk_level, r.confidence,
		  r.overall_score, r.score_json, r.report_json, r.report_markdown, r.error_message, r.started_at, r.finished_at, r.created_at,
		  t.name
		FROM model_audit_runs r
		JOIN model_audit_targets t ON t.id = r.target_id
		WHERE r.id=$1
	`, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return modelaudit.Run{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Model audit run not found", "审计运行不存在。")
	}
	if err != nil {
		return modelaudit.Run{}, internalStoreError()
	}
	return run, nil
}

func (s *Store) UpdateModelAuditRun(ctx context.Context, run modelaudit.Run) (modelaudit.Run, *domain.AppError) {
	row := s.pool.QueryRow(ctx, `
		UPDATE model_audit_runs
		SET status=$2, risk_level=$3, confidence=$4, overall_score=$5, score_json=$6,
		    report_json=$7, report_markdown=$8, error_message=$9, started_at=$10, finished_at=$11
		WHERE id=$1
		RETURNING id, target_id, claimed_model, baseline_id, status, mode, risk_level, confidence,
		  overall_score, score_json, report_json, report_markdown, error_message, started_at, finished_at, created_at
	`, run.ID, run.Status, nullText(string(run.RiskLevel)), run.Confidence, run.OverallScore, jsonMapBytes(run.ScoreJSON),
		jsonMapBytes(run.ReportJSON), nullText(run.ReportMarkdown), nullText(run.ErrorMessage), run.StartedAt, run.FinishedAt)
	run, err := scanModelAuditRun(row)
	if err != nil {
		return modelaudit.Run{}, internalStoreError()
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE model_audit_targets
		SET last_risk_level=$2, last_run_id=$3, updated_at=now()
		WHERE id=$1
	`, run.TargetID, nullText(string(run.RiskLevel)), run.ID)
	return run, nil
}

func (s *Store) CancelModelAuditRun(ctx context.Context, runID string) (modelaudit.Run, *domain.AppError) {
	run, err := scanModelAuditRun(s.pool.QueryRow(ctx, `
		UPDATE model_audit_runs
		SET status='cancelled', finished_at=COALESCE(finished_at, now())
		WHERE id=$1 AND status IN ('queued', 'running')
		RETURNING id, target_id, claimed_model, baseline_id, status, mode, risk_level, confidence,
		  overall_score, score_json, report_json, report_markdown, error_message, started_at, finished_at, created_at
	`, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.GetModelAuditRun(ctx, runID)
	}
	if err != nil {
		return modelaudit.Run{}, internalStoreError()
	}
	return run, nil
}

func (s *Store) CreateModelAuditSample(ctx context.Context, sample modelaudit.Sample) (modelaudit.Sample, *domain.AppError) {
	if sample.ID == "" {
		sample.ID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO model_audit_samples (
		  id, run_id, target_id, probe_type, prompt_id, prompt_hash, prompt_text, response_text,
		  response_hash, parsed_value, raw_json, request_params_json, latency_ms, first_token_latency_ms,
		  usage_prompt_tokens, usage_completion_tokens, estimated_prompt_tokens, estimated_completion_tokens,
		  error_message, session_id, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
	`, sample.ID, sample.RunID, sample.TargetID, sample.ProbeType, sample.PromptID, sample.PromptHash, nullText(sample.PromptText),
		nullText(sample.ResponseText), nullText(sample.ResponseHash), nullText(sample.ParsedValue), jsonMapBytes(sample.RawJSON),
		jsonMapBytes(sample.RequestParamsJSON), nullInt(sample.LatencyMS), nullInt(sample.FirstTokenLatencyMS), nullInt(sample.UsagePromptTokens),
		nullInt(sample.UsageCompletionTokens), nullInt(sample.EstimatedPromptTokens), nullInt(sample.EstimatedCompletionTokens),
		nullText(sample.ErrorMessage), nullText(sample.SessionID), sample.CreatedAt)
	if err != nil {
		return modelaudit.Sample{}, internalStoreError()
	}
	return sample, nil
}

func (s *Store) ListModelAuditSamples(ctx context.Context, runID string) ([]modelaudit.Sample, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, target_id, probe_type, prompt_id, prompt_hash, prompt_text, response_text,
		  response_hash, parsed_value, raw_json, request_params_json, latency_ms, first_token_latency_ms,
		  usage_prompt_tokens, usage_completion_tokens, estimated_prompt_tokens, estimated_completion_tokens,
		  error_message, session_id, created_at
		FROM model_audit_samples
		WHERE run_id=$1
		ORDER BY created_at, id
	`, runID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	samples := []modelaudit.Sample{}
	for rows.Next() {
		sample, err := scanModelAuditSample(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return samples, nil
}

func (s *Store) CreateModelAuditProbeScore(ctx context.Context, runID string, score modelaudit.ProbeScore) *domain.AppError {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO model_audit_probe_scores (run_id, probe, risk, confidence, score, evidence_json)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, runID, score.Probe, score.Risk, score.Confidence, score.Score, jsonMapBytes(score.Evidence))
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListModelAuditProbeScores(ctx context.Context, runID string) ([]modelaudit.ProbeScore, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT probe, risk, confidence, score, evidence_json
		FROM model_audit_probe_scores
		WHERE run_id=$1
		ORDER BY created_at, id
	`, runID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	scores := []modelaudit.ProbeScore{}
	for rows.Next() {
		var score modelaudit.ProbeScore
		var evidence []byte
		if err := rows.Scan(&score.Probe, &score.Risk, &score.Confidence, &score.Score, &evidence); err != nil {
			return nil, internalStoreError()
		}
		score.Evidence = unmarshalMap(evidence)
		scores = append(scores, score)
	}
	return scores, nil
}

func (s *Store) CreateModelAuditMonitor(ctx context.Context, monitor modelaudit.Monitor) (modelaudit.Monitor, *domain.AppError) {
	if monitor.ID == "" {
		monitor.ID = uuid.NewString()
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO model_audit_scheduled_monitors (
		  id, target_id, baseline_id, mode, enabled, cron_spec, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, target_id, baseline_id, mode, enabled, cron_spec, last_run_id, last_risk, last_run_at, created_at, updated_at
	`, monitor.ID, monitor.TargetID, nullUUID(monitor.BaselineID), monitor.Mode, monitor.Enabled, nullText(monitor.CronSpec),
		monitor.CreatedAt, monitor.UpdatedAt)
	monitor, err := scanModelAuditMonitor(row)
	if err != nil {
		return modelaudit.Monitor{}, internalStoreError()
	}
	return monitor, nil
}

func (s *Store) ListModelAuditMonitors(ctx context.Context) ([]modelaudit.Monitor, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, target_id, baseline_id, mode, enabled, cron_spec, last_run_id, last_risk, last_run_at, created_at, updated_at
		FROM model_audit_scheduled_monitors
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	monitors := []modelaudit.Monitor{}
	for rows.Next() {
		monitor, err := scanModelAuditMonitor(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		monitors = append(monitors, monitor)
	}
	return monitors, nil
}

func scanModelAuditTarget(row scanner) (modelaudit.Target, error) {
	var target modelaudit.Target
	var apiServiceID, apiServiceModelID, lastRisk, lastRunID sql.NullString
	err := row.Scan(
		&target.ID, &target.Name, &target.BaseURL, &target.ProviderType, &target.ClaimedModel, &target.Enabled,
		&apiServiceID, &apiServiceModelID, &lastRisk, &lastRunID, &target.CreatedAt, &target.UpdatedAt,
	)
	target.APIServiceID = applyNullString(apiServiceID)
	target.APIServiceModelID = applyNullString(apiServiceModelID)
	target.LastRiskLevel = modelaudit.RiskLevel(applyNullString(lastRisk))
	target.LastRunID = applyNullString(lastRunID)
	return target, err
}

func scanModelAuditTargetSecret(row scanner, ciphertext, nonce *[]byte) (modelaudit.Target, error) {
	var target modelaudit.Target
	var apiServiceID, apiServiceModelID, lastRisk, lastRunID sql.NullString
	err := row.Scan(
		&target.ID, &target.Name, &target.BaseURL, &target.ProviderType, &target.ClaimedModel, &target.Enabled,
		&apiServiceID, &apiServiceModelID, &lastRisk, &lastRunID, &target.CreatedAt, &target.UpdatedAt,
		ciphertext, nonce,
	)
	target.APIServiceID = applyNullString(apiServiceID)
	target.APIServiceModelID = applyNullString(apiServiceModelID)
	target.LastRiskLevel = modelaudit.RiskLevel(applyNullString(lastRisk))
	target.LastRunID = applyNullString(lastRunID)
	return target, err
}

func scanModelAuditBaseline(row scanner) (modelaudit.Baseline, error) {
	var baseline modelaudit.Baseline
	var sourceTargetID sql.NullString
	var validTo sql.NullTime
	var params, features []byte
	err := row.Scan(
		&baseline.ID, &baseline.BaselineName, &sourceTargetID, &baseline.Model, &baseline.SourceType, &baseline.ProbeSetVersion,
		&params, &features, &baseline.SampleCount, &baseline.ValidFrom, &validTo, &baseline.CreatedAt,
	)
	baseline.SourceTargetID = applyNullString(sourceTargetID)
	baseline.ValidTo = applyNullTime(validTo)
	baseline.ParamsJSON = unmarshalMap(params)
	baseline.FeatureJSON = unmarshalMap(features)
	return baseline, err
}

func scanModelAuditRun(row scanner) (modelaudit.Run, error) {
	run, err := scanModelAuditRunCore(row, nil)
	return run, err
}

func scanModelAuditRunWithTargetName(row scanner) (modelaudit.Run, error) {
	var targetName string
	return scanModelAuditRunCore(row, &targetName)
}

func scanModelAuditRunCore(row scanner, targetName *string) (modelaudit.Run, error) {
	var run modelaudit.Run
	var baselineID, riskLevel, reportMarkdown, errorMessage sql.NullString
	var scoreJSON, reportJSON []byte
	var startedAt, finishedAt sql.NullTime
	dest := []any{
		&run.ID, &run.TargetID, &run.ClaimedModel, &baselineID, &run.Status, &run.Mode, &riskLevel, &run.Confidence,
		&run.OverallScore, &scoreJSON, &reportJSON, &reportMarkdown, &errorMessage, &startedAt, &finishedAt, &run.CreatedAt,
	}
	if targetName != nil {
		dest = append(dest, targetName)
	}
	err := row.Scan(dest...)
	run.BaselineID = applyNullString(baselineID)
	run.RiskLevel = modelaudit.RiskLevel(applyNullString(riskLevel))
	run.ReportMarkdown = applyNullString(reportMarkdown)
	run.ErrorMessage = applyNullString(errorMessage)
	run.StartedAt = applyNullTime(startedAt)
	run.FinishedAt = applyNullTime(finishedAt)
	run.ScoreJSON = unmarshalMap(scoreJSON)
	run.ReportJSON = unmarshalMap(reportJSON)
	if targetName != nil {
		run.TargetName = *targetName
	}
	return run, err
}

func scanModelAuditMonitor(row scanner) (modelaudit.Monitor, error) {
	var monitor modelaudit.Monitor
	var baselineID, cronSpec, lastRunID, lastRisk sql.NullString
	var lastRunAt sql.NullTime
	err := row.Scan(
		&monitor.ID, &monitor.TargetID, &baselineID, &monitor.Mode, &monitor.Enabled, &cronSpec, &lastRunID, &lastRisk, &lastRunAt,
		&monitor.CreatedAt, &monitor.UpdatedAt,
	)
	monitor.BaselineID = applyNullString(baselineID)
	monitor.CronSpec = applyNullString(cronSpec)
	monitor.LastRunID = applyNullString(lastRunID)
	monitor.LastRisk = modelaudit.RiskLevel(applyNullString(lastRisk))
	monitor.LastRunAt = applyNullTime(lastRunAt)
	return monitor, err
}

func scanModelAuditSample(row scanner) (modelaudit.Sample, error) {
	var sample modelaudit.Sample
	var promptText, responseText, responseHash, parsedValue, errorMessage, sessionID sql.NullString
	var rawJSON, requestParamsJSON []byte
	var latencyMS, firstTokenLatencyMS, usagePromptTokens, usageCompletionTokens, estimatedPromptTokens, estimatedCompletionTokens sql.NullInt64
	err := row.Scan(
		&sample.ID, &sample.RunID, &sample.TargetID, &sample.ProbeType, &sample.PromptID, &sample.PromptHash, &promptText, &responseText,
		&responseHash, &parsedValue, &rawJSON, &requestParamsJSON, &latencyMS, &firstTokenLatencyMS, &usagePromptTokens,
		&usageCompletionTokens, &estimatedPromptTokens, &estimatedCompletionTokens, &errorMessage, &sessionID, &sample.CreatedAt,
	)
	sample.PromptText = applyNullString(promptText)
	sample.ResponseText = applyNullString(responseText)
	sample.ResponseHash = applyNullString(responseHash)
	sample.ParsedValue = applyNullString(parsedValue)
	sample.RawJSON = unmarshalMap(rawJSON)
	sample.RequestParamsJSON = unmarshalMap(requestParamsJSON)
	sample.LatencyMS = applyNullInt(latencyMS)
	sample.FirstTokenLatencyMS = applyNullInt(firstTokenLatencyMS)
	sample.UsagePromptTokens = applyNullInt(usagePromptTokens)
	sample.UsageCompletionTokens = applyNullInt(usageCompletionTokens)
	sample.EstimatedPromptTokens = applyNullInt(estimatedPromptTokens)
	sample.EstimatedCompletionTokens = applyNullInt(estimatedCompletionTokens)
	sample.ErrorMessage = applyNullString(errorMessage)
	sample.SessionID = applyNullString(sessionID)
	return sample, err
}

func jsonMapBytes(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	body, _ := json.Marshal(value)
	return body
}

func unmarshalMap(body []byte) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return out
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func applyNullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func applyNullTime(value sql.NullTime) *time.Time {
	if value.Valid {
		return &value.Time
	}
	return nil
}

func applyNullInt(value sql.NullInt64) int {
	if value.Valid {
		return int(value.Int64)
	}
	return 0
}
