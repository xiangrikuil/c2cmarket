-- Roll back administrator user-directory governance lookup support.
-- 日期：2026-08-01
-- 执行者：Codex

DROP INDEX IF EXISTS ix_admin_audit_logs_user_target_recent;
