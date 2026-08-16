-- Restore the pre-launch carpool schema only when no carpool data can be lost.
-- Date: 2026-08-17
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM carpool_listings)
     OR EXISTS (SELECT 1 FROM carpool_applications)
     OR EXISTS (SELECT 1 FROM carpool_memberships)
     OR EXISTS (SELECT 1 FROM carpool_listing_condition_versions)
     OR EXISTS (SELECT 1 FROM carpool_application_condition_acceptances)
     OR EXISTS (SELECT 1 FROM contact_sessions WHERE ends_at IS NULL) THEN
    RAISE EXCEPTION 'cannot roll back migration 111 while carpool matching data or open-ended contact sessions exist';
  END IF;
END $$;

DROP TABLE IF EXISTS carpool_application_condition_acceptances;
DROP TABLE IF EXISTS carpool_listing_condition_versions;

ALTER TABLE carpool_applications
DROP CONSTRAINT IF EXISTS ck_carpool_application_conditions_snapshot,
DROP CONSTRAINT IF EXISTS ck_carpool_application_conditions_versions,
DROP CONSTRAINT IF EXISTS ck_carpool_applications_status,
DROP COLUMN IF EXISTS conditions_accepted_at,
DROP COLUMN IF EXISTS accepted_conditions_version,
DROP COLUMN IF EXISTS conditions_snapshot,
DROP COLUMN IF EXISTS conditions_version_snapshot;

ALTER TABLE carpool_applications
ADD CONSTRAINT ck_carpool_applications_status
CHECK (status IN (
  'pending_owner', 'accepted_reserved', 'joined', 'rejected',
  'cancelled_by_buyer', 'cancelled_by_owner', 'expired'
)),
ADD CONSTRAINT ck_carpool_application_reservation_deadline
CHECK (
  (status = 'accepted_reserved' AND reservation_expires_at IS NOT NULL)
  OR status <> 'accepted_reserved'
),
ADD CONSTRAINT ck_carpool_application_join_confirmation_deadline
CHECK (
  (status = 'accepted_reserved' AND join_confirmation_deadline IS NOT NULL)
  OR status <> 'accepted_reserved'
),
ADD CONSTRAINT ck_carpool_application_session_status
CHECK (
  contact_session_id IS NULL
  OR status IN (
    'accepted_reserved', 'joined', 'expired',
    'cancelled_by_buyer', 'cancelled_by_owner'
  )
);

DROP INDEX IF EXISTS ux_carpool_applications_one_ongoing;
CREATE UNIQUE INDEX ux_carpool_applications_one_ongoing
ON carpool_applications(carpool_listing_id, buyer_user_id)
WHERE status IN ('pending_owner', 'accepted_reserved');

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS ck_carpool_membership_conditions_snapshot,
DROP CONSTRAINT IF EXISTS ck_carpool_membership_conditions_version,
DROP CONSTRAINT IF EXISTS ck_carpool_membership_end_state,
DROP CONSTRAINT IF EXISTS ck_carpool_memberships_status,
DROP COLUMN IF EXISTS conditions_snapshot,
DROP COLUMN IF EXISTS conditions_version_snapshot;

ALTER TABLE carpool_memberships
ADD CONSTRAINT ck_carpool_memberships_status
CHECK (status IN ('active', 'completed', 'left', 'removed')),
ADD CONSTRAINT ck_carpool_membership_end_state
CHECK (
  (status = 'active' AND ended_at IS NULL AND ended_reason = '' AND ended_by_user_id IS NULL)
  OR (status <> 'active' AND ended_at IS NOT NULL AND ended_reason <> '')
);

ALTER TABLE carpool_listings
DROP CONSTRAINT IF EXISTS ck_carpool_listings_total_occupied_seats,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_offline_occupied_seats,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_conditions_version,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_governance_status,
DROP CONSTRAINT IF EXISTS ck_carpool_listings_recruitment_status,
DROP COLUMN IF EXISTS offline_occupied_seats,
DROP COLUMN IF EXISTS conditions_version,
DROP COLUMN IF EXISTS recruitment_stop_reason,
DROP COLUMN IF EXISTS governance_status;

ALTER TABLE carpool_listings RENAME COLUMN daily_spend_limit_usd TO daily_quota_amount;
ALTER TABLE carpool_listings RENAME COLUMN weekly_spend_limit_usd TO weekly_quota_amount;
ALTER TABLE carpool_listings RENAME CONSTRAINT ck_carpool_listings_daily_spend_limit_usd_positive TO ck_carpool_listings_daily_quota_positive;
ALTER TABLE carpool_listings RENAME CONSTRAINT ck_carpool_listings_weekly_spend_limit_usd_nonnegative TO ck_carpool_listings_weekly_quota_nonnegative;
ALTER TABLE carpool_listings RENAME CONSTRAINT carpool_listings_weekly_spend_limit_usd_not_null TO carpool_listings_weekly_quota_amount_not_null;

ALTER TABLE carpool_listings
ADD CONSTRAINT carpool_listings_status_check
CHECK (status IN (
  'draft', 'pending_review', 'changes_requested', 'active',
  'paused', 'rejected', 'removed'
));

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

