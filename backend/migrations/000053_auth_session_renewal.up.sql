-- 增加登录会话的节流续期与绝对到期边界。
-- 日期：2026-07-21
-- 执行者：Codex

ALTER TABLE auth_sessions
ADD COLUMN renewed_at timestamptz,
ADD COLUMN absolute_expires_at timestamptz,
ADD COLUMN updated_at timestamptz;

UPDATE auth_sessions
SET expires_at = LEAST(expires_at, created_at + interval '30 days'),
    renewed_at = created_at,
    absolute_expires_at = created_at + interval '30 days',
    updated_at = COALESCE(last_seen_at, created_at);

ALTER TABLE auth_sessions
ALTER COLUMN renewed_at SET DEFAULT now(),
ALTER COLUMN renewed_at SET NOT NULL,
ALTER COLUMN absolute_expires_at SET DEFAULT (now() + interval '30 days'),
ALTER COLUMN absolute_expires_at SET NOT NULL,
ALTER COLUMN updated_at SET DEFAULT now(),
ALTER COLUMN updated_at SET NOT NULL,
ADD CONSTRAINT ck_auth_sessions_expiry_order
  CHECK (
    created_at <= renewed_at
    AND renewed_at < expires_at
    AND expires_at <= absolute_expires_at
  );
