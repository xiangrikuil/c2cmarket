-- Roll back seller-first API-order after-sales handling when no new facts exist.
-- Migration: 000108
-- Date: 2026-08-16
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM dispute_cases
    WHERE seller_decision <> ''
       OR seller_decided_at IS NOT NULL
       OR applicant_decision_due_at IS NOT NULL
       OR status IN ('pending_applicant_decision', 'voluntary_fulfillment')
       OR EXISTS (
         SELECT 1 FROM api_order_dispute_remedies
         WHERE source = 'seller_acceptance'
       )
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 108 after seller-first after-sales data exists';
  END IF;
END $$;

DROP INDEX IF EXISTS ix_dispute_cases_applicant_decision_due;

UPDATE api_orders order_row
SET dispute_status = 'open',
    updated_at = now(),
    version = order_row.version + 1
FROM dispute_cases dispute
WHERE order_row.dispute_case_id = dispute.id
  AND order_row.dispute_status = 'pending_seller_response'
  AND dispute.status = 'pending_seller_response';

UPDATE dispute_cases
SET status = 'open',
    public_result = '等待被申请方正式答复',
    updated_at = now(),
    version = version + 1
WHERE status = 'pending_seller_response';

ALTER TABLE api_order_dispute_remedies
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source_shape,
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source,
ADD CONSTRAINT ck_api_order_dispute_remedy_source
  CHECK (source IN ('admin_decision', 'mutual_agreement')),
ADD CONSTRAINT ck_api_order_dispute_remedy_source_shape CHECK (
  (source = 'admin_decision' AND created_by_admin_id IS NOT NULL AND settlement_proposal_id IS NULL)
  OR (source = 'mutual_agreement' AND created_by_admin_id IS NULL AND settlement_proposal_id IS NOT NULL)
);

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status,
ADD CONSTRAINT ck_api_orders_dispute_status CHECK (dispute_status IN (
  'none', 'negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation'
));

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_applicant_decision_due,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_seller_decision_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_seller_decision,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_platform_escalation_shape,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_appeal_window,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status CHECK (
  status IN ('open', 'waiting_info', 'resolved', 'closed', 'withdrawn', 'self_resolved')
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
),
ADD CONSTRAINT ck_dispute_cases_appeal_window CHECK (
  (active = true AND appeal_expires_at IS NULL AND final_reason = '' AND cardinality(adversely_affected_user_ids) = 0)
  OR (
    active = false AND status IN ('resolved', 'closed')
    AND appeal_expires_at IS NOT NULL AND final_reason <> ''
    AND cardinality(adversely_affected_user_ids) > 0
  )
  OR (
    active = false AND status NOT IN ('resolved', 'closed')
    AND appeal_expires_at IS NULL AND final_reason = ''
    AND cardinality(adversely_affected_user_ids) = 0
  )
),
DROP COLUMN applicant_decision_due_at,
DROP COLUMN seller_response_late,
DROP COLUMN seller_decided_at,
DROP COLUMN seller_decided_by_user_id,
DROP COLUMN seller_decision_reason,
DROP COLUMN seller_decision;
