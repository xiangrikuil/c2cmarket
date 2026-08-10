-- Replace service-scoped probes with reusable seller connections and canonical model keys.
-- Date: 2026-08-08
-- Executor: Codex

DROP TABLE api_service_probe_samples;
DROP TABLE api_service_probe_authorization_events;
DROP TABLE api_service_probe_configs;

CREATE TABLE api_probe_connections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  name text NOT NULL CHECK (trim(name) <> ''),
  base_url text NOT NULL CHECK (trim(base_url) <> ''),
  normalized_base_url text NOT NULL CHECK (trim(normalized_base_url) <> ''),
  credential_ciphertext bytea,
  credential_nonce bytea,
  credential_key_version text,
  credential_cipher_format text,
  credential_fingerprint bytea,
  enabled boolean NOT NULL DEFAULT false,
  verification_status text NOT NULL DEFAULT 'unverified'
    CHECK (verification_status IN ('unverified', 'verified', 'failed')),
  verified_at timestamptz,
  last_verification_error_code text,
  measurement_version bigint NOT NULL DEFAULT 1 CHECK (measurement_version > 0),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, owner_user_id),
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
  CHECK (
    (verification_status = 'verified' AND verified_at IS NOT NULL AND last_verification_error_code IS NULL)
    OR (verification_status = 'unverified' AND verified_at IS NULL AND last_verification_error_code IS NULL)
    OR (verification_status = 'failed' AND verified_at IS NULL AND last_verification_error_code IS NOT NULL)
  ),
  CHECK (enabled = false OR (verification_status = 'verified' AND credential_ciphertext IS NOT NULL))
);

CREATE INDEX ix_api_probe_connections_owner
ON api_probe_connections(owner_user_id, updated_at DESC, id DESC);

CREATE INDEX ix_api_probe_connections_due
ON api_probe_connections(updated_at, id)
WHERE enabled = true AND verification_status = 'verified';

CREATE INDEX ix_api_probe_connections_normalized_base_url
ON api_probe_connections(owner_user_id, normalized_base_url, updated_at DESC);

CREATE TABLE api_probe_connection_samples (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  connection_id uuid NOT NULL REFERENCES api_probe_connections(id) ON DELETE CASCADE,
  measurement_version bigint NOT NULL CHECK (measurement_version > 0),
  slot_started_at timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
  total_duration_ms integer,
  http_status_class integer,
  error_code text,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (connection_id, slot_started_at),
  CHECK (slot_started_at = date_bin(interval '5 minutes', slot_started_at, timestamptz '1970-01-01 00:00:00+00')),
  CHECK (http_status_class IS NULL OR http_status_class BETWEEN 1 AND 5),
  CHECK (
    (status = 'running' AND total_duration_ms IS NULL AND error_code IS NULL AND finished_at IS NULL)
    OR (
      status = 'succeeded'
      AND total_duration_ms >= 0
      AND error_code IS NULL
      AND finished_at IS NOT NULL
    )
    OR (
      status = 'failed'
      AND total_duration_ms >= 0
      AND error_code IS NOT NULL
      AND trim(error_code) <> ''
      AND finished_at IS NOT NULL
    )
  )
);

CREATE INDEX ix_api_probe_connection_samples_summary
ON api_probe_connection_samples(connection_id, measurement_version, slot_started_at DESC)
WHERE status IN ('succeeded', 'failed');

CREATE INDEX ix_api_probe_connection_samples_retention
ON api_probe_connection_samples(finished_at, id)
WHERE status IN ('succeeded', 'failed');

CREATE INDEX ix_api_probe_connection_samples_running
ON api_probe_connection_samples(started_at, id)
WHERE status = 'running';

ALTER TABLE api_services
ADD COLUMN probe_connection_id uuid,
ADD CONSTRAINT fk_api_services_probe_connection_owner
  FOREIGN KEY (probe_connection_id, owner_user_id)
  REFERENCES api_probe_connections(id, owner_user_id)
  ON DELETE RESTRICT;

CREATE INDEX ix_api_services_probe_connection
ON api_services(probe_connection_id)
WHERE probe_connection_id IS NOT NULL;

ALTER TABLE api_orders
ADD COLUMN probe_connection_id_snapshot uuid,
ADD COLUMN api_base_url_snapshot text,
ADD COLUMN normalized_api_base_url_snapshot text,
ADD CONSTRAINT ck_api_orders_probe_target_snapshot
CHECK (
  (
    probe_connection_id_snapshot IS NULL
    AND api_base_url_snapshot IS NULL
    AND normalized_api_base_url_snapshot IS NULL
  )
  OR (
    probe_connection_id_snapshot IS NOT NULL
    AND api_base_url_snapshot IS NOT NULL
    AND trim(api_base_url_snapshot) <> ''
    AND normalized_api_base_url_snapshot IS NOT NULL
    AND trim(normalized_api_base_url_snapshot) <> ''
  )
);

DROP INDEX ix_api_service_models_search_trgm;
DROP INDEX ix_api_model_catalog_search_trgm;

UPDATE api_service_models AS service_model
SET model_name_snapshot = catalog.model_key
FROM api_model_catalog AS catalog
WHERE catalog.id = service_model.model_catalog_id;

ALTER TABLE api_service_models
RENAME COLUMN model_name_snapshot TO model_key_snapshot;

ALTER TABLE api_model_catalog
DROP COLUMN display_name;

CREATE INDEX ix_api_service_models_search_trgm
ON api_service_models
USING gin ((lower(model_key_snapshot || ' ' || provider_snapshot)) gin_trgm_ops);

CREATE INDEX ix_api_model_catalog_search_trgm
ON api_model_catalog
USING gin ((lower(model_key)) gin_trgm_ops);
