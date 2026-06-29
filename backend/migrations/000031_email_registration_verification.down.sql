-- Email registration verification rollback.
-- 日期：2026-06-26
-- 执行者：Codex

DELETE FROM email_verification_codes
WHERE purpose = 'email_registration';

DROP INDEX IF EXISTS ix_email_verification_codes_registration_email;

ALTER TABLE email_verification_codes
  ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE email_verification_codes
  DROP CONSTRAINT IF EXISTS email_verification_codes_purpose_check;

ALTER TABLE email_verification_codes
  ADD CONSTRAINT email_verification_codes_purpose_check
  CHECK (purpose IN ('bind_email', 'password_reset'));
