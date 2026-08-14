-- Account-governance business disposition jobs and durable resource outcomes.
-- Date: 2026-08-13
-- Author: Codex

CREATE TABLE account_governance_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  governance_action_id uuid NOT NULL REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  expected_governance_version bigint NOT NULL CHECK (expected_governance_version >= 1),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
  phase text NOT NULL DEFAULT 'sales' CHECK (phase IN ('sales', 'api_orders', 'api_intents', 'carpool', 'completed')),
  cursor_resource_type text,
  cursor_id uuid,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at timestamptz NOT NULL,
  locked_at timestamptz,
  lease_expires_at timestamptz,
  completed_at timestamptz,
  last_error_code text,
  last_error_summary text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT ux_account_governance_job_action UNIQUE(governance_action_id),
  CONSTRAINT ck_account_governance_job_lease CHECK (
    (locked_at IS NULL AND lease_expires_at IS NULL)
    OR (locked_at IS NOT NULL AND lease_expires_at IS NOT NULL AND lease_expires_at > locked_at)
  ),
  CONSTRAINT ck_account_governance_job_completion CHECK (
    (status = 'completed' AND phase = 'completed' AND completed_at IS NOT NULL)
    OR (status <> 'completed' AND completed_at IS NULL)
  ),
  CONSTRAINT ck_account_governance_job_timestamps CHECK (
    created_at <= updated_at
    AND (completed_at IS NULL OR completed_at >= created_at)
  )
);

CREATE INDEX ix_account_governance_jobs_available
ON account_governance_jobs(available_at, id)
WHERE status IN ('pending', 'failed') OR (status = 'processing' AND lease_expires_at IS NOT NULL);

CREATE TABLE account_governance_resource_dispositions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_type text NOT NULL CHECK (
    resource_type IN (
      'api_service', 'api_quota_batch', 'api_quota_offer', 'api_service_promotion',
      'api_order', 'api_purchase_intent', 'carpool_listing',
      'carpool_application', 'carpool_membership'
    )
  ),
  resource_id uuid NOT NULL,
  result text NOT NULL CHECK (result IN ('cancelled', 'preserved', 'already_terminal', 'sales_stopped')),
  reason_code text NOT NULL CHECK (reason_code = 'ACCOUNT_GOVERNANCE_CANCELLED'),
  trigger_roles text[] NOT NULL,
  before_status text NOT NULL,
  after_status text NOT NULL,
  released_resource_type text,
  released_quantity numeric(18,6),
  governance_effective_at timestamptz NOT NULL,
  payment_claim_deadline_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT ux_account_governance_resource_disposition UNIQUE(resource_type, resource_id),
  CONSTRAINT ck_account_governance_disposition_roles CHECK (
    cardinality(trigger_roles) BETWEEN 1 AND 2
    AND trigger_roles <@ ARRAY['buyer', 'seller']::text[]
  ),
  CONSTRAINT ck_account_governance_disposition_release CHECK (
    (released_resource_type IS NULL AND released_quantity IS NULL)
    OR (
      released_resource_type IN ('package_stock', 'usd_allowance', 'quota_inventory_unit', 'carpool_seat')
      AND released_quantity IS NOT NULL
      AND released_quantity > 0
    )
  ),
  CONSTRAINT ck_account_governance_disposition_claim_window CHECK (
    payment_claim_deadline_at IS NULL
    OR (
      result = 'cancelled'
      AND resource_type IN ('api_order', 'carpool_application')
      AND payment_claim_deadline_at = governance_effective_at + interval '7 days'
    )
  ),
  CONSTRAINT ck_account_governance_disposition_timestamps CHECK (
    created_at >= governance_effective_at AND created_at <= updated_at
  )
);

CREATE INDEX ix_account_governance_dispositions_recent
ON account_governance_resource_dispositions(updated_at DESC, id DESC);

CREATE TABLE account_governance_disposition_actions (
  disposition_id uuid NOT NULL REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT,
  governance_action_id uuid NOT NULL REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  trigger_role text NOT NULL CHECK (trigger_role IN ('buyer', 'seller')),
  linked_at timestamptz NOT NULL,
  PRIMARY KEY (disposition_id, governance_action_id)
);

CREATE INDEX ix_account_governance_disposition_actions_action
ON account_governance_disposition_actions(governance_action_id, disposition_id);

ALTER TABLE api_orders
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT,
  ADD COLUMN governance_cancelled_at timestamptz,
  ADD CONSTRAINT ck_api_orders_governance_cancellation CHECK (
    (governance_disposition_id IS NULL AND governance_cancelled_at IS NULL)
    OR (
      governance_disposition_id IS NOT NULL
      AND governance_cancelled_at IS NOT NULL
      AND status = 'cancelled'
      AND cancel_reason = 'ACCOUNT_GOVERNANCE_CANCELLED'
    )
  );

CREATE UNIQUE INDEX ux_api_orders_governance_disposition
ON api_orders(governance_disposition_id)
WHERE governance_disposition_id IS NOT NULL;

ALTER TABLE api_purchase_intents
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT,
  ADD COLUMN governance_closed_at timestamptz;

CREATE UNIQUE INDEX ux_api_purchase_intents_governance_disposition
ON api_purchase_intents(governance_disposition_id)
WHERE governance_disposition_id IS NOT NULL;

ALTER TABLE carpool_applications
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT,
  ADD COLUMN governance_cancelled_at timestamptz;

CREATE UNIQUE INDEX ux_carpool_applications_governance_disposition
ON carpool_applications(governance_disposition_id)
WHERE governance_disposition_id IS NOT NULL;

ALTER TABLE api_services
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT;

ALTER TABLE api_quota_batches
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT;

ALTER TABLE api_quota_offers
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT;

ALTER TABLE api_service_promotions
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT,
  ADD COLUMN stopped_by_governance_action_id uuid REFERENCES account_governance_actions(id) ON DELETE RESTRICT,
  DROP CONSTRAINT ck_api_service_promotions_stop_facts,
  ADD CONSTRAINT ck_api_service_promotions_stop_facts CHECK (
    (stopped_at IS NULL AND stopped_by_admin_id IS NULL AND stopped_by_governance_action_id IS NULL AND stopped_reason = '')
    OR
    (
      stopped_at IS NOT NULL
      AND (stopped_by_admin_id IS NULL) <> (stopped_by_governance_action_id IS NULL)
      AND trim(stopped_reason) <> ''
      AND char_length(stopped_reason) <= 500
    )
  );

ALTER TABLE carpool_listings
  ADD COLUMN governance_disposition_id uuid REFERENCES account_governance_resource_dispositions(id) ON DELETE RESTRICT;
