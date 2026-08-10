-- Remove multi-contact snapshots and issue occurrence time while legacy contact columns remain.
-- Date: 2026-08-10
-- Author: Codex

ALTER TABLE dispute_cases
DROP COLUMN IF EXISTS issue_occurred_at;

DROP INDEX IF EXISTS ix_api_intent_owner_contact_snapshots_owner;
DROP TABLE IF EXISTS api_purchase_intent_owner_contact_snapshots;

DROP INDEX IF EXISTS ix_api_service_contact_methods_owner;
DROP TABLE IF EXISTS api_service_contact_methods;
