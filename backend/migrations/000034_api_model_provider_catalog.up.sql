-- Add manageable API model providers and make models reference providers.
-- 日期：2026-06-29
-- 执行者：Codex

CREATE TABLE IF NOT EXISTS api_model_providers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_category text NOT NULL CHECK (provider_category IN ('gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other')),
  code text NOT NULL UNIQUE,
  display_name text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO api_model_providers (id, provider_category, code, display_name, active, sort_order)
VALUES
  ('00000000-0000-0000-0000-000000000c01', 'gpt', 'openai', 'OpenAI', true, 10),
  ('00000000-0000-0000-0000-000000000c02', 'claude', 'anthropic', 'Anthropic', true, 20),
  ('00000000-0000-0000-0000-000000000c03', 'gemini', 'google', 'Google', true, 30),
  ('00000000-0000-0000-0000-000000000c04', 'perplexity', 'perplexity', 'Perplexity', true, 40),
  ('00000000-0000-0000-0000-000000000c05', 'other', 'openrouter', 'OpenRouter', true, 50)
ON CONFLICT (code) DO UPDATE
SET provider_category = EXCLUDED.provider_category,
    display_name = EXCLUDED.display_name,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'api_model_catalog' AND column_name = 'provider'
  ) AND EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'api_model_catalog' AND column_name = 'provider_category'
  ) THEN
    EXECUTE $sql$
      INSERT INTO api_model_providers (provider_category, code, display_name, active, sort_order)
      SELECT DISTINCT
        catalog.provider_category,
        lower(regexp_replace(catalog.provider, '[^a-zA-Z0-9]+', '-', 'g')) AS code,
        catalog.provider AS display_name,
        true AS active,
        100 + dense_rank() OVER (ORDER BY catalog.provider) AS sort_order
      FROM api_model_catalog catalog
      WHERE catalog.provider IS NOT NULL
        AND trim(catalog.provider) <> ''
        AND NOT EXISTS (
          SELECT 1
          FROM api_model_providers provider
          WHERE provider.code = lower(regexp_replace(catalog.provider, '[^a-zA-Z0-9]+', '-', 'g'))
        )
    $sql$;

    IF NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = 'public' AND table_name = 'api_model_catalog' AND column_name = 'provider_id'
    ) THEN
      ALTER TABLE api_model_catalog ADD COLUMN provider_id uuid;
    END IF;

    EXECUTE $sql$
      UPDATE api_model_catalog catalog
      SET provider_id = provider.id
      FROM api_model_providers provider
      WHERE catalog.provider_id IS NULL
        AND provider.code = lower(regexp_replace(catalog.provider, '[^a-zA-Z0-9]+', '-', 'g'))
    $sql$;

    UPDATE api_model_catalog
    SET provider_id = '00000000-0000-0000-0000-000000000c05'
    WHERE provider_id IS NULL;

    ALTER TABLE api_model_catalog
    ALTER COLUMN provider_id SET NOT NULL;

    ALTER TABLE api_model_catalog
    DROP CONSTRAINT IF EXISTS api_model_catalog_provider_id_fkey;

    ALTER TABLE api_model_catalog
    ADD CONSTRAINT api_model_catalog_provider_id_fkey
    FOREIGN KEY (provider_id) REFERENCES api_model_providers(id);

    ALTER TABLE api_model_catalog
    DROP COLUMN provider_category,
    DROP COLUMN provider;
  END IF;
END $$;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS ix_api_model_catalog_search_trgm
ON api_model_catalog
USING gin ((lower(display_name || ' ' || model_key)) gin_trgm_ops);

CREATE INDEX IF NOT EXISTS ix_api_model_providers_search_trgm
ON api_model_providers
USING gin ((lower(display_name || ' ' || code)) gin_trgm_ops);
