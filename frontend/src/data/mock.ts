export type OfficialPrice = {
  id: string
  product: string
  plan: string
  region: string
  channel: string
  openingMethod: string
  originalPrice: string
  cny: number | null
  status: '已验证' | '待验证' | '需复核' | '有争议' | '已过期'
  source: string
  submitter: string
  submitterTrust: number
  updatedAt: string
  isLowest?: boolean
}

export type PricingMode = 'fixed' | 'equal_share' | 'tiered'

export type PricingTier = {
  memberCount: number
  price: number
}

export type QuotaPeriod = 'monthly'

export type Carpool = {
  id: string
  product: string
  region: string
  monthly: number
  serviceMultiplier?: number
  monthlyQuotaAmount?: number
  quotaLabel?: string
  quotaUnit?: string
  quotaPeriod?: QuotaPeriod
  seats: string
  pricingMode: PricingMode
  fixedMonthlyPrice?: number
  totalShareableCost?: number
  currentConfirmedMembers: number
  maxMembers: number
  settlementDeadline?: string
  pricingTiers?: PricingTier[]
  owner: string
  trustLevel: number
  ownerType: '个人车主' | '商户车源' | '可信新车主'
  warranty: '车主承诺' | '售后协商'
  openingMethod: 'Apple Store' | '虚拟卡' | '其他' | '本地卡'
  status: '可上车' | '已满' | '候补' | '暂停' | '审核中'
  confirmedAt: string
  confirmedWithin48h: boolean
  linuxdoBound: boolean
  sourcePostAccessible: boolean
  hasInfoConflict: boolean
  hasUnresolvedDispute: boolean
  accessArrangementMode?: CarpoolAccessArrangementMode
  accessArrangementNote?: string
  riskNoticeCode?: string
  riskAcknowledged?: boolean
}

export type CarpoolAccessArrangementMode =
  | 'personal_account_cost_share'
  | 'provider_member_invitation'
  | 'owner_managed_access'
  | 'other_off_platform'
  | 'not_allowed'

export type CarpoolApplicationStatus =
  | 'pending_owner'
  | 'accepted_reserved'
  | 'waiting_contact'
  | 'contacted'
  | 'joined_pending_confirmation'
  | 'active'
  | 'pending_completion'
  | 'completed'
  | 'rejected'
  | 'cancelled_by_buyer'
  | 'cancelled_by_owner'
  | 'expired'
  | 'disputed'

export type CarpoolApplicationActorRole = 'buyer' | 'owner' | 'admin' | 'system'
export type CarpoolApplicationEventType =
  | 'application_created'
  | 'owner_accepted'
  | 'owner_rejected'
  | 'buyer_contacted'
  | 'buyer_confirmed_joined'
  | 'owner_confirmed_joined'
  | 'service_started'
  | 'pending_completion'
  | 'buyer_confirmed_completed'
  | 'owner_confirmed_completed'
  | 'completed'
  | 'cancelled'
  | 'expired'
  | 'disputed'
  | 'admin_updated'

export type CarpoolCancellationResponsibility = 'buyer' | 'owner' | 'mutual' | 'platform' | 'undetermined'

export type CarpoolSeatSummary = {
  carpoolId: string
  totalSeats: number
  activeMemberCount: number
  reservedSeatCount: number
  availableSeats: number
}

export type CarpoolApplicationSnapshot = {
  carpoolId: string
  productName: string
  regionName: string
  monthlyPriceCny: number
  serviceMultiplier?: number
  monthlyQuotaAmount?: number
  quotaLabel?: string
  quotaUnit?: string
  quotaPeriod?: QuotaPeriod
  priceLabel: string
  openingChannelName: string
  paymentMethodNames: string[]
  warrantyText: string
  rulesVersion: string
  rulesText: string
  ownerUserId: string
  ownerUsername: string
  ownerTrustLevel: number
  ownerType: Carpool['ownerType']
  sourceTopicUrl: string
  accessArrangementMode?: CarpoolAccessArrangementMode
  accessArrangementNote?: string
  riskNoticeCode?: string
  riskAcknowledged?: boolean
}

export type CarpoolApplicantStats = {
  linuxdoBound: boolean
  trustLevel: number
  completed30d: number
  buyerResponsibleCancellations: number
  ownerResponsibleCancellations: number
  unresolvedDisputes: number
}

export type CarpoolApplicationReview = {
  rating: number
  tags: string[]
  note: string
  createdAt: string
}

export type CarpoolApplication = {
  id: string
  carpoolId: string
  applicantUserId: string
  applicantUsername: string
  applicantStats: CarpoolApplicantStats
  ownerUserId: string
  ownerUsername: string
  status: CarpoolApplicationStatus
  seatsRequested: number
  snapshot: CarpoolApplicationSnapshot
  reservedUntil: string | null
  buyerContactedAt: string | null
  buyerConfirmedJoinedAt: string | null
  ownerConfirmedJoinedAt: string | null
  startedAt: string | null
  expectedEndAt: string | null
  buyerConfirmedCompletedAt: string | null
  ownerConfirmedCompletedAt: string | null
  completedAt: string | null
  completionMode: 'mutual' | 'automatic' | 'admin' | null
  cancellationReasonCode: string | null
  cancellationReasonText: string | null
  responsibility: CarpoolCancellationResponsibility | null
  disputeReason: string | null
  buyerReview?: CarpoolApplicationReview
  ownerReview?: CarpoolApplicationReview
  createdAt: string
  updatedAt: string
}

export type CarpoolApplicationEvent = {
  id: string
  applicationId: string
  actorId: string
  actorLabel: string
  actorRole: CarpoolApplicationActorRole
  type: CarpoolApplicationEventType
  fromStatus?: CarpoolApplicationStatus
  toStatus?: CarpoolApplicationStatus
  note?: string
  createdAt: string
}

export type UserAccountStatus = 'normal' | 'warning' | 'partially_restricted' | 'under_review' | 'temporarily_suspended' | 'permanently_banned'

export type AvatarMode = 'linuxdo' | 'custom_url'

export type UserBadge = {
  id: string
  code: string
  label: string
  type: 'identity' | 'trust' | 'merchant' | 'contributor' | 'system'
}

export type UserPrivacySettings = {
  showCreatedAt: boolean
  showLastActiveAt: boolean
  showCompletionStats: boolean
  showResponseMedian: boolean
  showResolvedDisputeSummary: boolean
  allowPublicProfileReport: boolean
}

export type UserProfile = {
  id: string
  username: string
  displayName: string
  bio: string | null
  avatarMode: AvatarMode
  avatarUrl: string | null
  customAvatarUrl: string | null
  email: string | null
  emailVerified: boolean
  emailVerifiedAt: string | null
  passwordConfigured: boolean
  regionCode: string | null
  timezone: string | null
  linuxDoBinding: {
    bound: boolean
    linuxDoUserId: string | null
    linuxDoUsername: string | null
    linuxDoAvatarUrl: string | null
    trustLevel: number | null
    lastSyncedAt: string | null
  }
  badges: UserBadge[]
  accountStatus: UserAccountStatus
  permissions: Array<'admin'>
  restrictions: string[]
  usernameChangePolicy: {
    canChange: boolean
    nextAvailableAt: string | null
  }
  privacy: UserPrivacySettings
  createdAt: string
  lastActiveAt: string
}

export type PublicUserProfile = {
  id: string
  username: string
  displayName: string
  bio: string | null
  avatarUrl: string | null
  avatarText: string
  linuxDoBound: boolean
  linuxDoUsername: string | null
  trustLevel: number | null
  badges: UserBadge[]
  accountStatus: UserAccountStatus
  createdAt: string | null
  lastActiveAt: string | null
  stats: {
    completedCarpoolsLast30Days: number | null
    completedApiOrdersLast30Days: number | null
    responseMedianMinutes: number | null
    buyerResponsibilityCancellationCount: number
    sellerResponsibilityCancellationCount: number
    unresolvedDisputeCount: number
    resolvedDisputeCountLast90Days: number | null
  }
  privacy: UserPrivacySettings
}

export type ContactMethodType = 'linuxdo' | 'wechat' | 'email' | 'telegram' | 'other'
export type ContactUsageScope = 'carpool_owner' | 'api_merchant' | 'buyer' | 'dispute'

export type UserContactMethod = {
  id: string
  userId: string
  type: ContactMethodType
  label: string
  maskedValue: string
  displayValue: string
  usageScopes: ContactUsageScope[]
  isDefault: boolean
  enabled: boolean
  verified: boolean
  createdAt: string
  updatedAt: string
}

export type OrderContactSnapshotItem = {
  type: ContactMethodType
  label: string
  maskedValue: string
  displayValue?: string
  verified: boolean
  usageScope: ContactUsageScope
  actionUrl?: string
}

export type OrderContactSnapshot = {
  id: string
  orderType: 'carpool_application' | 'api_order'
  orderId: string
  sellerContacts: OrderContactSnapshotItem[]
  buyerContacts: OrderContactSnapshotItem[]
  contactWindowEndsAt: string | null
  canView: boolean
  unavailableReason: string | null
  createdAt: string
}

export type ContactReportReasonCode = 'invalid' | 'unreachable' | 'impersonation' | 'other'

export type CreateContactReportRequest = {
  orderType: OrderContactSnapshot['orderType']
  orderId: string
  contactType: ContactMethodType
  reasonCode: ContactReportReasonCode
  note: string
}

export type AdminUserRiskProfile = {
  id: string
  username: string
  linuxdoBound: boolean
  trustLevel: number
  identity: '普通用户' | '个人车主' | 'API 商户' | '可信新车主'
  accountStatus: UserAccountStatus
  carpoolCompletions: number
  apiCompletions: number
  buyerResponsibleCancellations: number
  ownerResponsibleCancellations: number
  unresolvedDisputes: number
  restrictions: string[]
  lastActiveAt: string
}

export type AdminAuditLog = {
  id: string
  actorType: 'admin' | 'system'
  actorLabel: string
  action: string
  targetType: string
  targetId: string
  targetLabel: string
  beforeStatus: string | null
  afterStatus: string | null
  reason: string | null
  createdAt: string
}

export type ConfidenceLevel = 'high' | 'medium' | 'low'
export type OpeningChannelCode = 'web' | 'ios_app_store' | 'google_play' | 'team_seat' | 'other'
export type PaymentMethodCode = 'credit_card' | 'virtual_card' | 'apple_pay' | 'google_pay' | 'gift_card' | 'local_payment' | 'other'
export type CarpoolWarrantyMode = 'no_warranty' | 'remaining_days_compensation' | 'fixed_days_warranty'
export type ProductPublishPolicy = 'allowed' | 'info_only' | 'blocked'
export type ProductAccessMode = 'personal_account_cost_share' | 'provider_member_invitation' | 'owner_managed_access' | 'other_off_platform'
export type ProviderPolicyStatus = 'known_restricted' | 'possibly_restricted' | 'unknown'
export type ProductRiskLevel = 'normal' | 'elevated' | 'high'
export type ProductQuotaPeriod = 'monthly'

