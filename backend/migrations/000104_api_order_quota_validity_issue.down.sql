-- Remove quota-validity issue fields only when no fact would be lost.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_orders WHERE quota_validity_issue_at IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot roll back quota validity issues after facts have been recorded';
  END IF;
END $$;

DROP INDEX IF EXISTS ix_api_orders_seller_quota_validity_issue;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_quota_validity_issue,
DROP COLUMN IF EXISTS quota_validity_issue_reason,
DROP COLUMN IF EXISTS quota_validity_issue_at;
