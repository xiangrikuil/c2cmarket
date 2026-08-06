-- 回滚联系人密文格式标记。
-- 日期：2026-07-26
-- 执行者：Codex

ALTER TABLE api_quota_credentials
DROP CONSTRAINT IF EXISTS ck_api_quota_credentials_encryption_format,
DROP COLUMN IF EXISTS secret_encryption_format;

ALTER TABLE api_order_delivery_credentials
DROP CONSTRAINT IF EXISTS ck_api_order_delivery_credentials_encryption_format,
DROP COLUMN IF EXISTS secret_encryption_format;

ALTER TABLE model_audit_targets
DROP CONSTRAINT IF EXISTS ck_model_audit_targets_encryption_format,
DROP COLUMN IF EXISTS api_key_encryption_format;

ALTER TABLE contact_method_versions
DROP CONSTRAINT IF EXISTS ck_contact_method_versions_encryption_format,
DROP COLUMN IF EXISTS encryption_format;
