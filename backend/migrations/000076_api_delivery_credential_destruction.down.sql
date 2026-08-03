-- Roll back API delivery credential destruction metadata only when no secret has been destroyed.
-- Destroyed secret material is irreversible and cannot be reconstructed by a down migration.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_order_delivery_credentials WHERE destroyed_at IS NOT NULL)
     OR EXISTS (SELECT 1 FROM api_quota_credentials WHERE destroyed_at IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot roll back credential destruction after secret material has been destroyed';
  END IF;
END
$$;

DROP INDEX IF EXISTS ix_api_quota_credentials_retention;
DROP INDEX IF EXISTS ix_api_orders_delivery_credential_retention;

ALTER TABLE api_quota_credentials
DROP CONSTRAINT IF EXISTS ck_api_quota_credentials_state,
DROP CONSTRAINT IF EXISTS ck_api_quota_credentials_payload,
DROP CONSTRAINT IF EXISTS ck_api_quota_credentials_destruction;

ALTER TABLE api_quota_credentials
ADD CONSTRAINT api_quota_credentials_check
CHECK (
  octet_length(secret_fingerprint) > 0
),
ADD CONSTRAINT api_quota_credentials_check1
CHECK (
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
),
ADD CONSTRAINT api_quota_credentials_check2
CHECK (
  (status = 'available' AND reserved_order_id IS NULL AND reserved_at IS NULL AND delivered_at IS NULL AND retired_at IS NULL)
  OR (status = 'reserved' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NULL AND retired_at IS NULL)
  OR (status = 'delivered' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NOT NULL AND retired_at IS NULL)
  OR (status = 'retired' AND delivered_at IS NULL AND retired_at IS NOT NULL)
);

ALTER TABLE api_quota_credentials
ALTER COLUMN secret_fingerprint SET NOT NULL;

ALTER TABLE api_quota_credentials
DROP COLUMN IF EXISTS destroy_reason,
DROP COLUMN IF EXISTS destroyed_at;

ALTER TABLE api_order_delivery_credentials
DROP CONSTRAINT IF EXISTS ck_api_order_delivery_credentials_payload,
DROP CONSTRAINT IF EXISTS ck_api_order_delivery_credentials_destruction;

ALTER TABLE api_order_delivery_credentials
ADD CONSTRAINT api_order_delivery_credentials_check1
CHECK (
  (
    delivery_kind = 'api_key_endpoint'
    AND api_base_url IS NOT NULL
    AND trim(api_base_url) <> ''
    AND api_key_ciphertext IS NOT NULL
    AND api_key_nonce IS NOT NULL
    AND panel_login_url IS NULL
    AND username IS NULL
    AND password_ciphertext IS NULL
    AND password_nonce IS NULL
  )
  OR (
    delivery_kind = 'login_account'
    AND panel_login_url IS NOT NULL
    AND trim(panel_login_url) <> ''
    AND username IS NOT NULL
    AND trim(username) <> ''
    AND password_ciphertext IS NOT NULL
    AND password_nonce IS NOT NULL
    AND api_key_ciphertext IS NULL
    AND api_key_nonce IS NULL
  )
);

ALTER TABLE api_order_delivery_credentials
DROP COLUMN IF EXISTS destroy_reason,
DROP COLUMN IF EXISTS destroyed_at;
