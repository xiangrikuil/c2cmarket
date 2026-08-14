-- Account-governance actions, restricted business sessions, suspension expiry,
-- and purpose-bound administrator reauthentication.
-- Date: 2026-08-13
-- Author: Codex

ALTER TABLE users
  ADD COLUMN governance_version bigint NOT NULL DEFAULT 1,
  ADD COLUMN current_governance_action_id uuid,
  ADD COLUMN security_locked_at timestamptz,
  ADD COLUMN security_lock_reason text,
  ADD CONSTRAINT ck_users_governance_version CHECK (governance_version >= 1),
  ADD CONSTRAINT ck_users_security_lock_shape CHECK (
    (security_locked_at IS NULL AND security_lock_reason IS NULL)
    OR (
      security_locked_at IS NOT NULL
      AND security_lock_reason = btrim(security_lock_reason)
      AND char_length(security_lock_reason) BETWEEN 1 AND 500
    )
  );

CREATE TABLE account_governance_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  action_type text NOT NULL CHECK (
    action_type IN (
      'suspend',
      'extend_suspension',
      'ban',
      'restore',
      'security_lock',
      'security_unlock',
      'archive',
      'restore_archive'
    )
  ),
  status text NOT NULL DEFAULT 'effective' CHECK (status IN ('effective', 'superseded', 'completed')),
  governance_version bigint NOT NULL CHECK (governance_version >= 1),
  reason_code text NOT NULL,
  public_reason text NOT NULL,
  internal_note text,
  linked_case_type text,
  linked_case_id text,
  effective_at timestamptz NOT NULL,
  expires_at timestamptz,
  is_indefinite boolean NOT NULL DEFAULT false,
  supersedes_action_id uuid REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  superseded_at timestamptz,
  actor_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
  request_id text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  completed_at timestamptz,
  CONSTRAINT ux_account_governance_actions_user_version
    UNIQUE(target_user_id, governance_version),
  CONSTRAINT ck_account_governance_action_reason CHECK (
    reason_code = btrim(reason_code)
    AND char_length(reason_code) BETWEEN 1 AND 80
    AND public_reason = btrim(public_reason)
    AND char_length(public_reason) BETWEEN 1 AND 500
    AND (
      internal_note IS NULL
      OR (
        internal_note = btrim(internal_note)
        AND char_length(internal_note) BETWEEN 1 AND 2000
      )
    )
  ),
  CONSTRAINT ck_account_governance_action_case CHECK (
    (linked_case_type IS NULL AND linked_case_id IS NULL)
    OR (
      linked_case_type IS NOT NULL
      AND linked_case_id IS NOT NULL
      AND linked_case_type = btrim(linked_case_type)
      AND linked_case_id = btrim(linked_case_id)
      AND char_length(linked_case_type) BETWEEN 1 AND 80
      AND char_length(linked_case_id) BETWEEN 1 AND 160
    )
  ),
  CONSTRAINT ck_account_governance_action_expiry CHECK (
    (
      action_type IN ('suspend', 'extend_suspension')
      AND (
        (is_indefinite AND expires_at IS NULL)
        OR (NOT is_indefinite AND expires_at > effective_at)
      )
    )
    OR (
      action_type NOT IN ('suspend', 'extend_suspension')
      AND NOT is_indefinite
      AND expires_at IS NULL
    )
  ),
  CONSTRAINT ck_account_governance_action_superseded CHECK (
    (status <> 'superseded' AND superseded_at IS NULL)
    OR (status = 'superseded' AND superseded_at IS NOT NULL AND superseded_at >= effective_at)
  ),
  CONSTRAINT ck_account_governance_action_timestamps CHECK (
    created_at <= effective_at
    AND created_at <= updated_at
    AND (completed_at IS NULL OR completed_at >= effective_at)
  )
);

ALTER TABLE users
  ADD CONSTRAINT fk_users_current_governance_action
  FOREIGN KEY (current_governance_action_id)
  REFERENCES account_governance_actions(id)
  ON DELETE RESTRICT;

CREATE UNIQUE INDEX ux_account_governance_actions_active_user
ON account_governance_actions(target_user_id)
WHERE status = 'effective';

CREATE INDEX ix_account_governance_actions_user_recent
ON account_governance_actions(target_user_id, created_at DESC, id DESC);

