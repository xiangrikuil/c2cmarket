-- Add seller-first API-order after-sales handling before platform intervention.
-- Migration: 000108
-- Date: 2026-08-16
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN seller_decision text NOT NULL DEFAULT '',
ADD COLUMN seller_decision_reason text NOT NULL DEFAULT '',
ADD COLUMN seller_decided_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
ADD COLUMN seller_decided_at timestamptz,
ADD COLUMN seller_response_late boolean NOT NULL DEFAULT false,
ADD COLUMN applicant_decision_due_at timestamptz;

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_platform_escalation_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_appeal_window,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status CHECK (
  status IN (
    'pending_seller_response', 'pending_applicant_decision', 'voluntary_fulfillment',
    'open', 'waiting_info', 'resolved', 'closed', 'withdrawn', 'self_resolved'
  )
),
ADD CONSTRAINT ck_dispute_cases_seller_decision CHECK (
  seller_decision IN ('', 'accepted', 'rejected')
),
ADD CONSTRAINT ck_dispute_cases_seller_decision_shape CHECK (
  (
    seller_decision = ''
    AND seller_decision_reason = ''
    AND seller_decided_by_user_id IS NULL
    AND seller_decided_at IS NULL
    AND seller_response_late = false
  )
  OR (
    seller_decision IN ('accepted', 'rejected')
    AND length(btrim(seller_decision_reason)) BETWEEN 2 AND 2000
    AND seller_decided_by_user_id IS NOT NULL
    AND seller_decided_at IS NOT NULL
  )
),
ADD CONSTRAINT ck_dispute_cases_applicant_decision_due CHECK (
  status <> 'pending_applicant_decision' OR applicant_decision_due_at IS NOT NULL
),
ADD CONSTRAINT ck_dispute_cases_platform_escalation_shape CHECK (
  (
    escalated_at IS NULL
    AND escalated_by_user_id IS NULL
  )
  OR (
    escalated_at IS NOT NULL
    AND escalated_by_user_id IS NOT NULL
    AND btrim(requested_platform_action) <> ''
    AND (
      (
        negotiation_ended_confirmed = true
        AND cardinality(negotiation_channels) BETWEEN 1 AND 5
        AND btrim(negotiation_summary) <> ''
      )
      OR (
        api_order_id IS NOT NULL
        AND negotiation_ended_confirmed = false
        AND cardinality(negotiation_channels) = 0
        AND btrim(negotiation_summary) = ''
      )
    )
  )
),
ADD CONSTRAINT ck_dispute_cases_appeal_window CHECK (
  (active = true AND appeal_expires_at IS NULL AND final_reason = '' AND cardinality(adversely_affected_user_ids) = 0)
  OR (
    active = false AND status IN ('resolved', 'closed')
    AND appeal_expires_at IS NOT NULL AND final_reason <> ''
    AND cardinality(adversely_affected_user_ids) > 0
  )
  OR (
    active = false AND status = 'closed'
    AND appeal_expires_at IS NULL
    AND final_reason IN (
      'voluntary_fulfillment_confirmed',
      'voluntary_confirmation_no_objection',
      'applicant_decision_expired'
    )
    AND cardinality(adversely_affected_user_ids) = 0
  )
  OR (
    active = false AND status NOT IN ('resolved', 'closed')
    AND appeal_expires_at IS NULL AND final_reason = ''
    AND cardinality(adversely_affected_user_ids) = 0
  )
);

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status,
ADD CONSTRAINT ck_api_orders_dispute_status CHECK (dispute_status IN (
  'none', 'negotiating', 'pending_seller_response', 'pending_applicant_decision',
  'open', 'awaiting_fulfillment', 'fulfillment_confirmation'
));

ALTER TABLE api_order_dispute_remedies
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source_shape,
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source,
ADD CONSTRAINT ck_api_order_dispute_remedy_source
  CHECK (source IN ('admin_decision', 'mutual_agreement', 'seller_acceptance')),
ADD CONSTRAINT ck_api_order_dispute_remedy_source_shape CHECK (
  (source = 'admin_decision' AND created_by_admin_id IS NOT NULL AND settlement_proposal_id IS NULL)
  OR (source = 'mutual_agreement' AND created_by_admin_id IS NULL AND settlement_proposal_id IS NOT NULL)
  OR (source = 'seller_acceptance' AND created_by_admin_id IS NULL AND settlement_proposal_id IS NULL)
);

UPDATE dispute_cases dispute
SET status = 'pending_seller_response',
    public_result = '等待卖家处理售后申请',
    next_actor = 'respondent',
    due_at = dispute.opened_at + interval '24 hours',
    updated_at = now(),
    version = version + 1
WHERE dispute.api_order_id IS NOT NULL
  AND dispute.active = true
  AND dispute.status = 'open'
  AND dispute.responded_at IS NULL
  AND dispute.next_actor = 'respondent'
  AND dispute.escalated_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM moderation_info_requests request
    WHERE request.dispute_case_id = dispute.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM api_order_dispute_remedies remedy
    WHERE remedy.dispute_case_id = dispute.id
  );

UPDATE api_orders order_row
SET dispute_status = 'pending_seller_response',
    updated_at = now(),
    version = order_row.version + 1
FROM dispute_cases dispute
WHERE order_row.dispute_case_id = dispute.id
  AND order_row.dispute_status IN ('open', 'negotiating')
  AND dispute.status = 'pending_seller_response';

CREATE INDEX ix_dispute_cases_applicant_decision_due
ON dispute_cases(applicant_decision_due_at, id)
WHERE active = true AND status = 'pending_applicant_decision';
