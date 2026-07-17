-- 清理早期建表遗留的匿名意向状态约束，确保 ordered 状态可在订单事务中持久化。
-- 日期：2026-07-17
-- 执行者：Codex

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_check3,
DROP CONSTRAINT IF EXISTS ck_api_intent_status_timestamps;

ALTER TABLE api_purchase_intents
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
