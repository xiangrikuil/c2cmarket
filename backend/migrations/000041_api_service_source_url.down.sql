-- Roll back optional API service linux.do source topic.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE api_services
DROP CONSTRAINT IF EXISTS ck_api_services_source_url_linuxdo;

ALTER TABLE api_services
DROP COLUMN IF EXISTS source_url;
