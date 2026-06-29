-- Roll back API order dispute target type support.
-- 日期：2026-06-24
-- 执行者：Codex

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS appeals_target_type_check,
ADD CONSTRAINT appeals_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent'));

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS dispute_cases_target_type_check,
ADD CONSTRAINT dispute_cases_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent'));

ALTER TABLE reports
DROP CONSTRAINT IF EXISTS reports_target_type_check,
ADD CONSTRAINT reports_target_type_check
  CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent'));
