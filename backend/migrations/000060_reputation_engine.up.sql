-- 增加版本化信誉快照、等级历史和事实变更失效触发器。
-- 日期：2026-07-24
-- 执行者：Codex

CREATE TABLE user_reputation_states (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('buyer', 'seller')),
  scope text NOT NULL CHECK (scope IN ('overall', 'carpool', 'api')),
  tier text NOT NULL CHECK (tier IN ('insufficient', 'normal', 'reliable', 'high_trust')),
  state text NOT NULL CHECK (state IN ('active', 'caution', 'restricted')),
  confidence text NOT NULL CHECK (confidence IN ('low', 'medium', 'high')),
  rule_version text NOT NULL CHECK (trim(rule_version) <> ''),
  metrics_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  warnings_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  badges_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  progress_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  tier_entered_at timestamptz NOT NULL,
  reliable_since timestamptz,
  state_entered_at timestamptz NOT NULL,
  dirty_at timestamptz,
  calculated_at timestamptz NOT NULL,
  source_data_updated_at timestamptz,
  next_recalculation_at timestamptz,
  PRIMARY KEY (user_id, role, scope),
  CHECK (jsonb_typeof(metrics_json) = 'object'),
  CHECK (jsonb_typeof(warnings_json) = 'array'),
  CHECK (jsonb_typeof(badges_json) = 'array'),
  CHECK (jsonb_typeof(progress_json) = 'array'),
  CHECK (reliable_since IS NULL OR reliable_since <= calculated_at),
  CHECK (tier_entered_at <= calculated_at),
  CHECK (state_entered_at <= calculated_at)
);

CREATE INDEX ix_user_reputation_states_due
ON user_reputation_states(next_recalculation_at, user_id)
WHERE dirty_at IS NULL AND next_recalculation_at IS NOT NULL;

CREATE INDEX ix_user_reputation_states_dirty
ON user_reputation_states(dirty_at, user_id)
WHERE dirty_at IS NOT NULL;

CREATE INDEX ix_user_reputation_states_public_summary
ON user_reputation_states(role, scope, state, tier, user_id);

CREATE TABLE user_reputation_history (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('buyer', 'seller')),
  scope text NOT NULL CHECK (scope IN ('overall', 'carpool', 'api')),
  from_tier text CHECK (from_tier IS NULL OR from_tier IN ('insufficient', 'normal', 'reliable', 'high_trust')),
  to_tier text NOT NULL CHECK (to_tier IN ('insufficient', 'normal', 'reliable', 'high_trust')),
  from_state text CHECK (from_state IS NULL OR from_state IN ('active', 'caution', 'restricted')),
  to_state text NOT NULL CHECK (to_state IN ('active', 'caution', 'restricted')),
  rule_version text NOT NULL CHECK (trim(rule_version) <> ''),
  reason_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL,
  CHECK (jsonb_typeof(reason_snapshot) = 'object')
);

