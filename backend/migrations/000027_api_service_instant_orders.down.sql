-- Roll back API service instant order layer.
-- 日期：2026-06-24
-- 执行者：Codex

DROP TABLE IF EXISTS api_order_payment_instruction_access_logs;
DROP TABLE IF EXISTS api_order_events;
DROP TABLE IF EXISTS api_orders;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS chk_api_purchase_intents_seller_quote,
DROP COLUMN IF EXISTS seller_quote_version,
DROP COLUMN IF EXISTS seller_quote_expires_at,
DROP COLUMN IF EXISTS seller_quoted_at,
DROP COLUMN IF EXISTS seller_quote_note,
DROP COLUMN IF EXISTS seller_quoted_currency,
DROP COLUMN IF EXISTS seller_quoted_amount,
DROP COLUMN IF EXISTS seller_quote_status;

DROP TABLE IF EXISTS api_service_payment_options;

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS chk_api_services_payment_window,
DROP COLUMN IF EXISTS payment_window_minutes,
DROP COLUMN IF EXISTS accepting_orders;
