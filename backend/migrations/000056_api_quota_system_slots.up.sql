-- 为平台固定限时抢场次增加稳定查询键。
-- 日期：2026-07-24
-- 执行者：Codex

ALTER TABLE api_quota_sale_rounds
ADD COLUMN system_slot_key text,
ADD CONSTRAINT ck_api_quota_sale_rounds_system_slot_key
  CHECK (
    system_slot_key IS NULL
    OR system_slot_key ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}@(09|13|20):00$'
  );

CREATE INDEX ix_api_quota_sale_rounds_system_slot
ON api_quota_sale_rounds(system_slot_key, status, starts_at, id)
WHERE system_slot_key IS NOT NULL;
