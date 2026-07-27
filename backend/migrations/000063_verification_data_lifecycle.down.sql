-- Roll back verification and data lifecycle schema additions.
-- Date: 2026-07-26
-- Author: Codex

DROP INDEX IF EXISTS ix_domain_events_created_at;
DROP INDEX IF EXISTS ix_notifications_created_at;
DROP INDEX IF EXISTS ix_contact_sessions_open_ends_at;
DROP INDEX IF EXISTS ix_auth_sessions_revoked_at;
DROP INDEX IF EXISTS ix_auth_sessions_expires_at;
DROP INDEX IF EXISTS ix_email_verification_codes_consumed_at;
DROP INDEX IF EXISTS ux_email_verification_codes_active_bind_user;

DELETE FROM idempotency_keys
WHERE status = 'failed';

ALTER TABLE idempotency_keys
DROP CONSTRAINT IF EXISTS ck_idempotency_failed_response,
DROP CONSTRAINT IF EXISTS ck_idempotency_completed_response,
DROP CONSTRAINT IF EXISTS ck_idempotency_processing_empty_response,
DROP CONSTRAINT IF EXISTS idempotency_keys_status_check;

ALTER TABLE idempotency_keys
ADD CONSTRAINT idempotency_keys_status_check
CHECK (status IN ('processing', 'completed')),
ADD CONSTRAINT ck_idempotency_processing_empty_response
CHECK (
  status <> 'processing'
  OR (
    completed_at IS NULL
    AND response_status IS NULL
    AND response_content_type IS NULL
    AND response_body_json IS NULL
    AND resource_type IS NULL
    AND resource_id IS NULL
  )
),
ADD CONSTRAINT ck_idempotency_completed_response
CHECK (
  status <> 'completed'
  OR (
    completed_at IS NOT NULL
    AND response_status IS NOT NULL
    AND response_content_type IS NOT NULL
    AND resource_type IS NOT NULL
    AND resource_id IS NOT NULL
    AND (
      response_body_cache_allowed = true
      OR response_body_json IS NULL
    )
  )
);
