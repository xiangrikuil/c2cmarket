-- Verification challenge and bounded data lifecycle hardening.
-- Date: 2026-07-26
-- Author: Codex

-- Existing bind-email challenges use the retired unkeyed digest and cannot be
-- verified after this release. Invalidate them before enforcing one active row.
UPDATE email_verification_codes
SET consumed_at = now()
WHERE purpose = 'bind_email'
  AND consumed_at IS NULL;

CREATE UNIQUE INDEX ux_email_verification_codes_active_bind_user
ON email_verification_codes(user_id)
WHERE purpose = 'bind_email' AND consumed_at IS NULL;

CREATE INDEX ix_email_verification_codes_consumed_at
ON email_verification_codes(consumed_at)
WHERE consumed_at IS NOT NULL;

CREATE INDEX ix_auth_sessions_expires_at
ON auth_sessions(expires_at);

CREATE INDEX ix_auth_sessions_revoked_at
ON auth_sessions(revoked_at)
WHERE revoked_at IS NOT NULL;

CREATE INDEX ix_contact_sessions_open_ends_at
ON contact_sessions(ends_at, id)
WHERE status = 'open';

CREATE INDEX ix_notifications_created_at
ON notifications(created_at, id);

CREATE INDEX ix_domain_events_created_at
ON domain_events(created_at, id);

ALTER TABLE idempotency_keys
DROP CONSTRAINT IF EXISTS idempotency_keys_status_check,
DROP CONSTRAINT IF EXISTS ck_idempotency_processing_empty_response,
DROP CONSTRAINT IF EXISTS ck_idempotency_completed_response;

ALTER TABLE idempotency_keys
ADD CONSTRAINT idempotency_keys_status_check
CHECK (status IN ('processing', 'completed', 'failed')),
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
),
ADD CONSTRAINT ck_idempotency_failed_response
CHECK (
  status <> 'failed'
  OR (
    completed_at IS NOT NULL
    AND response_status IS NULL
    AND response_content_type IS NULL
    AND response_body_json IS NULL
    AND response_body_cache_allowed = false
    AND resource_type IS NULL
    AND resource_id IS NULL
  )
);

UPDATE idempotency_keys
SET expires_at = LEAST(expires_at, created_at + interval '15 minutes')
WHERE status = 'processing';

UPDATE idempotency_keys
SET expires_at = completed_at + interval '7 days'
WHERE status = 'completed'
  AND completed_at IS NOT NULL;

UPDATE idempotency_keys
SET response_body_json = NULL,
    response_body_cache_allowed = false
WHERE status = 'completed'
  AND response_body_json IS NOT NULL
  AND octet_length(response_body_json::text) > 65536;
