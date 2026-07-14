-- Add an optional admin-managed icon to product categories.
-- 日期：2026-07-11
-- 执行者：Codex

ALTER TABLE product_categories
ADD COLUMN icon_data_url text NOT NULL DEFAULT '';
