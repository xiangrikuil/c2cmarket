-- Freeze multiple API-service contacts and record after-sales issue occurrence time.
-- Date: 2026-08-10
-- Author: Codex

CREATE TABLE api_service_contact_methods (
  api_service_id uuid NOT NULL,
  owner_user_id uuid NOT NULL,
  contact_method_id uuid NOT NULL,
  sort_order integer NOT NULL CHECK (sort_order >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (api_service_id, contact_method_id),
  UNIQUE (api_service_id, sort_order),
  FOREIGN KEY (api_service_id, owner_user_id)
    REFERENCES api_services(id, owner_user_id) ON DELETE CASCADE,
  FOREIGN KEY (contact_method_id, owner_user_id)
    REFERENCES contact_methods(id, user_id) ON DELETE RESTRICT
);

INSERT INTO api_service_contact_methods (
  api_service_id, owner_user_id, contact_method_id, sort_order, created_at
)
SELECT id, owner_user_id, owner_contact_method_id, 0, created_at
FROM api_services;

CREATE INDEX ix_api_service_contact_methods_owner
ON api_service_contact_methods(owner_user_id, api_service_id, sort_order);

CREATE TABLE api_purchase_intent_owner_contact_snapshots (
  api_purchase_intent_id uuid NOT NULL,
  owner_user_id uuid NOT NULL,
  contact_method_id uuid NOT NULL,
  contact_method_version_id uuid NOT NULL,
  contact_type_snapshot text NOT NULL,
  contact_label_snapshot text NOT NULL,
  sort_order integer NOT NULL CHECK (sort_order >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (api_purchase_intent_id, contact_method_id),
  UNIQUE (api_purchase_intent_id, sort_order),
  FOREIGN KEY (api_purchase_intent_id, owner_user_id)
    REFERENCES api_purchase_intents(id, owner_user_id) ON DELETE CASCADE,
  FOREIGN KEY (contact_method_version_id, contact_method_id, owner_user_id)
    REFERENCES contact_method_versions(id, contact_method_id, owner_user_id) ON DELETE RESTRICT
);

INSERT INTO api_purchase_intent_owner_contact_snapshots (
  api_purchase_intent_id,
  owner_user_id,
  contact_method_id,
  contact_method_version_id,
  contact_type_snapshot,
  contact_label_snapshot,
  sort_order,
  created_at
)
SELECT
  id,
  owner_user_id,
  owner_contact_method_id,
  owner_contact_method_version_id,
  owner_contact_type_snapshot,
  owner_contact_label_snapshot,
  0,
  created_at
FROM api_purchase_intents;

CREATE INDEX ix_api_intent_owner_contact_snapshots_owner
ON api_purchase_intent_owner_contact_snapshots(owner_user_id, api_purchase_intent_id, sort_order);

ALTER TABLE dispute_cases
ADD COLUMN issue_occurred_at timestamptz;
