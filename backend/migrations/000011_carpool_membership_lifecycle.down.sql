DROP TRIGGER IF EXISTS trg_carpool_membership_joined_application ON carpool_memberships;
DROP FUNCTION IF EXISTS enforce_carpool_membership_joined_application();

DROP INDEX IF EXISTS ix_carpool_memberships_owner_listing;
DROP INDEX IF EXISTS ix_carpool_memberships_buyer;
DROP INDEX IF EXISTS ux_carpool_memberships_active_listing_buyer;
DROP TABLE IF EXISTS carpool_memberships;

DROP TRIGGER IF EXISTS trg_carpool_join_confirmation_actor ON carpool_join_confirmations;
DROP FUNCTION IF EXISTS enforce_carpool_join_confirmation_actor();

DROP INDEX IF EXISTS ix_carpool_join_confirmations_actor;
DROP TABLE IF EXISTS carpool_join_confirmations;

DROP INDEX IF EXISTS ux_carpool_applications_one_ongoing;

UPDATE carpool_applications
SET status = 'accepted_reserved',
    reservation_expires_at = COALESCE(reservation_expires_at, join_confirmation_deadline, updated_at + interval '30 minutes'),
    joined_at = NULL
WHERE status = 'joined';

CREATE UNIQUE INDEX ux_carpool_applications_one_ongoing
ON carpool_applications(carpool_listing_id, buyer_user_id)
WHERE status IN ('pending_owner', 'accepted_reserved');

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS uq_carpool_applications_membership_identity;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_joined_at;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_join_confirmation_deadline;

ALTER TABLE carpool_applications
DROP COLUMN IF EXISTS joined_at,
DROP COLUMN IF EXISTS join_confirmation_deadline;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_session_status;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_session_status
CHECK (
  contact_session_id IS NULL
  OR status IN ('accepted_reserved', 'expired')
);

CREATE OR REPLACE FUNCTION enforce_carpool_application_contact_session()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  session_buyer uuid;
  session_seller uuid;
BEGIN
  IF NEW.contact_session_id IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT buyer_user_id, seller_user_id
  INTO session_buyer, session_seller
  FROM contact_sessions
  WHERE id = NEW.contact_session_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'contact session % not found', NEW.contact_session_id
      USING ERRCODE = '23503';
  END IF;

  IF session_buyer <> NEW.buyer_user_id THEN
    RAISE EXCEPTION 'carpool application contact session buyer mismatch'
      USING ERRCODE = '23514';
  END IF;

  IF session_seller <> NEW.owner_user_id THEN
    RAISE EXCEPTION 'carpool application contact session seller mismatch'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.status NOT IN ('accepted_reserved', 'expired') THEN
    RAISE EXCEPTION 'carpool application contact session requires accepted_reserved or expired status'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_applications_status;

ALTER TABLE carpool_applications
ADD CONSTRAINT carpool_applications_status_check
CHECK (status IN ('pending_owner', 'accepted_reserved', 'rejected', 'cancelled_by_buyer', 'expired'));
