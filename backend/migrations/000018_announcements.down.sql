-- Roll back announcement real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

DROP INDEX IF EXISTS ix_announcement_audit_logs_created_at;
DROP TABLE IF EXISTS announcement_audit_logs;
DROP TABLE IF EXISTS announcement_receipts;
DROP INDEX IF EXISTS ix_announcements_home;
DROP INDEX IF EXISTS ix_announcements_user_visible;
DROP TABLE IF EXISTS announcements;
