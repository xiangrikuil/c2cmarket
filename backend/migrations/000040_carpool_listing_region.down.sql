-- Roll back persisted carpool listing opening region display values.
-- 日期：2026-07-08
-- 执行者：Codex

DROP INDEX IF EXISTS ix_carpool_listings_region;

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS ck_carpool_listings_region_name_not_blank,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_region_code_not_blank,
DROP COLUMN IF EXISTS region_name,
DROP COLUMN IF EXISTS region_code;
