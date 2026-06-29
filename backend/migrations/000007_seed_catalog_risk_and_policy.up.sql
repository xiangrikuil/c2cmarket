-- Initial catalog, risk notice, and publish policy seed data.
-- 日期：2026-06-21
-- 执行者：Codex

INSERT INTO product_categories (id, code, display_name, sort_order, active) VALUES
  ('00000000-0000-0000-0000-000000000101', 'gpt', 'GPT', 10, true),
  ('00000000-0000-0000-0000-000000000102', 'claude', 'Claude', 20, true),
  ('00000000-0000-0000-0000-000000000103', 'cursor', 'Cursor', 30, true),
  ('00000000-0000-0000-0000-000000000104', 'gemini', 'Gemini', 40, true),
  ('00000000-0000-0000-0000-000000000105', 'perplexity', 'Perplexity', 50, true),
  ('00000000-0000-0000-0000-000000000199', 'other', '其他', 999, true);

INSERT INTO risk_notices (id, code, created_at) VALUES
  ('00000000-0000-0000-0000-000000000201', 'openai_subscription_carpool', now());

INSERT INTO risk_notice_versions (id, risk_notice_id, version, title, body_markdown, effective_at) VALUES
  (
    '00000000-0000-0000-0000-000000000202',
    '00000000-0000-0000-0000-000000000201',
    1,
    'OpenAI 订阅拼车风险告知',
    '该品类由 C2CMarket 当前开放，不代表服务提供商认可。个人订阅费用分摊可能受到服务提供商账号、成员、访问权限或使用规则限制，并可能造成账号限制、成员移除、工作区停用、封号、额度耗尽、数据暴露、费用损失或服务中断。平台不保存、不交付任何密码、API Key、Token、Cookie、Session、验证码、恢复码或面板主账号凭据。',
    now()
  );

INSERT INTO product_plans (
  id,
  category_id,
  provider_code,
  slug,
  display_name,
  description,
  publish_policy,
  access_mode,
  provider_policy_status,
  risk_level,
  risk_ack_required,
  risk_notice_code,
  policy_version,
  policy_note,
  active,
  allow_custom_variant,
  sort_order,
  policy_updated_at
) VALUES
  ('00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000101', 'openai', 'chatgpt-plus', 'ChatGPT Plus', '个人订阅费用分摊，高风险需确认。', 'allowed', 'personal_account_cost_share', 'known_restricted', 'high', true, 'openai_subscription_carpool', 1, 'C2CMarket 当前开放该品类，不代表服务提供商认可。', true, false, 10, now()),
  ('00000000-0000-0000-0000-000000000302', '00000000-0000-0000-0000-000000000101', 'openai', 'chatgpt-pro-5x-web', 'ChatGPT Pro 5x Web', '个人订阅费用分摊，高风险需确认。', 'allowed', 'personal_account_cost_share', 'known_restricted', 'high', true, 'openai_subscription_carpool', 1, 'C2CMarket 当前开放该品类，不代表服务提供商认可。', true, false, 20, now()),
  ('00000000-0000-0000-0000-000000000303', '00000000-0000-0000-0000-000000000101', 'openai', 'chatgpt-pro-20x-web', 'ChatGPT Pro 20x Web', '个人订阅费用分摊，高风险需确认。', 'allowed', 'personal_account_cost_share', 'known_restricted', 'high', true, 'openai_subscription_carpool', 1, 'C2CMarket 当前开放该品类，不代表服务提供商认可。', true, false, 30, now()),
  ('00000000-0000-0000-0000-000000000304', '00000000-0000-0000-0000-000000000101', 'openai', 'chatgpt-business', 'ChatGPT Business', 'OpenAI Business workspace 成员邀请，需确认风险。', 'allowed', 'provider_member_invitation', 'possibly_restricted', 'elevated', true, 'openai_subscription_carpool', 1, 'Business 按现有独立配置执行。', true, false, 40, now()),
  ('00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000102', 'anthropic', 'claude-pro', 'Claude Pro', '社区 Claude Pro 拼车品类。', 'allowed', 'owner_managed_access', 'unknown', 'elevated', false, null, 1, '需说明成员、席位或站外访问安排。', true, false, 50, now()),
  ('00000000-0000-0000-0000-000000000402', '00000000-0000-0000-0000-000000000102', 'anthropic', 'claude-pro-5x', 'Claude Pro 5x', '社区 Claude Pro 5x 拼车品类。', 'allowed', 'owner_managed_access', 'unknown', 'elevated', false, null, 1, '需说明成员、席位或站外访问安排。', true, false, 60, now()),
  ('00000000-0000-0000-0000-000000000403', '00000000-0000-0000-0000-000000000102', 'anthropic', 'claude-pro-20x', 'Claude Pro 20x', '社区 Claude Pro 20x 拼车品类。', 'allowed', 'owner_managed_access', 'unknown', 'elevated', false, null, 1, '需说明成员、席位或站外访问安排。', true, false, 70, now()),
  ('00000000-0000-0000-0000-000000000501', '00000000-0000-0000-0000-000000000199', 'other', 'other-custom', '其他 / 自定义', '提交后由管理员映射或新增目录项。', 'allowed', 'other_off_platform', 'unknown', 'normal', false, null, 1, '自定义产品需要管理员映射或新增目录项。', true, true, 999, now());

INSERT INTO product_plan_policy_history (
  product_plan_id,
  policy_version,
  publish_policy,
  access_mode,
  provider_policy_status,
  risk_level,
  risk_ack_required,
  risk_notice_version_id,
  enforcement_mode,
  reason,
  changed_by_admin_id,
  effective_at
)
SELECT
  plan.id,
  plan.policy_version,
  plan.publish_policy,
  plan.access_mode,
  plan.provider_policy_status,
  plan.risk_level,
  plan.risk_ack_required,
  notice_version.id,
  'new_actions_only',
  'Initial catalog policy seed.',
  null,
  plan.policy_updated_at
FROM product_plans plan
LEFT JOIN risk_notices notice ON notice.code = plan.risk_notice_code
LEFT JOIN risk_notice_versions notice_version
  ON notice_version.risk_notice_id = notice.id
  AND notice_version.retired_at IS NULL;
