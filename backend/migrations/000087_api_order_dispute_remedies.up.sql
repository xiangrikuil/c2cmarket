-- Add auditable post-ruling remedies for API-order disputes.
-- Date: 2026-08-09
-- Executor: Codex

CREATE TABLE api_order_dispute_remedies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dispute_case_id uuid NOT NULL REFERENCES dispute_cases(id) ON DELETE CASCADE,
  action text NOT NULL CHECK (action IN ('full_refund', 'partial_refund', 'continue_fulfillment', 'other')),
  amount_cny numeric(20, 2),
  currency text NOT NULL DEFAULT 'CNY' CHECK (currency = 'CNY'),
  responsible_user_id uuid NOT NULL REFERENCES users(id),
  beneficiary_user_id uuid NOT NULL REFERENCES users(id),
  instructions text NOT NULL CHECK (length(btrim(instructions)) BETWEEN 2 AND 2000),
  status text NOT NULL CHECK (status IN (
    'pending',
    'claimed_fulfilled',
    'confirmed',
    'contested',
    'confirmation_expired',
    'overdue',
    'cancelled'
  )),
  due_at timestamptz NOT NULL,
  claimed_at timestamptz,
  confirmation_due_at timestamptz,
  confirmed_at timestamptz,
  contested_at timestamptz,
  confirmation_expired_at timestamptz,
  overdue_at timestamptz,
  claim_note text NOT NULL DEFAULT '',
  response_note text NOT NULL DEFAULT '',
  created_by_admin_id uuid NOT NULL REFERENCES users(id),
  created_request_id text NOT NULL DEFAULT '',
  claim_request_id text NOT NULL DEFAULT '',
  response_request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (responsible_user_id <> beneficiary_user_id),
  CHECK (due_at > created_at),
  CHECK (
    (action = 'partial_refund' AND amount_cny IS NOT NULL AND amount_cny > 0)
    OR (action <> 'partial_refund' AND amount_cny IS NULL)
  ),
  CHECK (
    (status = 'pending'
      AND claimed_at IS NULL AND confirmation_due_at IS NULL
      AND confirmed_at IS NULL AND contested_at IS NULL
      AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
    OR (status = 'claimed_fulfilled'
      AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL
      AND confirmed_at IS NULL AND contested_at IS NULL
      AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
    OR (status = 'confirmed'
      AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL
      AND confirmed_at IS NOT NULL AND contested_at IS NULL
      AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
    OR (status = 'contested'
      AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL
      AND confirmed_at IS NULL AND contested_at IS NOT NULL
      AND confirmation_expired_at IS NULL AND overdue_at IS NULL)
    OR (status = 'confirmation_expired'
      AND claimed_at IS NOT NULL AND confirmation_due_at IS NOT NULL
      AND confirmed_at IS NULL AND contested_at IS NULL
      AND confirmation_expired_at IS NOT NULL AND overdue_at IS NULL)
    OR (status = 'overdue'
      AND claimed_at IS NULL AND confirmation_due_at IS NULL
      AND confirmed_at IS NULL AND contested_at IS NULL
      AND confirmation_expired_at IS NULL AND overdue_at IS NOT NULL)
    OR status = 'cancelled'
  )
);

ALTER TABLE moderation_audit_logs
DROP CONSTRAINT moderation_audit_logs_action_check,
ADD CONSTRAINT moderation_audit_logs_action_check
CHECK (action IN ('triage', 'request_info', 'reject', 'open_dispute', 'close', 'resolve', 'approve', 'mark_overdue'));

CREATE INDEX ix_api_order_dispute_remedies_case_created
ON api_order_dispute_remedies(dispute_case_id, created_at DESC, id DESC);

CREATE INDEX ix_api_order_dispute_remedies_confirmation_due
ON api_order_dispute_remedies(confirmation_due_at, id)
WHERE status = 'claimed_fulfilled';

CREATE INDEX ix_api_order_dispute_remedies_due
ON api_order_dispute_remedies(due_at, id)
WHERE status = 'pending';

CREATE UNIQUE INDEX ux_api_order_dispute_remedies_active
ON api_order_dispute_remedies(dispute_case_id)
WHERE status IN ('pending', 'claimed_fulfilled');

CREATE UNIQUE INDEX ux_api_order_dispute_remedies_created_request
ON api_order_dispute_remedies(dispute_case_id, created_request_id)
WHERE created_request_id <> '';

CREATE UNIQUE INDEX ux_api_order_dispute_remedies_claim_request
ON api_order_dispute_remedies(dispute_case_id, claim_request_id)
WHERE claim_request_id <> '';

CREATE UNIQUE INDEX ux_api_order_dispute_remedies_response_request
ON api_order_dispute_remedies(dispute_case_id, response_request_id)
WHERE response_request_id <> '';
