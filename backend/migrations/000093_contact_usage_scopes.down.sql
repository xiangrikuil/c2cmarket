-- 出现迁移后的显式范围策略后，拒绝回滚并抹除该策略。
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM contact_methods
    WHERE usage_scopes <> ARRAY[
      'carpool_owner',
      'api_merchant',
      'buyer',
      'dispute'
    ]::text[]
  ) THEN
    RAISE EXCEPTION 'cannot roll back migration 93 after explicit contact usage scopes exist';
  END IF;
END;
$$;

ALTER TABLE contact_methods
  DROP CONSTRAINT IF EXISTS ck_contact_methods_usage_scopes,
  DROP COLUMN IF EXISTS usage_scopes;

DROP FUNCTION IF EXISTS canonical_contact_usage_scopes(text[]);
