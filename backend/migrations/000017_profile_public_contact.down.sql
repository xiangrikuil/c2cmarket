-- Roll back profile public contact fields.
-- 日期：2026-06-23
-- 执行者：Codex

DROP INDEX IF EXISTS ix_merchant_profiles_public_slug;
DROP INDEX IF EXISTS ix_users_public_username;

ALTER TABLE users
  DROP COLUMN IF EXISTS privacy_settings,
  DROP COLUMN IF EXISTS avatar_mode,
  DROP COLUMN IF EXISTS timezone,
  DROP COLUMN IF EXISTS region_code;
