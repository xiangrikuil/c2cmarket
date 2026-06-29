-- Contact method versioning contract.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE contact_methods (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  type text NOT NULL CHECK (type IN ('linuxdo', 'telegram', 'wechat', 'email', 'other')),
  label text NOT NULL,
  current_version_id uuid,
  is_default boolean NOT NULL DEFAULT false,
  enabled boolean NOT NULL DEFAULT true,
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE(id, user_id)
);

CREATE TABLE contact_method_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  contact_method_id uuid NOT NULL,
  owner_user_id uuid NOT NULL,
  value_ciphertext bytea NOT NULL,
  value_nonce bytea NOT NULL,
  masked_value text NOT NULL,
  value_fingerprint text NOT NULL,
  encryption_key_version text NOT NULL,
  fingerprint_key_version text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  retired_at timestamptz,
  destroyed_at timestamptz,
  UNIQUE(id, owner_user_id),
  FOREIGN KEY(contact_method_id, owner_user_id) REFERENCES contact_methods(id, user_id)
);

ALTER TABLE contact_methods
ADD CONSTRAINT fk_contact_methods_current_version
FOREIGN KEY(current_version_id, user_id) REFERENCES contact_method_versions(id, owner_user_id);
