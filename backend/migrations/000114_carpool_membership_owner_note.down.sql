-- Remove the private owner note column.
-- Date: 2026-08-18
-- Executor: Codex

ALTER TABLE carpool_memberships
DROP COLUMN owner_note;
