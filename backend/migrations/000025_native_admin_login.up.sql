-- Native account password credential foundation and initial admin account.
-- 日期：2026-06-23
-- 执行者：Codex

CREATE TABLE user_password_credentials (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_algorithm text NOT NULL CHECK (password_algorithm IN ('sha256_salted_v1')),
  password_salt text NOT NULL,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  password_updated_at timestamptz NOT NULL DEFAULT now()
);

WITH admin_user AS (
  INSERT INTO users (username, display_name, account_status, created_at, updated_at)
  VALUES ('admin', 'C2CMarket Admin', 'active', now(), now())
  ON CONFLICT (username) DO UPDATE
  SET display_name = COALESCE(NULLIF(users.display_name, ''), EXCLUDED.display_name),
      updated_at = users.updated_at
  RETURNING id
)
INSERT INTO user_permissions (user_id, permission)
SELECT id, 'admin' FROM admin_user
ON CONFLICT DO NOTHING;

WITH admin_user AS (
  SELECT id FROM users WHERE username = 'admin'
)
INSERT INTO user_password_credentials (
  user_id,
  password_algorithm,
  password_salt,
  password_hash,
  created_at,
  password_updated_at
)
SELECT
  id,
  'sha256_salted_v1',
  '03d25913',
  '7923c4653183e601eb3267759668895a85d46cbccdd65515b297f07674191192',
  now(),
  now()
FROM admin_user
ON CONFLICT (user_id) DO NOTHING;
