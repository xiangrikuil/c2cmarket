-- Roll back API model provider catalog.
-- 日期：2026-06-29
-- 执行者：Codex

ALTER TABLE api_model_catalog
ADD COLUMN IF NOT EXISTS provider_category text,
ADD COLUMN IF NOT EXISTS provider text;

UPDATE api_model_catalog catalog
SET provider_category = provider.provider_category,
    provider = provider.display_name
FROM api_model_providers provider
WHERE provider.id = catalog.provider_id;

ALTER TABLE api_model_catalog
ALTER COLUMN provider_category SET NOT NULL,
ALTER COLUMN provider SET NOT NULL;

ALTER TABLE api_model_catalog
DROP CONSTRAINT IF EXISTS api_model_catalog_provider_id_fkey;

ALTER TABLE api_model_catalog
DROP COLUMN IF EXISTS provider_id;

DROP TABLE IF EXISTS api_model_providers;
