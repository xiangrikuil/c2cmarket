-- Search trigram indexes for public marketplace search.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX ix_api_services_search_trgm
ON api_services
USING gin ((lower(title || ' ' || short_description)) gin_trgm_ops);

CREATE INDEX ix_api_service_models_search_trgm
ON api_service_models
USING gin ((lower(model_name_snapshot || ' ' || provider_snapshot)) gin_trgm_ops);

CREATE INDEX ix_carpool_listings_search_trgm
ON carpool_listings
USING gin ((lower(title || ' ' || summary || ' ' || access_arrangement)) gin_trgm_ops);

CREATE INDEX ix_demands_search_trgm
ON demands
USING gin ((lower(title || ' ' || region_code || ' ' || owner_preference || ' ' || COALESCE(note, ''))) gin_trgm_ops);

CREATE INDEX ix_product_plans_search_trgm
ON product_plans
USING gin ((lower(display_name || ' ' || provider_code || ' ' || slug)) gin_trgm_ops);

CREATE INDEX ix_api_model_catalog_search_trgm
ON api_model_catalog
USING gin ((lower(display_name || ' ' || model_key)) gin_trgm_ops);

CREATE INDEX ix_api_model_providers_search_trgm
ON api_model_providers
USING gin ((lower(display_name || ' ' || code)) gin_trgm_ops);

CREATE INDEX ix_users_search_trgm
ON users
USING gin ((lower(username || ' ' || display_name)) gin_trgm_ops);

CREATE INDEX ix_linux_do_bindings_search_trgm
ON linux_do_bindings
USING gin ((lower(linux_do_username)) gin_trgm_ops);

CREATE INDEX ix_merchant_profiles_search_trgm
ON merchant_profiles
USING gin ((lower(slug || ' ' || display_name)) gin_trgm_ops);
