-- 回退平台固定限时抢场次查询键。
-- 日期：2026-07-24
-- 执行者：Codex

DROP INDEX IF EXISTS ix_api_quota_sale_rounds_system_slot;

ALTER TABLE api_quota_sale_rounds
DROP CONSTRAINT IF EXISTS ck_api_quota_sale_rounds_system_slot_key,
DROP COLUMN IF EXISTS system_slot_key;
