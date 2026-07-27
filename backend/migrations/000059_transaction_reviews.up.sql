-- 统一拼车与 API 订单双向评价，保留旧评价并增加双盲冻结审计。
-- 日期：2026-07-24
-- 执行者：Codex

CREATE TABLE transaction_reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_type text NOT NULL CHECK (transaction_type IN ('carpool_membership', 'api_order')),
  carpool_membership_id uuid REFERENCES carpool_memberships(id),
  api_order_id uuid REFERENCES api_orders(id),
  reviewer_user_id uuid NOT NULL REFERENCES users(id),
  reviewee_user_id uuid NOT NULL REFERENCES users(id),
  reviewer_role text NOT NULL CHECK (reviewer_role IN ('buyer', 'seller')),
  reviewee_role text NOT NULL CHECK (reviewee_role IN ('buyer', 'seller')),
  rating integer NOT NULL CHECK (rating BETWEEN 1 AND 5),
  tags text[] NOT NULL DEFAULT '{}',
  note text NOT NULL,
  status text NOT NULL DEFAULT 'sealed' CHECK (status IN ('sealed', 'published', 'removed')),
  review_deadline_at timestamptz NOT NULL,
  visible_at timestamptz,
  frozen_at timestamptz,
  removed_at timestamptz,
  removed_by_admin_id uuid REFERENCES users(id),
  removal_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CHECK (
    (
      transaction_type = 'carpool_membership'
      AND carpool_membership_id IS NOT NULL
      AND api_order_id IS NULL
    )
    OR (
      transaction_type = 'api_order'
      AND carpool_membership_id IS NULL
      AND api_order_id IS NOT NULL
    )
  ),
  CHECK (reviewer_user_id <> reviewee_user_id),
  CHECK (reviewer_role <> reviewee_role),
  CHECK (
    (
      status = 'sealed'
      AND visible_at IS NULL
      AND frozen_at IS NULL
      AND removed_at IS NULL
      AND removed_by_admin_id IS NULL
      AND removal_reason IS NULL
    )
    OR (
      status = 'published'
      AND visible_at IS NOT NULL
      AND frozen_at IS NOT NULL
      AND removed_at IS NULL
      AND removed_by_admin_id IS NULL
      AND removal_reason IS NULL
    )
    OR (
      status = 'removed'
      AND visible_at IS NOT NULL
      AND frozen_at IS NOT NULL
      AND removed_at IS NOT NULL
      AND removed_by_admin_id IS NOT NULL
      AND trim(removal_reason) <> ''
    )
  ),
  CHECK (frozen_at IS NULL OR visible_at IS NOT NULL),
  CHECK (frozen_at IS NULL OR frozen_at >= visible_at)
);

CREATE UNIQUE INDEX ux_transaction_reviews_carpool_reviewer
ON transaction_reviews(carpool_membership_id, reviewer_user_id)
WHERE transaction_type = 'carpool_membership';

CREATE UNIQUE INDEX ux_transaction_reviews_api_order_reviewer
ON transaction_reviews(api_order_id, reviewer_user_id)
WHERE transaction_type = 'api_order';

CREATE INDEX ix_transaction_reviews_reviewer_updated
ON transaction_reviews(reviewer_user_id, updated_at DESC, id DESC);

CREATE INDEX ix_transaction_reviews_reviewee_visible
ON transaction_reviews(reviewee_user_id, visible_at DESC, id DESC)
WHERE status = 'published';

CREATE INDEX ix_transaction_reviews_sealed_deadline
ON transaction_reviews(review_deadline_at, id)
WHERE status = 'sealed';

CREATE TABLE transaction_review_revisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_review_id uuid NOT NULL REFERENCES transaction_reviews(id),
  revision_number bigint NOT NULL CHECK (revision_number > 0),
  action text NOT NULL CHECK (action IN ('created', 'edited', 'published', 'removed', 'migrated')),
  actor_user_id uuid REFERENCES users(id),
  before_snapshot jsonb,
  after_snapshot jsonb,
  reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (transaction_review_id, revision_number),
  CHECK (
    action <> 'removed'
    OR (actor_user_id IS NOT NULL AND reason IS NOT NULL AND trim(reason) <> '')
  )
);

CREATE INDEX ix_transaction_review_revisions_review
ON transaction_review_revisions(transaction_review_id, revision_number DESC);

