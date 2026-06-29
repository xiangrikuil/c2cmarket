-- Account identity profile and email verification rollback.
-- 日期：2026-06-24
-- 执行者：Codex

DROP INDEX IF EXISTS ix_email_verification_codes_expiry;
DROP INDEX IF EXISTS ix_email_verification_codes_user_email;
DROP TABLE IF EXISTS email_verification_codes;

DROP INDEX IF EXISTS ux_users_verified_email;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_avatar_mode_check;

UPDATE users
SET avatar_mode = 'custom'
WHERE avatar_mode = 'custom_url';

ALTER TABLE users
  ADD CONSTRAINT users_avatar_mode_check
  CHECK (avatar_mode IN ('linuxdo', 'custom'));

ALTER TABLE users
  DROP COLUMN IF EXISTS custom_avatar_url,
  DROP COLUMN IF EXISTS email_verified_at,
  DROP COLUMN IF EXISTS email;
