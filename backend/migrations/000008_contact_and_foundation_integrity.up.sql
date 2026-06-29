-- Contact, idempotency, and official price integrity hardening.
-- 日期：2026-06-21
-- 执行者：Codex

ALTER TABLE contact_methods
DROP CONSTRAINT IF EXISTS fk_contact_methods_current_version;

ALTER TABLE contact_method_versions
ADD CONSTRAINT uq_contact_method_version_identity
UNIQUE (id, contact_method_id, owner_user_id);

ALTER TABLE contact_methods
ADD CONSTRAINT fk_contact_methods_current_version
FOREIGN KEY (current_version_id, id, user_id)
REFERENCES contact_method_versions (id, contact_method_id, owner_user_id);

ALTER TABLE contact_method_versions
ADD CONSTRAINT ck_contact_method_versions_nonce_12
CHECK (octet_length(value_nonce) = 12);

ALTER TABLE contact_sessions
ADD CONSTRAINT ck_contact_sessions_time_window
CHECK (ends_at > opens_at);

CREATE UNIQUE INDEX ux_contact_methods_one_default
ON contact_methods(user_id)
WHERE is_default = true AND enabled = true;

CREATE INDEX ix_contact_method_versions_method_created
ON contact_method_versions(contact_method_id, created_at DESC);

CREATE INDEX ix_contact_session_items_session
ON contact_session_items(contact_session_id);

CREATE INDEX ix_contact_access_logs_session_accessed
ON contact_access_logs(contact_session_id, accessed_at DESC);

CREATE INDEX ix_contact_sessions_buyer_ends
ON contact_sessions(buyer_user_id, ends_at);

CREATE INDEX ix_contact_sessions_seller_ends
ON contact_sessions(seller_user_id, ends_at);

CREATE OR REPLACE FUNCTION enforce_contact_session_item_participant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  session_buyer uuid;
  session_seller uuid;
BEGIN
  SELECT buyer_user_id, seller_user_id
  INTO session_buyer, session_seller
  FROM contact_sessions
  WHERE id = NEW.contact_session_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'contact session % not found', NEW.contact_session_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.side = 'buyer' AND NEW.subject_user_id <> session_buyer THEN
    RAISE EXCEPTION 'buyer contact session item subject must match session buyer'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.side = 'seller' AND NEW.subject_user_id <> session_seller THEN
    RAISE EXCEPTION 'seller contact session item subject must match session seller'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.side NOT IN ('buyer', 'seller') THEN
    RAISE EXCEPTION 'invalid contact session item side'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_contact_session_item_participant
BEFORE INSERT OR UPDATE OF contact_session_id, subject_user_id, side
ON contact_session_items
FOR EACH ROW
EXECUTE FUNCTION enforce_contact_session_item_participant();

ALTER TABLE idempotency_keys
ADD CONSTRAINT ck_idempotency_key_lengths
CHECK (
  length(idempotency_key) BETWEEN 1 AND 128
  AND length(route_key) BETWEEN 1 AND 256
  AND length(request_hash) BETWEEN 1 AND 128
);

ALTER TABLE idempotency_keys
ADD CONSTRAINT ck_idempotency_processing_empty_response
CHECK (
  status <> 'processing'
  OR (
    completed_at IS NULL
    AND response_status IS NULL
    AND response_content_type IS NULL
    AND response_body_json IS NULL
    AND resource_type IS NULL
    AND resource_id IS NULL
  )
);

ALTER TABLE idempotency_keys
ADD CONSTRAINT ck_idempotency_completed_response
CHECK (
  status <> 'completed'
  OR (
    completed_at IS NOT NULL
    AND response_status IS NOT NULL
    AND response_content_type IS NOT NULL
    AND resource_type IS NOT NULL
    AND resource_id IS NOT NULL
    AND (
      response_body_cache_allowed = true
      OR response_body_json IS NULL
    )
  )
);

ALTER TABLE official_price_leads
ADD CONSTRAINT ck_official_price_leads_positive_amount
CHECK (original_amount > 0);

ALTER TABLE official_price_leads
ADD CONSTRAINT ck_official_price_leads_per_seat_requires_seat
CHECK (price_unit <> 'per_seat' OR seat_count IS NOT NULL);

ALTER TABLE official_price_leads
ADD CONSTRAINT ck_official_price_leads_approved_fields
CHECK (
  status <> 'approved'
  OR (
    product_plan_id IS NOT NULL
    AND reviewed_by_admin_id IS NOT NULL
    AND reviewed_at IS NOT NULL
    AND review_reason IS NOT NULL
    AND normalized_monthly_cny IS NOT NULL
    AND normalized_monthly_cny > 0
    AND fx_rate IS NOT NULL
    AND fx_rate > 0
    AND fx_source IS NOT NULL
    AND fx_observed_at IS NOT NULL
    AND conversion_mode IS NOT NULL
    AND rounding_rule IS NOT NULL
    AND offer_key IS NOT NULL
  )
);

ALTER TABLE official_price_records
ADD CONSTRAINT ck_official_price_records_positive_values
CHECK (
  original_amount > 0
  AND fx_rate > 0
  AND normalized_monthly_cny > 0
);

ALTER TABLE official_price_records
ADD CONSTRAINT ck_official_price_records_valid_window
CHECK (valid_to IS NULL OR valid_to > valid_from);

ALTER TABLE official_price_records
ADD CONSTRAINT ck_official_price_records_per_seat_requires_seat
CHECK (price_unit <> 'per_seat' OR seat_count IS NOT NULL);

CREATE INDEX ix_official_price_leads_status_created
ON official_price_leads(status, created_at DESC);

CREATE INDEX ix_official_price_records_status_plan_valid
ON official_price_records(status, product_plan_id, valid_from DESC);

CREATE INDEX ix_official_price_records_plan_region_status
ON official_price_records(product_plan_id, region_code, status);
