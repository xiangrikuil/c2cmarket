-- Unify catalog lifecycle and add Grok/xAI as first-class catalog data.
-- 日期：2026-08-14
-- 执行者：Codex

ALTER TABLE api_model_providers
  DROP CONSTRAINT IF EXISTS api_model_providers_provider_category_check;

ALTER TABLE product_categories
  ADD COLUMN core_key text,
  ADD COLUMN status text NOT NULL DEFAULT 'active',
  ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN status_reason text NOT NULL DEFAULT '',
  ADD COLUMN status_changed_by uuid REFERENCES users(id),
  ADD COLUMN version bigint NOT NULL DEFAULT 1;

ALTER TABLE product_plans
  ADD COLUMN core_key text,
  ADD COLUMN status text NOT NULL DEFAULT 'active',
  ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN status_reason text NOT NULL DEFAULT '',
  ADD COLUMN status_changed_by uuid REFERENCES users(id),
  ADD COLUMN version bigint NOT NULL DEFAULT 1;

ALTER TABLE api_model_providers
  ADD COLUMN core_key text,
  ADD COLUMN status text NOT NULL DEFAULT 'active',
  ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN status_reason text NOT NULL DEFAULT '',
  ADD COLUMN status_changed_by uuid REFERENCES users(id),
  ADD COLUMN version bigint NOT NULL DEFAULT 1;

ALTER TABLE api_model_catalog
  ADD COLUMN core_key text,
  ADD COLUMN status text NOT NULL DEFAULT 'active',
  ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN status_reason text NOT NULL DEFAULT '',
  ADD COLUMN status_changed_by uuid REFERENCES users(id),
  ADD COLUMN version bigint NOT NULL DEFAULT 1;

UPDATE product_categories
SET status = CASE WHEN active THEN 'active' ELSE 'deprecated' END,
    status_reason = CASE WHEN active THEN '' ELSE '由旧停用状态迁移' END;

UPDATE product_plans
SET status = CASE WHEN active THEN 'active' ELSE 'deprecated' END,
    status_reason = CASE WHEN active THEN '' ELSE '由旧停用状态迁移' END;

UPDATE api_model_providers
SET status = CASE WHEN active THEN 'active' ELSE 'deprecated' END,
    status_reason = CASE WHEN active THEN '' ELSE '由旧停用状态迁移' END;

UPDATE api_model_catalog
SET status = CASE WHEN active THEN 'active' ELSE 'deprecated' END,
    status_reason = CASE WHEN active THEN '' ELSE '由旧停用状态迁移' END;

-- Collapse case/whitespace-only development duplicates before normalization.
-- Historical UNIQUE(code) constraints are case-sensitive, so lowercasing first
-- would fail for pairs such as `Grok` and `grok` before targeted merges run.
DO $$
DECLARE
  normalized_code text;
  canonical_id uuid;
  duplicate_id uuid;
BEGIN
  FOR normalized_code IN
    SELECT lower(btrim(code)) FROM product_categories
    GROUP BY lower(btrim(code)) HAVING count(*) > 1
  LOOP
    SELECT id INTO canonical_id FROM product_categories
    WHERE lower(btrim(code)) = normalized_code
    ORDER BY (normalized_code = 'grok' AND id = '00000000-0000-0000-0000-000000000106'::uuid) DESC, id
    LIMIT 1;
    FOR duplicate_id IN
      SELECT id FROM product_categories
      WHERE lower(btrim(code)) = normalized_code AND id <> canonical_id ORDER BY id
    LOOP
      UPDATE product_plans SET category_id = canonical_id WHERE category_id = duplicate_id;
      UPDATE product_categories
      SET code = left(normalized_code, 40) || '-retired-' || left(replace(id::text, '-', ''), 8),
          status = 'deprecated', status_reason = '合并规范化重复目录', active = false
      WHERE id = duplicate_id;
    END LOOP;
  END LOOP;
END $$;

DO $$
DECLARE
  normalized_code text;
  canonical_id uuid;
  duplicate_id uuid;
BEGIN
  FOR normalized_code IN
    SELECT lower(btrim(code)) FROM api_model_providers
    GROUP BY lower(btrim(code)) HAVING count(*) > 1
  LOOP
    SELECT id INTO canonical_id FROM api_model_providers
    WHERE lower(btrim(code)) = normalized_code
    ORDER BY (normalized_code = 'xai' AND id = '00000000-0000-0000-0000-000000000c06'::uuid) DESC, created_at, id
    LIMIT 1;
    FOR duplicate_id IN
      SELECT id FROM api_model_providers
      WHERE lower(btrim(code)) = normalized_code AND id <> canonical_id ORDER BY id
    LOOP
      UPDATE api_model_catalog SET provider_id = canonical_id WHERE provider_id = duplicate_id;
      UPDATE api_model_providers
      SET code = left(normalized_code, 40) || '-retired-' || left(replace(id::text, '-', ''), 8),
          status = 'deprecated', status_reason = '合并规范化重复目录', active = false
      WHERE id = duplicate_id;
    END LOOP;
  END LOOP;
