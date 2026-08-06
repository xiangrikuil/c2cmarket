-- API 服务管理员运营推广排期。
-- 日期：2026-08-02
-- 执行者：Codex

CREATE TABLE api_service_promotions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL REFERENCES api_services(id),
  placement text NOT NULL CHECK (placement IN ('api_market_top')),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  created_reason text NOT NULL CHECK (trim(created_reason) <> '' AND char_length(created_reason) <= 500),
  created_by_admin_id uuid NOT NULL REFERENCES users(id),
  stopped_at timestamptz,
  stopped_by_admin_id uuid REFERENCES users(id),
  stopped_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CONSTRAINT ck_api_service_promotions_period CHECK (ends_at > starts_at),
  CONSTRAINT ck_api_service_promotions_stop_facts CHECK (
    (stopped_at IS NULL AND stopped_by_admin_id IS NULL AND stopped_reason = '')
    OR
    (stopped_at IS NOT NULL AND stopped_by_admin_id IS NOT NULL AND trim(stopped_reason) <> '' AND char_length(stopped_reason) <= 500)
  )
);

CREATE INDEX ix_api_service_promotions_placement_period
ON api_service_promotions(placement, starts_at, ends_at);

CREATE INDEX ix_api_service_promotions_service_period
ON api_service_promotions(api_service_id, starts_at, ends_at);

CREATE INDEX ix_api_service_promotions_lifecycle
ON api_service_promotions(stopped_at, ends_at);
