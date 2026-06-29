-- Official price lead and approved price record contracts.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE official_price_leads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  submitter_user_id uuid NOT NULL REFERENCES users(id),
  product_plan_id uuid REFERENCES product_plans(id),
  product_text text NOT NULL,
  plan_text text,
  region_code text NOT NULL,
  channel text NOT NULL,
  opening_method text NOT NULL,
  source_url text NOT NULL,
  source_title text,
  evidence_summary text NOT NULL,
  note text,
  status text NOT NULL CHECK (status IN ('pending', 'changes_requested', 'approved', 'rejected')),
  reviewed_by_admin_id uuid REFERENCES users(id),
  reviewed_at timestamptz,
  review_reason text,
  observed_at timestamptz NOT NULL,
  billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'annual', 'one_time', 'custom')),
  commitment_months integer CHECK (commitment_months IS NULL OR commitment_months > 0),
  price_unit text NOT NULL CHECK (price_unit IN ('per_account', 'per_seat', 'per_package')),
  seat_count integer CHECK (seat_count IS NULL OR seat_count > 0),
  quantity integer NOT NULL CHECK (quantity > 0),
  currency char(3) NOT NULL CHECK (currency = upper(currency)),
  original_amount numeric(12,2) NOT NULL CHECK (original_amount >= 0),
  original_price_text text NOT NULL,
  tax_included boolean NOT NULL,
  normalized_monthly_cny numeric(12,2),
  fx_rate numeric(18,8),
  fx_source text,
  fx_observed_at timestamptz,
  conversion_mode text,
  rounding_rule text,
  fingerprint text,
  offer_key text,
  duplicate_of_lead_id uuid REFERENCES official_price_leads(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1
);

CREATE INDEX ix_official_price_leads_submitter_created
ON official_price_leads(submitter_user_id, created_at DESC);

CREATE TABLE official_price_records (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  lead_id uuid NOT NULL UNIQUE REFERENCES official_price_leads(id),
  product_plan_id uuid NOT NULL REFERENCES product_plans(id),
  region_code text NOT NULL,
  channel text NOT NULL,
  opening_method text NOT NULL,
  source_url text NOT NULL,
  approved_by_admin_id uuid NOT NULL REFERENCES users(id),
  approved_at timestamptz NOT NULL,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz,
  status text NOT NULL CHECK (status IN ('active', 'superseded', 'expired', 'taken_down')),
  observed_at timestamptz NOT NULL,
  billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'annual', 'one_time', 'custom')),
  commitment_months integer CHECK (commitment_months IS NULL OR commitment_months > 0),
  price_unit text NOT NULL CHECK (price_unit IN ('per_account', 'per_seat', 'per_package')),
  seat_count integer CHECK (seat_count IS NULL OR seat_count > 0),
  quantity integer NOT NULL CHECK (quantity > 0),
  currency char(3) NOT NULL CHECK (currency = upper(currency)),
  original_amount numeric(12,2) NOT NULL CHECK (original_amount >= 0),
  tax_included boolean NOT NULL,
  normalized_monthly_cny numeric(12,2) NOT NULL,
  fx_rate numeric(18,8) NOT NULL,
  fx_source text NOT NULL,
  fx_observed_at timestamptz NOT NULL,
  conversion_mode text NOT NULL,
  rounding_rule text NOT NULL,
  fingerprint text NOT NULL,
  offer_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX ux_official_price_records_active_offer
ON official_price_records(offer_key)
WHERE status = 'active';
