-- Roll back API order delivery credentials and payment QR snapshots.
-- 日期：2026-07-09
-- 执行者：Codex

DROP INDEX IF EXISTS ix_api_order_delivery_credentials_seller;
DROP INDEX IF EXISTS ix_api_order_delivery_credentials_buyer;
DROP INDEX IF EXISTS ux_api_order_delivery_credentials_order;
DROP TABLE IF EXISTS api_order_delivery_credentials;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_payment_payload_snapshot;

ALTER TABLE api_orders
DROP COLUMN IF EXISTS payment_qr_code_data_url_snapshot;

ALTER TABLE api_orders
ADD CONSTRAINT api_orders_payment_instructions_snapshot_check
CHECK (trim(payment_instructions_snapshot) <> '') NOT VALID;

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS ck_api_service_payment_options_payment_payload;

ALTER TABLE api_service_payment_options
DROP COLUMN IF EXISTS payment_qr_code_data_url;

ALTER TABLE api_service_payment_options
ADD CONSTRAINT api_service_payment_options_payment_instructions_check
CHECK (trim(payment_instructions) <> '') NOT VALID;
