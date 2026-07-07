DROP INDEX IF EXISTS ix_api_services_quota_expires_at;

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS ck_api_services_metered_quota_expires_at;

ALTER TABLE api_services
DROP COLUMN IF EXISTS quota_expires_at;
