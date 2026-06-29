-- Carpool join confirmation and membership lifecycle.
-- 日期：2026-06-21
-- 执行者：Codex

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS carpool_applications_status_check;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_applications_status
CHECK (status IN ('pending_owner', 'accepted_reserved', 'joined', 'rejected', 'cancelled_by_buyer', 'expired'));

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_session_status;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_session_status
CHECK (
  contact_session_id IS NULL
  OR status IN ('accepted_reserved', 'joined', 'expired')
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

  IF NEW.status NOT IN ('accepted_reserved', 'joined', 'expired') THEN
    RAISE EXCEPTION 'carpool application contact session requires accepted_reserved, joined, or expired status'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

ALTER TABLE carpool_applications
ADD COLUMN join_confirmation_deadline timestamptz,
ADD COLUMN joined_at timestamptz;

UPDATE carpool_applications
SET join_confirmation_deadline = reservation_expires_at
WHERE status = 'accepted_reserved'
  AND join_confirmation_deadline IS NULL;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_join_confirmation_deadline
CHECK (
  (status = 'accepted_reserved' AND join_confirmation_deadline IS NOT NULL)
  OR status <> 'accepted_reserved'
);

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_joined_at
CHECK (
  (status = 'joined' AND joined_at IS NOT NULL)
  OR status <> 'joined'
);

ALTER TABLE carpool_applications
ADD CONSTRAINT uq_carpool_applications_membership_identity
UNIQUE (id, carpool_listing_id, buyer_user_id, owner_user_id, product_plan_id);

DROP INDEX IF EXISTS ux_carpool_applications_one_ongoing;

CREATE UNIQUE INDEX ux_carpool_applications_one_ongoing
ON carpool_applications(carpool_listing_id, buyer_user_id)
WHERE status IN ('pending_owner', 'accepted_reserved');

CREATE TABLE carpool_join_confirmations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_application_id uuid NOT NULL REFERENCES carpool_applications(id) ON DELETE CASCADE,
  actor_user_id uuid NOT NULL REFERENCES users(id),
  actor_role text NOT NULL CHECK (actor_role IN ('buyer', 'owner')),
  confirmed_at timestamptz NOT NULL,
  request_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(carpool_application_id, actor_role)
);

CREATE INDEX ix_carpool_join_confirmations_actor
ON carpool_join_confirmations(actor_user_id, confirmed_at DESC);

CREATE OR REPLACE FUNCTION enforce_carpool_join_confirmation_actor()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  application_buyer uuid;
  application_owner uuid;
  application_status text;
  application_deadline timestamptz;
BEGIN
  SELECT buyer_user_id, owner_user_id, status, join_confirmation_deadline
  INTO application_buyer, application_owner, application_status, application_deadline
  FROM carpool_applications
  WHERE id = NEW.carpool_application_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool application % not found', NEW.carpool_application_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.actor_role = 'buyer' AND NEW.actor_user_id <> application_buyer THEN
    RAISE EXCEPTION 'buyer join confirmation actor must match application buyer'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.actor_role = 'owner' AND NEW.actor_user_id <> application_owner THEN
    RAISE EXCEPTION 'owner join confirmation actor must match application owner'
      USING ERRCODE = '23514';
  END IF;

  IF application_status NOT IN ('accepted_reserved', 'joined') THEN
    RAISE EXCEPTION 'join confirmation requires accepted_reserved or joined application'
      USING ERRCODE = '23514';
  END IF;

  IF application_deadline IS NULL OR NEW.confirmed_at > application_deadline THEN
    RAISE EXCEPTION 'join confirmation deadline expired'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_join_confirmation_actor
AFTER INSERT OR UPDATE OF carpool_application_id, actor_user_id, actor_role
ON carpool_join_confirmations
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_join_confirmation_actor();

CREATE TABLE carpool_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_listing_id uuid NOT NULL,
  carpool_application_id uuid NOT NULL UNIQUE,
  buyer_user_id uuid NOT NULL,
  owner_user_id uuid NOT NULL,
  product_plan_id uuid NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'left', 'removed')),
  seat_count integer NOT NULL DEFAULT 1 CHECK (seat_count = 1),
  price_monthly_cny_snapshot numeric(12,2) NOT NULL CHECK (price_monthly_cny_snapshot >= 0),
  policy_version_snapshot bigint NOT NULL,
  risk_notice_code_snapshot text REFERENCES risk_notices(code),
  joined_at timestamptz NOT NULL,
  ended_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (buyer_user_id <> owner_user_id),
  CHECK (
    (status = 'active' AND ended_at IS NULL)
    OR (status <> 'active' AND ended_at IS NOT NULL)
  ),
  FOREIGN KEY (
    carpool_application_id,
    carpool_listing_id,
    buyer_user_id,
    owner_user_id,
    product_plan_id
  )
  REFERENCES carpool_applications (
    id,
    carpool_listing_id,
    buyer_user_id,
    owner_user_id,
    product_plan_id
  )
);

CREATE UNIQUE INDEX ux_carpool_memberships_active_listing_buyer
ON carpool_memberships(carpool_listing_id, buyer_user_id)
WHERE status = 'active';

CREATE INDEX ix_carpool_memberships_buyer
ON carpool_memberships(buyer_user_id, status, updated_at DESC);

CREATE INDEX ix_carpool_memberships_owner_listing
ON carpool_memberships(owner_user_id, carpool_listing_id, status, updated_at DESC);

CREATE OR REPLACE FUNCTION enforce_carpool_membership_joined_application()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  application_status text;
  application_joined_at timestamptz;
BEGIN
  SELECT status, joined_at
  INTO application_status, application_joined_at
  FROM carpool_applications
  WHERE id = NEW.carpool_application_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool application % not found', NEW.carpool_application_id
      USING ERRCODE = '23503';
  END IF;

  IF application_status <> 'joined' OR application_joined_at IS NULL THEN
    RAISE EXCEPTION 'carpool membership requires joined application'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.joined_at <> application_joined_at THEN
    RAISE EXCEPTION 'carpool membership joined_at must match application joined_at'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_membership_joined_application
AFTER INSERT OR UPDATE OF carpool_application_id, joined_at
ON carpool_memberships
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_membership_joined_application();
