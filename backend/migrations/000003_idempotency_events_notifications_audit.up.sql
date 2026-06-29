-- Idempotency, domain events, notifications, and admin audit logs.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE idempotency_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  route_key text NOT NULL,
  idempotency_key text NOT NULL,
  request_hash text NOT NULL,
  status text NOT NULL CHECK (status IN ('processing', 'completed')),
  locked_until timestamptz,
  response_status integer,
  response_content_type text,
  response_body_json jsonb,
  response_body_cache_allowed boolean NOT NULL DEFAULT false,
  resource_type text,
  resource_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  expires_at timestamptz NOT NULL,
  UNIQUE(user_id, route_key, idempotency_key)
);

CREATE INDEX ix_idempotency_keys_expires_at
ON idempotency_keys(expires_at);

CREATE TABLE domain_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  aggregate_type text NOT NULL,
  aggregate_id uuid NOT NULL,
  event_type text NOT NULL,
  actor_user_id uuid REFERENCES users(id),
  actor_kind text NOT NULL CHECK (actor_kind IN ('user', 'admin', 'system')),
  aggregate_version bigint NOT NULL,
  request_id text NOT NULL,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(aggregate_type, aggregate_id, aggregate_version)
);

CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  type text NOT NULL,
  title text NOT NULL,
  body text NOT NULL,
  target_type text NOT NULL,
  target_id uuid NOT NULL,
  target_url text NOT NULL,
  source_event_type text NOT NULL,
  source_event_id uuid REFERENCES domain_events(id),
  dedupe_key text,
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_notifications_user_dedupe
ON notifications(user_id, dedupe_key)
WHERE dedupe_key IS NOT NULL;

CREATE TABLE admin_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid NOT NULL REFERENCES users(id),
  action text NOT NULL,
  target_type text NOT NULL,
  target_id uuid NOT NULL,
  reason text,
  before_json jsonb,
  after_json jsonb,
  request_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
