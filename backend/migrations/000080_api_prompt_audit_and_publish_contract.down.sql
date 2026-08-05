-- Remove prompt-audit snapshots and restore the historical grouped performance declaration.
-- Date: 2026-08-05
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_services WHERE prompt_audit_enabled IS NOT NULL)
    OR EXISTS (SELECT 1 FROM api_purchase_intents WHERE prompt_audit_enabled_snapshot IS NOT NULL)
    OR EXISTS (SELECT 1 FROM api_orders WHERE prompt_audit_enabled_snapshot IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot roll back migration 80 while prompt-audit declarations or snapshots exist';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM api_services
    WHERE NOT (
      (declared_ttft_band IS NULL AND declared_max_concurrency IS NULL AND performance_confirmed_at IS NULL)
      OR
      (declared_ttft_band IS NOT NULL AND declared_max_concurrency IS NOT NULL AND performance_confirmed_at IS NOT NULL)
    )
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 80 while independent performance declarations exist';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM api_orders
    WHERE purchase_kind = 'limited_quota_offer'
      AND quota_performance_confirmed_at_snapshot IS NULL
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 80 while limited quota orders lack historical performance confirmation';
  END IF;
END $$;

ALTER TABLE api_services
ADD CONSTRAINT ck_api_services_performance_declaration
CHECK (
  (declared_ttft_band IS NULL AND declared_max_concurrency IS NULL AND performance_confirmed_at IS NULL)
  OR
  (declared_ttft_band IS NOT NULL AND declared_max_concurrency IS NOT NULL AND performance_confirmed_at IS NOT NULL)
);

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

ALTER TABLE api_orders
DROP COLUMN prompt_audit_enabled_snapshot;

ALTER TABLE api_purchase_intents
DROP COLUMN prompt_audit_enabled_snapshot;

ALTER TABLE api_services
DROP COLUMN prompt_audit_enabled;
