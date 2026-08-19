-- Make WeChat the required all-purpose transaction contact.
-- Date: 2026-08-18
-- Executor: Codex

UPDATE contact_methods
SET usage_scopes = ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[]
WHERE type = 'wechat';

ALTER TABLE contact_methods
  DROP CONSTRAINT IF EXISTS ck_contact_methods_wechat_all_usage_scopes,
  ADD CONSTRAINT ck_contact_methods_wechat_all_usage_scopes
    CHECK (
      type <> 'wechat'
      OR usage_scopes = ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[]
    );
