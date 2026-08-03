-- Roll back dedicated restricted-account appeal sessions.
-- Date: 2026-08-03
-- Author: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM appeals
    WHERE target_type = 'account_governance'
  ) THEN
    RAISE EXCEPTION 'cannot roll back account appeal sessions after account-governance appeals exist'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

DROP INDEX IF EXISTS ux_appeals_submitted_account_governance;

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS ck_appeals_source,
DROP CONSTRAINT IF EXISTS ck_appeals_target_type,
ADD CONSTRAINT ck_appeals_target_type
CHECK (
  target_type IN (
    'contact_snapshot',
    'public_user',
    'carpool_application',
    'carpool_membership',
    'api_purchase_intent',
    'api_order'
  )
),
ADD CONSTRAINT ck_appeals_source
CHECK (report_id IS NOT NULL OR dispute_case_id IS NOT NULL);

DROP INDEX IF EXISTS ix_account_appeal_sessions_lifecycle;
DROP INDEX IF EXISTS ix_account_appeal_sessions_user;
DROP TABLE IF EXISTS account_appeal_sessions;
