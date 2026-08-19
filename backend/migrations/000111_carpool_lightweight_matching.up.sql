-- Replace the pre-launch carpool reservation workflow with one-step matching.
-- Date: 2026-08-17
-- Executor: Codex

DELETE FROM transaction_review_revisions
WHERE transaction_review_id IN (
  SELECT id FROM transaction_reviews WHERE transaction_type = 'carpool_membership'
);

DELETE FROM transaction_reviews WHERE transaction_type = 'carpool_membership';
DELETE FROM carpool_reviews;

CREATE TEMP TABLE removed_carpool_contact_sessions ON COMMIT DROP AS
SELECT DISTINCT contact_session_id AS id
FROM carpool_applications
WHERE contact_session_id IS NOT NULL;

DELETE FROM carpool_memberships;
DELETE FROM carpool_applications;

DELETE FROM contact_session_items item
USING removed_carpool_contact_sessions removed
WHERE item.contact_session_id = removed.id;

DELETE FROM contact_access_logs access_log
USING removed_carpool_contact_sessions removed
WHERE access_log.contact_session_id = removed.id;

DELETE FROM contact_sessions session
USING removed_carpool_contact_sessions removed
WHERE session.id = removed.id;

DROP TABLE IF EXISTS carpool_completion_confirmations;
DROP TABLE IF EXISTS carpool_join_confirmations;
DROP FUNCTION IF EXISTS enforce_carpool_completion_confirmation_actor();
DROP FUNCTION IF EXISTS enforce_carpool_join_confirmation_actor();

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS carpool_listings_status_check;

ALTER TABLE carpool_listings
ADD COLUMN governance_status text NOT NULL DEFAULT 'clear',
ADD COLUMN recruitment_stop_reason text NOT NULL DEFAULT '',
ADD COLUMN conditions_version bigint NOT NULL DEFAULT 1,
ADD COLUMN offline_occupied_seats integer NOT NULL DEFAULT 0;

UPDATE carpool_listings
SET governance_status = CASE WHEN status = 'removed' THEN 'removed' ELSE 'clear' END,
    recruitment_stop_reason = CASE WHEN status = 'active' THEN '' ELSE 'migration' END,
    offline_occupied_seats = active_buyer_members,
    active_buyer_members = 0,
    status = CASE WHEN status = 'draft' THEN 'draft' ELSE 'stopped' END;

ALTER TABLE carpool_listings
ADD CONSTRAINT ck_carpool_listings_recruitment_status
CHECK (status IN ('draft', 'active', 'stopped')),
ADD CONSTRAINT ck_carpool_listings_governance_status
CHECK (governance_status IN ('clear', 'removed')),
ADD CONSTRAINT ck_carpool_listings_conditions_version CHECK (conditions_version > 0),
ADD CONSTRAINT ck_carpool_listings_offline_occupied_seats CHECK (offline_occupied_seats >= 0),
ADD CONSTRAINT ck_carpool_listings_total_occupied_seats
CHECK (offline_occupied_seats + active_buyer_members <= buyer_seat_capacity);

ALTER TABLE carpool_listings RENAME COLUMN daily_quota_amount TO daily_spend_limit_usd;
ALTER TABLE carpool_listings RENAME COLUMN weekly_quota_amount TO weekly_spend_limit_usd;
ALTER TABLE carpool_listings RENAME CONSTRAINT ck_carpool_listings_daily_quota_positive TO ck_carpool_listings_daily_spend_limit_usd_positive;
ALTER TABLE carpool_listings RENAME CONSTRAINT ck_carpool_listings_weekly_quota_nonnegative TO ck_carpool_listings_weekly_spend_limit_usd_nonnegative;
ALTER TABLE carpool_listings RENAME CONSTRAINT carpool_listings_weekly_quota_amount_not_null TO carpool_listings_weekly_spend_limit_usd_not_null;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_applications_status,
DROP CONSTRAINT IF EXISTS ck_carpool_application_reservation_deadline,
DROP CONSTRAINT IF EXISTS ck_carpool_application_join_confirmation_deadline,
DROP CONSTRAINT IF EXISTS ck_carpool_application_session_status;

ALTER TABLE carpool_applications
ADD COLUMN conditions_version_snapshot bigint NOT NULL,
ADD COLUMN conditions_snapshot jsonb NOT NULL,
ADD COLUMN accepted_conditions_version bigint NOT NULL,
ADD COLUMN conditions_accepted_at timestamptz NOT NULL;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_applications_status
CHECK (status IN ('pending_owner', 'joined', 'rejected', 'cancelled_by_buyer')),
ADD CONSTRAINT ck_carpool_application_conditions_versions
CHECK (conditions_version_snapshot > 0 AND accepted_conditions_version >= conditions_version_snapshot),
ADD CONSTRAINT ck_carpool_application_conditions_snapshot
CHECK (jsonb_typeof(conditions_snapshot) = 'object');

DROP INDEX IF EXISTS ux_carpool_applications_one_ongoing;
CREATE UNIQUE INDEX ux_carpool_applications_one_ongoing
ON carpool_applications(carpool_listing_id, buyer_user_id)
WHERE status = 'pending_owner';

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS ck_carpool_memberships_status,
DROP CONSTRAINT IF EXISTS ck_carpool_membership_end_state;

