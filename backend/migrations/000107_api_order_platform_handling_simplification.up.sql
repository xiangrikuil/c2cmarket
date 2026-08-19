-- Simplify new API-order disputes into direct platform handling.
-- Migration: 000107
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN next_actor text NOT NULL DEFAULT 'none',
ADD COLUMN due_at timestamptz,
ADD COLUMN fact_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN applicant_statement text NOT NULL DEFAULT '',
ADD COLUMN respondent_response text NOT NULL DEFAULT '',
ADD COLUMN responded_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
ADD COLUMN responded_at timestamptz;

UPDATE dispute_cases dispute
SET status = 'open',
    public_result = '平台处理中',
    next_actor = CASE
      WHEN dispute.api_order_id IS NOT NULL THEN 'respondent'
      ELSE 'admin'
    END,
    due_at = CASE
      WHEN dispute.api_order_id IS NOT NULL THEN dispute.updated_at + interval '48 hours'
      ELSE NULL
    END
WHERE dispute.status = 'negotiating';

UPDATE dispute_cases dispute
SET next_actor = CASE
      WHEN request.requested_from_user_id = dispute.primary_user_id THEN 'applicant'
      ELSE 'respondent'
    END,
    due_at = dispute.updated_at + interval '48 hours'
FROM moderation_info_requests request
WHERE dispute.status = 'waiting_info'
  AND request.dispute_case_id = dispute.id
  AND request.status = 'open';

UPDATE dispute_cases dispute
SET next_actor = CASE
      WHEN remedy.status = 'claimed_fulfilled' THEN 'counterparty'
      ELSE 'responsible_party'
    END,
    due_at = CASE
      WHEN remedy.status = 'claimed_fulfilled' THEN remedy.confirmation_due_at
      ELSE remedy.due_at
    END
FROM api_order_dispute_remedies remedy
WHERE dispute.status = 'resolved'
  AND remedy.dispute_case_id = dispute.id
  AND remedy.status IN ('pending', 'claimed_fulfilled');

UPDATE dispute_cases
SET next_actor = 'admin', due_at = NULL
WHERE status = 'open' AND next_actor = 'none';

UPDATE dispute_cases
SET next_actor = CASE
      WHEN status = 'open' THEN 'admin'
      WHEN status = 'resolved' THEN 'responsible_party'
      ELSE 'none'
    END
WHERE next_actor = 'none' AND active = true;

UPDATE api_order_dispute_settlement_proposals
SET status = 'superseded',
    superseded_reason = 'platform_flow_simplified',
    updated_at = now(),
    version = version + 1
WHERE status = 'pending';

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status CHECK (
  status IN ('open', 'waiting_info', 'resolved', 'closed', 'withdrawn', 'self_resolved')
),
ADD CONSTRAINT ck_dispute_cases_next_actor CHECK (
  next_actor IN ('applicant', 'respondent', 'admin', 'responsible_party', 'counterparty', 'none')
),
ADD CONSTRAINT ck_dispute_cases_response_shape CHECK (
  (responded_at IS NULL AND responded_by_user_id IS NULL AND respondent_response = '')
  OR (responded_at IS NOT NULL AND responded_by_user_id IS NOT NULL AND btrim(respondent_response) <> '')
),
ADD CONSTRAINT ck_dispute_cases_terminal_next_actor CHECK (
  status NOT IN ('closed', 'withdrawn', 'self_resolved') OR (active = false AND next_actor = 'none' AND due_at IS NULL)
);

ALTER TABLE api_order_dispute_settlement_proposals
DROP CONSTRAINT IF EXISTS api_order_dispute_settlement_proposals_superseded_reason_check,
ADD CONSTRAINT api_order_dispute_settlement_proposals_superseded_reason_check CHECK (
  superseded_reason IN ('', 'new_proposal', 'platform_escalation', 'platform_flow_simplified')
);

ALTER TABLE api_order_evidence_bindings
DROP CONSTRAINT IF EXISTS api_order_evidence_bindings_usage_check,
DROP CONSTRAINT IF EXISTS api_order_evidence_bindings_check,
ADD CONSTRAINT api_order_evidence_bindings_usage_check CHECK (usage IN (
  'dispute_initial', 'platform_escalation', 'formal_response', 'message',
  'info_supplement', 'remedy_claim', 'remedy_contest', 'appeal'
)),
ADD CONSTRAINT api_order_evidence_bindings_check CHECK (
  (usage IN ('dispute_initial', 'platform_escalation', 'formal_response') AND source_type = 'dispute_case'
    AND source_id = dispute_case_id
    AND dispute_message_id IS NULL AND info_supplement_id IS NULL
    AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
  OR (usage = 'message' AND source_type = 'dispute_message'
    AND source_id = dispute_message_id
    AND dispute_message_id IS NOT NULL AND info_supplement_id IS NULL
    AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
  OR (usage = 'info_supplement' AND source_type = 'info_supplement'
    AND source_id = info_supplement_id
    AND dispute_message_id IS NULL AND info_supplement_id IS NOT NULL
    AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
  OR (usage IN ('remedy_claim', 'remedy_contest') AND source_type = 'dispute_remedy'
    AND source_id = dispute_remedy_id
    AND dispute_message_id IS NULL AND info_supplement_id IS NULL
    AND dispute_remedy_id IS NOT NULL AND appeal_id IS NULL)
  OR (usage = 'appeal' AND source_type = 'appeal'
    AND source_id = appeal_id
    AND dispute_message_id IS NULL AND info_supplement_id IS NULL
    AND dispute_remedy_id IS NULL AND appeal_id IS NOT NULL)
);

CREATE INDEX ix_dispute_cases_next_actor_due
ON dispute_cases(next_actor, due_at, id)
WHERE active = true AND next_actor <> 'none';
