-- Account identity profile and email verification contract.
-- 日期：2026-06-24
-- 执行者：Codex

ALTER TABLE users
  ADD COLUMN email text,
  ADD COLUMN email_verified_at timestamptz,
  ADD COLUMN custom_avatar_url text;

UPDATE users
SET avatar_mode = 'custom_url',
    custom_avatar_url = avatar_url
WHERE avatar_mode = 'custom';

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_avatar_mode_check;

ALTER TABLE users
  ADD CONSTRAINT users_avatar_mode_check
  CHECK (avatar_mode IN ('linuxdo', 'custom_url'));

CREATE UNIQUE INDEX ux_users_verified_email
ON users(lower(email))
WHERE email_verified_at IS NOT NULL;

CREATE TABLE email_verification_codes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email text NOT NULL,
  purpose text NOT NULL CHECK (purpose IN ('bind_email', 'password_reset')),
  code_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  attempt_count integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (email = lower(email)),
  CHECK (expires_at > created_at),
  CHECK (attempt_count >= 0)
);

CREATE INDEX ix_email_verification_codes_user_email
ON email_verification_codes(user_id, email, purpose, created_at DESC);

CREATE INDEX ix_email_verification_codes_expiry
ON email_verification_codes(expires_at)
WHERE consumed_at IS NULL;
