-- API order in-platform delivery credentials and payment QR snapshots.
-- 日期：2026-07-09
-- 执行者：Codex

ALTER TABLE api_service_payment_options
ADD COLUMN payment_qr_code_data_url text;

ALTER TABLE api_service_payment_options
ALTER COLUMN payment_instructions SET DEFAULT '';

ALTER TABLE api_service_payment_options
DROP CONSTRAINT IF EXISTS api_service_payment_options_payment_instructions_check;

ALTER TABLE api_service_payment_options
ADD CONSTRAINT ck_api_service_payment_options_payment_payload
CHECK (
  trim(payment_instructions) <> ''
  OR payment_qr_code_data_url IS NOT NULL
) NOT VALID;

ALTER TABLE api_orders
ADD COLUMN payment_qr_code_data_url_snapshot text;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS api_orders_payment_instructions_snapshot_check;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_payment_payload_snapshot
CHECK (
  trim(payment_instructions_snapshot) <> ''
  OR payment_qr_code_data_url_snapshot IS NOT NULL
) NOT VALID;

CREATE TABLE api_order_delivery_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_order_id uuid NOT NULL REFERENCES api_orders(id) ON DELETE CASCADE,
  seller_user_id uuid NOT NULL REFERENCES users(id),
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  delivery_kind text NOT NULL CHECK (delivery_kind IN ('api_key_endpoint', 'login_account')),
  api_base_url text,
  panel_login_url text,
  username text,
  instructions text,
  api_key_ciphertext bytea,
  api_key_nonce bytea,
  password_ciphertext bytea,
  password_nonce bytea,
  secret_encryption_key_version text NOT NULL,
  submitted_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (buyer_user_id <> seller_user_id),
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
  )
);

CREATE UNIQUE INDEX ux_api_order_delivery_credentials_order
ON api_order_delivery_credentials(api_order_id);

CREATE INDEX ix_api_order_delivery_credentials_buyer
ON api_order_delivery_credentials(buyer_user_id, submitted_at DESC);

CREATE INDEX ix_api_order_delivery_credentials_seller
ON api_order_delivery_credentials(seller_user_id, submitted_at DESC);
