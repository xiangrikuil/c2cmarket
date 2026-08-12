-- Unified operation-audit indexes and the lineage-preserving probe event ledger.
-- Date: 2026-08-12
-- Author: Codex

ALTER TABLE api_probe_connection_model_changes
  RENAME TO api_probe_connection_events;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN connection_id TO target_connection_id;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN changed_by_user_id TO actor_user_id;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN changed_at TO occurred_at;

DROP INDEX IF EXISTS ix_api_probe_connection_model_changes_connection;

ALTER TABLE api_probe_connection_events
  DROP CONSTRAINT IF EXISTS api_probe_connection_model_changes_connection_id_fkey,
	DROP CONSTRAINT IF EXISTS api_probe_connection_model_changes_changed_by_user_id_fkey,
	ALTER COLUMN actor_user_id DROP NOT NULL,
  ALTER COLUMN new_measurement_version DROP NOT NULL,
  ALTER COLUMN new_model DROP NOT NULL,
  ALTER COLUMN new_protocol DROP NOT NULL,
  ALTER COLUMN environment DROP NOT NULL,
	ADD COLUMN owner_user_id uuid,
  ADD COLUMN actor_kind text NOT NULL DEFAULT 'user',
  ADD COLUMN action text NOT NULL DEFAULT 'model_changed',
  ADD COLUMN from_verification_status text,
  ADD COLUMN to_verification_status text,
  ADD COLUMN changed_fields text[] NOT NULL DEFAULT '{}'::text[],
  ADD COLUMN request_id text;

UPDATE api_probe_connection_events
SET owner_user_id = actor_user_id,
    actor_kind = 'user',
    action = 'model_changed',
    changed_fields = ARRAY['probe_model', 'probe_protocol']::text[],
    request_id = 'legacy-probe-' || id::text;

ALTER TABLE api_probe_connection_events
	ALTER COLUMN owner_user_id SET NOT NULL,
	ALTER COLUMN request_id SET NOT NULL,
	ADD CONSTRAINT ck_api_probe_connection_events_actor_kind
    CHECK (actor_kind IN ('user', 'admin', 'system')),
  ADD CONSTRAINT ck_api_probe_connection_events_actor_shape
    CHECK (
      (actor_kind IN ('user', 'admin') AND actor_user_id IS NOT NULL)
      OR actor_kind = 'system'
    ),
  ADD CONSTRAINT ck_api_probe_connection_events_action
    CHECK (action IN (
      'created', 'updated', 'model_changed', 'verify_succeeded',
      'verify_failed', 'enabled', 'disabled', 'deleted'
    )),
  ADD CONSTRAINT ck_api_probe_connection_events_verification_status
    CHECK (
      (from_verification_status IS NULL OR from_verification_status IN ('unverified', 'verified', 'failed'))
      AND (to_verification_status IS NULL OR to_verification_status IN ('unverified', 'verified', 'failed'))
    ),
  ADD CONSTRAINT ck_api_probe_connection_events_changed_fields
    CHECK (
      changed_fields <@ ARRAY[
        'name', 'base_url', 'credential', 'probe_model',
        'probe_protocol', 'environment', 'enabled'
      ]::text[]
      AND array_position(changed_fields, NULL) IS NULL
    ),
  ADD CONSTRAINT ck_api_probe_connection_events_request_id
    CHECK (request_id = btrim(request_id) AND octet_length(request_id) BETWEEN 1 AND 200),
  ADD CONSTRAINT ck_api_probe_connection_events_model_change_shape
    CHECK (
      action <> 'model_changed'
      OR (
        new_measurement_version IS NOT NULL
        AND new_measurement_version > 0
        AND new_model IS NOT NULL
        AND btrim(new_model) <> ''
        AND new_protocol IS NOT NULL
        AND environment IS NOT NULL
        AND btrim(environment) <> ''
      )
    );

