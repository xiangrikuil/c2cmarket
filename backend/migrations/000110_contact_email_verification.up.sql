-- Independent transaction-contact email verification.
-- Date: 2026-08-16
-- Author: Codex

ALTER TABLE email_verification_codes
  DROP CONSTRAINT IF EXISTS email_verification_codes_purpose_check;

ALTER TABLE email_verification_codes
  ADD COLUMN contact_method_id uuid,
  ADD COLUMN contact_method_version_id uuid,
  ADD CONSTRAINT email_verification_codes_purpose_check
    CHECK (purpose IN ('bind_email', 'password_reset', 'email_registration', 'contact_email')),
  ADD CONSTRAINT ck_email_verification_codes_purpose_shape
    CHECK (
      (
        purpose = 'contact_email'
        AND user_id IS NOT NULL
        AND contact_method_id IS NOT NULL
        AND contact_method_version_id IS NOT NULL
      )
      OR (
        purpose <> 'contact_email'
        AND contact_method_id IS NULL
        AND contact_method_version_id IS NULL
      )
    ),
  ADD CONSTRAINT fk_email_verification_contact_method
    FOREIGN KEY (contact_method_id, user_id)
    REFERENCES contact_methods(id, user_id)
    ON DELETE CASCADE,
  ADD CONSTRAINT fk_email_verification_contact_version
    FOREIGN KEY (contact_method_version_id, contact_method_id, user_id)
    REFERENCES contact_method_versions(id, contact_method_id, owner_user_id)
    ON DELETE CASCADE;

CREATE UNIQUE INDEX ux_email_verification_codes_active_contact_email
ON email_verification_codes(contact_method_id)
WHERE purpose = 'contact_email' AND consumed_at IS NULL;
