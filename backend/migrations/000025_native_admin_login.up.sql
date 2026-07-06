-- Native account password credential foundation.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE user_password_credentials (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_algorithm text NOT NULL CONSTRAINT user_password_credentials_password_algorithm_check CHECK (password_algorithm IN ('argon2id_v1', 'sha256_salted_v1')),
  password_salt text NOT NULL,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  password_updated_at timestamptz NOT NULL DEFAULT now()
);
