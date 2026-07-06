-- Align search trigram expressions with current public search predicates.
-- 日期：2026-07-06
-- 执行者：Codex

CREATE EXTENSION IF NOT EXISTS pg_trgm;

DROP INDEX IF EXISTS ix_merchant_profiles_search_trgm;

CREATE INDEX ix_merchant_profiles_search_trgm
ON merchant_profiles
USING gin ((lower(display_name)) gin_trgm_ops);
