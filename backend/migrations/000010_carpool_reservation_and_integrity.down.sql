DROP TRIGGER IF EXISTS trg_carpool_application_ack_integrity ON carpool_application_policy_acknowledgements;
DROP FUNCTION IF EXISTS enforce_carpool_application_ack_integrity();

DROP TRIGGER IF EXISTS trg_carpool_listing_ack_integrity ON carpool_listing_policy_acknowledgements;
DROP FUNCTION IF EXISTS enforce_carpool_listing_ack_integrity();

ALTER TABLE carpool_application_policy_acknowledgements
DROP CONSTRAINT IF EXISTS fk_carpool_application_ack_notice_version;

ALTER TABLE carpool_application_policy_acknowledgements
DROP COLUMN IF EXISTS risk_notice_version_id;

ALTER TABLE carpool_listing_policy_acknowledgements
DROP CONSTRAINT IF EXISTS fk_carpool_listing_ack_notice_version;

ALTER TABLE carpool_listing_policy_acknowledgements
DROP COLUMN IF EXISTS risk_notice_version_id;

DROP TRIGGER IF EXISTS trg_carpool_application_contact_session ON carpool_applications;
DROP FUNCTION IF EXISTS enforce_carpool_application_contact_session();

DROP INDEX IF EXISTS ix_carpool_applications_reservation_expiry;
DROP INDEX IF EXISTS ux_carpool_applications_contact_session;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_session_status;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_reservation_deadline;

ALTER TABLE carpool_applications
DROP COLUMN IF EXISTS reservation_expires_at;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS fk_carpool_application_buyer_contact;

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS fk_carpool_listing_owner_contact;

ALTER TABLE carpool_listings
DROP COLUMN IF EXISTS owner_contact_method_id;

ALTER TABLE carpool_listings
RENAME COLUMN active_buyer_members TO current_active_members;

ALTER TABLE carpool_listings
RENAME COLUMN buyer_seat_capacity TO total_seats;
