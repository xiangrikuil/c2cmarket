-- Restore the single dispute pointer only when no new history would be lost.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT api_order_id FROM dispute_cases
    WHERE api_order_id IS NOT NULL
    GROUP BY api_order_id HAVING count(*) > 1
  ) OR EXISTS (
    SELECT 1 FROM api_order_dispute_remedies WHERE source = 'mutual_agreement'
  ) THEN
    RAISE EXCEPTION 'cannot roll back active dispute history after new history or mutual remedies exist';
  END IF;
END $$;

DROP INDEX IF EXISTS ux_api_order_dispute_remedies_settlement_proposal;

ALTER TABLE api_order_dispute_remedies
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source_shape,
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_remedy_source,
DROP COLUMN IF EXISTS settlement_proposal_id,
DROP COLUMN IF EXISTS source,
ALTER COLUMN created_by_admin_id SET NOT NULL;

ALTER TABLE api_order_dispute_settlement_proposals
DROP CONSTRAINT IF EXISTS ck_api_order_dispute_proposal_fulfillment_shape,
DROP COLUMN IF EXISTS due_at,
DROP COLUMN IF EXISTS beneficiary_user_id,
DROP COLUMN IF EXISTS responsible_user_id,
DROP COLUMN IF EXISTS fulfillment_required;

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS fk_api_orders_latest_dispute,
DROP CONSTRAINT IF EXISTS fk_api_orders_active_dispute,
DROP CONSTRAINT IF EXISTS ck_api_orders_active_dispute_shape,
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status;

UPDATE api_orders
SET dispute_case_id = latest_dispute_case_id,
    dispute_status = CASE WHEN latest_dispute_case_id IS NULL THEN 'none' ELSE 'closed' END;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_dispute_status CHECK (dispute_status IN (
  'none', 'negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation', 'closed'
)),
DROP COLUMN IF EXISTS active_remedy_action,
DROP COLUMN IF EXISTS latest_dispute_case_id;

DROP INDEX IF EXISTS ix_dispute_cases_api_order_history;
DROP INDEX IF EXISTS ux_dispute_cases_one_active_api_order;

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_active_api_order,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_api_order_link,
DROP CONSTRAINT IF EXISTS uq_dispute_cases_id_api_order,
DROP CONSTRAINT IF EXISTS fk_dispute_cases_api_order,
DROP COLUMN IF EXISTS active,
DROP COLUMN IF EXISTS api_order_id;
