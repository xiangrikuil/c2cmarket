-- 新增限时 API 额度批次、固定规格、放量轮次与订单冻结快照。
-- 日期：2026-07-19
-- 执行者：Codex

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'api_service_models'::regclass
      AND conname = 'ck_api_service_models_sub2api_multiplier'
  ) THEN
    ALTER TABLE api_service_models
    ADD CONSTRAINT ck_api_service_models_sub2api_multiplier
    CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000);
  END IF;
END $$;

ALTER TABLE api_services
ADD COLUMN declared_ttft_band text,
ADD COLUMN recommended_concurrency integer,
ADD COLUMN performance_confirmed_at timestamptz,
ADD CONSTRAINT ck_api_services_declared_ttft_band
  CHECK (
    declared_ttft_band IS NULL
    OR declared_ttft_band IN ('under_1s', '1_to_3s', '3_to_5s', '5_to_10s', 'over_10s')
  ),
ADD CONSTRAINT ck_api_services_recommended_concurrency
  CHECK (recommended_concurrency IS NULL OR recommended_concurrency > 0),
ADD CONSTRAINT ck_api_services_performance_declaration
  CHECK (
    (declared_ttft_band IS NULL AND recommended_concurrency IS NULL AND performance_confirmed_at IS NULL)
    OR
    (declared_ttft_band IS NOT NULL AND recommended_concurrency IS NOT NULL AND performance_confirmed_at IS NOT NULL)
  );

CREATE TABLE api_quota_batches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  source_type text NOT NULL CHECK (source_type IN ('sub2api', 'new_api_proxy', 'self_hosted', 'other')),
  source_label text,
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'paused', 'archived')),
  declared_total_usd_allowance numeric(18,6) NOT NULL CHECK (declared_total_usd_allowance > 0),
  unallocated_usd_allowance numeric(18,6) NOT NULL CHECK (unallocated_usd_allowance >= 0),
  sale_cutoff_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  source_confirmed_at timestamptz NOT NULL,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, api_service_id),
  UNIQUE (id, api_service_id, owner_user_id),
  FOREIGN KEY (api_service_id, owner_user_id) REFERENCES api_services(id, owner_user_id),
  CHECK (unallocated_usd_allowance <= declared_total_usd_allowance),
  CHECK (sale_cutoff_at <= expires_at - interval '1 hour'),
  CHECK (source_type <> 'other' OR (source_label IS NOT NULL AND trim(source_label) <> '')),
  CHECK (source_type = 'other' OR source_label IS NULL),
  CHECK (
    (status = 'draft' AND published_at IS NULL)
    OR (status <> 'draft' AND published_at IS NOT NULL)
  )
);

CREATE INDEX ix_api_quota_batches_owner
ON api_quota_batches(owner_user_id, updated_at DESC, id DESC);

CREATE INDEX ix_api_quota_batches_service
ON api_quota_batches(api_service_id, status, expires_at, id DESC);

CREATE TABLE api_quota_offers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id uuid NOT NULL,
  api_service_id uuid NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  previous_version_id uuid REFERENCES api_quota_offers(id),
  distribution_system text NOT NULL,
  name text NOT NULL CHECK (trim(name) <> ''),
  usd_allowance numeric(18,6) NOT NULL CHECK (usd_allowance > 0),
  price_cny numeric(12,2) NOT NULL CHECK (price_cny > 0),
  model_multiplier numeric(8,4) NOT NULL DEFAULT 1.0000 CHECK (model_multiplier > 0),
  delivery_mode text NOT NULL CHECK (delivery_mode IN ('manual', 'preimported')),
  delivery_eta_minutes integer NOT NULL CHECK (delivery_eta_minutes BETWEEN 1 AND 10),
  sale_mode text NOT NULL CHECK (sale_mode IN ('continuous', 'scheduled')),
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'paused', 'archived')),
  sort_order integer NOT NULL DEFAULT 0,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, owner_user_id),
  UNIQUE (id, api_service_id),
  UNIQUE (id, batch_id, api_service_id, owner_user_id),
  FOREIGN KEY (batch_id, api_service_id, owner_user_id)
    REFERENCES api_quota_batches(id, api_service_id, owner_user_id),
  FOREIGN KEY (api_service_id, distribution_system)
    REFERENCES api_services(id, distribution_system),
  CHECK (
    (status = 'draft' AND published_at IS NULL)
    OR (status <> 'draft' AND published_at IS NOT NULL)
  )
);

