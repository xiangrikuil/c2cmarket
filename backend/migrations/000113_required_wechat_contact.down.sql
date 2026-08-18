-- Remove the required all-purpose WeChat scope constraint.
-- Date: 2026-08-18
-- Executor: Codex

ALTER TABLE contact_methods
  DROP CONSTRAINT IF EXISTS ck_contact_methods_wechat_all_usage_scopes;

-- Existing scopes are retained because their previous user-selected values cannot be reconstructed.
