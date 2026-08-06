-- Referral rewards and API service promotion coupons.
-- Date: 2026-08-02
-- Executor: Codex

CREATE TABLE promotion_reward_campaigns (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  program_enabled boolean NOT NULL DEFAULT false,
  welcome_enabled boolean NOT NULL DEFAULT false,
  referral_enabled boolean NOT NULL DEFAULT false,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz,
  promotion_duration_hours integer NOT NULL DEFAULT 24 CHECK (promotion_duration_hours BETWEEN 1 AND 168),
  coupon_valid_days integer NOT NULL DEFAULT 30 CHECK (coupon_valid_days BETWEEN 1 AND 365),
  reward_delay_hours integer NOT NULL DEFAULT 72 CHECK (reward_delay_hours BETWEEN 0 AND 720),
  inviter_monthly_limit integer NOT NULL DEFAULT 10 CHECK (inviter_monthly_limit BETWEEN 0 AND 1000),
  rules_text text NOT NULL CHECK (trim(rules_text) <> '' AND char_length(rules_text) <= 2000),
  created_by_admin_id uuid REFERENCES users(id),
  updated_by_admin_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CONSTRAINT ck_promotion_reward_campaign_period CHECK (ends_at IS NULL OR ends_at > starts_at)
);

INSERT INTO promotion_reward_campaigns (
  code,
  program_enabled,
  welcome_enabled,
  referral_enabled,
  starts_at,
  promotion_duration_hours,
  coupon_valid_days,
  reward_delay_hours,
  inviter_monthly_limit,
  rules_text
)
VALUES (
  'api_service_referral_v1',
  false,
  false,
  false,
  TIMESTAMPTZ '2026-08-02 00:00:00+08',
  24,
  30,
  72,
  10,
  '邀请好友完成首次有效 API 服务发布后，双方各得一张推广券；推广券用于进入平台推广轮换池，不承诺固定排名或曝光。'
);

CREATE TABLE referral_codes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES promotion_reward_campaigns(id),
  user_id uuid NOT NULL REFERENCES users(id),
  code text NOT NULL CHECK (code ~ '^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{8}$'),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  UNIQUE(campaign_id, user_id),
  UNIQUE(code)
);

CREATE INDEX ix_referral_codes_user_status
ON referral_codes(user_id, status);

CREATE TABLE referral_relations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES promotion_reward_campaigns(id),
  referral_code_id uuid NOT NULL REFERENCES referral_codes(id),
  inviter_user_id uuid NOT NULL REFERENCES users(id),
  invitee_user_id uuid NOT NULL REFERENCES users(id),
  status text NOT NULL DEFAULT 'bound' CHECK (status IN ('bound', 'qualified', 'rewarded', 'rejected', 'revoked')),
  bound_at timestamptz NOT NULL,
  qualified_at timestamptz,
  rewarded_at timestamptz,
  qualified_api_service_id uuid REFERENCES api_services(id),
  rejected_at timestamptz,
  rejected_reason text NOT NULL DEFAULT '',
  revoked_at timestamptz,
  revoked_reason text NOT NULL DEFAULT '',
  risk_flags text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CONSTRAINT ck_referral_relation_distinct_users CHECK (inviter_user_id <> invitee_user_id),
  CONSTRAINT ck_referral_relation_qualification CHECK (
    (qualified_at IS NULL AND qualified_api_service_id IS NULL)
    OR (qualified_at IS NOT NULL AND qualified_api_service_id IS NOT NULL)
  ),
  CONSTRAINT ck_referral_relation_rejection CHECK (
    (status <> 'rejected' AND rejected_at IS NULL AND rejected_reason = '')
    OR (status = 'rejected' AND rejected_at IS NOT NULL AND trim(rejected_reason) <> '' AND char_length(rejected_reason) <= 500)
  ),
  CONSTRAINT ck_referral_relation_revocation CHECK (
    (status <> 'revoked' AND revoked_at IS NULL AND revoked_reason = '')
    OR (status = 'revoked' AND revoked_at IS NOT NULL AND trim(revoked_reason) <> '' AND char_length(revoked_reason) <= 500)
  ),
  UNIQUE(invitee_user_id)
);

