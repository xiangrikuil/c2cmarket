-- Roll back independent transaction-contact email verification.
-- Date: 2026-08-16
-- Author: Codex

DELETE FROM email_verification_codes
WHERE purpose = 'contact_email';

DROP INDEX IF EXISTS ux_email_verification_codes_active_contact_email;

ALTER TABLE email_verification_codes
  DROP CONSTRAINT IF EXISTS fk_email_verification_contact_version,
  DROP CONSTRAINT IF EXISTS fk_email_verification_contact_method,
  DROP CONSTRAINT IF EXISTS ck_email_verification_codes_purpose_shape,
  DROP CONSTRAINT IF EXISTS email_verification_codes_purpose_check,
  DROP COLUMN IF EXISTS contact_method_version_id,
  DROP COLUMN IF EXISTS contact_method_id;

ALTER TABLE email_verification_codes
  ADD CONSTRAINT email_verification_codes_purpose_check
  CHECK (purpose IN ('bind_email', 'password_reset', 'email_registration'));
