-- 将已生成订单的购买意向移出活动意向唯一约束。
-- 日期：2026-07-11
-- 执行者：Codex

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_status_check,
DROP CONSTRAINT IF EXISTS ck_api_intent_status_timestamps;

UPDATE api_purchase_intents AS intent
SET status = 'ordered',
    updated_at = now(),
    version = version + 1
WHERE intent.status IN ('open', 'contacted')
  AND EXISTS (
    SELECT 1
    FROM api_orders AS orders
    WHERE orders.api_purchase_intent_id = intent.id
  );

ALTER TABLE api_purchase_intents
ADD CONSTRAINT api_purchase_intents_status_check
CHECK (status IN ('open', 'contacted', 'ordered', 'buyer_cancelled', 'owner_closed')),
ADD CONSTRAINT ck_api_intent_status_timestamps
CHECK (
  (
    status = 'open'
    AND contacted_at IS NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'contacted'
    AND contacted_at IS NOT NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'ordered'
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'buyer_cancelled'
    AND buyer_cancelled_at IS NOT NULL
    AND buyer_cancel_reason IS NOT NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'owner_closed'
    AND owner_closed_at IS NOT NULL
    AND owner_close_reason IS NOT NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
  )
);