CREATE INDEX ix_referral_relations_inviter_bound
ON referral_relations(inviter_user_id, bound_at DESC, id DESC);

CREATE INDEX ix_referral_relations_campaign_status
ON referral_relations(campaign_id, status, bound_at DESC, id DESC);

CREATE TABLE promotion_coupons (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid REFERENCES promotion_reward_campaigns(id),
  user_id uuid NOT NULL REFERENCES users(id),
  source_type text NOT NULL CHECK (source_type IN (
    'welcome_first_api_service',
    'referral_inviter',
    'referral_invitee',
    'admin_grant'
  )),
  source_id uuid,
  status text NOT NULL CHECK (status IN ('pending', 'available', 'used', 'expired', 'revoked')),
  available_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  duration_hours integer NOT NULL CHECK (duration_hours BETWEEN 1 AND 168),
  used_api_service_id uuid REFERENCES api_services(id),
  activation_id uuid UNIQUE,
  promotion_starts_at timestamptz,
  promotion_ends_at timestamptz,
  used_at timestamptz,
  revoked_at timestamptz,
  revoked_reason text NOT NULL DEFAULT '',
  revoked_by_admin_id uuid REFERENCES users(id),
  created_by_admin_id uuid REFERENCES users(id),
  grant_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  CONSTRAINT ck_promotion_coupon_availability CHECK (expires_at > available_at),
  CONSTRAINT ck_promotion_coupon_source CHECK (
    (source_type = 'welcome_first_api_service' AND campaign_id IS NOT NULL AND source_id IS NOT NULL)
    OR (source_type IN ('referral_inviter', 'referral_invitee') AND campaign_id IS NOT NULL AND source_id IS NOT NULL)
    OR (source_type = 'admin_grant' AND campaign_id IS NULL AND source_id IS NULL AND created_by_admin_id IS NOT NULL AND trim(grant_reason) <> '' AND char_length(grant_reason) <= 500)
  ),
  CONSTRAINT ck_promotion_coupon_usage_facts CHECK (
    (used_api_service_id IS NULL AND activation_id IS NULL AND promotion_starts_at IS NULL AND promotion_ends_at IS NULL AND used_at IS NULL)
    OR (
      used_api_service_id IS NOT NULL
      AND activation_id IS NOT NULL
      AND promotion_starts_at IS NOT NULL
      AND promotion_ends_at IS NOT NULL
      AND (
        promotion_ends_at > promotion_starts_at
        OR (status = 'revoked' AND promotion_ends_at = promotion_starts_at)
      )
      AND used_at IS NOT NULL
    )
  ),
  CONSTRAINT ck_promotion_coupon_status_usage CHECK (
    (status = 'used' AND used_at IS NOT NULL)
    OR (status IN ('pending', 'available', 'expired') AND used_at IS NULL)
    OR status = 'revoked'
  ),
  CONSTRAINT ck_promotion_coupon_revocation CHECK (
    (status <> 'revoked' AND revoked_at IS NULL AND revoked_reason = '' AND revoked_by_admin_id IS NULL)
    OR (status = 'revoked' AND revoked_at IS NOT NULL AND trim(revoked_reason) <> '' AND char_length(revoked_reason) <= 500)
  )
);

CREATE UNIQUE INDEX ux_promotion_coupons_welcome_user
ON promotion_coupons(user_id)
WHERE source_type = 'welcome_first_api_service';

CREATE UNIQUE INDEX ux_promotion_coupons_referral_source
ON promotion_coupons(user_id, source_type, source_id)
WHERE source_type IN ('referral_inviter', 'referral_invitee');

CREATE INDEX ix_promotion_coupons_user_status
ON promotion_coupons(user_id, status, available_at DESC, id DESC);

CREATE INDEX ix_promotion_coupons_campaign_source_created
ON promotion_coupons(campaign_id, source_type, created_at DESC, id DESC);

CREATE INDEX ix_promotion_coupons_reward_activation
ON promotion_coupons(promotion_starts_at, promotion_ends_at, used_api_service_id)
WHERE activation_id IS NOT NULL AND status = 'used';
