-- 联系方式使用范围持久化。
-- 日期：2026-08-12
-- 执行者：Codex

CREATE FUNCTION canonical_contact_usage_scopes(scopes text[])
RETURNS text[]
LANGUAGE sql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
  SELECT COALESCE(
    array_agg(scope ORDER BY CASE scope
      WHEN 'carpool_owner' THEN 1
      WHEN 'api_merchant' THEN 2
      WHEN 'buyer' THEN 3
      WHEN 'dispute' THEN 4
      ELSE 5
    END),
    ARRAY[]::text[]
  )
  FROM (
    SELECT DISTINCT unnest(scopes) AS scope
  ) normalized;
$$;

ALTER TABLE contact_methods
  ADD COLUMN usage_scopes text[];

-- 历史联系方式早于显式范围选择，因此保留原先支持的全部用途。
UPDATE contact_methods
SET usage_scopes = ARRAY[
  'carpool_owner',
  'api_merchant',
  'buyer',
  'dispute'
]::text[];

ALTER TABLE contact_methods
  ALTER COLUMN usage_scopes SET DEFAULT ARRAY['buyer', 'dispute']::text[],
  ALTER COLUMN usage_scopes SET NOT NULL,
  ADD CONSTRAINT ck_contact_methods_usage_scopes
    CHECK (
      cardinality(usage_scopes) > 0
      AND array_position(usage_scopes, NULL) IS NULL
      AND usage_scopes <@ ARRAY[
        'carpool_owner',
        'api_merchant',
        'buyer',
        'dispute'
      ]::text[]
      AND usage_scopes = canonical_contact_usage_scopes(usage_scopes)
    );
