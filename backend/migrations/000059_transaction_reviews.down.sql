-- 回退统一双向评价；旧 carpool_reviews 表及数据保持不变。
-- 日期：2026-07-24
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_transaction_review_revisions_append_only
ON transaction_review_revisions;
DROP FUNCTION IF EXISTS reject_transaction_review_revision_mutation();

DROP TRIGGER IF EXISTS trg_transaction_review_freeze
ON transaction_reviews;
DROP FUNCTION IF EXISTS enforce_transaction_review_freeze();

DROP TRIGGER IF EXISTS trg_transaction_review_source
ON transaction_reviews;
DROP FUNCTION IF EXISTS enforce_transaction_review_source();

DROP TABLE IF EXISTS transaction_review_revisions;
DROP TABLE IF EXISTS transaction_reviews;
