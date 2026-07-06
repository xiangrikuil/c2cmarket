DROP INDEX IF EXISTS ix_merchant_profiles_search_trgm;

CREATE INDEX ix_merchant_profiles_search_trgm
ON merchant_profiles
USING gin ((lower(slug || ' ' || display_name)) gin_trgm_ops);
