-- Guarded rollback for the unified operation-audit projection.
-- Date: 2026-08-12
-- Author: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM api_probe_connection_events
    WHERE action <> 'model_changed'
  ) OR EXISTS (
    SELECT 1
    FROM api_probe_connection_events event
    WHERE event.target_connection_id IS NOT NULL
      AND NOT EXISTS (
        SELECT 1 FROM api_probe_connections connection
        WHERE connection.id = event.target_connection_id
      )
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 92 after generic probe events or preserved deleted targets exist';
  END IF;
END;
$$;

DROP INDEX IF EXISTS ix_api_order_access_operation_actor_cursor;
DROP INDEX IF EXISTS ix_api_order_access_operation_cursor;
DROP INDEX IF EXISTS ix_api_intent_contact_access_operation_target_cursor;
DROP INDEX IF EXISTS ix_api_intent_contact_access_operation_cursor;
DROP INDEX IF EXISTS ix_contact_access_logs_operation_actor_cursor;
DROP INDEX IF EXISTS ix_contact_access_logs_operation_cursor;
DROP INDEX IF EXISTS ix_api_order_events_operation_target_cursor;
DROP INDEX IF EXISTS ix_api_order_events_operation_actor_cursor;
DROP INDEX IF EXISTS ix_api_order_events_operation_cursor;
DROP INDEX IF EXISTS ix_domain_events_operation_target_cursor;
DROP INDEX IF EXISTS ix_domain_events_operation_actor_cursor;
DROP INDEX IF EXISTS ix_domain_events_operation_cursor;
DROP INDEX IF EXISTS ix_moderation_audit_logs_operation_cursor;
DROP INDEX IF EXISTS ix_admin_audit_logs_operation_target_cursor;
DROP INDEX IF EXISTS ix_admin_audit_logs_operation_actor_cursor;
DROP INDEX IF EXISTS ix_admin_audit_logs_operation_cursor;

DROP VIEW IF EXISTS api_probe_connection_model_changes;

DROP TRIGGER IF EXISTS trg_api_probe_connection_events_append_only ON api_probe_connection_events;
DROP FUNCTION IF EXISTS reject_api_probe_connection_event_change();
DROP TRIGGER IF EXISTS trg_api_probe_connection_event_prepare ON api_probe_connection_events;
DROP FUNCTION IF EXISTS prepare_api_probe_connection_event();

DROP INDEX IF EXISTS ix_api_probe_connection_events_time;
DROP INDEX IF EXISTS ix_api_probe_connection_events_actor_time;
DROP INDEX IF EXISTS ix_api_probe_connection_events_target_time;
DROP INDEX IF EXISTS ux_api_probe_connection_events_request;

ALTER TABLE api_probe_connection_events
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_model_change_shape,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_request_id,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_changed_fields,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_verification_status,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_action,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_actor_shape,
  DROP CONSTRAINT IF EXISTS ck_api_probe_connection_events_actor_kind,
  DROP COLUMN IF EXISTS request_id,
  DROP COLUMN IF EXISTS changed_fields,
  DROP COLUMN IF EXISTS to_verification_status,
  DROP COLUMN IF EXISTS from_verification_status,
  DROP COLUMN IF EXISTS action,
  DROP COLUMN IF EXISTS actor_kind,
  DROP COLUMN IF EXISTS owner_user_id,
	ALTER COLUMN actor_user_id SET NOT NULL,
  ALTER COLUMN environment SET NOT NULL,
  ALTER COLUMN new_protocol SET NOT NULL,
  ALTER COLUMN new_model SET NOT NULL,
  ALTER COLUMN new_measurement_version SET NOT NULL;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN occurred_at TO changed_at;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN actor_user_id TO changed_by_user_id;

ALTER TABLE api_probe_connection_events
  RENAME COLUMN target_connection_id TO connection_id;

ALTER TABLE api_probe_connection_events
  ADD CONSTRAINT api_probe_connection_model_changes_connection_id_fkey
  FOREIGN KEY (connection_id) REFERENCES api_probe_connections(id) ON DELETE SET NULL;

ALTER TABLE api_probe_connection_events
  ADD CONSTRAINT api_probe_connection_model_changes_changed_by_user_id_fkey
  FOREIGN KEY (changed_by_user_id) REFERENCES users(id);

ALTER TABLE api_probe_connection_events
  RENAME TO api_probe_connection_model_changes;

CREATE INDEX ix_api_probe_connection_model_changes_connection
ON api_probe_connection_model_changes(connection_id, changed_at DESC, id DESC);
