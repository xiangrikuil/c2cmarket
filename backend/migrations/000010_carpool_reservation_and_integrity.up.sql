-- Carpool reservation lifecycle and ownership integrity hardening.
-- 日期：2026-06-21
-- 执行者：Codex

ALTER TABLE carpool_listings
RENAME COLUMN total_seats TO buyer_seat_capacity;

ALTER TABLE carpool_listings
RENAME COLUMN current_active_members TO active_buyer_members;

ALTER TABLE carpool_listings
ADD COLUMN owner_contact_method_id uuid;

UPDATE carpool_listings listing
SET owner_contact_method_id = (
  SELECT method.id
  FROM contact_methods method
  WHERE method.user_id = listing.owner_user_id
    AND method.enabled = true
    AND method.current_version_id IS NOT NULL
  ORDER BY method.is_default DESC, method.created_at DESC
  LIMIT 1
)
WHERE owner_contact_method_id IS NULL;

ALTER TABLE carpool_listings
ALTER COLUMN owner_contact_method_id SET NOT NULL;

ALTER TABLE carpool_listings
ADD CONSTRAINT fk_carpool_listing_owner_contact
FOREIGN KEY (owner_contact_method_id, owner_user_id)
REFERENCES contact_methods(id, user_id);

ALTER TABLE carpool_applications
ADD CONSTRAINT fk_carpool_application_buyer_contact
FOREIGN KEY (buyer_contact_method_id, buyer_user_id)
REFERENCES contact_methods(id, user_id);

ALTER TABLE carpool_applications
ADD COLUMN reservation_expires_at timestamptz;

UPDATE carpool_applications application
SET reservation_expires_at = COALESCE(session.ends_at, application.decided_at + interval '30 minutes', application.updated_at + interval '30 minutes')
FROM contact_sessions session
WHERE application.contact_session_id = session.id
  AND application.status = 'accepted_reserved'
  AND application.reservation_expires_at IS NULL;

UPDATE carpool_applications
SET reservation_expires_at = COALESCE(decided_at + interval '30 minutes', updated_at + interval '30 minutes')
WHERE status = 'accepted_reserved'
  AND reservation_expires_at IS NULL;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_reservation_deadline
CHECK (
  (status = 'accepted_reserved' AND reservation_expires_at IS NOT NULL)
  OR status <> 'accepted_reserved'
);

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_application_session_status
CHECK (
  contact_session_id IS NULL
  OR status IN ('accepted_reserved', 'expired')
);

CREATE UNIQUE INDEX ux_carpool_applications_contact_session
ON carpool_applications(contact_session_id)
WHERE contact_session_id IS NOT NULL;

CREATE INDEX ix_carpool_applications_reservation_expiry
ON carpool_applications(carpool_listing_id, reservation_expires_at)
WHERE status = 'accepted_reserved';

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

CREATE CONSTRAINT TRIGGER trg_carpool_application_contact_session
AFTER INSERT OR UPDATE OF contact_session_id, buyer_user_id, owner_user_id, status
ON carpool_applications
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_application_contact_session();

ALTER TABLE carpool_listing_policy_acknowledgements
ADD COLUMN risk_notice_version_id uuid;

UPDATE carpool_listing_policy_acknowledgements acknowledgement
SET risk_notice_version_id = version.id
FROM risk_notices notice
JOIN risk_notice_versions version ON version.risk_notice_id = notice.id
WHERE notice.code = acknowledgement.risk_notice_code
  AND version.version = acknowledgement.policy_version;

ALTER TABLE carpool_listing_policy_acknowledgements
ALTER COLUMN risk_notice_version_id SET NOT NULL;

ALTER TABLE carpool_listing_policy_acknowledgements
ADD CONSTRAINT fk_carpool_listing_ack_notice_version
FOREIGN KEY (risk_notice_version_id)
REFERENCES risk_notice_versions(id);

ALTER TABLE carpool_application_policy_acknowledgements
ADD COLUMN risk_notice_version_id uuid;

UPDATE carpool_application_policy_acknowledgements acknowledgement
SET risk_notice_version_id = version.id
FROM risk_notices notice
JOIN risk_notice_versions version ON version.risk_notice_id = notice.id
WHERE notice.code = acknowledgement.risk_notice_code
  AND version.version = acknowledgement.policy_version;

ALTER TABLE carpool_application_policy_acknowledgements
ALTER COLUMN risk_notice_version_id SET NOT NULL;

ALTER TABLE carpool_application_policy_acknowledgements
ADD CONSTRAINT fk_carpool_application_ack_notice_version
FOREIGN KEY (risk_notice_version_id)
REFERENCES risk_notice_versions(id);

CREATE OR REPLACE FUNCTION enforce_carpool_listing_ack_integrity()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  listing_owner uuid;
  notice_code text;
  notice_version integer;
BEGIN
  SELECT owner_user_id
  INTO listing_owner
  FROM carpool_listings
  WHERE id = NEW.carpool_listing_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool listing % not found', NEW.carpool_listing_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.user_id <> listing_owner THEN
    RAISE EXCEPTION 'carpool listing acknowledgement user must match listing owner'
      USING ERRCODE = '23514';
  END IF;

  SELECT notice.code, version.version
  INTO notice_code, notice_version
  FROM risk_notice_versions version
  JOIN risk_notices notice ON notice.id = version.risk_notice_id
  WHERE version.id = NEW.risk_notice_version_id;

  IF NOT FOUND OR notice_code <> NEW.risk_notice_code OR notice_version <> NEW.policy_version THEN
    RAISE EXCEPTION 'carpool listing acknowledgement risk notice version mismatch'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_listing_ack_integrity
AFTER INSERT OR UPDATE OF carpool_listing_id, user_id, risk_notice_code, policy_version, risk_notice_version_id
ON carpool_listing_policy_acknowledgements
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_listing_ack_integrity();

CREATE OR REPLACE FUNCTION enforce_carpool_application_ack_integrity()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  application_buyer uuid;
  notice_code text;
  notice_version integer;
BEGIN
  SELECT buyer_user_id
  INTO application_buyer
  FROM carpool_applications
  WHERE id = NEW.carpool_application_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool application % not found', NEW.carpool_application_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.user_id <> application_buyer THEN
    RAISE EXCEPTION 'carpool application acknowledgement user must match application buyer'
      USING ERRCODE = '23514';
  END IF;

  SELECT notice.code, version.version
  INTO notice_code, notice_version
  FROM risk_notice_versions version
  JOIN risk_notices notice ON notice.id = version.risk_notice_id
  WHERE version.id = NEW.risk_notice_version_id;

  IF NOT FOUND OR notice_code <> NEW.risk_notice_code OR notice_version <> NEW.policy_version THEN
    RAISE EXCEPTION 'carpool application acknowledgement risk notice version mismatch'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_application_ack_integrity
AFTER INSERT OR UPDATE OF carpool_application_id, user_id, risk_notice_code, policy_version, risk_notice_version_id
ON carpool_application_policy_acknowledgements
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_application_ack_integrity();
