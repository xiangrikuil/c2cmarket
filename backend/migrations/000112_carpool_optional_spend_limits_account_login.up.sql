-- Allow independent unlimited spend limits and account-login distribution.
-- Date: 2026-08-17
-- Executor: Codex

ALTER TABLE carpool_listings
ALTER COLUMN weekly_spend_limit_usd DROP NOT NULL;

ALTER TABLE carpool_listings
DROP CONSTRAINT ck_carpool_listings_distribution_method,
ADD CONSTRAINT ck_carpool_listings_distribution_method
CHECK (distribution_method IN ('sub2api', 'account_login', 'other'));
