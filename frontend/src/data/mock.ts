import type { ReputationSummary } from '@/types/reputation'
import type { ApiServiceHealthSummary } from '@/types/apiHealth'
import type { ApiQuotaUsagePolicy } from '@/types/apiQuota'
import type { Capability } from '@/lib/capabilities'

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

export type SourceAuthorVerificationStatus =
  | 'not_submitted'
  | 'pending'
  | 'verified'
  | 'mismatch'
  | 'expired'

export type SourceAuthorResourceSummary = {
  status: SourceAuthorVerificationStatus
  verifiedAt?: string
  expiresAt?: string
}

export type Carpool = {
  id: string
  product: string
  region: string
  monthly: number
  serviceMultiplier?: number
  dailyQuotaAmount?: number
  weeklyQuotaAmount?: number
  followsOfficialQuotaReset?: boolean | null
  vpsRegion?: string | null
  supportsMainlandChinaDirectConnection?: boolean | null
  openingChannelCode?: OpeningChannelCode | null
  customOpeningChannel?: string | null
  paymentMethodCode?: PaymentMethodCode | null
  customPaymentMethod?: string | null
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
  ownerUserId?: string
  trustLevel: number | null
  ownerType: '个人车主' | '商户车源' | '可信新车主'
  warranty: '车主承诺' | '售后协商'
  openingMethod: 'Apple Store' | '虚拟卡' | '其他' | '本地卡'
  status: '可上车' | '已满' | '候补' | '暂停' | '审核中'
  confirmedAt: string
  confirmedWithin48h: boolean
  linuxdoBound: boolean | null
  sourceUrl?: string
  sourceAuthorVerification?: SourceAuthorResourceSummary
  sellerReputation?: ReputationSummary | null
  communityIdentities?: CommunityIdentity[]
  hasInfoConflict: boolean
  hasUnresolvedDispute: boolean | null
  distributionMethod: CarpoolDistributionMethod
  distributionMethodNote: string
  providesAdminAccount: boolean
  accessArrangementMode?: CarpoolAccessArrangementMode
  accessArrangementNote?: string
  riskNoticeCode?: string
  riskAcknowledged?: boolean
  applicationEligibility?: CarpoolApplicationEligibility
}

export type CarpoolApplicationEligibilityCode =
  | 'eligible'
  | 'sold_out'
  | 'paused'
  | 'credential_risk'
  | 'owner_action_required'
  | 'already_applied'
  | 'already_member'
  | 'self_owned'

export type CarpoolApplicationEligibility = {
  code: CarpoolApplicationEligibilityCode
  canApply: boolean
  reason: string
  resolutionAction: string
}

export type CarpoolAccessArrangementMode =
  | 'personal_account_cost_share'
  | 'provider_member_invitation'
  | 'owner_managed_access'
  | 'other_off_platform'
  | 'not_allowed'

export type CarpoolDistributionMethod = 'sub2api' | 'account_login' | 'other'

export type CarpoolApplicationStatus =
  | 'pending_owner'
  | 'active'
  | 'rejected'
  | 'cancelled_by_buyer'
  | 'cancelled_by_owner'
  | 'disputed'

export type CarpoolApplicationActorRole = 'buyer' | 'owner' | 'admin' | 'system'
export type CarpoolApplicationEventType =
  | 'application_created'
  | 'owner_accepted'
  | 'owner_rejected'
  | 'cancelled'
  | 'disputed'
  | 'admin_updated'

export type CarpoolCancellationResponsibility = 'buyer' | 'owner' | 'mutual' | 'platform' | 'undetermined'

export type CarpoolSeatSummary = {
  carpoolId: string
  totalSeats: number
  activeMemberCount: number
  occupiedSeatCount: number
  availableSeats: number
}

export type CarpoolApplicationSnapshot = {
  carpoolId: string
  productName: string
  regionName: string
  monthlyPriceCny: number
  serviceMultiplier?: number
  dailyQuotaAmount?: number
  weeklyQuotaAmount?: number
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
  ownerTrustLevel: number | null
  ownerReputation?: ReputationSummary | null
  ownerType: Carpool['ownerType']
  accessArrangementMode?: CarpoolAccessArrangementMode
  accessArrangementNote?: string
  riskNoticeCode?: string
  riskAcknowledged?: boolean
}

export type CarpoolApplicantStats = {
  linuxdoBound: boolean | null
  trustLevel: number | null
  completed30d: number | null
  buyerResponsibleCancellations: number | null
  ownerResponsibleCancellations: number | null
  unresolvedDisputes: number | null
}

