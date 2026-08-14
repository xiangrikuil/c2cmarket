-- linux.do 联系方式由账号身份绑定统一管理。历史交易快照保持不变，
-- 仅归并仍会参与后续业务写入的联系方式引用。
CREATE TEMP TABLE linuxdo_contact_canonical ON COMMIT DROP AS
SELECT user_id, id AS canonical_id
FROM (
  SELECT
    user_id,
    id,
    row_number() OVER (
      PARTITION BY user_id
      ORDER BY is_default DESC, created_at ASC, id ASC
    ) AS position
  FROM contact_methods
  WHERE type = 'linuxdo'
    AND enabled = true
) ranked
WHERE position = 1;

UPDATE api_services service
SET owner_contact_method_id = canonical.canonical_id
FROM linuxdo_contact_canonical canonical,
     contact_methods current_method
WHERE service.owner_user_id = canonical.user_id
  AND current_method.id = service.owner_contact_method_id
  AND current_method.user_id = canonical.user_id
  AND current_method.type = 'linuxdo'
  AND service.owner_contact_method_id <> canonical.canonical_id;

CREATE TEMP TABLE linuxdo_service_contact_positions ON COMMIT DROP AS
SELECT
  selected.api_service_id,
  selected.owner_user_id,
  canonical.canonical_id AS contact_method_id,
  min(selected.sort_order) AS sort_order,
  min(selected.created_at) AS created_at
FROM api_service_contact_methods selected
JOIN contact_methods method
  ON method.id = selected.contact_method_id
 AND method.user_id = selected.owner_user_id
JOIN linuxdo_contact_canonical canonical
  ON canonical.user_id = selected.owner_user_id
WHERE method.type = 'linuxdo'
GROUP BY selected.api_service_id, selected.owner_user_id, canonical.canonical_id;

DELETE FROM api_service_contact_methods selected
USING contact_methods method,
      linuxdo_contact_canonical canonical
WHERE method.id = selected.contact_method_id
  AND method.user_id = selected.owner_user_id
  AND method.type = 'linuxdo'
  AND canonical.user_id = selected.owner_user_id;

INSERT INTO api_service_contact_methods (
  api_service_id,
  owner_user_id,
  contact_method_id,
  sort_order,
  created_at
)
SELECT
  api_service_id,
  owner_user_id,
  contact_method_id,
  sort_order,
  created_at
FROM linuxdo_service_contact_positions;

UPDATE carpool_listings listing
SET owner_contact_method_id = canonical.canonical_id
FROM linuxdo_contact_canonical canonical,
     contact_methods current_method
WHERE listing.owner_user_id = canonical.user_id
  AND current_method.id = listing.owner_contact_method_id
  AND current_method.user_id = canonical.user_id
  AND current_method.type = 'linuxdo'
  AND listing.owner_contact_method_id <> canonical.canonical_id;

UPDATE carpool_applications application
SET buyer_contact_method_id = canonical.canonical_id
FROM linuxdo_contact_canonical canonical,
     contact_methods current_method
WHERE application.buyer_user_id = canonical.user_id
  AND current_method.id = application.buyer_contact_method_id
  AND current_method.user_id = canonical.user_id
  AND current_method.type = 'linuxdo'
  AND application.buyer_contact_method_id <> canonical.canonical_id;

UPDATE contact_methods method
SET enabled = false,
    is_default = false,
    updated_at = now(),
    version = version + 1
FROM linuxdo_contact_canonical canonical
WHERE method.user_id = canonical.user_id
  AND method.type = 'linuxdo'
  AND method.enabled = true
  AND method.id <> canonical.canonical_id;

CREATE UNIQUE INDEX ux_contact_methods_one_enabled_linuxdo
ON contact_methods(user_id)
WHERE type = 'linuxdo' AND enabled = true;
