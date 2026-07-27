-- 为所有复用联系人密钥环的密文增加显式格式标记，支持无歧义的 AAD 迁移。
-- 日期：2026-07-26
-- 执行者：Codex

ALTER TABLE contact_method_versions
ADD COLUMN encryption_format text NOT NULL DEFAULT 'legacy_no_aad_v1';

ALTER TABLE contact_method_versions
ADD CONSTRAINT ck_contact_method_versions_encryption_format
CHECK (encryption_format IN ('legacy_no_aad_v1', 'aad_v1'));

ALTER TABLE model_audit_targets
ADD COLUMN api_key_encryption_format text NOT NULL DEFAULT 'legacy_no_aad_v1';

ALTER TABLE model_audit_targets
ADD CONSTRAINT ck_model_audit_targets_encryption_format
CHECK (api_key_encryption_format IN ('legacy_no_aad_v1', 'aad_v1'));

ALTER TABLE api_order_delivery_credentials
ADD COLUMN secret_encryption_format text NOT NULL DEFAULT 'legacy_no_aad_v1';

ALTER TABLE api_order_delivery_credentials
ADD CONSTRAINT ck_api_order_delivery_credentials_encryption_format
CHECK (secret_encryption_format IN ('legacy_no_aad_v1', 'aad_v1'));

ALTER TABLE api_quota_credentials
ADD COLUMN secret_encryption_format text NOT NULL DEFAULT 'legacy_no_aad_v1';

ALTER TABLE api_quota_credentials
ADD CONSTRAINT ck_api_quota_credentials_encryption_format
CHECK (secret_encryption_format IN ('legacy_no_aad_v1', 'aad_v1'));
