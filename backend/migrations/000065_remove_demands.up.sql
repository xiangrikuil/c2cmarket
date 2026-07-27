-- 上线前删除求车模块。求车数据和关联幂等结果不保留。
-- 日期：2026-07-27
-- 执行者：Codex

DELETE FROM idempotency_keys
WHERE resource_type = 'demand';

DROP TABLE IF EXISTS demands;
