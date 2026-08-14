-- Add structured API-order negotiation, immutable messages, and bilateral settlement proposals.
-- Date: 2026-08-09
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN issue_code text NOT NULL DEFAULT '',
ADD COLUMN requested_resolution text NOT NULL DEFAULT '',
ADD COLUMN requested_amount_cny numeric(20, 2),
ADD CONSTRAINT ck_dispute_cases_issue_code
CHECK (issue_code IN (
  '',
  'service_unavailable',
  'description_mismatch',
  'quota_shortage',
  'expired_early',
  'not_delivered',
  'refund_not_received',
  'payment_dispute',
  'other'
)),
ADD CONSTRAINT ck_dispute_cases_requested_resolution
CHECK (requested_resolution IN ('', 'full_refund', 'partial_refund', 'continue_fulfillment', 'other')),
ADD CONSTRAINT ck_dispute_cases_requested_amount
CHECK (requested_amount_cny IS NULL OR requested_amount_cny > 0),
ADD CONSTRAINT ck_dispute_cases_api_order_request_shape
CHECK (
  target_type <> 'api_order'
  OR (
    issue_code = ''
    AND requested_resolution = ''
    AND requested_amount_cny IS NULL
  )
  OR (
    issue_code <> ''
    AND requested_resolution <> ''
    AND (
      (requested_resolution = 'partial_refund' AND requested_amount_cny IS NOT NULL)
      OR (requested_resolution <> 'partial_refund' AND requested_amount_cny IS NULL)
    )
  )
);

CREATE TABLE api_order_dispute_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dispute_case_id uuid NOT NULL REFERENCES dispute_cases(id) ON DELETE CASCADE,
  sender_user_id uuid NOT NULL REFERENCES users(id),
  body text NOT NULL CHECK (length(btrim(body)) BETWEEN 1 AND 2000),
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_api_order_dispute_messages_case_created
ON api_order_dispute_messages(dispute_case_id, created_at, id);

CREATE UNIQUE INDEX ux_api_order_dispute_messages_request
ON api_order_dispute_messages(dispute_case_id, request_id)
WHERE request_id <> '';

CREATE OR REPLACE FUNCTION reject_api_order_dispute_message_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION 'api_order_dispute_messages are append-only';
END;
$$;

CREATE TRIGGER trg_api_order_dispute_messages_append_only
BEFORE UPDATE OR DELETE ON api_order_dispute_messages
FOR EACH ROW
EXECUTE FUNCTION reject_api_order_dispute_message_mutation();

CREATE TABLE api_order_dispute_settlement_proposals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dispute_case_id uuid NOT NULL REFERENCES dispute_cases(id) ON DELETE CASCADE,
  proposed_by_user_id uuid NOT NULL REFERENCES users(id),
  resolution text NOT NULL CHECK (resolution IN ('full_refund', 'partial_refund', 'continue_fulfillment', 'other')),
  amount_cny numeric(20, 2),
  terms text NOT NULL CHECK (length(btrim(terms)) BETWEEN 1 AND 2000),
  status text NOT NULL CHECK (status IN ('pending', 'accepted', 'rejected', 'superseded')),
  accepted_by_user_id uuid REFERENCES users(id),
  accepted_at timestamptz,
  rejected_by_user_id uuid REFERENCES users(id),
  rejected_at timestamptz,
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (
    (resolution = 'partial_refund' AND amount_cny IS NOT NULL AND amount_cny > 0)
    OR (resolution <> 'partial_refund' AND amount_cny IS NULL)
  ),
  CHECK (accepted_by_user_id IS NULL OR accepted_by_user_id <> proposed_by_user_id),
  CHECK (rejected_by_user_id IS NULL OR rejected_by_user_id <> proposed_by_user_id),
  CHECK (
    (status = 'pending' AND accepted_by_user_id IS NULL AND accepted_at IS NULL AND rejected_by_user_id IS NULL AND rejected_at IS NULL)
    OR (status = 'accepted' AND accepted_by_user_id IS NOT NULL AND accepted_at IS NOT NULL AND rejected_by_user_id IS NULL AND rejected_at IS NULL)
    OR (status = 'rejected' AND rejected_by_user_id IS NOT NULL AND rejected_at IS NOT NULL AND accepted_by_user_id IS NULL AND accepted_at IS NULL)
    OR (status = 'superseded' AND accepted_by_user_id IS NULL AND accepted_at IS NULL AND rejected_by_user_id IS NULL AND rejected_at IS NULL)
  )
);

CREATE INDEX ix_api_order_dispute_proposals_case_created
ON api_order_dispute_settlement_proposals(dispute_case_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX ux_api_order_dispute_proposals_pending
ON api_order_dispute_settlement_proposals(dispute_case_id)
WHERE status = 'pending';

CREATE UNIQUE INDEX ux_api_order_dispute_proposals_request
ON api_order_dispute_settlement_proposals(dispute_case_id, request_id)
WHERE request_id <> '';
