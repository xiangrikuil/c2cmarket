-- Roll back registered-user growth facts.
-- Date: 2026-08-02
-- Executor: Codex

DROP TRIGGER IF EXISTS trg_api_service_first_published_at ON api_services;
DROP FUNCTION IF EXISTS preserve_api_service_first_published_at();

DROP TRIGGER IF EXISTS trg_carpool_first_published_at ON carpool_listings;
DROP FUNCTION IF EXISTS preserve_carpool_first_published_at();

DROP INDEX IF EXISTS ix_api_services_owner_first_published;
DROP INDEX IF EXISTS ix_carpool_listings_owner_first_published;

ALTER TABLE api_services
DROP COLUMN IF EXISTS first_published_at;

ALTER TABLE carpool_listings
DROP COLUMN IF EXISTS first_published_at;

DROP TABLE IF EXISTS user_activity_daily;
DROP TABLE IF EXISTS user_registration_attributions;

ALTER TABLE users
DROP CONSTRAINT IF EXISTS uq_users_analytics_user_id,
DROP COLUMN IF EXISTS analytics_user_id;
