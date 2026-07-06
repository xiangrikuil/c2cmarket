-- Reports, disputes, and appeals real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_user_id uuid NOT NULL REFERENCES users(id),
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
  target_id text NOT NULL,
  canonical_target_type text NOT NULL CHECK (canonical_target_type IN ('public_user', 'contact_snapshot', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
  canonical_target_id text NOT NULL,
  target_label text NOT NULL DEFAULT '',
  target_snapshot_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  reported_user_id uuid REFERENCES users(id),
  reported_username text NOT NULL DEFAULT '',
  reason_code text NOT NULL CHECK (reason_code IN ('unreachable', 'contact_invalid', 'impersonation', 'description_mismatch', 'seat_rule_dispute', 'api_quota_dispute', 'order_delivery_dispute', 'other')),
  title text NOT NULL,
  description text NOT NULL,
  status text NOT NULL CHECK (status IN ('submitted', 'triaged', 'needs_info', 'rejected', 'dispute_opened', 'closed')),
  admin_reason text NOT NULL DEFAULT '',
  handled_by_admin_id uuid REFERENCES users(id),
  handled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (reporter_user_id <> reported_user_id OR reported_user_id IS NULL)
);

CREATE INDEX ix_reports_reporter_updated
ON reports(reporter_user_id, updated_at DESC);

CREATE INDEX ix_reports_admin_status_updated
ON reports(status, updated_at DESC);

CREATE INDEX ix_reports_target
ON reports(target_type, target_id);

CREATE INDEX ix_reports_canonical_target
ON reports(canonical_target_type, canonical_target_id);

CREATE UNIQUE INDEX ux_reports_active_canonical_target
ON reports(reporter_user_id, canonical_target_type, canonical_target_id)
WHERE status IN ('submitted', 'triaged', 'needs_info', 'dispute_opened');

CREATE TABLE dispute_cases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id uuid UNIQUE REFERENCES reports(id) ON DELETE SET NULL,
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
  target_id text NOT NULL,
  target_label text NOT NULL DEFAULT '',
  primary_user_id uuid NOT NULL REFERENCES users(id),
  counterparty_user_id uuid REFERENCES users(id),
  status text NOT NULL CHECK (status IN ('open', 'waiting_info', 'resolved', 'closed')),
  public_summary text NOT NULL,
  public_result_code text NOT NULL DEFAULT 'no_action' CHECK (public_result_code IN ('no_action', 'contact_invalid', 'impersonation_confirmed', 'description_mismatch', 'rule_or_seat_issue', 'api_delivery_issue', 'other_resolved')),
  public_result text NOT NULL,
  admin_reason text NOT NULL DEFAULT '',
  opened_by_admin_id uuid NOT NULL REFERENCES users(id),
  opened_at timestamptz NOT NULL,
  resolved_at timestamptz,
  closed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (primary_user_id <> counterparty_user_id OR counterparty_user_id IS NULL)
);

CREATE INDEX ix_dispute_cases_primary_updated
ON dispute_cases(primary_user_id, updated_at DESC);

CREATE INDEX ix_dispute_cases_counterparty_updated
ON dispute_cases(counterparty_user_id, updated_at DESC)
WHERE counterparty_user_id IS NOT NULL;

CREATE INDEX ix_dispute_cases_admin_status_updated
ON dispute_cases(status, updated_at DESC);

CREATE INDEX ix_dispute_cases_target
ON dispute_cases(target_type, target_id);

ALTER TABLE reports
ADD COLUMN dispute_case_id uuid UNIQUE REFERENCES dispute_cases(id) ON DELETE SET NULL;

CREATE TABLE appeals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  appellant_user_id uuid NOT NULL REFERENCES users(id),
  report_id uuid REFERENCES reports(id) ON DELETE SET NULL,
  dispute_case_id uuid REFERENCES dispute_cases(id) ON DELETE SET NULL,
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order')),
  target_id text NOT NULL,
  title text NOT NULL,
  statement text NOT NULL,
  status text NOT NULL CHECK (status IN ('submitted', 'approved', 'rejected')),
  admin_reason text NOT NULL DEFAULT '',
  handled_by_admin_id uuid REFERENCES users(id),
  handled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (report_id IS NOT NULL OR dispute_case_id IS NOT NULL)
);

CREATE INDEX ix_appeals_appellant_updated
ON appeals(appellant_user_id, updated_at DESC);

CREATE INDEX ix_appeals_admin_status_updated
ON appeals(status, updated_at DESC);

CREATE INDEX ix_appeals_target
ON appeals(target_type, target_id);

CREATE TABLE dispute_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL CHECK (entity_type IN ('report', 'dispute', 'appeal')),
  entity_id uuid NOT NULL,
  action text NOT NULL,
  actor_user_id uuid REFERENCES users(id),
  actor_role text NOT NULL CHECK (actor_role IN ('user', 'admin', 'system')),
  reason text NOT NULL DEFAULT '',
  public boolean NOT NULL DEFAULT false,
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_dispute_events_entity
ON dispute_events(entity_type, entity_id, created_at DESC);

CREATE INDEX ix_dispute_events_actor
ON dispute_events(actor_user_id, created_at DESC)
WHERE actor_user_id IS NOT NULL;

CREATE TABLE moderation_audit_logs (
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

CREATE INDEX ix_moderation_audit_logs_actor
ON moderation_audit_logs(actor_admin_id, created_at DESC);

CREATE INDEX ix_moderation_audit_logs_object
ON moderation_audit_logs(object_type, object_id, created_at DESC);

CREATE INDEX ix_moderation_audit_logs_basis_report
ON moderation_audit_logs(basis_report_id, created_at DESC)
WHERE basis_report_id IS NOT NULL;

CREATE INDEX ix_moderation_audit_logs_basis_dispute_case
ON moderation_audit_logs(basis_dispute_case_id, created_at DESC)
WHERE basis_dispute_case_id IS NOT NULL;

CREATE INDEX ix_moderation_audit_logs_basis_appeal
ON moderation_audit_logs(basis_appeal_id, created_at DESC)
WHERE basis_appeal_id IS NOT NULL;