export type CarpoolApplication = {
  id: string
  carpoolId: string
  applicantUserId: string
  applicantUsername: string
  applicantStats: CarpoolApplicantStats
  buyerReputation?: ReputationSummary | null
  ownerUserId: string
  ownerUsername: string
  status: CarpoolApplicationStatus
  seatsRequested: number
  snapshot: CarpoolApplicationSnapshot
  startedAt: string | null
  cancellationReasonCode: string | null
  cancellationReasonText: string | null
  responsibility: CarpoolCancellationResponsibility | null
  disputeReason: string | null
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

export type CommunityIdentity = {
  code: 'FOUNDING_USER' | 'BETA_CONTRIBUTOR'
  name: string
  description: string
  grantedAt: string
  source?: 'AUTO' | 'ADMIN' | 'BACKFILL'
  revokedAt?: string | null
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
  communityIdentities: CommunityIdentity[]
  accountStatus: UserAccountStatus
  permissions: Array<'admin'>
  capabilities: Capability[]
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
  communityIdentities: CommunityIdentity[]
  accountStatus: UserAccountStatus
  createdAt: string | null
  lastActiveAt: string | null
  stats: {
    completedCarpools: number | null
    completedApiOrders: number | null
    completedCarpoolsLast90Days: number | null
    completedApiOrdersLast90Days: number | null
    responseMedianMinutes: number | null
    buyerResponsibilityCancellationCount: number | null
    sellerResponsibilityCancellationCount: number | null
    unknownResponsibilityCancellationCount: number | null
    unresolvedDisputeCount: number | null
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

export type ContactReportReasonCode = 'contact_invalid' | 'unreachable' | 'impersonation' | 'other'

export type CreateContactReportRequest = {
  orderType: OrderContactSnapshot['orderType']
  orderId: string
  contactType: ContactMethodType
  reasonCode: ContactReportReasonCode
  note: string
}

export type AdminDirectoryUser = {
  id: string
  username: string
  displayName: string
  linuxdoBound: boolean
  trustLevel: number | null
  isAdmin: boolean
  accountStatus: '正常' | '已暂停' | '已封禁' | '已归档'
  createdAt: string
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

export type OpeningChannelCode = 'web' | 'ios_app_store' | 'google_play' | 'team_seat' | 'other'
export type PaymentMethodCode = 'credit_card' | 'virtual_card' | 'apple_pay' | 'google_pay' | 'app_store_gift_card' | 'google_play_gift_card' | 'paypal' | 'u_card' | 'other'
export type CarpoolWarrantyMode = 'no_warranty' | 'remaining_days_compensation' | 'fixed_days_warranty'
export type ProductPublishPolicy = 'allowed' | 'info_only' | 'blocked'
export type ProductAccessMode = 'personal_account_cost_share' | 'provider_member_invitation' | 'owner_managed_access' | 'other_off_platform'
export type ProviderPolicyStatus = 'known_restricted' | 'possibly_restricted' | 'unknown'
export type ProductRiskLevel = 'normal' | 'elevated' | 'high'
export type ProductQuotaPeriod = 'monthly'

export type CarpoolProductCatalogItem = {
  id: string
  categoryCode: string
  providerCode: string
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
export type ApiServiceSalesView = 'active' | 'expired' | 'paused' | 'draft' | 'all'
export type ApiServiceSalesState = 'selling' | 'upcoming' | 'paused' | 'sold_out' | 'expired' | 'draft' | 'offline' | 'archived'
export type ApiServiceSalesChannelKind = 'flexible_quota' | 'limited_quota'
export type ApiMerchantIdentityMode = 'public_profile' | 'store_alias'
export type ApiTTFTBand = 'under_1s' | '1_to_3s' | '3_to_5s' | '5_to_10s' | 'over_10s'
export type ApiQuotaBatchStatus = 'draft' | 'published' | 'paused' | 'archived'
export type ApiQuotaOfferStatus = 'draft' | 'published' | 'paused' | 'archived'
export type ApiQuotaSaleMode = 'continuous' | 'scheduled'
export type ApiQuotaDeliveryMode = 'manual' | 'preimported'
export type ApiQuotaSourceType = 'sub2api' | 'new_api_proxy' | 'self_hosted' | 'other'
export type ApiQuotaDistributionSystem = 'sub2api' | 'new_api_proxy' | 'other'
export type ApiQuotaSystemSaleSlotState = 'registration_open' | 'registration_closed' | 'active' | 'ended'
export type ApiQuotaOrderabilityCode =
  | 'orderable'
  | 'service_unavailable'
  | 'batch_paused'
  | 'offer_paused'
  | 'not_started'
  | 'round_ended'
  | 'sold_out'
  | 'credential_unavailable'
  | 'fulfillment_confirmation_required'
  | 'batch_expired'
export type ApiPurchaseIntentStatus =
  | 'open'
  | 'contacted'
  | 'ordered'
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

export type ApiServicePackageModel = {
  serviceModelId: string
  modelCatalogId: string
  modelPriceVersionId: string
  modelName: string
  provider: string
  merchantMultiplier: number
}

export type ApiServicePackage = {
  id: string
  name: string
  priceCny: number
  panelAllowance: number
  durationDays: 1 | 3 | 7 | 30
  stockTotal: number
  stockAvailable: number
  description: string
  enabled: boolean
  sortOrder: number
  models: ApiServicePackageModel[]
  quotaUsagePolicy: ApiQuotaUsagePolicy
}

export type ApiServicePackageSnapshot = {
  id: string
  name: string
  priceCny: number
  panelAllowance: number
  durationDays: 1 | 3 | 7 | 30
  description: string
  models: Array<{
    serviceModelId: string
    modelCatalogId: string
    modelPriceVersionId: string
    modelName: string
    merchantMultiplier: number
  }>
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
  provider: string
  name: string
  capabilities: ModelCapability[]
  officialInputPricePerMillion: number | null
  officialCachedInputPricePerMillion: number | null
  officialOutputPricePerMillion: number | null
  active: boolean
  sortOrder?: number
  createdAt?: string
  updatedAt?: string
}

export type ApiContactChannel = {
  type: ContactMethodType
  label: string
  value: string
}

export type ApiQuotaBatch = {
  id: string
  apiServiceId: string
  sourceType: ApiQuotaSourceType
  sourceLabel?: string
  status: ApiQuotaBatchStatus
  declaredTotalUsdAllowance: string
  unallocatedUsdAllowance: string
  saleCutoffAt: string
  expiresAt: string
  sourceConfirmedAt: string
  publishedAt?: string
  version: number
}

export type ApiQuotaOffer = {
  id: string
  batchId: string
  apiServiceId: string
  distributionSystem: ApiQuotaDistributionSystem
  name: string
  usdAllowance: string
  priceCny: string
  cnyPerUsd: string
  modelMultiplier: string
  quotaUsagePolicy: ApiQuotaUsagePolicy
  deliveryMode: ApiQuotaDeliveryMode
  deliveryEtaMinutes: number
  saleMode: ApiQuotaSaleMode
  status: ApiQuotaOfferStatus
  sortOrder: number
  publishedAt?: string
  version: number
}

export type ApiQuotaAllocation = {
  id: string
  offerId: string
  saleRoundId?: string
  saleMode: ApiQuotaSaleMode
  copyLimit: number
  availableCopies: number
  reservedCopies: number
  consumedCopies: number
  allocatedUsdAllowance: string
  returnedUsdAllowance: string
  status: 'planned' | 'active' | 'closed' | 'cancelled'
}

export type ApiQuotaRound = {
  id: string
  batchId: string
  systemSlotKey?: string
  fulfillmentConfirmedAt?: string
  name: string
  startsAt: string
  endsAt: string
  status: 'scheduled' | 'closed' | 'cancelled'
  allocations: ApiQuotaAllocation[]
  version: number
}

export type ApiQuotaSystemSaleSlot = {
  key: string
  startsAt: string
  endsAt: string
  registrationClosesAt: string
  state: ApiQuotaSystemSaleSlotState
}

export type ApiQuotaSystemSaleSlotList = {
  serverNow: string
  items: ApiQuotaSystemSaleSlot[]
}

export type PublicApiQuotaOffer = ApiQuotaOffer & {
  batchStatus: Extract<ApiQuotaBatchStatus, 'published' | 'paused'>
  serviceTitle: string
  sellerDisplayName: string
  sellerIdentityType: 'individual' | 'merchant'
  merchantAvatarUrl?: string
  sellerLinuxDoBound: boolean
  promptAuditEnabled?: boolean | null
  healthSummary?: ApiServiceHealthSummary
  declaredTtftBand?: ApiTTFTBand
  declaredMaxConcurrency: number
  performanceConfirmedAt?: string
  performanceDisclaimer?: '商户自报，平台未测速'
  saleCutoffAt: string
  expiresAt: string
  currentRound?: ApiQuotaRound
  nextRound?: ApiQuotaRound
  availableCopies: number
  credentialAvailableCopies: number
  isOrderable: boolean
  orderabilityCode: ApiQuotaOrderabilityCode
  orderabilityReason: string
}

export type ApiQuotaCredentialSummary = {
  offerId: string
  available: number
  reserved: number
  delivered: number
  retired: number
}

export type ApiService = {
  id: string
  version?: number
  probeConnectionId?: string
  probeReady?: boolean
  title: string
  sourceUrl?: string
  sourceAuthorVerification?: SourceAuthorResourceSummary
  sellerReputation?: ReputationSummary | null
  communityIdentities?: CommunityIdentity[]
  healthSummary?: ApiServiceHealthSummary
  quotaUsagePolicy: ApiQuotaUsagePolicy
  merchantId: string
  merchantUsername: string
  merchant: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
  merchantAvatarUrl?: string
  trustLevel: number | null
  merchantType: '个人车主' | '个人卖家' | '商户' | '可信新车主'
  models: string[]
  modelMultipliers: ApiModelMultiplier[]
  rate: string
  defaultMultiplier: number
  creditPerCny: number
  cnyPerUsdAllowance?: string
  availableUsdAllowance?: string
  maxUsdAllowancePerOrder?: string
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
  declaredTtftBand?: ApiTTFTBand
  declaredMaxConcurrency?: number
  performanceConfirmedAt?: string
  promptAuditEnabled?: boolean | null
  accountPoolType?: 'gpt_pro_20x' | 'gpt_pro_5x' | 'gpt_plus' | 'custom'
  accountPoolLabel?: string
  merchantRefundCommitment?: boolean
  merchantRefundPolicyVersion?: string
  expectedResponseMinutes: number
  responseMedianMinutes: number | null
  dailyOrderLimit: number
  todayOrderCount: number
  unresolvedDisputes: number | null
  warning?: string
  warranty: string
  refundPolicy: string
  quotaExpiresAt?: string
  expiresAt: string
  completed30d: number | null
  reviewCount: number | null
  officialPricingVersion: string
  officialPricingUpdatedAt: string
  merchantNote: string
  modelPriceRows: ModelPriceRow[]
  packages?: ApiServicePackage[]
  recommendationResponseMedianMinutes?: number | null
  serviceUpdatedAt?: string
  contactChannels: ApiContactChannel[]
  acceptedPaymentMethods?: Array<'wechat' | 'alipay'>
}

export type ApiServiceSalesChannel = {
  kind: ApiServiceSalesChannelKind
  state: ApiServiceSalesState
  availableUsdAllowance?: string
  availableCopies?: number
  nextStartsAt?: string
  saleCutoffAt?: string
  expiresAt?: string
}

export type ApiServiceSalesSummary = {
  overallState: ApiServiceSalesState
  channels: ApiServiceSalesChannel[]
}

export type OwnerApiService = ApiService & {
  healthSummary: ApiServiceHealthSummary
  salesSummary: ApiServiceSalesSummary
}

export type PublicMerchantProfile = {
  username: string
  displayName: string
  avatarUrl?: string | null
  avatarText: string
  merchantId: string
  identity: '个人商户' | '可信新商户' | 'API 商户'
  trustLevel: number | null
  linuxdoBound: boolean | null
  originalPostBound: boolean | null
  joinedAt: string
  lastActiveAt: string
  linuxdoUrl: string
  completedLast90Days: number | null
  responseMedianMinutes: number | null
  merchantResponsibleCancellations: number | null
  unresolvedDisputes: number | null
  handledDisputesLast90Days: number | null
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

export type PublicProfileCarpool = {
  id: string
  title: string
  summary: string
  regionName: string
  priceMonthlyCny: string
  availableSeats: number
  updatedAt: string
}

export type PublicProfileAPIService = {
  id: string
  title: string
  shortDescription: string
  billingMode: ApiBillingMode
  availableUsdAllowance: string
  usageVisibility: ApiUsageVisibility
  refundCommitment: boolean
  updatedAt: string
}

export type PublicProfileCompletion = {
  id: string
  kind: 'carpool' | 'api_order'
  title: string
  role: 'buyer' | 'seller'
  completedAt: string
}

export type PublicReviewRecord = {
  id: string
  username: string
  date: string
  serviceType: string
  rating: number
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

export type ApiServiceCommercialSnapshot = {
	warranty?: string
	refundPolicy?: string
  accountPoolType?: ApiService['accountPoolType']
  accountPoolLabel?: string
  declaredMaxConcurrency?: number
  promptAuditEnabled?: boolean | null
  merchantRefundCommitment?: boolean
  merchantRefundPolicyVersion?: string
  serviceValidityExpiresAt?: string | null
  commercialFactsSnapshotIssue?: 'missing' | 'invalid'
}

export type ApiPurchaseIntentSnapshot = ApiServiceCommercialSnapshot & {
  serviceId: string
  serviceTitle: string
  sourceUrl?: string
  merchantId: string
  merchant: string
  merchantUsername: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
  trustLevel: number | null
  merchantType: ApiService['merchantType']
  models: string[]
  multiplier: string
  defaultMultiplier: number
  creditPerCny: number
  cnyPerUsdAllowance?: string
	warranty: string
	refundPolicy: string
  merchantNote?: string
  pricingSnapshotIssue?: 'missing' | 'invalid'
  usageVisibilitySnapshotMissing?: boolean
  usageVisibility: ApiUsageVisibility
  supportedDeliveryModes: ApiDeliveryMode[]
  selectedDeliveryMode: ApiDeliveryMode
  selectedPackageId?: string
  selectedPackageSnapshot?: ApiServicePackageSnapshot
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
  paymentMethod: 'wechat' | 'alipay'
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
  selectedPackageId?: string
  purchaseAmountCny: number
  purchasedCredit: number
  purchaseAmountCnyDecimal?: string
  purchasedCreditDecimal?: string
  quotaUsagePolicySnapshot: ApiQuotaUsagePolicy
  targetModel: string
  buyerNote?: string
  snapshot: ApiPurchaseIntentSnapshot
  handoff: ApiCredentialHandoffRecord
  contactChannels: ApiContactChannel[]
  buyerContactChannels?: ApiContactChannel[]
  viewerRole?: 'buyer' | 'merchant'
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
  { product: 'ChatGPT Plus', detail: '个人订阅费用分摊 / 高风险需确认', verifiedLowest: 108, leadLowest: 18, carpoolCount: 6 },
  { product: 'ChatGPT Business', detail: 'workspace 成员邀请 / 风险需确认', verifiedLowest: 188, leadLowest: 92, carpoolCount: 18 },
  { product: 'ChatGPT Pro 20x Web', detail: '个人订阅费用分摊 / 高风险需确认', verifiedLowest: 988, leadLowest: 56, carpoolCount: 5 },
  { product: 'Claude Max 5x', detail: '5x / 20x 订阅', verifiedLowest: 724, leadLowest: 47, carpoolCount: 14 },
  { product: 'Cursor Pro', detail: '团队席位 / 独立座位', verifiedLowest: 154, leadLowest: 36, carpoolCount: 18 },
]

export const carpoolProductCatalog: CarpoolProductCatalogItem[] = [
  { id: '00000000-0000-0000-0000-000000000303', categoryCode: 'gpt', providerCode: 'openai', displayName: 'ChatGPT Pro 20x Web', slug: 'chatgpt-pro-20x-web', description: '个人订阅费用分摊，高风险需确认。', publishPolicy: 'allowed', accessMode: 'personal_account_cost_share', providerPolicyStatus: 'known_restricted', riskLevel: 'high', riskAckRequired: true, policyVersion: 1, policyNote: 'C2CMarket 当前开放该品类，不代表服务提供商认可。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', riskNoticeCode: 'openai_subscription_carpool', active: true, sortOrder: 30, allowCustomVariant: false, createdAt: '2026-08-14', updatedAt: '2026-08-14' },
  { id: '00000000-0000-0000-0000-000000000401', categoryCode: 'claude', providerCode: 'anthropic', displayName: 'Claude Pro', slug: 'claude-pro', description: '社区 Claude Pro 拼车品类。', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, policyNote: '需说明成员、席位或站外访问安排。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 50, allowCustomVariant: false, createdAt: '2026-08-14', updatedAt: '2026-08-14' },
  { id: '00000000-0000-0000-0000-000000000601', categoryCode: 'grok', providerCode: 'xai', displayName: 'Grok Premium', slug: 'grok-premium', description: '社区 Grok 订阅拼车品类', publishPolicy: 'allowed', accessMode: 'owner_managed_access', providerPolicyStatus: 'unknown', riskLevel: 'elevated', riskAckRequired: false, policyVersion: 1, policyNote: '需说明成员、席位或站外访问安排。', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', active: true, sortOrder: 60, allowCustomVariant: false, createdAt: '2026-08-14', updatedAt: '2026-08-14' },
]

export const carpoolRegions: RegionOption[] = [
  { code: 'ph', displayName: '菲律宾区', active: true, sortOrder: 10 },
  { code: 'tr', displayName: '土耳其区', active: true, sortOrder: 20 },
  { code: 'hk', displayName: '香港区', active: true, sortOrder: 30 },
  { code: 'jp', displayName: '日本区', active: true, sortOrder: 40 },
  { code: 'us', displayName: '美国区', active: true, sortOrder: 50 },
  { code: 'other', displayName: '其他 / 自定义', active: true, sortOrder: 999 },
]

export const carpoolOpeningChannels: OpeningChannelOption[] = [
  { code: 'web', displayName: 'Web 官网', active: true, sortOrder: 10 },
  { code: 'ios_app_store', displayName: 'iOS App Store', active: true, sortOrder: 20 },
  { code: 'google_play', displayName: 'Google Play', active: true, sortOrder: 30 },
  { code: 'team_seat', displayName: 'Team / Business 席位', active: true, sortOrder: 40 },
  { code: 'other', displayName: '其他', active: true, sortOrder: 999 },
]

export const carpoolPaymentMethods: PaymentMethodOption[] = [
  { code: 'credit_card', displayName: '信用卡', active: true, sortOrder: 10 },
  { code: 'virtual_card', displayName: '虚拟卡', active: true, sortOrder: 20 },
  { code: 'apple_pay', displayName: 'Apple Pay', active: true, sortOrder: 30 },
  { code: 'google_pay', displayName: 'Google Pay', active: true, sortOrder: 40 },
  { code: 'app_store_gift_card', displayName: 'App Store 礼品卡', active: true, sortOrder: 50 },
  { code: 'google_play_gift_card', displayName: 'Google Play 礼品卡', active: true, sortOrder: 60 },
  { code: 'paypal', displayName: 'PayPal', active: true, sortOrder: 70 },
  { code: 'u_card', displayName: 'U 卡', active: true, sortOrder: 80 },
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
  communityIdentities: [
    {
      code: 'BETA_CONTRIBUTOR',
      name: '内测共建者',
      description: '帮助平台测试和改进产品的社区成员',
      grantedAt: '2026-06-10T10:00:00+08:00',
      source: 'ADMIN',
    },
  ],
  accountStatus: 'normal',
  permissions: ['admin'],
  capabilities: [
    'admin.access',
    'api_order.create',
    'api_probe.manage',
    'api_quota.publish',
    'api_service.publish',
    'carpool.apply',
    'carpool.publish',
  ],
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
    usageScopes: ['carpool_owner', 'api_merchant', 'buyer', 'dispute'],
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

export const officialPrices: OfficialPrice[] = [
  { id: 'p1', product: 'ChatGPT', plan: 'Plus', region: '土耳其区', channel: 'iOS', openingMethod: 'Apple Store', originalPrice: 'TRY 499', cny: 108, status: '已验证', source: 'linux.do 低价帖', submitter: '青柠', submitterTrust: 3, updatedAt: '12 分钟前', isLowest: true },
  { id: 'p2', product: 'ChatGPT', plan: 'Pro', region: '菲律宾区', channel: 'Web', openingMethod: '虚拟卡', originalPrice: 'PHP 7,990', cny: 988, status: '已验证', source: 'linux.do 低价帖', submitter: 'orbit', submitterTrust: 3, updatedAt: '今天 16:30', isLowest: true },
  { id: 'p3', product: 'Claude', plan: 'Max 5x', region: '香港区', channel: 'Web', openingMethod: '本地卡', originalPrice: 'HKD 780', cny: 724, status: '待验证', source: '用户线索', submitter: '北风', submitterTrust: 2, updatedAt: '2 小时前' },
  { id: 'p4', product: 'Cursor', plan: 'Pro', region: '新加坡区', channel: 'Web', openingMethod: '虚拟卡', originalPrice: 'SGD 28', cny: 154, status: '需复核', source: '官方页面', submitter: '管理员', submitterTrust: 4, updatedAt: '3 天前' },
]

export const carpools: Carpool[] = [
  { id: 'c1', product: 'ChatGPT Business', region: '美国区', monthly: 188, serviceMultiplier: 1, dailyQuotaAmount: 50, weeklyQuotaAmount: 200, followsOfficialQuotaReset: true, vpsRegion: '美国西部', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'team_seat', paymentMethodCode: 'credit_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '3/5', pricingMode: 'fixed', fixedMonthlyPrice: 188, currentConfirmedMembers: 3, maxMembers: 5, settlementDeadline: '2026-06-25', owner: 'orbit', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '其他', status: '可上车', confirmedAt: '12 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'sub2api', distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。', providesAdminAccount: true, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: 'Business workspace 管理员邀请成员席位；不得共享 Pro/Plus 主账号、密码、Session 或 Cookie。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c2', product: 'Cursor Pro', region: '土耳其区', monthly: 68, serviceMultiplier: 1, dailyQuotaAmount: 125, weeklyQuotaAmount: 500, followsOfficialQuotaReset: true, vpsRegion: '香港', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'web', paymentMethodCode: 'u_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '1/6', pricingMode: 'fixed', fixedMonthlyPrice: 68, currentConfirmedMembers: 1, maxMembers: 6, owner: '青柠', trustLevel: 3, ownerType: '个人车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '35 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'sub2api', distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。', providesAdminAccount: true, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: '团队成员邀请或独立席位授权。' },
  { id: 'c3', product: 'Claude Max 5x', region: '香港区', monthly: 80, serviceMultiplier: 1, dailyQuotaAmount: 75, weeklyQuotaAmount: 300, followsOfficialQuotaReset: false, vpsRegion: '香港', supportsMainlandChinaDirectConnection: false, openingChannelCode: 'web', paymentMethodCode: 'paypal', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/4', pricingMode: 'tiered', currentConfirmedMembers: 2, maxMembers: 4, pricingTiers: [{ memberCount: 2, price: 120 }, { memberCount: 3, price: 80 }, { memberCount: 4, price: 60 }], owner: '北风', trustLevel: 4, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '本地卡', status: '审核中', confirmedAt: '1 小时前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'pending' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'other', distributionMethodNote: '站外托管访问，具体方式加入前确认。', providesAdminAccount: false, accessArrangementMode: 'owner_managed_access', accessArrangementNote: '车主站外管理成员访问，不在平台保存凭据。' },
  { id: 'c4', product: 'Cursor Pro', region: '新加坡区', monthly: 39, serviceMultiplier: 1, dailyQuotaAmount: 50, weeklyQuotaAmount: 200, followsOfficialQuotaReset: true, vpsRegion: '新加坡', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'other', customOpeningChannel: '企业席位', paymentMethodCode: 'virtual_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '4/4', pricingMode: 'fixed', fixedMonthlyPrice: 39, currentConfirmedMembers: 4, maxMembers: 4, owner: '周末研究员', trustLevel: 2, ownerType: '商户车源', warranty: '售后协商', openingMethod: '其他', status: '已满', confirmedAt: '今天 09:24', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'other', distributionMethodNote: '商户站外安排，加入前确认。', providesAdminAccount: false },
  { id: 'c5', product: 'ChatGPT Business', region: '日本区', monthly: 198, serviceMultiplier: 1, dailyQuotaAmount: 50, weeklyQuotaAmount: 200, followsOfficialQuotaReset: true, vpsRegion: '日本东京', supportsMainlandChinaDirectConnection: false, openingChannelCode: 'team_seat', paymentMethodCode: 'credit_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/5', pricingMode: 'fixed', fixedMonthlyPrice: 198, currentConfirmedMembers: 2, maxMembers: 5, settlementDeadline: '2026-06-24', owner: '木舟', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '其他', status: '可上车', confirmedAt: '2 小时前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'sub2api', distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。', providesAdminAccount: true, accessArrangementMode: 'provider_member_invitation', accessArrangementNote: 'Business workspace 管理员邀请成员席位。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c6', product: 'ChatGPT Pro 20x Web', region: '香港区', monthly: 178, serviceMultiplier: 1, dailyQuotaAmount: 75, weeklyQuotaAmount: 300, followsOfficialQuotaReset: true, vpsRegion: '香港', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'web', paymentMethodCode: 'u_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '5/6', pricingMode: 'fixed', fixedMonthlyPrice: 178, currentConfirmedMembers: 5, maxMembers: 6, owner: '纸船', trustLevel: 2, ownerType: '可信新车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '今天 10:20', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'expired' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'sub2api', distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。', providesAdminAccount: true, accessArrangementMode: 'personal_account_cost_share', accessArrangementNote: '个人订阅费用分摊，平台不保存、不交付任何密码、Session、Cookie 或 token。', riskNoticeCode: 'openai_subscription_carpool', riskAcknowledged: true },
  { id: 'c7', product: 'Cursor Pro', region: '新加坡区', monthly: 49, serviceMultiplier: 1, dailyQuotaAmount: 125, weeklyQuotaAmount: 500, followsOfficialQuotaReset: true, vpsRegion: '新加坡', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'web', paymentMethodCode: 'virtual_card', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/4', pricingMode: 'tiered', currentConfirmedMembers: 2, maxMembers: 4, pricingTiers: [{ memberCount: 2, price: 69 }, { memberCount: 3, price: 56 }, { memberCount: 4, price: 49 }], owner: '栈帧', trustLevel: 3, ownerType: '个人车主', warranty: '售后协商', openingMethod: '虚拟卡', status: '可上车', confirmedAt: '45 分钟前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'sub2api', distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。', providesAdminAccount: true },
  { id: 'c8', product: 'Perplexity Pro', region: '美国区', monthly: 42, serviceMultiplier: 1, dailyQuotaAmount: 40, weeklyQuotaAmount: 150, followsOfficialQuotaReset: false, vpsRegion: '美国西部', supportsMainlandChinaDirectConnection: false, openingChannelCode: 'web', paymentMethodCode: 'paypal', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '1/3', pricingMode: 'fixed', fixedMonthlyPrice: 42, currentConfirmedMembers: 1, maxMembers: 3, owner: '海盐', trustLevel: 2, ownerType: '可信新车主', warranty: '售后协商', openingMethod: '其他', status: '可上车', confirmedAt: '1 小时前', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'mismatch' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'other', distributionMethodNote: '共享家庭组席位，具体方式站外确认。', providesAdminAccount: false },
  { id: 'c9', product: 'Gemini Advanced', region: '日本区', monthly: 36, serviceMultiplier: 1, dailyQuotaAmount: 25, weeklyQuotaAmount: 100, followsOfficialQuotaReset: true, vpsRegion: '日本东京', supportsMainlandChinaDirectConnection: true, openingChannelCode: 'google_play', paymentMethodCode: 'google_pay', quotaLabel: '额度', quotaUnit: 'USD', quotaPeriod: 'monthly', seats: '2/5', pricingMode: 'equal_share', totalShareableCost: 108, currentConfirmedMembers: 2, maxMembers: 5, settlementDeadline: '2026-06-26', owner: '雨季', trustLevel: 3, ownerType: '个人车主', warranty: '车主承诺', openingMethod: '本地卡', status: '可上车', confirmedAt: '今天 11:05', confirmedWithin48h: true, linuxdoBound: true, sourceAuthorVerification: { status: 'verified' }, hasInfoConflict: false, hasUnresolvedDispute: false, distributionMethod: 'other', distributionMethodNote: 'Google 家庭组成员安排，具体方式站外确认。', providesAdminAccount: false },
]

const carpoolApplicationSnapshots: Record<string, CarpoolApplicationSnapshot> = {
  c1: {
    carpoolId: 'c1',
    productName: 'ChatGPT Business',
    regionName: '美国区',
    monthlyPriceCny: 188,
    serviceMultiplier: 1,
    weeklyQuotaAmount: 200,
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
    accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: 'Business workspace 管理员邀请成员席位。',
  },
  c2: {
    carpoolId: 'c2',
    productName: 'Cursor Pro',
    regionName: '土耳其区',
    monthlyPriceCny: 68,
    serviceMultiplier: 1,
    weeklyQuotaAmount: 500,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    priceLabel: '固定月费',
    openingChannelName: 'Team / Business 席位',
    paymentMethodNames: ['信用卡'],
    warrantyText: '售后协商',
    rulesVersion: '2026-06-19 11:10',
    rulesText: '按月确认成员席位资格，异常情况由双方站外协商处理。',
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    ownerTrustLevel: 3,
    ownerType: '个人车主',
    accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: '团队成员邀请或独立席位授权。',
  },
  c3: {
    carpoolId: 'c3',
    productName: 'Claude Max 5x',
    regionName: '香港区',
    monthlyPriceCny: 80,
    serviceMultiplier: 1,
    weeklyQuotaAmount: 300,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    priceLabel: '当前阶梯价',
    openingChannelName: 'Web 官网',
    paymentMethodNames: ['PayPal'],
    warrantyText: '车主承诺',
    rulesVersion: '2026-06-18 22:40',
    rulesText: '服务周期内保持席位，需提前一天确认续期。',
    ownerUserId: 'owner-beifeng',
    ownerUsername: '北风',
    ownerTrustLevel: 4,
    ownerType: '个人车主',
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
    startedAt: null,
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
    status: 'active',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c1,
    startedAt: '2026-06-19 16:35',
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
    startedAt: '2026-06-18 20:26',
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
    status: 'active',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c3,
    startedAt: '2026-05-19 12:48',
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
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 2, completed30d: 0, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    status: 'cancelled_by_buyer',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c2,
    startedAt: '2026-05-10 10:22',
    cancellationReasonCode: 'buyer_left',
    cancellationReasonText: '买家已退出成员关系。',
    responsibility: 'buyer',
    disputeReason: null,
    createdAt: '2026-05-10 10:00',
    updatedAt: '2026-06-10 12:10',
  },
  {
    id: 'ride-app-7',
    carpoolId: 'c2',
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 3, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    status: 'cancelled_by_owner',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c2,
    startedAt: '2026-07-23 18:22',
    cancellationReasonCode: 'owner_removed',
    cancellationReasonText: '车主已移除该成员。',
    responsibility: 'owner',
    disputeReason: null,
    createdAt: '2026-07-23 17:55',
    updatedAt: '2026-07-24 09:15',
  },
  {
    id: 'ride-app-8',
    carpoolId: 'c3',
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 3, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-beifeng',
    ownerUsername: '北风',
    status: 'cancelled_by_buyer',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c3,
    startedAt: '2026-07-22 11:22',
    cancellationReasonCode: 'buyer_left',
    cancellationReasonText: '买家已退出成员关系。',
    responsibility: 'buyer',
    disputeReason: null,
    createdAt: '2026-07-22 10:55',
    updatedAt: '2026-07-23 12:18',
  },
  {
    id: 'ride-app-9',
    carpoolId: 'c2',
    applicantUserId: 'buyer-demo-user',
    applicantUsername: 'demo_user',
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 3, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: 'owner-qingning',
    ownerUsername: '青柠',
    status: 'cancelled_by_owner',
    seatsRequested: 1,
    snapshot: carpoolApplicationSnapshots.c2,
    startedAt: '2026-06-30 18:22',
    cancellationReasonCode: 'owner_removed',
    cancellationReasonText: '车主已移除该成员。',
    responsibility: 'owner',
    disputeReason: null,
    createdAt: '2026-06-30 17:55',
    updatedAt: '2026-07-01 09:04',
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
    startedAt: '2026-06-16 09:52',
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
  { id: 'ride-event-3', applicationId: 'ride-app-2', actorId: 'owner-orbit', actorLabel: 'orbit', actorRole: 'owner', type: 'owner_accepted', fromStatus: 'pending_owner', toStatus: 'active', note: '车主确认上车，成员关系立即生效。', createdAt: '2026-06-19 16:35' },
  { id: 'ride-event-4', applicationId: 'ride-app-3', actorId: 'owner-qingning', actorLabel: '青柠', actorRole: 'owner', type: 'owner_accepted', fromStatus: 'pending_owner', toStatus: 'active', note: '车主确认上车，成员关系立即生效。', createdAt: '2026-06-18 20:26' },
  { id: 'ride-event-5', applicationId: 'ride-app-5', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'cancelled', fromStatus: 'active', toStatus: 'cancelled_by_buyer', note: '买家已退出成员关系。', createdAt: '2026-06-10 12:04' },
  { id: 'ride-event-6', applicationId: 'ride-app-7', actorId: 'owner-qingning', actorLabel: '青柠', actorRole: 'owner', type: 'cancelled', fromStatus: 'active', toStatus: 'cancelled_by_owner', note: '车主已移除该成员。', createdAt: '2026-07-24 09:04' },
  { id: 'ride-event-8', applicationId: 'ride-app-6', actorId: 'buyer-yuji', actorLabel: '雨季', actorRole: 'buyer', type: 'disputed', fromStatus: 'active', toStatus: 'disputed', note: '买家发起纠纷，等待管理员处理。', createdAt: '2026-06-18 15:30' },
]

export const adminDirectoryUsers: AdminDirectoryUser[] = [
  { id: 'buyer-demo-user', username: 'demo_user', displayName: 'demo_user', linuxdoBound: true, trustLevel: 3, isAdmin: false, accountStatus: '正常', createdAt: '2026-06-01 09:00', lastActiveAt: '刚刚' },
  { id: 'owner-orbit', username: 'orbit', displayName: 'orbit', linuxdoBound: true, trustLevel: 3, isAdmin: false, accountStatus: '正常', createdAt: '2026-06-02 09:00', lastActiveAt: '12 分钟前' },
  { id: 'owner-qingning', username: '青柠', displayName: '青柠', linuxdoBound: true, trustLevel: 3, isAdmin: false, accountStatus: '正常', createdAt: '2026-06-03 09:00', lastActiveAt: '35 分钟前' },
  { id: 'buyer-yuji', username: '雨季', displayName: '雨季', linuxdoBound: false, trustLevel: 0, isAdmin: false, accountStatus: '已暂停', createdAt: '2026-06-04 09:00', lastActiveAt: '昨天 18:20' },
  { id: 'merchant-beifeng', username: 'beifeng-api', displayName: 'beifeng-api', linuxdoBound: true, trustLevel: 2, isAdmin: false, accountStatus: '已暂停', createdAt: '2026-06-05 09:00', lastActiveAt: '今天 09:40' },
  { id: 'user-banned', username: '灰名单用户', displayName: '灰名单用户', linuxdoBound: false, trustLevel: 0, isAdmin: false, accountStatus: '已封禁', createdAt: '2026-06-06 09:00', lastActiveAt: '3 天前' },
]

export const adminAuditLogs: AdminAuditLog[] = [
  { id: 'audit-1', actorType: 'admin', actorLabel: '管理员', action: '审核通过', targetType: 'carpool', targetId: 'c1', targetLabel: 'ChatGPT Business', beforeStatus: '待审核', afterStatus: 'approved_offline', reason: '已声明 Business workspace 成员席位机制与使用边界。', createdAt: '2026-06-19 15:42' },
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
    capabilities: ['chat', 'vision'],
    officialInputPricePerMillion: 0.1,
    officialCachedInputPricePerMillion: 0.025,
    officialOutputPricePerMillion: 0.4,
    active: true,
  },
]

const limitedServiceQuotaPolicy: ApiQuotaUsagePolicy = {
  fiveHour: { mode: 'limited', amountUsd: '50' },
  daily: { mode: 'limited', amountUsd: '200' },
  scope: 'per_buyer_credential',
  dailyReset: 'utc_plus_8_calendar_day',
}

const unlimitedServiceQuotaPolicy: ApiQuotaUsagePolicy = {
  fiveHour: { mode: 'unlimited', amountUsd: null },
  daily: { mode: 'unlimited', amountUsd: null },
  scope: 'per_buyer_credential',
  dailyReset: 'utc_plus_8_calendar_day',
}

const unspecifiedServiceQuotaPolicy: ApiQuotaUsagePolicy = {
  fiveHour: { mode: 'unspecified', amountUsd: null },
  daily: { mode: 'unspecified', amountUsd: null },
  scope: 'per_buyer_credential',
  dailyReset: 'utc_plus_8_calendar_day',
}

export const apiServices: ApiService[] = [
  {
    id: 'a1',
    title: 'GPT / Claude API 服务',
    sourceUrl: 'https://linux.do/t/api-quota-sub2api/123456',
    sourceAuthorVerification: { status: 'verified' },
    quotaUsagePolicy: limitedServiceQuotaPolicy,
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
    acceptedPaymentMethods: ['wechat', 'alipay'],
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
    merchantNote: '建议首次提交 ¥20 意向测试。站外只允许确认买家专属的子账号或子 Key；禁止共享主账号、主 Key、Session、Cookie 或第三方 Token。高峰期部分模型可能短时排队，维护状态会在商户面板公告。',
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
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }],
  },
  {
    id: 'a2',
    title: '轻量模型开发额度',
    sourceUrl: '',
    sourceAuthorVerification: { status: 'not_submitted' },
    quotaUsagePolicy: unlimitedServiceQuotaPolicy,
    merchantId: 'merchant-qingning',
    merchantUsername: 'qingning',
    merchant: '青柠',
    merchantIdentityMode: 'public_profile',
    merchantDisplayName: '青柠',
    trustLevel: 3,
    merchantType: '可信新车主',
    models: ['GPT-5.5', 'Gemini Flash'],
    modelMultipliers: [{ model: 'GPT-5.5', multiplier: '0.50x' }, { model: 'Gemini Flash', multiplier: '0.45x' }],
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
        modelId: 'gpt-5-5',
        modelName: 'GPT-5.5',
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
    packages: [
      {
        id: 'a2-package-3d',
        name: 'GPT-5.5 开发流量包',
        priceCny: 9.9,
        panelAllowance: 5,
        quotaUsagePolicy: {
          fiveHour: { mode: 'limited', amountUsd: '5' },
          daily: { mode: 'limited', amountUsd: '10' },
          scope: 'per_buyer_credential',
          dailyReset: 'utc_plus_8_calendar_day',
        },
        durationDays: 3,
        stockTotal: 12,
        stockAvailable: 8,
        description: '商户提交交付后开始计算 3 天有效期。',
        enabled: true,
        sortOrder: 0,
        models: [
          { serviceModelId: 'a2-gpt-5-5', modelCatalogId: 'gpt-5-5', modelPriceVersionId: 'gpt-5-5-2026-06', modelName: 'GPT-5.5', provider: 'OpenAI', merchantMultiplier: 0.5 },
          { serviceModelId: 'a2-gemini-flash', modelCatalogId: 'gemini-flash', modelPriceVersionId: 'gemini-flash-2026-06', modelName: 'Gemini Flash', provider: 'Google', merchantMultiplier: 0.45 },
        ],
      },
      {
        id: 'a2-package-7d',
        name: '轻量模型周包',
        priceCny: 18.8,
        panelAllowance: 12,
        quotaUsagePolicy: {
          fiveHour: { mode: 'limited', amountUsd: '12' },
          daily: { mode: 'limited', amountUsd: '24' },
          scope: 'per_buyer_credential',
          dailyReset: 'utc_plus_8_calendar_day',
        },
        durationDays: 7,
        stockTotal: 8,
        stockAvailable: 5,
        description: '商户提交交付后开始计算 7 天有效期。',
        enabled: true,
        sortOrder: 1,
        models: [
          { serviceModelId: 'a2-gpt-5-5', modelCatalogId: 'gpt-5-5', modelPriceVersionId: 'gpt-5-5-2026-06', modelName: 'GPT-5.5', provider: 'OpenAI', merchantMultiplier: 0.5 },
        ],
      },
    ],
    contactChannels: [{ type: 'wechat', label: '微信', value: 'qingning_wechat' }],
  },
  {
    id: 'a3',
    title: '多模型备用池',
    sourceUrl: 'https://linux.do/t/multi-model-api/234567',
    sourceAuthorVerification: { status: 'mismatch' },
    quotaUsagePolicy: unspecifiedServiceQuotaPolicy,
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
    contactChannels: [{ type: 'wechat', label: '微信', value: 'beifeng_api' }],
  },
]

export const apiQuotaBatches: ApiQuotaBatch[] = [
  {
    id: 'quota-batch-a1',
    apiServiceId: 'a1',
    sourceType: 'sub2api',
    status: 'published',
    declaredTotalUsdAllowance: '2500.000000',
    unallocatedUsdAllowance: '500.000000',
    saleCutoffAt: '2026-12-30T22:00:00Z',
    expiresAt: '2026-12-31T00:00:00Z',
    sourceConfirmedAt: '2026-07-19T00:30:00Z',
    publishedAt: '2026-07-19T00:40:00Z',
    version: 2,
  },
]

export const apiQuotaOffers: PublicApiQuotaOffer[] = [
  {
    id: 'quota-offer-a1-50',
    batchId: 'quota-batch-a1',
    apiServiceId: 'a1',
    distributionSystem: 'sub2api',
    name: '$50 日内开发额度',
    usdAllowance: '50.000000',
    priceCny: '5.00',
    cnyPerUsd: '0.100000',
    modelMultiplier: '1.0000',
    quotaUsagePolicy: {
      fiveHour: { mode: 'limited', amountUsd: '20' },
      daily: { mode: 'unlimited', amountUsd: null },
      scope: 'per_buyer_credential',
      dailyReset: 'utc_plus_8_calendar_day',
    },
    deliveryMode: 'preimported',
    deliveryEtaMinutes: 2,
    saleMode: 'continuous',
    status: 'published',
    sortOrder: 10,
    publishedAt: '2026-07-19T00:40:00Z',
    version: 1,
    batchStatus: 'published',
    serviceTitle: 'GPT / Claude API 服务',
    sellerDisplayName: '小葵 API',
    sellerIdentityType: 'individual',
    sellerLinuxDoBound: true,
    promptAuditEnabled: false,
    declaredTtftBand: '1_to_3s',
    declaredMaxConcurrency: 8,
    performanceConfirmedAt: '2026-07-19T00:30:00Z',
    performanceDisclaimer: '商户自报，平台未测速',
    saleCutoffAt: '2026-12-30T22:00:00Z',
    expiresAt: '2026-12-31T00:00:00Z',
    availableCopies: 30,
    credentialAvailableCopies: 30,
    isOrderable: true,
    orderabilityCode: 'orderable',
    orderabilityReason: '当前可购买。',
  },
  {
    id: 'quota-offer-a1-100',
    batchId: 'quota-batch-a1',
    apiServiceId: 'a1',
    distributionSystem: 'sub2api',
    name: '$100 整点额度',
    usdAllowance: '100.000000',
    priceCny: '8.80',
    cnyPerUsd: '0.088000',
    modelMultiplier: '1.0000',
    quotaUsagePolicy: unspecifiedServiceQuotaPolicy,
    deliveryMode: 'manual',
    deliveryEtaMinutes: 10,
    saleMode: 'scheduled',
    status: 'published',
    sortOrder: 20,
    publishedAt: '2026-07-19T00:40:00Z',
    version: 1,
    batchStatus: 'published',
    serviceTitle: 'GPT / Claude API 服务',
    sellerDisplayName: '小葵 API',
    sellerIdentityType: 'individual',
    sellerLinuxDoBound: true,
    promptAuditEnabled: true,
    declaredTtftBand: 'under_1s',
    declaredMaxConcurrency: 5,
    performanceConfirmedAt: '2026-07-19T00:30:00Z',
    performanceDisclaimer: '商户自报，平台未测速',
    saleCutoffAt: '2026-12-30T22:00:00Z',
    expiresAt: '2026-12-31T00:00:00Z',
    nextRound: {
      id: 'quota-round-a1-evening',
      batchId: 'quota-batch-a1',
      name: '晚间整点场',
      startsAt: '2026-12-30T12:00:00Z',
      endsAt: '2026-12-30T12:20:00Z',
      status: 'scheduled',
      allocations: [{
        id: 'quota-allocation-a1-100',
        offerId: 'quota-offer-a1-100',
        saleRoundId: 'quota-round-a1-evening',
        saleMode: 'scheduled',
        copyLimit: 5,
        availableCopies: 5,
        reservedCopies: 0,
        consumedCopies: 0,
        allocatedUsdAllowance: '500.000000',
        returnedUsdAllowance: '0.000000',
        status: 'planned',
      }],
      version: 1,
    },
    availableCopies: 0,
    credentialAvailableCopies: 0,
    isOrderable: false,
    orderabilityCode: 'not_started',
    orderabilityReason: '下一轮尚未开始。',
  },
]

export const apiQuotaRounds: ApiQuotaRound[] = apiQuotaOffers.flatMap(item => [item.currentRound, item.nextRound].filter((round): round is ApiQuotaRound => Boolean(round)))

export const apiQuotaCredentialSummaries: ApiQuotaCredentialSummary[] = [
  { offerId: 'quota-offer-a1-50', available: 30, reserved: 0, delivered: 0, retired: 0 },
  { offerId: 'quota-offer-a1-100', available: 0, reserved: 0, delivered: 0, retired: 0 },
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
    quotaUsagePolicySnapshot: limitedServiceQuotaPolicy,
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
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'demo_wechat' }],
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
    quotaUsagePolicySnapshot: limitedServiceQuotaPolicy,
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
      offPlatformContactChannel: '微信',
      status: 'contacted',
      requiresFirstLoginPasswordReset: true,
      note: '商户已记录已进行站外联系',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'muzhou_wechat' }],
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
    quotaUsagePolicySnapshot: unlimitedServiceQuotaPolicy,
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
      offPlatformContactChannel: '微信',
      status: 'contacted',
      requiresFirstLoginPasswordReset: false,
      note: '商户已记录已进行站外联系',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'qingning_wechat' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'demo_wechat' }],
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
    quotaUsagePolicySnapshot: limitedServiceQuotaPolicy,
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
      offPlatformContactChannel: '微信',
      status: 'closed',
      requiresFirstLoginPasswordReset: true,
      note: '商户已关闭本次意向记录',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'c2c_xiaokui' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'demo_wechat' }],
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
    quotaUsagePolicySnapshot: unspecifiedServiceQuotaPolicy,
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
    contactChannels: [{ type: 'wechat', label: '微信', value: 'beifeng_api' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'demo_wechat' }],
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
    quotaUsagePolicySnapshot: unspecifiedServiceQuotaPolicy,
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
      offPlatformContactChannel: '微信',
      status: 'closed',
      requiresFirstLoginPasswordReset: true,
      note: '买家取消该购买意向',
    },
    contactChannels: [{ type: 'wechat', label: '微信', value: 'beifeng_api' }],
    buyerContactChannels: [{ type: 'wechat', label: '微信', value: 'muzhou_wechat' }],
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
  { id: 'api-event-3', intentId: 'api-intent-1002', actorId: 'merchant-orbit', actorLabel: 'orbit', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: '微信' }, createdAt: '2026-06-19 16:01' },
  { id: 'api-event-4', intentId: 'api-intent-1003', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 30, deliveryMode: 'api_key_endpoint' }, createdAt: '2026-06-19 13:03' },
  { id: 'api-event-5', intentId: 'api-intent-1003', actorId: 'merchant-qingning', actorLabel: '青柠', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: '微信' }, createdAt: '2026-06-19 13:18' },
  { id: 'api-event-6', intentId: 'api-intent-0998', actorId: 'buyer-demo-user', actorLabel: 'demo_user', actorRole: 'buyer', type: 'intent_created', toStatus: 'open', metadata: { amount: 60, deliveryMode: 'sub2api_panel_account' }, createdAt: '2026-06-18 19:20' },
  { id: 'api-event-7', intentId: 'api-intent-0998', actorId: 'merchant-orbit', actorLabel: 'orbit', actorRole: 'merchant', type: 'contacted', fromStatus: 'open', toStatus: 'contacted', metadata: { channel: '微信' }, createdAt: '2026-06-18 19:28' },
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
    completedLast90Days: 6,
    responseMedianMinutes: 3,
    merchantResponsibleCancellations: 0,
    unresolvedDisputes: 0,
    handledDisputesLast90Days: 1,
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
    completedLast90Days: 3,
    responseMedianMinutes: 4,
    merchantResponsibleCancellations: 0,
    unresolvedDisputes: 0,
    handledDisputesLast90Days: 0,
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
    completedLast90Days: 25,
    responseMedianMinutes: 9,
    merchantResponsibleCancellations: 1,
    unresolvedDisputes: 1,
    handledDisputesLast90Days: 3,
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
    communityIdentities: myUserProfile.communityIdentities,
    accountStatus: 'normal',
    createdAt: myUserProfile.privacy.showCreatedAt ? '2025-11-18' : null,
    lastActiveAt: myUserProfile.privacy.showLastActiveAt ? '12 分钟前' : null,
    stats: {
      completedCarpools: 8,
      completedApiOrders: 18,
      completedCarpoolsLast90Days: 2,
      completedApiOrdersLast90Days: 6,
      responseMedianMinutes: 3,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 0,
      unknownResponsibilityCancellationCount: 0,
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
    communityIdentities: [],
    accountStatus: 'normal',
    createdAt: '2026-04-09',
    lastActiveAt: '28 分钟前',
    stats: {
      completedCarpools: 4,
      completedApiOrders: 8,
      completedCarpoolsLast90Days: 1,
      completedApiOrdersLast90Days: 3,
      responseMedianMinutes: 4,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 0,
      unknownResponsibilityCancellationCount: 0,
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
    communityIdentities: [],
    accountStatus: 'under_review',
    createdAt: '2025-08-26',
    lastActiveAt: '2 小时前',
    stats: {
      completedCarpools: 0,
      completedApiOrders: 42,
      completedCarpoolsLast90Days: 0,
      completedApiOrdersLast90Days: 25,
      responseMedianMinutes: 9,
      buyerResponsibilityCancellationCount: 0,
      sellerResponsibilityCancellationCount: 1,
      unknownResponsibilityCancellationCount: 0,
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
  { id: 'review-orbit-1', username: 'orbit', date: '2026-06-18', serviceType: 'GPT / Claude API 服务', rating: 5, tags: ['响应及时', '说明清楚', '核对顺畅'], note: '站外确认节奏清楚，用量核对说明充分。', verified: true },
  { id: 'review-orbit-2', username: 'orbit', date: '2026-06-12', serviceType: 'Claude Sonnet API 服务', rating: 4, tags: ['倍率一致', '售后正常'], note: '倍率和页面说明一致。', verified: true },
  { id: 'review-qingning-1', username: 'qingning', date: '2026-06-19', serviceType: '轻量模型开发额度', rating: 5, tags: ['响应及时', '倍率一致'], note: '记录较少，但本单信息清楚。', verified: true },
  { id: 'review-beifeng-1', username: 'beifeng-api', date: '2026-06-15', serviceType: '多模型备用池', rating: 2, tags: ['响应较慢', '用量不透明'], note: '已完成交易，用量展示需要提前说明。', verified: true },
]

export const publicDisputeRecords: PublicDisputeRecord[] = [
  { id: 'dispute-orbit-1', username: 'orbit', type: '响应超时', result: '已补偿等值额度，记录关闭', handledAt: '2026-05-28', unresolved: false },
  { id: 'dispute-beifeng-1', username: 'beifeng-api', type: '用量核对说明不一致', result: '处理中，服务已暂停接单', handledAt: '2026-06-17', unresolved: true },
  { id: 'dispute-beifeng-2', username: 'beifeng-api', type: '站外确认信息缺失', result: '商户补充说明后关闭', handledAt: '2026-05-31', unresolved: false },
]

export const orderContactSnapshots: OrderContactSnapshot[] = [
  {
    id: 'contact-snapshot-ride-app-1',
    orderType: 'carpool_application',
    orderId: 'ride-app-1',
    sellerContacts: [],
    buyerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'zhi***', displayValue: 'zhichuan_wechat', verified: false, usageScope: 'buyer' },
    ],
    contactWindowEndsAt: null,
    canView: false,
    unavailableReason: '车主确认上车并建立有效成员关系后才展示联系方式。',
    createdAt: '2026-06-19 16:18',
  },
  {
    id: 'contact-snapshot-ride-app-2',
    orderType: 'carpool_application',
    orderId: 'ride-app-2',
    sellerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'c2c_***', displayValue: 'c2c_orbit', verified: false, usageScope: 'carpool_owner' },
    ],
    buyerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'muz***', displayValue: 'muzhou_wechat', verified: false, usageScope: 'buyer' },
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
      { type: 'wechat', label: '微信', maskedValue: 'qin***', displayValue: 'qingning_wechat', verified: false, usageScope: 'carpool_owner' },
    ],
    buyerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'dem***', displayValue: 'demo_wechat', verified: false, usageScope: 'buyer' },
    ],
    contactWindowEndsAt: '2026-06-18 20:42',
    canView: true,
    unavailableReason: null,
    createdAt: '2026-06-18 20:12',
  },
  {
    id: 'contact-snapshot-ride-app-4',
    orderType: 'carpool_application',
    orderId: 'ride-app-4',
    sellerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'bei***', displayValue: 'beifeng_wechat', verified: false, usageScope: 'carpool_owner' },
    ],
    buyerContacts: [
      { type: 'wechat', label: '微信', maskedValue: 'dem***', displayValue: 'demo_wechat', verified: false, usageScope: 'buyer' },
    ],
    contactWindowEndsAt: null,
    canView: true,
    unavailableReason: null,
    createdAt: '2026-05-19 12:48',
  },
]

export const adminCards = [
  { label: '低价线索待审', value: 12, hint: '含 3 条疑似重复' },
  { label: '车源异常待处理', value: 8, hint: '含下架恢复和高风险字段变更' },
  { label: '在线 API 商户', value: 6, hint: '1 个未响应预警' },
  { label: '未解决纠纷', value: 4, hint: '今日新增 1 条' },
]
