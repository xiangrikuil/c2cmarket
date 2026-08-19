-- Separate the active API-order dispute projection from immutable dispute history.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN api_order_id uuid,
ADD COLUMN active boolean NOT NULL DEFAULT false;

DO $$
DECLARE
  invalid_count bigint;
BEGIN
  SELECT count(*) INTO invalid_count
  FROM dispute_cases dispute
  LEFT JOIN api_orders order_row
    ON dispute.target_type = 'api_order'
   AND dispute.target_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
   AND order_row.id = dispute.target_id::uuid
  WHERE dispute.target_type = 'api_order'
    AND (order_row.id IS NULL OR order_row.dispute_case_id IS DISTINCT FROM dispute.id);

  IF invalid_count > 0 THEN
    RAISE EXCEPTION 'cannot migrate API-order dispute history: % inconsistent case(s)', invalid_count;
  END IF;
END $$;

UPDATE dispute_cases
SET api_order_id = target_id::uuid
WHERE target_type = 'api_order';

ALTER TABLE api_orders
ADD COLUMN latest_dispute_case_id uuid,
ADD COLUMN active_remedy_action text NOT NULL DEFAULT '';

UPDATE api_orders
SET latest_dispute_case_id = dispute_case_id
WHERE dispute_case_id IS NOT NULL;

UPDATE dispute_cases dispute
SET active = true
FROM api_orders order_row
WHERE dispute.id = order_row.dispute_case_id
  AND order_row.dispute_status IN (
    'negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation'
  );

UPDATE api_orders
SET dispute_status = 'none', dispute_case_id = NULL
WHERE dispute_status = 'closed';

ALTER TABLE dispute_cases
ADD CONSTRAINT fk_dispute_cases_api_order
  FOREIGN KEY (api_order_id) REFERENCES api_orders(id) ON DELETE RESTRICT,
ADD CONSTRAINT uq_dispute_cases_id_api_order UNIQUE (id, api_order_id),
ADD CONSTRAINT ck_dispute_cases_api_order_link CHECK (
  (target_type = 'api_order' AND api_order_id IS NOT NULL)
  OR (target_type <> 'api_order' AND api_order_id IS NULL)
),
ADD CONSTRAINT ck_dispute_cases_active_api_order CHECK (active = false OR api_order_id IS NOT NULL);

CREATE UNIQUE INDEX ux_dispute_cases_one_active_api_order
ON dispute_cases(api_order_id)
WHERE active = true;

CREATE INDEX ix_dispute_cases_api_order_history
ON dispute_cases(api_order_id, opened_at DESC, id DESC)
WHERE api_order_id IS NOT NULL;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status,
ADD CONSTRAINT ck_api_orders_dispute_status CHECK (dispute_status IN (
  'none', 'negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation'
)),
ADD CONSTRAINT ck_api_orders_active_dispute_shape CHECK (
  (dispute_status = 'none' AND dispute_case_id IS NULL AND active_remedy_action = '')
  OR (
    dispute_status <> 'none'
    AND dispute_case_id IS NOT NULL
    AND active_remedy_action IN ('', 'full_refund', 'partial_refund', 'continue_fulfillment', 'other')
  )
),
ADD CONSTRAINT fk_api_orders_active_dispute
  FOREIGN KEY (dispute_case_id, id) REFERENCES dispute_cases(id, api_order_id),
ADD CONSTRAINT fk_api_orders_latest_dispute
  FOREIGN KEY (latest_dispute_case_id, id) REFERENCES dispute_cases(id, api_order_id);

ALTER TABLE api_order_dispute_settlement_proposals
ADD COLUMN fulfillment_required boolean NOT NULL DEFAULT false,
ADD COLUMN responsible_user_id uuid REFERENCES users(id),
ADD COLUMN beneficiary_user_id uuid REFERENCES users(id),
ADD COLUMN due_at timestamptz,
ADD CONSTRAINT ck_api_order_dispute_proposal_fulfillment_shape CHECK (
  (
    fulfillment_required = false
    AND responsible_user_id IS NULL
    AND beneficiary_user_id IS NULL
    AND due_at IS NULL
  )
  OR (
    fulfillment_required = true
    AND responsible_user_id IS NOT NULL
    AND beneficiary_user_id IS NOT NULL
    AND responsible_user_id <> beneficiary_user_id
    AND due_at IS NOT NULL
    AND due_at > created_at
  )
);

ALTER TABLE api_order_dispute_remedies
ALTER COLUMN created_by_admin_id DROP NOT NULL,
ADD COLUMN source text NOT NULL DEFAULT 'admin_decision',
ADD COLUMN settlement_proposal_id uuid REFERENCES api_order_dispute_settlement_proposals(id),
ADD CONSTRAINT ck_api_order_dispute_remedy_source
  CHECK (source IN ('admin_decision', 'mutual_agreement')),
ADD CONSTRAINT ck_api_order_dispute_remedy_source_shape CHECK (
  (source = 'admin_decision' AND created_by_admin_id IS NOT NULL AND settlement_proposal_id IS NULL)
  OR (source = 'mutual_agreement' AND created_by_admin_id IS NULL AND settlement_proposal_id IS NOT NULL)
);

CREATE UNIQUE INDEX ux_api_order_dispute_remedies_settlement_proposal
ON api_order_dispute_remedies(settlement_proposal_id)
WHERE settlement_proposal_id IS NOT NULL;
