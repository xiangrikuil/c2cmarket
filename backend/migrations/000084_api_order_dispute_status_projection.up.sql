-- Extend API order dispute phases and keep the order projection independently constrained.
-- Date: 2026-08-09
-- Executor: Codex

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS dispute_cases_status_check,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status
CHECK (status IN ('negotiating', 'open', 'waiting_info', 'resolved', 'closed'));

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS api_orders_dispute_status_check,
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status,
ADD CONSTRAINT ck_api_orders_dispute_status
CHECK (dispute_status IN (
  'none',
  'negotiating',
  'open',
  'awaiting_fulfillment',
  'fulfillment_confirmation',
  'closed'
));
