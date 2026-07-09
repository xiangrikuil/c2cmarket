-- Remove USDT from API service payment methods.
-- 日期：2026-07-08
-- 执行者：Codex

UPDATE api_service_payment_options
SET enabled = false,
    updated_at = now(),
    version = version + 1
WHERE payment_method = 'usdt'
  AND enabled = true;

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS api_service_payment_options_payment_method_check;

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS ck_api_service_payment_options_payment_method_current;

ALTER TABLE api_service_payment_options
ADD CONSTRAINT ck_api_service_payment_options_payment_method_current
CHECK (payment_method IN ('wechat', 'alipay')) NOT VALID;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS api_orders_selected_payment_method_check;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_selected_payment_method_current;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_selected_payment_method_current
CHECK (selected_payment_method IN ('wechat', 'alipay')) NOT VALID;