ALTER TABLE carpool_memberships
ADD COLUMN conditions_version_snapshot bigint NOT NULL,
ADD COLUMN conditions_snapshot jsonb NOT NULL;

ALTER TABLE carpool_memberships
ADD CONSTRAINT ck_carpool_memberships_status CHECK (status IN ('active', 'left', 'removed')),
ADD CONSTRAINT ck_carpool_membership_end_state CHECK (
  (status = 'active' AND ended_at IS NULL AND ended_reason = '' AND ended_by_user_id IS NULL)
  OR (status IN ('left', 'removed') AND ended_at IS NOT NULL AND ended_reason <> '')
),
ADD CONSTRAINT ck_carpool_membership_conditions_version CHECK (conditions_version_snapshot > 0),
ADD CONSTRAINT ck_carpool_membership_conditions_snapshot CHECK (jsonb_typeof(conditions_snapshot) = 'object');

CREATE TABLE carpool_listing_condition_versions (
  carpool_listing_id uuid NOT NULL REFERENCES carpool_listings(id) ON DELETE CASCADE,
  conditions_version bigint NOT NULL CHECK (conditions_version > 0),
  conditions_snapshot jsonb NOT NULL CHECK (jsonb_typeof(conditions_snapshot) = 'object'),
  changed_by_user_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (carpool_listing_id, conditions_version)
);

CREATE TABLE carpool_application_condition_acceptances (
  carpool_application_id uuid NOT NULL REFERENCES carpool_applications(id) ON DELETE CASCADE,
  conditions_version bigint NOT NULL CHECK (conditions_version > 0),
  conditions_snapshot jsonb NOT NULL CHECK (jsonb_typeof(conditions_snapshot) = 'object'),
  accepted_by_user_id uuid NOT NULL REFERENCES users(id),
  accepted_at timestamptz NOT NULL,
  request_id text NOT NULL,
  PRIMARY KEY (carpool_application_id, conditions_version)
);

ALTER TABLE contact_sessions ALTER COLUMN ends_at DROP NOT NULL;

DROP TRIGGER IF EXISTS trg_carpool_applications_reputation_dirty ON carpool_applications;
DROP TRIGGER IF EXISTS trg_carpool_memberships_reputation_dirty ON carpool_memberships;
DROP INDEX IF EXISTS ux_transaction_reviews_carpool_reviewer;

CREATE OR REPLACE FUNCTION enforce_transaction_review_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  transaction_buyer uuid;
  transaction_seller uuid;
  transaction_commercial_outcome text;
  transaction_terminal_at timestamptz;
BEGIN
  IF NEW.transaction_type <> 'api_order' OR NEW.api_order_id IS NULL OR NEW.carpool_membership_id IS NOT NULL THEN
    RAISE EXCEPTION 'carpool matching does not produce reviews' USING ERRCODE = '23514';
  END IF;
  IF TG_OP = 'UPDATE' AND OLD.frozen_at IS NOT NULL THEN RETURN NEW; END IF;

  SELECT buyer_user_id, seller_user_id, commercial_outcome, commercial_outcome_updated_at
  INTO transaction_buyer, transaction_seller, transaction_commercial_outcome, transaction_terminal_at
  FROM api_orders WHERE id = NEW.api_order_id;
  IF NOT FOUND THEN RAISE EXCEPTION 'review transaction not found' USING ERRCODE = '23503'; END IF;
  IF EXISTS (SELECT 1 FROM dispute_cases dispute WHERE dispute.api_order_id = NEW.api_order_id AND dispute.active = true) THEN
    RAISE EXCEPTION 'active API-order dispute pauses review mutation and publication' USING ERRCODE = '23514';
  END IF;
  IF transaction_terminal_at IS NULL OR transaction_commercial_outcome NOT IN ('normal_fulfillment', 'full_refund', 'partial_refund', 'continued_fulfillment') THEN
    RAISE EXCEPTION 'API order commercial outcome is not reviewable' USING ERRCODE = '23514';
  END IF;
  IF NEW.reviewer_role = 'buyer' THEN
    IF NEW.reviewer_user_id <> transaction_buyer OR NEW.reviewee_user_id <> transaction_seller OR NEW.reviewee_role <> 'seller' THEN
      RAISE EXCEPTION 'buyer review participants do not match transaction' USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.reviewer_role = 'seller' THEN
    IF NEW.reviewer_user_id <> transaction_seller OR NEW.reviewee_user_id <> transaction_buyer OR NEW.reviewee_role <> 'buyer' THEN
      RAISE EXCEPTION 'seller review participants do not match transaction' USING ERRCODE = '23514';
    END IF;
  ELSE
    RAISE EXCEPTION 'unsupported reviewer role' USING ERRCODE = '23514';
  END IF;
  NEW.commercial_outcome := transaction_commercial_outcome;
  NEW.review_deadline_at := transaction_terminal_at + interval '14 days';
  IF EXISTS (
    SELECT 1 FROM reputation_transaction_exclusions exclusion
    WHERE exclusion.transaction_type = 'api_order' AND exclusion.transaction_id = NEW.api_order_id AND exclusion.restored_at IS NULL
  ) THEN
    RAISE EXCEPTION 'excluded transaction cannot produce a review' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
