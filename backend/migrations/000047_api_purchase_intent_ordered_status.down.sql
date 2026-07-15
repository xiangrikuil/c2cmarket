-- 恢复此前的 API 购买意向状态约束。
-- 日期：2026-07-11
-- 执行者：Codex

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_status_check,
DROP CONSTRAINT IF EXISTS ck_api_intent_status_timestamps;

UPDATE api_purchase_intents
SET status = 'owner_closed',
    owner_closed_at = COALESCE(owner_closed_at, updated_at),
    owner_close_reason = COALESCE(NULLIF(owner_close_reason, ''), '订单已生成，意向已归档。'),
    updated_at = now(),
    version = version + 1
WHERE status = 'ordered';

ALTER TABLE api_purchase_intents
ADD CONSTRAINT api_purchase_intents_status_check
CHECK (status IN ('open', 'contacted', 'buyer_cancelled', 'owner_closed')),
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
