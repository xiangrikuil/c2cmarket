-- Add AI API model audit targets, baselines, runs, samples, probe scores, and monitors.
-- Date: 2026-07-07
-- Executor: Codex

CREATE TABLE IF NOT EXISTS model_audit_targets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  base_url text NOT NULL,
  provider_type text NOT NULL DEFAULT 'openai_compatible',
  api_key_ciphertext bytea NOT NULL,
  api_key_nonce bytea NOT NULL,
  api_key_fingerprint text NOT NULL,
  api_key_key_version text NOT NULL,
  claimed_model text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  api_service_id uuid NULL REFERENCES api_services(id) ON DELETE SET NULL,
  api_service_model_id uuid NULL REFERENCES api_service_models(id) ON DELETE SET NULL,
  last_risk_level text NULL CHECK (last_risk_level IN ('consistent', 'suspicious', 'high_risk', 'insufficient_data')),
  last_run_id uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (provider_type IN ('openai_compatible')),
  CHECK (trim(name) <> ''),
  CHECK (trim(base_url) <> ''),
  CHECK (trim(claimed_model) <> '')
);

CREATE INDEX IF NOT EXISTS ix_model_audit_targets_enabled
ON model_audit_targets (enabled, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS model_audit_baselines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  baseline_name text NOT NULL,
  source_target_id uuid NULL REFERENCES model_audit_targets(id) ON DELETE SET NULL,
  model text NOT NULL,
  source_type text NOT NULL,
  probe_set_version text NOT NULL,
  params_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  feature_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  sample_count integer NOT NULL DEFAULT 0,
  valid_from timestamptz NOT NULL DEFAULT now(),
  valid_to timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (source_type IN ('official_api', 'trusted_api', 'local_model', 'manual_import')),
  CHECK (trim(baseline_name) <> ''),
  CHECK (trim(model) <> ''),
  CHECK (trim(probe_set_version) <> ''),
  CHECK (sample_count >= 0)
);

CREATE INDEX IF NOT EXISTS ix_model_audit_baselines_model
ON model_audit_baselines (model, valid_from DESC, id DESC);

CREATE TABLE IF NOT EXISTS model_audit_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_id uuid NOT NULL REFERENCES model_audit_targets(id) ON DELETE RESTRICT,
  claimed_model text NOT NULL,
  baseline_id uuid NULL REFERENCES model_audit_baselines(id) ON DELETE SET NULL,
  status text NOT NULL,
  mode text NOT NULL,
  risk_level text NULL,
  confidence numeric NULL,
  overall_score numeric NULL,
  score_json jsonb NULL,
  report_json jsonb NULL,
  report_markdown text NULL,
  error_message text NULL,
  started_at timestamptz NULL,
  finished_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
  CHECK (mode IN ('quick', 'standard', 'strict', 'scheduled')),
  CHECK (risk_level IS NULL OR risk_level IN ('consistent', 'suspicious', 'high_risk', 'insufficient_data'))
);

CREATE INDEX IF NOT EXISTS ix_model_audit_runs_target_created
ON model_audit_runs (target_id, created_at DESC, id DESC);

ALTER TABLE model_audit_targets
ADD CONSTRAINT model_audit_targets_last_run_id_fkey
FOREIGN KEY (last_run_id) REFERENCES model_audit_runs(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS model_audit_samples (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES model_audit_runs(id) ON DELETE CASCADE,
  target_id uuid NOT NULL REFERENCES model_audit_targets(id) ON DELETE RESTRICT,
  probe_type text NOT NULL,
  prompt_id text NOT NULL,
  prompt_hash text NOT NULL,
  prompt_text text NULL,
  response_text text NULL,
  response_hash text NULL,
  parsed_value text NULL,
  raw_json jsonb NULL,
  request_params_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  latency_ms integer NULL,
  first_token_latency_ms integer NULL,
  usage_prompt_tokens integer NULL,
  usage_completion_tokens integer NULL,
  estimated_prompt_tokens integer NULL,
  estimated_completion_tokens integer NULL,
  error_message text NULL,
  session_id text NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_model_audit_samples_run_probe
ON model_audit_samples (run_id, probe_type, created_at, id);

CREATE TABLE IF NOT EXISTS model_audit_probe_scores (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES model_audit_runs(id) ON DELETE CASCADE,
  probe text NOT NULL,
  risk text NOT NULL,
  confidence numeric NOT NULL DEFAULT 0,
  score numeric NOT NULL DEFAULT 0,
  evidence_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (risk IN ('consistent', 'suspicious', 'high_risk', 'insufficient_data', 'not_applicable'))
);

CREATE INDEX IF NOT EXISTS ix_model_audit_probe_scores_run
ON model_audit_probe_scores (run_id, probe);

CREATE TABLE IF NOT EXISTS model_audit_passive_call_features (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_id uuid NOT NULL REFERENCES model_audit_targets(id) ON DELETE RESTRICT,
  claimed_model text NOT NULL,
  request_hash text NULL,
  response_hash text NULL,
  prompt_length_chars integer NULL,
  response_length_chars integer NULL,
  estimated_prompt_tokens integer NULL,
  estimated_completion_tokens integer NULL,
  reported_prompt_tokens integer NULL,
  reported_completion_tokens integer NULL,
  latency_ms integer NULL,
  first_token_latency_ms integer NULL,
  status_code integer NULL,
  error_code text NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_model_audit_passive_target_created
ON model_audit_passive_call_features (target_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS model_audit_scheduled_monitors (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_id uuid NOT NULL REFERENCES model_audit_targets(id) ON DELETE CASCADE,
  baseline_id uuid NULL REFERENCES model_audit_baselines(id) ON DELETE SET NULL,
  mode text NOT NULL DEFAULT 'scheduled',
  enabled boolean NOT NULL DEFAULT true,
  cron_spec text NULL,
  last_run_id uuid NULL REFERENCES model_audit_runs(id) ON DELETE SET NULL,
  last_risk text NULL,
  last_run_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (mode IN ('quick', 'standard', 'strict', 'scheduled')),
  CHECK (last_risk IS NULL OR last_risk IN ('consistent', 'suspicious', 'high_risk', 'insufficient_data'))
);

CREATE INDEX IF NOT EXISTS ix_model_audit_monitors_enabled
ON model_audit_scheduled_monitors (enabled, updated_at DESC, id DESC);
