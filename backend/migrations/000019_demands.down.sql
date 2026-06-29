-- Roll back demand posts real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

DROP INDEX IF EXISTS ix_demands_admin_status_updated;
DROP INDEX IF EXISTS ix_demands_publisher_updated;
DROP INDEX IF EXISTS ix_demands_public_active;
DROP TABLE IF EXISTS demands;
