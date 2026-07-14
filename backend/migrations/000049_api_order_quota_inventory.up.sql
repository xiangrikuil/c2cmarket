-- 为 API 美元额度服务增加真实剩余额度，并把额度/单价冻结到订单。
-- 日期：2026-07-12
-- 执行者：Codex

ALTER TABLE api_services
ADD COLUMN available_usd_allowance numeric(18,6);

UPDATE api_services
SET available_usd_allowance = declared_max_usd_allowance_per_intent
WHERE billing_mode = 'metered_usd_quota';

ALTER TABLE api_services
ADD CONSTRAINT ck_api_services_available_usd_allowance
CHECK (
  (billing_mode = 'metered_usd_quota' AND available_usd_allowance IS NOT NULL AND available_usd_allowance >= 0)
  OR
  (billing_mode <> 'metered_usd_quota' AND available_usd_allowance IS NULL)
);

ALTER TABLE api_orders
ADD COLUMN requested_usd_allowance_snapshot numeric(18,6),
ADD COLUMN cny_per_usd_allowance_snapshot numeric(12,4),
ADD COLUMN pricing_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;

UPDATE api_orders AS order_row
SET requested_usd_allowance_snapshot = intent.requested_usd_allowance,
    cny_per_usd_allowance_snapshot = intent.declared_cny_per_usd_allowance_snapshot,
    pricing_snapshot = intent.pricing_snapshot
FROM api_purchase_intents AS intent
WHERE intent.id = order_row.api_purchase_intent_id;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_metered_quota_snapshot
CHECK (
  (
    billing_mode_snapshot = 'metered_usd_quota'
    AND requested_usd_allowance_snapshot IS NOT NULL
    AND requested_usd_allowance_snapshot > 0
    AND cny_per_usd_allowance_snapshot IS NOT NULL
    AND cny_per_usd_allowance_snapshot > 0
  )
  OR
  (
    billing_mode_snapshot <> 'metered_usd_quota'
    AND requested_usd_allowance_snapshot IS NULL
    AND cny_per_usd_allowance_snapshot IS NULL
  )
);
