-- Roll back product-plan-driven carpool quota display fields.
-- 日期：2026-06-28
-- 执行者：Codex

ALTER TABLE carpool_listings
DROP COLUMN quota_period,
DROP COLUMN quota_unit,
DROP COLUMN quota_label,
DROP COLUMN monthly_quota_amount;

ALTER TABLE product_plans
DROP COLUMN quota_period,
DROP COLUMN quota_unit,
DROP COLUMN quota_label;
