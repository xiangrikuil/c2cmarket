-- Roll back referral rewards and API service promotion coupons.
-- Date: 2026-08-02
-- Executor: Codex

DROP TABLE IF EXISTS promotion_coupons;
DROP TABLE IF EXISTS referral_relations;
DROP TABLE IF EXISTS referral_codes;
DROP TABLE IF EXISTS promotion_reward_campaigns;
