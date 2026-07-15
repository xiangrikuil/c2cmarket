-- 补齐已应用旧版举报迁移缺失的字段、约束与审计表。
-- 日期：2026-07-11
-- 执行者：Codex

ALTER TABLE reports
ADD COLUMN IF NOT EXISTS canonical_target_type text,
ADD COLUMN IF NOT EXISTS canonical_target_id text,
ADD COLUMN IF NOT EXISTS target_snapshot_json jsonb NOT NULL DEFAULT '{}'::jsonb;

UPDATE reports
SET canonical_target_type = target_type,
    canonical_target_id = CASE
      WHEN target_type = 'public_user' AND reported_user_id IS NOT NULL THEN reported_user_id::text
      ELSE target_id
    END,
    target_snapshot_json = jsonb_build_object(
      'submittedTargetType', target_type,
      'submittedTargetId', target_id,
      'containsContactValue', false
    )
WHERE canonical_target_type IS NULL
   OR canonical_target_id IS NULL
   OR target_snapshot_json = '{}'::jsonb;

UPDATE reports
SET reason_code = 'other'
WHERE reason_code = 'invalid';

ALTER TABLE reports
DROP CONSTRAINT IF EXISTS reports_target_type_check,
DROP CONSTRAINT IF EXISTS reports_reason_code_check,
DROP CONSTRAINT IF EXISTS reports_status_check,
DROP CONSTRAINT IF EXISTS ck_reports_target_type,
DROP CONSTRAINT IF EXISTS ck_reports_canonical_target_type,
DROP CONSTRAINT IF EXISTS ck_reports_reason_code,
DROP CONSTRAINT IF EXISTS ck_reports_status;

WITH ranked_reports AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY reporter_user_id, canonical_target_type, canonical_target_id
           ORDER BY updated_at DESC, id DESC
         ) AS position
  FROM reports
  WHERE status IN ('submitted', 'triaged', 'dispute_opened')
)
UPDATE reports AS report
SET status = 'closed',
    admin_reason = COALESCE(NULLIF(report.admin_reason, ''), '历史重复举报已归档。'),
    updated_at = now(),
    version = version + 1
FROM ranked_reports
WHERE report.id = ranked_reports.id
  AND ranked_reports.position > 1;

ALTER TABLE reports
ALTER COLUMN canonical_target_type SET NOT NULL,
ALTER COLUMN canonical_target_id SET NOT NULL,
ADD CONSTRAINT ck_reports_target_type
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
ADD CONSTRAINT ck_reports_canonical_target_type
CHECK (canonical_target_type IN ('public_user', 'contact_snapshot', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
ADD CONSTRAINT ck_reports_reason_code
CHECK (reason_code IN ('unreachable', 'contact_invalid', 'impersonation', 'description_mismatch', 'seat_rule_dispute', 'api_quota_dispute', 'order_delivery_dispute', 'other')),
ADD CONSTRAINT ck_reports_status
CHECK (status IN ('submitted', 'triaged', 'needs_info', 'rejected', 'dispute_opened', 'closed'));

CREATE INDEX IF NOT EXISTS ix_reports_canonical_target
ON reports(canonical_target_type, canonical_target_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_reports_active_canonical_target
ON reports(reporter_user_id, canonical_target_type, canonical_target_id)
WHERE status IN ('submitted', 'triaged', 'needs_info', 'dispute_opened');

ALTER TABLE dispute_cases
ADD COLUMN IF NOT EXISTS public_result_code text NOT NULL DEFAULT 'no_action';

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS dispute_cases_target_type_check,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_target_type,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_public_result_code,
ADD CONSTRAINT ck_dispute_cases_target_type
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
ADD CONSTRAINT ck_dispute_cases_public_result_code
CHECK (public_result_code IN ('no_action', 'contact_invalid', 'impersonation_confirmed', 'description_mismatch', 'rule_or_seat_issue', 'api_delivery_issue', 'other_resolved'));

ALTER TABLE appeals
DROP CONSTRAINT IF EXISTS appeals_target_type_check,
DROP CONSTRAINT IF EXISTS ck_appeals_target_type,
ADD CONSTRAINT ck_appeals_target_type
CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order'));

CREATE TABLE IF NOT EXISTS moderation_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_admin_id uuid NOT NULL REFERENCES users(id),
  action text NOT NULL CHECK (action IN ('triage', 'request_info', 'reject', 'open_dispute', 'close', 'resolve', 'approve')),
  object_type text NOT NULL CHECK (object_type IN ('report', 'dispute_case', 'appeal')),
  object_id uuid NOT NULL,
  basis_report_id uuid REFERENCES reports(id) ON DELETE SET NULL,
  basis_dispute_case_id uuid REFERENCES dispute_cases(id) ON DELETE SET NULL,
  basis_appeal_id uuid REFERENCES appeals(id) ON DELETE SET NULL,
  before_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  after_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  reason_internal text NOT NULL DEFAULT '',
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_actor
ON moderation_audit_logs(actor_admin_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_object
ON moderation_audit_logs(object_type, object_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_basis_report
ON moderation_audit_logs(basis_report_id, created_at DESC)
WHERE basis_report_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_basis_dispute_case
ON moderation_audit_logs(basis_dispute_case_id, created_at DESC)
WHERE basis_dispute_case_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_moderation_audit_logs_basis_appeal
ON moderation_audit_logs(basis_appeal_id, created_at DESC)
WHERE basis_appeal_id IS NOT NULL;
