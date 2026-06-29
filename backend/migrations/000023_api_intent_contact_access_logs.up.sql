-- API purchase intent contact disclosure audit logs.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE api_purchase_intent_contact_access_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_purchase_intent_id uuid NOT NULL REFERENCES api_purchase_intents(id) ON DELETE CASCADE,
  viewer_user_id uuid NOT NULL REFERENCES users(id),
  viewed_contact_owner_side text NOT NULL CHECK (viewed_contact_owner_side IN ('buyer', 'merchant')),
  request_id text NOT NULL,
  accessed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_api_intent_contact_access_logs_intent_accessed
ON api_purchase_intent_contact_access_logs(api_purchase_intent_id, accessed_at DESC);

CREATE INDEX ix_api_intent_contact_access_logs_viewer_accessed
ON api_purchase_intent_contact_access_logs(viewer_user_id, accessed_at DESC);
