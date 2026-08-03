-- Administrator user-directory governance lookup support.
-- 日期：2026-08-01
-- 执行者：Codex

CREATE INDEX ix_admin_audit_logs_user_target_recent
ON admin_audit_logs(target_id, created_at DESC, id DESC)
WHERE target_type = 'user';
