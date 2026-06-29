-- Product-plan-driven monthly quota display fields for carpool listings.
-- 日期：2026-06-28
-- 执行者：Codex

ALTER TABLE product_plans
ADD COLUMN quota_label text NOT NULL DEFAULT '额度',
ADD COLUMN quota_unit text NOT NULL DEFAULT 'USD',
ADD COLUMN quota_period text NOT NULL DEFAULT 'monthly' CHECK (quota_period IN ('monthly')),
ADD CONSTRAINT ck_product_plans_quota_label_not_blank CHECK (btrim(quota_label) <> ''),
ADD CONSTRAINT ck_product_plans_quota_unit_not_blank CHECK (btrim(quota_unit) <> '');

ALTER TABLE carpool_listings
ADD COLUMN monthly_quota_amount numeric(12,2) NOT NULL DEFAULT 0 CHECK (monthly_quota_amount >= 0),
ADD COLUMN quota_label text NOT NULL DEFAULT '额度',
ADD COLUMN quota_unit text NOT NULL DEFAULT 'USD',
ADD COLUMN quota_period text NOT NULL DEFAULT 'monthly' CHECK (quota_period IN ('monthly')),
ADD CONSTRAINT ck_carpool_listings_quota_label_not_blank CHECK (btrim(quota_label) <> ''),
ADD CONSTRAINT ck_carpool_listings_quota_unit_not_blank CHECK (btrim(quota_unit) <> '');

UPDATE carpool_listings
SET monthly_quota_amount = average_quota_usd,
    quota_label = '额度',
    quota_unit = 'USD',
    quota_period = 'monthly';
