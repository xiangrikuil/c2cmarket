-- Persist first-delivery validity failures without replacing credentials, inventory, or expiry.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE api_orders
ADD COLUMN quota_validity_issue_at timestamptz,
ADD COLUMN quota_validity_issue_reason text;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_quota_validity_issue CHECK (
  (quota_validity_issue_at IS NULL AND quota_validity_issue_reason IS NULL)
  OR (
    quota_validity_issue_at IS NOT NULL
    AND quota_validity_issue_reason = 'delivery_insufficient'
    AND status = 'paid_confirmed'
  )
);

CREATE INDEX ix_api_orders_seller_quota_validity_issue
ON api_orders(seller_user_id, quota_validity_issue_at DESC, id)
WHERE quota_validity_issue_at IS NOT NULL;
