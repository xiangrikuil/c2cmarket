-- Email registration verification contract.
-- 日期：2026-06-26
-- 执行者：Codex

ALTER TABLE email_verification_codes
  ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE email_verification_codes
  DROP CONSTRAINT IF EXISTS email_verification_codes_purpose_check;

ALTER TABLE email_verification_codes
  ADD CONSTRAINT email_verification_codes_purpose_check
  CHECK (purpose IN ('bind_email', 'password_reset', 'email_registration'));

CREATE INDEX ix_email_verification_codes_registration_email
ON email_verification_codes(email, created_at DESC)
WHERE purpose = 'email_registration' AND consumed_at IS NULL;
