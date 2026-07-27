-- 回退登录会话的节流续期与绝对到期边界。
-- 日期：2026-07-21
-- 执行者：Codex

ALTER TABLE auth_sessions
DROP CONSTRAINT IF EXISTS ck_auth_sessions_expiry_order,
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS absolute_expires_at,
DROP COLUMN IF EXISTS renewed_at;
