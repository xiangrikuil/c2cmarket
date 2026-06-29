DELETE FROM user_password_credentials
WHERE user_id IN (SELECT id FROM users WHERE username = 'admin')
  AND password_algorithm = 'sha256_salted_v1'
  AND password_salt = '03d25913';

DROP TABLE IF EXISTS user_password_credentials;
