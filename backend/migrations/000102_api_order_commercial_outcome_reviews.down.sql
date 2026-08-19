-- Remove commercial outcomes only when no post-migration terminal facts would be lost.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM api_orders
    WHERE commercial_outcome IN ('full_refund', 'partial_refund', 'continued_fulfillment')
  ) THEN
    RAISE EXCEPTION 'cannot roll back commercial outcomes after settlement outcomes exist';
  END IF;
END $$;

DROP TRIGGER IF EXISTS trg_transaction_review_source ON transaction_reviews;
DROP TRIGGER IF EXISTS trg_transaction_review_freeze ON transaction_reviews;

ALTER TABLE transaction_reviews
DROP CONSTRAINT IF EXISTS ck_transaction_reviews_commercial_outcome,
DROP COLUMN IF EXISTS commercial_outcome;

CREATE OR REPLACE FUNCTION enforce_transaction_review_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  transaction_buyer uuid;
  transaction_seller uuid;
  transaction_status text;
  transaction_completed_at timestamptz;
  target_transaction_id uuid;
BEGIN
  IF NEW.transaction_type = 'carpool_membership' THEN
    target_transaction_id := NEW.carpool_membership_id;
    SELECT buyer_user_id, owner_user_id, status, COALESCE(ended_at, updated_at)
    INTO transaction_buyer, transaction_seller, transaction_status, transaction_completed_at
    FROM carpool_memberships WHERE id = NEW.carpool_membership_id;
  ELSE
    target_transaction_id := NEW.api_order_id;
    SELECT buyer_user_id, seller_user_id, status, completed_at
    INTO transaction_buyer, transaction_seller, transaction_status, transaction_completed_at
    FROM api_orders WHERE id = NEW.api_order_id;
  END IF;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'review transaction not found' USING ERRCODE = '23503';
  END IF;
  IF transaction_status <> 'completed' OR transaction_completed_at IS NULL THEN
    RAISE EXCEPTION 'review requires a completed transaction' USING ERRCODE = '23514';
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
  NEW.review_deadline_at := transaction_completed_at + interval '14 days';
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

CREATE TRIGGER trg_transaction_review_source
BEFORE INSERT OR UPDATE OF transaction_type, carpool_membership_id, api_order_id,
  reviewer_user_id, reviewee_user_id, reviewer_role, reviewee_role
ON transaction_reviews
FOR EACH ROW EXECUTE FUNCTION enforce_transaction_review_source();

CREATE OR REPLACE FUNCTION enforce_transaction_review_freeze()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status = 'removed' THEN
    RAISE EXCEPTION 'removed review is immutable' USING ERRCODE = '55000';
  END IF;
  IF OLD.frozen_at IS NOT NULL THEN
    IF NEW.status <> 'removed' THEN
      RAISE EXCEPTION 'published review is immutable' USING ERRCODE = '55000';
    END IF;
    IF NEW.transaction_type IS DISTINCT FROM OLD.transaction_type
       OR NEW.carpool_membership_id IS DISTINCT FROM OLD.carpool_membership_id
       OR NEW.api_order_id IS DISTINCT FROM OLD.api_order_id
       OR NEW.reviewer_user_id IS DISTINCT FROM OLD.reviewer_user_id
       OR NEW.reviewee_user_id IS DISTINCT FROM OLD.reviewee_user_id
       OR NEW.reviewer_role IS DISTINCT FROM OLD.reviewer_role
       OR NEW.reviewee_role IS DISTINCT FROM OLD.reviewee_role
       OR NEW.rating IS DISTINCT FROM OLD.rating
       OR NEW.tags IS DISTINCT FROM OLD.tags
       OR NEW.note IS DISTINCT FROM OLD.note
       OR NEW.review_deadline_at IS DISTINCT FROM OLD.review_deadline_at
       OR NEW.visible_at IS DISTINCT FROM OLD.visible_at
       OR NEW.frozen_at IS DISTINCT FROM OLD.frozen_at
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
      RAISE EXCEPTION 'review removal cannot rewrite frozen content' USING ERRCODE = '55000';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_transaction_review_freeze
BEFORE UPDATE ON transaction_reviews
FOR EACH ROW EXECUTE FUNCTION enforce_transaction_review_freeze();

ALTER TABLE api_orders
DROP CONSTRAINT IF EXISTS ck_api_orders_commercial_outcome_time,
DROP CONSTRAINT IF EXISTS ck_api_orders_commercial_outcome,
DROP COLUMN IF EXISTS commercial_outcome_updated_at,
DROP COLUMN IF EXISTS commercial_outcome;
