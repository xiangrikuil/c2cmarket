-- Roll back account-governance business disposition before business outcomes exist.
-- Date: 2026-08-13
-- Author: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM account_governance_resource_dispositions)
     OR EXISTS (SELECT 1 FROM account_governance_jobs) THEN
    RAISE EXCEPTION 'cannot roll back account governance business disposition after records exist'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

ALTER TABLE carpool_listings DROP COLUMN IF EXISTS governance_disposition_id;
ALTER TABLE api_service_promotions
  DROP CONSTRAINT IF EXISTS ck_api_service_promotions_stop_facts,
  DROP COLUMN IF EXISTS stopped_by_governance_action_id,
  DROP COLUMN IF EXISTS governance_disposition_id,
  ADD CONSTRAINT ck_api_service_promotions_stop_facts CHECK (
    (stopped_at IS NULL AND stopped_by_admin_id IS NULL AND stopped_reason = '')
    OR
    (stopped_at IS NOT NULL AND stopped_by_admin_id IS NOT NULL AND trim(stopped_reason) <> '' AND char_length(stopped_reason) <= 500)
  );
ALTER TABLE api_quota_offers DROP COLUMN IF EXISTS governance_disposition_id;
ALTER TABLE api_quota_batches DROP COLUMN IF EXISTS governance_disposition_id;
ALTER TABLE api_services DROP COLUMN IF EXISTS governance_disposition_id;

DROP INDEX IF EXISTS ux_carpool_applications_governance_disposition;
ALTER TABLE carpool_applications
  DROP COLUMN IF EXISTS governance_cancelled_at,
  DROP COLUMN IF EXISTS governance_disposition_id;

DROP INDEX IF EXISTS ux_api_purchase_intents_governance_disposition;
ALTER TABLE api_purchase_intents
  DROP COLUMN IF EXISTS governance_closed_at,
  DROP COLUMN IF EXISTS governance_disposition_id;

DROP INDEX IF EXISTS ux_api_orders_governance_disposition;
ALTER TABLE api_orders
  DROP CONSTRAINT IF EXISTS ck_api_orders_governance_cancellation,
  DROP COLUMN IF EXISTS governance_cancelled_at,
  DROP COLUMN IF EXISTS governance_disposition_id;

DROP INDEX IF EXISTS ix_account_governance_disposition_actions_action;
DROP TABLE IF EXISTS account_governance_disposition_actions;

DROP INDEX IF EXISTS ix_account_governance_dispositions_recent;
DROP TABLE IF EXISTS account_governance_resource_dispositions;

DROP INDEX IF EXISTS ix_account_governance_jobs_available;
DROP TABLE IF EXISTS account_governance_jobs;