CREATE TABLE restricted_business_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_token_hash text NOT NULL UNIQUE,
  csrf_token_hash text NOT NULL,
  governance_action_id uuid NOT NULL REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  governance_version bigint NOT NULL CHECK (governance_version >= 1),
  restriction_effective_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  last_seen_at timestamptz NOT NULL,
  CONSTRAINT ck_restricted_business_session_lifetime CHECK (
    expires_at = created_at + interval '24 hours'
  ),
  CONSTRAINT ck_restricted_business_session_timestamps CHECK (
    restriction_effective_at <= created_at
    AND last_seen_at >= created_at
    AND (revoked_at IS NULL OR revoked_at >= created_at)
  )
);

CREATE INDEX ix_restricted_business_sessions_user
ON restricted_business_sessions(user_id, created_at DESC);

CREATE INDEX ix_restricted_business_sessions_lifecycle
ON restricted_business_sessions(COALESCE(revoked_at, expires_at), id);

CREATE TABLE restricted_business_oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  CONSTRAINT ck_restricted_business_oauth_state_lifetime CHECK (
    expires_at = created_at + interval '10 minutes'
  ),
  CONSTRAINT ck_restricted_business_oauth_state_consumed CHECK (
    consumed_at IS NULL OR consumed_at >= created_at
  )
);

CREATE INDEX ix_restricted_business_oauth_states_lifecycle
ON restricted_business_oauth_states(COALESCE(consumed_at, expires_at), id);

CREATE TABLE account_appeal_oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  CONSTRAINT ck_account_appeal_oauth_state_lifetime CHECK (
    expires_at = created_at + interval '10 minutes'
  ),
  CONSTRAINT ck_account_appeal_oauth_state_consumed CHECK (
    consumed_at IS NULL OR consumed_at >= created_at
  )
);

CREATE INDEX ix_account_appeal_oauth_states_lifecycle
ON account_appeal_oauth_states(COALESCE(consumed_at, expires_at), id);

CREATE TABLE account_governance_expiry_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  suspension_action_id uuid NOT NULL REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  expected_governance_version bigint NOT NULL CHECK (expected_governance_version >= 1),
  expected_expires_at timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (
    status IN ('pending', 'processing', 'restored', 'noop_superseded', 'failed')
  ),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at timestamptz NOT NULL,
  locked_at timestamptz,
  completed_at timestamptz,
  last_error_code text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT ux_account_governance_expiry_action UNIQUE(suspension_action_id),
  CONSTRAINT ck_account_governance_expiry_schedule CHECK (
    expected_expires_at = available_at
    AND created_at <= updated_at
    AND (locked_at IS NULL OR locked_at >= created_at)
    AND (completed_at IS NULL OR completed_at >= created_at)
  )
);

CREATE INDEX ix_account_governance_expiry_jobs_due
ON account_governance_expiry_jobs(available_at, id)
WHERE status IN ('pending', 'failed');

CREATE TABLE admin_reauthentication_grants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  auth_session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
  purpose text NOT NULL CHECK (purpose = 'grant_admin'),
  method text NOT NULL CHECK (method IN ('password', 'linux_do_oauth')),
  verified_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL,
  CONSTRAINT ck_admin_reauth_lifetime CHECK (
    expires_at = verified_at + interval '10 minutes'
  ),
  CONSTRAINT ck_admin_reauth_timestamps CHECK (
    created_at <= verified_at
    AND (consumed_at IS NULL OR consumed_at >= verified_at)
    AND (revoked_at IS NULL OR revoked_at >= verified_at)
    AND NOT (consumed_at IS NOT NULL AND revoked_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_admin_reauthentication_grants_active
ON admin_reauthentication_grants(auth_session_id, purpose)
WHERE consumed_at IS NULL AND revoked_at IS NULL;

CREATE INDEX ix_admin_reauthentication_grants_lifecycle
ON admin_reauthentication_grants(COALESCE(consumed_at, revoked_at, expires_at), id);

CREATE TABLE admin_reauthentication_oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  auth_session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
  state_hash text NOT NULL UNIQUE,
  purpose text NOT NULL CHECK (purpose = 'grant_admin_reauth'),
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  CONSTRAINT ck_admin_reauth_oauth_state_lifetime CHECK (
    expires_at = created_at + interval '10 minutes'
  ),
  CONSTRAINT ck_admin_reauth_oauth_state_consumed CHECK (
    consumed_at IS NULL OR consumed_at >= created_at
  )
);

CREATE UNIQUE INDEX ux_admin_reauthentication_oauth_states_active
ON admin_reauthentication_oauth_states(auth_session_id, purpose)
WHERE consumed_at IS NULL;

CREATE INDEX ix_admin_reauthentication_oauth_states_lifecycle
ON admin_reauthentication_oauth_states(COALESCE(consumed_at, expires_at), id);
