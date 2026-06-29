-- User favorites real backend contract.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE favorites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  target_type text NOT NULL CHECK (target_type IN ('carpool', 'api_service')),
  target_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, target_type, target_id)
);

CREATE INDEX ix_favorites_user_created
ON favorites(user_id, created_at DESC);

CREATE INDEX ix_favorites_target
ON favorites(target_type, target_id);
