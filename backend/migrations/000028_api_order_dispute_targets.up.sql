-- Extend report/dispute target types for API order disputes.
-- 日期：2026-06-24
-- 执行者：Codex

ALTER TABLE reports
DROP CONSTRAINT IF EXISTS reports_target_type_check,
ADD CONSTRAINT reports_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order'));

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS dispute_cases_target_type_check,
ADD CONSTRAINT dispute_cases_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order'));

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS appeals_target_type_check,
ADD CONSTRAINT appeals_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order'));
