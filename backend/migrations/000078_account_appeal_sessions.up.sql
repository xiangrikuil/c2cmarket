-- Dedicated restricted-account appeal sessions and account-governance appeals.
-- Date: 2026-08-03
-- Author: Codex

CREATE TABLE account_appeal_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_token_hash text NOT NULL UNIQUE,
  csrf_token_hash text NOT NULL,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CONSTRAINT ck_account_appeal_sessions_fixed_expiry
    CHECK (expires_at = created_at + interval '15 minutes'),
  CONSTRAINT ck_account_appeal_sessions_revocation_time
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX ix_account_appeal_sessions_user
ON account_appeal_sessions(user_id, created_at DESC);

CREATE INDEX ix_account_appeal_sessions_lifecycle
ON account_appeal_sessions(COALESCE(revoked_at, expires_at), id);

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS appeals_target_type_check,
DROP CONSTRAINT IF EXISTS ck_appeals_target_type,
DROP CONSTRAINT IF EXISTS appeals_check,
DROP CONSTRAINT IF EXISTS ck_appeals_source,
ADD CONSTRAINT ck_appeals_target_type
CHECK (
  target_type IN (
    'contact_snapshot',
    'public_user',
    'carpool_application',
    'carpool_membership',
    'api_purchase_intent',
    'api_order',
    'account_governance'
  )
),
ADD CONSTRAINT ck_appeals_source
CHECK (
  (
    target_type = 'account_governance'
    AND report_id IS NULL
    AND dispute_case_id IS NULL
    AND target_id = appellant_user_id::text
  )
  OR (
    target_type <> 'account_governance'
    AND (report_id IS NOT NULL OR dispute_case_id IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_appeals_submitted_account_governance
ON appeals(appellant_user_id)
WHERE target_type = 'account_governance' AND status = 'submitted';
