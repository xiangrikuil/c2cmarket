-- 新增限时 API 额度包的加密凭据预导入库存。
-- 日期：2026-07-19
-- 执行者：Codex

CREATE TABLE api_quota_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_quota_offer_id uuid NOT NULL,
  seller_user_id uuid NOT NULL REFERENCES users(id),
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
  secret_fingerprint bytea NOT NULL,
  status text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'reserved', 'delivered', 'retired')),
  reserved_order_id uuid REFERENCES api_orders(id),
  reserved_at timestamptz,
  delivered_at timestamptz,
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, api_quota_offer_id, seller_user_id),
  FOREIGN KEY (api_quota_offer_id, seller_user_id)
    REFERENCES api_quota_offers(id, owner_user_id),
  CHECK (octet_length(secret_fingerprint) > 0),
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
  CHECK (
    (status = 'available' AND reserved_order_id IS NULL AND reserved_at IS NULL AND delivered_at IS NULL AND retired_at IS NULL)
    OR (status = 'reserved' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NULL AND retired_at IS NULL)
    OR (status = 'delivered' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NOT NULL AND retired_at IS NULL)
    OR (status = 'retired' AND delivered_at IS NULL AND retired_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_api_quota_credentials_seller_fingerprint
ON api_quota_credentials(seller_user_id, secret_fingerprint);

CREATE UNIQUE INDEX ux_api_quota_credentials_reserved_order
ON api_quota_credentials(reserved_order_id)
WHERE reserved_order_id IS NOT NULL;

CREATE INDEX ix_api_quota_credentials_available
ON api_quota_credentials(api_quota_offer_id, id)
WHERE status = 'available';

CREATE INDEX ix_api_quota_credentials_offer_status
ON api_quota_credentials(api_quota_offer_id, status, id);

ALTER TABLE api_orders
ADD COLUMN api_quota_credential_id uuid,
ADD CONSTRAINT fk_api_orders_quota_credential
  FOREIGN KEY (api_quota_credential_id, api_quota_offer_id, seller_user_id)
  REFERENCES api_quota_credentials(id, api_quota_offer_id, seller_user_id),
ADD CONSTRAINT ck_api_orders_quota_credential_selection
  CHECK (
    (purchase_kind = 'api_service' AND api_quota_credential_id IS NULL)
    OR (
      purchase_kind = 'limited_quota_offer'
      AND (
        (quota_delivery_mode_snapshot = 'manual' AND api_quota_credential_id IS NULL)
        OR (quota_delivery_mode_snapshot = 'preimported' AND api_quota_credential_id IS NOT NULL)
      )
    )
  );