export type CarpoolProductCatalogItem = {
  id: string
  categoryCode: 'gpt' | 'claude' | 'cursor' | 'gemini' | 'perplexity' | 'other'
  providerCode: 'openai' | 'anthropic' | 'other'
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

export type TransactionTrendPoint = {
  date: string
  medianPrice: number
  p25Price: number
  p75Price: number
  transactionCount: number
}

export type TransactionRecord = {
  id: string
  productSlug: string
  product: string
  sourceType: '拼车成交' | 'API 意向' | '官方订阅'
  trustLevel: number
  finalSettlementPrice: number
  regionNote: string
  completedAt: string
  status: 'completed' | 'pending' | 'cancelled' | 'refunded'
  hasUnresolvedDispute: boolean
}

export type ProductTrend = {
  slug: string
  label: string
  officialVerifiedLow: number
  officialRegion: string
  officialSource: string
  verifiedAt: string
  points: Record<'7d' | '30d' | '90d', TransactionTrendPoint[]>
}

export type ApiDeliveryMode = 'api_key_endpoint' | 'sub2api_panel_account'
export type ApiUsageVisibility = 'none' | 'merchant_readonly' | 'panel_realtime'
export type ApiGateway = 'Sub2API' | 'NewAPI Proxy' | '自建中转' | '固定套餐' | '商户手工核对' | '其他'
export type ApiBillingMode = 'metered_credit' | 'manual_credit' | 'fixed_package'
export type ApiVisibilityRule = 'public' | 'after_intent' | 'off_platform'
export type ApiServiceState = 'online' | 'offline' | 'reviewing' | 'paused'
export type ApiMerchantIdentityMode = 'public_profile' | 'store_alias'
export type ApiPurchaseIntentStatus =
  | 'open'
  | 'contacted'
  | 'buyer_cancelled'
  | 'owner_closed'
export type ApiActorRole = 'buyer' | 'merchant' | 'admin' | 'system'
export type ApiPurchaseIntentEventType =
  | 'intent_created'
  | 'contacted'
  | 'buyer_cancelled'
  | 'owner_closed'

export type ApiModelMultiplier = {
  model: string
  multiplier: string
}

export type ModelPriceRow = {
  modelId: string
  modelName: string
  provider: string
  officialInputPricePerMillion: number
  officialCachedInputPricePerMillion: number | null
  officialOutputPricePerMillion: number
  merchantMultiplier: number
  actualInputPricePerMillion: number
  actualCachedInputPricePerMillion: number | null
  actualOutputPricePerMillion: number
}

export type ModelCapability = 'chat' | 'vision' | 'image_generation' | 'image_edit' | 'reasoning'

export type ModelCatalogItem = {
  id: string
  provider: 'openai' | 'anthropic' | 'other'
  name: string
  displayName: string
  capabilities: ModelCapability[]
  officialInputPricePerMillion: number | null
  officialCachedInputPricePerMillion: number | null
  officialOutputPricePerMillion: number | null
  active: boolean
}

export type ApiContactChannel = {
  type: ContactMethodType
  label: string
  value: string
}

export type ApiService = {
  id: string
  title: string
  merchantId: string
  merchantUsername: string
  merchant: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
  trustLevel: number
  merchantType: '个人车主' | '商户' | '可信新车主'
  models: string[]
  modelMultipliers: ApiModelMultiplier[]
  rate: string
  defaultMultiplier: number
  creditPerCny: number
  minimumPurchaseCny: number
  maxBuy: number
  balance: number
  delivery: ApiGateway
  billingMode: ApiBillingMode
  deliveryModes: ApiDeliveryMode[]
  usageVisibility: ApiUsageVisibility
  panelBaseUrl: string | null
  imagePricing: {
    supported: boolean
    textToImage: boolean
    imageToImage: boolean
    oneKPriceUsd: number | null
    twoKPriceUsd: number | null
    fourKPriceUsd: number | null
  }
  independentApiKey: boolean
  independentPanelAccount: boolean
  panelRequiresPasswordReset: boolean
  apiBaseUrlVisibility: ApiVisibilityRule
  panelLoginUrlVisibility: ApiVisibilityRule
  publicApiBaseUrl?: string
  publicPanelLoginUrl?: string
  state: ApiServiceState
  online: boolean
  publiclyOrderable: boolean
  lastOnlineConfirmedAt: string
  onlineExpiresAt: string
  expectedResponseMinutes: number
  responseMedianMinutes: number
  dailyOrderLimit: number
  todayOrderCount: number
  unresolvedDisputes: number
  warning?: string
  warranty: string
  refundPolicy: string
  expiresAt: string
  completed30d: number
  reviewCount: number
  officialPricingVersion: string
  officialPricingUpdatedAt: string
  merchantNote: string
  modelPriceRows: ModelPriceRow[]
  contactChannels: ApiContactChannel[]
}

export type PublicMerchantProfile = {
  username: string
  displayName: string
  avatarText: string
  merchantId: string
  identity: '个人商户' | '可信新商户' | 'API 商户'
  trustLevel: number
  linuxdoBound: boolean
  originalPostBound: boolean
  joinedAt: string
  lastActiveAt: string
  linuxdoUrl: string
  completed30d: number
  responseMedianMinutes: number
  merchantResponsibleCancellations: number
  unresolvedDisputes: number
  handledDisputes90d: number
}

export type PublicCompletionRecord = {
  id: string
  username: string
  date: string
  serviceType: string
  deliveryMode: ApiDeliveryMode
  amountRange: string
  status: '平台确认完成'
}

export type PublicReviewRecord = {
  id: string
  username: string
  date: string
  serviceType: string
  rating?: number
  tags: string[]
  note: string
  verified: boolean
}

export type PublicDisputeRecord = {
  id: string
  username: string
  type: string
  result: string
  handledAt: string
  unresolved: boolean
}

export type ApiPurchaseIntentSnapshot = {
  serviceId: string
  serviceTitle: string
  merchantId: string
  merchant: string
  merchantUsername: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
  trustLevel: number
  merchantType: ApiService['merchantType']
  models: string[]
  multiplier: string
  defaultMultiplier: number
  creditPerCny: number
  warranty: string
  refundPolicy: string
  usageVisibility: ApiUsageVisibility
  supportedDeliveryModes: ApiDeliveryMode[]
  selectedDeliveryMode: ApiDeliveryMode
  minimumPurchaseCny: number
  panelBaseUrl: string | null
  apiBaseUrlVisibility: ApiVisibilityRule
  panelLoginUrlVisibility: ApiVisibilityRule
  panelRequiresPasswordReset: boolean
  expiresAt: string
  officialPricingVersion: string
  officialPricingUpdatedAt: string
  modelPrices: ModelPriceRow[]
  paymentOptions?: ApiIntentPaymentOption[]
}

export type ApiIntentPaymentOption = {
  paymentMethod: 'wechat' | 'alipay' | 'usdt'
  enabled: boolean
  paymentInstructions: string
  paymentQrCodeDataUrl: string | null
}

export type ApiCredentialHandoffRecord = {
  intentId: string
  selectedDeliveryMode: ApiDeliveryMode
  offPlatformContactChannel?: string
  status: 'not_started' | 'contacted' | 'closed'
  requiresFirstLoginPasswordReset: boolean
  note?: string
}

export type ApiPurchaseIntent = {
  id: string
  serviceId: string
  version?: number
  buyerId: string
  buyer: string
  merchantId: string
  merchant: string
  status: ApiPurchaseIntentStatus
  selectedDeliveryMode: ApiDeliveryMode
  purchaseAmountCny: number
  purchasedCredit: number
  targetModel: string
  buyerNote?: string
  snapshot: ApiPurchaseIntentSnapshot
  handoff: ApiCredentialHandoffRecord
  contactChannels: ApiContactChannel[]
  merchantResponseDeadline?: string
  createdAt: string
  updatedAt: string
  buyerCancelledAt?: string
  buyerCancelReason?: string
  ownerClosedAt?: string
  ownerCloseReason?: string
}

export type ApiPurchaseIntentEvent = {
  id: string
  intentId: string
  actorId: string
  actorLabel: string
  actorRole: ApiActorRole
  type: ApiPurchaseIntentEventType
  fromStatus?: ApiPurchaseIntentStatus
  toStatus?: ApiPurchaseIntentStatus
  metadata?: Record<string, string | number | boolean>
  createdAt: string
}

export const categoryRows = [
  { product: 'ChatGPT Plus', detail: '个人订阅费用分摊 / 高风险需确认', verifiedLowest: 108, leadLowest: 18, carpoolCount: 6, demandCount: 88 },
  { product: 'ChatGPT Business', detail: 'workspace 成员邀请 / 风险需确认', verifiedLowest: 188, leadLowest: 92, carpoolCount: 18, demandCount: 35 },
  { product: 'ChatGPT Pro 20x Web', detail: '个人订阅费用分摊 / 高风险需确认', verifiedLowest: 988, leadLowest: 56, carpoolCount: 5, demandCount: 72 },
  { product: 'Claude Max 5x', detail: '5x / 20x 订阅', verifiedLowest: 724, leadLowest: 47, carpoolCount: 14, demandCount: 29 },
  { product: 'Cursor Pro', detail: '团队席位 / 独立座位', verifiedLowest: 154, leadLowest: 36, carpoolCount: 18, demandCount: 24 },
]

export const carpoolProductCatalog: CarpoolProductCatalogItem[] = [
  { id: 'chatgpt-plus', categoryCode: 'gpt', providerCode: 'openai', displayName: 'ChatGPT Plus', slug: 'chatgpt-plus', description: '个人订阅费用分摊，高风险需确认', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, policyNote: 'C2CMarket 当前开放该品类，不代表服务提供商认可。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', riskNoticeCode: 'openai_subscription_carpool', active: true, sortOrder: 10, allowCustomVariant: false, createdAt: '2026-06-21', updatedAt: '2026-06-21' },
  { id: 'chatgpt-pro-5x-web', categoryCode: 'gpt', providerCode: 'openai', displayName: 'ChatGPT Pro 5x Web', slug: 'chatgpt-pro-5x-web', description: '个人订阅费用分摊，高风险需确认', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, policyNote: 'C2CMarket 当前开放该品类，不代表服务提供商认可。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', riskNoticeCode: 'openai_subscription_carpool', active: true, sortOrder: 20, allowCustomVariant: false, createdAt: '2026-06-21', updatedAt: '2026-06-21' },
  { id: 'chatgpt-pro-20x-web', categoryCode: 'gpt', providerCode: 'openai', displayName: 'ChatGPT Pro 20x Web', slug: 'chatgpt-pro-20x-web', description: '个人订阅费用分摊，高风险需确认', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, policyNote: 'C2CMarket 当前开放该品类，不代表服务提供商认可。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', riskNoticeCode: 'openai_subscription_carpool', active: true, sortOrder: 30, allowCustomVariant: false, createdAt: '2026-06-21', updatedAt: '2026-06-21' },
  { id: 'chatgpt-business', categoryCode: 'gpt', providerCode: 'openai', displayName: 'ChatGPT Business', slug: 'chatgpt-business', description: 'OpenAI Business workspace 成员邀请，需确认风险', publishPolicy: 'allowed', accessMode: 'provider_member_invitation', providerPolicyStatus: 'possibly_restricted', riskLevel: 'elevated', riskAckRequired: true, policyVersion: 1, policyNote: 'Business 按现有独立配置执行。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', riskNoticeCode: 'openai_subscription_carpool', active: true, sortOrder: 40, allowCustomVariant: false, createdAt: '2026-06-18', updatedAt: '2026-06-21' },
  { id: 'claude-pro', categoryCode: 'claude', providerCode: 'anthropic', displayName: 'Claude Pro', slug: 'claude-pro', description: '社区 Claude Pro 拼车品类', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, policyNote: '需说明成员、席位或站外访问安排。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 50, allowCustomVariant: false, createdAt: '2026-06-18', updatedAt: '2026-06-18' },
  { id: 'claude-pro-5x', categoryCode: 'claude', providerCode: 'anthropic', displayName: 'Claude Pro 5x', slug: 'claude-pro-5x', description: '社区 Claude Pro 5x 拼车品类', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, policyNote: '需说明成员、席位或站外访问安排。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 60, allowCustomVariant: false, createdAt: '2026-06-18', updatedAt: '2026-06-18' },
  { id: 'claude-pro-20x', categoryCode: 'claude', providerCode: 'anthropic', displayName: 'Claude Pro 20x', slug: 'claude-pro-20x', description: '社区 Claude Pro 20x 拼车品类', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, policyNote: '需说明成员、席位或站外访问安排。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 70, allowCustomVariant: false, createdAt: '2026-06-18', updatedAt: '2026-06-18' },
  { id: 'other-custom', categoryCode: 'other', providerCode: 'other', displayName: '其他 / 自定义', slug: 'other-custom', description: '提交后由管理员映射或新增目录项', publishPolicy: 'allowed', accessMode: 'other_off_platform', providerPolicyStatus: 'unknown', riskLevel: 'normal', riskAckRequired: false, policyVersion: 1, policyNote: '自定义产品需要管理员映射或新增目录项。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 999, allowCustomVariant: true, createdAt: '2026-06-18', updatedAt: '2026-06-18' },
]

export const carpoolRegions: RegionOption[] = [
  { code: 'ph', displayName: '菲律宾区', active: true, sortOrder: 10 },
  { code: 'tr', displayName: '土耳其区', active: true, sortOrder: 20 },
  { code: 'hk', displayName: '香港区', active: true, sortOrder: 30 },
  { code: 'jp', displayName: '日本区', active: true, sortOrder: 40 },
  { code: 'us', displayName: '美国区', active: true, sortOrder: 50 },
  { code: 'other', displayName: '其他', active: true, sortOrder: 999 },
]

export const carpoolOpeningChannels: OpeningChannelOption[] = [
  { code: 'web', displayName: 'Web', active: true, sortOrder: 10 },
  { code: 'ios_app_store', displayName: 'iOS App Store', active: true, sortOrder: 20 },
  { code: 'google_play', displayName: 'Google Play', active: true, sortOrder: 30 },
  { code: 'team_seat', displayName: '团队 / Business 席位', active: true, sortOrder: 40 },
  { code: 'other', displayName: '其他', active: true, sortOrder: 999 },
]

export const carpoolPaymentMethods: PaymentMethodOption[] = [
  { code: 'credit_card', displayName: '信用卡', active: true, sortOrder: 10 },
  { code: 'virtual_card', displayName: '虚拟卡', active: true, sortOrder: 20 },
  { code: 'apple_pay', displayName: 'Apple Pay', active: true, sortOrder: 30 },
  { code: 'google_pay', displayName: 'Google Pay', active: true, sortOrder: 40 },
  { code: 'gift_card', displayName: '礼品卡', active: true, sortOrder: 50 },
  { code: 'local_payment', displayName: '本地支付', active: true, sortOrder: 60 },
  { code: 'other', displayName: '其他', active: true, sortOrder: 999 },
]

export const myUserProfile: UserProfile = {
  id: 'user-orbit',
  username: 'orbit',
  displayName: 'orbit',
  bio: '个人车主和 API 商户，偏好小额测试后再长期合作。',
  avatarMode: 'linuxdo',
  avatarUrl: null,
  customAvatarUrl: null,
  email: null,
  emailVerified: false,
  emailVerifiedAt: null,
  passwordConfigured: false,
  regionCode: 'cn-east',
  timezone: 'Asia/Shanghai',
  linuxDoBinding: {
    bound: true,
    linuxDoUserId: '1024',
    linuxDoUsername: 'orbit',
    linuxDoAvatarUrl: null,
    trustLevel: 4,
    lastSyncedAt: '2026-06-19 16:40',
  },
  badges: [
    { id: 'badge-linuxdo-bound', code: 'linuxdo_bound', label: '已绑定 linux.do', type: 'system' },
    { id: 'badge-personal-owner', code: 'personal_owner', label: '个人车主', type: 'identity' },
    { id: 'badge-api-merchant', code: 'api_merchant', label: 'API 商户', type: 'merchant' },
  ],
  accountStatus: 'normal',
  permissions: ['admin'],
  restrictions: [],
  usernameChangePolicy: {
    canChange: false,
    nextAvailableAt: '2026-07-18',
  },
  privacy: {
    showCreatedAt: true,
    showLastActiveAt: true,
    showCompletionStats: true,
    showResponseMedian: true,
    showResolvedDisputeSummary: true,
    allowPublicProfileReport: true,
  },
  createdAt: '2025-11-18',
  lastActiveAt: '12 分钟前',
}

export const myContactMethods: UserContactMethod[] = [
  {
    id: 'contact-linuxdo-orbit',
    userId: 'user-orbit',
    type: 'linuxdo',
    label: 'linux.do 私信',
    maskedValue: '@orbit',
    displayValue: '@orbit',
    usageScopes: ['carpool_owner', 'api_merchant', 'buyer', 'dispute'],
    isDefault: true,
    enabled: true,
    verified: true,
    createdAt: '2026-06-01 10:00',
    updatedAt: '2026-06-19 16:40',
  },
  {
    id: 'contact-wechat-orbit',
    userId: 'user-orbit',
    type: 'wechat',
    label: '微信',
    maskedValue: 'c2c_***',
    displayValue: 'c2c_orbit',
    usageScopes: ['carpool_owner', 'api_merchant'],
    isDefault: true,
    enabled: true,
    verified: false,
    createdAt: '2026-06-10 11:20',
    updatedAt: '2026-06-18 09:30',
  },
  {
    id: 'contact-email-orbit',
    userId: 'user-orbit',
    type: 'email',
    label: '联系窗口邮箱',
    maskedValue: 'he***@example.com',
    displayValue: 'hello@example.com',
    usageScopes: ['api_merchant', 'dispute'],
    isDefault: false,
    enabled: true,
    verified: true,
    createdAt: '2026-06-12 14:10',
    updatedAt: '2026-06-18 20:05',
  },
]

export const parsedLinuxDoTopicMock: ParsedLinuxDoTopic = {
  topicId: '123456',
  topicUrl: 'https://linux.do/t/topic/123456',
  title: 'ChatGPT Business workspace 成员席位，剩余 2 席',
  authorUsername: 'orbit',
  authorUserId: 'linuxdo-1024',
  createdAt: '2026-06-18 21:20',
  updatedAt: '2026-06-19 09:42',
  authorMatchesBoundUser: true,
  detected: {
    productId: 'chatgpt-business',
    productText: 'ChatGPT Business',
    regionCode: 'ph',
    regionText: '菲律宾区',
    monthlyPriceCny: 188,
    totalSeats: 5,
    availableSeats: 2,
    occupiedSeats: 3,
    openingChannelId: 'team_seat',
    paymentMethodIds: ['credit_card'],
    warrantyMode: 'remaining_days_compensation',
  },
  confidence: {
    product: 'high',
    region: 'high',
    monthlyPrice: 'high',
    seats: 'medium',
  },
}

export const officialPrices: OfficialPrice[] = [
  { id: 'p1', product: 'ChatGPT', plan: 'Plus', region: '土耳其区', channel: 'iOS', openingMethod: 'Apple Store', originalPrice: 'TRY 499', cny: 108, status: '已验证', source: 'linux.do 低价帖', submitter: '青柠', submitterTrust: 3, updatedAt: '12 分钟前', isLowest: true },
  { id: 'p2', product: 'ChatGPT', plan: 'Pro', region: '菲律宾区', channel: 'Web', openingMethod: '虚拟卡', originalPrice: 'PHP 7,990', cny: 988, status: '已验证', source: 'linux.do 低价帖', submitter: 'orbit', submitterTrust: 3, updatedAt: '今天 16:30', isLowest: true },
  { id: 'p3', product: 'Claude', plan: 'Max 5x', region: '香港区', channel: 'Web', openingMethod: '本地卡', originalPrice: 'HKD 780', cny: 724, status: '待验证', source: '用户线索', submitter: '北风', submitterTrust: 2, updatedAt: '2 小时前' },
  { id: 'p4', product: 'Cursor', plan: 'Pro', region: '新加坡区', channel: 'Web', openingMethod: '虚拟卡', originalPrice: 'SGD 28', cny: 154, status: '需复核', source: '官方页面', submitter: '管理员', submitterTrust: 4, updatedAt: '3 天前' },
]

export const carpools: Carpool[] = [
  { id: 'c1', product: 'ChatGPT Business', region: '美国区', monthly: 188, serviceMultiplier: 1.2, monthlyQuotaAmount: 200, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '3/5', pricingMode: 'fixed', fixedMonthlyPrice: 188, currentConfirmedMembers: 3, maxMembers: 5, settlementDeadline: '2026-06-25', owner: 'orbit', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '其他', status: '可上车', confirmedAt: '12 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: 'Business workspace 管理员邀请成员席位；不得共享 Pro/Plus 主账号、密码、Session 或 Cookie。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c2', product: 'Cursor Pro', region: '土耳其区', monthly: 68, serviceMultiplier: 1, monthlyQuotaAmount: 500, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '1/6', pricingMode: 'fixed', fixedMonthlyPrice: 68, currentConfirmedMembers: 1, maxMembers: 6, owner: '青柠', trustLevel: 3, ownerType: '个人车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '35 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: '团队成员邀请或独立席位授权。' },
  { id: 'c3', product: 'Claude Max 5x', region: '香港区', monthly: 80, serviceMultiplier: 1.1, monthlyQuotaAmount: 300, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/4', pricingMode: 'tiered', currentConfirmedMembers: 2, maxMembers: 4, pricingTiers: [{ memberCount: 2, price: 120 }, { memberCount: 3, price: 80 }, { memberCount: 4, price: 60 }], owner: '北风', trustLevel: 4, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '本地卡', status: '审核中', confirmedAt: '1 小时前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false, accessArrangementMode: 'owner_managed_access', accessArrangementNote: '车主站外管理成员访问，不在平台保存凭据。' },
  { id: 'c4', product: 'Cursor Pro', region: '新加坡区', monthly: 39, serviceMultiplier: 1, monthlyQuotaAmount: 200, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '4/4', pricingMode: 'fixed', fixedMonthlyPrice: 39, currentConfirmedMembers: 4, maxMembers: 4, owner: '周末研究员', trustLevel: 2, ownerType: '商户车源', warranty: '售后协商', openingMethod: '其他', status: '已满', confirmedAt: '今天 09:24', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false },
  { id: 'c5', product: 'ChatGPT Business', region: '日本区', monthly: 198, serviceMultiplier: 1.25, monthlyQuotaAmount: 200, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/5', pricingMode: 'fixed', fixedMonthlyPrice: 198, currentConfirmedMembers: 2, maxMembers: 5, settlementDeadline: '2026-06-24', owner: '木舟', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '其他', status: '可上车', confirmedAt: '2 小时前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: 'Business workspace 管理员邀请成员席位。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c6', product: 'ChatGPT Pro 20x Web', region: '香港区', monthly: 178, serviceMultiplier: 1.35, monthlyQuotaAmount: 300, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '5/6', pricingMode: 'fixed', fixedMonthlyPrice: 178, currentConfirmedMembers: 5, maxMembers: 6, owner: '纸船', trustLevel: 2, ownerType: '可信新车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '今天 10:20', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false, accessArrangementMode: 'personal_account_cost_share', accessArrangementNote: '个人订阅费用分摊，平台不保存、不交付任何密码、Session、Cookie 或 token。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c7', product: 'Cursor Pro', region: '新加坡区', monthly: 49, serviceMultiplier: 1, monthlyQuotaAmount: 500, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/4', pricingMode: 'tiered', currentConfirmedMembers: 2, maxMembers: 4, pricingTiers: [{ memberCount: 2, price: 69 }, { memberCount: 3, price: 56 }, { memberCount: 4, price: 49 }], owner: '栈帧', trustLevel: 3, ownerType: '个人车主', warranty: '售后协商', openingMethod: '虚拟卡', status: '可上车', confirmedAt: '45 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false },
  { id: 'c8', product: 'Perplexity Pro', region: '美国区', monthly: 42, serviceMultiplier: 1, monthlyQuotaAmount: 150, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '1/3', pricingMode: 'fixed', fixedMonthlyPrice: 42, currentConfirmedMembers: 1, maxMembers: 3, owner: '海盐', trustLevel: 2, ownerType: '可信新车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '1 小时前', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false },
  { id: 'c9', product: 'Gemini Advanced', region: '日本区', monthly: 36, serviceMultiplier: 1, monthlyQuotaAmount: 100, quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/5', pricingMode: 'equal_share', totalShareableCost: 108, currentConfirmedMembers: 2, maxMembers: 5, settlementDeadline: '2026-06-26', owner: '雨季', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '本地卡', status: '可上车', confirmedAt: '今天 11:05', confirmedWithin48h: true, linuxdoBound: true, sourcePostAccessible: true, hasInfoConflict: false, hasUnresolvedDispute: false },
]

const carpoolApplicationSnapshots: Record<string, CarpoolApplicationSnapshot> = {
  c1: {
    carpoolId: 'c1',
    productName: 'ChatGPT Business',
    regionName: '美国区',
    monthlyPriceCny: 188,
    serviceMultiplier: 1.2,
    monthlyQuotaAmount: 200,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    priceLabel: '成员席位价',
    openingChannelName: 'Business workspace 成员席位',
    paymentMethodNames: ['信用卡'],
    warrantyText: '车主承诺',
    rulesVersion: '2026-06-19 16:20',
    rulesText: '付款周期按自然月结算；通过 Business workspace 邀请成员席位；不得共享主账号、密码、Session 或 Cookie。',
    ownerUserId: 'owner-orbit',
    ownerUsername: 'orbit',
    ownerTrustLevel: 3,
    ownerType: '个人车主',
    sourceTopicUrl: 'https://linux.do/t/topic/123456',
    accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: 'Business workspace 管理员邀请成员席位。',
  },
  c2: {
    carpoolId: 'c2',
    productName: 'Cursor Pro',
    regionName: '土耳其区',
    monthlyPriceCny: 68,
    serviceMultiplier: 1,
    monthlyQuotaAmount: 500,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    priceLabel: '固定月费',
    openingChannelName: '团队成员席位',
    paymentMethodNames: ['信用卡'],
    warrantyText: '售后协商',
    rulesVersion: '2026-06-19 11:10',
    rulesText: '按月确认成员席位资格，异常情况由双方站外协商处理。',
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    ownerTrustLevel: 3,
    ownerType: '个人车主',
    sourceTopicUrl: 'https://linux.do/t/topic/223456',
    accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: '团队成员邀请或独立席位授权。',
  },
  c3: {
    carpoolId: 'c3',
    productName: 'Claude Max 5x',
    regionName: '香港区',
    monthlyPriceCny: 80,
    serviceMultiplier: 1.1,
    monthlyQuotaAmount: 300,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    priceLabel: '当前阶梯价',
    openingChannelName: '本地卡',
    paymentMethodNames: ['本地支付'],
    warrantyText: '车主承诺',
    rulesVersion: '2026-06-18 22:40',
    rulesText: '服务周期内保持席位，需提前一天确认续期。',
    ownerUserId: 'owner-beifeng',
    ownerUsername: '北风',
    ownerTrustLevel: 4,
    ownerType: '个人车主',
    sourceTopicUrl: 'https://linux.do/t/topic/323456',
    accessArrangementMode: 'owner_managed_access',
    accessArrangementNote: '车主站外管理成员访问，不在平台保存凭据。',
  },
}

export const carpoolApplications: CarpoolApplication[] = [
  {
    id: 'ride-app-1',
    carpoolId: 'c1',
    applicantUserId: 'buyer-zhichuan',
    applicantUsername: '纸船',
    applicantStats: { linuxdoBound: true, trustLevel: 2, completed30d: 1, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-orbit',
    ownerUsername: 'orbit',
    status: 'pending_owner',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c1,
    reservedUntil: null,
    buyerContactedAt: null,
    buyerConfirmedJoinedAt: null,
    ownerConfirmedJoinedAt: null,
    startedAt: null,
    expectedEndAt: null,
    buyerConfirmedCompletedAt: null,
    ownerConfirmedCompletedAt: null,
    completedAt: null,
    completionMode: null,
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: null,
    disputeReason: null,
    createdAt: '2026-06-19 16:18',
    updatedAt: '2026-06-19 16:18',
  },
  {
    id: 'ride-app-2',
    carpoolId: 'c1',
    applicantUserId: 'buyer-muzhou',
    applicantUsername: '木舟',
    applicantStats: { linuxdoBound: true, trustLevel: 2, completed30d: 1, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-orbit',
    ownerUsername: 'orbit',
    status: 'accepted_reserved',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c1,
    reservedUntil: '2026-06-19 17:05',
    buyerContactedAt: null,
    buyerConfirmedJoinedAt: null,
    ownerConfirmedJoinedAt: null,
    startedAt: null,
    expectedEndAt: null,
    buyerConfirmedCompletedAt: null,
    ownerConfirmedCompletedAt: null,
    completedAt: null,
    completionMode: null,
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: null,
    disputeReason: null,
    createdAt: '2026-06-19 15:55',
    updatedAt: '2026-06-19 16:35',
  },
  {
    id: 'ride-app-3',
    carpoolId: 'c2',
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 2, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    status: 'active',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c2,
    reservedUntil: null,
    buyerContactedAt: '2026-06-18 20:12',
    buyerConfirmedJoinedAt: '2026-06-18 20:24',
    ownerConfirmedJoinedAt: '2026-06-18 20:26',
    startedAt: '2026-06-18 20:26',
    expectedEndAt: '2026-07-18 20:26',
    buyerConfirmedCompletedAt: null,
    ownerConfirmedCompletedAt: null,
    completedAt: null,
    completionMode: null,
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: null,
    disputeReason: null,
    createdAt: '2026-06-18 19:50',
    updatedAt: '2026-06-18 20:26',
  },
  {
    id: 'ride-app-4',
    carpoolId: 'c3',
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 2, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-beifeng',
    ownerUsername: '北风',
    status: 'pending_completion',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c3,
    reservedUntil: null,
    buyerContactedAt: '2026-05-19 12:30',
    buyerConfirmedJoinedAt: '2026-05-19 12:45',
    ownerConfirmedJoinedAt: '2026-05-19 12:48',
    startedAt: '2026-05-19 12:48',
    expectedEndAt: '2026-06-19 12:48',
    buyerConfirmedCompletedAt: null,
    ownerConfirmedCompletedAt: null,
    completedAt: null,
    completionMode: null,
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: null,
    disputeReason: null,
    createdAt: '2026-05-19 12:12',
    updatedAt: '2026-06-19 12:48',
  },
  {
    id: 'ride-app-5',
    carpoolId: 'c2',
    applicantUserId: 'buyer-haiyan',
    applicantUsername: '海盐',
    applicantStats: { linuxdoBound: true, trustLevel: 2, completed30d: 0, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    status: 'completed',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c2,
    reservedUntil: null,
    buyerContactedAt: '2026-05-10 10:12',
    buyerConfirmedJoinedAt: '2026-05-10 10:20',
    ownerConfirmedJoinedAt: '2026-05-10 10:22',
    startedAt: '2026-05-10 10:22',
    expectedEndAt: '2026-06-10 10:22',
    buyerConfirmedCompletedAt: '2026-06-10 12:00',
    ownerConfirmedCompletedAt: '2026-06-10 12:04',
    completedAt: '2026-06-10 12:04',
    completionMode: 'mutual',
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: null,
    disputeReason: null,
    buyerReview: { rating: 5, tags: ['规则清楚', '服务稳定'], note: '本地 mock 已验证评价。', createdAt: '2026-06-10 12:08' },
    createdAt: '2026-05-10 10:00',
    updatedAt: '2026-06-10 12:08',
  },
  {
    id: 'ride-app-6',
    carpoolId: 'c3',
    applicantUserId: 'buyer-yuji',
    applicantUsername: '雨季',
    applicantStats: { linuxdoBound: false, trustLevel: 1, completed30d: 0, buyerResponsibleCancellations: 1, ownerResponsibleCancellations: 0, unresolvedDisputes: 1 },
    ownerUserId: 'owner-beifeng',
    ownerUsername: '北风',
    status: 'disputed',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c3,
    reservedUntil: null,
    buyerContactedAt: '2026-06-16 09:30',
    buyerConfirmedJoinedAt: '2026-06-16 09:50',
    ownerConfirmedJoinedAt: '2026-06-16 09:52',
    startedAt: '2026-06-16 09:52',
    expectedEndAt: '2026-07-16 09:52',
    buyerConfirmedCompletedAt: null,
    ownerConfirmedCompletedAt: null,
    completedAt: null,
    completionMode: null,
    cancellationReasonCode: null,
    cancellationReasonText: null,
    responsibility: 'undetermined',
    disputeReason: '用户反馈开通区说明与实际不一致，待管理员复核。',
    createdAt: '2026-06-16 09:10',
    updatedAt: '2026-06-18 15:30',
  },
]

export const carpoolApplicationEvents: CarpoolApplicationEvent[] = [
  { id: 'ride-event-1', applicationId: 'ride-app-1', actorId: 'buyer-zhichuan', actorLabel: '纸船', actorRole: 'buyer', type: 'application_created', toStatus: 'pending_owner', note: '买家提交上车申请，等待车主处理。', createdAt: '2026-06-19 16:18' },
  { id: 'ride-event-2', applicationId: 'ride-app-2', actorId: 'buyer-muzhou', actorLabel: '木舟', actorRole: 'buyer', type: 'application_created', toStatus: 'pending_owner', note: '买家提交上车申请。', createdAt: '2026-06-19 15:55' },
  { id: 'ride-event-3', applicationId: 'ride-app-2', actorId: 'owner-orbit', actorLabel: 'orbit', actorRole: 'owner', type: 'owner_accepted', fromStatus: 'pending_owner', toStatus: 'accepted_reserved', note: '车主接受申请，预留 1 席 30 分钟。', createdAt: '2026-06-19 16:35' },
  { id: 'ride-event-4', applicationId: 'ride-app-3', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'buyer_contacted', fromStatus: 'accepted_reserved', toStatus: 'contacted', note: '买家已记录完成站外联系。', createdAt: '2026-06-18 20:12' },
  { id: 'ride-event-5', applicationId: 'ride-app-3', actorId: 'system', actorLabel: '系统', actorRole: 'system', type: 'service_started', fromStatus: 'joined_pending_confirmation', toStatus: 'active', note: '双方确认后进入服务中。', createdAt: '2026-06-18 20:26' },
  { id: 'ride-event-6', applicationId: 'ride-app-4', actorId: 'system', actorLabel: '系统', actorRole: 'system', type: 'pending_completion', fromStatus: 'active', toStatus: 'pending_completion', note: '服务周期到期，等待双方确认完成。', createdAt: '2026-06-19 12:48' },
  { id: 'ride-event-7', applicationId: 'ride-app-5', actorId: 'system', actorLabel: '系统', actorRole: 'system', type: 'completed', fromStatus: 'pending_completion', toStatus: 'completed', note: '双方确认完成，评价可用。', createdAt: '2026-06-10 12:04' },
  { id: 'ride-event-8', applicationId: 'ride-app-6', actorId: 'buyer-yuji', actorLabel: '雨季', actorRole: 'buyer', type: 'disputed', fromStatus: 'active', toStatus: 'disputed', note: '买家发起纠纷，等待管理员处理。', createdAt: '2026-06-18 15:30' },
]

export const adminUserRiskProfiles: AdminUserRiskProfile[] = [
  { id: 'buyer-demo-user', username: 'demo_user', linuxdoBound: true, trustLevel: 3, identity: '普通用户', accountStatus: 'normal', carpoolCompletions: 2, apiCompletions: 1, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0, restrictions: [], lastActiveAt: '刚刚' },
  { id: 'owner-orbit', username: 'orbit', linuxdoBound: true, trustLevel: 3, identity: '个人车主', accountStatus: 'normal', carpoolCompletions: 12, apiCompletions: 8, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 1, unresolvedDisputes: 0, restrictions: [], lastActiveAt: '12 分钟前' },
  { id: 'owner-qingning', username: '青柠', linuxdoBound: true, trustLevel: 3, identity: '个人车主', accountStatus: 'warning', carpoolCompletions: 9, apiCompletions: 0, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0, restrictions: ['低价线索需复核'], lastActiveAt: '35 分钟前' },
  { id: 'buyer-yuji', username: '雨季', linuxdoBound: false, trustLevel: 1, identity: '普通用户', accountStatus: 'partially_restricted', carpoolCompletions: 0, apiCompletions: 0, buyerResponsibleCancellations: 1, ownerResponsibleCancellations: 0, unresolvedDisputes: 1, restrictions: ['禁止申请上车'], lastActiveAt: '昨天 18:20' },
  { id: 'merchant-beifeng', username: 'beifeng-api', linuxdoBound: true, trustLevel: 2, identity: 'API 商户', accountStatus: 'temporarily_suspended', carpoolCompletions: 4, apiCompletions: 6, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 2, unresolvedDisputes: 1, restrictions: ['暂停商户资格', '禁止发布 API 服务'], lastActiveAt: '今天 09:40' },
  { id: 'user-banned', username: '灰名单用户', linuxdoBound: false, trustLevel: 0, identity: '普通用户', accountStatus: 'permanently_banned', carpoolCompletions: 0, apiCompletions: 0, buyerResponsibleCancellations: 3, ownerResponsibleCancellations: 0, unresolvedDisputes: 2, restrictions: ['账号登录封禁'], lastActiveAt: '3 天前' },
]

export const adminAuditLogs: AdminAuditLog[] = [
  { id: 'audit-1', actorType: 'admin', actorLabel: '管理员', action: '审核通过', targetType: 'carpool', targetId: 'c1', targetLabel: 'ChatGPT Business', beforeStatus: '待审核', afterStatus: 'approved_offline', reason: '原帖已绑定，已声明 Business workspace 成员席位机制。', createdAt: '2026-06-19 15:42' },
  { id: 'audit-2', actorType: 'admin', actorLabel: '管理员', action: '标记风险', targetType: 'application', targetId: 'ride-app-6', targetLabel: 'Claude Max 5x 上车申请', beforeStatus: '服务中', afterStatus: '纠纷中', reason: '开通区说明存在争议。', createdAt: '2026-06-18 15:30' },
  { id: 'audit-3', actorType: 'system', actorLabel: '系统', action: '自动提醒', targetType: 'api-service', targetId: 'a3', targetLabel: '多模型备用池', beforeStatus: 'online', afterStatus: 'paused', reason: '连续 2 次未响应购买意向。', createdAt: '2026-06-19 14:10' },
]

export const productTrends: ProductTrend[] = [
  {
    slug: 'chatgpt-business',
    label: 'ChatGPT Business',
    officialVerifiedLow: 188,
    officialRegion: '美国区 / Business workspace',
    officialSource: 'OpenAI Business 帮助页 + 社区完成参考样本',
    verifiedAt: '今天 16:50',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 208, p25Price: 188, p75Price: 236, transactionCount: 2 },
        { date: '06-14', medianPrice: 198, p25Price: 188, p75Price: 218, transactionCount: 3 },
        { date: '06-16', medianPrice: 192, p25Price: 178, p75Price: 208, transactionCount: 4 },
        { date: '06-18', medianPrice: 188, p25Price: 178, p75Price: 198, transactionCount: 3 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 228, p25Price: 208, p75Price: 258, transactionCount: 4 },
        { date: '05-26', medianPrice: 218, p25Price: 198, p75Price: 248, transactionCount: 5 },
        { date: '06-01', medianPrice: 208, p25Price: 188, p75Price: 232, transactionCount: 7 },
        { date: '06-07', medianPrice: 198, p25Price: 184, p75Price: 218, transactionCount: 6 },
        { date: '06-13', medianPrice: 192, p25Price: 178, p75Price: 208, transactionCount: 8 },
        { date: '06-18', medianPrice: 188, p25Price: 178, p75Price: 198, transactionCount: 6 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 258, p25Price: 228, p75Price: 298, transactionCount: 8 },
        { date: '04-08', medianPrice: 248, p25Price: 218, p75Price: 284, transactionCount: 9 },
        { date: '04-22', medianPrice: 236, p25Price: 208, p75Price: 268, transactionCount: 11 },
        { date: '05-06', medianPrice: 222, p25Price: 198, p75Price: 252, transactionCount: 13 },
        { date: '05-20', medianPrice: 208, p25Price: 188, p75Price: 236, transactionCount: 15 },
        { date: '06-03', medianPrice: 198, p25Price: 184, p75Price: 218, transactionCount: 14 },
        { date: '06-18', medianPrice: 188, p25Price: 178, p75Price: 198, transactionCount: 12 },
      ],
    },
  },
  {
    slug: 'chatgpt-plus',
    label: 'ChatGPT Plus',
    officialVerifiedLow: 108,
    officialRegion: '土耳其区 / iOS',
    officialSource: 'linux.do 低价帖',
    verifiedAt: '12 分钟前',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 68, p25Price: 62, p75Price: 74, transactionCount: 3 },
        { date: '06-13', medianPrice: 66, p25Price: 61, p75Price: 72, transactionCount: 4 },
        { date: '06-14', medianPrice: 70, p25Price: 64, p75Price: 76, transactionCount: 5 },
        { date: '06-15', medianPrice: 67, p25Price: 62, p75Price: 73, transactionCount: 3 },
        { date: '06-16', medianPrice: 65, p25Price: 60, p75Price: 71, transactionCount: 6 },
        { date: '06-17', medianPrice: 68, p25Price: 63, p75Price: 75, transactionCount: 4 },
        { date: '06-18', medianPrice: 66, p25Price: 61, p75Price: 72, transactionCount: 5 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 72, p25Price: 65, p75Price: 78, transactionCount: 5 },
        { date: '05-25', medianPrice: 69, p25Price: 63, p75Price: 75, transactionCount: 8 },
        { date: '05-30', medianPrice: 70, p25Price: 62, p75Price: 77, transactionCount: 6 },
        { date: '06-04', medianPrice: 67, p25Price: 61, p75Price: 73, transactionCount: 9 },
        { date: '06-09', medianPrice: 68, p25Price: 62, p75Price: 74, transactionCount: 7 },
        { date: '06-14', medianPrice: 66, p25Price: 60, p75Price: 72, transactionCount: 10 },
        { date: '06-18', medianPrice: 66, p25Price: 61, p75Price: 72, transactionCount: 5 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 78, p25Price: 70, p75Price: 86, transactionCount: 10 },
        { date: '04-08', medianPrice: 74, p25Price: 68, p75Price: 82, transactionCount: 12 },
        { date: '04-22', medianPrice: 73, p25Price: 67, p75Price: 80, transactionCount: 14 },
        { date: '05-06', medianPrice: 70, p25Price: 63, p75Price: 76, transactionCount: 16 },
        { date: '05-20', medianPrice: 69, p25Price: 62, p75Price: 75, transactionCount: 18 },
        { date: '06-03', medianPrice: 67, p25Price: 61, p75Price: 73, transactionCount: 15 },
        { date: '06-18', medianPrice: 66, p25Price: 61, p75Price: 72, transactionCount: 19 },
      ],
    },
  },
  {
    slug: 'chatgpt-pro-5x-web',
    label: 'ChatGPT Pro 5x Web',
    officialVerifiedLow: 588,
    officialRegion: '美国区 / Web',
    officialSource: '官方页面截图',
    verifiedAt: '今天 15:40',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 108, p25Price: 96, p75Price: 124, transactionCount: 2 },
        { date: '06-14', medianPrice: 102, p25Price: 94, p75Price: 118, transactionCount: 1 },
        { date: '06-18', medianPrice: 96, p25Price: 88, p75Price: 110, transactionCount: 1 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 128, p25Price: 112, p75Price: 146, transactionCount: 4 },
        { date: '05-26', medianPrice: 118, p25Price: 104, p75Price: 132, transactionCount: 5 },
        { date: '06-01', medianPrice: 112, p25Price: 100, p75Price: 126, transactionCount: 7 },
        { date: '06-07', medianPrice: 108, p25Price: 96, p75Price: 122, transactionCount: 6 },
        { date: '06-13', medianPrice: 102, p25Price: 92, p75Price: 116, transactionCount: 8 },
        { date: '06-18', medianPrice: 96, p25Price: 88, p75Price: 110, transactionCount: 6 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 148, p25Price: 132, p75Price: 166, transactionCount: 9 },
        { date: '04-08', medianPrice: 142, p25Price: 126, p75Price: 160, transactionCount: 11 },
        { date: '04-22', medianPrice: 136, p25Price: 120, p75Price: 154, transactionCount: 13 },
        { date: '05-06', medianPrice: 126, p25Price: 112, p75Price: 142, transactionCount: 16 },
        { date: '05-20', medianPrice: 118, p25Price: 104, p75Price: 132, transactionCount: 17 },
        { date: '06-03', medianPrice: 108, p25Price: 96, p75Price: 122, transactionCount: 14 },
        { date: '06-18', medianPrice: 96, p25Price: 88, p75Price: 110, transactionCount: 16 },
      ],
    },
  },
  {
    slug: 'chatgpt-pro-20x-web',
    label: 'ChatGPT Pro 20x Web',
    officialVerifiedLow: 988,
    officialRegion: '菲律宾区 / Web',
    officialSource: 'linux.do 低价帖',
    verifiedAt: '18 分钟前',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 146, p25Price: 128, p75Price: 168, transactionCount: 4 },
        { date: '06-13', medianPrice: 138, p25Price: 122, p75Price: 160, transactionCount: 5 },
        { date: '06-14', medianPrice: 132, p25Price: 118, p75Price: 152, transactionCount: 6 },
        { date: '06-15', medianPrice: 126, p25Price: 112, p75Price: 144, transactionCount: 4 },
        { date: '06-16', medianPrice: 122, p25Price: 108, p75Price: 138, transactionCount: 7 },
        { date: '06-17', medianPrice: 120, p25Price: 106, p75Price: 136, transactionCount: 8 },
        { date: '06-18', medianPrice: 118, p25Price: 104, p75Price: 134, transactionCount: 7 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 168, p25Price: 146, p75Price: 196, transactionCount: 7 },
        { date: '05-25', medianPrice: 158, p25Price: 138, p75Price: 182, transactionCount: 9 },
        { date: '05-30', medianPrice: 148, p25Price: 130, p75Price: 172, transactionCount: 11 },
        { date: '06-04', medianPrice: 136, p25Price: 120, p75Price: 156, transactionCount: 12 },
        { date: '06-09', medianPrice: 128, p25Price: 112, p75Price: 148, transactionCount: 14 },
        { date: '06-14', medianPrice: 122, p25Price: 108, p75Price: 140, transactionCount: 16 },
        { date: '06-18', medianPrice: 118, p25Price: 104, p75Price: 134, transactionCount: 13 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 220, p25Price: 188, p75Price: 260, transactionCount: 13 },
        { date: '04-08', medianPrice: 198, p25Price: 172, p75Price: 232, transactionCount: 15 },
        { date: '04-22', medianPrice: 184, p25Price: 160, p75Price: 216, transactionCount: 19 },
        { date: '05-06', medianPrice: 166, p25Price: 144, p75Price: 194, transactionCount: 22 },
        { date: '05-20', medianPrice: 148, p25Price: 128, p75Price: 174, transactionCount: 24 },
        { date: '06-03', medianPrice: 132, p25Price: 116, p75Price: 154, transactionCount: 25 },
        { date: '06-18', medianPrice: 118, p25Price: 104, p75Price: 134, transactionCount: 21 },
      ],
    },
  },
  {
    slug: 'claude-max-5x',
    label: 'Claude Max 5x',
    officialVerifiedLow: 724,
    officialRegion: '香港区 / Web',
    officialSource: '用户线索待复核',
    verifiedAt: '2 小时前',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 132, p25Price: 118, p75Price: 148, transactionCount: 3 },
        { date: '06-14', medianPrice: 126, p25Price: 112, p75Price: 144, transactionCount: 4 },
        { date: '06-16', medianPrice: 120, p25Price: 108, p75Price: 136, transactionCount: 5 },
        { date: '06-18', medianPrice: 116, p25Price: 104, p75Price: 132, transactionCount: 3 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 148, p25Price: 132, p75Price: 170, transactionCount: 5 },
        { date: '05-26', medianPrice: 142, p25Price: 126, p75Price: 162, transactionCount: 6 },
        { date: '06-01', medianPrice: 136, p25Price: 120, p75Price: 156, transactionCount: 8 },
        { date: '06-07', medianPrice: 128, p25Price: 114, p75Price: 146, transactionCount: 7 },
        { date: '06-13', medianPrice: 122, p25Price: 110, p75Price: 138, transactionCount: 9 },
        { date: '06-18', medianPrice: 116, p25Price: 104, p75Price: 132, transactionCount: 6 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 176, p25Price: 154, p75Price: 204, transactionCount: 8 },
        { date: '04-08', medianPrice: 164, p25Price: 146, p75Price: 190, transactionCount: 10 },
        { date: '04-22', medianPrice: 156, p25Price: 138, p75Price: 180, transactionCount: 12 },
        { date: '05-06', medianPrice: 146, p25Price: 130, p75Price: 166, transactionCount: 13 },
        { date: '05-20', medianPrice: 136, p25Price: 120, p75Price: 156, transactionCount: 15 },
        { date: '06-03', medianPrice: 126, p25Price: 112, p75Price: 144, transactionCount: 14 },
        { date: '06-18', medianPrice: 116, p25Price: 104, p75Price: 132, transactionCount: 12 },
      ],
    },
  },
  {
    slug: 'cursor-pro',
    label: 'Cursor Pro',
    officialVerifiedLow: 154,
    officialRegion: '新加坡区 / Web',
    officialSource: '官方页面',
    verifiedAt: '3 天前',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 48, p25Price: 44, p75Price: 54, transactionCount: 2 },
        { date: '06-13', medianPrice: 46, p25Price: 42, p75Price: 51, transactionCount: 3 },
        { date: '06-14', medianPrice: 45, p25Price: 41, p75Price: 50, transactionCount: 2 },
        { date: '06-15', medianPrice: 43, p25Price: 39, p75Price: 48, transactionCount: 4 },
        { date: '06-16', medianPrice: 42, p25Price: 38, p75Price: 46, transactionCount: 3 },
        { date: '06-17', medianPrice: 41, p25Price: 38, p75Price: 44, transactionCount: 4 },
        { date: '06-18', medianPrice: 39, p25Price: 36, p75Price: 42, transactionCount: 3 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 54, p25Price: 48, p75Price: 61, transactionCount: 4 },
        { date: '05-25', medianPrice: 50, p25Price: 45, p75Price: 57, transactionCount: 6 },
        { date: '05-30', medianPrice: 48, p25Price: 43, p75Price: 54, transactionCount: 5 },
        { date: '06-04', medianPrice: 46, p25Price: 41, p75Price: 51, transactionCount: 7 },
        { date: '06-09', medianPrice: 44, p25Price: 39, p75Price: 49, transactionCount: 6 },
        { date: '06-14', medianPrice: 42, p25Price: 38, p75Price: 46, transactionCount: 8 },
        { date: '06-18', medianPrice: 39, p25Price: 36, p75Price: 42, transactionCount: 7 },
      ],
      '90d': [
        { date: '04-01', medianPrice: 48, p25Price: 43, p75Price: 54, transactionCount: 4 },
        { date: '04-26', medianPrice: 45, p25Price: 40, p75Price: 50, transactionCount: 5 },
        { date: '05-21', medianPrice: 42, p25Price: 38, p75Price: 46, transactionCount: 6 },
        { date: '06-18', medianPrice: 39, p25Price: 36, p75Price: 42, transactionCount: 4 },
      ],
    },
  },
  {
    slug: 'more-products',
    label: '更多产品',
    officialVerifiedLow: 0,
    officialRegion: '多产品聚合',
    officialSource: '社区线索',
    verifiedAt: '持续收集',
    points: {
      '7d': [
        { date: '06-12', medianPrice: 52, p25Price: 39, p75Price: 88, transactionCount: 5 },
        { date: '06-13', medianPrice: 50, p25Price: 38, p75Price: 84, transactionCount: 6 },
        { date: '06-14', medianPrice: 48, p25Price: 36, p75Price: 80, transactionCount: 7 },
        { date: '06-15', medianPrice: 46, p25Price: 35, p75Price: 76, transactionCount: 6 },
        { date: '06-16', medianPrice: 44, p25Price: 34, p75Price: 72, transactionCount: 8 },
        { date: '06-17', medianPrice: 42, p25Price: 33, p75Price: 68, transactionCount: 9 },
        { date: '06-18', medianPrice: 40, p25Price: 32, p75Price: 64, transactionCount: 8 },
      ],
      '30d': [
        { date: '05-20', medianPrice: 64, p25Price: 42, p75Price: 118, transactionCount: 10 },
        { date: '05-25', medianPrice: 58, p25Price: 40, p75Price: 104, transactionCount: 12 },
        { date: '05-30', medianPrice: 54, p25Price: 38, p75Price: 96, transactionCount: 14 },
        { date: '06-04', medianPrice: 50, p25Price: 36, p75Price: 88, transactionCount: 15 },
        { date: '06-09', medianPrice: 46, p25Price: 34, p75Price: 78, transactionCount: 16 },
        { date: '06-14', medianPrice: 43, p25Price: 33, p75Price: 70, transactionCount: 18 },
        { date: '06-18', medianPrice: 40, p25Price: 32, p75Price: 64, transactionCount: 17 },
      ],
      '90d': [
        { date: '03-25', medianPrice: 86, p25Price: 55, p75Price: 168, transactionCount: 18 },
        { date: '04-08', medianPrice: 76, p25Price: 50, p75Price: 144, transactionCount: 22 },
        { date: '04-22', medianPrice: 68, p25Price: 45, p75Price: 126, transactionCount: 26 },
        { date: '05-06', medianPrice: 60, p25Price: 40, p75Price: 108, transactionCount: 31 },
        { date: '05-20', medianPrice: 54, p25Price: 38, p75Price: 92, transactionCount: 34 },
        { date: '06-03', medianPrice: 46, p25Price: 34, p75Price: 76, transactionCount: 37 },
        { date: '06-18', medianPrice: 40, p25Price: 32, p75Price: 64, transactionCount: 39 },
      ],
    },
  },
]

