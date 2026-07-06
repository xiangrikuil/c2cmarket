-- Roll back reports, disputes, and appeals real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_appeal;
DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_dispute_case;
DROP INDEX IF EXISTS ix_moderation_audit_logs_basis_report;
DROP INDEX IF EXISTS ix_moderation_audit_logs_object;
DROP INDEX IF EXISTS ix_moderation_audit_logs_actor;
DROP TABLE IF EXISTS moderation_audit_logs;

DROP INDEX IF EXISTS ix_dispute_events_actor;
DROP INDEX IF EXISTS ix_dispute_events_entity;
DROP TABLE IF EXISTS dispute_events;

DROP INDEX IF EXISTS ix_appeals_target;
DROP INDEX IF EXISTS ix_appeals_admin_status_updated;
DROP INDEX IF EXISTS ix_appeals_appellant_updated;
DROP TABLE IF EXISTS appeals;

ALTER TABLE reports
DROP COLUMN IF EXISTS dispute_case_id;

DROP INDEX IF EXISTS ix_dispute_cases_target;
DROP INDEX IF EXISTS ix_dispute_cases_admin_status_updated;
DROP INDEX IF EXISTS ix_dispute_cases_counterparty_updated;
DROP INDEX IF EXISTS ix_dispute_cases_primary_updated;
DROP TABLE IF EXISTS dispute_cases;

DROP INDEX IF EXISTS ux_reports_active_canonical_target;
DROP INDEX IF EXISTS ix_reports_canonical_target;
DROP INDEX IF EXISTS ix_reports_target;
DROP INDEX IF EXISTS ix_reports_admin_status_updated;
DROP INDEX IF EXISTS ix_reports_reporter_updated;
DROP TABLE IF EXISTS reports;
