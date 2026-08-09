-- Restore the pre-V1 dispute status constraints without rewriting business rows.
-- Date: 2026-08-09
-- Executor: Codex

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_dispute_status,
ADD CONSTRAINT ck_api_orders_dispute_status
CHECK (dispute_status IN ('none', 'open', 'closed'));

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_status,
ADD CONSTRAINT ck_dispute_cases_status
CHECK (status IN ('open', 'waiting_info', 'resolved', 'closed'));
