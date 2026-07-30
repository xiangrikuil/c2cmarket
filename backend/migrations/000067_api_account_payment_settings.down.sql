-- Remove account-level API payment persistence.
-- 日期：2026-07-30
-- 执行者：Codex

DROP TABLE IF EXISTS api_payment_account_options;
DROP INDEX IF EXISTS ux_api_service_payment_options_one_enabled;
