-- Roll back only before any durable student identity has been assigned.
-- Date: 2026-08-12
-- Author: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM student_email_claims) THEN
    RAISE EXCEPTION 'cannot roll back durable student identity after claims exist';
  END IF;
END;
$$;

DROP INDEX IF EXISTS ux_auth_sessions_oauth_link_state_hash;

ALTER TABLE auth_sessions
  DROP CONSTRAINT IF EXISTS ck_auth_sessions_oauth_link_state_shape,
  DROP CONSTRAINT IF EXISTS ck_auth_sessions_password_reauthenticated_at,
  DROP COLUMN IF EXISTS oauth_link_state_consumed_at,
  DROP COLUMN IF EXISTS oauth_link_state_expires_at,
  DROP COLUMN IF EXISTS oauth_link_state_purpose,
  DROP COLUMN IF EXISTS oauth_link_state_hash,
  DROP COLUMN IF EXISTS password_reauthenticated_at;

ALTER TABLE email_verification_codes
  DROP CONSTRAINT IF EXISTS ck_email_verification_codes_attempt_limit;

DROP INDEX IF EXISTS ux_email_verification_codes_active_registration_email;

CREATE INDEX ix_email_verification_codes_registration_email
ON email_verification_codes(email, created_at DESC)
WHERE purpose = 'email_registration' AND consumed_at IS NULL;

DROP TRIGGER IF EXISTS trg_users_student_claimed_profile_email ON users;
DROP FUNCTION IF EXISTS reject_cross_user_student_claimed_profile_email();

DROP TRIGGER IF EXISTS trg_student_email_claim_append_only ON student_email_claims;
DROP FUNCTION IF EXISTS reject_student_email_claim_change();

DROP TRIGGER IF EXISTS trg_student_institution_domain_identity_immutable ON student_institution_domains;
DROP FUNCTION IF EXISTS reject_student_institution_domain_identity_change();

DROP TABLE IF EXISTS student_email_claims;
DROP TABLE IF EXISTS student_institution_domains;
DROP TABLE IF EXISTS student_registration_settings;
