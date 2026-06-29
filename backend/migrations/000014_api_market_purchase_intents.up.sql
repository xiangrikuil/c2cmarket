-- API market purchase intent contract.
-- 日期：2026-06-22
-- 执行者：Codex

CREATE TABLE api_purchase_intents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL,
  api_service_owner_user_id uuid NOT NULL,
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  buyer_contact_method_id uuid NOT NULL,
  buyer_contact_method_version_id uuid NOT NULL,
  owner_contact_method_id uuid NOT NULL,
  owner_contact_method_version_id uuid NOT NULL,
  status text NOT NULL CHECK (status IN ('open', 'contacted', 'buyer_cancelled', 'owner_closed')),
  requested_cny_amount numeric(12,2) NOT NULL CHECK (requested_cny_amount > 0),
  requested_usd_allowance numeric(18,6) CHECK (requested_usd_allowance IS NULL OR requested_usd_allowance > 0),
  selected_access_mode text NOT NULL,
  selected_package_id uuid,
  selected_package_snapshot jsonb,
  service_version_snapshot bigint NOT NULL,
  service_title_snapshot text NOT NULL,
  distribution_system_snapshot text NOT NULL,
  billing_mode_snapshot text NOT NULL,
  buyer_contact_type_snapshot text NOT NULL,
  buyer_contact_label_snapshot text NOT NULL,
  owner_contact_type_snapshot text NOT NULL,
  owner_contact_label_snapshot text NOT NULL,
  declared_cny_per_usd_allowance_snapshot numeric(12,4),
  declared_max_usd_allowance_per_intent_snapshot numeric(18,6),
  minimum_intent_cny_snapshot numeric(12,2) NOT NULL,
  maximum_intent_cny_snapshot numeric(12,2),
  pricing_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  buyer_note text,
  contacted_at timestamptz,
  buyer_cancelled_at timestamptz,
  buyer_cancel_reason text,
  owner_closed_at timestamptz,
  owner_close_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, buyer_user_id),
  UNIQUE (id, owner_user_id),
  FOREIGN KEY (api_service_id, api_service_owner_user_id) REFERENCES api_services(id, owner_user_id),
  FOREIGN KEY (buyer_contact_method_id, buyer_user_id) REFERENCES contact_methods(id, user_id),
  FOREIGN KEY (owner_contact_method_id, owner_user_id) REFERENCES contact_methods(id, user_id),
  FOREIGN KEY (buyer_contact_method_version_id, buyer_contact_method_id, buyer_user_id) REFERENCES contact_method_versions(id, contact_method_id, owner_user_id),
  FOREIGN KEY (owner_contact_method_version_id, owner_contact_method_id, owner_user_id) REFERENCES contact_method_versions(id, contact_method_id, owner_user_id),
  FOREIGN KEY (api_service_id, selected_access_mode) REFERENCES api_service_access_modes(api_service_id, access_mode),
  FOREIGN KEY (api_service_id, selected_package_id) REFERENCES api_service_packages(api_service_id, id),
  CHECK (api_service_owner_user_id = owner_user_id),
  CHECK (buyer_user_id <> owner_user_id),
  CHECK (maximum_intent_cny_snapshot IS NULL OR maximum_intent_cny_snapshot >= minimum_intent_cny_snapshot),
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
  )
);

CREATE INDEX ix_api_purchase_intents_buyer
ON api_purchase_intents(buyer_user_id, updated_at DESC);

CREATE INDEX ix_api_purchase_intents_owner
ON api_purchase_intents(owner_user_id, updated_at DESC);

CREATE INDEX ix_api_purchase_intents_admin
ON api_purchase_intents(status, updated_at DESC);

CREATE INDEX ix_api_purchase_intents_service
ON api_purchase_intents(api_service_id, created_at DESC);

CREATE UNIQUE INDEX ux_api_purchase_intents_active_buyer_service
ON api_purchase_intents(buyer_user_id, api_service_id)
WHERE status IN ('open', 'contacted');
