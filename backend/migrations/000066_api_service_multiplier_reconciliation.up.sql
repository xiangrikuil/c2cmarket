-- Reconcile API service model multipliers with the current positive-decimal contract.
-- Date: 2026-07-27
-- Executor: Codex

DO $$
DECLARE constraint_row record;
BEGIN
  FOR constraint_row IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'api_service_models'::regclass
      AND contype = 'c'
      AND (
        conname = 'ck_api_service_models_sub2api_multiplier'
        OR pg_get_constraintdef(oid) ~*
          'merchant_multiplier[[:space:]]*=[[:space:]]*[(]?[[:space:]]*1([.]0+)?([^0-9.]|$)'
      )
  LOOP
    EXECUTE format('ALTER TABLE api_service_models DROP CONSTRAINT %I', constraint_row.conname);
  END LOOP;
END $$;
