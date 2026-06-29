-- Subscription carpool listing and application contract.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE carpool_listings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  product_plan_id uuid NOT NULL REFERENCES product_plans(id),
  title text NOT NULL,
  summary text NOT NULL,
  access_arrangement text NOT NULL,
  source_url text,
  price_monthly_cny numeric(12,2) NOT NULL CHECK (price_monthly_cny >= 0),
  total_seats integer NOT NULL CHECK (total_seats > 0),
  current_active_members integer NOT NULL DEFAULT 0 CHECK (current_active_members >= 0),
  status text NOT NULL CHECK (status IN ('draft', 'pending_review', 'changes_requested', 'active', 'paused', 'rejected', 'removed')),
  reviewed_by_admin_id uuid REFERENCES users(id),
  reviewed_at timestamptz,
  review_reason text,
  policy_version bigint NOT NULL,
  risk_notice_code text REFERENCES risk_notices(code),
  risk_ack_required boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (current_active_members <= total_seats)
);

CREATE INDEX ix_carpool_listings_public
ON carpool_listings(status, product_plan_id, updated_at DESC);

CREATE INDEX ix_carpool_listings_owner
ON carpool_listings(owner_user_id, updated_at DESC);

CREATE TABLE carpool_listing_policy_acknowledgements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_listing_id uuid NOT NULL REFERENCES carpool_listings(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  risk_notice_code text NOT NULL REFERENCES risk_notices(code),
  policy_version bigint NOT NULL,
  acknowledged_at timestamptz NOT NULL,
  UNIQUE(carpool_listing_id, user_id, risk_notice_code, policy_version)
);

CREATE TABLE carpool_applications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_listing_id uuid NOT NULL REFERENCES carpool_listings(id),
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  product_plan_id uuid NOT NULL REFERENCES product_plans(id),
  buyer_contact_method_id uuid NOT NULL REFERENCES contact_methods(id),
  status text NOT NULL CHECK (status IN ('pending_owner', 'accepted_reserved', 'rejected', 'cancelled_by_buyer', 'expired')),
  seat_count integer NOT NULL DEFAULT 1 CHECK (seat_count = 1),
  listing_title_snapshot text NOT NULL,
  price_monthly_cny_snapshot numeric(12,2) NOT NULL CHECK (price_monthly_cny_snapshot >= 0),
  policy_version_snapshot bigint NOT NULL,
  risk_notice_code_snapshot text REFERENCES risk_notices(code),
  contact_session_id uuid REFERENCES contact_sessions(id),
  decision_reason text,
  decided_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (buyer_user_id <> owner_user_id)
);

CREATE UNIQUE INDEX ux_carpool_applications_one_ongoing
ON carpool_applications(carpool_listing_id, buyer_user_id)
WHERE status IN ('pending_owner', 'accepted_reserved');

CREATE INDEX ix_carpool_applications_buyer
ON carpool_applications(buyer_user_id, updated_at DESC);

CREATE INDEX ix_carpool_applications_owner
ON carpool_applications(owner_user_id, status, updated_at DESC);

CREATE INDEX ix_carpool_applications_listing_status
ON carpool_applications(carpool_listing_id, status);

CREATE TABLE carpool_application_policy_acknowledgements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_application_id uuid NOT NULL REFERENCES carpool_applications(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  risk_notice_code text NOT NULL REFERENCES risk_notices(code),
  policy_version bigint NOT NULL,
  acknowledged_at timestamptz NOT NULL,
  UNIQUE(carpool_application_id, user_id, risk_notice_code, policy_version)
);
