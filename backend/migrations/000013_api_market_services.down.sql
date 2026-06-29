-- Roll back API market service publishing and review contract.
-- 日期：2026-06-22
-- 执行者：Codex

DROP TABLE IF EXISTS api_service_packages;
DROP TABLE IF EXISTS api_service_models;
DROP TABLE IF EXISTS api_service_access_modes;
DROP TABLE IF EXISTS api_services;
DROP TABLE IF EXISTS api_model_price_versions;
DROP TABLE IF EXISTS api_model_catalog;
DROP TABLE IF EXISTS api_model_providers;

ALTER TABLE merchant_profiles
DROP CONSTRAINT IF EXISTS uq_merchant_profiles_id_owner;
