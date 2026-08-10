ALTER TABLE announcements
ADD COLUMN content_updated_at timestamptz;

UPDATE announcements
SET content_updated_at = publish_at;

ALTER TABLE announcements
ALTER COLUMN content_updated_at SET NOT NULL;
