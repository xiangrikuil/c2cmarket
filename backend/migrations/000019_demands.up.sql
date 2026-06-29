-- Demand posts real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE demands (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  publisher_user_id uuid NOT NULL REFERENCES users(id),
  title text NOT NULL,
  max_price_cny numeric(12,2) NOT NULL CHECK (max_price_cny > 0),
  region_code text NOT NULL,
  owner_preference text NOT NULL CHECK (owner_preference IN ('personal', 'only_personal', 'any')),
  source_url text NOT NULL,
  note text,
  status text NOT NULL CHECK (status IN ('pending_review', 'active', 'changes_requested', 'rejected', 'closed', 'taken_down')),
  review_reason text,
  reviewed_by_admin_id uuid REFERENCES users(id),
  reviewed_at timestamptz,
  closed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (source_url LIKE 'https://linux.do/t/%')
);

CREATE INDEX ix_demands_public_active
ON demands(updated_at DESC)
WHERE status = 'active';

CREATE INDEX ix_demands_publisher_updated
ON demands(publisher_user_id, updated_at DESC);

CREATE INDEX ix_demands_admin_status_updated
ON demands(status, updated_at DESC);
