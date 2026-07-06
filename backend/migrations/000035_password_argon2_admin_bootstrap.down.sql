ALTER TABLE user_password_credentials
  DROP CONSTRAINT IF EXISTS user_password_credentials_password_algorithm_check;

ALTER TABLE user_password_credentials
  ADD CONSTRAINT user_password_credentials_password_algorithm_check
  CHECK (password_algorithm IN ('sha256_salted_v1'));
