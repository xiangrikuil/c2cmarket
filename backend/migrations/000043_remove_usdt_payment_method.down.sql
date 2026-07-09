-- Restore USDT as an allowed API service payment method.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_selected_payment_method_current;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS api_orders_selected_payment_method_check;

ALTER TABLE api_orders
ADD CONSTRAINT api_orders_selected_payment_method_check
CHECK (selected_payment_method IN ('wechat', 'alipay', 'usdt')) NOT VALID;

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS ck_api_service_payment_options_payment_method_current;

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS api_service_payment_options_payment_method_check;

ALTER TABLE api_service_payment_options
ADD CONSTRAINT api_service_payment_options_payment_method_check
CHECK (payment_method IN ('wechat', 'alipay', 'usdt')) NOT VALID;
