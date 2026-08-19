-- Store a private note for the carpool owner on each membership.
-- Date: 2026-08-18
-- Executor: Codex

ALTER TABLE carpool_memberships
ADD COLUMN owner_note text NOT NULL DEFAULT '';
