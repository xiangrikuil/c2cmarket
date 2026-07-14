-- Add a recoverable merchant payment-verification issue state.
-- 日期：2026-07-12
-- 执行者：Codex

ALTER TABLE api_orders
ADD COLUMN payment_issue_reason text,
ADD COLUMN payment_issue_note text,
ADD COLUMN payment_issue_reported_at timestamptz;

DO $$
DECLARE constraint_row record;
BEGIN
  FOR constraint_row IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'api_orders'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%pending_payment%'
  LOOP
    EXECUTE format('ALTER TABLE api_orders DROP CONSTRAINT %I', constraint_row.conname);
  END LOOP;
END $$;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_status
CHECK (status IN (
  'pending_payment', 'payment_submitted', 'payment_issue', 'paid_confirmed',
  'delivery_submitted', 'completed', 'cancelled'
));

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_state_shape
CHECK (
  (
    status = 'pending_payment'
    AND payment_summary IS NULL AND payment_submitted_at IS NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'payment_submitted'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'payment_issue'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IN ('not_received', 'amount_mismatch', 'remark_mismatch')
    AND payment_issue_reported_at IS NOT NULL
    AND paid_confirmed_at IS NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'paid_confirmed'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NULL AND delivery_submitted_at IS NULL
    AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'delivery_submitted'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL
    AND completed_at IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'completed'
    AND payment_summary IS NOT NULL AND payment_submitted_at IS NOT NULL
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND paid_confirmed_at IS NOT NULL AND delivery_note IS NOT NULL AND delivery_submitted_at IS NOT NULL
    AND completed_at IS NOT NULL AND cancelled_at IS NULL AND cancel_reason IS NULL
  ) OR (
    status = 'cancelled'
    AND payment_issue_reason IS NULL AND payment_issue_note IS NULL AND payment_issue_reported_at IS NULL
    AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL AND completed_at IS NULL
  )
);

CREATE INDEX ix_api_orders_payment_issue_buyer
ON api_orders(buyer_user_id, updated_at DESC)
WHERE status = 'payment_issue';