CREATE INDEX ix_user_reputation_history_user
ON user_reputation_history(user_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_user_reputation_history_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'user reputation history is append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_user_reputation_history_append_only
BEFORE UPDATE OR DELETE ON user_reputation_history
FOR EACH ROW
EXECUTE FUNCTION reject_user_reputation_history_mutation();

CREATE OR REPLACE FUNCTION mark_user_reputation_dirty(
  target_user_id uuid,
  changed_at timestamptz DEFAULT now()
)
RETURNS void
LANGUAGE sql
AS $$
  UPDATE user_reputation_states
  SET dirty_at = CASE
    WHEN dirty_at IS NULL THEN COALESCE(changed_at, now())
    ELSE LEAST(dirty_at, COALESCE(changed_at, now()))
  END
  WHERE user_id = target_user_id;
$$;

CREATE OR REPLACE FUNCTION mark_transaction_reputation_dirty(
  target_type text,
  target_id uuid,
  changed_at timestamptz DEFAULT now()
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  buyer_id uuid;
  seller_id uuid;
BEGIN
  CASE target_type
    WHEN 'carpool_application' THEN
      SELECT buyer_user_id, owner_user_id
      INTO buyer_id, seller_id
      FROM carpool_applications
      WHERE id = target_id;
    WHEN 'carpool_membership' THEN
      SELECT buyer_user_id, owner_user_id
      INTO buyer_id, seller_id
      FROM carpool_memberships
      WHERE id = target_id;
    WHEN 'api_purchase_intent' THEN
      SELECT buyer_user_id, owner_user_id
      INTO buyer_id, seller_id
      FROM api_purchase_intents
      WHERE id = target_id;
    WHEN 'api_order' THEN
      SELECT buyer_user_id, seller_user_id
      INTO buyer_id, seller_id
      FROM api_orders
      WHERE id = target_id;
    ELSE
      RETURN;
  END CASE;

  IF buyer_id IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(buyer_id, changed_at);
  END IF;
  IF seller_id IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(seller_id, changed_at);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION dirty_reputation_for_participant_row()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  old_buyer uuid;
  old_seller uuid;
  new_buyer uuid;
  new_seller uuid;
  changed_at timestamptz := now();
BEGIN
  IF TG_OP IN ('UPDATE', 'DELETE') THEN
    IF TG_TABLE_NAME IN ('carpool_applications', 'carpool_memberships') THEN
      old_buyer := OLD.buyer_user_id;
      old_seller := OLD.owner_user_id;
    ELSE
      old_buyer := OLD.buyer_user_id;
      old_seller := OLD.seller_user_id;
    END IF;
  END IF;

  IF TG_OP IN ('INSERT', 'UPDATE') THEN
    IF TG_TABLE_NAME IN ('carpool_applications', 'carpool_memberships') THEN
      new_buyer := NEW.buyer_user_id;
      new_seller := NEW.owner_user_id;
    ELSE
      new_buyer := NEW.buyer_user_id;
      new_seller := NEW.seller_user_id;
    END IF;
    changed_at := COALESCE(NEW.updated_at, now());
  ELSE
    changed_at := COALESCE(OLD.updated_at, now());
  END IF;

  IF old_buyer IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(old_buyer, changed_at);
  END IF;
  IF old_seller IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(old_seller, changed_at);
  END IF;
  IF new_buyer IS NOT NULL AND new_buyer IS DISTINCT FROM old_buyer THEN
    PERFORM mark_user_reputation_dirty(new_buyer, changed_at);
  END IF;
  IF new_seller IS NOT NULL AND new_seller IS DISTINCT FROM old_seller THEN
    PERFORM mark_user_reputation_dirty(new_seller, changed_at);
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_carpool_applications_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON carpool_applications
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_participant_row();

CREATE TRIGGER trg_carpool_memberships_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON carpool_memberships
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_participant_row();

CREATE TRIGGER trg_api_orders_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON api_orders
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_participant_row();

CREATE OR REPLACE FUNCTION dirty_reputation_for_api_order_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM mark_transaction_reputation_dirty(
      'api_order',
      OLD.api_order_id,
      COALESCE(OLD.created_at, now())
    );
  ELSE
    PERFORM mark_transaction_reputation_dirty(
      'api_order',
      NEW.api_order_id,
      COALESCE(NEW.created_at, now())
    );
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_api_order_events_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON api_order_events
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_api_order_event();

CREATE OR REPLACE FUNCTION dirty_reputation_for_exclusion()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM mark_transaction_reputation_dirty(
      OLD.transaction_type,
      OLD.transaction_id,
      COALESCE(OLD.updated_at, now())
    );
  ELSE
    PERFORM mark_transaction_reputation_dirty(
      NEW.transaction_type,
      NEW.transaction_id,
      COALESCE(NEW.updated_at, now())
    );
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_reputation_exclusions_state_dirty
AFTER INSERT OR UPDATE OR DELETE ON reputation_transaction_exclusions
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_exclusion();

CREATE OR REPLACE FUNCTION dirty_reputation_for_review()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  changed_at timestamptz;
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM mark_user_reputation_dirty(
      NEW.reviewee_user_id,
      COALESCE(NEW.updated_at, now())
    );
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM mark_user_reputation_dirty(
      OLD.reviewee_user_id,
      COALESCE(OLD.updated_at, now())
    );
  ELSE
    changed_at := COALESCE(NEW.updated_at, OLD.updated_at, now());
    PERFORM mark_user_reputation_dirty(OLD.reviewee_user_id, changed_at);
    IF NEW.reviewee_user_id IS DISTINCT FROM OLD.reviewee_user_id THEN
      PERFORM mark_user_reputation_dirty(NEW.reviewee_user_id, changed_at);
    END IF;
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_transaction_reviews_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON transaction_reviews
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_review();

CREATE OR REPLACE FUNCTION dirty_reputation_for_dispute()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  changed_at timestamptz;
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM mark_user_reputation_dirty(
      NEW.subject_user_id,
      COALESCE(NEW.updated_at, now())
    );
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM mark_user_reputation_dirty(
      OLD.subject_user_id,
      COALESCE(OLD.updated_at, now())
    );
  ELSE
    changed_at := COALESCE(NEW.updated_at, OLD.updated_at, now());
    PERFORM mark_user_reputation_dirty(OLD.subject_user_id, changed_at);
    IF NEW.subject_user_id IS DISTINCT FROM OLD.subject_user_id THEN
      PERFORM mark_user_reputation_dirty(NEW.subject_user_id, changed_at);
    END IF;
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_dispute_cases_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON dispute_cases
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_dispute();

CREATE OR REPLACE FUNCTION dirty_reputation_for_outcome()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  changed_at timestamptz;
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM mark_user_reputation_dirty(
      NEW.subject_user_id,
      COALESCE(NEW.updated_at, now())
    );
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM mark_user_reputation_dirty(
      OLD.subject_user_id,
      COALESCE(OLD.updated_at, now())
    );
  ELSE
    changed_at := COALESCE(NEW.updated_at, OLD.updated_at, now());
    PERFORM mark_user_reputation_dirty(OLD.subject_user_id, changed_at);
    IF NEW.subject_user_id IS DISTINCT FROM OLD.subject_user_id THEN
      PERFORM mark_user_reputation_dirty(NEW.subject_user_id, changed_at);
    END IF;
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_dispute_outcomes_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON dispute_reputation_outcomes
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_outcome();

CREATE OR REPLACE FUNCTION dirty_reputation_for_restriction()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  changed_at timestamptz;
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM mark_user_reputation_dirty(
      NEW.user_id,
      COALESCE(NEW.updated_at, now())
    );
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM mark_user_reputation_dirty(
      OLD.user_id,
      COALESCE(OLD.updated_at, now())
    );
  ELSE
    changed_at := COALESCE(NEW.updated_at, OLD.updated_at, now());
    PERFORM mark_user_reputation_dirty(OLD.user_id, changed_at);
    IF NEW.user_id IS DISTINCT FROM OLD.user_id THEN
      PERFORM mark_user_reputation_dirty(NEW.user_id, changed_at);
    END IF;
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_user_restrictions_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON user_restrictions
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_restriction();

CREATE OR REPLACE FUNCTION dirty_reputation_for_linux_do_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM mark_user_reputation_dirty(NEW.user_id, now());
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM mark_user_reputation_dirty(OLD.user_id, now());
  ELSE
    PERFORM mark_user_reputation_dirty(OLD.user_id, now());
    IF NEW.user_id IS DISTINCT FROM OLD.user_id THEN
      PERFORM mark_user_reputation_dirty(NEW.user_id, now());
    END IF;
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_linux_do_bindings_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON linux_do_bindings
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_linux_do_binding();
