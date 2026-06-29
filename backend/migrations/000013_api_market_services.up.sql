-- API market service publishing and review contract.
-- 日期：2026-06-22
-- 执行者：Codex

ALTER TABLE merchant_profiles
ADD CONSTRAINT uq_merchant_profiles_id_owner
UNIQUE (id, owner_user_id);

CREATE TABLE api_model_providers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_category text NOT NULL CHECK (provider_category IN ('gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other')),
  code text NOT NULL UNIQUE,
  display_name text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_model_catalog (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id uuid NOT NULL REFERENCES api_model_providers(id),
  model_key text NOT NULL UNIQUE,
  display_name text NOT NULL,
  capabilities text[] NOT NULL DEFAULT '{}',
  active boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_model_price_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  model_catalog_id uuid NOT NULL REFERENCES api_model_catalog(id),
  source_url text,
  source_version text,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz,
  input_price_per_million numeric(14,6),
  cached_input_price_per_million numeric(14,6),
  output_price_per_million numeric(14,6),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, model_catalog_id),
  CHECK (valid_to IS NULL OR valid_to > valid_from),
  CHECK (input_price_per_million IS NULL OR input_price_per_million >= 0),
  CHECK (cached_input_price_per_million IS NULL OR cached_input_price_per_million >= 0),
  CHECK (output_price_per_million IS NULL OR output_price_per_million >= 0)
);

CREATE INDEX ix_api_model_price_versions_current
ON api_model_price_versions(model_catalog_id, valid_from DESC)
WHERE valid_to IS NULL;

CREATE TABLE api_services (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  merchant_profile_id uuid,
  merchant_identity_mode text NOT NULL CHECK (merchant_identity_mode IN ('public_profile', 'store_alias')),
  owner_contact_method_id uuid NOT NULL,
  title text NOT NULL,
  short_description text NOT NULL,
  distribution_system text NOT NULL CHECK (distribution_system IN ('sub2api', 'new_api_proxy', 'other')),
  billing_mode text NOT NULL CHECK (billing_mode IN ('metered_usd_quota', 'manual_usage_check', 'fixed_package')),
  declared_cny_per_usd_allowance numeric(12,4),
  declared_max_usd_allowance_per_intent numeric(18,6),
  minimum_intent_cny numeric(12,2) NOT NULL CHECK (minimum_intent_cny > 0),
  maximum_intent_cny numeric(12,2),
  usage_visibility text NOT NULL CHECK (usage_visibility IN ('none', 'merchant_reported', 'offsite_panel_readonly', 'fixed_package_only')),
  public_access_note text,
  merchant_note text,
  merchant_support_note text,
  review_status text NOT NULL CHECK (review_status IN ('draft', 'pending_review', 'changes_requested', 'approved', 'rejected')),
  publication_status text NOT NULL CHECK (publication_status IN ('offline', 'online', 'owner_paused', 'archived')),
  moderation_status text NOT NULL CHECK (moderation_status IN ('clear', 'admin_suspended', 'removed')),
  approved_by_admin_id uuid REFERENCES users(id),
  approved_at timestamptz,
  moderation_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, owner_user_id),
  UNIQUE (id, distribution_system),
  FOREIGN KEY (owner_contact_method_id, owner_user_id) REFERENCES contact_methods(id, user_id),
  FOREIGN KEY (merchant_profile_id, owner_user_id) REFERENCES merchant_profiles(id, owner_user_id),
  CHECK (merchant_identity_mode <> 'store_alias' OR merchant_profile_id IS NOT NULL),
  CHECK (maximum_intent_cny IS NULL OR maximum_intent_cny >= minimum_intent_cny),
  CHECK (
    billing_mode <> 'metered_usd_quota'
    OR declared_cny_per_usd_allowance IS NOT NULL
  ),
  CHECK (declared_cny_per_usd_allowance IS NULL OR declared_cny_per_usd_allowance > 0),
  CHECK (declared_max_usd_allowance_per_intent IS NULL OR declared_max_usd_allowance_per_intent > 0)
);

CREATE INDEX ix_api_services_public
ON api_services(review_status, publication_status, moderation_status, updated_at DESC);

CREATE INDEX ix_api_services_owner
ON api_services(owner_user_id, updated_at DESC);

CREATE INDEX ix_api_services_admin
ON api_services(review_status, publication_status, moderation_status, updated_at DESC);

CREATE TABLE api_service_access_modes (
  api_service_id uuid NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
  access_mode text NOT NULL CHECK (access_mode IN (
    'merchant_operated_endpoint',
    'buyer_dedicated_sub_key',
    'buyer_dedicated_panel_subaccount',
    'fixed_package_offsite',
    'manual_offsite_arrangement'
  )),
  public_note text,
  PRIMARY KEY (api_service_id, access_mode)
);