END $$;

-- Normalize open catalog codes before adding case-insensitive uniqueness.
UPDATE product_categories
SET code = lower(btrim(code));

UPDATE api_model_providers
SET code = lower(btrim(code));

-- Reuse an existing Grok category when development data already contains one.
DO $$
DECLARE
  canonical_id uuid;
  duplicate_id uuid;
BEGIN
  SELECT id INTO canonical_id
  FROM product_categories
  WHERE lower(btrim(code)) = 'grok'
  ORDER BY (id = '00000000-0000-0000-0000-000000000106'::uuid) DESC, id
  LIMIT 1;

  IF canonical_id IS NULL THEN
    canonical_id := '00000000-0000-0000-0000-000000000106'::uuid;
    INSERT INTO product_categories (id, code, display_name, sort_order, active, status)
    VALUES (canonical_id, 'grok', 'Grok', 30, true, 'active');
  END IF;

  FOR duplicate_id IN
    SELECT id FROM product_categories
    WHERE lower(btrim(code)) = 'grok' AND id <> canonical_id
    ORDER BY id
  LOOP
    UPDATE product_plans SET category_id = canonical_id WHERE category_id = duplicate_id;
    UPDATE product_categories
    SET code = 'grok-retired-' || left(replace(id::text, '-', ''), 8),
        status = 'deprecated',
        status_reason = '合并重复 Grok 目录',
        active = false
    WHERE id = duplicate_id;
  END LOOP;

  UPDATE product_categories
  SET code = 'grok', display_name = 'Grok', core_key = 'grok', status = 'active',
      status_reason = '', active = true, sort_order = 30
  WHERE id = canonical_id;
END $$;

-- Reuse an existing xAI provider and repoint every model reference before retiring duplicates.
DO $$
DECLARE
  canonical_id uuid;
  duplicate_id uuid;
BEGIN
  SELECT id INTO canonical_id
  FROM api_model_providers
  WHERE lower(regexp_replace(btrim(code), '[^a-z0-9]+', '', 'g')) = 'xai'
  ORDER BY (id = '00000000-0000-0000-0000-000000000c06'::uuid) DESC, created_at, id
  LIMIT 1;

  IF canonical_id IS NULL THEN
    canonical_id := '00000000-0000-0000-0000-000000000c06'::uuid;
    INSERT INTO api_model_providers (
      id, provider_category, code, display_name, active, sort_order, status
    ) VALUES (
      canonical_id, 'grok', 'xai', 'xAI', true, 30, 'active'
    );
  END IF;

  FOR duplicate_id IN
    SELECT id FROM api_model_providers
    WHERE lower(regexp_replace(btrim(code), '[^a-z0-9]+', '', 'g')) = 'xai'
      AND id <> canonical_id
    ORDER BY id
  LOOP
    UPDATE api_model_catalog SET provider_id = canonical_id WHERE provider_id = duplicate_id;
    UPDATE api_model_providers
    SET code = 'xai-retired-' || left(replace(id::text, '-', ''), 8),
        status = 'deprecated',
        status_reason = '合并重复 xAI 目录',
        active = false
    WHERE id = duplicate_id;
  END LOOP;

  UPDATE api_model_providers
  SET provider_category = 'grok', code = 'xai', display_name = 'xAI', core_key = 'grok',
      status = 'active', status_reason = '', active = true, sort_order = 30
  WHERE id = canonical_id;
END $$;

INSERT INTO product_plans (
  id, category_id, provider_code, slug, display_name, description, publish_policy,
  access_mode, provider_policy_status, risk_level, risk_ack_required, policy_note,
  active, allow_custom_variant, sort_order, quota_label, quota_unit, quota_period,
  core_key, status
)
SELECT
  '00000000-0000-0000-0000-000000000501', category.id, 'xai', 'grok-premium',
  'Grok Premium', '社区 Grok 订阅拼车品类。', 'allowed', 'owner_managed_access',
  'unknown', 'elevated', false, '需说明成员、席位或站外访问安排。',
  true, false, 60, '额度', 'USD', 'monthly', 'grok', 'active'
FROM product_categories category
WHERE category.core_key = 'grok'
ON CONFLICT (slug) DO UPDATE
SET category_id = EXCLUDED.category_id,
    provider_code = EXCLUDED.provider_code,
    core_key = 'grok',
    status = 'active',
    active = true,
    updated_at = now();

INSERT INTO api_model_catalog (
  id, provider_id, model_key, capabilities, active, sort_order, core_key, status
)
SELECT
  '00000000-0000-0000-0000-000000000a31', provider.id, 'grok-4',
  ARRAY['text'], true, 310, 'grok', 'active'
