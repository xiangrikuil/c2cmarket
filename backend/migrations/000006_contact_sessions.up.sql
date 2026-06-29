-- Contact window infrastructure contract.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE contact_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  seller_user_id uuid NOT NULL REFERENCES users(id),
  opens_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('open', 'expired', 'revoked')),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (buyer_user_id <> seller_user_id)
);

CREATE TABLE contact_session_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  contact_session_id uuid NOT NULL REFERENCES contact_sessions(id),
  subject_user_id uuid NOT NULL REFERENCES users(id),
  side text NOT NULL CHECK (side IN ('buyer', 'seller')),
  contact_method_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY(contact_method_version_id, subject_user_id) REFERENCES contact_method_versions(id, owner_user_id)
);

CREATE TABLE contact_access_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  contact_session_id uuid NOT NULL REFERENCES contact_sessions(id),
  viewer_user_id uuid NOT NULL REFERENCES users(id),
  accessed_at timestamptz NOT NULL DEFAULT now(),
  request_id text NOT NULL
);
