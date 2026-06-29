-- Roll back structured multiplier and quota reference fields from carpool listings.
-- 日期：2026-06-26
-- 执行者：Codex

ALTER TABLE carpool_listings
DROP COLUMN IF EXISTS average_quota_usd,
DROP COLUMN IF EXISTS average_quota_period,
DROP COLUMN IF EXISTS service_multiplier;
