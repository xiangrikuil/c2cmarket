-- Persist public carpool distribution and administrator-account signals.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE carpool_listings
ADD COLUMN distribution_method text NOT NULL DEFAULT 'other',
ADD COLUMN distribution_method_note text NOT NULL DEFAULT '历史车源未声明分发方式，需站外确认。',
ADD COLUMN provides_admin_account boolean NOT NULL DEFAULT false,
ADD CONSTRAINT ck_carpool_listings_distribution_method
  CHECK (distribution_method IN ('sub2api', 'other')),
ADD CONSTRAINT ck_carpool_listings_distribution_note_required
  CHECK (distribution_method <> 'other' OR btrim(distribution_method_note) <> '');
