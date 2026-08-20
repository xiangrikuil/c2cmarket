-- Let each transaction action select an eligible email or WeChat contact.
-- Date: 2026-08-20
-- Executor: Codex

ALTER TABLE contact_methods
  DROP CONSTRAINT IF EXISTS ck_contact_methods_wechat_all_usage_scopes,
  DROP CONSTRAINT IF EXISTS ck_contact_methods_usage_scopes,
  DROP COLUMN IF EXISTS usage_scopes;

DROP FUNCTION IF EXISTS canonical_contact_usage_scopes(text[]);
