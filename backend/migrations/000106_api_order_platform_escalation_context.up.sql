-- Record the completed negotiation context when participants request platform intervention.
-- Migration: 000106
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN negotiation_channels text[] NOT NULL DEFAULT ARRAY[]::text[],
ADD COLUMN negotiation_ended_confirmed boolean NOT NULL DEFAULT false,
ADD COLUMN negotiation_summary text NOT NULL DEFAULT '',
ADD COLUMN requested_platform_action text NOT NULL DEFAULT '',
ADD COLUMN escalated_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
ADD COLUMN escalated_at timestamptz,
ADD CONSTRAINT ck_dispute_cases_negotiation_channels CHECK (
  negotiation_channels <@ ARRAY['wechat', 'email', 'linux_do', 'in_site', 'other']::text[]
),
ADD CONSTRAINT ck_dispute_cases_platform_escalation_shape CHECK (
  (
    escalated_at IS NULL
    AND escalated_by_user_id IS NULL
  )
  OR (
    escalated_at IS NOT NULL
    AND escalated_by_user_id IS NOT NULL
    AND negotiation_ended_confirmed = true
    AND cardinality(negotiation_channels) BETWEEN 1 AND 5
    AND btrim(negotiation_summary) <> ''
    AND btrim(requested_platform_action) <> ''
  )
);

ALTER TABLE api_order_dispute_settlement_proposals
ADD COLUMN superseded_reason text NOT NULL DEFAULT '';