CREATE TABLE carpool_completion_confirmations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_membership_id uuid NOT NULL REFERENCES carpool_memberships(id) ON DELETE CASCADE,
  actor_user_id uuid NOT NULL REFERENCES users(id),
  actor_role text NOT NULL CHECK (actor_role IN ('buyer', 'owner')),
  confirmed_at timestamptz NOT NULL,
  request_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(carpool_membership_id, actor_role)
);

CREATE INDEX ix_carpool_completion_confirmations_actor
ON carpool_completion_confirmations(actor_user_id, confirmed_at DESC);

CREATE OR REPLACE FUNCTION enforce_carpool_completion_confirmation_actor()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  membership_buyer uuid;
  membership_owner uuid;
  membership_status text;
BEGIN
  SELECT buyer_user_id, owner_user_id, status
  INTO membership_buyer, membership_owner, membership_status
  FROM carpool_memberships
  WHERE id = NEW.carpool_membership_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool membership % not found', NEW.carpool_membership_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.actor_role = 'buyer' AND NEW.actor_user_id <> membership_buyer THEN
    RAISE EXCEPTION 'buyer completion confirmation actor must match membership buyer'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.actor_role = 'owner' AND NEW.actor_user_id <> membership_owner THEN
    RAISE EXCEPTION 'owner completion confirmation actor must match membership owner'
      USING ERRCODE = '23514';
  END IF;

  IF membership_status NOT IN ('active', 'completed') THEN
    RAISE EXCEPTION 'completion confirmation requires active or completed membership'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_completion_confirmation_actor
AFTER INSERT OR UPDATE OF carpool_membership_id, actor_user_id, actor_role
ON carpool_completion_confirmations
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_completion_confirmation_actor();

ALTER TABLE contact_sessions ALTER COLUMN ends_at SET NOT NULL;

CREATE UNIQUE INDEX ux_transaction_reviews_carpool_reviewer
ON transaction_reviews(carpool_membership_id, reviewer_user_id)
WHERE transaction_type = 'carpool_membership';

CREATE OR REPLACE FUNCTION enforce_transaction_review_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  transaction_buyer uuid;
  transaction_seller uuid;
  transaction_status text;
  transaction_terminal_at timestamptz;
  transaction_commercial_outcome text := '';
  target_transaction_id uuid;
BEGIN
  -- Published and removed reviews retain their frozen source snapshot.
  IF TG_OP = 'UPDATE' AND OLD.frozen_at IS NOT NULL THEN
    RETURN NEW;
  END IF;

  IF NEW.transaction_type = 'carpool_membership' THEN
    target_transaction_id := NEW.carpool_membership_id;
    SELECT buyer_user_id, owner_user_id, status, COALESCE(ended_at, updated_at)
    INTO transaction_buyer, transaction_seller, transaction_status, transaction_terminal_at
    FROM carpool_memberships
    WHERE id = NEW.carpool_membership_id;
  ELSE
    target_transaction_id := NEW.api_order_id;
    SELECT buyer_user_id, seller_user_id, status, commercial_outcome_updated_at, commercial_outcome
    INTO transaction_buyer, transaction_seller, transaction_status, transaction_terminal_at, transaction_commercial_outcome
    FROM api_orders
    WHERE id = NEW.api_order_id;
  END IF;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'review transaction not found' USING ERRCODE = '23503';
  END IF;

  IF NEW.transaction_type = 'carpool_membership' THEN
    IF transaction_status <> 'completed' OR transaction_terminal_at IS NULL THEN
      RAISE EXCEPTION 'review requires a completed transaction' USING ERRCODE = '23514';
    END IF;
    NEW.commercial_outcome := '';
  ELSE
    IF EXISTS (
      SELECT 1 FROM dispute_cases dispute
      WHERE dispute.api_order_id = NEW.api_order_id AND dispute.active = true
    ) THEN
      RAISE EXCEPTION 'active API-order dispute pauses review mutation and publication' USING ERRCODE = '23514';
    END IF;
    IF transaction_terminal_at IS NULL OR transaction_commercial_outcome NOT IN (
      'normal_fulfillment', 'full_refund', 'partial_refund', 'continued_fulfillment'
    ) THEN
      RAISE EXCEPTION 'API order commercial outcome is not reviewable' USING ERRCODE = '23514';
    END IF;
    NEW.commercial_outcome := transaction_commercial_outcome;
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

  NEW.review_deadline_at := transaction_terminal_at + interval '14 days';

  IF EXISTS (
    SELECT 1 FROM reputation_transaction_exclusions exclusion
    WHERE exclusion.transaction_type = NEW.transaction_type
      AND exclusion.transaction_id = target_transaction_id
      AND exclusion.restored_at IS NULL
  ) THEN
    RAISE EXCEPTION 'excluded transaction cannot produce a review' USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_transaction_review_source ON transaction_reviews;
CREATE TRIGGER trg_transaction_review_source
BEFORE INSERT OR UPDATE ON transaction_reviews
FOR EACH ROW EXECUTE FUNCTION enforce_transaction_review_source();

CREATE TRIGGER trg_carpool_applications_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON carpool_applications
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_participant_row();

CREATE TRIGGER trg_carpool_memberships_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON carpool_memberships
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_participant_row();
