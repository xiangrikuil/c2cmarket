-- Registered-user growth facts and privacy-preserving analytics identity.
-- Date: 2026-08-02
-- Executor: Codex

ALTER TABLE users
ADD COLUMN analytics_user_id uuid NOT NULL DEFAULT gen_random_uuid(),
ADD CONSTRAINT uq_users_analytics_user_id UNIQUE (analytics_user_id);

CREATE TABLE user_registration_attributions (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  registration_method text NOT NULL CHECK (registration_method IN ('oauth_linux_do', 'email', 'unknown')),
  source_type text NOT NULL CHECK (source_type IN ('campaign', 'referral', 'direct', 'unknown')),
  source text NOT NULL CHECK (trim(source) <> '' AND char_length(source) <= 100),
  medium text CHECK (medium IS NULL OR (trim(medium) <> '' AND char_length(medium) <= 100)),
  campaign text CHECK (campaign IS NULL OR (trim(campaign) <> '' AND char_length(campaign) <= 100)),
  referrer_host text CHECK (referrer_host IS NULL OR (trim(referrer_host) <> '' AND char_length(referrer_host) <= 255)),
  landing_path text NOT NULL CHECK (
    left(landing_path, 1) = '/'
    AND position('?' IN landing_path) = 0
    AND position('#' IN landing_path) = 0
    AND char_length(landing_path) <= 160
  ),
  captured_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_user_registration_attributions_source
ON user_registration_attributions(source_type, source, captured_at DESC);

INSERT INTO user_registration_attributions (
  user_id,
  registration_method,
  source_type,
  source,
  landing_path,
  captured_at
)
SELECT
  users.id,
  CASE
    WHEN EXISTS (
      SELECT 1
      FROM auth_identities identity
      WHERE identity.user_id = users.id
        AND replace(replace(lower(identity.provider), '.', ''), '_', '') = 'linuxdo'
    ) THEN 'oauth_linux_do'
    WHEN users.email_verified_at IS NOT NULL THEN 'email'
    ELSE 'unknown'
  END,
  'unknown',
  'unknown',
  '/',
  users.created_at
FROM users
ON CONFLICT (user_id) DO NOTHING;

CREATE TABLE user_activity_daily (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  activity_date date NOT NULL,
  first_seen_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, activity_date)
);

CREATE INDEX ix_user_activity_daily_date
ON user_activity_daily(activity_date, user_id);

ALTER TABLE carpool_listings
ADD COLUMN first_published_at timestamptz;

ALTER TABLE api_services
ADD COLUMN first_published_at timestamptz;

UPDATE carpool_listings
SET first_published_at = COALESCE(reviewed_at, updated_at, created_at)
WHERE first_published_at IS NULL
  AND status IN ('active', 'paused');

UPDATE api_services
SET first_published_at = COALESCE(approved_at, updated_at, created_at)
WHERE first_published_at IS NULL
  AND review_status = 'approved'
  AND publication_status IN ('online', 'owner_paused');

CREATE INDEX ix_carpool_listings_owner_first_published
ON carpool_listings(owner_user_id, first_published_at)
WHERE first_published_at IS NOT NULL;

CREATE INDEX ix_api_services_owner_first_published
ON api_services(owner_user_id, first_published_at)
WHERE first_published_at IS NOT NULL;

CREATE OR REPLACE FUNCTION preserve_carpool_first_published_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND OLD.first_published_at IS NOT NULL THEN
    NEW.first_published_at := OLD.first_published_at;
  ELSIF NEW.first_published_at IS NULL AND NEW.status = 'active' THEN
    NEW.first_published_at := COALESCE(NEW.reviewed_at, NEW.updated_at, now());
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_carpool_first_published_at
BEFORE INSERT OR UPDATE ON carpool_listings
FOR EACH ROW
EXECUTE FUNCTION preserve_carpool_first_published_at();

CREATE OR REPLACE FUNCTION preserve_api_service_first_published_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND OLD.first_published_at IS NOT NULL THEN
    NEW.first_published_at := OLD.first_published_at;
  ELSIF NEW.first_published_at IS NULL
    AND NEW.review_status = 'approved'
    AND NEW.publication_status = 'online'
    AND NEW.moderation_status = 'clear'
  THEN
    NEW.first_published_at := COALESCE(NEW.approved_at, NEW.updated_at, now());
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_api_service_first_published_at
BEFORE INSERT OR UPDATE ON api_services
FOR EACH ROW
EXECUTE FUNCTION preserve_api_service_first_published_at();
