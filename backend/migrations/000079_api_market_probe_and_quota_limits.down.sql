-- Remove API quota usage policies and platform-operated service probe facts.
-- Date: 2026-08-04
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_service_probe_configs)
    OR EXISTS (SELECT 1 FROM api_service_probe_authorization_events)
    OR EXISTS (SELECT 1 FROM api_service_probe_samples) THEN
    RAISE EXCEPTION 'cannot roll back migration 79 while API probe data exists';
  END IF;

  IF EXISTS (
    SELECT 1 FROM api_services
    WHERE five_hour_limit_mode <> 'unspecified'
       OR five_hour_limit_usd IS NOT NULL
       OR daily_limit_mode <> 'unspecified'
       OR daily_limit_usd IS NOT NULL
  ) OR EXISTS (
    SELECT 1 FROM api_service_packages
    WHERE five_hour_limit_mode <> 'unspecified'
       OR five_hour_limit_usd IS NOT NULL
       OR daily_limit_mode <> 'unspecified'
       OR daily_limit_usd IS NOT NULL
  ) OR EXISTS (
    SELECT 1 FROM api_quota_offers
    WHERE five_hour_limit_mode <> 'unspecified'
       OR five_hour_limit_usd IS NOT NULL
       OR daily_limit_mode <> 'unspecified'
       OR daily_limit_usd IS NOT NULL
  ) OR EXISTS (
    SELECT 1 FROM api_purchase_intents
    WHERE five_hour_limit_mode_snapshot <> 'unspecified'
       OR five_hour_limit_usd_snapshot IS NOT NULL
       OR daily_limit_mode_snapshot <> 'unspecified'
       OR daily_limit_usd_snapshot IS NOT NULL
  ) OR EXISTS (
    SELECT 1 FROM api_orders
    WHERE five_hour_limit_mode_snapshot <> 'unspecified'
       OR five_hour_limit_usd_snapshot IS NOT NULL
       OR daily_limit_mode_snapshot <> 'unspecified'
       OR daily_limit_usd_snapshot IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 79 while API quota usage policy data exists';
  END IF;
END $$;

DROP TABLE api_service_probe_samples;
DROP TABLE api_service_probe_authorization_events;
DROP TABLE api_service_probe_configs;

ALTER TABLE api_orders
DROP CONSTRAINT ck_api_orders_daily_limit_snapshot,
DROP CONSTRAINT ck_api_orders_five_hour_limit_snapshot,
DROP COLUMN daily_limit_usd_snapshot,
DROP COLUMN daily_limit_mode_snapshot,
DROP COLUMN five_hour_limit_usd_snapshot,
DROP COLUMN five_hour_limit_mode_snapshot;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT ck_api_purchase_intents_daily_limit_snapshot,
DROP CONSTRAINT ck_api_purchase_intents_five_hour_limit_snapshot,
DROP COLUMN daily_limit_usd_snapshot,
DROP COLUMN daily_limit_mode_snapshot,
DROP COLUMN five_hour_limit_usd_snapshot,
DROP COLUMN five_hour_limit_mode_snapshot;

ALTER TABLE api_quota_offers
DROP CONSTRAINT ck_api_quota_offers_daily_limit,
DROP CONSTRAINT ck_api_quota_offers_five_hour_limit,
DROP COLUMN daily_limit_usd,
DROP COLUMN daily_limit_mode,
DROP COLUMN five_hour_limit_usd,
DROP COLUMN five_hour_limit_mode;

ALTER TABLE api_service_packages
DROP CONSTRAINT ck_api_service_packages_daily_limit,
DROP CONSTRAINT ck_api_service_packages_five_hour_limit,
DROP COLUMN daily_limit_usd,
DROP COLUMN daily_limit_mode,
DROP COLUMN five_hour_limit_usd,
DROP COLUMN five_hour_limit_mode;

ALTER TABLE api_services
DROP CONSTRAINT ck_api_services_daily_limit,
DROP CONSTRAINT ck_api_services_five_hour_limit,
DROP COLUMN daily_limit_usd,
DROP COLUMN daily_limit_mode,
DROP COLUMN five_hour_limit_usd,
DROP COLUMN five_hour_limit_mode;