CREATE INDEX ix_api_quota_offers_public
ON api_quota_offers(status, sort_order, updated_at DESC, id DESC);

CREATE INDEX ix_api_quota_offers_batch
ON api_quota_offers(batch_id, status, sort_order, id DESC);

CREATE TABLE api_quota_sale_rounds (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id uuid NOT NULL,
  api_service_id uuid NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  name text NOT NULL CHECK (trim(name) <> ''),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'closed', 'cancelled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, batch_id, api_service_id, owner_user_id),
  FOREIGN KEY (batch_id, api_service_id, owner_user_id)
    REFERENCES api_quota_batches(id, api_service_id, owner_user_id),
  CHECK (ends_at > starts_at)
);

CREATE INDEX ix_api_quota_sale_rounds_batch
ON api_quota_sale_rounds(batch_id, starts_at, id);

CREATE INDEX ix_api_quota_sale_rounds_open_window
ON api_quota_sale_rounds(starts_at, ends_at, id)
WHERE status = 'scheduled';

CREATE TABLE api_quota_allocations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id uuid NOT NULL,
  offer_id uuid NOT NULL,
  api_service_id uuid NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  sale_round_id uuid,
  sale_mode text NOT NULL CHECK (sale_mode IN ('continuous', 'scheduled')),
  copy_limit integer NOT NULL CHECK (copy_limit > 0),
  allocated_usd_allowance numeric(18,6) NOT NULL CHECK (allocated_usd_allowance > 0),
  returned_usd_allowance numeric(18,6) NOT NULL DEFAULT 0 CHECK (returned_usd_allowance >= 0),
  status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'closed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, batch_id, offer_id),
  FOREIGN KEY (offer_id, batch_id, api_service_id, owner_user_id)
    REFERENCES api_quota_offers(id, batch_id, api_service_id, owner_user_id),
  FOREIGN KEY (sale_round_id, batch_id, api_service_id, owner_user_id)
    REFERENCES api_quota_sale_rounds(id, batch_id, api_service_id, owner_user_id),
  CHECK (returned_usd_allowance <= allocated_usd_allowance),
  CHECK (
    (sale_mode = 'continuous' AND sale_round_id IS NULL)
    OR (sale_mode = 'scheduled' AND sale_round_id IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_api_quota_allocations_continuous_offer
ON api_quota_allocations(offer_id)
WHERE sale_round_id IS NULL AND status IN ('planned', 'active');

CREATE UNIQUE INDEX ux_api_quota_allocations_round_offer
ON api_quota_allocations(sale_round_id, offer_id)
WHERE sale_round_id IS NOT NULL;

CREATE INDEX ix_api_quota_allocations_round
ON api_quota_allocations(sale_round_id, status, offer_id);

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_check3,
ADD COLUMN purchase_kind text NOT NULL DEFAULT 'api_service',
ADD COLUMN api_quota_batch_id uuid,
ADD COLUMN api_quota_offer_id uuid,
ADD COLUMN api_quota_sale_round_id uuid,
ADD COLUMN api_quota_allocation_id uuid,
ADD COLUMN api_quota_inventory_unit_id uuid,
ADD COLUMN quota_offer_snapshot jsonb,
ADD CONSTRAINT ck_api_purchase_intents_purchase_kind
  CHECK (purchase_kind IN ('api_service', 'limited_quota_offer'));

ALTER TABLE api_orders
ADD COLUMN purchase_kind text NOT NULL DEFAULT 'api_service',
ADD COLUMN api_quota_batch_id uuid,
ADD COLUMN api_quota_offer_id uuid,
ADD COLUMN api_quota_sale_round_id uuid,
ADD COLUMN api_quota_allocation_id uuid,
ADD COLUMN api_quota_inventory_unit_id uuid,
ADD COLUMN quota_offer_snapshot jsonb,
ADD COLUMN quota_offer_name_snapshot text,
ADD COLUMN quota_usd_allowance_snapshot numeric(18,6),
ADD COLUMN quota_price_cny_snapshot numeric(12,2),
ADD COLUMN quota_cny_per_usd_snapshot numeric(18,6),
ADD COLUMN quota_model_multiplier_snapshot numeric(8,4),
ADD COLUMN quota_sale_cutoff_at_snapshot timestamptz,
ADD COLUMN quota_expires_at_snapshot timestamptz,
ADD COLUMN quota_sale_mode_snapshot text,
ADD COLUMN quota_round_starts_at_snapshot timestamptz,
ADD COLUMN quota_round_ends_at_snapshot timestamptz,
ADD COLUMN quota_distribution_system_snapshot text,
ADD COLUMN quota_ttft_band_snapshot text,
ADD COLUMN quota_recommended_concurrency_snapshot integer,
ADD COLUMN quota_performance_confirmed_at_snapshot timestamptz,
ADD COLUMN quota_performance_unverified_snapshot boolean,
ADD COLUMN quota_delivery_eta_minutes_snapshot integer,
ADD COLUMN quota_delivery_mode_snapshot text,
ADD CONSTRAINT ck_api_orders_purchase_kind
  CHECK (purchase_kind IN ('api_service', 'limited_quota_offer'));

CREATE TABLE api_quota_inventory_units (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  allocation_id uuid NOT NULL,
  batch_id uuid NOT NULL,
  offer_id uuid NOT NULL,
  usd_allowance numeric(18,6) NOT NULL CHECK (usd_allowance > 0),
  status text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'reserved', 'consumed', 'retired')),
  reserved_order_id uuid REFERENCES api_orders(id),
  reserved_at timestamptz,
  consumed_at timestamptz,
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, allocation_id, offer_id, batch_id),
  FOREIGN KEY (allocation_id, batch_id, offer_id)
    REFERENCES api_quota_allocations(id, batch_id, offer_id),
  CHECK (
    (status = 'available' AND reserved_order_id IS NULL AND reserved_at IS NULL AND consumed_at IS NULL AND retired_at IS NULL)
    OR (status = 'reserved' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND consumed_at IS NULL AND retired_at IS NULL)
    OR (status = 'consumed' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND consumed_at IS NOT NULL AND retired_at IS NULL)
    OR (status = 'retired' AND consumed_at IS NULL AND retired_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_api_quota_inventory_units_reserved_order
ON api_quota_inventory_units(reserved_order_id)
WHERE reserved_order_id IS NOT NULL;

CREATE INDEX ix_api_quota_inventory_units_available
ON api_quota_inventory_units(allocation_id, id)
WHERE status = 'available';

CREATE INDEX ix_api_quota_inventory_units_offer_status
ON api_quota_inventory_units(offer_id, status, id);

ALTER TABLE api_purchase_intents
ADD CONSTRAINT fk_api_purchase_intents_quota_offer
  FOREIGN KEY (api_quota_offer_id, api_quota_batch_id, api_service_id, owner_user_id)
  REFERENCES api_quota_offers(id, batch_id, api_service_id, owner_user_id),
ADD CONSTRAINT fk_api_purchase_intents_quota_round
  FOREIGN KEY (api_quota_sale_round_id, api_quota_batch_id, api_service_id, owner_user_id)
  REFERENCES api_quota_sale_rounds(id, batch_id, api_service_id, owner_user_id),
ADD CONSTRAINT fk_api_purchase_intents_quota_inventory
  FOREIGN KEY (api_quota_inventory_unit_id, api_quota_allocation_id, api_quota_offer_id, api_quota_batch_id)
  REFERENCES api_quota_inventory_units(id, allocation_id, offer_id, batch_id);

ALTER TABLE api_orders
ADD CONSTRAINT fk_api_orders_quota_offer
  FOREIGN KEY (api_quota_offer_id, api_quota_batch_id, api_service_id, seller_user_id)
  REFERENCES api_quota_offers(id, batch_id, api_service_id, owner_user_id),
ADD CONSTRAINT fk_api_orders_quota_round
  FOREIGN KEY (api_quota_sale_round_id, api_quota_batch_id, api_service_id, seller_user_id)
  REFERENCES api_quota_sale_rounds(id, batch_id, api_service_id, owner_user_id),
ADD CONSTRAINT fk_api_orders_quota_inventory
  FOREIGN KEY (api_quota_inventory_unit_id, api_quota_allocation_id, api_quota_offer_id, api_quota_batch_id)
  REFERENCES api_quota_inventory_units(id, allocation_id, offer_id, batch_id);

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS ck_api_intent_billing_selection;

ALTER TABLE api_purchase_intents
ADD CONSTRAINT ck_api_purchase_intents_kind_shape
CHECK (
  (
    purchase_kind = 'api_service'
    AND api_quota_batch_id IS NULL
    AND api_quota_offer_id IS NULL
    AND api_quota_sale_round_id IS NULL
    AND api_quota_allocation_id IS NULL
    AND api_quota_inventory_unit_id IS NULL
    AND quota_offer_snapshot IS NULL
    AND (
      (
        billing_mode_snapshot = 'fixed_package'
        AND selected_package_id IS NOT NULL
        AND selected_package_snapshot IS NOT NULL
        AND requested_usd_allowance IS NULL
      )
      OR (
        billing_mode_snapshot <> 'fixed_package'
        AND selected_package_id IS NULL
      )
    )
  )
  OR (
    purchase_kind = 'limited_quota_offer'
    AND api_quota_batch_id IS NOT NULL
    AND api_quota_offer_id IS NOT NULL
    AND api_quota_allocation_id IS NOT NULL
    AND api_quota_inventory_unit_id IS NOT NULL
    AND quota_offer_snapshot IS NOT NULL
    AND requested_usd_allowance IS NOT NULL
    AND requested_usd_allowance > 0
    AND selected_package_id IS NULL
    AND selected_package_snapshot IS NULL
  )
);

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_metered_quota_snapshot;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_kind_shape
CHECK (
  (
    purchase_kind = 'api_service'
    AND api_quota_batch_id IS NULL
    AND api_quota_offer_id IS NULL
    AND api_quota_sale_round_id IS NULL
    AND api_quota_allocation_id IS NULL
    AND api_quota_inventory_unit_id IS NULL
    AND quota_offer_snapshot IS NULL
    AND quota_offer_name_snapshot IS NULL
    AND quota_usd_allowance_snapshot IS NULL
    AND quota_price_cny_snapshot IS NULL
    AND quota_cny_per_usd_snapshot IS NULL
    AND quota_model_multiplier_snapshot IS NULL
    AND quota_sale_cutoff_at_snapshot IS NULL
    AND quota_expires_at_snapshot IS NULL
    AND quota_sale_mode_snapshot IS NULL
    AND quota_round_starts_at_snapshot IS NULL
    AND quota_round_ends_at_snapshot IS NULL
    AND quota_distribution_system_snapshot IS NULL
    AND quota_ttft_band_snapshot IS NULL
    AND quota_recommended_concurrency_snapshot IS NULL
    AND quota_performance_confirmed_at_snapshot IS NULL
    AND quota_performance_unverified_snapshot IS NULL
    AND quota_delivery_eta_minutes_snapshot IS NULL
    AND quota_delivery_mode_snapshot IS NULL
    AND (
      (
        billing_mode_snapshot = 'metered_usd_quota'
        AND requested_usd_allowance_snapshot IS NOT NULL
        AND requested_usd_allowance_snapshot > 0
        AND cny_per_usd_allowance_snapshot IS NOT NULL
        AND cny_per_usd_allowance_snapshot > 0
      )
      OR (
        billing_mode_snapshot <> 'metered_usd_quota'
        AND requested_usd_allowance_snapshot IS NULL
        AND cny_per_usd_allowance_snapshot IS NULL
      )
    )
  )
  OR (
    purchase_kind = 'limited_quota_offer'
    AND api_quota_batch_id IS NOT NULL
    AND api_quota_offer_id IS NOT NULL
    AND api_quota_allocation_id IS NOT NULL
    AND api_quota_inventory_unit_id IS NOT NULL
    AND quota_offer_snapshot IS NOT NULL
    AND quota_offer_name_snapshot IS NOT NULL
    AND trim(quota_offer_name_snapshot) <> ''
    AND quota_usd_allowance_snapshot IS NOT NULL
    AND quota_usd_allowance_snapshot > 0
    AND quota_price_cny_snapshot IS NOT NULL
    AND quota_price_cny_snapshot > 0
    AND quota_cny_per_usd_snapshot IS NOT NULL
    AND quota_cny_per_usd_snapshot > 0
    AND quota_model_multiplier_snapshot IS NOT NULL
    AND quota_model_multiplier_snapshot > 0
    AND quota_sale_cutoff_at_snapshot IS NOT NULL
    AND quota_expires_at_snapshot IS NOT NULL
    AND quota_sale_cutoff_at_snapshot <= quota_expires_at_snapshot - interval '1 hour'
    AND quota_sale_mode_snapshot IN ('continuous', 'scheduled')
    AND (
      (quota_sale_mode_snapshot = 'continuous' AND api_quota_sale_round_id IS NULL AND quota_round_starts_at_snapshot IS NULL AND quota_round_ends_at_snapshot IS NULL)
      OR
      (quota_sale_mode_snapshot = 'scheduled' AND api_quota_sale_round_id IS NOT NULL AND quota_round_starts_at_snapshot IS NOT NULL AND quota_round_ends_at_snapshot IS NOT NULL)
    )
    AND quota_distribution_system_snapshot IN ('sub2api', 'new_api_proxy', 'other')
    AND quota_recommended_concurrency_snapshot IS NOT NULL
    AND quota_recommended_concurrency_snapshot > 0
    AND quota_performance_confirmed_at_snapshot IS NOT NULL
    AND quota_performance_unverified_snapshot = true
    AND quota_delivery_eta_minutes_snapshot BETWEEN 1 AND 10
    AND quota_delivery_mode_snapshot IN ('manual', 'preimported')
    AND requested_usd_allowance_snapshot = quota_usd_allowance_snapshot
    AND cny_per_usd_allowance_snapshot = quota_cny_per_usd_snapshot
    AND selected_package_id IS NULL
    AND selected_package_snapshot IS NULL
  )
);

CREATE TABLE api_quota_round_claims (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_round_id uuid NOT NULL,
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  api_order_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (sale_round_id, buyer_user_id),
  UNIQUE (api_order_id),
  FOREIGN KEY (api_order_id, buyer_user_id) REFERENCES api_orders(id, buyer_user_id),
  FOREIGN KEY (sale_round_id) REFERENCES api_quota_sale_rounds(id)
);

CREATE INDEX ix_api_quota_round_claims_buyer
ON api_quota_round_claims(buyer_user_id, created_at DESC, id DESC);
