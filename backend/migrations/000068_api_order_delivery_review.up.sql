-- Add the buyer delivery-review window and explicit completion source.
-- 日期：2026-08-02
-- 执行者：Codex

ALTER TABLE api_orders
ADD COLUMN delivery_review_expires_at timestamptz,
ADD COLUMN delivery_review_reminded_at timestamptz,
ADD COLUMN completion_source text;

UPDATE api_orders
SET delivery_review_expires_at = now() + interval '24 hours'
WHERE status = 'delivery_submitted';

UPDATE api_orders
SET delivery_review_expires_at = delivery_submitted_at + interval '24 hours',
    completion_source = 'buyer_confirmed'
WHERE status = 'completed';

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_state_shape;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_completion_source
CHECK (completion_source IS NULL OR completion_source IN ('buyer_confirmed', 'auto_completed')),
ADD CONSTRAINT ck_api_orders_state_shape
CHECK (
  (
    status = 'pending_payment'
    AND payment_summary IS NULL AND payment_submitted_at IS NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'payment_submitted'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'payment_issue'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IN ('not_received', 'amount_mismatch', 'remark_mismatch')
    AND payment_issue_reported_at IS NOT NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'paid_confirmed'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'delivery_submitted'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL
    AND delivery_review_expires_at IS NOT NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'completed'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL
    AND delivery_review_expires_at IS NOT NULL
    AND completion_source IN ('buyer_confirmed', 'auto_completed') AND completed_at IS NOT NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'cancelled'
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL
  )
);

CREATE INDEX ix_api_orders_delivery_review_expiry
ON api_orders(delivery_review_expires_at, id)
WHERE status = 'delivery_submitted';
