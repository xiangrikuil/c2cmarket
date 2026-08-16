-- Remove private API-order dispute evidence only when no immutable binding exists.
-- Migration: 000105
-- Date: 2026-08-13
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM api_order_evidence_bindings) THEN
    RAISE EXCEPTION 'cannot roll back API-order evidence assets while immutable bindings exist';
  END IF;
END $$;

DROP TRIGGER IF EXISTS trg_api_order_evidence_bindings_append_only ON api_order_evidence_bindings;
DROP TRIGGER IF EXISTS trg_api_order_evidence_bindings_validate_source ON api_order_evidence_bindings;
DROP FUNCTION IF EXISTS reject_api_order_evidence_binding_mutation();
DROP FUNCTION IF EXISTS validate_api_order_evidence_binding_source();
DROP TABLE IF EXISTS api_order_evidence_bindings;
DROP TABLE IF EXISTS api_order_evidence_assets;
