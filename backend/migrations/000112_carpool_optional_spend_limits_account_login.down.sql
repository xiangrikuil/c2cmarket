-- Restore required weekly spend limits and the previous distribution methods.
-- Date: 2026-08-17
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM carpool_listings
    WHERE weekly_spend_limit_usd IS NULL
       OR distribution_method = 'account_login'
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 112 while unlimited weekly spend limits or account-login listings exist';
  END IF;
END
$$;

ALTER TABLE carpool_listings
DROP CONSTRAINT ck_carpool_listings_distribution_method,
ADD CONSTRAINT ck_carpool_listings_distribution_method
CHECK (distribution_method IN ('sub2api', 'other'));

ALTER TABLE carpool_listings
ALTER COLUMN weekly_spend_limit_usd SET NOT NULL;
