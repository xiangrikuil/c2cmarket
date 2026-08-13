export type ProductCategoryCode = string

export type ProductProviderCode = string

export type CatalogStatus = 'active' | 'deprecated' | 'blocked'
export type CatalogLifecycleAction = 'deprecate' | 'block' | 'reactivate' | 'unblock'
export type CatalogCoreKey = 'gpt' | 'claude' | 'grok'

export type CatalogLifecycleFields = {
  coreKey?: CatalogCoreKey
  status: CatalogStatus
  effectiveStatus: CatalogStatus
  effectiveStatusSource: 'self' | 'parent'
  statusChangedAt: string
  statusChangedBy?: string
  statusReason?: string
  version: number
  identityLocked: boolean
  identityLockReason?: string
  active: boolean
}

export type ProductPublishPolicy = 'allowed' | 'info_only' | 'blocked'

export type ProductAccessMode =
  | 'personal_account_cost_share'
  | 'provider_member_invitation'
  | 'owner_managed_access'
  | 'other_off_platform'
  | 'unsupported'

export type ProviderPolicyStatus = 'known_restricted' | 'possibly_restricted' | 'unknown'

export type ProductRiskLevel = 'normal' | 'elevated' | 'high'
export type ProductQuotaPeriod = 'monthly'

export type ProductCategory = CatalogLifecycleFields & {
  id: string
  code: ProductCategoryCode
  displayName: string
  iconDataUrl: string
  sortOrder: number
  active: boolean
}

export type ProductCategoryInput = {
  code: string
  displayName: string
  iconDataUrl: string
  sortOrder: number
}

export type ProductPlan = CatalogLifecycleFields & {
  id: string
  categoryId: string
  categoryCode: ProductCategoryCode
  providerCode: ProductProviderCode
  slug: string
  displayName: string
  description: string
  publishPolicy: ProductPublishPolicy
  accessMode: ProductAccessMode
  providerPolicyStatus: ProviderPolicyStatus
  riskLevel: ProductRiskLevel
  riskAckRequired: boolean
  riskNoticeCode?: string
  policyVersion: number
  policyNote: string
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: ProductQuotaPeriod
  allowCustomVariant: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export type ProductPlanInput = {
  categoryId: string
  providerCode: string
  slug: string
  displayName: string
  description: string
  publishPolicy: ProductPublishPolicy
  accessMode: ProductAccessMode
  providerPolicyStatus: ProviderPolicyStatus
  riskLevel: ProductRiskLevel
  riskAckRequired: boolean
  riskNoticeCode: string
  policyNote: string
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: ProductQuotaPeriod
  allowCustomVariant: boolean
  sortOrder: number
}
