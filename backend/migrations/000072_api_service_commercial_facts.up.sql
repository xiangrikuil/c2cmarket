-- Add truthful API-service account-pool and merchant-refund facts, and rename
-- the merchant-declared concurrency field to match its public meaning.
-- Date: 2026-08-02
-- Executor: Codex

ALTER TABLE api_services
RENAME COLUMN recommended_concurrency TO declared_max_concurrency;

ALTER TABLE api_services
RENAME CONSTRAINT ck_api_services_recommended_concurrency
TO ck_api_services_declared_max_concurrency;

ALTER TABLE api_orders
RENAME COLUMN quota_recommended_concurrency_snapshot
TO quota_declared_max_concurrency_snapshot;

ALTER TABLE api_services
ADD COLUMN account_pool_type text,
ADD COLUMN account_pool_custom_name text,
ADD COLUMN merchant_refund_commitment boolean NOT NULL DEFAULT false,
ADD CONSTRAINT ck_api_services_account_pool_type
  CHECK (
    account_pool_type IS NULL
    OR account_pool_type IN ('gpt_pro_20x', 'gpt_pro_5x', 'gpt_plus', 'custom')
  ),
ADD CONSTRAINT ck_api_services_account_pool_shape
  CHECK (
    (account_pool_type IS NULL AND account_pool_custom_name IS NULL)
    OR
    (
      account_pool_type = 'custom'
      AND account_pool_custom_name IS NOT NULL
      AND char_length(trim(account_pool_custom_name)) BETWEEN 2 AND 40
    )
    OR
    (
      account_pool_type IN ('gpt_pro_20x', 'gpt_pro_5x', 'gpt_plus')
      AND account_pool_custom_name IS NULL
    )
  );

COMMENT ON COLUMN api_services.account_pool_type IS
  'Single merchant-declared upstream account-pool type; NULL only for historical services not yet revised.';
COMMENT ON COLUMN api_services.account_pool_custom_name IS
  'Public custom account-pool label when account_pool_type=custom; never credentials or connection data.';
COMMENT ON COLUMN api_services.merchant_refund_commitment IS
  'Merchant elected the versioned full-refund promise; the platform records but does not fund or execute refunds.';
