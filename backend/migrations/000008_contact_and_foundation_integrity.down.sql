DROP INDEX IF EXISTS ix_official_price_records_plan_region_status;
DROP INDEX IF EXISTS ix_official_price_records_status_plan_valid;
DROP INDEX IF EXISTS ix_official_price_leads_status_created;

ALTER TABLE official_price_records
DROP CONSTRAINT IF EXISTS ck_official_price_records_per_seat_requires_seat;

ALTER TABLE official_price_records
DROP CONSTRAINT IF EXISTS ck_official_price_records_valid_window;

ALTER TABLE official_price_records
DROP CONSTRAINT IF EXISTS ck_official_price_records_positive_values;

ALTER TABLE official_price_leads
DROP CONSTRAINT IF EXISTS ck_official_price_leads_approved_fields;

ALTER TABLE official_price_leads
DROP CONSTRAINT IF EXISTS ck_official_price_leads_per_seat_requires_seat;

ALTER TABLE official_price_leads
DROP CONSTRAINT IF EXISTS ck_official_price_leads_positive_amount;

ALTER TABLE idempotency_keys
DROP CONSTRAINT IF EXISTS ck_idempotency_completed_response;

ALTER TABLE idempotency_keys
DROP CONSTRAINT IF EXISTS ck_idempotency_processing_empty_response;

ALTER TABLE idempotency_keys
DROP CONSTRAINT IF EXISTS ck_idempotency_key_lengths;

DROP TRIGGER IF EXISTS trg_contact_session_item_participant ON contact_session_items;
DROP FUNCTION IF EXISTS enforce_contact_session_item_participant();

DROP INDEX IF EXISTS ix_contact_sessions_seller_ends;
DROP INDEX IF EXISTS ix_contact_sessions_buyer_ends;
DROP INDEX IF EXISTS ix_contact_access_logs_session_accessed;
DROP INDEX IF EXISTS ix_contact_session_items_session;
DROP INDEX IF EXISTS ix_contact_method_versions_method_created;
DROP INDEX IF EXISTS ux_contact_methods_one_default;

ALTER TABLE contact_sessions
DROP CONSTRAINT IF EXISTS ck_contact_sessions_time_window;

ALTER TABLE contact_method_versions
DROP CONSTRAINT IF EXISTS ck_contact_method_versions_nonce_12;

ALTER TABLE contact_methods
DROP CONSTRAINT IF EXISTS fk_contact_methods_current_version;

ALTER TABLE contact_methods
ADD CONSTRAINT fk_contact_methods_current_version
FOREIGN KEY(current_version_id, user_id)
REFERENCES contact_method_versions(id, owner_user_id);

ALTER TABLE contact_method_versions
DROP CONSTRAINT IF EXISTS uq_contact_method_version_identity;
