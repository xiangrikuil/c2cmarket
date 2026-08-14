-- Roll back account-governance session foundation before dependent records exist.
-- Date: 2026-08-13
-- Author: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM account_governance_actions)
     OR EXISTS (SELECT 1 FROM restricted_business_sessions)
     OR EXISTS (SELECT 1 FROM restricted_business_oauth_states)
     OR EXISTS (SELECT 1 FROM account_appeal_oauth_states)
     OR EXISTS (SELECT 1 FROM admin_reauthentication_grants)
     OR EXISTS (SELECT 1 FROM admin_reauthentication_oauth_states) THEN
    RAISE EXCEPTION 'cannot roll back account governance foundation after records exist'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

DROP INDEX IF EXISTS ux_admin_reauthentication_oauth_states_active;
DROP INDEX IF EXISTS ix_admin_reauthentication_oauth_states_lifecycle;
DROP TABLE IF EXISTS admin_reauthentication_oauth_states;

DROP INDEX IF EXISTS ux_admin_reauthentication_grants_active;
DROP INDEX IF EXISTS ix_admin_reauthentication_grants_lifecycle;
DROP TABLE IF EXISTS admin_reauthentication_grants;

DROP INDEX IF EXISTS ix_account_governance_expiry_jobs_due;
DROP TABLE IF EXISTS account_governance_expiry_jobs;

DROP INDEX IF EXISTS ix_account_appeal_oauth_states_lifecycle;
DROP TABLE IF EXISTS account_appeal_oauth_states;

DROP INDEX IF EXISTS ix_restricted_business_oauth_states_lifecycle;
DROP TABLE IF EXISTS restricted_business_oauth_states;

DROP INDEX IF EXISTS ix_restricted_business_sessions_lifecycle;
DROP INDEX IF EXISTS ix_restricted_business_sessions_user;
DROP TABLE IF EXISTS restricted_business_sessions;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS fk_users_current_governance_action;

DROP INDEX IF EXISTS ux_account_governance_actions_active_user;
DROP INDEX IF EXISTS ix_account_governance_actions_user_recent;
DROP TABLE IF EXISTS account_governance_actions;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS ck_users_security_lock_shape,
  DROP CONSTRAINT IF EXISTS ck_users_governance_version,
  DROP COLUMN IF EXISTS security_lock_reason,
  DROP COLUMN IF EXISTS security_locked_at,
  DROP COLUMN IF EXISTS current_governance_action_id,
  DROP COLUMN IF EXISTS governance_version;