export const transactionRecords: TransactionRecord[] = [
  { id: 't1', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 188, regionNote: '美国区 · Business 成员席位', completedAt: '8 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't2', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 2, finalSettlementPrice: 178, regionNote: '香港区 · workspace 邀请', completedAt: '26 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't3', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 198, regionNote: '日本区 · 成员席位', completedAt: '40 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't4', productSlug: 'claude-max-5x', product: 'Claude Max 5x', sourceType: '拼车成交', trustLevel: 4, finalSettlementPrice: 116, regionNote: '香港区 · 个人车主', completedAt: '1 小时前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't5', productSlug: 'cursor-pro', product: 'Cursor Pro', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 56, regionNote: '土耳其区 · 团队席位', completedAt: '2 小时前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't6', productSlug: 'cursor-pro', product: 'Cursor Pro', sourceType: '拼车成交', trustLevel: 2, finalSettlementPrice: 39, regionNote: '新加坡区 · 商户车源', completedAt: '昨天 21:10', status: 'completed', hasUnresolvedDispute: false },
  { id: 't7', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 192, regionNote: '美国区 · workspace 成员', completedAt: '昨天 19:20', status: 'completed', hasUnresolvedDispute: false },
  { id: 't10', productSlug: 'cursor-pro', product: 'Cursor Pro', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 49, regionNote: '新加坡区 · 个人车主', completedAt: '18 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't11', productSlug: 'cursor-pro', product: 'Cursor Pro', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 43, regionNote: '日本区 · 个人车主', completedAt: '58 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't12', productSlug: 'more-products', product: 'Perplexity Pro', sourceType: '拼车成交', trustLevel: 2, finalSettlementPrice: 42, regionNote: '美国区 · 可信新车主', completedAt: '22 分钟前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't13', productSlug: 'more-products', product: 'Gemini Advanced', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 36, regionNote: '日本区 · 个人车主', completedAt: '1 小时前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't14', productSlug: 'more-products', product: 'Notion AI Plus', sourceType: '拼车成交', trustLevel: 2, finalSettlementPrice: 28, regionNote: '美国区 · 个人车主', completedAt: '2 小时前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't15', productSlug: 'more-products', product: 'Poe 订阅', sourceType: '拼车成交', trustLevel: 3, finalSettlementPrice: 32, regionNote: '香港区 · 可信新车主', completedAt: '3 小时前', status: 'completed', hasUnresolvedDispute: false },
  { id: 't8', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 1, finalSettlementPrice: 160, regionNote: '纠纷记录，不计入趋势', completedAt: '昨天 18:00', status: 'completed', hasUnresolvedDispute: true },
  { id: 't9', productSlug: 'chatgpt-business', product: 'ChatGPT Business', sourceType: '拼车成交', trustLevel: 2, finalSettlementPrice: 172, regionNote: '已取消，不计入趋势', completedAt: '昨天 17:00', status: 'cancelled', hasUnresolvedDispute: false },
]

export const modelCatalog: ModelCatalogItem[] = [
  {
    id: 'gpt-5-mini',
    provider: 'openai',
    name: 'gpt-5-mini',
    displayName: 'GPT-5 mini',
    capabilities: ['chat', 'vision', 'reasoning'],
    officialInputPricePerMillion: 0.25,
    officialCachedInputPricePerMillion: 0.025,
    officialOutputPricePerMillion: 2,
    active: true,
  },
  {
    id: 'gpt-5-5',
    provider: 'openai',
    name: 'gpt-5.5',
    displayName: 'GPT-5.5',
    capabilities: ['chat', 'vision', 'reasoning'],
    officialInputPricePerMillion: 1.75,
    officialCachedInputPricePerMillion: 0.175,
    officialOutputPricePerMillion: 14,
    active: true,
  },
  {
    id: 'gpt-image',
    provider: 'openai',
    name: 'gpt-image',
    displayName: 'GPT Image',
    capabilities: ['image_generation', 'image_edit'],
    officialInputPricePerMillion: null,
    officialCachedInputPricePerMillion: null,
    officialOutputPricePerMillion: null,
    active: true,
  },
  {
    id: 'claude-sonnet',
    provider: 'anthropic',
    name: 'claude-sonnet',
    displayName: 'Claude Sonnet',
    capabilities: ['chat', 'vision'],
    officialInputPricePerMillion: 3,
    officialCachedInputPricePerMillion: null,
    officialOutputPricePerMillion: 15,
    active: true,
  },
  {
    id: 'claude-opus',
    provider: 'anthropic',
    name: 'claude-opus',
    displayName: 'Claude Opus',
    capabilities: ['chat', 'vision', 'reasoning'],
    officialInputPricePerMillion: 15,
    officialCachedInputPricePerMillion: null,
    officialOutputPricePerMillion: 75,
    active: true,
  },
  {
    id: 'gemini-flash',
    provider: 'other',
    name: 'gemini-flash',
    displayName: 'Gemini Flash',
    capabilities: ['chat', 'vision'],
    officialInputPricePerMillion: 0.1,
    officialCachedInputPricePerMillion: 0.025,
    officialOutputPricePerMillion: 0.4,
    active: true,
  },
]

export const apiServices: ApiService[] = [
  {
    id: 'a1',
    title: 'GPT / Claude API 服务',
    merchantId: 'merchant-orbit',
    merchantUsername: 'orbit',
    merchant: 'orbit',
    merchantIdentityMode: 'store_alias',
    merchantDisplayName: '小葵 API',
    trustLevel: 3,
    merchantType: '个人车主',
    models: ['GPT-5 mini', 'GPT-5.5', 'Claude Sonnet'],
    modelMultipliers: [{ model: 'GPT-5 mini', multiplier: '1.00x' }, { model: 'GPT-5.5', multiplier: '1.00x' }, { model: 'Claude Sonnet', multiplier: '1.00x' }],
    rate: '1.00x',
    defaultMultiplier: 1,
    creditPerCny: 1,
    minimumPurchaseCny: 20,
    maxBuy: 300,
    balance: 320,
    delivery: 'Sub2API',
    billingMode: 'metered_credit',
    deliveryModes: ['api_key_endpoint', 'sub2api_panel_account'],
    usageVisibility: 'panel_realtime',
    panelBaseUrl: 'https://panel.sub2api.example.dev',
    imagePricing: { supported: true, textToImage: true, imageToImage: true, oneKPriceUsd: 0.134, twoKPriceUsd: 0.201, fourKPriceUsd: 0.268 },
    independentApiKey: true,
    independentPanelAccount: true,
    panelRequiresPasswordReset: true,
    apiBaseUrlVisibility: 'after_intent',
    panelLoginUrlVisibility: 'after_intent',
    publicApiBaseUrl: '购买意向创建后显示服务地址说明',
    state: 'online',
    online: true,
    publiclyOrderable: true,
    lastOnlineConfirmedAt: '2026-06-19 16:28',
    onlineExpiresAt: '2026-06-19 17:28',
    expectedResponseMinutes: 3,
    responseMedianMinutes: 3,
    dailyOrderLimit: 8,
    todayOrderCount: 3,
    unresolvedDisputes: 0,
    warranty: '商户承诺：24 小时不可用补偿',
    refundPolicy: '额度未开始使用时可协商取消',
    expiresAt: '2026-07-01',
    completed30d: 12,
    reviewCount: 9,
    officialPricingVersion: '2026-06',
    officialPricingUpdatedAt: '2026-06-18',
    merchantNote: '建议首次提交 ¥20 意向测试。站外只允许确认买家专属、可撤销的子账号或子 Key；禁止共享主账号、主 Key、Session、Cookie 或第三方 Token。高峰期部分模型可能短时排队，维护状态会在商户面板公告。',
    modelPriceRows: [
      {
        modelId: 'gpt-5-mini',
        modelName: 'GPT-5 mini',
        provider: 'OpenAI',
        officialInputPricePerMillion: 0.25,
        officialCachedInputPricePerMillion: 0.025,
        officialOutputPricePerMillion: 2,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 0.25,
        actualCachedInputPricePerMillion: 0.025,
        actualOutputPricePerMillion: 2,
      },
      {
        modelId: 'gpt-5-5',
        modelName: 'GPT-5.5',
        provider: 'OpenAI',
        officialInputPricePerMillion: 1.75,
        officialCachedInputPricePerMillion: 0.175,
        officialOutputPricePerMillion: 14,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 1.75,
        actualCachedInputPricePerMillion: 0.175,
        actualOutputPricePerMillion: 14,
      },
      {
        modelId: 'claude-sonnet',
        modelName: 'Claude Sonnet',
        provider: 'Anthropic',
        officialInputPricePerMillion: 3,
        officialCachedInputPricePerMillion: null,
        officialOutputPricePerMillion: 15,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 3,
        actualCachedInputPricePerMillion: null,
        actualOutputPricePerMillion: 15,
      },
    ],
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }, { type: 'telegram', label: 'Telegram', value: '@xiaokui_api' }],
  },
  {
    id: 'a2',
    title: '轻量模型开发额度',
    merchantId: 'merchant-qingning',
    merchantUsername: 'qingning',
    merchant: '青柠',
    merchantIdentityMode: 'public_profile',
    merchantDisplayName: '青柠',
    trustLevel: 3,
    merchantType: '可信新车主',
    models: ['GPT mini', 'Gemini Flash'],
    modelMultipliers: [{ model: 'GPT mini', multiplier: '0.50x' }, { model: 'Gemini Flash', multiplier: '0.45x' }],
    rate: '0.45x',
    defaultMultiplier: 0.45,
    creditPerCny: 1,
    minimumPurchaseCny: 10,
    maxBuy: 120,
    balance: 86,
    delivery: 'NewAPI Proxy',
    billingMode: 'fixed_package',
    deliveryModes: ['api_key_endpoint'],
    usageVisibility: 'merchant_readonly',
    panelBaseUrl: null,
    imagePricing: { supported: false, textToImage: false, imageToImage: false, oneKPriceUsd: null, twoKPriceUsd: null, fourKPriceUsd: null },
    independentApiKey: true,
    independentPanelAccount: false,
    panelRequiresPasswordReset: false,
    apiBaseUrlVisibility: 'public',
    panelLoginUrlVisibility: 'off_platform',
    publicApiBaseUrl: 'https://api.example.dev/v1',
    state: 'online',
    online: true,
    publiclyOrderable: true,
    lastOnlineConfirmedAt: '2026-06-19 16:10',
    onlineExpiresAt: '2026-06-19 17:10',
    expectedResponseMinutes: 3,
    responseMedianMinutes: 4,
    dailyOrderLimit: 5,
    todayOrderCount: 1,
    unresolvedDisputes: 0,
    warranty: '接口不可用按天补',
    refundPolicy: '未使用额度可按剩余比例协商',
    expiresAt: '2026-06-30',
    completed30d: 3,
    reviewCount: 3,
    officialPricingVersion: '2026-06',
    officialPricingUpdatedAt: '2026-06-18',
    merchantNote: '适合轻量开发和测试用途。建议先小额确认响应速度和用量查看方式，批量使用前请先在意向记录中和商户确认当前剩余额度。',
    modelPriceRows: [
      {
        modelId: 'gpt-mini',
        modelName: 'GPT mini',
        provider: 'OpenAI',
        officialInputPricePerMillion: 0.15,
        officialCachedInputPricePerMillion: 0.015,
        officialOutputPricePerMillion: 0.6,
        merchantMultiplier: 0.5,
        actualInputPricePerMillion: 0.075,
        actualCachedInputPricePerMillion: 0.008,
        actualOutputPricePerMillion: 0.3,
      },
      {
        modelId: 'gemini-flash',
        modelName: 'Gemini Flash',
        provider: 'Google',
        officialInputPricePerMillion: 0.1,
        officialCachedInputPricePerMillion: 0.025,
        officialOutputPricePerMillion: 0.4,
        merchantMultiplier: 0.45,
        actualInputPricePerMillion: 0.045,
        actualCachedInputPricePerMillion: 0.011,
        actualOutputPricePerMillion: 0.18,
      },
    ],
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@qingning' }],
  },
  {
    id: 'a3',
    title: '多模型备用池',
    merchantId: 'merchant-beifeng',
    merchantUsername: 'beifeng-api',
    merchant: '北风商户',
    merchantIdentityMode: 'public_profile',
    merchantDisplayName: '北风商户',
    trustLevel: 4,
    merchantType: '商户',
    models: ['GPT', 'Claude', 'Gemini'],
    modelMultipliers: [{ model: 'GPT', multiplier: '1.00x' }, { model: 'Claude', multiplier: '1.00x' }, { model: 'Gemini', multiplier: '1.00x' }],
    rate: '1.00x',
    defaultMultiplier: 1,
    creditPerCny: 1,
    minimumPurchaseCny: 50,
    maxBuy: 1000,
    balance: 1200,
    delivery: 'Sub2API',
    billingMode: 'metered_credit',
    deliveryModes: ['sub2api_panel_account'],
    usageVisibility: 'merchant_readonly',
    panelBaseUrl: 'https://panel.example.dev',
    imagePricing: { supported: false, textToImage: false, imageToImage: false, oneKPriceUsd: null, twoKPriceUsd: null, fourKPriceUsd: null },
    independentApiKey: false,
    independentPanelAccount: true,
    panelRequiresPasswordReset: true,
    apiBaseUrlVisibility: 'off_platform',
    panelLoginUrlVisibility: 'public',
    publicPanelLoginUrl: 'https://panel.example.dev',
    state: 'paused',
    online: false,
    publiclyOrderable: false,
    lastOnlineConfirmedAt: '2026-06-19 14:10',
    onlineExpiresAt: '2026-06-19 15:10',
    expectedResponseMinutes: 5,
    responseMedianMinutes: 9,
    dailyOrderLimit: 16,
    todayOrderCount: 6,
    unresolvedDisputes: 1,
    warning: '近期有未响应记录',
    warranty: '售后协商',
    refundPolicy: '异常情况人工协商',
    expiresAt: '2026-07-15',
    completed30d: 25,
    reviewCount: 18,
    officialPricingVersion: '2026-06',
    officialPricingUpdatedAt: '2026-06-18',
    merchantNote: '备用池覆盖多模型，适合有冗余要求的开发场景。暂停接单期间仅展示规则快照，恢复在线后再提交意向。',
    modelPriceRows: [
      {
        modelId: 'gpt',
        modelName: 'GPT',
        provider: 'OpenAI',
        officialInputPricePerMillion: 2,
        officialCachedInputPricePerMillion: 0.2,
        officialOutputPricePerMillion: 8,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 2,
        actualCachedInputPricePerMillion: 0.2,
        actualOutputPricePerMillion: 8,
      },
      {
        modelId: 'claude',
        modelName: 'Claude',
        provider: 'Anthropic',
        officialInputPricePerMillion: 3,
        officialCachedInputPricePerMillion: null,
        officialOutputPricePerMillion: 15,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 3,
        actualCachedInputPricePerMillion: null,
        actualOutputPricePerMillion: 15,
      },
      {
        modelId: 'gemini',
        modelName: 'Gemini',
        provider: 'Google',
        officialInputPricePerMillion: 0.35,
        officialCachedInputPricePerMillion: 0.0875,
        officialOutputPricePerMillion: 1.05,
        merchantMultiplier: 1,
        actualInputPricePerMillion: 0.35,
        actualCachedInputPricePerMillion: 0.0875,
        actualOutputPricePerMillion: 1.05,
      },
    ],
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@beifeng-api' }, { type: 'email', label: '邮箱', value: 'support@example.dev' }],
  },
]

