-- Roll back limited-package inventory and model associations.
-- Date: 2026-07-16
-- Executor: Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_package_expiry,
DROP CONSTRAINT IF EXISTS ck_api_orders_package_stock_reservation,
DROP COLUMN IF EXISTS package_expires_at,
DROP COLUMN IF EXISTS package_stock_reserved;

DROP INDEX IF EXISTS ix_api_service_package_models_service;
DROP TABLE IF EXISTS api_service_package_models;

ALTER TABLE api_service_packages
DROP CONSTRAINT IF EXISTS ck_api_service_packages_limited_fields,
DROP COLUMN IF EXISTS stock_available,
DROP COLUMN IF EXISTS stock_total,
DROP COLUMN IF EXISTS panel_allowance;

ALTER TABLE api_service_models
ADD CONSTRAINT ck_api_service_models_sub2api_multiplier
CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000);
