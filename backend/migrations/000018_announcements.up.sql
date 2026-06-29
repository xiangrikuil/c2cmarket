-- Announcement real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE announcements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  title text NOT NULL,
  summary text NOT NULL,
  content_markdown text NOT NULL,
  category text NOT NULL CHECK (category IN ('platform', 'rules', 'maintenance', 'feature', 'risk', 'operation')),
  level text NOT NULL CHECK (level IN ('normal', 'important')),
  status text NOT NULL CHECK (status IN ('draft', 'scheduled', 'published', 'offline', 'expired', 'archived')),
  channels text[] NOT NULL CHECK (array_position(channels, 'message_center') IS NOT NULL),
  audience_json jsonb NOT NULL DEFAULT '{"type":"all"}'::jsonb,
  is_pinned boolean NOT NULL DEFAULT false,
  is_dismissible boolean NOT NULL DEFAULT true,
  cta_label text,
  cta_url text,
  publish_at timestamptz NOT NULL,
  expire_at timestamptz,
  created_by_user_id uuid REFERENCES users(id),
  updated_by_user_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (expire_at IS NULL OR expire_at > publish_at),
  CHECK (array_length(channels, 1) >= 1)
);

CREATE INDEX ix_announcements_user_visible
ON announcements(status, publish_at DESC);

CREATE INDEX ix_announcements_home
ON announcements(publish_at DESC)
WHERE array_position(channels, 'home_banner') IS NOT NULL;

CREATE TABLE announcement_receipts (
  announcement_id uuid NOT NULL REFERENCES announcements(id),
  user_id uuid NOT NULL REFERENCES users(id),
  announcement_version bigint NOT NULL,
  first_seen_at timestamptz,
  read_at timestamptz,
  dismissed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (announcement_id, user_id)
);

CREATE TABLE announcement_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action text NOT NULL CHECK (action IN (
    'announcement_created',
    'announcement_updated',
    'announcement_published',
    'announcement_offlined',
    'announcement_duplicated'
  )),
  announcement_id uuid NOT NULL REFERENCES announcements(id),
  announcement_title text NOT NULL,
  operator_user_id uuid REFERENCES users(id),
  operator_name text NOT NULL,
  reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_announcement_audit_logs_created_at
ON announcement_audit_logs(created_at DESC);
