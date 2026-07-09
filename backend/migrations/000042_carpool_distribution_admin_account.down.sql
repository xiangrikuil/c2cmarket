-- Roll back public carpool distribution and administrator-account signals.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS ck_carpool_listings_distribution_note_required,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_distribution_method,
DROP COLUMN IF EXISTS provides_admin_account,
DROP COLUMN IF EXISTS distribution_method_note,
DROP COLUMN IF EXISTS distribution_method;
