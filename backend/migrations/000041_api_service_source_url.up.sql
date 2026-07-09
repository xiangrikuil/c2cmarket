-- Store optional linux.do source topic for API quota services.
-- 日期：2026-07-08
-- 执行者：Codex

ALTER TABLE api_services
ADD COLUMN source_url text;

ALTER TABLE api_services
ADD CONSTRAINT ck_api_services_source_url_linuxdo
CHECK (source_url IS NULL OR source_url LIKE 'https://linux.do/t/%');
