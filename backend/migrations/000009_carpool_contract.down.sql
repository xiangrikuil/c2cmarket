-- Roll back subscription carpool listing and application contract.
-- 日期：2026-06-21
-- 执行者：Codex

DROP TABLE IF EXISTS carpool_application_policy_acknowledgements;
DROP TABLE IF EXISTS carpool_applications;
DROP TABLE IF EXISTS carpool_listing_policy_acknowledgements;
DROP TABLE IF EXISTS carpool_listings;
