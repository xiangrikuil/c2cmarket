-- Remove deadline facts only when no durable fact would be lost.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM api_orders
    WHERE merchant_confirm_overdue_at IS NOT NULL
       OR delivery_overdue_at IS NOT NULL
       OR delivery_due_reminded_at IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot roll back deadline facts after facts have been recorded';
  END IF;
END $$;

DROP INDEX IF EXISTS ix_api_orders_delivery_due_reminder;
DROP INDEX IF EXISTS ix_api_orders_delivery_overdue_due;
DROP INDEX IF EXISTS ix_api_orders_merchant_confirm_overdue_due;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_delivery_due_reminder,
DROP CONSTRAINT IF EXISTS ck_api_orders_delivery_overdue_fact,
DROP CONSTRAINT IF EXISTS ck_api_orders_merchant_confirm_overdue_fact,
DROP COLUMN IF EXISTS delivery_due_reminded_at,
DROP COLUMN IF EXISTS delivery_overdue_at,
DROP COLUMN IF EXISTS merchant_confirm_overdue_at;
