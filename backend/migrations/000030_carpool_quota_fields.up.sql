-- Add structured multiplier and quota reference fields to carpool listings.
-- 日期：2026-06-26
-- 执行者：Codex

ALTER TABLE carpool_listings
ADD COLUMN service_multiplier numeric(8,4) NOT NULL DEFAULT 1.0000 CHECK (service_multiplier > 0),
ADD COLUMN average_quota_period text NOT NULL DEFAULT 'monthly' CHECK (average_quota_period IN ('weekly', 'monthly')),
ADD COLUMN average_quota_usd numeric(12,2) NOT NULL DEFAULT 0 CHECK (average_quota_usd >= 0);
