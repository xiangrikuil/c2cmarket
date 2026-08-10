-- Upgrade reusable probe connections from /models checks to real model health samples.
-- Date: 2026-08-08
-- Executor: Codex

DELETE FROM api_probe_connection_samples;

ALTER TABLE api_probe_connections
ADD COLUMN probe_model text,
ADD COLUMN probe_protocol text
  CHECK (probe_protocol IS NULL OR probe_protocol IN ('openai_responses_v1', 'openai_chat_completions_v1')),
ADD COLUMN probe_models_snapshot jsonb NOT NULL DEFAULT '[]'::jsonb
  CHECK (jsonb_typeof(probe_models_snapshot) = 'array'),
ADD COLUMN probe_environment text NOT NULL DEFAULT 'us-west-v1'
  CHECK (trim(probe_environment) <> ''),
ADD COLUMN probe_model_changed_at timestamptz,
ADD COLUMN probe_price_version_id uuid REFERENCES api_model_price_versions(id),
ADD COLUMN probe_input_price_per_million numeric(14,6)
  CHECK (probe_input_price_per_million IS NULL OR probe_input_price_per_million >= 0),
ADD COLUMN probe_cached_input_price_per_million numeric(14,6)
  CHECK (probe_cached_input_price_per_million IS NULL OR probe_cached_input_price_per_million >= 0),
ADD COLUMN probe_output_price_per_million numeric(14,6)
  CHECK (probe_output_price_per_million IS NULL OR probe_output_price_per_million >= 0),
ADD COLUMN probe_price_currency text
  CHECK (probe_price_currency IS NULL OR trim(probe_price_currency) <> '');

UPDATE api_probe_connections
SET enabled = false,
    verification_status = 'unverified',
    verified_at = NULL,
    last_verification_error_code = NULL,
    measurement_version = measurement_version + 1,
    updated_at = now();

ALTER TABLE api_probe_connections
ADD CONSTRAINT ck_api_probe_connections_real_probe_ready
CHECK (
  enabled = false
  OR (
    verification_status = 'verified'
    AND credential_ciphertext IS NOT NULL
    AND probe_model IS NOT NULL
    AND trim(probe_model) <> ''
    AND probe_protocol IS NOT NULL
  )
);