export const apiPurchaseIntents: ApiPurchaseIntent[] = [
  {
    id: 'api-intent-1001',
    serviceId: 'a1',
    buyerId: 'buyer-demo-user',
    buyer: 'demo_user',
    merchantId: 'merchant-orbit',
    merchant: '小葵 API',
    status: 'open',
    selectedDeliveryMode: 'api_key_endpoint',
    purchaseAmountCny: 80,
    purchasedCredit: 80,
    targetModel: 'GPT-5 mini',
    buyerNote: '开发测试额度',
    snapshot: {
      serviceId: 'a1',
      serviceTitle: 'GPT / Claude API 服务',
      merchantId: 'merchant-orbit',
      merchant: 'orbit',
      merchantUsername: 'orbit',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: '小葵 API',
      trustLevel: 3,
      merchantType: '个人车主',
      models: ['GPT-5 mini', 'GPT-5.5', 'Claude Sonnet'],
      multiplier: '1x 起',
      defaultMultiplier: 1,
      creditPerCny: 1,
      warranty: '商户承诺：24 小时不可用补偿',
      refundPolicy: '额度未开始使用时可协商取消',
      usageVisibility: 'panel_realtime',
      supportedDeliveryModes: ['api_key_endpoint', 'sub2api_panel_account'],
      selectedDeliveryMode: 'api_key_endpoint',
      minimumPurchaseCny: 20,
      panelBaseUrl: 'https://panel.sub2api.example.dev',
      apiBaseUrlVisibility: 'after_intent',
      panelLoginUrlVisibility: 'after_intent',
      panelRequiresPasswordReset: true,
      expiresAt: '2026-07-01',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[0].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-1001',
      selectedDeliveryMode: 'api_key_endpoint',
      offPlatformContactChannel: '微信',
      status: 'not_started',
      requiresFirstLoginPasswordReset: false,
      note: '购买意向已创建，商户联系方式已向买家展示，商户可查看买家选择的联系方式',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }, { type: 'telegram', label: 'Telegram', value: '@xiaokui_api' }],
    merchantResponseDeadline: '2026-06-19 16:33',
    createdAt: '2026-06-19 16:30',
    updatedAt: '2026-06-19 16:32',
  },
  {
    id: 'api-intent-1002',
    serviceId: 'a1',
    buyerId: 'buyer-muzhou',
    buyer: '木舟',
    merchantId: 'merchant-orbit',
    merchant: '小葵 API',
    status: 'contacted',
    selectedDeliveryMode: 'sub2api_panel_account',
    purchaseAmountCny: 120,
    purchasedCredit: 120,
    targetModel: 'Claude Sonnet',
    snapshot: {
      serviceId: 'a1',
      serviceTitle: 'GPT / Claude API 服务',
      merchantId: 'merchant-orbit',
      merchant: 'orbit',
      merchantUsername: 'orbit',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: '小葵 API',
      trustLevel: 3,
      merchantType: '个人车主',
      models: ['GPT-5 mini', 'GPT-5.5', 'Claude Sonnet'],
      multiplier: '1x 起',
      defaultMultiplier: 1,
      creditPerCny: 1,
      warranty: '商户承诺：24 小时不可用补偿',
      refundPolicy: '额度未开始使用时可协商取消',
      usageVisibility: 'panel_realtime',
      supportedDeliveryModes: ['api_key_endpoint', 'sub2api_panel_account'],
      selectedDeliveryMode: 'sub2api_panel_account',
      minimumPurchaseCny: 20,
      panelBaseUrl: 'https://panel.sub2api.example.dev',
      apiBaseUrlVisibility: 'after_intent',
      panelLoginUrlVisibility: 'after_intent',
      panelRequiresPasswordReset: true,
      expiresAt: '2026-07-01',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[0].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-1002',
      selectedDeliveryMode: 'sub2api_panel_account',
      offPlatformContactChannel: 'Telegram',
      status: 'contacted',
      requiresFirstLoginPasswordReset: true,
      note: '商户已记录已进行站外联系',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }, { type: 'telegram', label: 'Telegram', value: '@xiaokui_api' }],
    merchantResponseDeadline: '2026-06-19 15:53',
    createdAt: '2026-06-19 15:50',
    updatedAt: '2026-06-19 16:01',
  },
  {
    id: 'api-intent-1003',
    serviceId: 'a2',
    buyerId: 'buyer-demo-user',
    buyer: 'demo_user',
    merchantId: 'merchant-qingning',
    merchant: '青柠',
    status: 'contacted',
    selectedDeliveryMode: 'api_key_endpoint',
    purchaseAmountCny: 30,
    purchasedCredit: 30,
    targetModel: 'GPT mini',
    snapshot: {
      serviceId: 'a2',
      serviceTitle: '轻量模型开发额度',
      merchantId: 'merchant-qingning',
      merchant: '青柠',
      merchantUsername: 'qingning',
      merchantIdentityMode: 'public_profile',
      merchantDisplayName: '青柠',
      trustLevel: 3,
      merchantType: '可信新车主',
      models: ['GPT mini', 'Gemini Flash'],
      multiplier: '0.9x 起',
      defaultMultiplier: 0.9,
      creditPerCny: 1,
      warranty: '接口不可用按天补',
      refundPolicy: '未使用额度可按剩余比例协商',
      usageVisibility: 'merchant_readonly',
      supportedDeliveryModes: ['api_key_endpoint'],
      selectedDeliveryMode: 'api_key_endpoint',
      minimumPurchaseCny: 10,
      panelBaseUrl: null,
      apiBaseUrlVisibility: 'public',
      panelLoginUrlVisibility: 'off_platform',
      panelRequiresPasswordReset: false,
      expiresAt: '2026-06-30',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[1].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-1003',
      selectedDeliveryMode: 'api_key_endpoint',
      offPlatformContactChannel: 'linux.do 私信',
      status: 'contacted',
      requiresFirstLoginPasswordReset: false,
      note: '商户已记录已进行站外联系',
    },
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@qingning' }],
    merchantResponseDeadline: '2026-06-19 13:06',
    createdAt: '2026-06-19 13:03',
    updatedAt: '2026-06-19 13:18',
  },
  {
    id: 'api-intent-0998',
    serviceId: 'a1',
    buyerId: 'buyer-demo-user',
    buyer: 'demo_user',
    merchantId: 'merchant-orbit',
    merchant: '小葵 API',
    status: 'owner_closed',
    selectedDeliveryMode: 'sub2api_panel_account',
    purchaseAmountCny: 60,
    purchasedCredit: 60,
    targetModel: 'GPT-5 mini',
    snapshot: {
      serviceId: 'a1',
      serviceTitle: 'GPT / Claude API 服务',
      merchantId: 'merchant-orbit',
      merchant: 'orbit',
      merchantUsername: 'orbit',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: '小葵 API',
      trustLevel: 3,
      merchantType: '个人车主',
      models: ['GPT-5 mini', 'GPT-5.5', 'Claude Sonnet'],
      multiplier: '1x 起',
      defaultMultiplier: 1,
      creditPerCny: 1,
      warranty: '商户承诺：24 小时不可用补偿',
      refundPolicy: '额度未开始使用时可协商取消',
      usageVisibility: 'panel_realtime',
      supportedDeliveryModes: ['api_key_endpoint', 'sub2api_panel_account'],
      selectedDeliveryMode: 'sub2api_panel_account',
      minimumPurchaseCny: 20,
      panelBaseUrl: 'https://panel.sub2api.example.dev',
      apiBaseUrlVisibility: 'after_intent',
      panelLoginUrlVisibility: 'after_intent',
      panelRequiresPasswordReset: true,
      expiresAt: '2026-07-01',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[0].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-0998',
      selectedDeliveryMode: 'sub2api_panel_account',
      offPlatformContactChannel: 'Telegram',
      status: 'closed',
      requiresFirstLoginPasswordReset: true,
      note: '商户已关闭本次意向记录',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }, { type: 'telegram', label: 'Telegram', value: '@xiaokui_api' }],
    merchantResponseDeadline: '2026-06-18 19:23',
    createdAt: '2026-06-18 19:20',
    updatedAt: '2026-06-18 19:52',
    ownerClosedAt: '2026-06-18 19:52',
    ownerCloseReason: '双方已站外沟通，商户关闭本次意向记录。',
  },
  {
    id: 'api-intent-0997',
    serviceId: 'a3',
    buyerId: 'buyer-demo-user',
    buyer: 'demo_user',
    merchantId: 'merchant-beifeng',
    merchant: '北风商户',
    status: 'owner_closed',
    selectedDeliveryMode: 'sub2api_panel_account',
    purchaseAmountCny: 100,
    purchasedCredit: 100,
    targetModel: 'Claude',
    snapshot: {
      serviceId: 'a3',
      serviceTitle: '多模型备用池',
      merchantId: 'merchant-beifeng',
      merchant: '北风商户',
      merchantUsername: 'beifeng-api',
      merchantIdentityMode: 'public_profile',
      merchantDisplayName: '北风商户',
      trustLevel: 4,
      merchantType: '商户',
      models: ['GPT', 'Claude', 'Gemini'],
      multiplier: '1.00x',
      defaultMultiplier: 1,
      creditPerCny: 1,
      warranty: '售后协商',
      refundPolicy: '异常情况人工协商',
      usageVisibility: 'merchant_readonly',
      supportedDeliveryModes: ['sub2api_panel_account'],
      selectedDeliveryMode: 'sub2api_panel_account',
      minimumPurchaseCny: 50,
      panelBaseUrl: 'https://panel.example.dev',
      apiBaseUrlVisibility: 'off_platform',
      panelLoginUrlVisibility: 'public',
      panelRequiresPasswordReset: true,
      expiresAt: '2026-07-15',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[2].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-0997',
      selectedDeliveryMode: 'sub2api_panel_account',
      status: 'not_started',
      requiresFirstLoginPasswordReset: true,
      note: '商户关闭该购买意向',
    },
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@beifeng-api' }],
    merchantResponseDeadline: '2026-06-18 16:03',
    createdAt: '2026-06-18 16:00',
    updatedAt: '2026-06-18 16:12',
    ownerClosedAt: '2026-06-18 16:12',
    ownerCloseReason: '商户暂不继续处理该购买意向。',
  },
  {
    id: 'api-intent-0996',
    serviceId: 'a3',
    buyerId: 'buyer-muzhou',
    buyer: '木舟',
    merchantId: 'merchant-beifeng',
    merchant: '北风商户',
    status: 'buyer_cancelled',
    selectedDeliveryMode: 'sub2api_panel_account',
    purchaseAmountCny: 200,
    purchasedCredit: 200,
    targetModel: 'GPT',
    snapshot: {
      serviceId: 'a3',
      serviceTitle: '多模型备用池',
      merchantId: 'merchant-beifeng',
      merchant: '北风商户',
      merchantUsername: 'beifeng-api',
      merchantIdentityMode: 'public_profile',
      merchantDisplayName: '北风商户',
      trustLevel: 4,
      merchantType: '商户',
      models: ['GPT', 'Claude', 'Gemini'],
      multiplier: '1.00x',
      defaultMultiplier: 1,
      creditPerCny: 1,
      warranty: '售后协商',
      refundPolicy: '异常情况人工协商',
      usageVisibility: 'merchant_readonly',
      supportedDeliveryModes: ['sub2api_panel_account'],
      selectedDeliveryMode: 'sub2api_panel_account',
      minimumPurchaseCny: 50,
      panelBaseUrl: 'https://panel.example.dev',
      apiBaseUrlVisibility: 'off_platform',
      panelLoginUrlVisibility: 'public',
      panelRequiresPasswordReset: true,
      expiresAt: '2026-07-15',
      officialPricingVersion: '2026-06',
      officialPricingUpdatedAt: '2026-06-18',
      modelPrices: apiServices[2].modelPriceRows.map(row => ({ ...row })),
    },
    handoff: {
      intentId: 'api-intent-0996',
      selectedDeliveryMode: 'sub2api_panel_account',
      offPlatformContactChannel: '邮箱',
      status: 'closed',
      requiresFirstLoginPasswordReset: true,
      note: '买家取消该购买意向',
    },
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@beifeng-api' }, { type: 'email', label: '邮箱', value: 'support@example.dev' }],
    merchantResponseDeadline: '2026-06-17 21:18',
    createdAt: '2026-06-17 21:15',
    updatedAt: '2026-06-17 22:05',
    buyerCancelledAt: '2026-06-17 22:05',
    buyerCancelReason: '买家不再继续该购买意向。',
  },
]

