-- Carpool application cancel and acceptance withdrawal lifecycle.
-- 日期：2026-06-28
-- 执行者：Codex

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_applications_status;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_applications_status
CHECK (status IN ('pending_owner', 'accepted_reserved', 'joined', 'rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'));

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_session_status;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_session_status
CHECK (
  contact_session_id IS NULL
  OR status IN ('accepted_reserved', 'joined', 'expired', 'cancelled_by_buyer', 'cancelled_by_owner')
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

  IF NEW.status NOT IN ('accepted_reserved', 'joined', 'expired', 'cancelled_by_buyer', 'cancelled_by_owner') THEN
    RAISE EXCEPTION 'carpool application contact session requires accepted_reserved, joined, expired, or cancelled status'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;
