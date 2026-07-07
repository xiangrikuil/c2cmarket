ALTER TABLE api_services
ADD COLUMN quota_expires_at timestamptz;

UPDATE api_services
SET quota_expires_at = updated_at + interval '30 days'
WHERE billing_mode = 'metered_usd_quota'
  AND quota_expires_at IS NULL;

ALTER TABLE api_services
ADD CONSTRAINT ck_api_services_metered_quota_expires_at
CHECK (billing_mode <> 'metered_usd_quota' OR quota_expires_at IS NOT NULL);

CREATE INDEX ix_api_services_quota_expires_at
ON api_services(quota_expires_at)
WHERE billing_mode = 'metered_usd_quota';