FROM api_model_providers provider
WHERE provider.core_key = 'grok'
ON CONFLICT (model_key) DO UPDATE
SET provider_id = EXCLUDED.provider_id,
    capabilities = EXCLUDED.capabilities,
    core_key = 'grok',
    status = 'active',
    active = true,
    updated_at = now();

UPDATE product_categories
SET core_key = CASE code WHEN 'gpt' THEN 'gpt' WHEN 'claude' THEN 'claude' ELSE core_key END;

UPDATE product_plans
SET core_key = CASE slug
  WHEN 'chatgpt-pro-20x-web' THEN 'gpt'
  WHEN 'claude-pro' THEN 'claude'
  ELSE core_key
END;

UPDATE api_model_providers
SET core_key = CASE code WHEN 'openai' THEN 'gpt' WHEN 'anthropic' THEN 'claude' ELSE core_key END;

UPDATE api_model_catalog
SET core_key = CASE model_key
  WHEN 'gpt-4.1' THEN 'gpt'
  WHEN 'claude-sonnet-4' THEN 'claude'
  ELSE core_key
END;

UPDATE product_categories
SET status = 'deprecated', active = false, status_reason = '首发目录范围收口'
WHERE code NOT IN ('gpt', 'claude', 'grok');

UPDATE api_model_providers
SET status = 'deprecated', active = false, status_reason = '首发目录范围收口'
WHERE code NOT IN ('openai', 'anthropic', 'xai');

UPDATE api_model_catalog model
SET status = 'deprecated', active = false, status_reason = '供应商目录已退役'
FROM api_model_providers provider
WHERE provider.id = model.provider_id AND provider.status <> 'active';

ALTER TABLE product_categories
  DROP COLUMN active,
  ADD CONSTRAINT ck_product_categories_status CHECK (status IN ('active', 'deprecated', 'blocked')),
  ADD CONSTRAINT ck_product_categories_core_key CHECK (core_key IS NULL OR core_key IN ('gpt', 'claude', 'grok')),
  ADD CONSTRAINT ck_product_categories_code CHECK (code ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$');

ALTER TABLE product_plans
  DROP COLUMN active,
  ADD CONSTRAINT ck_product_plans_status CHECK (status IN ('active', 'deprecated', 'blocked')),
  ADD CONSTRAINT ck_product_plans_core_key CHECK (core_key IS NULL OR core_key IN ('gpt', 'claude', 'grok'));

ALTER TABLE api_model_providers
  DROP COLUMN active,
  ADD CONSTRAINT ck_api_model_providers_status CHECK (status IN ('active', 'deprecated', 'blocked')),
  ADD CONSTRAINT ck_api_model_providers_core_key CHECK (core_key IS NULL OR core_key IN ('gpt', 'claude', 'grok')),
  ADD CONSTRAINT ck_api_model_providers_code CHECK (code ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$');

ALTER TABLE api_model_catalog
  DROP COLUMN active,
  ADD CONSTRAINT ck_api_model_catalog_status CHECK (status IN ('active', 'deprecated', 'blocked')),
  ADD CONSTRAINT ck_api_model_catalog_core_key CHECK (core_key IS NULL OR core_key IN ('gpt', 'claude', 'grok'));

CREATE UNIQUE INDEX ux_product_categories_code_lower ON product_categories(lower(code));
CREATE UNIQUE INDEX ux_api_model_providers_code_lower ON api_model_providers(lower(code));
CREATE UNIQUE INDEX ux_product_categories_core_key ON product_categories(core_key) WHERE core_key IS NOT NULL;
CREATE UNIQUE INDEX ux_product_plans_core_key ON product_plans(core_key) WHERE core_key IS NOT NULL;
CREATE UNIQUE INDEX ux_api_model_providers_core_key ON api_model_providers(core_key) WHERE core_key IS NOT NULL;
CREATE UNIQUE INDEX ux_api_model_catalog_core_key ON api_model_catalog(core_key) WHERE core_key IS NOT NULL;

CREATE TABLE api_order_catalog_risk_holds (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_order_id uuid NOT NULL REFERENCES api_orders(id),
  source_type text NOT NULL CHECK (source_type IN ('api_model_provider', 'api_model_catalog')),
  source_id uuid NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'restored', 'refund_pending', 'dispute_opened')),
  reason text NOT NULL,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_by uuid REFERENCES users(id),
  resolved_at timestamptz,
  resolution_note text,
  version bigint NOT NULL DEFAULT 1,
  CHECK (btrim(reason) <> ''),
  CHECK ((status = 'active' AND resolved_at IS NULL) OR (status <> 'active' AND resolved_at IS NOT NULL))
);

CREATE UNIQUE INDEX ux_api_order_catalog_risk_holds_active
ON api_order_catalog_risk_holds(api_order_id)
WHERE status = 'active';

CREATE INDEX ix_api_order_catalog_risk_holds_source
ON api_order_catalog_risk_holds(source_type, source_id, status, api_order_id);
