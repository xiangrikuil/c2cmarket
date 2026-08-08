-- Restore the pre-connection schema. Deleted legacy probe samples cannot be recovered.
-- Date: 2026-08-08
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_probe_connections)
    OR EXISTS (SELECT 1 FROM api_probe_connection_samples)
    OR EXISTS (SELECT 1 FROM api_services WHERE probe_connection_id IS NOT NULL)
    OR EXISTS (
      SELECT 1 FROM api_orders
      WHERE probe_connection_id_snapshot IS NOT NULL
         OR api_base_url_snapshot IS NOT NULL
         OR normalized_api_base_url_snapshot IS NOT NULL
    ) THEN
    RAISE EXCEPTION 'cannot roll back migration 81 while probe connection data, bindings, or order target snapshots exist';
  END IF;
END $$;

DROP INDEX ix_api_model_catalog_search_trgm;
DROP INDEX ix_api_service_models_search_trgm;

ALTER TABLE api_model_catalog
ADD COLUMN display_name text;

UPDATE api_model_catalog
SET display_name = model_key;

ALTER TABLE api_model_catalog
ALTER COLUMN display_name SET NOT NULL;

ALTER TABLE api_service_models
RENAME COLUMN model_key_snapshot TO model_name_snapshot;

CREATE INDEX ix_api_service_models_search_trgm
ON api_service_models
USING gin ((lower(model_name_snapshot || ' ' || provider_snapshot)) gin_trgm_ops);

CREATE INDEX ix_api_model_catalog_search_trgm
ON api_model_catalog
USING gin ((lower(display_name || ' ' || model_key)) gin_trgm_ops);

ALTER TABLE api_orders
DROP CONSTRAINT ck_api_orders_probe_target_snapshot,
DROP COLUMN normalized_api_base_url_snapshot,
DROP COLUMN api_base_url_snapshot,
DROP COLUMN probe_connection_id_snapshot;

DROP INDEX ix_api_services_probe_connection;

ALTER TABLE api_services
DROP CONSTRAINT fk_api_services_probe_connection_owner,
DROP COLUMN probe_connection_id;

DROP TABLE api_probe_connection_samples;
DROP TABLE api_probe_connections;

CREATE TABLE api_service_probe_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL UNIQUE,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  protocol text NOT NULL DEFAULT 'openai_chat_completions_v1'
    CHECK (protocol = 'openai_chat_completions_v1'),
  base_url text NOT NULL CHECK (trim(base_url) <> ''),
  normalized_origin text NOT NULL CHECK (trim(normalized_origin) <> ''),
  model text NOT NULL CHECK (trim(model) <> ''),
  credential_ciphertext bytea,
  credential_nonce bytea,
  credential_key_version text,
  credential_cipher_format text,
  credential_fingerprint bytea,
  enabled boolean NOT NULL DEFAULT false,
  authorization_status text NOT NULL DEFAULT 'pending'
    CHECK (authorization_status IN ('pending', 'verified', 'approved', 'rejected')),
  authorization_method text
    CHECK (authorization_method IS NULL OR authorization_method IN ('dns_txt', 'http_challenge', 'admin_approval')),
  verified_origin text,
  verified_at timestamptz,
  approved_by_admin_id uuid REFERENCES users(id),
  approved_at timestamptz,
  rejection_reason text,
  challenge_token_hash bytea,
  challenge_expires_at timestamptz,
  measurement_version bigint NOT NULL DEFAULT 1 CHECK (measurement_version > 0),
  last_config_error_code text,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, api_service_id),
  UNIQUE (id, api_service_id, owner_user_id),
  FOREIGN KEY (api_service_id, owner_user_id)
    REFERENCES api_services(id, owner_user_id) ON DELETE CASCADE,
  CHECK (
    (
      credential_ciphertext IS NULL
      AND credential_nonce IS NULL
      AND credential_key_version IS NULL
      AND credential_cipher_format IS NULL
      AND credential_fingerprint IS NULL
    )
    OR (
      credential_ciphertext IS NOT NULL
      AND credential_nonce IS NOT NULL
      AND credential_key_version IS NOT NULL
      AND credential_cipher_format IS NOT NULL
      AND credential_fingerprint IS NOT NULL
    )
  ),
  CHECK (enabled = false OR credential_ciphertext IS NOT NULL),
  CHECK (
    (challenge_token_hash IS NULL AND challenge_expires_at IS NULL)
    OR (challenge_token_hash IS NOT NULL AND challenge_expires_at IS NOT NULL)
  ),
  CHECK (
    (authorization_status = 'pending' AND verified_origin IS NULL AND verified_at IS NULL AND approved_by_admin_id IS NULL AND approved_at IS NULL)
    OR (
      authorization_status = 'verified'
      AND authorization_method IN ('dns_txt', 'http_challenge')
      AND verified_origin = normalized_origin
      AND verified_at IS NOT NULL
      AND approved_by_admin_id IS NULL
      AND approved_at IS NULL
      AND rejection_reason IS NULL
    )
    OR (
      authorization_status = 'approved'
      AND authorization_method = 'admin_approval'
      AND verified_origin = normalized_origin
      AND verified_at IS NOT NULL
      AND approved_by_admin_id IS NOT NULL
      AND approved_at IS NOT NULL
      AND rejection_reason IS NULL
    )
    OR (
      authorization_status = 'rejected'
      AND authorization_method = 'admin_approval'
      AND verified_origin IS NULL
      AND verified_at IS NULL
      AND approved_by_admin_id IS NOT NULL
      AND approved_at IS NOT NULL
      AND rejection_reason IS NOT NULL
      AND trim(rejection_reason) <> ''
    )
  )
);

