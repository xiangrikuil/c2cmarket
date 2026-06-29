-- Reports, disputes, and appeals real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_user_id uuid NOT NULL REFERENCES users(id),
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent')),
  target_id text NOT NULL,
  target_label text NOT NULL DEFAULT '',
  reported_user_id uuid REFERENCES users(id),
  reported_username text NOT NULL DEFAULT '',
  reason_code text NOT NULL CHECK (reason_code IN ('invalid', 'unreachable', 'impersonation', 'other')),
  title text NOT NULL,
  description text NOT NULL,
  status text NOT NULL CHECK (status IN ('submitted', 'triaged', 'rejected', 'dispute_opened')),
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

CREATE TABLE dispute_cases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id uuid UNIQUE REFERENCES reports(id) ON DELETE SET NULL,
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent')),
  target_id text NOT NULL,
  target_label text NOT NULL DEFAULT '',
  primary_user_id uuid NOT NULL REFERENCES users(id),
  counterparty_user_id uuid REFERENCES users(id),
  status text NOT NULL CHECK (status IN ('open', 'waiting_info', 'resolved', 'closed')),
  public_summary text NOT NULL,
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
  target_type text NOT NULL CHECK (target_type IN ('contact_snapshot', 'public_user', 'carpool_membership', 'api_purchase_intent')),
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