export const apiPurchaseIntentEvents: ApiPurchaseIntentEvent[] = [
  { id: 'api-event-1', intentId: 'api-intent-1001', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 80, deliveryMode: 'api_key_endpoint' }, createdAt: '2026-06-19 16:30' },
  { id: 'api-event-2', intentId: 'api-intent-1002', actorId: 'buyer-muzhou', actorLabel: '木舟', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 120, deliveryMode: 'sub2api_panel_account' }, createdAt: '2026-06-19 15:50' },
  { id: 'api-event-3', intentId: 'api-intent-1002', actorId: 'merchant-orbit', actorLabel: 'orbit', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: 'Telegram' }, createdAt: '2026-06-19 16:01' },
  { id: 'api-event-4', intentId: 'api-intent-1003', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 30, deliveryMode: 'api_key_endpoint' }, createdAt: '2026-06-19 13:03' },
  { id: 'api-event-5', intentId: 'api-intent-1003', actorId: 'merchant-qingning', actorLabel: '青柠', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: 'linux.do 私信' }, createdAt: '2026-06-19 13:18' },
  { id: 'api-event-6', intentId: 'api-intent-0998', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 60, deliveryMode: 'sub2api_panel_account' }, createdAt: '2026-06-18 19:20' },
  { id: 'api-event-7', intentId: 'api-intent-0998', actorId: 'merchant-orbit', actorLabel: 'orbit', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: 'Telegram' }, createdAt: '2026-06-18 19:28' },
  { id: 'api-event-8', intentId: 'api-intent-0998', actorId: 'merchant-orbit', actorLabel: 'orbit', actorRole: 'merchant', type: 'owner_closed', fromStatus: 'contacted', toStatus: 'owner_closed', createdAt: '2026-06-18 19:52' },
  { id: 'api-event-9', intentId: 'api-intent-0997', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 100, deliveryMode: 'sub2api_panel_account' }, createdAt: '2026-06-18 16:00' },
  { id: 'api-event-10', intentId: 'api-intent-0997', actorId: 'merchant-beifeng', actorLabel: '北风商户', actorRole: 'merchant', type: 'owner_closed', fromStatus: 'open', toStatus: 'owner_closed', createdAt: '2026-06-18 16:12' },
  { id: 'api-event-11', intentId: 'api-intent-0996', actorId: 'buyer-muzhou', actorLabel: '木舟', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 200, deliveryMode: 'sub2api_panel_account' }, createdAt: '2026-06-17 21:15' },
  { id: 'api-event-12', intentId: 'api-intent-0996', actorId: 'buyer-muzhou', actorLabel: '木舟', actorRole: 'buyer', type: 'buyer_cancelled', fromStatus: 'open', toStatus: 'buyer_cancelled', createdAt: '2026-06-17 22:05' },
]

