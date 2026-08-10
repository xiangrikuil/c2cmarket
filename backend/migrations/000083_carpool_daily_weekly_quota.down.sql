-- Restore carpool quota periods from daily/weekly to weekly/monthly.
-- Date: 2026-08-09
-- Executor: Codex

ALTER TABLE carpool_listings
RENAME CONSTRAINT ck_carpool_listings_daily_quota_positive
TO ck_carpool_listings_weekly_quota_positive;

ALTER TABLE carpool_listings
RENAME CONSTRAINT ck_carpool_listings_weekly_quota_nonnegative
TO carpool_listings_monthly_quota_amount_check;

ALTER TABLE carpool_listings
RENAME CONSTRAINT carpool_listings_weekly_quota_amount_not_null
TO carpool_listings_monthly_quota_amount_not_null;

ALTER TABLE carpool_listings
RENAME COLUMN weekly_quota_amount TO monthly_quota_amount;

ALTER TABLE carpool_listings
RENAME COLUMN daily_quota_amount TO weekly_quota_amount;
