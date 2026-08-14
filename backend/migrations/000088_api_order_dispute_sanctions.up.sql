-- Link API-order sanctions directly to administrator-confirmed overdue remedies.
-- Date: 2026-08-09
-- Author: Codex

ALTER TABLE user_restrictions
ADD COLUMN source_dispute_remedy_id uuid
REFERENCES api_order_dispute_remedies(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX ux_user_restrictions_source_dispute_remedy
ON user_restrictions(source_dispute_remedy_id)
WHERE source_dispute_remedy_id IS NOT NULL;

CREATE INDEX ix_api_order_dispute_remedies_responsible_overdue
ON api_order_dispute_remedies(responsible_user_id, overdue_at DESC, id DESC)
WHERE status = 'overdue';
