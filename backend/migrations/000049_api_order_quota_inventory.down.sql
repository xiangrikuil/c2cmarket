-- 回退 API 美元额度库存与订单快照字段。
-- 日期：2026-07-12
-- 执行者：Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_metered_quota_snapshot,
DROP COLUMN pricing_snapshot,
DROP COLUMN cny_per_usd_allowance_snapshot,
DROP COLUMN requested_usd_allowance_snapshot;

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS ck_api_services_available_usd_allowance,
DROP COLUMN available_usd_allowance;
