-- Persist merchant/delivery overdue facts and an idempotent final-three-minute delivery reminder.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE api_orders
ADD COLUMN merchant_confirm_overdue_at timestamptz,
ADD COLUMN delivery_overdue_at timestamptz,
ADD COLUMN delivery_due_reminded_at timestamptz;

UPDATE api_orders
SET merchant_confirm_overdue_at = merchant_confirm_due_at
WHERE merchant_confirm_due_at IS NOT NULL
  AND paid_confirmed_at IS NOT NULL
  AND paid_confirmed_at >= merchant_confirm_due_at;

UPDATE api_orders
SET delivery_overdue_at = delivery_due_at
WHERE delivery_due_at IS NOT NULL
  AND delivery_submitted_at IS NOT NULL
  AND delivery_submitted_at >= delivery_due_at;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_merchant_confirm_overdue_fact CHECK (
  merchant_confirm_overdue_at IS NULL
  OR (merchant_confirm_due_at IS NOT NULL AND merchant_confirm_overdue_at >= merchant_confirm_due_at)
),
ADD CONSTRAINT ck_api_orders_delivery_overdue_fact CHECK (
  delivery_overdue_at IS NULL
  OR (delivery_due_at IS NOT NULL AND delivery_overdue_at >= delivery_due_at)
),
ADD CONSTRAINT ck_api_orders_delivery_due_reminder CHECK (
  delivery_due_reminded_at IS NULL
  OR (
    paid_confirmed_at IS NOT NULL
    AND delivery_due_at IS NOT NULL
    AND delivery_due_reminded_at >= delivery_due_at - interval '3 minutes'
    AND delivery_due_reminded_at < delivery_due_at
  )
);

CREATE INDEX ix_api_orders_merchant_confirm_overdue_due
ON api_orders(merchant_confirm_due_at, id)
WHERE status = 'payment_submitted' AND merchant_confirm_overdue_at IS NULL;

CREATE INDEX ix_api_orders_delivery_overdue_due
ON api_orders(delivery_due_at, id)
WHERE status = 'paid_confirmed' AND delivery_overdue_at IS NULL;

CREATE INDEX ix_api_orders_delivery_due_reminder
ON api_orders(delivery_due_at, id)
WHERE status = 'paid_confirmed'
  AND dispute_status = 'none'
  AND delivery_overdue_at IS NULL
  AND delivery_due_reminded_at IS NULL;
