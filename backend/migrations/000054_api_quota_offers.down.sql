-- 回退限时 API 额度批次、固定规格、放量轮次与订单冻结快照。
-- 日期：2026-07-19
-- 执行者：Codex

DROP INDEX IF EXISTS ix_api_quota_round_claims_buyer;
DROP TABLE IF EXISTS api_quota_round_claims;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_kind_shape,
DROP CONSTRAINT IF EXISTS fk_api_orders_quota_inventory,
DROP CONSTRAINT IF EXISTS fk_api_orders_quota_round,
DROP CONSTRAINT IF EXISTS fk_api_orders_quota_offer;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS ck_api_purchase_intents_kind_shape,
DROP CONSTRAINT IF EXISTS fk_api_purchase_intents_quota_inventory,
DROP CONSTRAINT IF EXISTS fk_api_purchase_intents_quota_round,
DROP CONSTRAINT IF EXISTS fk_api_purchase_intents_quota_offer;

DROP INDEX IF EXISTS ix_api_quota_inventory_units_offer_status;
DROP INDEX IF EXISTS ix_api_quota_inventory_units_available;
DROP INDEX IF EXISTS ux_api_quota_inventory_units_reserved_order;
DROP TABLE IF EXISTS api_quota_inventory_units;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_purchase_kind,
DROP COLUMN IF EXISTS quota_delivery_mode_snapshot,
DROP COLUMN IF EXISTS quota_delivery_eta_minutes_snapshot,
DROP COLUMN IF EXISTS quota_performance_unverified_snapshot,
DROP COLUMN IF EXISTS quota_performance_confirmed_at_snapshot,
DROP COLUMN IF EXISTS quota_recommended_concurrency_snapshot,
DROP COLUMN IF EXISTS quota_ttft_band_snapshot,
DROP COLUMN IF EXISTS quota_distribution_system_snapshot,
DROP COLUMN IF EXISTS quota_round_ends_at_snapshot,
DROP COLUMN IF EXISTS quota_round_starts_at_snapshot,
DROP COLUMN IF EXISTS quota_sale_mode_snapshot,
DROP COLUMN IF EXISTS quota_expires_at_snapshot,
DROP COLUMN IF EXISTS quota_sale_cutoff_at_snapshot,
DROP COLUMN IF EXISTS quota_model_multiplier_snapshot,
DROP COLUMN IF EXISTS quota_cny_per_usd_snapshot,
DROP COLUMN IF EXISTS quota_price_cny_snapshot,
DROP COLUMN IF EXISTS quota_usd_allowance_snapshot,
DROP COLUMN IF EXISTS quota_offer_name_snapshot,
DROP COLUMN IF EXISTS quota_offer_snapshot,
DROP COLUMN IF EXISTS api_quota_inventory_unit_id,
DROP COLUMN IF EXISTS api_quota_allocation_id,
DROP COLUMN IF EXISTS api_quota_sale_round_id,
DROP COLUMN IF EXISTS api_quota_offer_id,
DROP COLUMN IF EXISTS api_quota_batch_id,
DROP COLUMN IF EXISTS purchase_kind;

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS ck_api_purchase_intents_purchase_kind,
DROP COLUMN IF EXISTS quota_offer_snapshot,
DROP COLUMN IF EXISTS api_quota_inventory_unit_id,
DROP COLUMN IF EXISTS api_quota_allocation_id,
DROP COLUMN IF EXISTS api_quota_sale_round_id,
DROP COLUMN IF EXISTS api_quota_offer_id,
DROP COLUMN IF EXISTS api_quota_batch_id,
DROP COLUMN IF EXISTS purchase_kind;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_metered_quota_snapshot
CHECK (
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
);

ALTER TABLE api_purchase_intents
ADD CONSTRAINT ck_api_intent_billing_selection
CHECK (
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
);

DROP INDEX IF EXISTS ix_api_quota_allocations_round;
DROP INDEX IF EXISTS ux_api_quota_allocations_round_offer;
DROP INDEX IF EXISTS ux_api_quota_allocations_continuous_offer;
DROP TABLE IF EXISTS api_quota_allocations;

DROP INDEX IF EXISTS ix_api_quota_sale_rounds_open_window;
DROP INDEX IF EXISTS ix_api_quota_sale_rounds_batch;
DROP TABLE IF EXISTS api_quota_sale_rounds;

DROP INDEX IF EXISTS ix_api_quota_offers_batch;
DROP INDEX IF EXISTS ix_api_quota_offers_public;
DROP TABLE IF EXISTS api_quota_offers;

DROP INDEX IF EXISTS ix_api_quota_batches_service;
DROP INDEX IF EXISTS ix_api_quota_batches_owner;
DROP TABLE IF EXISTS api_quota_batches;

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS ck_api_services_performance_declaration,
DROP CONSTRAINT IF EXISTS ck_api_services_recommended_concurrency,
DROP CONSTRAINT IF EXISTS ck_api_services_declared_ttft_band,
DROP COLUMN IF EXISTS performance_confirmed_at,
DROP COLUMN IF EXISTS recommended_concurrency,
DROP COLUMN IF EXISTS declared_ttft_band;

ALTER TABLE api_service_models
DROP CONSTRAINT IF EXISTS ck_api_service_models_sub2api_multiplier;
