DROP TRIGGER IF EXISTS trg_carpool_completion_confirmation_actor ON carpool_completion_confirmations;
DROP FUNCTION IF EXISTS enforce_carpool_completion_confirmation_actor();

DROP INDEX IF EXISTS ix_carpool_completion_confirmations_actor;
DROP TABLE IF EXISTS carpool_completion_confirmations;

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS fk_carpool_membership_cycle_term;

DROP INDEX IF EXISTS ix_carpool_cycle_terms_owner;
DROP TABLE IF EXISTS carpool_cycle_terms;

UPDATE carpool_memberships
SET status = 'active',
    ended_at = NULL,
    ended_reason = '',
    ended_by_user_id = NULL
WHERE status = 'completed';

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS ck_carpool_membership_end_state;

ALTER TABLE carpool_memberships
ADD CONSTRAINT carpool_memberships_check
CHECK (
  (status = 'active' AND ended_at IS NULL)
  OR (status <> 'active' AND ended_at IS NOT NULL)
);

ALTER TABLE carpool_memberships
DROP COLUMN IF EXISTS ended_by_user_id,
DROP COLUMN IF EXISTS ended_reason,
DROP COLUMN IF EXISTS cycle_term_id;

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS uq_carpool_listings_id_owner;

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS ck_carpool_memberships_status;

ALTER TABLE carpool_memberships
ADD CONSTRAINT carpool_memberships_status_check
CHECK (status IN ('active', 'left', 'removed'));