CREATE TABLE api_service_models (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL,
  distribution_system text NOT NULL,
  model_catalog_id uuid NOT NULL REFERENCES api_model_catalog(id),
  model_price_version_id uuid REFERENCES api_model_price_versions(id),
  model_name_snapshot text NOT NULL,
  provider_snapshot text NOT NULL,
  capabilities_snapshot text[] NOT NULL DEFAULT '{}',
  merchant_multiplier numeric(8,4) NOT NULL DEFAULT 1.0000,
  effective_input_price_per_million numeric(14,6),
  effective_cached_input_price_per_million numeric(14,6),
  effective_output_price_per_million numeric(14,6),
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (api_service_id, id),
  UNIQUE (api_service_id, model_catalog_id),
  FOREIGN KEY (api_service_id, distribution_system) REFERENCES api_services(id, distribution_system) ON DELETE CASCADE,
  FOREIGN KEY (model_price_version_id, model_catalog_id) REFERENCES api_model_price_versions(id, model_catalog_id),
  CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000),
  CHECK (merchant_multiplier > 0),
  CHECK (effective_input_price_per_million IS NULL OR effective_input_price_per_million >= 0),
  CHECK (effective_cached_input_price_per_million IS NULL OR effective_cached_input_price_per_million >= 0),
  CHECK (effective_output_price_per_million IS NULL OR effective_output_price_per_million >= 0)
);

CREATE INDEX ix_api_service_models_service
ON api_service_models(api_service_id, enabled);

CREATE TABLE api_service_packages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
  name text NOT NULL,
  price_cny numeric(12,2) NOT NULL CHECK (price_cny > 0),
  duration_days integer CHECK (duration_days IS NULL OR duration_days > 0),
  description text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (api_service_id, id)
);

CREATE INDEX ix_api_service_packages_service
ON api_service_packages(api_service_id, enabled, sort_order);

INSERT INTO api_model_providers (
  id, provider_category, code, display_name, active, sort_order
) VALUES
  ('00000000-0000-0000-0000-000000000c01', 'gpt', 'openai', 'OpenAI', true, 10),
  ('00000000-0000-0000-0000-000000000c02', 'claude', 'anthropic', 'Anthropic', true, 20),
  ('00000000-0000-0000-0000-000000000c03', 'gemini', 'google', 'Google', true, 30),
  ('00000000-0000-0000-0000-000000000c04', 'perplexity', 'perplexity', 'Perplexity', true, 40),
  ('00000000-0000-0000-0000-000000000c05', 'other', 'openrouter', 'OpenRouter', true, 50)
ON CONFLICT (code) DO NOTHING;

INSERT INTO api_model_catalog (
  id, provider_id, model_key, display_name, capabilities, active, sort_order
) VALUES
  ('00000000-0000-0000-0000-000000000a01', '00000000-0000-0000-0000-000000000c01', 'gpt-4.1', 'GPT-4.1', ARRAY['text'], true, 10),
  ('00000000-0000-0000-0000-000000000a02', '00000000-0000-0000-0000-000000000c01', 'gpt-4.1-mini', 'GPT-4.1 mini', ARRAY['text'], true, 20),
  ('00000000-0000-0000-0000-000000000a03', '00000000-0000-0000-0000-000000000c01', 'gpt-4o', 'GPT-4o', ARRAY['text'], true, 30),
  ('00000000-0000-0000-0000-000000000a11', '00000000-0000-0000-0000-000000000c02', 'claude-sonnet-4', 'Claude Sonnet 4', ARRAY['text'], true, 110),
  ('00000000-0000-0000-0000-000000000a21', '00000000-0000-0000-0000-000000000c03', 'gemini-2.5-pro', 'Gemini 2.5 Pro', ARRAY['text'], true, 210)
ON CONFLICT (model_key) DO NOTHING;

INSERT INTO api_model_price_versions (
  id, model_catalog_id, source_url, source_version, valid_from,
  input_price_per_million, cached_input_price_per_million, output_price_per_million
) VALUES
  ('00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-000000000a01', 'https://platform.openai.com/docs/pricing', 'seed-2026-06-22', '2026-06-22T00:00:00Z', 2.000000, 0.500000, 8.000000),
  ('00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-000000000a02', 'https://platform.openai.com/docs/pricing', 'seed-2026-06-22', '2026-06-22T00:00:00Z', 0.400000, 0.100000, 1.600000),
  ('00000000-0000-0000-0000-000000000b03', '00000000-0000-0000-0000-000000000a03', 'https://platform.openai.com/docs/pricing', 'seed-2026-06-22', '2026-06-22T00:00:00Z', 5.000000, NULL, 15.000000),
  ('00000000-0000-0000-0000-000000000b11', '00000000-0000-0000-0000-000000000a11', NULL, 'seed-2026-06-22', '2026-06-22T00:00:00Z', 3.000000, 0.300000, 15.000000),
  ('00000000-0000-0000-0000-000000000b21', '00000000-0000-0000-0000-000000000a21', NULL, 'seed-2026-06-22', '2026-06-22T00:00:00Z', 1.250000, 0.310000, 10.000000)
ON CONFLICT (id) DO NOTHING;
