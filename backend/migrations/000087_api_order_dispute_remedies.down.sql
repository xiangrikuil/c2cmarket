-- Remove API-order dispute remedy records.
-- Date: 2026-08-09
-- Executor: Codex

DROP INDEX IF EXISTS ux_api_order_dispute_remedies_response_request;
DROP INDEX IF EXISTS ux_api_order_dispute_remedies_claim_request;
DROP INDEX IF EXISTS ux_api_order_dispute_remedies_created_request;
DROP INDEX IF EXISTS ux_api_order_dispute_remedies_active;
DROP INDEX IF EXISTS ix_api_order_dispute_remedies_due;
DROP INDEX IF EXISTS ix_api_order_dispute_remedies_confirmation_due;
DROP INDEX IF EXISTS ix_api_order_dispute_remedies_case_created;
DROP TABLE IF EXISTS api_order_dispute_remedies;

ALTER TABLE moderation_audit_logs
DROP CONSTRAINT moderation_audit_logs_action_check,
ADD CONSTRAINT moderation_audit_logs_action_check
CHECK (action IN ('triage', 'request_info', 'reject', 'open_dispute', 'close', 'resolve', 'approve'));
