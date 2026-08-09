-- Rename carpool quota periods from weekly/monthly to daily/weekly.
-- Date: 2026-08-09
-- Executor: Codex

ALTER TABLE carpool_listings
RENAME COLUMN weekly_quota_amount TO daily_quota_amount;

ALTER TABLE carpool_listings
RENAME COLUMN monthly_quota_amount TO weekly_quota_amount;

ALTER TABLE carpool_listings
RENAME CONSTRAINT ck_carpool_listings_weekly_quota_positive
TO ck_carpool_listings_daily_quota_positive;

ALTER TABLE carpool_listings
RENAME CONSTRAINT carpool_listings_monthly_quota_amount_check
TO ck_carpool_listings_weekly_quota_nonnegative;

ALTER TABLE carpool_listings
RENAME CONSTRAINT carpool_listings_monthly_quota_amount_not_null
TO carpool_listings_weekly_quota_amount_not_null;
