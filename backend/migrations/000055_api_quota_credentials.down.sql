-- 回退限时 API 额度包的加密凭据预导入库存。
-- 日期：2026-07-19
-- 执行者：Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_quota_credential_selection,
DROP CONSTRAINT IF EXISTS fk_api_orders_quota_credential,
DROP COLUMN IF EXISTS api_quota_credential_id;

DROP INDEX IF EXISTS ix_api_quota_credentials_offer_status;
DROP INDEX IF EXISTS ix_api_quota_credentials_available;
DROP INDEX IF EXISTS ux_api_quota_credentials_reserved_order;
DROP INDEX IF EXISTS ux_api_quota_credentials_seller_fingerprint;
DROP TABLE IF EXISTS api_quota_credentials;
