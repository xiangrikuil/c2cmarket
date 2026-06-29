export type ProductCategoryKey = 'all' | 'gpt' | 'claude' | 'cursor' | 'gemini' | 'perplexity' | 'other'

export type ConcreteProductCategoryKey = Exclude<ProductCategoryKey, 'all'>

export type ProductCategoryOption = {
  key: ProductCategoryKey
  label: string
}

export type ProductPlanOption = {
  slug: string
  label: string
  category: ConcreteProductCategoryKey
  publishPolicy: 'allowed' | 'info_only' | 'blocked'
  accessMode: 'personal_account_cost_share' | 'provider_member_invitation' | 'owner_managed_access' | 'other_off_platform'
  providerPolicyStatus: 'known_restricted' | 'possibly_restricted' | 'unknown'
  riskLevel: 'normal' | 'elevated' | 'high'
  riskAckRequired: boolean
  policyVersion: number
  riskNoticeCode?: string
  note: string
}

export const allProductPlanValue = 'all'

export const productCategoryOptions: ProductCategoryOption[] = [
  { key: 'all', label: '全部' },
  { key: 'gpt', label: 'GPT' },
  { key: 'claude', label: 'Claude' },
  { key: 'cursor', label: 'Cursor' },
  { key: 'gemini', label: 'Gemini' },
  { key: 'perplexity', label: 'Perplexity' },
  { key: 'other', label: '其他' },
]

export const productPlanOptions: ProductPlanOption[] = [
  { slug: 'chatgpt-business', label: 'ChatGPT Business', category: 'gpt', publishPolicy: 'allowed', accessMode: 'provider_member_invitation', providerPolicyStatus: 'possibly_restricted', riskLevel: 'elevated', riskAckRequired: true, policyVersion: 1, riskNoticeCode: 'openai_subscription_carpool', note: '成员邀请或 workspace 席位，需确认风险' },
  { slug: 'chatgpt-plus', label: 'ChatGPT Plus', category: 'gpt', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, riskNoticeCode: 'openai_subscription_carpool', note: '个人订阅费用分摊，高风险需确认' },
  { slug: 'chatgpt-pro-5x-web', label: 'ChatGPT Pro 5x Web', category: 'gpt', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, riskNoticeCode: 'openai_subscription_carpool', note: '个人订阅费用分摊，高风险需确认' },
  { slug: 'chatgpt-pro-20x-web', label: 'ChatGPT Pro 20x Web', category: 'gpt', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, riskNoticeCode: 'openai_subscription_carpool', note: '个人订阅费用分摊，高风险需确认' },
  { slug: 'claude-max-5x', label: 'Claude Max 5x', category: 'claude', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, note: '需说明成员、席位或站外访问安排' },
  { slug: 'claude-pro', label: 'Claude Pro', category: 'claude', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, note: '需说明成员、席位或站外访问安排' },
  { slug: 'cursor-pro', label: 'Cursor Pro', category: 'cursor', publishPolicy: 'allowed', accessMode: 'provider_member_invitation', providerPolicyStatus: 'unknown', riskLevel: 'normal', riskAckRequired: false, policyVersion: 1, note: '团队席位或独立座位' },
  { slug: 'gemini-advanced', label: 'Gemini Advanced', category: 'gemini', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, note: '需说明成员、席位或站外访问安排' },
  { slug: 'perplexity-pro', label: 'Perplexity Pro', category: 'perplexity', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, note: '需说明成员、席位或站外访问安排' },
  { slug: 'notion-ai-plus', label: 'Notion AI Plus', category: 'other', publishPolicy: 'allowed', accessMode: 'other_off_platform', providerPolicyStatus: 'unknown', riskLevel: 'normal', riskAckRequired: false, policyVersion: 1, note: '其他订阅品类' },
  { slug: 'poe-subscription', label: 'Poe 订阅', category: 'other', publishPolicy: 'allowed', accessMode: 'other_off_platform', providerPolicyStatus: 'unknown', riskLevel: 'normal', riskAckRequired: false, policyVersion: 1, note: '其他订阅品类' },
]

export function normalizeProductCategory(value: unknown): ProductCategoryKey {
  const raw = typeof value === 'string' ? value : ''
  return productCategoryOptions.some(item => item.key === raw) ? raw as ProductCategoryKey : 'all'
}

export function getProductCategoryLabel(category: ProductCategoryKey) {
  return productCategoryOptions.find(item => item.key === category)?.label ?? '全部'
}

export function getProductCategory(productName: string): ConcreteProductCategoryKey {
  const name = productName.toLowerCase()
  if (name.includes('chatgpt') || name.includes('openai') || /\bgpt\b/.test(name)) return 'gpt'
  if (name.includes('claude') || name.includes('anthropic')) return 'claude'
  if (name.includes('cursor')) return 'cursor'
  if (name.includes('gemini') || name.includes('google ai')) return 'gemini'
  if (name.includes('perplexity')) return 'perplexity'
  return 'other'
}

export function productMatchesCategory(productName: string, category: ProductCategoryKey) {
  return category === 'all' || getProductCategory(productName) === category
}

export function getProductPlanBySlug(slug: string) {
  return productPlanOptions.find(item => item.slug === slug)
}

export function getProductPlanForName(productName: string) {
  const normalizedName = productName.toLowerCase()
  return productPlanOptions.find(item => normalizedName === item.label.toLowerCase())
    ?? productPlanOptions.find(item => normalizedName.includes(item.label.toLowerCase()) || item.label.toLowerCase().includes(normalizedName))
}

export function getProductPlanOptions(category: ProductCategoryKey) {
  if (category === 'all') return []
  return productPlanOptions.filter(item => item.category === category)
}

export function normalizeProductPlan(category: ProductCategoryKey, value: unknown) {
  const raw = typeof value === 'string' ? value : ''
  if (raw === allProductPlanValue) return allProductPlanValue
  return getProductPlanOptions(category).some(item => item.slug === raw) ? raw : allProductPlanValue
}

export function productMatchesPlan(productName: string, planSlug: string) {
  if (planSlug === allProductPlanValue) return true
  const plan = getProductPlanBySlug(planSlug)
  if (!plan) return true
  const normalizedName = productName.toLowerCase()
  const normalizedLabel = plan.label.toLowerCase()
  return normalizedName === normalizedLabel || normalizedName.includes(normalizedLabel) || normalizedLabel.includes(normalizedName)
}

export function isHighRiskGptCarpoolPlan(productName: string) {
  const plan = getProductPlanForName(productName)
  return plan?.category === 'gpt' && plan.riskLevel === 'high'
}

export function canPublishProductPlan(plan: ProductPlanOption | null | undefined) {
  return plan?.publishPolicy === 'allowed'
}
