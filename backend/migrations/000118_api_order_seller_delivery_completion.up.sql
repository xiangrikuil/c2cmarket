-- Complete API orders when the seller submits the immutable delivery credential.
-- Date: 2026-08-20
-- Executor: Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_state_shape,
DROP CONSTRAINT IF EXISTS ck_api_orders_completion_source;

UPDATE api_orders
SET status = 'completed',
    completion_source = 'seller_delivered',
    completed_at = COALESCE(delivery_submitted_at, updated_at),
    commercial_outcome = 'normal_fulfillment',
    commercial_outcome_updated_at = COALESCE(delivery_submitted_at, updated_at),
    updated_at = GREATEST(updated_at, COALESCE(delivery_submitted_at, updated_at))
WHERE status = 'delivery_submitted';

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_completion_source
CHECK (completion_source IS NULL OR completion_source IN ('buyer_confirmed', 'auto_completed', 'seller_delivered', 'remedy_confirmed')),
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
    AND completion_source IN ('buyer_confirmed', 'auto_completed', 'seller_delivered', 'remedy_confirmed') AND completed_at IS NOT NULL
    AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'cancelled'
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND delivery_review_expires_at IS NULL AND delivery_review_reminded_at IS NULL
    AND completion_source IS NULL AND completed_at IS NULL
    AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL
  )
);

DROP INDEX IF EXISTS ix_api_orders_delivery_review_expiry;
