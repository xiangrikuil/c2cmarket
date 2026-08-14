-- 回滚 API 订单上线前收口字段。
-- 日期：2026-08-14
-- 执行者：Codex

DROP INDEX IF EXISTS ix_api_quota_sale_rounds_confirmation;

ALTER TABLE api_quota_sale_rounds
  DROP CONSTRAINT IF EXISTS ck_api_quota_sale_rounds_fulfillment_confirmation,
  DROP COLUMN IF EXISTS fulfillment_confirmed_at;

DROP INDEX IF EXISTS ix_api_orders_seller_late_payment;
DROP INDEX IF EXISTS ix_api_orders_buyer_pending_capacity;

ALTER TABLE api_orders
  DROP CONSTRAINT IF EXISTS ck_api_orders_late_payment,
  DROP CONSTRAINT IF EXISTS ck_api_orders_delivery_due,
  DROP CONSTRAINT IF EXISTS ck_api_orders_merchant_confirm_due,
  DROP COLUMN IF EXISTS late_payment_resolved_at,
  DROP COLUMN IF EXISTS late_payment_note,
  DROP COLUMN IF EXISTS late_payment_reported_at,
  DROP COLUMN IF EXISTS late_payment_status,
  DROP COLUMN IF EXISTS delivery_due_at,
  DROP COLUMN IF EXISTS merchant_confirm_due_at;
