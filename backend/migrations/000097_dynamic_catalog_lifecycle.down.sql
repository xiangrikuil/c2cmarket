-- Guarded rollback for the dynamic catalog lifecycle.
-- 日期：2026-08-14
-- 执行者：Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_order_catalog_risk_holds) THEN
    RAISE EXCEPTION 'cannot roll back dynamic catalog lifecycle after risk holds were created';
  END IF;
  IF EXISTS (
    SELECT 1 FROM product_categories WHERE status = 'blocked'
    UNION ALL SELECT 1 FROM product_plans WHERE status = 'blocked'
    UNION ALL SELECT 1 FROM api_model_providers WHERE status = 'blocked'
    UNION ALL SELECT 1 FROM api_model_catalog WHERE status = 'blocked'
  ) THEN
    RAISE EXCEPTION 'cannot roll back dynamic catalog lifecycle while blocked records exist';
  END IF;
  IF EXISTS (
    SELECT 1 FROM product_categories
    WHERE core_key IS NOT NULL AND NOT (code = core_key AND core_key IN ('gpt', 'claude', 'grok'))
    UNION ALL
    SELECT 1 FROM product_plans
    WHERE core_key IS NOT NULL AND NOT (
      (core_key = 'gpt' AND slug = 'chatgpt-pro-20x-web') OR
      (core_key = 'claude' AND slug = 'claude-pro') OR
      (core_key = 'grok' AND slug = 'grok-premium')
    )
    UNION ALL
    SELECT 1 FROM api_model_providers
    WHERE core_key IS NOT NULL AND NOT (
      (core_key = 'gpt' AND code = 'openai') OR
      (core_key = 'claude' AND code = 'anthropic') OR
      (core_key = 'grok' AND code = 'xai')
    )
    UNION ALL
    SELECT 1 FROM api_model_catalog
    WHERE core_key IS NOT NULL AND NOT (
      (core_key = 'gpt' AND model_key = 'gpt-4.1') OR
      (core_key = 'claude' AND model_key = 'claude-sonnet-4') OR
      (core_key = 'grok' AND model_key = 'grok-4')
    )
  ) THEN
    RAISE EXCEPTION 'cannot roll back dynamic catalog lifecycle after core identity assignments changed';
  END IF;
END $$;

DROP TABLE api_order_catalog_risk_holds;

DROP INDEX IF EXISTS ux_api_model_catalog_core_key;
DROP INDEX IF EXISTS ux_api_model_providers_core_key;
DROP INDEX IF EXISTS ux_product_plans_core_key;
DROP INDEX IF EXISTS ux_product_categories_core_key;
DROP INDEX IF EXISTS ux_api_model_providers_code_lower;
DROP INDEX IF EXISTS ux_product_categories_code_lower;

ALTER TABLE product_categories DROP CONSTRAINT ck_product_categories_code;
ALTER TABLE api_model_providers DROP CONSTRAINT ck_api_model_providers_code;

ALTER TABLE product_categories ADD COLUMN active boolean NOT NULL DEFAULT true;
ALTER TABLE product_plans ADD COLUMN active boolean NOT NULL DEFAULT true;
ALTER TABLE api_model_providers ADD COLUMN active boolean NOT NULL DEFAULT true;
ALTER TABLE api_model_catalog ADD COLUMN active boolean NOT NULL DEFAULT true;

UPDATE product_categories SET active = status = 'active';
UPDATE product_plans SET active = status = 'active';
UPDATE api_model_providers SET active = status = 'active';
UPDATE api_model_catalog SET active = status = 'active';

-- The historical provider-category enum has no Grok member. Preserve xAI as
-- an open-code provider under the old generic category during local rollback.
UPDATE api_model_providers SET provider_category = 'other' WHERE code = 'xai';

ALTER TABLE product_categories
  DROP COLUMN version, DROP COLUMN status_changed_by, DROP COLUMN status_reason,
  DROP COLUMN status_changed_at, DROP COLUMN status, DROP COLUMN core_key;
ALTER TABLE product_plans
  DROP COLUMN version, DROP COLUMN status_changed_by, DROP COLUMN status_reason,
  DROP COLUMN status_changed_at, DROP COLUMN status, DROP COLUMN core_key;
ALTER TABLE api_model_providers
  DROP COLUMN version, DROP COLUMN status_changed_by, DROP COLUMN status_reason,
  DROP COLUMN status_changed_at, DROP COLUMN status, DROP COLUMN core_key;
ALTER TABLE api_model_catalog
  DROP COLUMN version, DROP COLUMN status_changed_by, DROP COLUMN status_reason,
  DROP COLUMN status_changed_at, DROP COLUMN status, DROP COLUMN core_key;

ALTER TABLE api_model_providers
  ADD CONSTRAINT api_model_providers_provider_category_check
  CHECK (provider_category IN ('gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other'));
