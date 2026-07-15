-- 回退举报、纠纷与申诉的架构升级。
-- 日期：2026-07-11
-- 执行者：Codex

DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_appeal;
DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_dispute_case;
DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_report;
DROP INDEX IF EXISTS ix_moderation_audit_logs_object;
DROP INDEX IF EXISTS ix_moderation_audit_logs_actor;
DROP TABLE IF EXISTS moderation_audit_logs;

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS ck_appeals_target_type,
ADD CONSTRAINT appeals_target_type_check
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order'));

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_public_result_code,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_target_type,
DROP COLUMN public_result_code,
ADD CONSTRAINT dispute_cases_target_type_check
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order'));

DROP INDEX IF EXISTS ux_reports_active_canonical_target;
DROP INDEX IF EXISTS ix_reports_canonical_target;

ALTER TABLE reports
DROP CONSTRAINT IF EXISTS ck_reports_status,
DROP CONSTRAINT IF EXISTS ck_reports_reason_code,
DROP CONSTRAINT IF EXISTS ck_reports_canonical_target_type,
DROP CONSTRAINT IF EXISTS ck_reports_target_type,
DROP COLUMN target_snapshot_json,
DROP COLUMN canonical_target_id,
DROP COLUMN canonical_target_type,
ADD CONSTRAINT reports_target_type_check
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent', 'api_order')),
ADD CONSTRAINT reports_reason_code_check
CHECK (reason_code IN ('invalid', 'unreachable', 'impersonation', 'other')),
ADD CONSTRAINT reports_status_check
CHECK (status IN ('submitted', 'triaged', 'rejected', 'dispute_opened'));
