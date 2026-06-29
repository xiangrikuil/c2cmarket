-- Roll back user favorites real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

DROP INDEX IF EXISTS ix_favorites_target;
DROP INDEX IF EXISTS ix_favorites_user_created;
DROP TABLE IF EXISTS favorites;
