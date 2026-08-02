-- Roll back API-service commercial facts and restore the historical concurrency names.

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS ck_api_services_account_pool_shape,
DROP CONSTRAINT IF EXISTS ck_api_services_account_pool_type,
DROP COLUMN IF EXISTS merchant_refund_commitment,
DROP COLUMN IF EXISTS account_pool_custom_name,
DROP COLUMN IF EXISTS account_pool_type;

ALTER TABLE api_orders
RENAME COLUMN quota_declared_max_concurrency_snapshot
TO quota_recommended_concurrency_snapshot;

ALTER TABLE api_services
RENAME CONSTRAINT ck_api_services_declared_max_concurrency
TO ck_api_services_recommended_concurrency;

ALTER TABLE api_services
RENAME COLUMN declared_max_concurrency TO recommended_concurrency;
