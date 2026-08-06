-- Roll back immutable public API order numbers.
-- Date: 2026-08-02
-- Executor: Codex

DROP TRIGGER IF EXISTS trg_api_orders_order_no_immutable ON api_orders;
DROP FUNCTION IF EXISTS preserve_api_order_no();

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ux_api_orders_order_no,
DROP CONSTRAINT IF EXISTS ck_api_orders_order_no_format,
DROP COLUMN IF EXISTS order_no;
