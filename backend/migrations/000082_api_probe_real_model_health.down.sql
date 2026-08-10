-- Restore the reusable /models-only probe schema. Real probe history cannot be downgraded.
-- Date: 2026-08-08
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_probe_connection_attempts)
    OR EXISTS (SELECT 1 FROM api_probe_connection_model_changes)
    OR EXISTS (SELECT 1 FROM api_probe_latency_rules) THEN
    RAISE EXCEPTION 'cannot roll back migration 82 while real probe attempts, model history, or latency rules exist';
  END IF;
END $$;

DROP TABLE api_probe_connection_model_changes;
DROP TABLE api_probe_connection_attempts;

DROP INDEX ix_api_probe_connection_samples_calibration;

ALTER TABLE api_probe_connection_samples
DROP CONSTRAINT ck_api_probe_connection_samples_real_outcome,
DROP COLUMN retry_cost_usd,
DROP COLUMN base_cost_usd,
DROP COLUMN usage_complete,
DROP COLUMN reasoning_tokens,
DROP COLUMN output_tokens,
DROP COLUMN cached_input_tokens,
DROP COLUMN input_tokens,
DROP COLUMN final_http_status,
DROP COLUMN recovery_duration_ms,
DROP COLUMN first_attempt_total_duration_ms,
DROP COLUMN first_attempt_ttft_ms,
DROP COLUMN attempt_count,
DROP COLUMN outcome,
DROP COLUMN latency_rule_version_id,
DROP COLUMN probe_environment,
DROP COLUMN probe_protocol,
DROP COLUMN probe_model;

DROP TABLE api_probe_latency_rules;

ALTER TABLE api_probe_connections
DROP CONSTRAINT ck_api_probe_connections_real_probe_ready,
DROP COLUMN probe_price_currency,
DROP COLUMN probe_output_price_per_million,
DROP COLUMN probe_cached_input_price_per_million,
DROP COLUMN probe_input_price_per_million,
DROP COLUMN probe_price_version_id,
DROP COLUMN probe_model_changed_at,
DROP COLUMN probe_environment,
DROP COLUMN probe_models_snapshot,
DROP COLUMN probe_protocol,
DROP COLUMN probe_model;
