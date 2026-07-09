-- Persist carpool listing opening region display values.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE carpool_listings
ADD COLUMN region_code text NOT NULL DEFAULT 'other',
ADD COLUMN region_name text NOT NULL DEFAULT '其他',
ADD CONSTRAINT ck_carpool_listings_region_code_not_blank CHECK (btrim(region_code) <> ''),
ADD CONSTRAINT ck_carpool_listings_region_name_not_blank CHECK (btrim(region_name) <> '');

CREATE INDEX ix_carpool_listings_region
ON carpool_listings(region_code, updated_at DESC);
