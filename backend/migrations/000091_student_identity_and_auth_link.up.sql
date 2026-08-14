-- Durable student identity, purpose-isolated registration, and linux.do link state.
-- Date: 2026-08-12
-- Author: Codex

CREATE TABLE student_registration_settings (
  singleton_key text PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  updated_by_admin_id uuid REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_student_registration_settings_singleton
    CHECK (singleton_key = 'global')
);

INSERT INTO student_registration_settings (singleton_key, enabled)
VALUES ('global', false);

CREATE TABLE student_institution_domains (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain text NOT NULL UNIQUE,
  institution_name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_by_admin_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  updated_by_admin_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_student_institution_domain_canonical CHECK (
    domain = lower(domain)
    AND octet_length(domain) BETWEEN 3 AND 253
    AND domain ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$'
  ),
  CONSTRAINT ck_student_institution_name_nonempty CHECK (
    institution_name = btrim(institution_name)
    AND char_length(institution_name) BETWEEN 1 AND 120
  )
);

CREATE TABLE student_email_claims (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
  normalized_email text NOT NULL UNIQUE,
  institution_domain_id uuid NOT NULL REFERENCES student_institution_domains(id) ON DELETE RESTRICT,
  claimed_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_student_email_claim_canonical CHECK (
    normalized_email = lower(normalized_email)
    AND normalized_email = btrim(normalized_email)
    AND normalized_email ~ '^[^[:space:]@]+@[^[:space:]@]+$'
  )
);

CREATE FUNCTION reject_student_institution_domain_identity_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'student institution domains are immutable';
  END IF;
  IF NEW.domain IS DISTINCT FROM OLD.domain THEN
    RAISE EXCEPTION 'student institution domain identity is immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_student_institution_domain_identity_immutable
BEFORE UPDATE OR DELETE ON student_institution_domains
FOR EACH ROW
EXECUTE FUNCTION reject_student_institution_domain_identity_change();

CREATE FUNCTION reject_student_email_claim_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'student email claims are append-only';
END;
$$;

CREATE TRIGGER trg_student_email_claim_append_only
BEFORE UPDATE OR DELETE ON student_email_claims
FOR EACH ROW
EXECUTE FUNCTION reject_student_email_claim_change();

CREATE FUNCTION reject_cross_user_student_claimed_profile_email()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.email_verified_at IS NOT NULL
     AND NEW.email IS NOT NULL
     AND EXISTS (
       SELECT 1
       FROM student_email_claims claim
       WHERE claim.normalized_email = lower(btrim(NEW.email))
         AND claim.user_id <> NEW.id
     ) THEN
    RAISE EXCEPTION USING
      ERRCODE = '23505',
      CONSTRAINT = 'ux_student_email_claims_normalized_email',
      MESSAGE = 'verified profile email belongs to another student claim';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_student_claimed_profile_email
BEFORE INSERT OR UPDATE OF email, email_verified_at ON users
FOR EACH ROW
EXECUTE FUNCTION reject_cross_user_student_claimed_profile_email();

-- Registration challenges written before the purpose-bound HMAC contract are
-- terminally invalidated. A replacement request creates the only active row.
UPDATE email_verification_codes
SET consumed_at = now()
WHERE purpose IN ('email_registration', 'bind_email')
  AND consumed_at IS NULL;

DROP INDEX IF EXISTS ix_email_verification_codes_registration_email;

CREATE UNIQUE INDEX ux_email_verification_codes_active_registration_email
ON email_verification_codes(email)
WHERE purpose = 'email_registration' AND consumed_at IS NULL;

ALTER TABLE email_verification_codes
  ADD CONSTRAINT ck_email_verification_codes_attempt_limit
  CHECK (attempt_count BETWEEN 0 AND 5);

ALTER TABLE auth_sessions
  ADD COLUMN password_reauthenticated_at timestamptz,
  ADD COLUMN oauth_link_state_hash text,
  ADD COLUMN oauth_link_state_purpose text,
  ADD COLUMN oauth_link_state_expires_at timestamptz,
  ADD COLUMN oauth_link_state_consumed_at timestamptz,
  ADD CONSTRAINT ck_auth_sessions_password_reauthenticated_at CHECK (
    password_reauthenticated_at IS NULL
    OR password_reauthenticated_at >= created_at
  ),
  ADD CONSTRAINT ck_auth_sessions_oauth_link_state_shape CHECK (
    (
      oauth_link_state_hash IS NULL
      AND oauth_link_state_purpose IS NULL
      AND oauth_link_state_expires_at IS NULL
      AND oauth_link_state_consumed_at IS NULL
    )
    OR (
      oauth_link_state_hash IS NOT NULL
      AND oauth_link_state_purpose = 'link_linuxdo'
      AND oauth_link_state_expires_at IS NOT NULL
      AND oauth_link_state_expires_at > created_at
      AND (
        oauth_link_state_consumed_at IS NULL
        OR oauth_link_state_consumed_at >= created_at
      )
    )
  );

CREATE UNIQUE INDEX ux_auth_sessions_oauth_link_state_hash
ON auth_sessions(oauth_link_state_hash)
WHERE oauth_link_state_hash IS NOT NULL;