INSERT INTO transaction_reviews (
  id,
  transaction_type,
  carpool_membership_id,
  reviewer_user_id,
  reviewee_user_id,
  reviewer_role,
  reviewee_role,
  rating,
  tags,
  note,
  status,
  review_deadline_at,
  visible_at,
  frozen_at,
  created_at,
  updated_at,
  version
)
SELECT
  legacy.id,
  'carpool_membership',
  legacy.source_id,
  legacy.reviewer_user_id,
  legacy.reviewee_user_id,
  'buyer',
  'seller',
  legacy.rating,
  legacy.tags,
  legacy.note,
  'published',
  COALESCE(membership.ended_at, membership.updated_at) + interval '14 days',
  legacy.created_at,
  GREATEST(legacy.created_at, legacy.updated_at),
  legacy.created_at,
  legacy.updated_at,
  1
FROM carpool_reviews legacy
JOIN carpool_memberships membership ON membership.id = legacy.source_id
ON CONFLICT DO NOTHING;

INSERT INTO transaction_review_revisions (
  transaction_review_id,
  revision_number,
  action,
  actor_user_id,
  before_snapshot,
  after_snapshot,
  created_at
)
SELECT
  review.id,
  1,
  'migrated',
  review.reviewer_user_id,
  NULL,
  jsonb_build_object(
    'rating', review.rating,
    'tags', to_jsonb(review.tags),
    'note', review.note,
    'status', review.status,
    'visibleAt', review.visible_at,
    'frozenAt', review.frozen_at
  ),
  review.updated_at
FROM transaction_reviews review;

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
    FROM carpool_memberships
    WHERE id = NEW.carpool_membership_id;
  ELSE
    target_transaction_id := NEW.api_order_id;
    SELECT buyer_user_id, seller_user_id, status, completed_at
    INTO transaction_buyer, transaction_seller, transaction_status, transaction_completed_at
    FROM api_orders
    WHERE id = NEW.api_order_id;
  END IF;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'review transaction not found'
      USING ERRCODE = '23503';
  END IF;

  IF transaction_status <> 'completed' OR transaction_completed_at IS NULL THEN
    RAISE EXCEPTION 'review requires a completed transaction'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.reviewer_role = 'buyer' THEN
    IF NEW.reviewer_user_id <> transaction_buyer
       OR NEW.reviewee_user_id <> transaction_seller
       OR NEW.reviewee_role <> 'seller' THEN
      RAISE EXCEPTION 'buyer review participants do not match transaction'
        USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.reviewer_role = 'seller' THEN
    IF NEW.reviewer_user_id <> transaction_seller
       OR NEW.reviewee_user_id <> transaction_buyer
       OR NEW.reviewee_role <> 'buyer' THEN
      RAISE EXCEPTION 'seller review participants do not match transaction'
        USING ERRCODE = '23514';
    END IF;
  ELSE
    RAISE EXCEPTION 'unsupported reviewer role'
      USING ERRCODE = '23514';
  END IF;

  NEW.review_deadline_at := transaction_completed_at + interval '14 days';

  IF EXISTS (
    SELECT 1
    FROM reputation_transaction_exclusions exclusion
    WHERE exclusion.transaction_type = NEW.transaction_type
      AND exclusion.transaction_id = target_transaction_id
      AND exclusion.restored_at IS NULL
  ) THEN
    RAISE EXCEPTION 'excluded transaction cannot produce a review'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_transaction_review_source
BEFORE INSERT OR UPDATE OF
  transaction_type,
  carpool_membership_id,
  api_order_id,
  reviewer_user_id,
  reviewee_user_id,
  reviewer_role,
  reviewee_role
ON transaction_reviews
FOR EACH ROW
EXECUTE FUNCTION enforce_transaction_review_source();

CREATE OR REPLACE FUNCTION enforce_transaction_review_freeze()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status = 'removed' THEN
    RAISE EXCEPTION 'removed review is immutable'
      USING ERRCODE = '55000';
  END IF;

  IF OLD.frozen_at IS NOT NULL THEN
    IF NEW.status <> 'removed' THEN
      RAISE EXCEPTION 'published review is immutable'
        USING ERRCODE = '55000';
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
      RAISE EXCEPTION 'review removal cannot rewrite frozen content'
        USING ERRCODE = '55000';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_transaction_review_freeze
BEFORE UPDATE ON transaction_reviews
FOR EACH ROW
EXECUTE FUNCTION enforce_transaction_review_freeze();

CREATE OR REPLACE FUNCTION reject_transaction_review_revision_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'transaction review revisions are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_transaction_review_revisions_append_only
BEFORE UPDATE OR DELETE ON transaction_review_revisions
FOR EACH ROW
EXECUTE FUNCTION reject_transaction_review_revision_mutation();
