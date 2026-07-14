-- Roll back product category icons.
-- 日期：2026-07-11
-- 执行者：Codex

ALTER TABLE product_categories
DROP COLUMN IF EXISTS icon_data_url;
