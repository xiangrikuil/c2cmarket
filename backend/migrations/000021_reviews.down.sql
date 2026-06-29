-- Roll back carpool review center real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_carpool_review_membership ON carpool_reviews;
DROP FUNCTION IF EXISTS enforce_carpool_review_membership();
DROP INDEX IF EXISTS ix_carpool_reviews_reviewee_updated;
DROP INDEX IF EXISTS ix_carpool_reviews_reviewer_updated;
DROP TABLE IF EXISTS carpool_reviews;
