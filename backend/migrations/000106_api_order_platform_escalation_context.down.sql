-- Roll back platform-intervention negotiation context.
-- Migration: 000106
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE api_order_dispute_settlement_proposals
DROP COLUMN IF EXISTS superseded_reason;

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_platform_escalation_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_negotiation_channels,
DROP COLUMN IF EXISTS escalated_at,
DROP COLUMN IF EXISTS escalated_by_user_id,
DROP COLUMN IF EXISTS requested_platform_action,
DROP COLUMN IF EXISTS negotiation_summary,
DROP COLUMN IF EXISTS negotiation_ended_confirmed,
DROP COLUMN IF EXISTS negotiation_channels;
