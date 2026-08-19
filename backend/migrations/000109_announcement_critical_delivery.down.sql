-- Roll back strong announcement delivery.
-- Date: 2026-08-16
-- Author: Codex

DROP INDEX IF EXISTS ix_announcement_recipients_user_version;
DROP TABLE IF EXISTS announcement_recipients;

ALTER TABLE announcement_receipts
  DROP COLUMN IF EXISTS acknowledged_at;

ALTER TABLE announcements
  DROP CONSTRAINT IF EXISTS ck_announcements_delivery_matrix,
  DROP CONSTRAINT IF EXISTS ck_announcements_channels_supported;

UPDATE announcements
SET level = CASE WHEN level = 'critical' THEN 'important' ELSE level END,
    channels = ARRAY(
      SELECT channel
      FROM unnest(channels) AS channel
      WHERE channel IN ('message_center', 'home_banner')
    ),
    requires_ack = false;

ALTER TABLE announcements
  DROP COLUMN IF EXISTS requires_ack,
  DROP CONSTRAINT announcements_level_check;

ALTER TABLE announcements
  ADD CONSTRAINT announcements_level_check
  CHECK (level IN ('normal', 'important'));
