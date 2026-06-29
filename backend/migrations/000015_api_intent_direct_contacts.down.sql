-- Restore legacy API purchase intent contact-window columns for local migration rollback.
-- 日期：2026-06-22
-- 执行者：Codex

DROP INDEX IF EXISTS ux_api_purchase_intents_active_buyer_service;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_status_check;

ALTER TABLE api_purchase_intents
ADD COLUMN IF NOT EXISTS contact_session_id uuid REFERENCES contact_sessions(id),
ADD COLUMN IF NOT EXISTS contact_opens_at timestamptz,
ADD COLUMN IF NOT EXISTS contact_expires_at timestamptz;

UPDATE api_purchase_intents
SET status = 'contact_open'
WHERE status = 'open';

ALTER TABLE api_purchase_intents
ADD CONSTRAINT api_purchase_intents_status_check
CHECK (status IN ('contact_open', 'contacted', 'buyer_cancelled', 'owner_closed'));

CREATE UNIQUE INDEX IF NOT EXISTS api_purchase_intents_contact_session_id_key
ON api_purchase_intents(contact_session_id)
WHERE contact_session_id IS NOT NULL;