export const publicMerchantProfiles: PublicMerchantProfile[] = [
  {
    username: 'orbit',
    displayName: 'orbit',
    avatarText: 'O',
    merchantId: 'merchant-orbit',
    identity: '个人商户',
    trustLevel: 3,
    linuxdoBound: true,
    originalPostBound: true,
    joinedAt: '2025-11-18',
    lastActiveAt: '12 分钟前',
    linuxdoUrl: 'https://linux.do/u/orbit',
    completed30d: 6,
    responseMedianMinutes: 3,
    merchantResponsibleCancellations: 0,
    unresolvedDisputes: 0,
    handledDisputes90d: 1,
  },
  {
    username: 'qingning',
    displayName: '青柠',
    avatarText: '青',
    merchantId: 'merchant-qingning',
    identity: '可信新商户',
    trustLevel: 3,
    linuxdoBound: true,
    originalPostBound: true,
    joinedAt: '2026-04-09',
    lastActiveAt: '28 分钟前',
    linuxdoUrl: 'https://linux.do/u/qingning',
    completed30d: 3,
    responseMedianMinutes: 4,
    merchantResponsibleCancellations: 0,
    unresolvedDisputes: 0,
    handledDisputes90d: 0,
  },
  {
    username: 'beifeng-api',
    displayName: '北风商户',
    avatarText: '北',
    merchantId: 'merchant-beifeng',
    identity: 'API 商户',
    trustLevel: 4,
    linuxdoBound: true,
    originalPostBound: false,
    joinedAt: '2025-08-26',
    lastActiveAt: '2 小时前',
    linuxdoUrl: 'https://linux.do/u/beifeng-api',
    completed30d: 25,
    responseMedianMinutes: 9,
    merchantResponsibleCancellations: 1,
    unresolvedDisputes: 1,
    handledDisputes90d: 3,
  },
]

