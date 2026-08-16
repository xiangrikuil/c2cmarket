-- Restore the legacy overdue terminal state only when new lateness facts are absent.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM api_order_dispute_remedies
    WHERE lateness_status IN ('on_time', 'late_unreviewed', 'late_excused')
       OR claimed_late
       OR (lateness_status = 'late_confirmed' AND NOT (status = 'cancelled' AND claimed_at IS NULL AND overdue_at IS NOT NULL))
  ) THEN
    RAISE EXCEPTION 'cannot roll back remedy lateness after new facts exist';
  END IF;
END $$;

ALTER TABLE moderation_audit_logs
DROP CONSTRAINT IF EXISTS moderation_audit_logs_action_check,
ADD CONSTRAINT moderation_audit_logs_action_check CHECK (action IN (
  'triage', 'request_info', 'reject', 'open_dispute', 'close', 'resolve', 'approve', 'mark_overdue'
));

DROP INDEX IF EXISTS ix_api_order_dispute_remedies_responsible_late_confirmed;
DROP INDEX IF EXISTS ix_api_order_dispute_remedies_lateness_due;

ALTER TABLE api_order_dispute_remedies
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedies_claimed_late,
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedies_lateness,
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedies_lateness_status,
DROP CONSTRAINT IF EXISTS api_order_dispute_remedies_check3,
DROP CONSTRAINT IF EXISTS api_order_dispute_remedies_status_check;

UPDATE api_order_dispute_remedies
SET status = 'overdue'
WHERE status = 'cancelled' AND lateness_status = 'late_confirmed' AND claimed_at IS NULL AND overdue_at IS NOT NULL;

ALTER TABLE api_order_dispute_remedies
ADD CONSTRAINT api_order_dispute_remedies_status_check CHECK (status IN (
  'pending', 'claimed_fulfilled', 'confirmed', 'contested', 'confirmation_expired', 'overdue', 'cancelled'
)),
ADD CONSTRAINT api_order_dispute_remedies_check3 CHECK (
  (status = 'pending' AND claimed_at IS NULL AND confirmation_due_at IS NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
  OR (status = 'claimed_fulfilled' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
  OR (status = 'confirmed' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NOT NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
  OR (status = 'contested' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NOT NULL AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
  OR (status = 'confirmation_expired' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NOT NULL AND overdue_at IS NULL)
  OR (status = 'overdue' AND claimed_at IS NULL AND confirmation_due_at IS NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL AND overdue_at IS NOT NULL)
  OR status = 'cancelled'
);

CREATE INDEX ix_api_order_dispute_remedies_responsible_overdue
ON api_order_dispute_remedies(responsible_user_id, overdue_at DESC, id DESC)
WHERE status = 'overdue';

ALTER TABLE api_order_dispute_remedies
DROP COLUMN IF EXISTS claimed_late,
DROP COLUMN IF EXISTS lateness_reversal_reason,
DROP COLUMN IF EXISTS lateness_reversal_appeal_id,
DROP COLUMN IF EXISTS lateness_reversed_by_admin_id,
DROP COLUMN IF EXISTS lateness_reversed_at,
DROP COLUMN IF EXISTS lateness_reason,
DROP COLUMN IF EXISTS lateness_decided_by_admin_id,
DROP COLUMN IF EXISTS lateness_decided_at,
DROP COLUMN IF EXISTS late_at,
DROP COLUMN IF EXISTS lateness_status;
