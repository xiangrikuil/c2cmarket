-- Add irreversible retention destruction states for API delivery credentials.
-- Date: 2026-08-03
-- Executor: Codex

ALTER TABLE api_order_delivery_credentials
ADD COLUMN destroyed_at timestamptz,
ADD COLUMN destroy_reason text;

ALTER TABLE api_order_delivery_credentials
DROP CONSTRAINT IF EXISTS api_order_delivery_credentials_check1;

ALTER TABLE api_order_delivery_credentials
ADD CONSTRAINT ck_api_order_delivery_credentials_destruction
CHECK (
  (destroyed_at IS NULL AND destroy_reason IS NULL)
  OR (
    destroyed_at IS NOT NULL
    AND destroy_reason IN ('retention_expired', 'retired_unused')
  )
),
ADD CONSTRAINT ck_api_order_delivery_credentials_payload
CHECK (
  (
    destroyed_at IS NULL
    AND (
      (
        delivery_kind = 'api_key_endpoint'
        AND api_base_url IS NOT NULL AND trim(api_base_url) <> ''
        AND api_key_ciphertext IS NOT NULL AND api_key_nonce IS NOT NULL
        AND panel_login_url IS NULL AND username IS NULL
        AND password_ciphertext IS NULL AND password_nonce IS NULL
      )
      OR (
        delivery_kind = 'login_account'
        AND panel_login_url IS NOT NULL AND trim(panel_login_url) <> ''
        AND username IS NOT NULL AND trim(username) <> ''
        AND password_ciphertext IS NOT NULL AND password_nonce IS NOT NULL
        AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
      )
    )
  )
  OR (
    destroyed_at IS NOT NULL
    AND api_base_url IS NULL AND panel_login_url IS NULL
    AND username IS NULL AND instructions IS NULL
    AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
    AND password_ciphertext IS NULL AND password_nonce IS NULL
  )
);

ALTER TABLE api_quota_credentials
ADD COLUMN destroyed_at timestamptz,
ADD COLUMN destroy_reason text;

ALTER TABLE api_quota_credentials
ALTER COLUMN secret_fingerprint DROP NOT NULL;

ALTER TABLE api_quota_credentials
DROP CONSTRAINT IF EXISTS api_quota_credentials_check,
DROP CONSTRAINT IF EXISTS api_quota_credentials_check1;

ALTER TABLE api_quota_credentials
ADD CONSTRAINT ck_api_quota_credentials_destruction
CHECK (
  (destroyed_at IS NULL AND destroy_reason IS NULL)
  OR (
    destroyed_at IS NOT NULL
    AND destroy_reason IN ('retention_expired', 'retired_unused')
  )
),
ADD CONSTRAINT ck_api_quota_credentials_payload
CHECK (
  (
    destroyed_at IS NULL
    AND secret_fingerprint IS NOT NULL
    AND octet_length(secret_fingerprint) > 0
    AND (
      (
        delivery_kind = 'api_key_endpoint'
        AND api_base_url IS NOT NULL AND trim(api_base_url) <> ''
        AND api_key_ciphertext IS NOT NULL AND api_key_nonce IS NOT NULL
        AND panel_login_url IS NULL AND username IS NULL
        AND password_ciphertext IS NULL AND password_nonce IS NULL
      )
      OR (
        delivery_kind = 'login_account'
        AND panel_login_url IS NOT NULL AND trim(panel_login_url) <> ''
        AND username IS NOT NULL AND trim(username) <> ''
        AND password_ciphertext IS NOT NULL AND password_nonce IS NOT NULL
        AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
      )
    )
  )
  OR (
    destroyed_at IS NOT NULL
    AND secret_fingerprint IS NULL
    AND api_base_url IS NULL AND panel_login_url IS NULL
    AND username IS NULL AND instructions IS NULL
    AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
    AND password_ciphertext IS NULL AND password_nonce IS NULL
  )
),
ADD CONSTRAINT ck_api_quota_credentials_state
CHECK (
  (status = 'available' AND reserved_order_id IS NULL AND reserved_at IS NULL AND delivered_at IS NULL AND retired_at IS NULL AND destroyed_at IS NULL)
  OR (status = 'reserved' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NULL AND retired_at IS NULL AND destroyed_at IS NULL)
  OR (status = 'delivered' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NOT NULL AND retired_at IS NULL)
  OR (status = 'retired' AND delivered_at IS NULL AND retired_at IS NOT NULL)
);

CREATE INDEX ix_api_orders_delivery_credential_retention
ON api_orders(completed_at, id)
WHERE status = 'completed' AND completed_at IS NOT NULL;

CREATE INDEX ix_api_quota_credentials_retention
ON api_quota_credentials(retired_at, id)
WHERE status = 'retired' AND destroyed_at IS NULL;
