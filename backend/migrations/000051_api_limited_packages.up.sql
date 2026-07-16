-- Add stable limited-package inventory, model associations, and order expiry snapshots.
-- Date: 2026-07-16
-- Executor: Codex

ALTER TABLE api_service_packages
ADD COLUMN panel_allowance numeric(18,6),
ADD COLUMN stock_total integer NOT NULL DEFAULT 0,
ADD COLUMN stock_available integer NOT NULL DEFAULT 0;

DO $$
DECLARE constraint_row record;
BEGIN
  FOR constraint_row IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'api_service_models'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%merchant_multiplier = 1%'
  LOOP
    EXECUTE format('ALTER TABLE api_service_models DROP CONSTRAINT %I', constraint_row.conname);
  END LOOP;
END $$;

-- Historical fixed packages predate inventory. Keep them readable but require the
-- merchant to review and explicitly re-enable them under the new contract.
UPDATE api_service_packages
SET panel_allowance = 1.000000,
    duration_days = CASE WHEN duration_days IN (1, 3, 7, 30) THEN duration_days ELSE 1 END,
    enabled = false;

ALTER TABLE api_service_packages
ALTER COLUMN panel_allowance SET NOT NULL,
ADD CONSTRAINT ck_api_service_packages_limited_fields
CHECK (
  panel_allowance > 0
  AND stock_total >= 0
  AND stock_available >= 0
  AND stock_available <= stock_total
  AND duration_days IN (1, 3, 7, 30)
);

CREATE TABLE api_service_package_models (
  api_service_package_id uuid NOT NULL,
  api_service_model_id uuid NOT NULL,
  api_service_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (api_service_package_id, api_service_model_id),
  FOREIGN KEY (api_service_id, api_service_package_id)
    REFERENCES api_service_packages(api_service_id, id) ON DELETE CASCADE,
  FOREIGN KEY (api_service_id, api_service_model_id)
    REFERENCES api_service_models(api_service_id, id) ON DELETE CASCADE
);

CREATE INDEX ix_api_service_package_models_service
ON api_service_package_models(api_service_id, api_service_package_id);

ALTER TABLE api_orders
ADD COLUMN package_stock_reserved boolean NOT NULL DEFAULT false,
ADD COLUMN package_expires_at timestamptz,
ADD CONSTRAINT ck_api_orders_package_stock_reservation
CHECK (
  package_stock_reserved = false
  OR (
    billing_mode_snapshot = 'fixed_package'
    AND selected_package_id IS NOT NULL
    AND status IN ('pending_payment', 'payment_submitted', 'payment_issue')
  )
),
ADD CONSTRAINT ck_api_orders_package_expiry
CHECK (
  package_expires_at IS NULL
  OR (
    billing_mode_snapshot = 'fixed_package'
    AND delivery_submitted_at IS NOT NULL
    AND package_expires_at > delivery_submitted_at
  )
);
