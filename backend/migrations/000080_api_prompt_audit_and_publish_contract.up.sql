-- Freeze seller prompt-audit declarations while retiring seller-written performance facts.
-- Date: 2026-08-05
-- Executor: Codex

ALTER TABLE api_services
ADD COLUMN prompt_audit_enabled boolean;

ALTER TABLE api_purchase_intents
ADD COLUMN prompt_audit_enabled_snapshot boolean;

ALTER TABLE api_orders
ADD COLUMN prompt_audit_enabled_snapshot boolean;

ALTER TABLE api_services
DROP CONSTRAINT ck_api_services_performance_declaration;

ALTER TABLE api_orders
DROP CONSTRAINT ck_api_orders_kind_shape;

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
    AND quota_declared_max_concurrency_snapshot IS NULL
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
    AND quota_declared_max_concurrency_snapshot IS NOT NULL
    AND quota_declared_max_concurrency_snapshot > 0
    AND quota_performance_unverified_snapshot = true
    AND quota_delivery_eta_minutes_snapshot BETWEEN 1 AND 10
    AND quota_delivery_mode_snapshot IN ('manual', 'preimported')
    AND requested_usd_allowance_snapshot = quota_usd_allowance_snapshot
    AND cny_per_usd_allowance_snapshot = quota_cny_per_usd_snapshot
    AND selected_package_id IS NULL
    AND selected_package_snapshot IS NULL
  )
);
