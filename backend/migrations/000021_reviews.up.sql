-- Carpool review center real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE carpool_reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type text NOT NULL CHECK (source_type = 'carpool_membership'),
  source_id uuid NOT NULL REFERENCES carpool_memberships(id) ON DELETE CASCADE,
  reviewer_user_id uuid NOT NULL REFERENCES users(id),
  reviewee_user_id uuid NOT NULL REFERENCES users(id),
  reviewer_role text NOT NULL CHECK (reviewer_role = 'buyer'),
  reviewee_role text NOT NULL CHECK (reviewee_role = 'owner'),
  rating integer NOT NULL CHECK (rating BETWEEN 1 AND 5),
  tags text[] NOT NULL DEFAULT '{}',
  note text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (source_type, source_id, reviewer_user_id),
  CHECK (reviewer_user_id <> reviewee_user_id)
);

CREATE INDEX ix_carpool_reviews_reviewer_updated
ON carpool_reviews(reviewer_user_id, updated_at DESC);

CREATE INDEX ix_carpool_reviews_reviewee_updated
ON carpool_reviews(reviewee_user_id, updated_at DESC);

CREATE OR REPLACE FUNCTION enforce_carpool_review_membership()
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
  WHERE id = NEW.source_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'carpool membership % not found', NEW.source_id
      USING ERRCODE = '23503';
  END IF;

  IF NEW.source_type <> 'carpool_membership' THEN
    RAISE EXCEPTION 'review source_type must be carpool_membership'
      USING ERRCODE = '23514';
  END IF;

  IF membership_status <> 'completed' THEN
    RAISE EXCEPTION 'review requires completed carpool membership'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.reviewer_role <> 'buyer' OR NEW.reviewer_user_id <> membership_buyer THEN
    RAISE EXCEPTION 'reviewer must be membership buyer'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.reviewee_role <> 'owner' OR NEW.reviewee_user_id <> membership_owner THEN
    RAISE EXCEPTION 'reviewee must be membership owner'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER trg_carpool_review_membership
AFTER INSERT OR UPDATE OF source_type, source_id, reviewer_user_id, reviewee_user_id, reviewer_role, reviewee_role
ON carpool_reviews
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW
EXECUTE FUNCTION enforce_carpool_review_membership();
