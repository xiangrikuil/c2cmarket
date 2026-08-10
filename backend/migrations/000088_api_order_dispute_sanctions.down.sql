-- Remove the direct remedy link while preserving existing restriction rows.
-- Date: 2026-08-09
-- Author: Codex

DROP INDEX IF EXISTS ix_api_order_dispute_remedies_responsible_overdue;
DROP INDEX IF EXISTS ux_user_restrictions_source_dispute_remedy;

ALTER TABLE user_restrictions
DROP COLUMN IF EXISTS source_dispute_remedy_id;
