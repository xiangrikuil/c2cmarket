-- 增加资源级原帖作者验证、审计事件和信誉快照失效联动。
-- 日期：2026-07-24
-- 执行者：Codex

CREATE TABLE source_author_verifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_type text NOT NULL CHECK (resource_type IN ('carpool', 'api_service')),
  resource_id uuid NOT NULL,
  source_url text NOT NULL CHECK (trim(source_url) <> ''),
  expected_external_user_id text NOT NULL DEFAULT '',
  actual_external_user_id text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('not_submitted', 'pending', 'verified', 'mismatch', 'expired')),
  verification_method text NOT NULL DEFAULT '',
  verified_by_admin_id uuid NOT NULL REFERENCES users(id),
  verified_at timestamptz,
  expires_at timestamptz,
  failure_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  UNIQUE (resource_type, resource_id),
  CHECK (status <> 'verified' OR trim(actual_external_user_id) <> ''),
  CHECK (status <> 'mismatch' OR trim(actual_external_user_id) <> ''),
  CHECK (status <> 'mismatch' OR trim(failure_reason) <> ''),
  CHECK (expires_at IS NULL OR verified_at IS NULL OR expires_at >= verified_at)
);

CREATE INDEX ix_source_author_verifications_status
ON source_author_verifications(status, expires_at, resource_type, resource_id);

CREATE TABLE source_author_verification_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  verification_id uuid NOT NULL REFERENCES source_author_verifications(id),
  resource_type text NOT NULL CHECK (resource_type IN ('carpool', 'api_service')),
  resource_id uuid NOT NULL,
  action text NOT NULL CHECK (action IN ('created', 'updated')),
  from_status text CHECK (
    from_status IS NULL
    OR from_status IN ('not_submitted', 'pending', 'verified', 'mismatch', 'expired')
  ),
  to_status text NOT NULL CHECK (to_status IN ('not_submitted', 'pending', 'verified', 'mismatch', 'expired')),
  source_url text NOT NULL,
  expected_external_user_id text NOT NULL DEFAULT '',
  actual_external_user_id text NOT NULL DEFAULT '',
  verification_method text NOT NULL DEFAULT '',
  verified_by_admin_id uuid NOT NULL REFERENCES users(id),
  verified_at timestamptz,
  expires_at timestamptz,
  failure_reason text NOT NULL DEFAULT '',
  version bigint NOT NULL CHECK (version > 0),
  created_at timestamptz NOT NULL
);

CREATE INDEX ix_source_author_verification_events_resource
ON source_author_verification_events(resource_type, resource_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_source_author_verification_event_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'source author verification events are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_source_author_verification_events_append_only
BEFORE UPDATE OR DELETE ON source_author_verification_events
FOR EACH ROW
EXECUTE FUNCTION reject_source_author_verification_event_mutation();

CREATE OR REPLACE FUNCTION dirty_reputation_for_source_author_verification()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_type text;
  target_id uuid;
  owner_id uuid;
  changed_at timestamptz;
BEGIN
  IF TG_OP = 'DELETE' THEN
    target_type := OLD.resource_type;
    target_id := OLD.resource_id;
    changed_at := COALESCE(OLD.updated_at, now());
  ELSE
    target_type := NEW.resource_type;
    target_id := NEW.resource_id;
    changed_at := COALESCE(NEW.updated_at, now());
  END IF;

  IF target_type = 'carpool' THEN
    SELECT owner_user_id INTO owner_id FROM carpool_listings WHERE id = target_id;
  ELSIF target_type = 'api_service' THEN
    SELECT owner_user_id INTO owner_id FROM api_services WHERE id = target_id;
  END IF;

  IF owner_id IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(owner_id, changed_at);
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_source_author_verifications_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON source_author_verifications
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_source_author_verification();

CREATE OR REPLACE FUNCTION dirty_reputation_for_source_resource()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  old_owner_id uuid;
  new_owner_id uuid;
  changed_at timestamptz := now();
BEGIN
  IF TG_OP IN ('UPDATE', 'DELETE') THEN
    old_owner_id := OLD.owner_user_id;
    changed_at := COALESCE(OLD.updated_at, now());
  END IF;
  IF TG_OP IN ('INSERT', 'UPDATE') THEN
    new_owner_id := NEW.owner_user_id;
    changed_at := COALESCE(NEW.updated_at, now());
  END IF;

  IF old_owner_id IS NOT NULL THEN
    PERFORM mark_user_reputation_dirty(old_owner_id, changed_at);
  END IF;
  IF new_owner_id IS NOT NULL AND new_owner_id IS DISTINCT FROM old_owner_id THEN
    PERFORM mark_user_reputation_dirty(new_owner_id, changed_at);
  END IF;
  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_carpool_listings_source_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON carpool_listings
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_source_resource();

CREATE TRIGGER trg_api_services_source_reputation_dirty
AFTER INSERT OR UPDATE OR DELETE ON api_services
FOR EACH ROW
EXECUTE FUNCTION dirty_reputation_for_source_resource();