CREATE FUNCTION prepare_api_probe_connection_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.owner_user_id := COALESCE(NEW.owner_user_id, NEW.actor_user_id);
  NEW.actor_kind := COALESCE(NULLIF(btrim(NEW.actor_kind), ''), 'user');
  NEW.action := COALESCE(NULLIF(btrim(NEW.action), ''), 'model_changed');
  NEW.request_id := COALESCE(NULLIF(btrim(NEW.request_id), ''), 'probe-event-' || gen_random_uuid()::text);
  IF NEW.action = 'model_changed' AND cardinality(NEW.changed_fields) = 0 THEN
    NEW.changed_fields := ARRAY['probe_model', 'probe_protocol']::text[];
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_api_probe_connection_event_prepare
BEFORE INSERT ON api_probe_connection_events
FOR EACH ROW
EXECUTE FUNCTION prepare_api_probe_connection_event();

CREATE FUNCTION reject_api_probe_connection_event_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'api probe connection events are append-only';
END;
$$;

CREATE TRIGGER trg_api_probe_connection_events_append_only
BEFORE UPDATE OR DELETE ON api_probe_connection_events
FOR EACH ROW
EXECUTE FUNCTION reject_api_probe_connection_event_change();

CREATE UNIQUE INDEX ux_api_probe_connection_events_request
ON api_probe_connection_events(target_connection_id, action, request_id)
WHERE target_connection_id IS NOT NULL;

CREATE INDEX ix_api_probe_connection_events_target_time
ON api_probe_connection_events(target_connection_id, occurred_at DESC, id DESC);

CREATE INDEX ix_api_probe_connection_events_actor_time
ON api_probe_connection_events(actor_user_id, occurred_at DESC, id DESC)
WHERE actor_user_id IS NOT NULL;

CREATE INDEX ix_api_probe_connection_events_time
ON api_probe_connection_events(occurred_at DESC, id DESC);

-- 旧探针仓储仍可通过该可更新视图写入模型切换；事实只落在通用 ledger 一处。
CREATE VIEW api_probe_connection_model_changes AS
SELECT id,
       target_connection_id AS connection_id,
       actor_user_id AS changed_by_user_id,
       old_measurement_version,
       new_measurement_version,
       old_model,
       new_model,
       old_protocol,
       new_protocol,
       environment,
       occurred_at AS changed_at,
       created_at
FROM api_probe_connection_events
WHERE action = 'model_changed';

CREATE INDEX IF NOT EXISTS ix_admin_audit_logs_operation_cursor
ON admin_audit_logs(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_admin_audit_logs_operation_actor_cursor
ON admin_audit_logs(admin_user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_admin_audit_logs_operation_target_cursor
ON admin_audit_logs(target_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_operation_cursor
ON moderation_audit_logs(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_domain_events_operation_cursor
ON domain_events(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_domain_events_operation_actor_cursor
ON domain_events(actor_user_id, created_at DESC, id DESC)
WHERE actor_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_domain_events_operation_target_cursor
ON domain_events(aggregate_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_order_events_operation_cursor
ON api_order_events(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_order_events_operation_actor_cursor
ON api_order_events(actor_user_id, created_at DESC, id DESC)
WHERE actor_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_api_order_events_operation_target_cursor
ON api_order_events(api_order_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_contact_access_logs_operation_cursor
ON contact_access_logs(accessed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_contact_access_logs_operation_actor_cursor
ON contact_access_logs(viewer_user_id, accessed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_intent_contact_access_operation_cursor
ON api_purchase_intent_contact_access_logs(accessed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_intent_contact_access_operation_target_cursor
ON api_purchase_intent_contact_access_logs(api_purchase_intent_id, accessed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_order_access_operation_cursor
ON api_order_payment_instruction_access_logs(accessed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_api_order_access_operation_actor_cursor
ON api_order_payment_instruction_access_logs(buyer_user_id, accessed_at DESC, id DESC);