export const publicUserProfiles: PublicUserProfile[] = [
  {
    id: 'user-orbit',
    username: 'orbit',
    displayName: 'orbit',
    bio: '个人车主和 API 商户，偏好小额测试后再长期合作。',
    avatarUrl: null,
    avatarText: 'O',
    linuxDoBound: true,
    linuxDoUsername: 'orbit',
    trustLevel: 4,
    badges: myUserProfile.badges,
    accountStatus: 'normal',
    createdAt: myUserProfile.privacy.showCreatedAt ? '2025-11-18' : null,
    lastActiveAt: myUserProfile.privacy.showLastActiveAt ? '12 分钟前' : null,
    stats: {
      completedCarpoolsLast30Days: 2,
      completedApiOrdersLast30Days: 6,
      responseMedianMinutes: 3,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 0,
      unresolvedDisputeCount: 0,
      resolvedDisputeCountLast90Days: 1,
    },
    privacy: { ...myUserProfile.privacy },
  },
  {
    id: 'user-qingning',
    username: 'qingning',
    displayName: '青柠',
    bio: '轻量模型额度和订阅车源，优先站内确认规则。',
    avatarUrl: null,
    avatarText: '青',
    linuxDoBound: true,
    linuxDoUsername: 'qingning',
    trustLevel: 3,
    badges: [
      { id: 'badge-qingning-linuxdo', code: 'linuxdo_bound', label: '已绑定 linux.do', type: 'system' },
      { id: 'badge-qingning-owner', code: 'trusted_new_owner', label: '可信新车主', type: 'identity' },
    ],
    accountStatus: 'normal',
    createdAt: '2026-04-09',
    lastActiveAt: '28 分钟前',
    stats: {
      completedCarpoolsLast30Days: 1,
      completedApiOrdersLast30Days: 3,
      responseMedianMinutes: 4,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 0,
      unresolvedDisputeCount: 0,
      resolvedDisputeCountLast90Days: 0,
    },
    privacy: {
      showCreatedAt: true,
      showLastActiveAt: true,
      showCompletionStats: true,
      showResponseMedian: true,
      showResolvedDisputeSummary: true,
      allowPublicProfileReport: true,
    },
  },
  {
    id: 'user-beifeng-api',
    username: 'beifeng-api',
    displayName: '北风商户',
    bio: '多模型备用额度，当前部分服务暂停接单。',
    avatarUrl: null,
    avatarText: '北',
    linuxDoBound: true,
    linuxDoUsername: 'beifeng-api',
    trustLevel: 4,
    badges: [
      { id: 'badge-beifeng-linuxdo', code: 'linuxdo_bound', label: '已绑定 linux.do', type: 'system' },
      { id: 'badge-beifeng-api', code: 'api_merchant', label: 'API 商户', type: 'merchant' },
    ],
    accountStatus: 'under_review',
    createdAt: '2025-08-26',
    lastActiveAt: '2 小时前',
    stats: {
      completedCarpoolsLast30Days: 0,
      completedApiOrdersLast30Days: 25,
      responseMedianMinutes: 9,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 1,
      unresolvedDisputeCount: 1,
      resolvedDisputeCountLast90Days: 3,
    },
    privacy: {
      showCreatedAt: true,
      showLastActiveAt: true,
      showCompletionStats: true,
      showResponseMedian: true,
      showResolvedDisputeSummary: true,
      allowPublicProfileReport: true,
    },
  },
]

