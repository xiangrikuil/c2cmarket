-- The removed pre-launch workflow data cannot be reconstructed.
-- Date: 2026-08-17
-- Executor: Codex

DO $$
BEGIN
  RAISE EXCEPTION 'migration 111 cannot be rolled back after carpool workflow data cleanup';
END;
$$;
