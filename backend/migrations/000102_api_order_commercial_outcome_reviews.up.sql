-- Separate API-order commercial outcomes from fulfillment status and pause reviews during active disputes.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE api_orders
ADD COLUMN commercial_outcome text NOT NULL DEFAULT 'pending',
ADD COLUMN commercial_outcome_updated_at timestamptz;

UPDATE api_orders
SET commercial_outcome = CASE
      WHEN status = 'completed' AND dispute_case_id IS NOT NULL THEN 'pending'
      WHEN status = 'completed' AND latest_dispute_case_id IS NULL THEN 'normal_fulfillment'
      WHEN status = 'completed' THEN 'closed_unverified'
      WHEN status = 'cancelled' THEN 'cancelled_unpaid'
      ELSE 'pending'
    END,
    commercial_outcome_updated_at = CASE
      WHEN status = 'completed' AND dispute_case_id IS NULL THEN completed_at
      WHEN status = 'cancelled' THEN cancelled_at
      ELSE NULL
    END;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_commercial_outcome CHECK (commercial_outcome IN (
  'pending', 'cancelled_unpaid', 'normal_fulfillment', 'full_refund',
  'partial_refund', 'continued_fulfillment', 'closed_unverified'
)),
ADD CONSTRAINT ck_api_orders_commercial_outcome_time CHECK (
  (commercial_outcome = 'pending' AND commercial_outcome_updated_at IS NULL)
  OR (commercial_outcome <> 'pending' AND commercial_outcome_updated_at IS NOT NULL)
);

ALTER TABLE transaction_reviews
ADD COLUMN commercial_outcome text NOT NULL DEFAULT '';

DROP TRIGGER IF EXISTS trg_transaction_review_freeze ON transaction_reviews;

UPDATE transaction_reviews
SET commercial_outcome = 'legacy_fulfillment'
WHERE transaction_type = 'api_order';

ALTER TABLE transaction_reviews
ADD CONSTRAINT ck_transaction_reviews_commercial_outcome CHECK (
  (transaction_type = 'carpool_membership' AND commercial_outcome = '')
  OR (
    transaction_type = 'api_order'
    AND commercial_outcome IN (
      'normal_fulfillment', 'full_refund', 'partial_refund',
      'continued_fulfillment', 'legacy_fulfillment'
    )
  )
);

UPDATE transaction_reviews review
SET review_deadline_at = order_row.commercial_outcome_updated_at + interval '14 days',
    commercial_outcome = order_row.commercial_outcome
FROM api_orders order_row
WHERE review.transaction_type = 'api_order'
  AND review.api_order_id = order_row.id
  AND review.status = 'sealed'
  AND review.frozen_at IS NULL
  AND order_row.commercial_outcome IN (
    'normal_fulfillment', 'full_refund', 'partial_refund', 'continued_fulfillment'
  );

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
       OR NEW.commercial_outcome IS DISTINCT FROM OLD.commercial_outcome
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
