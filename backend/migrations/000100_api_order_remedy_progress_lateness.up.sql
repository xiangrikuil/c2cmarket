-- Model remedy fulfillment progress separately from objective and reviewed lateness.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE api_order_dispute_remedies
ADD COLUMN lateness_status text NOT NULL DEFAULT 'not_due',
ADD COLUMN late_at timestamptz,
ADD COLUMN lateness_decided_at timestamptz,
ADD COLUMN lateness_decided_by_admin_id uuid REFERENCES users(id),
ADD COLUMN lateness_reason text NOT NULL DEFAULT '',
ADD COLUMN lateness_reversed_at timestamptz,
ADD COLUMN lateness_reversed_by_admin_id uuid REFERENCES users(id),
ADD COLUMN lateness_reversal_appeal_id uuid REFERENCES appeals(id) ON DELETE RESTRICT,
ADD COLUMN lateness_reversal_reason text NOT NULL DEFAULT '',
ADD COLUMN claimed_late boolean NOT NULL DEFAULT false;

UPDATE api_order_dispute_remedies
SET status = 'cancelled',
    lateness_status = 'late_confirmed',
    late_at = due_at,
    lateness_decided_at = overdue_at,
    lateness_reason = COALESCE(NULLIF(btrim(response_note), ''), '历史整改逾期记录')
WHERE status = 'overdue';

UPDATE api_order_dispute_remedies
SET lateness_status = CASE WHEN claimed_at < due_at THEN 'on_time' ELSE 'late_unreviewed' END,
    late_at = CASE WHEN claimed_at >= due_at THEN due_at ELSE NULL END,
    claimed_late = claimed_at >= due_at
WHERE claimed_at IS NOT NULL;

ALTER TABLE api_order_dispute_remedies
DROP CONSTRAINT IF EXISTS api_order_dispute_remedies_status_check,
DROP CONSTRAINT IF EXISTS api_order_dispute_remedies_check3,
ADD CONSTRAINT api_order_dispute_remedies_status_check CHECK (status IN (
  'pending', 'claimed_fulfilled', 'confirmed', 'contested', 'confirmation_expired', 'cancelled'
)),
ADD CONSTRAINT api_order_dispute_remedies_check3 CHECK (
  (status = 'pending' AND claimed_at IS NULL AND confirmation_due_at IS NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL)
  OR (status = 'claimed_fulfilled' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL)
  OR (status = 'confirmed' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NOT NULL AND contested_at IS NULL AND confirmation_expired_at IS NULL)
  OR (status = 'contested' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NOT NULL AND confirmation_expired_at IS NULL)
  OR (status = 'confirmation_expired' AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL AND confirmed_at IS NULL AND contested_at IS NULL AND confirmation_expired_at IS NOT NULL)
  OR status = 'cancelled'
),
ADD CONSTRAINT ck_api_order_dispute_remedies_lateness_status CHECK (
  lateness_status IN ('not_due', 'on_time', 'late_unreviewed', 'late_confirmed', 'late_excused')
),
ADD CONSTRAINT ck_api_order_dispute_remedies_lateness CHECK (
  (lateness_status = 'not_due' AND late_at IS NULL AND lateness_decided_at IS NULL AND lateness_reason = '' AND lateness_reversed_at IS NULL AND lateness_reversal_appeal_id IS NULL)
  OR (lateness_status = 'on_time' AND claimed_at IS NOT NULL AND claimed_at < due_at AND late_at IS NULL AND lateness_decided_at IS NULL AND lateness_reason = '')
  OR (lateness_status = 'late_unreviewed' AND late_at = due_at AND lateness_decided_at IS NULL AND lateness_reason = '')
  OR (lateness_status IN ('late_confirmed', 'late_excused') AND late_at = due_at AND lateness_decided_at IS NOT NULL AND btrim(lateness_reason) <> '')
),
ADD CONSTRAINT ck_api_order_dispute_remedies_claimed_late CHECK (
  (claimed_at IS NULL AND claimed_late = false)
  OR (claimed_at < due_at AND claimed_late = false)
  OR (claimed_at >= due_at AND claimed_late = true)
);

DROP INDEX IF EXISTS ix_api_order_dispute_remedies_responsible_overdue;

CREATE INDEX ix_api_order_dispute_remedies_lateness_due
ON api_order_dispute_remedies(due_at, id)
WHERE status = 'pending' AND lateness_status = 'not_due';

CREATE INDEX ix_api_order_dispute_remedies_responsible_late_confirmed
ON api_order_dispute_remedies(responsible_user_id, lateness_decided_at DESC, id DESC)
WHERE lateness_status = 'late_confirmed' AND lateness_reversed_at IS NULL;

ALTER TABLE moderation_audit_logs
DROP CONSTRAINT IF EXISTS moderation_audit_logs_action_check,
ADD CONSTRAINT moderation_audit_logs_action_check CHECK (action IN (
  'triage', 'request_info', 'reject', 'open_dispute', 'close', 'resolve', 'approve',
  'mark_overdue', 'confirm_lateness', 'excuse_lateness'
));
