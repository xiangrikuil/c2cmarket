-- Remove migration-85 negotiation records and structured request fields.
-- Date: 2026-08-09
-- Executor: Codex

DROP INDEX IF EXISTS ux_api_order_dispute_proposals_request;
DROP INDEX IF EXISTS ux_api_order_dispute_proposals_pending;
DROP INDEX IF EXISTS ix_api_order_dispute_proposals_case_created;
DROP TABLE IF EXISTS api_order_dispute_settlement_proposals;

DROP TRIGGER IF EXISTS trg_api_order_dispute_messages_append_only ON api_order_dispute_messages;
DROP FUNCTION IF EXISTS reject_api_order_dispute_message_mutation();
DROP INDEX IF EXISTS ux_api_order_dispute_messages_request;
DROP INDEX IF EXISTS ix_api_order_dispute_messages_case_created;
DROP TABLE IF EXISTS api_order_dispute_messages;

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_api_order_request_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_requested_amount,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_requested_resolution,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_issue_code,
DROP COLUMN IF EXISTS requested_amount_cny,
DROP COLUMN IF EXISTS requested_resolution,
DROP COLUMN IF EXISTS issue_code;
