-- Persist one account-level API payment method and retain service/order snapshots.
-- 日期：2026-07-30
-- 执行者：Codex

WITH ranked_enabled AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY api_service_id
           ORDER BY updated_at DESC,
                    CASE payment_method WHEN 'wechat' THEN 0 ELSE 1 END,
                    id
         ) AS enabled_rank
  FROM api_service_payment_options
  WHERE enabled = true
    AND payment_method IN ('wechat', 'alipay')
)
UPDATE api_service_payment_options option_row
SET enabled = false,
    updated_at = now(),
    version = option_row.version + 1
FROM ranked_enabled
WHERE option_row.id = ranked_enabled.id
  AND ranked_enabled.enabled_rank > 1;

CREATE UNIQUE INDEX ux_api_service_payment_options_one_enabled
ON api_service_payment_options(api_service_id)
WHERE enabled = true
  AND payment_method IN ('wechat', 'alipay');

CREATE TABLE api_payment_account_options (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  payment_method text NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  payment_instructions text NOT NULL DEFAULT '',
  payment_qr_code_data_url text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  PRIMARY KEY (user_id, payment_method),
  CONSTRAINT ck_api_payment_account_options_method
    CHECK (payment_method IN ('wechat', 'alipay')),
  CONSTRAINT ck_api_payment_account_options_payload
    CHECK (
      enabled = false
      OR (
        payment_qr_code_data_url IS NOT NULL
        AND trim(payment_qr_code_data_url) <> ''
      )
    )
);

CREATE UNIQUE INDEX ux_api_payment_account_options_one_enabled
ON api_payment_account_options(user_id)
WHERE enabled = true;

WITH ranked_owner_options AS (
  SELECT service.owner_user_id AS user_id,
         option_row.payment_method,
         option_row.payment_instructions,
         option_row.payment_qr_code_data_url,
         option_row.updated_at,
         row_number() OVER (
           PARTITION BY service.owner_user_id
           ORDER BY service.updated_at DESC,
                    option_row.updated_at DESC,
                    CASE option_row.payment_method WHEN 'wechat' THEN 0 ELSE 1 END
         ) AS owner_rank
  FROM api_service_payment_options option_row
  JOIN api_services service ON service.id = option_row.api_service_id
  WHERE option_row.enabled = true
    AND option_row.payment_method IN ('wechat', 'alipay')
    AND option_row.payment_qr_code_data_url IS NOT NULL
    AND trim(option_row.payment_qr_code_data_url) <> ''
)
INSERT INTO api_payment_account_options (
  user_id, payment_method, enabled, payment_instructions,
  payment_qr_code_data_url, created_at, updated_at, version
)
SELECT user_id, payment_method, true, payment_instructions,
       payment_qr_code_data_url, updated_at, updated_at, 1
FROM ranked_owner_options
WHERE owner_rank = 1
ON CONFLICT (user_id, payment_method) DO NOTHING;
