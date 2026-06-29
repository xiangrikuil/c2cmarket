-- Product catalog, publish policy, and risk notice contracts.
-- 日期：2026-06-21
-- 执行者：Codex

CREATE TABLE product_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  display_name text NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true
);

CREATE TABLE risk_notices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE risk_notice_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  risk_notice_id uuid NOT NULL REFERENCES risk_notices(id),
  version integer NOT NULL,
  title text NOT NULL,
  body_markdown text NOT NULL,
  effective_at timestamptz NOT NULL,
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(risk_notice_id, version)
);

CREATE UNIQUE INDEX ux_risk_notice_current_version
ON risk_notice_versions(risk_notice_id)
WHERE retired_at IS NULL;

CREATE TABLE product_plans (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  category_id uuid NOT NULL REFERENCES product_categories(id),
  provider_code text NOT NULL,
  slug text NOT NULL UNIQUE,
  display_name text NOT NULL,
  description text NOT NULL DEFAULT '',
  publish_policy text NOT NULL DEFAULT 'info_only' CHECK (publish_policy IN ('allowed', 'info_only', 'blocked')),
  access_mode text NOT NULL CHECK (access_mode IN ('personal_account_cost_share', 'provider_member_invitation', 'owner_managed_access', 'other_off_platform', 'unsupported')),
  provider_policy_status text NOT NULL CHECK (provider_policy_status IN ('known_restricted', 'possibly_restricted', 'unknown')),
  risk_level text NOT NULL CHECK (risk_level IN ('normal', 'elevated', 'high')),
  risk_ack_required boolean NOT NULL DEFAULT false,
  risk_notice_code text REFERENCES risk_notices(code),
  policy_version bigint NOT NULL DEFAULT 1,
  policy_note text NOT NULL DEFAULT '',
  active boolean NOT NULL DEFAULT true,
  allow_custom_variant boolean NOT NULL DEFAULT false,
  sort_order integer NOT NULL DEFAULT 0,
  policy_updated_at timestamptz,
  policy_updated_by_user_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE product_plan_policy_history (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_plan_id uuid NOT NULL REFERENCES product_plans(id),
  policy_version bigint NOT NULL,
  publish_policy text NOT NULL CHECK (publish_policy IN ('allowed', 'info_only', 'blocked')),
  access_mode text NOT NULL CHECK (access_mode IN ('personal_account_cost_share', 'provider_member_invitation', 'owner_managed_access', 'other_off_platform', 'unsupported')),
  provider_policy_status text NOT NULL CHECK (provider_policy_status IN ('known_restricted', 'possibly_restricted', 'unknown')),
  risk_level text NOT NULL CHECK (risk_level IN ('normal', 'elevated', 'high')),
  risk_ack_required boolean NOT NULL,
  risk_notice_version_id uuid REFERENCES risk_notice_versions(id),
  enforcement_mode text NOT NULL CHECK (enforcement_mode IN ('new_actions_only', 'suspend_open_listings', 'close_unaccepted_interactions')),
  reason text NOT NULL,
  changed_by_admin_id uuid REFERENCES users(id),
  effective_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(product_plan_id, policy_version)
);
