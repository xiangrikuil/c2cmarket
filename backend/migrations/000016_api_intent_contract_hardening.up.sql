-- Harden API purchase intent identity, contact snapshots, and selected access mode.
-- 日期：2026-06-22
-- 执行者：Codex

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS api_purchase_intents_buyer_contact_method_version_id_buyer_fkey,
DROP CONSTRAINT IF EXISTS api_purchase_intents_owner_contact_method_version_id_owner_fkey,
DROP CONSTRAINT IF EXISTS api_purchase_intents_check4,
DROP CONSTRAINT IF EXISTS api_purchase_intents_check5,
DROP CONSTRAINT IF EXISTS api_purchase_intents_check6;

ALTER TABLE api_purchase_intents
ADD COLUMN IF NOT EXISTS selected_access_mode text,
ADD COLUMN IF NOT EXISTS buyer_contact_type_snapshot text,
ADD COLUMN IF NOT EXISTS buyer_contact_label_snapshot text,
ADD COLUMN IF NOT EXISTS owner_contact_type_snapshot text,
ADD COLUMN IF NOT EXISTS owner_contact_label_snapshot text;

UPDATE api_purchase_intents intent
SET
  selected_access_mode = COALESCE(
    selected_access_mode,
    (
      SELECT mode.access_mode
      FROM api_service_access_modes mode
      WHERE mode.api_service_id = intent.api_service_id
      ORDER BY mode.access_mode ASC
      LIMIT 1
    )
  ),
  buyer_contact_type_snapshot = COALESCE(buyer_contact_type_snapshot, buyer_method.type),
  buyer_contact_label_snapshot = COALESCE(buyer_contact_label_snapshot, buyer_method.label),
  owner_contact_type_snapshot = COALESCE(owner_contact_type_snapshot, owner_method.type),
  owner_contact_label_snapshot = COALESCE(owner_contact_label_snapshot, owner_method.label)
FROM contact_methods buyer_method, contact_methods owner_method
WHERE buyer_method.id = intent.buyer_contact_method_id
  AND owner_method.id = intent.owner_contact_method_id;

ALTER TABLE api_purchase_intents
ALTER COLUMN selected_access_mode SET NOT NULL,
ALTER COLUMN buyer_contact_type_snapshot SET NOT NULL,
ALTER COLUMN buyer_contact_label_snapshot SET NOT NULL,
ALTER COLUMN owner_contact_type_snapshot SET NOT NULL,
ALTER COLUMN owner_contact_label_snapshot SET NOT NULL;

ALTER TABLE api_purchase_intents
ADD CONSTRAINT fk_api_intent_buyer_contact_version_identity
FOREIGN KEY (buyer_contact_method_version_id, buyer_contact_method_id, buyer_user_id)
REFERENCES contact_method_versions(id, contact_method_id, owner_user_id),
ADD CONSTRAINT fk_api_intent_owner_contact_version_identity
FOREIGN KEY (owner_contact_method_version_id, owner_contact_method_id, owner_user_id)
REFERENCES contact_method_versions(id, contact_method_id, owner_user_id),
ADD CONSTRAINT fk_api_intent_selected_access_mode
FOREIGN KEY (api_service_id, selected_access_mode)
REFERENCES api_service_access_modes(api_service_id, access_mode),
ADD CONSTRAINT fk_api_intent_selected_package
FOREIGN KEY (api_service_id, selected_package_id)
REFERENCES api_service_packages(api_service_id, id),
ADD CONSTRAINT ck_api_intent_status_timestamps
CHECK (
  (
    status = 'open'
    AND contacted_at IS NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'contacted'
    AND contacted_at IS NOT NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'buyer_cancelled'
    AND buyer_cancelled_at IS NOT NULL
    AND buyer_cancel_reason IS NOT NULL
    AND owner_closed_at IS NULL
    AND owner_close_reason IS NULL
  )
  OR (
    status = 'owner_closed'
    AND owner_closed_at IS NOT NULL
    AND owner_close_reason IS NOT NULL
    AND buyer_cancelled_at IS NULL
    AND buyer_cancel_reason IS NULL
  )
),
ADD CONSTRAINT ck_api_intent_billing_selection
CHECK (
  (
    billing_mode_snapshot = 'fixed_package'
    AND selected_package_id IS NOT NULL
    AND selected_package_snapshot IS NOT NULL
    AND requested_usd_allowance IS NULL
  )
  OR (
    billing_mode_snapshot <> 'fixed_package'
    AND selected_package_id IS NULL
  )
);
