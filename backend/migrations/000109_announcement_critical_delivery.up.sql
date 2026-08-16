-- Add strong announcement delivery, acknowledgement receipts, and recipient snapshots.
-- Date: 2026-08-16
-- Author: Codex

ALTER TABLE announcements
  DROP CONSTRAINT announcements_level_check;

ALTER TABLE announcements
  ADD CONSTRAINT announcements_level_check
  CHECK (level IN ('normal', 'important', 'critical'));

ALTER TABLE announcements
  ADD COLUMN requires_ack boolean NOT NULL DEFAULT false;

ALTER TABLE announcements
  ADD CONSTRAINT ck_announcements_channels_supported
  CHECK (channels <@ ARRAY['message_center', 'home_banner', 'global_bar', 'modal']::text[]),
  ADD CONSTRAINT ck_announcements_delivery_matrix
  CHECK (
    (level = 'normal' AND NOT requires_ack
      AND array_position(channels, 'global_bar') IS NULL
      AND array_position(channels, 'modal') IS NULL)
    OR
    (level = 'important' AND NOT requires_ack
      AND array_position(channels, 'modal') IS NULL)
    OR
    (level = 'critical' AND requires_ack AND NOT is_dismissible
      AND array_position(channels, 'global_bar') IS NOT NULL
      AND array_position(channels, 'modal') IS NOT NULL)
  );

ALTER TABLE announcement_receipts
  ADD COLUMN acknowledged_at timestamptz;

CREATE TABLE announcement_recipients (
  announcement_id uuid NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  announcement_version bigint NOT NULL,
  snapshotted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (announcement_id, user_id)
);

CREATE INDEX ix_announcement_recipients_user_version
ON announcement_recipients(user_id, announcement_id, announcement_version);
