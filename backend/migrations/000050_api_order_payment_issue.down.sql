-- Roll back recoverable payment-verification issues.
-- 日期：2026-07-12
-- 执行者：Codex

DROP INDEX IF EXISTS ix_api_orders_payment_issue_buyer;

UPDATE api_orders
SET status = 'payment_submitted',
    payment_issue_reason = NULL,
    payment_issue_note = NULL,
    payment_issue_reported_at = NULL
WHERE status = 'payment_issue';

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_state_shape,
DROP CONSTRAINT IF EXISTS ck_api_orders_status;

ALTER TABLE api_orders
DROP COLUMN payment_issue_reason,
DROP COLUMN payment_issue_note,
DROP COLUMN payment_issue_reported_at;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_status
CHECK (status IN ('pending_payment', 'payment_submitted', 'paid_confirmed', 'delivery_submitted', 'completed', 'cancelled'));

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_state_shape
CHECK (
  (status = 'pending_payment' AND payment_summary IS NULL AND payment_submitted_at IS NULL AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
  OR (status = 'payment_submitted' AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
  OR (status = 'paid_confirmed' AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL AND paid_confirmed_at IS NOT NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
  OR (status = 'delivery_submitted' AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
  OR (status = 'completed' AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL AND completed_at IS NOT NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
  OR (status = 'cancelled' AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL AND completed_at IS NULL)
);
