-- Argon2id password algorithm support and fixed admin seed cleanup.
-- 日期：2026-07-06
-- 执行者：Codex

ALTER TABLE user_password_credentials
  DROP CONSTRAINT IF EXISTS user_password_credentials_password_algorithm_check;

ALTER TABLE user_password_credentials
  ADD CONSTRAINT user_password_credentials_password_algorithm_check
  CHECK (password_algorithm IN ('argon2id_v1', 'sha256_salted_v1'));

DELETE FROM user_password_credentials c
USING users u
WHERE c.user_id = u.id
  AND u.username = 'admin'
  AND c.password_algorithm = 'sha256_salted_v1'
  AND length(c.password_salt) = 8;
