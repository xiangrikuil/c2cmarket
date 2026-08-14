-- Purpose-isolated password reset challenge uniqueness.
-- Date: 2026-08-14
-- Author: Codex

-- No released application owns password-reset challenges yet. Invalidate any
-- manually inserted or legacy rows before enforcing the one-active-row rule.
UPDATE email_verification_codes
SET consumed_at = now()
WHERE purpose = 'password_reset'
  AND consumed_at IS NULL;

CREATE UNIQUE INDEX ux_email_verification_codes_active_password_reset
ON email_verification_codes(user_id, email)
WHERE purpose = 'password_reset' AND consumed_at IS NULL;
