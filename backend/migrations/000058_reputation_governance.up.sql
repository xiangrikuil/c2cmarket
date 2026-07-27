-- 增加纠纷主体、可撤销信誉裁定和角色/动作限制治理。
-- 日期：2026-07-24
-- 执行者：Codex

ALTER TABLE dispute_cases
ADD COLUMN subject_user_id uuid REFERENCES users(id);

CREATE INDEX ix_dispute_cases_subject_status_updated
ON dispute_cases(subject_user_id, status, updated_at DESC)
WHERE subject_user_id IS NOT NULL;

CREATE TABLE dispute_reputation_outcomes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dispute_case_id uuid NOT NULL UNIQUE REFERENCES dispute_cases(id),
  subject_user_id uuid NOT NULL REFERENCES users(id),
  responsibility text NOT NULL CHECK (responsibility IN (
    'responsible',
    'shared',
    'not_responsible',
    'undetermined'
  )),
  severity text NOT NULL CHECK (severity IN (
    'none',
    'low',
    'medium',
    'high',
    'critical'
  )),
  role_scope text NOT NULL CHECK (role_scope IN ('buyer', 'seller', 'all')),
  status text NOT NULL CHECK (status IN ('active', 'reversed')),
  reason_code text NOT NULL CHECK (reason_code ~ '^[a-z][a-z0-9_]{1,63}$'),
  public_reason text NOT NULL CHECK (trim(public_reason) <> ''),
  internal_reason text NOT NULL CHECK (trim(internal_reason) <> ''),
  decided_by_admin_id uuid NOT NULL REFERENCES users(id),
  decided_at timestamptz NOT NULL,
  reversed_at timestamptz,
  reversed_by_admin_id uuid REFERENCES users(id),
  reversal_appeal_id uuid REFERENCES appeals(id) ON DELETE SET NULL,
  reversal_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CHECK (
    (status = 'active'
      AND reversed_at IS NULL
      AND reversed_by_admin_id IS NULL
      AND reversal_appeal_id IS NULL
      AND reversal_reason = '')
    OR
    (status = 'reversed'
      AND reversed_at IS NOT NULL
      AND reversed_by_admin_id IS NOT NULL
      AND trim(reversal_reason) <> ''
      AND reversed_at >= decided_at)
  )
);

CREATE INDEX ix_dispute_reputation_outcomes_subject_status
ON dispute_reputation_outcomes(subject_user_id, status, updated_at DESC);

ALTER TABLE user_restrictions
ADD COLUMN role_scope text NOT NULL DEFAULT 'all',
ADD COLUMN action_code text NOT NULL DEFAULT 'all',
ADD COLUMN reason_code text NOT NULL DEFAULT 'legacy',
ADD COLUMN public_reason text NOT NULL DEFAULT '',
ADD COLUMN source_dispute_outcome_id uuid REFERENCES dispute_reputation_outcomes(id) ON DELETE SET NULL,
ADD COLUMN revoked_at timestamptz,
ADD COLUMN revoked_by_admin_id uuid REFERENCES users(id),
ADD COLUMN revocation_reason text NOT NULL DEFAULT '',
ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
ADD COLUMN version bigint NOT NULL DEFAULT 1;

UPDATE user_restrictions
SET public_reason = COALESCE(NULLIF(trim(reason), ''), '历史限制')
WHERE public_reason = '';

ALTER TABLE user_restrictions
ADD CONSTRAINT ck_user_restrictions_role_scope
CHECK (role_scope IN ('buyer', 'seller', 'all')),
ADD CONSTRAINT ck_user_restrictions_action_code
CHECK (action_code IN (
  'carpool_publish',
  'carpool_apply',
  'carpool_accept',
  'api_service_publish',
  'api_order_create',
  'contact_view',
  'review_submit',
  'all'
)),
ADD CONSTRAINT ck_user_restrictions_reason_code
CHECK (reason_code ~ '^[a-z][a-z0-9_]{1,63}$'),
ADD CONSTRAINT ck_user_restrictions_public_reason
CHECK (trim(public_reason) <> ''),
ADD CONSTRAINT ck_user_restrictions_version
CHECK (version > 0),
ADD CONSTRAINT ck_user_restrictions_period
CHECK (ends_at IS NULL OR ends_at > starts_at),
ADD CONSTRAINT ck_user_restrictions_revocation
CHECK (
  (revoked_at IS NULL
    AND revoked_by_admin_id IS NULL
    AND revocation_reason = '')
  OR
  (revoked_at IS NOT NULL
    AND revoked_by_admin_id IS NOT NULL
    AND trim(revocation_reason) <> '')
);

CREATE INDEX ix_user_restrictions_active_action
ON user_restrictions(user_id, role_scope, action_code, starts_at, ends_at)
WHERE revoked_at IS NULL;

CREATE TABLE reputation_governance_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL CHECK (entity_type IN ('outcome', 'restriction')),
  entity_id uuid NOT NULL,
  action text NOT NULL CHECK (action IN (
    'outcome_created',
    'outcome_reversed',
    'restriction_created',
    'restriction_revoked'
  )),
  actor_admin_id uuid NOT NULL REFERENCES users(id),
  before_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  reason text NOT NULL CHECK (trim(reason) <> ''),
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL
);

CREATE INDEX ix_reputation_governance_events_entity
ON reputation_governance_events(entity_type, entity_id, created_at DESC, id DESC);

CREATE INDEX ix_reputation_governance_events_actor
ON reputation_governance_events(actor_admin_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_reputation_governance_event_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'reputation governance events are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_reputation_governance_events_append_only
BEFORE UPDATE OR DELETE ON reputation_governance_events
FOR EACH ROW
EXECUTE FUNCTION reject_reputation_governance_event_mutation();
