-- WeChat is the single account-wide transaction contact. The service layer
-- returns a product error first; this index closes concurrent-write races.
UPDATE contact_methods
SET usage_scopes = ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[],
    updated_at = now(),
    version = version + 1
WHERE type = 'wechat'
  AND usage_scopes IS DISTINCT FROM ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[];

CREATE UNIQUE INDEX ux_contact_methods_one_enabled_wechat
ON contact_methods(user_id)
WHERE type = 'wechat' AND enabled = true;
