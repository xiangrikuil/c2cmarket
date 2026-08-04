-- Explicit moderation information requests and immutable user supplements.
-- Date: 2026-08-03
-- Author: Codex

CREATE TABLE moderation_info_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL CHECK (entity_type IN ('report', 'dispute')),
  report_id uuid REFERENCES reports(id) ON DELETE CASCADE,
  dispute_case_id uuid REFERENCES dispute_cases(id) ON DELETE CASCADE,
  requested_from_user_id uuid NOT NULL REFERENCES users(id),
  requested_by_admin_id uuid NOT NULL REFERENCES users(id),
  internal_reason text NOT NULL,
  status text NOT NULL CHECK (status IN ('open', 'answered', 'cancelled')),
  requested_at timestamptz NOT NULL,
  answered_at timestamptz,
  cancelled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (entity_type = 'report' AND report_id IS NOT NULL AND dispute_case_id IS NULL)
    OR (entity_type = 'dispute' AND report_id IS NULL AND dispute_case_id IS NOT NULL)
  ),
  CHECK (
    (status = 'open' AND answered_at IS NULL AND cancelled_at IS NULL)
    OR (status = 'answered' AND answered_at IS NOT NULL AND cancelled_at IS NULL)
    OR (status = 'cancelled' AND answered_at IS NULL AND cancelled_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX ux_moderation_info_requests_open_report
ON moderation_info_requests(report_id)
WHERE status = 'open' AND report_id IS NOT NULL;

CREATE UNIQUE INDEX ux_moderation_info_requests_open_dispute
ON moderation_info_requests(dispute_case_id)
WHERE status = 'open' AND dispute_case_id IS NOT NULL;

CREATE INDEX ix_moderation_info_requests_requested_user
ON moderation_info_requests(requested_from_user_id, status, requested_at DESC);

CREATE TABLE moderation_info_supplements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  info_request_id uuid NOT NULL UNIQUE REFERENCES moderation_info_requests(id) ON DELETE RESTRICT,
  submitted_by_user_id uuid NOT NULL REFERENCES users(id),
  body text NOT NULL,
  created_at timestamptz NOT NULL
);

CREATE INDEX ix_moderation_info_supplements_submitter
ON moderation_info_supplements(submitted_by_user_id, created_at DESC);

CREATE OR REPLACE FUNCTION reject_moderation_info_supplement_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'moderation information supplements are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_moderation_info_supplements_append_only
BEFORE UPDATE OR DELETE ON moderation_info_supplements
FOR EACH ROW
EXECUTE FUNCTION reject_moderation_info_supplement_mutation();
