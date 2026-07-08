export type ConfidenceLevel = 'high' | 'medium' | 'low'

export type OpeningChannelCode =
  | 'web'
  | 'ios_app_store'
  | 'google_play'
  | 'team_seat'
  | 'other'

export type PaymentMethodCode =
  | 'credit_card'
  | 'virtual_card'
  | 'apple_pay'
  | 'google_pay'
  | 'app_store_gift_card'
  | 'google_play_gift_card'
  | 'paypal'
  | 'other'

export type CarpoolWarrantyMode =
  | 'no_warranty'
  | 'remaining_days_compensation'
  | 'fixed_days_warranty'

export type CatalogProviderCode = 'openai' | 'anthropic' | 'other'
export type CatalogCategoryCode = 'gpt' | 'claude' | 'cursor' | 'gemini' | 'perplexity' | 'other'
export type ProductPublishPolicy = 'allowed' | 'info_only' | 'blocked'
export type ProductAccessMode = 'personal_account_cost_share' | 'provider_member_invitation' | 'owner_managed_access' | 'other_off_platform'
export type ProviderPolicyStatus = 'known_restricted' | 'possibly_restricted' | 'unknown'
export type ProductRiskLevel = 'normal' | 'elevated' | 'high'
export type ProductQuotaPeriod = 'monthly'

export type CarpoolProductCatalogItem = {
  id: string
  categoryCode: CatalogCategoryCode
  providerCode: CatalogProviderCode
  displayName: string
  slug: string
  description: string | null
  publishPolicy: ProductPublishPolicy
  accessMode: ProductAccessMode
  providerPolicyStatus: ProviderPolicyStatus
  riskLevel: ProductRiskLevel
  riskAckRequired: boolean
  policyVersion: number
  policyNote: string
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: ProductQuotaPeriod
  riskNoticeCode?: string
  active: boolean
  sortOrder: number
  allowCustomVariant: boolean
  createdAt: string
  updatedAt: string
}

export type RegionOption = {
  code: string
  displayName: string
  active: boolean
  sortOrder: number
}

export type OpeningChannelOption = {
  code: OpeningChannelCode
  displayName: string
  active: boolean
  sortOrder: number
}

export type PaymentMethodOption = {
  code: PaymentMethodCode
  displayName: string
  active: boolean
  sortOrder: number
}

export type CarpoolWarrantyConfig = {
  mode: CarpoolWarrantyMode
  fixedWarrantyDays: number | null
  compensationMethod: string | null
  exclusions: string | null
}

export type AccessArrangementMode =
  | 'personal_account_cost_share'
  | 'provider_member_invitation'
  | 'owner_managed_access'
  | 'other_off_platform'
  | 'not_allowed'

export type CarpoolPublishForm = {
  linuxDoTopicUrl: string
  parsedTopicId: string | null
  productId: string
  customProductName: string | null
  regionCode: string
  monthlyPriceCny: number | null
  serviceMultiplier: number | null
  monthlyQuotaAmount: number | null
  totalSeats: number
  occupiedSeats: number
  openingChannelCode: OpeningChannelCode | ''
  paymentMethodCodes: PaymentMethodCode[]
  accessArrangementMode: AccessArrangementMode
  accessArrangementNote: string
  riskAcknowledged: boolean
  policyVersion: number | null
  riskNoticeCode: string | null
  warranty: CarpoolWarrantyConfig
  rulesNote: string
}

export type ParsedLinuxDoTopic = {
  topicId: string
  topicUrl: string
  title: string
  authorUsername: string
  authorUserId: string | null
  createdAt: string
  updatedAt: string
  authorMatchesBoundUser: boolean
  detected: {
    productId: string | null
    productText: string | null
    regionCode: string | null
    regionText: string | null
    monthlyPriceCny: number | null
    totalSeats: number | null
    availableSeats: number | null
    occupiedSeats: number | null
    openingChannelId: OpeningChannelCode | null
    paymentMethodIds: PaymentMethodCode[]
    warrantyMode: CarpoolWarrantyMode | null
  }
  confidence: {
    product: ConfidenceLevel | null
    region: ConfidenceLevel | null
    monthlyPrice: ConfidenceLevel | null
    seats: ConfidenceLevel | null
  }
}

export type CompletenessStatus = 'done' | 'pending' | 'conflict'

export type CompletenessItem = {
  label: string
  status: CompletenessStatus
}

export type TrustItem = {
  label: string
  status: 'done' | 'pending'
  description?: string
}

export type PublishTaskKey =
  | 'product'
  | 'region'
  | 'monthlyPrice'
  | 'monthlyQuota'
  | 'openingChannel'
  | 'paymentMethods'
  | 'rulesNote'
  | 'linuxDoImport'

export type PublishSectionKey =
  | 'basic'
  | 'seats'
  | 'activationPayment'
  | 'rules'
  | 'tools'

export type PublishFieldState = 'idle' | 'pendingRequired' | 'error' | 'complete' | 'defaulted'

export type PublishTask = {
  key: PublishTaskKey
  label: string
  shortLabel: string
  section: PublishSectionKey
  fieldId: string
  description: string
  complete: boolean
  error?: string
}

export type PublishDefaultItem = {
  key: string
  label: string
  description: string
  status: PublishFieldState
}
