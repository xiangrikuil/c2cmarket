-- Harden OAuth identity ownership and record first-admin bootstrap provenance.
-- 日期：2026-07-26
-- 执行者：Codex

CREATE TABLE admin_bootstrap_runs (
  bootstrap_key text PRIMARY KEY CHECK (trim(bootstrap_key) <> ''),
  user_id uuid NOT NULL UNIQUE REFERENCES users(id),
  username_snapshot text NOT NULL UNIQUE CHECK (trim(username_snapshot) <> ''),
  created_at timestamptz NOT NULL
);
