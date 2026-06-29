-- Convert API purchase intents from contact-window flow to direct frozen contact disclosure.
-- 日期：2026-06-22
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_api_purchase_intent_contact_session ON api_purchase_intents;
DROP FUNCTION IF EXISTS enforce_api_purchase_intent_contact_session();

DROP INDEX IF EXISTS ix_api_purchase_intents_contact_expiry;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_contact_session_id_fkey,
DROP CONSTRAINT IF EXISTS api_purchase_intents_contact_session_id_key,
DROP CONSTRAINT IF EXISTS api_purchase_intents_contact_expires_at_check,
DROP CONSTRAINT IF EXISTS api_purchase_intents_status_check;

UPDATE api_purchase_intents
SET status = 'open'
WHERE status = 'contact_open';

ALTER TABLE api_purchase_intents
DROP COLUMN IF EXISTS contact_session_id,
DROP COLUMN IF EXISTS contact_opens_at,
DROP COLUMN IF EXISTS contact_expires_at;

ALTER TABLE api_purchase_intents
ADD CONSTRAINT api_purchase_intents_status_check
CHECK (status IN ('open', 'contacted', 'buyer_cancelled', 'owner_closed'));

CREATE UNIQUE INDEX IF NOT EXISTS ux_api_purchase_intents_active_buyer_service
ON api_purchase_intents(buyer_user_id, api_service_id)
WHERE status IN ('open', 'contacted');
