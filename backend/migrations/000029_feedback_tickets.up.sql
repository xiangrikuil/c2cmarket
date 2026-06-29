-- Feedback loop ticket contract.
-- 日期：2026-06-26
-- 执行者：Codex

CREATE TABLE feedback_tickets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  submitter_user_id uuid NOT NULL REFERENCES users(id),
  type text NOT NULL CHECK (type IN ('function_issue', 'data_correction', 'experience_suggestion', 'publish_contact_block')),
  impact text NOT NULL CHECK (impact IN ('general', 'blocks_operation', 'cannot_continue')),
  status text NOT NULL CHECK (status IN ('submitted', 'recorded', 'following_up', 'resolved', 'declined', 'needs_user_info', 'closed')),
  title text NOT NULL,
  description text NOT NULL,
  context_page_label text NOT NULL,
  context_target_type text NOT NULL DEFAULT '',
  context_target_id text NOT NULL DEFAULT '',
  context_target_label text NOT NULL DEFAULT '',
  context_role_label text NOT NULL DEFAULT '',
  admin_response text NOT NULL DEFAULT '',
  admin_internal_note text NOT NULL DEFAULT '',
  handled_by_admin_id uuid REFERENCES users(id),
  handled_at timestamptz,
  latest_admin_update_at timestamptz,
  submitter_read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1
);

CREATE INDEX ix_feedback_tickets_submitter_updated
ON feedback_tickets(submitter_user_id, updated_at DESC);

CREATE INDEX ix_feedback_tickets_admin_status_updated
ON feedback_tickets(status, updated_at DESC);

CREATE INDEX ix_feedback_tickets_unread_submitter
ON feedback_tickets(submitter_user_id, latest_admin_update_at DESC)
WHERE latest_admin_update_at IS NOT NULL;

CREATE TABLE feedback_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id uuid NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
  actor_user_id uuid REFERENCES users(id),
  actor_role text NOT NULL CHECK (actor_role IN ('user', 'admin', 'system')),
  action text NOT NULL CHECK (action IN ('submitted', 'admin_handled', 'user_supplemented', 'read')),
  public_message text NOT NULL DEFAULT '',
  internal_note text NOT NULL DEFAULT '',
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_feedback_events_ticket_created
ON feedback_events(ticket_id, created_at ASC);

CREATE INDEX ix_feedback_events_actor_created
ON feedback_events(actor_user_id, created_at DESC)
WHERE actor_user_id IS NOT NULL;
