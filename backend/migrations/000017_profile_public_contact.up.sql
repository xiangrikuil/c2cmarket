-- Profile, public page, and contact management contract.
-- 日期：2026-06-23
-- 执行者：Codex

ALTER TABLE users
  ADD COLUMN region_code text,
  ADD COLUMN timezone text,
  ADD COLUMN avatar_mode text NOT NULL DEFAULT 'linuxdo' CHECK (avatar_mode IN ('linuxdo', 'custom')),
  ADD COLUMN privacy_settings jsonb NOT NULL DEFAULT '{
    "showCreatedAt": true,
    "showLastActiveAt": true,
    "showCompletedCarpoolCount": true,
    "showCompletedApiIntentCount": true,
    "showResponseMedian": true,
    "showResolvedDisputeSummary": true,
    "allowPublicProfileReport": true
  }'::jsonb;

CREATE INDEX ix_users_public_username
ON users(username)
WHERE account_status = 'active';

CREATE INDEX ix_merchant_profiles_public_slug
ON merchant_profiles(slug)
WHERE status = 'active';