CREATE INDEX ix_api_service_probe_configs_due
ON api_service_probe_configs(enabled, authorization_status, updated_at, id)
WHERE enabled = true;

CREATE INDEX ix_api_service_probe_configs_admin_review
ON api_service_probe_configs(authorization_status, updated_at DESC, id DESC);

CREATE TABLE api_service_probe_authorization_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  probe_config_id uuid NOT NULL,
  api_service_id uuid NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
  actor_user_id uuid REFERENCES users(id),
  action text NOT NULL CHECK (action IN (
    'challenge_created',
    'verification_succeeded',
    'verification_failed',
    'admin_approved',
    'admin_rejected',
    'origin_invalidated',
    'config_deleted'
  )),
  method text CHECK (method IS NULL OR method IN ('dns_txt', 'http_challenge', 'admin_approval')),
  origin_snapshot text NOT NULL CHECK (trim(origin_snapshot) <> ''),
  reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_api_service_probe_authorization_events_config
ON api_service_probe_authorization_events(probe_config_id, created_at DESC, id DESC);

CREATE INDEX ix_api_service_probe_authorization_events_service
ON api_service_probe_authorization_events(api_service_id, created_at DESC, id DESC);

CREATE TABLE api_service_probe_samples (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
  probe_config_id uuid NOT NULL,
  measurement_version bigint NOT NULL CHECK (measurement_version > 0),
  probe_model_snapshot text NOT NULL CHECK (trim(probe_model_snapshot) <> ''),
  slot_started_at timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
  ttft_ms integer,
  total_duration_ms integer,
  http_status_class integer,
  error_code text,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (api_service_id, slot_started_at),
  FOREIGN KEY (probe_config_id, api_service_id)
    REFERENCES api_service_probe_configs(id, api_service_id) ON DELETE CASCADE,
  CHECK (slot_started_at = date_bin(interval '5 minutes', slot_started_at, timestamptz '1970-01-01 00:00:00+00')),
  CHECK (http_status_class IS NULL OR http_status_class BETWEEN 1 AND 5),
  CHECK (
    (status = 'running' AND ttft_ms IS NULL AND total_duration_ms IS NULL AND error_code IS NULL AND finished_at IS NULL)
    OR (
      status = 'succeeded'
      AND ttft_ms >= 0
      AND total_duration_ms >= ttft_ms
      AND error_code IS NULL
      AND finished_at IS NOT NULL
    )
    OR (
      status = 'failed'
      AND ttft_ms IS NULL
      AND total_duration_ms >= 0
      AND error_code IS NOT NULL
      AND trim(error_code) <> ''
      AND finished_at IS NOT NULL
    )
  )
);

CREATE INDEX ix_api_service_probe_samples_summary
ON api_service_probe_samples(api_service_id, measurement_version, slot_started_at DESC)
WHERE status IN ('succeeded', 'failed');

CREATE INDEX ix_api_service_probe_samples_retention
ON api_service_probe_samples(finished_at, id)
WHERE status IN ('succeeded', 'failed');

CREATE INDEX ix_api_service_probe_samples_running
ON api_service_probe_samples(started_at, id)
WHERE status = 'running';
