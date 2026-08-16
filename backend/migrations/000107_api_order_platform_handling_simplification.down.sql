-- Roll back direct platform handling fields while preserving historical cases.
-- Migration: 000107
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM dispute_cases
    WHERE status IN ('withdrawn', 'self_resolved')
       OR applicant_statement <> ''
       OR respondent_response <> ''
       OR responded_at IS NOT NULL
       OR responded_by_user_id IS NOT NULL
       OR fact_snapshot <> '{}'::jsonb
       OR EXISTS (SELECT 1 FROM api_order_evidence_bindings WHERE usage = 'formal_response')
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 107 after direct platform-handling data exists';
  END IF;
END $$;

DROP INDEX IF EXISTS ix_dispute_cases_next_actor_due;

UPDATE api_order_dispute_settlement_proposals
SET status = 'pending',
    superseded_reason = ''
WHERE superseded_reason = 'platform_flow_simplified';

UPDATE dispute_cases
SET status = 'negotiating'
WHERE status = 'open'
  AND next_actor = 'respondent'
  AND due_at IS NOT NULL
  AND escalated_at IS NULL;

ALTER TABLE api_order_dispute_settlement_proposals
DROP CONSTRAINT IF EXISTS api_order_dispute_settlement_proposals_superseded_reason_check;

ALTER TABLE api_order_evidence_bindings
DROP CONSTRAINT IF EXISTS api_order_evidence_bindings_usage_check,
DROP CONSTRAINT IF EXISTS api_order_evidence_bindings_check,
ADD CONSTRAINT api_order_evidence_bindings_usage_check CHECK (usage IN (
  'dispute_initial', 'platform_escalation', 'message', 'info_supplement',
  'remedy_claim', 'remedy_contest', 'appeal'
)),
ADD CONSTRAINT api_order_evidence_bindings_check CHECK (
  (usage IN ('dispute_initial', 'platform_escalation') AND source_type = 'dispute_case'
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

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_terminal_next_actor,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_response_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_next_actor,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status CHECK (
  status IN ('negotiating', 'open', 'waiting_info', 'resolved', 'closed')
),
DROP COLUMN responded_at,
DROP COLUMN responded_by_user_id,
DROP COLUMN respondent_response,
DROP COLUMN applicant_statement,
DROP COLUMN fact_snapshot,
DROP COLUMN due_at,
DROP COLUMN next_actor;
