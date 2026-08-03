-- Remove carpool quota, reset, connection, channel, and payment signals.
-- Date: 2026-08-01
-- Executor: Codex

ALTER TABLE carpool_listings
DROP COLUMN custom_payment_method,
DROP COLUMN payment_method_code,
DROP COLUMN custom_opening_channel,
DROP COLUMN opening_channel_code,
DROP COLUMN supports_mainland_china_direct_connection,
DROP COLUMN vps_region,
DROP COLUMN follows_official_quota_reset,
DROP COLUMN weekly_quota_amount;
