-- Independent community identity labels; these are not reputation badges.
-- Date: 2026-08-18
-- Executor: Codex

CREATE TABLE user_community_identities (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  identity_type text NOT NULL CHECK (identity_type IN ('FOUNDING_USER', 'BETA_CONTRIBUTOR')),
  source text NOT NULL CHECK (source IN ('AUTO', 'ADMIN', 'BACKFILL')),
  qualified_at timestamptz,
  granted_at timestamptz NOT NULL,
  granted_by uuid REFERENCES users(id),
  grant_reason text,
  revoked_at timestamptz,
  revoked_by uuid REFERENCES users(id),
  revoke_reason text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (user_id, identity_type),
  CHECK ((source = 'ADMIN' AND granted_by IS NOT NULL AND NULLIF(BTRIM(grant_reason), '') IS NOT NULL)
      OR (source <> 'ADMIN' AND granted_by IS NULL))
);

CREATE INDEX ix_user_community_identities_user_active
ON user_community_identities(user_id, revoked_at, identity_type);