CREATE TABLE api_probe_latency_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  model text NOT NULL CHECK (trim(model) <> ''),
  protocol text NOT NULL
    CHECK (protocol IN ('openai_responses_v1', 'openai_chat_completions_v1')),
  environment text NOT NULL CHECK (trim(environment) <> ''),
  version bigint NOT NULL CHECK (version > 0),
  slow_ttft_ms integer NOT NULL CHECK (slow_ttft_ms > 0),
  hard_timeout_ms integer NOT NULL CHECK (hard_timeout_ms > slow_ttft_ms AND hard_timeout_ms <= 30000),
  observation_started_at timestamptz NOT NULL,
  observation_ended_at timestamptz NOT NULL,
  complete_calendar_days integer NOT NULL CHECK (complete_calendar_days >= 0),
  connection_count integer NOT NULL CHECK (connection_count >= 0),
  sample_count bigint NOT NULL CHECK (sample_count >= 0),
  p50_ttft_ms integer,
  p90_ttft_ms integer,
  p95_ttft_ms integer,
  p99_ttft_ms integer,
  status text NOT NULL CHECK (status IN ('active', 'superseded')),
  published_by_admin_id uuid NOT NULL REFERENCES users(id),
  published_at timestamptz NOT NULL,
  superseded_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (model, protocol, environment, version),
  CHECK (observation_ended_at >= observation_started_at),
  CHECK (
    (status = 'active' AND superseded_at IS NULL)
    OR (status = 'superseded' AND superseded_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_api_probe_latency_rules_active
ON api_probe_latency_rules(model, protocol, environment)
WHERE status = 'active';

ALTER TABLE api_probe_connection_samples
ADD COLUMN probe_model text NOT NULL CHECK (trim(probe_model) <> ''),
ADD COLUMN probe_protocol text NOT NULL
  CHECK (probe_protocol IN ('openai_responses_v1', 'openai_chat_completions_v1')),
ADD COLUMN probe_environment text NOT NULL CHECK (trim(probe_environment) <> ''),
ADD COLUMN latency_rule_version_id uuid REFERENCES api_probe_latency_rules(id),
ADD COLUMN outcome text
  CHECK (outcome IS NULL OR outcome IN ('first_success', 'first_success_slow', 'retry_recovered', 'final_failure')),
ADD COLUMN attempt_count smallint NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 2),
ADD COLUMN first_attempt_ttft_ms integer CHECK (first_attempt_ttft_ms IS NULL OR first_attempt_ttft_ms >= 0),
ADD COLUMN first_attempt_total_duration_ms integer
  CHECK (first_attempt_total_duration_ms IS NULL OR first_attempt_total_duration_ms >= 0),
ADD COLUMN recovery_duration_ms integer CHECK (recovery_duration_ms IS NULL OR recovery_duration_ms >= 0),
ADD COLUMN final_http_status integer CHECK (final_http_status IS NULL OR final_http_status BETWEEN 100 AND 599),
ADD COLUMN input_tokens bigint CHECK (input_tokens IS NULL OR input_tokens >= 0),
ADD COLUMN cached_input_tokens bigint CHECK (cached_input_tokens IS NULL OR cached_input_tokens >= 0),
ADD COLUMN output_tokens bigint CHECK (output_tokens IS NULL OR output_tokens >= 0),
ADD COLUMN reasoning_tokens bigint CHECK (reasoning_tokens IS NULL OR reasoning_tokens >= 0),
ADD COLUMN usage_complete boolean NOT NULL DEFAULT false,
ADD COLUMN base_cost_usd numeric(20,10) CHECK (base_cost_usd IS NULL OR base_cost_usd >= 0),
ADD COLUMN retry_cost_usd numeric(20,10) CHECK (retry_cost_usd IS NULL OR retry_cost_usd >= 0),
ADD CONSTRAINT ck_api_probe_connection_samples_real_outcome
CHECK (
  (status = 'running' AND outcome IS NULL AND attempt_count = 0)
  OR (
    status = 'succeeded'
    AND outcome IN ('first_success', 'first_success_slow', 'retry_recovered')
    AND attempt_count BETWEEN 1 AND 2
  )
  OR (
    status = 'failed'
    AND outcome = 'final_failure'
    AND attempt_count BETWEEN 1 AND 2
  )
);

CREATE INDEX ix_api_probe_connection_samples_calibration
ON api_probe_connection_samples(probe_model, probe_protocol, probe_environment, slot_started_at)
WHERE outcome IN ('first_success', 'first_success_slow') AND first_attempt_ttft_ms IS NOT NULL;

CREATE TABLE api_probe_connection_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sample_id uuid NOT NULL REFERENCES api_probe_connection_samples(id) ON DELETE CASCADE,
  attempt_number smallint NOT NULL CHECK (attempt_number IN (1, 2)),
  started_at timestamptz NOT NULL,
  first_text_at timestamptz,
  finished_at timestamptz NOT NULL,
  http_status integer CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
  ttft_ms integer CHECK (ttft_ms IS NULL OR ttft_ms >= 0),
  total_duration_ms integer NOT NULL CHECK (total_duration_ms >= 0),
  succeeded boolean NOT NULL,
  retryable boolean NOT NULL,
  error_code text,
  retry_after_ms integer CHECK (retry_after_ms IS NULL OR retry_after_ms BETWEEN 0 AND 3000),
  input_tokens bigint CHECK (input_tokens IS NULL OR input_tokens >= 0),
  cached_input_tokens bigint CHECK (cached_input_tokens IS NULL OR cached_input_tokens >= 0),
  output_tokens bigint CHECK (output_tokens IS NULL OR output_tokens >= 0),
  reasoning_tokens bigint CHECK (reasoning_tokens IS NULL OR reasoning_tokens >= 0),
  usage_complete boolean NOT NULL DEFAULT false,
  cost_usd numeric(20,10) CHECK (cost_usd IS NULL OR cost_usd >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (sample_id, attempt_number),
  CHECK (
    (succeeded = true AND error_code IS NULL AND first_text_at IS NOT NULL AND ttft_ms IS NOT NULL)
    OR (succeeded = false AND error_code IS NOT NULL AND trim(error_code) <> '')
  )
);

CREATE INDEX ix_api_probe_connection_attempts_sample
ON api_probe_connection_attempts(sample_id, attempt_number);

CREATE TABLE api_probe_connection_model_changes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  connection_id uuid REFERENCES api_probe_connections(id) ON DELETE SET NULL,
  changed_by_user_id uuid NOT NULL REFERENCES users(id),
  old_measurement_version bigint,
  new_measurement_version bigint NOT NULL CHECK (new_measurement_version > 0),
  old_model text,
  new_model text NOT NULL CHECK (trim(new_model) <> ''),
  old_protocol text
    CHECK (old_protocol IS NULL OR old_protocol IN ('openai_responses_v1', 'openai_chat_completions_v1')),
  new_protocol text NOT NULL
    CHECK (new_protocol IN ('openai_responses_v1', 'openai_chat_completions_v1')),
  environment text NOT NULL CHECK (trim(environment) <> ''),
  changed_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_api_probe_connection_model_changes_connection
ON api_probe_connection_model_changes(connection_id, changed_at DESC, id DESC);
