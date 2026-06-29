-- Roll back API purchase intent contract hardening.
-- 日期：2026-06-22
-- 执行者：Codex

ALTER TABLE api_purchase_intents
DROP CONSTRAINT IF EXISTS ck_api_intent_billing_selection,
DROP CONSTRAINT IF EXISTS ck_api_intent_status_timestamps,
DROP CONSTRAINT IF EXISTS fk_api_intent_selected_package,
DROP CONSTRAINT IF EXISTS fk_api_intent_selected_access_mode,
DROP CONSTRAINT IF EXISTS fk_api_intent_owner_contact_version_identity,
DROP CONSTRAINT IF EXISTS fk_api_intent_buyer_contact_version_identity;

ALTER TABLE api_purchase_intents
ADD CONSTRAINT api_purchase_intents_buyer_contact_method_version_id_buyer_fkey
FOREIGN KEY (buyer_contact_method_version_id, buyer_user_id)
REFERENCES contact_method_versions(id, owner_user_id),
ADD CONSTRAINT api_purchase_intents_owner_contact_method_version_id_owner_fkey
FOREIGN KEY (owner_contact_method_version_id, owner_user_id)
REFERENCES contact_method_versions(id, owner_user_id);

ALTER TABLE api_purchase_intents
DROP COLUMN IF EXISTS owner_contact_label_snapshot,
DROP COLUMN IF EXISTS owner_contact_type_snapshot,
DROP COLUMN IF EXISTS buyer_contact_label_snapshot,
DROP COLUMN IF EXISTS buyer_contact_type_snapshot,
DROP COLUMN IF EXISTS selected_access_mode;
