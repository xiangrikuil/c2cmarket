-- 回退纠纷信誉治理结构，保留原纠纷、账号和交易记录。
-- 日期：2026-07-24
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_reputation_governance_events_append_only
ON reputation_governance_events;

DROP FUNCTION IF EXISTS reject_reputation_governance_event_mutation();
DROP TABLE IF EXISTS reputation_governance_events;

DROP INDEX IF EXISTS ix_user_restrictions_active_action;

ALTER TABLE user_restrictions
DROP CONSTRAINT IF EXISTS ck_user_restrictions_revocation,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_period,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_version,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_public_reason,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_reason_code,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_action_code,
DROP CONSTRAINT IF EXISTS ck_user_restrictions_role_scope,
DROP COLUMN IF EXISTS version,
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS revocation_reason,
DROP COLUMN IF EXISTS revoked_by_admin_id,
DROP COLUMN IF EXISTS revoked_at,
DROP COLUMN IF EXISTS source_dispute_outcome_id,
DROP COLUMN IF EXISTS public_reason,
DROP COLUMN IF EXISTS reason_code,
DROP COLUMN IF EXISTS action_code,
DROP COLUMN IF EXISTS role_scope;

DROP TABLE IF EXISTS dispute_reputation_outcomes;

DROP INDEX IF EXISTS ix_dispute_cases_subject_status_updated;

ALTER TABLE dispute_cases
DROP COLUMN IF EXISTS subject_user_id;
