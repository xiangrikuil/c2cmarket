-- Carpool membership cycle terms, completion, and exit lifecycle.
-- 日期：2026-06-22
-- 执行者：Codex

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS carpool_memberships_status_check;

ALTER TABLE carpool_memberships
ADD CONSTRAINT ck_carpool_memberships_status
CHECK (status IN ('active', 'completed', 'left', 'removed'));

ALTER TABLE carpool_memberships
ADD COLUMN cycle_term_id uuid,
ADD COLUMN ended_reason text NOT NULL DEFAULT '',
ADD COLUMN ended_by_user_id uuid REFERENCES users(id);

UPDATE carpool_memberships
SET ended_reason = 'legacy_end_state'
WHERE status <> 'active'
  AND ended_reason = '';

ALTER TABLE carpool_listings
ADD CONSTRAINT uq_carpool_listings_id_owner
UNIQUE (id, owner_user_id);

ALTER TABLE carpool_memberships
DROP CONSTRAINT IF EXISTS carpool_memberships_check;

ALTER TABLE carpool_memberships
ADD CONSTRAINT ck_carpool_membership_end_state
CHECK (
  (status = 'active' AND ended_at IS NULL AND ended_reason = '' AND ended_by_user_id IS NULL)
  OR (status <> 'active' AND ended_at IS NOT NULL AND ended_reason <> '')
);

CREATE TABLE carpool_cycle_terms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  carpool_listing_id uuid NOT NULL UNIQUE REFERENCES carpool_listings(id) ON DELETE CASCADE,
  owner_user_id uuid NOT NULL REFERENCES users(id),
  billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'weekly', 'custom')),
  cycle_start_day integer CHECK (cycle_start_day BETWEEN 1 AND 31),
  notice_days integer NOT NULL DEFAULT 0 CHECK (notice_days >= 0 AND notice_days <= 365),
  exit_policy text NOT NULL,
  usage_rules text NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, carpool_listing_id, owner_user_id),
  FOREIGN KEY (carpool_listing_id, owner_user_id)
    REFERENCES carpool_listings(id, owner_user_id)
);

CREATE INDEX ix_carpool_cycle_terms_owner
ON carpool_cycle_terms(owner_user_id, updated_at DESC);

INSERT INTO carpool_cycle_terms (
  carpool_listing_id, owner_user_id, billing_period, cycle_start_day,
  notice_days, exit_policy, usage_rules, created_at, updated_at
)
SELECT
  id,
  owner_user_id,
  'monthly',
  1,
  0,
  '历史车源规则未结构化，申请前需由双方在站外确认账期和退出安排。',
  '历史车源规则未结构化，平台不得收集、保存或转交任何密码、API Key、Token、Cookie 或 Session。',
  created_at,
  updated_at
FROM carpool_listings
ON CONFLICT (carpool_listing_id) DO NOTHING;

ALTER TABLE carpool_memberships
ADD CONSTRAINT fk_carpool_membership_cycle_term
FOREIGN KEY (cycle_term_id, carpool_listing_id, owner_user_id)
REFERENCES carpool_cycle_terms(id, carpool_listing_id, owner_user_id);

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