export const publicCompletionRecords: PublicCompletionRecord[] = [
  { id: 'complete-orbit-1', username: 'orbit', date: '2026-06-18', serviceType: 'GPT / Claude API 服务', deliveryMode: 'sub2api_panel_account', amountRange: '¥50-100', status: '平台确认完成' },
  { id: 'complete-orbit-2', username: 'orbit', date: '2026-06-16', serviceType: 'GPT mini API 服务', deliveryMode: 'api_key_endpoint', amountRange: '¥20-50', status: '平台确认完成' },
  { id: 'complete-orbit-3', username: 'orbit', date: '2026-06-12', serviceType: 'Claude Sonnet API 服务', deliveryMode: 'sub2api_panel_account', amountRange: '¥100-200', status: '平台确认完成' },
  { id: 'complete-qingning-1', username: 'qingning', date: '2026-06-19', serviceType: '轻量模型开发额度', deliveryMode: 'api_key_endpoint', amountRange: '¥20-50', status: '平台确认完成' },
  { id: 'complete-beifeng-1', username: 'beifeng-api', date: '2026-06-15', serviceType: '多模型备用池', deliveryMode: 'sub2api_panel_account', amountRange: '¥100-300', status: '平台确认完成' },
  { id: 'complete-beifeng-2', username: 'beifeng-api', date: '2026-06-11', serviceType: 'GPT 备用 API 服务', deliveryMode: 'sub2api_panel_account', amountRange: '¥50-100', status: '平台确认完成' },
]

export const publicReviewRecords: PublicReviewRecord[] = [
  { id: 'review-orbit-1', username: 'orbit', date: '2026-06-18', serviceType: 'GPT / Claude API 服务', tags: ['响应及时', '说明清楚', '核对顺畅'], note: '站外确认节奏清楚，用量核对说明充分。', verified: true },
  { id: 'review-orbit-2', username: 'orbit', date: '2026-06-12', serviceType: 'Claude Sonnet API 服务', tags: ['倍率一致', '售后正常'], note: '倍率和页面说明一致。', verified: true },
  { id: 'review-qingning-1', username: 'qingning', date: '2026-06-19', serviceType: '轻量模型开发额度', tags: ['响应及时', '倍率一致'], note: '记录较少，但本单信息清楚。', verified: true },
  { id: 'review-beifeng-1', username: 'beifeng-api', date: '2026-06-15', serviceType: '多模型备用池', tags: ['响应较慢', '用量不透明'], note: '已完成交易，用量展示需要提前说明。', verified: true },
]

export const publicDisputeRecords: PublicDisputeRecord[] = [
  { id: 'dispute-orbit-1', username: 'orbit', type: '响应超时', result: '已补偿等值额度，记录关闭', handledAt: '2026-05-28', unresolved: false },
  { id: 'dispute-beifeng-1', username: 'beifeng-api', type: '用量核对说明不一致', result: '处理中，服务已暂停接单', handledAt: '2026-06-17', unresolved: true },
  { id: 'dispute-beifeng-2', username: 'beifeng-api', type: '站外确认信息缺失', result: '商户补充说明后关闭', handledAt: '2026-05-31', unresolved: false },
]

export const orderContactSnapshots: OrderContactSnapshot[] = [
  {
    id: 'contact-snapshot-ride-app-2',
    orderType: 'carpool_application',
    orderId: 'ride-app-2',
    sellerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'c2c_***', displayValue: 'c2c_orbit', verified: false, usageScope: 'carpool_owner' },
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: '@orbit', displayValue: '@orbit', verified: true, usageScope: 'carpool_owner', actionUrl: 'https://linux.do/u/orbit/messages/new' },
    ],
    buyerContacts: [
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: '@muzhou', displayValue: '@muzhou', verified: true, usageScope: 'buyer', actionUrl: 'https://linux.do/u/muzhou/messages/new' },
    ],
    contactWindowEndsAt: '2026-06-19 17:05',
    canView: true,
    unavailableReason: null,
    createdAt: '2026-06-19 16:35',
  },
  {
    id: 'contact-snapshot-ride-app-3',
    orderType: 'carpool_application',
    orderId: 'ride-app-3',
    sellerContacts: [
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: '@qingning', displayValue: '@qingning', verified: true, usageScope: 'carpool_owner', actionUrl: 'https://linux.do/u/qingning/messages/new' },
    ],
    buyerContacts: [
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: '@demo_user', displayValue: '@demo_user', verified: true, usageScope: 'buyer', actionUrl: 'https://linux.do/u/demo_user/messages/new' },
    ],
    contactWindowEndsAt: '2026-06-18 20:42',
    canView: true,
    unavailableReason: null,
    createdAt: '2026-06-18 20:12',
  },
]

export const demands = [
  { id: 'd1', title: '求 ChatGPT Business 成员席位', maxPrice: 190, require: '个人车主 / 官方成员席位 / 原帖已绑', poster: '木木', trustLevel: 3, linuxdoPost: '已绑定求车帖', status: '匹配中' },
  { id: 'd2', title: '求 Claude Max 5x 香港区', maxPrice: 90, require: '近期确认 / 可候补', poster: '纸船', trustLevel: 2, linuxdoPost: '已绑定求车帖', status: '匹配中' },
]

export const adminCards = [
  { label: '低价线索待审', value: 12, hint: '含 3 条疑似重复' },
  { label: '车源治理待处理', value: 8, hint: '含下架恢复和高风险字段变更' },
  { label: '在线 API 商户', value: 6, hint: '1 个未响应预警' },
  { label: '未解决纠纷', value: 4, hint: '今日新增 1 条' },
]
