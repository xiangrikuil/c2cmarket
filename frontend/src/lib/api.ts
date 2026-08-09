import {
  adminCards,
  adminAuditLogs,
  adminDirectoryUsers,
  apiQuotaBatches,
  apiQuotaCredentialSummaries,
  apiQuotaOffers,
  apiQuotaRounds,
  apiPurchaseIntentEvents,
  apiPurchaseIntents,
  apiServices,
  carpoolApplicationEvents,
  carpoolApplications,
  carpoolOpeningChannels,
  carpoolPaymentMethods,
  carpoolProductCatalog,
  carpoolRegions,
  carpools,
  categoryRows,
  modelCatalog,
  officialPrices,
  myContactMethods,
  myUserProfile,
  orderContactSnapshots,
  productTrends,
  publicCompletionRecords,
  publicDisputeRecords,
  publicMerchantProfiles,
  publicUserProfiles,
  publicReviewRecords,
  transactionRecords,
  type AvatarMode,
  type ApiBillingMode,
  type ApiContactChannel,
  type ApiDeliveryMode,
  type ApiMerchantIdentityMode,
  type ApiPurchaseIntent,
  type ApiPurchaseIntentEvent,
  type ApiPurchaseIntentEventType,
  type ApiPurchaseIntentStatus,
  type ApiQuotaBatch,
  type ApiQuotaCredentialSummary,
  type ApiQuotaDeliveryMode,
  type ApiQuotaDistributionSystem,
  type ApiQuotaOffer,
  type ApiQuotaRound,
  type ApiQuotaSaleMode,
  type ApiQuotaSourceType,
  type ApiQuotaSystemSaleSlot,
  type ApiQuotaSystemSaleSlotList,
  type ApiService,
  type ApiServiceCommercialSnapshot,
  type ApiServiceSalesChannel,
  type ApiServiceSalesChannelKind,
  type ApiServiceSalesState,
  type ApiServiceSalesSummary,
  type ApiServiceSalesView,
  type OwnerApiService,
  type ApiServicePackage,
  type ApiServicePackageModel,
  type ApiServicePackageSnapshot,
  type ApiServiceState,
  type ApiTTFTBand,
  type ApiUsageVisibility,
  type AdminAuditLog,
  type Carpool,
  type CarpoolApplication,
  type CarpoolApplicationEligibility,
  type CarpoolApplicationEligibilityCode,
  type CarpoolApplicationEvent,
  type CarpoolApplicationEventType,
  type CarpoolApplicationReview,
  type CarpoolApplicationStatus,
  type CarpoolCancellationResponsibility,
  type CarpoolSeatSummary,
  type CarpoolProductCatalogItem,
  type ContactMethodType,
  type ContactUsageScope,
  type CreateContactReportRequest,
  type OpeningChannelOption,
  type OrderContactSnapshot,
  type OrderContactSnapshotItem,
  type PaymentMethodOption,
  type RegionOption,
  type ModelCatalogItem,
  type ModelPriceRow,
  type OfficialPrice,
  type PublicReviewRecord,
  type PublicMerchantProfile,
  type PublicUserProfile,
  type ProductTrend,
  type PublicApiQuotaOffer,
  type TransactionRecord,
  type TransactionTrendPoint,
  type UserContactMethod,
  type UserPrivacySettings,
  type UserProfile,
} from '@/data/mock'
import type { ReputationSummary } from '@/types/reputation'
import type { ApiServiceHealthHourlyBucket, ApiServiceHealthSample, ApiServiceHealthSummary } from '@/types/apiHealth'
import { getOwnerAPIProbeConnections, updateMockAPIProbeConnectionReference } from '@/lib/apiHealthFacade'
import type { ApiQuotaUsagePolicy, ApiQuotaUsagePolicyInput } from '@/types/apiQuota'
export type { ApiQuotaLimitMode, ApiQuotaUsageLimit, ApiQuotaUsageLimitInput, ApiQuotaUsagePolicy, ApiQuotaUsagePolicyInput, ApiWritableQuotaLimitMode } from '@/types/apiQuota'
import { mockPublicUserReputation } from '@/lib/reputationMock'
import { compareByTradablePrice, getPricingDisplay } from '@/lib/pricing'
import {
  getApiMerchantDisplayName,
  isApiServicePubliclyOrderable,
  type ApiIntentMerchantSource,
  type ApiMerchantIdentitySource,
} from '@/lib/apiServicePresentation'
export { getApiMerchantDisplayName, isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'
import { evaluateCarpoolApplicationEligibility, hasCredentialSharingLanguage } from '@/lib/carpoolEligibility'
import { matchesApiOrderSearch } from '@/lib/apiOrderUi'
import { isApiOrderDisputeActive, normalizeApiOrderDisputeStatus, type ApiOrderDisputeStatus, type OpenApiOrderDisputeInput } from '@/lib/apiOrderDispute'
export { canOpenApiOrderDispute, getApiOrderDisputeStatusDescription, getApiOrderDisputeStatusLabel, isApiOrderDisputeActive, normalizeApiOrderDisputeStatus } from '@/lib/apiOrderDispute'
export type { ApiOrderDisputeStatus } from '@/lib/apiOrderDispute'
export { evaluateCarpoolApplicationEligibility } from '@/lib/carpoolEligibility'
import { defaultQuotaLabel, defaultQuotaPeriod, defaultQuotaUnit } from '@/lib/quota'
import { beijingDateTimeInputToISOString, formatBeijingDateTimeInput, formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'
import {
  apiQuotaUsagePolicyFromInput,
  normalizeHistoricalApiQuotaUsagePolicy,
  toApiQuotaUsagePolicyInput,
} from '@/lib/apiQuotaPolicy'
import { getMockPublicAPIModels } from '@/lib/apiModelCatalogBackend'
import {
  cloneApiPaymentAccountSettings,
  isApiPaymentMethod,
  isApiPaymentOptionComplete,
  normalizeApiPaymentAccountSettings,
  normalizeQrCodeDataUrl,
  type ApiPaymentAccountSettings,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'
import {
  backendAcceptCarpoolApplication,
  backendAdminCarpoolRows,
  backendAdminCarpoolRowsPage,
  backendBuyerLeaveCarpool,
  backendCancelCarpoolApplication,
  backendBuyerConfirmCarpoolCompleted,
  backendBuyerConfirmCarpoolJoined,
  backendCarpoolApplicationEligibility,
  backendCarpoolApplicationById,
  backendCarpoolApplicationContacts,
  backendCarpoolApplicationEvents,
  backendCarpoolOpeningChannels,
  backendCarpoolPaymentMethods,
  backendCarpoolProductCatalog,
  backendCarpoolRegions,
  backendCreateCarpoolApplication,
  backendGetCarpoolById,
  backendGetCarpools,
  backendGetCarpoolsPage,
  backendOwnerCarpools,
  backendOwnerCarpoolsPage,
  backendMerchantCarpoolApplications,
  backendMerchantCarpoolApplicationsPage,
  backendMyCarpoolApplications,
  backendMyCarpoolApplicationsPage,
  backendOwnerRemoveCarpool,
  backendOwnerConfirmCarpoolCompleted,
  backendOwnerConfirmCarpoolJoined,
  backendRejectCarpoolApplication,
  backendRunAdminCarpoolAction,
  backendSubmitCarpool,
  backendUpdateAdminCarpoolStatus,
  backendWithdrawCarpoolAcceptance,
} from '@/lib/carpoolBackend'
import type { CarpoolPageFilters } from '@/lib/carpoolBackend'
import {
  backendAPIIntentById,
  backendAPIIntentEvents,
  backendAPIQuotaBatchAction,
  backendAPIQuotaCredentialSummary,
  backendAdminAPIOrder,
  backendAdminAPIOrderRows,
  backendAdminAPIOrderRowsPage,
  backendAPIServiceById,
  backendAPIServices,
  backendAPIServicesPage,
  backendAdminAPIServiceRows,
  backendAdminAPIServiceRowsPage,
  backendCancelAPIOrder,
  backendConfirmAPIOrderComplete,
  backendConfirmAPIOrderPayment,
  backendCreateAPIOrderFromIntent,
  backendCreateAPIQuotaBatch,
  backendCreateAPIQuotaOffer,
  backendCreateAPIQuotaOrder,
  backendCreateAPIQuotaRushOffer,
  backendCreateAPIQuotaRound,
  backendCancelAPIIntentById,
  backendCloseAPIIntent,
  backendCreateAPIPurchaseIntent,
  backendMarkAPIIntentContacted,
  backendModelCatalog,
  backendImportAPIQuotaCredentials,
  backendMyAPIOrder,
  backendMyAPIOrders,
  backendMyAPIOrdersPage,
  backendMyAPIIntents,
  backendOtherAPIServices,
  backendOwnerAPIQuotaBatches,
  backendOwnerAPIQuotaOffers,
  backendOwnerAPIQuotaRounds,
  backendOwnerAPIOrder,
  backendOwnerAPIOrders,
  backendOwnerAPIOrdersPage,
  backendOwnerAPIIntents,
  backendOwnerAPIServiceById,
  backendOwnerAPIServices,
  backendOwnerAPIServicesPage,
  backendOpenAPIOrderDispute,
  backendPauseAPIService,
  backendPublishAPIService,
  backendPublicAPIQuotaOffer,
  backendPublicAPIQuotaOffers,
  backendPublicAPIQuotaOffersPage,
  backendAPIQuotaSaleSlots,
  backendReadAPIOrderPaymentInstructions,
  backendReportAPIOrderPaymentIssue,
  backendRunAdminAPIServiceAction,
  backendResumeAPIService,
  backendSubmitAPIOrderDeliveryCredential,
  backendSubmitAPIOrderPayment,
  backendSub2APIServices,
  backendSubmitAPIService,
  backendUpdateAdminAPIServiceStatus,
  backendUpdateAPIServiceProbeConnection,
} from '@/lib/apiMarketBackend'
import type { AdminAPIServicePageFilters } from '@/lib/apiMarketBackend'
import { paginateCursorItems, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'
import { productMatchesCategory, productMatchesPlan, type ProductCategoryKey } from '@/lib/productCategories'
import { isCarpoolAdminActionStatus, isCarpoolExceptionStatus } from '@/lib/carpoolModeration'
import { isApiServiceAdminActionStatus, isApiServiceExceptionStatus, isApiServicePublicStatus } from '@/lib/apiServiceModeration'
import {
  backendCreateContact,
  backendDeleteContact,
  backendMyContactMethods,
  backendMyMerchantProfile,
  backendMyProfile,
  backendConfirmEmailVerification,
  backendPublicMerchantProfile,
  backendPublicUserProfile,
  backendSetDefaultContact,
  backendSetPassword,
  backendStartEmailVerification,
  backendUpdateContact,
  backendUpdateMyProfile,
  backendUpsertMerchantProfile,
  backendVerifyContact,
  type BackendMerchantProfile,
} from '@/lib/profileBackend'
import { backendReviewCenterPage, backendReviewCenterRows, backendSubmitReview } from '@/lib/reviewBackend'
import { backendSearchMarket } from '@/lib/searchBackend'
import {
  backendAdminOfficialPriceRows,
  backendAdminOfficialPriceRowsPage,
  backendMyOfficialPriceLeads,
  backendOfficialPriceById,
  backendOfficialPrices,
  backendOfficialPricesPage,
  backendRunOfficialPriceAdminAction,
  backendSubmitOfficialPriceLead,
  backendUpdateOfficialPriceAdminStatus,
} from '@/lib/officialPriceBackend'
import type { OfficialPricePageFilters } from '@/lib/officialPriceBackend'
import {
  backendFavoriteStatus,
  backendFavorites,
  backendToggleFavorite,
} from '@/lib/favoriteBackend'
import {
  backendAddFeedbackSupplement,
  backendAdminFeedbackRows,
  backendAdminFeedbackTicket,
  backendAdminFeedbackTickets,
  backendCreateFeedbackTicket,
  backendFeedbackUnreadCount,
  backendHandleFeedbackTicket,
  backendMarkFeedbackRead,
  backendMyFeedbackTicket,
  backendMyFeedbackTickets,
  feedbackImpactLabel,
  feedbackStatusLabel,
  feedbackTypeLabel,
} from '@/lib/feedbackBackend'
import {
  backendMarkAllNotificationsRead,
  backendMarkNotificationRead,
  backendNotifications,
} from '@/lib/notificationBackend'
import {
  backendNavigationBadges,
  type NavigationBadgeSummary,
} from '@/lib/navigationBadgeBackend'
import { getImportantAnnouncementUnreadCount } from '@/lib/announcementsApi'
import {
  backendAdminAppealRows,
  backendAdminReportRows,
  backendCreateManualInterventionReport,
  backendCreatePublicUserReport,
  backendCreateReport,
  backendRunReportAdminAction,
  backendUpdateReportAdminStatus,
  type CreateManualInterventionReportRequest,
  type CreatePublicUserReportRequest,
} from '@/lib/reportBackend'
import {
  backendAdminUserDetail,
  backendAdminUserDirectory,
  backendUpdateAdminUserPermission,
  backendUpdateAdminUserStatus,
  type AdminUser,
  type AdminUserDetail,
  type AdminUserDirectoryQuery,
  type AdminUserList,
  type AdminUserStatus,
} from '@/lib/adminUserBackend'
import { shouldUseRealBackend } from '@/lib/backendClient'
import { linuxDoProfileSummaryUrl } from '@/lib/linuxDo'
import {
  backendGetApiPaymentAccountSettings,
  backendUpdateApiPaymentAccountSettings,
} from '@/lib/apiPaymentSettingsBackend'
import { getBackupPasswordValidationMessage } from '@/lib/passwordPolicy'
import { compareDecimal, divideDecimal, normalizeDecimal, normalizeDecimalTrimmed } from '@/lib/decimal'
export type AdminSection =
  | 'official-prices'
  | 'price-leads'
  | 'carpools'
  | 'api-services'
  | 'trade-intents'
  | 'users'
  | 'feedback'
  | 'reports'
  | 'appeals'
  | 'logs'

export type AdminRow = {
  id: string
  primary: string
  secondary: string
  owner: string
  status: string
  risk: string
  targetType?: string
  backendKind?: string
  backendVersion?: number
  detailItems?: Array<{ label: string, value: string }>
  targetTo?: string | null
  moderationParticipants?: Array<{ userId: string, label: string }>
  moderationSupplements?: Array<{
    id: string
    submittedByUserId: string
    submittedByUsername: string
    submittedByName: string
    body: string
    createdAt: string
  }>
}

export type ApiOrderNotification = {
  id: string
  title: string
  detail: string
  time: string
  unread: boolean
}

export type CarpoolNotification = {
  id: string
  title: string
  detail: string
  time: string
  unread: boolean
  to: string
}

export type UnifiedNotification = {
  id: string
  type: '审核结果' | '上车申请' | 'API 意向' | 'API 订单' | '问题反馈' | '管理操作' | '边界提醒'
  title: string
  detail: string
  time: string
  unread: boolean
  to: string
}

export type FavoriteTargetType = 'carpool' | 'api-service'

export type FavoriteRecord = {
  id: string
  targetType: FavoriteTargetType
  targetId: string
  createdAt: string
}

export type FavoriteListItem = FavoriteRecord & {
  title: string
  subtitle: string
  status: string
  to: string
}

export type SearchResult = {
  id: string
  type: '官方价格' | '车源' | 'API 服务' | '用户' | '商户'
  title: string
  subtitle: string
  badge: string
  to: string
}

export type ReviewCenterRow = {
  id: string
  transactionType: 'carpool_membership' | 'api_order'
  transactionId: string
  direction: 'pending' | 'sent' | 'received'
  target: string
  counterparty: string
  counterpartyUsername: string
  reviewerRole: 'buyer' | 'seller'
  revieweeRole: 'buyer' | 'seller'
  status: 'reviewable' | 'expired' | 'sealed' | 'published' | 'removed'
  visibility: 'none' | 'sealed' | 'published' | 'removed'
  counterpartySubmitted: boolean
  canCreate: boolean
  canEdit: boolean
  rating: number | null
  tags: string[]
  note: string | null
  completedAt: string
  reviewDeadlineAt: string
  submittedAt: string | null
  visibleAt: string | null
  frozenAt: string | null
  createdAt: string
  updatedAt: string
  version: number
}

export type ReviewCenterData = {
  items: ReviewCenterRow[]
  presetTags: string[]
}

export type FeedbackTicketType = 'function_issue' | 'data_correction' | 'experience_suggestion' | 'publish_contact_block'
export type FeedbackImpact = 'general' | 'blocks_operation' | 'cannot_continue'
export type FeedbackStatus = 'submitted' | 'recorded' | 'following_up' | 'resolved' | 'declined' | 'needs_user_info' | 'closed'
export type FeedbackEventAction = 'submitted' | 'admin_handled' | 'user_supplemented' | 'read'

export type FeedbackEvent = {
  id: string
  actorUserId?: string
  actorName: string
  actorRole: 'user' | 'admin' | 'system'
  action: FeedbackEventAction
  publicMessage: string
  internalNote?: string
  createdAt: string
}

export type FeedbackTicket = {
  id: string
  submitterUserId?: string
  submitterUsername?: string
  submitterName: string
  type: FeedbackTicketType
  impact: FeedbackImpact
  status: FeedbackStatus
  title: string
  description: string
  contextPageLabel: string
  contextTargetType: string
  contextTargetId: string
  contextTargetLabel: string
  contextRoleLabel: string
  adminResponse?: string
  adminInternalNote?: string
  handledByAdminId?: string
  handledByAdminName?: string
  handledAt?: string | null
  latestAdminUpdateAt?: string | null
  submitterReadAt?: string | null
  unread: boolean
  createdAt: string
  updatedAt: string
  version: number
  events?: FeedbackEvent[]
}

export type SubmitFeedbackPayload = {
  type: FeedbackTicketType
  impact: FeedbackImpact
  title?: string
  description: string
  contextPageLabel: string
  contextTargetType?: string
  contextTargetId?: string
  contextTargetLabel?: string
  contextRoleLabel?: string
}

export type FeedbackSupplementPayload = {
  message: string
}

export type FeedbackAdminHandlePayload = {
  status: Exclude<FeedbackStatus, 'submitted'>
  response: string
  internalNote?: string
}

export type SubmitReviewPayload = {
  transactionType: ReviewCenterRow['transactionType']
  transactionId: string
  operation: 'create' | 'edit'
  rating: number
  tags: string[]
  note: string
}

export type CreatePublicProfileReportRequest = CreatePublicUserReportRequest

export type TransactionTrendRange = '7d' | '30d' | '90d'
export type { ApiPaymentAccountSettings, ApiPaymentMethod, ApiPaymentOption } from '@/lib/apiPaymentSettings'

export type ApiOrderStatus =
  | 'pending_payment'
  | 'payment_submitted'
  | 'payment_issue'
  | 'paid_confirmed'
  | 'delivery_submitted'
  | 'completed'
  | 'cancelled'

export type ApiOrderDeliveryKind = 'api_key_endpoint' | 'login_account'
export type ApiOrderPaymentIssueReason = 'not_received' | 'amount_mismatch' | 'remark_mismatch'
export type ApiOrderPurchaseKind = 'api_service' | 'limited_quota_offer'
export type ApiOrderCompletionSource = 'buyer_confirmed' | 'auto_completed'
export type ApiOrderViewerRole = 'buyer' | 'merchant' | 'admin'

export type ApiQuotaOrderSnapshot = ApiServiceCommercialSnapshot & {
  batchId: string
  offerId: string
  saleRoundId?: string
  offerName: string
  usdAllowance: string
  priceCny: string
  cnyPerUsd: string
  modelMultiplier: string
  saleCutoffAt: string
  expiresAt: string
  saleMode: ApiQuotaSaleMode
  roundStartsAt?: string
  roundEndsAt?: string
  distributionSystem: ApiQuotaDistributionSystem
  ttftBand: ApiTTFTBand
  declaredMaxConcurrency: number
  performanceConfirmedAt?: string
  performanceUnverified: boolean
  deliveryEtaMinutes: number
  deliveryMode: ApiQuotaDeliveryMode
}

export type ApiOrderDeliveryCredential = {
  deliveryKind: ApiOrderDeliveryKind
  apiBaseUrl?: string
  apiKey?: string
  panelLoginUrl?: string
  username?: string
  password?: string
  instructions?: string
  submittedAt: string
  destroyedAt?: string
  destroyReason?: 'retention_expired' | 'retired_unused'
}

export type SubmitApiOrderDeliveryCredentialPayload = {
  deliveryKind: ApiOrderDeliveryKind
  apiBaseUrl?: string
  apiKey?: string
  panelLoginUrl?: string
  username?: string
  password?: string
  instructions?: string
}

export type ApiOrderPaymentInstructions = {
  orderId: string
  paymentMethod: ApiPaymentOption['paymentMethod']
  paymentInstructions: string
  paymentQrCodeDataUrl: string | null
  paymentExpiresAt: string
}

export type ApiOrder = {
  id: string
  orderNo: string
  purchaseKind: ApiOrderPurchaseKind
  apiPurchaseIntentId: string
  apiServiceId: string
  buyerId: string
  buyer: string
  sellerId: string
  seller: string
  buyerReputation?: ReputationSummary | null
  sellerReputation?: ReputationSummary | null
  status: ApiOrderStatus
  disputeStatus?: ApiOrderDisputeStatus
  disputeCaseId?: string
  serviceTitle: string
  amount: number
  amountDecimal?: string
  currency: 'CNY'
  selectedPaymentMethod: ApiPaymentOption['paymentMethod']
  paymentWindowMinutes: number
  paymentExpiresAt: string
  paymentSummary?: string
  paymentSubmittedAt?: string
  paymentIssueReason?: ApiOrderPaymentIssueReason
  paymentIssueNote?: string
  paymentIssueReportedAt?: string
  paidConfirmedAt?: string
  deliveryNote?: string
  deliverySubmittedAt?: string
  deliveryReviewExpiresAt?: string
  deliveryCredential?: ApiOrderDeliveryCredential
  completionSource?: ApiOrderCompletionSource
  completedAt?: string
  cancelledAt?: string
  cancelReason?: string
  version: number
  intentSnapshot: ApiPurchaseIntent['snapshot']
  selectedDeliveryMode: ApiDeliveryMode
  selectedPackageId?: string
  packageSnapshot?: ApiServicePackageSnapshot
  packageStockReserved?: boolean
  packageExpiresAt?: string
  requestedUsdAllowance: number
  requestedUsdAllowanceDecimal?: string
  quotaSnapshot?: ApiQuotaOrderSnapshot
  quotaUsagePolicySnapshot: ApiQuotaUsagePolicy
  merchantContactChannels: ApiContactChannel[]
  buyerContactChannels: ApiContactChannel[]
  viewerRole?: 'buyer' | 'merchant'
  createdAt: string
  updatedAt: string
}

export type AdminApiOrderDetail = {
  id: string
  purchaseKind: ApiOrderPurchaseKind
  apiPurchaseIntentId: string
  apiServiceId: string
  buyerUserId: string
  sellerUserId: string
  status: ApiOrderStatus
  disputeStatus?: ApiOrderDisputeStatus
  disputeCaseId?: string
  serviceTitleSnapshot: string
  billingModeSnapshot?: string
  selectedPackageId?: string
  selectedPackageSnapshot?: string
  amount: string
  currency: 'CNY'
  requestedUsdAllowanceSnapshot?: string
  cnyPerUsdAllowanceSnapshot?: string
  selectedPaymentMethod: ApiPaymentOption['paymentMethod']
  paymentExpiresAt: string
  paymentSubmittedAt?: string
  paymentIssueReason?: ApiOrderPaymentIssueReason
  paymentIssueNote?: string
  paymentIssueReportedAt?: string
  paidConfirmedAt?: string
  deliveryNote?: string
  deliverySubmittedAt?: string
  deliveryReviewExpiresAt?: string
  completionSource?: ApiOrderCompletionSource
  completedAt?: string
  cancelledAt?: string
  cancelReason?: string
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiQuotaOfferFilters = {
  distributionSystem?: ApiQuotaDistributionSystem | 'all'
  oneMultiplier?: boolean
  onlyOrderable?: boolean
  slotKey?: string
  search?: string
  excludeSystemSlots?: boolean
}

export type CreateApiQuotaOrderPayload = {
  offerId: string
  saleRoundId?: string
}

export type CreateApiQuotaBatchPayload = {
  apiServiceId: string
  sourceType: ApiQuotaBatch['sourceType']
  sourceLabel?: string
  declaredTotalUsdAllowance: string
  saleCutoffAt: string
  expiresAt: string
  sourceConfirmedAt: string
}

export type CreateApiQuotaOfferPayload = {
  batchId: string
  name: string
  usdAllowance: string
  priceCny: string
  modelMultiplier: string
  quotaUsagePolicy: ApiQuotaUsagePolicyInput
  deliveryMode: ApiQuotaDeliveryMode
  deliveryEtaMinutes: number
  saleMode: ApiQuotaSaleMode
  continuousCopies: number
  sortOrder: number
}

export type CreateApiQuotaRoundPayload = {
  batchId: string
  name: string
  startsAt: string
  endsAt: string
  offers: Array<{ offerId: string, copies: number }>
}

export type CreateApiQuotaRushOfferPayload = {
  apiServiceId: string
  sourceType: ApiQuotaSourceType
  sourceLabel?: string
  name: string
  usdAllowance: string
  priceCny: string
  modelMultiplier: string
  quotaUsagePolicy: ApiQuotaUsagePolicyInput
  copies: number
  deliveryMode: ApiQuotaDeliveryMode
  deliveryEtaMinutes: number
  slotKey: string
  expiresAt: string
  sourceConfirmedAt: string
  deliveryKind?: ApiOrderDeliveryKind
  file?: File
}

export type ApiQuotaRushOfferPublication = {
  batch: ApiQuotaBatch
  offer: ApiQuotaOffer
  round: ApiQuotaRound
  credentialImported: number
  credentialSummary: ApiQuotaCredentialSummary
}

export type ApiOrderEvent = {
  id: string
  orderId: string
  actorLabel: string
  actorRole: 'buyer' | 'merchant' | 'system'
  type: 'created' | 'payment_submitted' | 'payment_issue_reported' | 'payment_confirmed' | 'delivery_submitted' | 'completed' | 'cancelled'
  fromStatus?: ApiOrderStatus
  toStatus?: ApiOrderStatus
  note?: string
  createdAt: string
}

export type TransactionTrendSummary = {
  productId: string
  productName: string
  range: TransactionTrendRange
  latestTransactionPrice: number | null
  medianPrice: number | null
  p25Price: number | null
  p75Price: number | null
  validSampleCount: number
  points: TransactionTrendPoint[]
  updatedAt: string
}

export type { ModelCatalogItem }
export type { ApiMerchantIdentityMode }
export type { ApiServiceCommercialSnapshot }
export type {
  CarpoolProductCatalogItem,
  CarpoolApplicationEligibility,
  CarpoolApplicationEligibilityCode,
  OpeningChannelOption,
  PaymentMethodOption,
  RegionOption,
}

export type BackendResourceMeta = {
  backendVersion?: number
  backendContactSessionId?: string
  backendMembershipId?: string
  backendStatus?: string
}

export type CarpoolWithMeta = Carpool & BackendResourceMeta & { seatSummary?: CarpoolSeatSummary }
export type CarpoolApplicationWithMeta = CarpoolApplication & BackendResourceMeta
export type OfficialPriceWithMeta = OfficialPrice & BackendResourceMeta

export type CarpoolDraftStatus = 'draft' | 'reviewing'

export type SaveCarpoolDraftPayload = {
  productId: string
  customProductName: string | null
  regionCode: string
  customRegionName: string | null
  monthlyPriceCny: number | null
  serviceMultiplier: number | null
  dailyQuotaAmount: number | null
  weeklyQuotaAmount: number | null
  followsOfficialQuotaReset: boolean | null
  vpsRegion: string
  supportsMainlandChinaDirectConnection: boolean | null
  totalSeats: number
  occupiedSeats: number
  openingChannelCode: string
  customOpeningChannel: string
  paymentMethodCode: string
  customPaymentMethod: string
  distributionMethod?: Carpool['distributionMethod'] | ''
  distributionMethodNote?: string
  providesAdminAccount?: boolean | null
  accessArrangementMode?: Carpool['accessArrangementMode']
  accessArrangementNote?: string
  riskAcknowledged?: boolean
  policyVersion?: number | null
  riskNoticeCode?: string | null
  warranty: {
    mode: string
    fixedWarrantyDays: number | null
    compensationMethod: string | null
    exclusions: string | null
  }
  rulesNote: string
  status: CarpoolDraftStatus
}

const wait = () => new Promise(resolve => setTimeout(resolve, 80))
const currentBuyerId = 'buyer-demo-user'
const currentBuyerName = 'demo_user'
const currentOwnerId = 'owner-orbit'
const currentOwnerName = 'orbit'
const currentMerchantId = 'merchant-orbit'
const currentMerchantName = 'orbit'
const apiPurchaseIntentStorageKey = 'c2cmarket.apiPurchaseIntents.v2'
const apiPurchaseIntentEventStorageKey = 'c2cmarket.apiPurchaseIntentEvents.v2'
const apiOrderStorageKey = 'c2cmarket.apiOrders.v1'
const apiOrderDeliveryReviewWindowMs = 24 * 60 * 60 * 1000
const apiQuotaBatchStorageKey = 'c2cmarket.apiQuotaBatches.v1'
const apiQuotaOfferStorageKey = 'c2cmarket.apiQuotaOffers.v1'
const apiQuotaRoundStorageKey = 'c2cmarket.apiQuotaRounds.v1'
const apiQuotaCredentialSummaryStorageKey = 'c2cmarket.apiQuotaCredentialSummaries.v1'
const carpoolApplicationStorageKey = 'c2cmarket.carpoolApplications.v1'
const carpoolApplicationEventStorageKey = 'c2cmarket.carpoolApplicationEvents.v1'
const adminAuditLogStorageKey = 'c2cmarket.adminAuditLogs.v1'
const officialPriceStorageKey = 'c2cmarket.officialPrices.v1'
const carpoolStorageKey = 'c2cmarket.carpools.v1'
const apiServiceStorageKey = 'c2cmarket.apiServices.v1'
const apiServicePaymentSnapshotStorageKey = 'c2cmarket.apiServicePaymentSnapshots.v1'
const apiPaymentAccountSettingsStorageKey = 'c2cmarket.apiPaymentAccountSettings.v1'
const feedbackStorageKey = 'c2cmarket.feedbackTickets.v1'
const notificationReadStorageKey = 'c2cmarket.notificationReadState.v1'
const favoriteStorageKey = 'c2cmarket.favorites.v1'
const apiOrderNumberAlphabet = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789'
const carpoolApplyAllowedStatuses: Carpool['status'][] = ['可上车']
const carpoolContactVisibleStatuses: CarpoolApplicationStatus[] = ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation', 'active', 'pending_completion', 'completed', 'disputed']
const apiContactVisibleStatuses: ApiPurchaseIntentStatus[] = ['open', 'contacted', 'ordered', 'buyer_cancelled', 'owner_closed']

let apiPurchaseIntentStore = normalizeApiPurchaseIntentStore(readSessionStore(apiPurchaseIntentStorageKey, apiPurchaseIntents))
let apiPurchaseIntentEventStore = normalizeApiPurchaseIntentEventStore(readSessionStore(apiPurchaseIntentEventStorageKey, apiPurchaseIntentEvents))
const loadedApiOrders = readSessionStore<ApiOrder[]>(apiOrderStorageKey, [])
let apiOrderStore = normalizeApiOrderStore(loadedApiOrders)
if (loadedApiOrders.some(order => !order.orderNo)) persistApiOrderStore()
let apiQuotaBatchStore = readSessionStore<ApiQuotaBatch[]>(apiQuotaBatchStorageKey, apiQuotaBatches)
let apiQuotaOfferStore = normalizeApiQuotaOfferStore(readSessionStore<PublicApiQuotaOffer[]>(apiQuotaOfferStorageKey, apiQuotaOffers))
let apiQuotaRoundStore = readSessionStore<ApiQuotaRound[]>(apiQuotaRoundStorageKey, apiQuotaRounds)
let apiQuotaCredentialSummaryStore = readSessionStore<ApiQuotaCredentialSummary[]>(apiQuotaCredentialSummaryStorageKey, apiQuotaCredentialSummaries)
let carpoolApplicationStore = readSessionStore(carpoolApplicationStorageKey, carpoolApplications)
let carpoolApplicationEventStore = readSessionStore(carpoolApplicationEventStorageKey, carpoolApplicationEvents)
let adminAuditLogStore = readSessionStore(adminAuditLogStorageKey, adminAuditLogs)
let officialPriceStore = readSessionStore<OfficialPrice[]>(officialPriceStorageKey, officialPrices)
let carpoolStore = normalizeCarpoolStore(readSessionStore<Carpool[]>(carpoolStorageKey, carpools))
let apiServiceStore = normalizeApiServiceStore(readSessionStore<ApiService[]>(apiServiceStorageKey, apiServices))
let apiServicePaymentSnapshotStore = readSessionStore<Record<string, ApiPaymentOption[]>>(apiServicePaymentSnapshotStorageKey, {})
let apiPaymentAccountSettingsStore = normalizeApiPaymentAccountSettings(readLocalStore<ApiPaymentAccountSettings | null>(apiPaymentAccountSettingsStorageKey, null))
let feedbackTicketStore = readSessionStore<FeedbackTicket[]>(feedbackStorageKey, [])
let notificationReadStore = readSessionStore<string[]>(notificationReadStorageKey, [])
let favoriteStore = readSessionStore<FavoriteRecord[]>(favoriteStorageKey, [])
let myUserProfileStore = clone(myUserProfile)
let myContactMethodStore = clone(myContactMethods)

function clone<T>(value: T): T {
  return structuredClone(value)
}

function readSessionStore<T>(key: string, seed: T): T {
  if (typeof window === 'undefined') return clone(seed)
  const stored = window.sessionStorage.getItem(key)
  if (!stored) return clone(seed)
  const parsed = JSON.parse(stored) as T
  if (isIdRecordArray(seed) && isIdRecordArray(parsed)) {
    return mergeSeedRecords(seed, parsed) as T
  }
  return parsed
}

function readLocalStore<T>(key: string, seed: T): T {
  if (typeof window === 'undefined') return clone(seed)
  const stored = window.localStorage.getItem(key)
  if (!stored) return clone(seed)
  return JSON.parse(stored) as T
}

type IdRecord = { id: string }

function isIdRecordArray(value: unknown): value is IdRecord[] {
  return Array.isArray(value)
    && value.every(item => item !== null && typeof item === 'object' && typeof (item as { id?: unknown }).id === 'string')
}

function mergeSeedRecords<T extends IdRecord>(seed: T[], stored: T[]) {
  const storedIds = new Set(stored.map(item => item.id))
  return [
    ...stored,
    ...clone(seed.filter(item => !storedIds.has(item.id))),
  ]
}

function normalizeApiPurchaseIntentStore(intents: ApiPurchaseIntent[]): ApiPurchaseIntent[] {
  return intents.map(intent => ({
    ...intent,
    status: intent.status,
    quotaUsagePolicySnapshot: normalizeHistoricalApiQuotaUsagePolicy(intent.quotaUsagePolicySnapshot),
  }))
}

function normalizeApiPurchaseIntentEventStore(events: ApiPurchaseIntentEvent[]): ApiPurchaseIntentEvent[] {
  return events.map(event => ({
    ...event,
    fromStatus: event.fromStatus,
    toStatus: event.toStatus,
  }))
}

function mockDeliveryReviewDeadline(submittedAt?: string) {
  const timestamp = submittedAt ? Date.parse(submittedAt) : Number.NaN
  return Number.isFinite(timestamp) ? new Date(timestamp + apiOrderDeliveryReviewWindowMs).toISOString() : undefined
}

function normalizeApiOrderStore(orders: ApiOrder[]): ApiOrder[] {
  return orders.map(order => ({
    ...order,
    orderNo: order.orderNo || createMockApiOrderNo(order.createdAt),
		purchaseKind: order.purchaseKind ?? 'api_service',
		disputeStatus: normalizeApiOrderDisputeStatus(order.disputeStatus),
    completionSource: order.completionSource ?? (order.status === 'completed' ? 'buyer_confirmed' : undefined),
    deliveryReviewExpiresAt: order.deliveryReviewExpiresAt
      ?? (order.status === 'delivery_submitted' ? mockDeliveryReviewDeadline(order.deliverySubmittedAt) : undefined),
    quotaUsagePolicySnapshot: normalizeHistoricalApiQuotaUsagePolicy(order.quotaUsagePolicySnapshot),
  }))
}

function normalizeApiQuotaOfferStore(offers: PublicApiQuotaOffer[]): PublicApiQuotaOffer[] {
  return offers.map(offer => ({
    ...offer,
    quotaUsagePolicy: normalizeHistoricalApiQuotaUsagePolicy(offer.quotaUsagePolicy),
  }))
}

function materializeMockApiOrderReviews(currentTime = Date.now()) {
  let changed = false
  for (const order of apiOrderStore) {
		if (order.status !== 'delivery_submitted' || isApiOrderDisputeActive(order.disputeStatus) || !order.deliveryReviewExpiresAt) continue
    const deadline = Date.parse(order.deliveryReviewExpiresAt)
    if (!Number.isFinite(deadline) || deadline > currentTime) continue
    const completedAt = new Date(deadline).toISOString()
    order.status = 'completed'
    order.completionSource = 'auto_completed'
    order.completedAt = completedAt
    order.updatedAt = completedAt
    order.version += 1
    changed = true
  }
  if (changed) persistApiOrderStore()
}

function createMockApiOrderNo(createdAt: string) {
  const dateParts = new Intl.DateTimeFormat('en', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date(createdAt))
  const datePart = ['year', 'month', 'day']
    .map(type => dateParts.find(part => part.type === type)?.value ?? '')
    .join('')
  let suffix = ''
  while (suffix.length < 10) {
    const bytes = new Uint8Array(16)
    globalThis.crypto.getRandomValues(bytes)
    for (const value of bytes) {
      if (value >= 248) continue
      suffix += apiOrderNumberAlphabet[value % apiOrderNumberAlphabet.length]
      if (suffix.length === 10) break
    }
  }
  return `API-${datePart}-${suffix}`
}

function productPlanForCarpoolName(productName: string) {
  const normalizedName = productName.toLowerCase()
  return carpoolProductCatalog.find(item => normalizedName === item.displayName.toLowerCase())
    ?? carpoolProductCatalog.find(item => normalizedName.includes(item.displayName.toLowerCase()) || item.displayName.toLowerCase().includes(normalizedName))
}

function normalizeCarpoolAccessArrangement(carpool: Carpool): Carpool {
  const legacy = carpool as Carpool & { seatEligibilityMode?: string, officialSeatMechanism?: string }
  const product = productPlanForCarpoolName(carpool.product)
  const normalized = {
    ...carpool,
    distributionMethod: carpool.distributionMethod ?? 'other',
    distributionMethodNote: carpool.distributionMethodNote ?? '历史车源未声明分发方式，需站外确认。',
    providesAdminAccount: carpool.providesAdminAccount ?? false,
  }
  if (carpool.accessArrangementMode && carpool.accessArrangementNote) return normalized
  const mode = legacy.seatEligibilityMode === 'official_member_seat'
    ? 'provider_member_invitation'
    : legacy.seatEligibilityMode === 'not_allowed'
      ? 'not_allowed'
      : product?.accessMode ?? 'owner_managed_access'
  return {
    ...normalized,
    accessArrangementMode: mode,
    accessArrangementNote: carpool.accessArrangementNote ?? legacy.officialSeatMechanism ?? '车主站外说明访问安排，平台不保存凭据。',
    riskAcknowledged: product?.riskAckRequired ? carpool.riskAcknowledged ?? true : carpool.riskAcknowledged,
    riskNoticeCode: product?.riskAckRequired ? carpool.riskNoticeCode ?? product.riskNoticeCode : carpool.riskNoticeCode,
  }
}

function normalizeCarpoolStore(carpools: Carpool[]): Carpool[] {
  return carpools.map(normalizeCarpoolAccessArrangement)
}

function normalizeSub2ApiService(service: ApiService): ApiService {
  return service
}

function isSupportedApiServiceBillingMode(value: unknown): value is Exclude<ApiBillingMode, 'manual_credit'> {
  return value === 'metered_credit' || value === 'fixed_package'
}

function requireSupportedApiServiceBillingMode(value: unknown) {
  if (!isSupportedApiServiceBillingMode(value)) {
    throw new Error('当前版本不支持该 API 服务计费方式。')
  }
  return value
}

function normalizeApiServiceStore(services: ApiService[]): ApiService[] {
  return services.map(service => {
    const normalized = normalizeSub2ApiService(service)
    return {
      ...normalized,
      quotaUsagePolicy: normalizeHistoricalApiQuotaUsagePolicy(normalized.quotaUsagePolicy),
      packages: normalized.packages?.map(item => ({
        ...item,
        quotaUsagePolicy: normalizeHistoricalApiQuotaUsagePolicy(item.quotaUsagePolicy),
      })),
      publiclyOrderable: isSupportedApiServiceBillingMode(normalized.billingMode)
        && (normalized.publiclyOrderable ?? normalized.online),
      expiresAt: normalized.quotaExpiresAt ? formatQuotaExpiresAtLabel(normalized.quotaExpiresAt) || normalized.expiresAt : normalized.expiresAt,
    }
  })
}

export function formatUsdQuota(value: number) {
  return `$${value.toLocaleString('zh-CN')} 美元额度`
}

export function apiUsdQuotaPerCnyLabel(creditPerCny: number) {
  return `¥1 对应 ${formatUsdQuota(creditPerCny)}`
}

export function getApiTTFTBandLabel(value?: ApiTTFTBand) {
  if (value === 'under_1s') return '<1 秒'
  if (value === '1_to_3s') return '1-3 秒'
  if (value === '3_to_5s') return '3-5 秒'
  if (value === '5_to_10s') return '5-10 秒'
  if (value === 'over_10s') return '>10 秒'
  return '未声明'
}

function apiTTFTApproxMinutes(value?: ApiTTFTBand) {
  if (value === 'under_1s') return 1 / 120
  if (value === '1_to_3s') return 2 / 60
  if (value === '3_to_5s') return 4 / 60
  if (value === '5_to_10s') return 7.5 / 60
  if (value === 'over_10s') return 11 / 60
  return 0
}

export function getApiQuotaDistributionLabel(value: ApiQuotaDistributionSystem) {
  if (value === 'sub2api') return 'Sub2API'
  if (value === 'new_api_proxy') return 'NewAPI'
  return '其他接入'
}

export function getApiQuotaSaleModeLabel(value: ApiQuotaSaleMode) {
  return value === 'scheduled' ? '定时放量' : '全天可买'
}

export function getApiQuotaDeliveryModeLabel(value: ApiQuotaDeliveryMode) {
  return value === 'preimported' ? '预导入凭据' : '商户手工交付'
}

function apiCreditPriceCny(service: ApiService) {
  return service.creditPerCny > 0 ? Number((1 / service.creditPerCny).toFixed(4)) : Number.POSITIVE_INFINITY
}

function normalizeModelName(value: string) {
  return value.trim().toLowerCase()
}

export function getSupportedModelPriceRows(service: Pick<ApiService, 'models' | 'modelMultipliers' | 'modelPriceRows'>): ModelPriceRow[] {
  const supported = new Set([
    ...service.models.map(normalizeModelName),
    ...service.modelMultipliers.map(row => normalizeModelName(row.model)),
  ])
  return service.modelPriceRows.filter(row => supported.has(normalizeModelName(row.modelName)))
}

function persistApiPurchaseStores() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(apiPurchaseIntentStorageKey, JSON.stringify(apiPurchaseIntentStore))
  window.sessionStorage.setItem(apiPurchaseIntentEventStorageKey, JSON.stringify(apiPurchaseIntentEventStore))
}

function persistApiOrderStore() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(apiOrderStorageKey, JSON.stringify(apiOrderStore))
}

function persistApiQuotaStores() {
  window.sessionStorage.setItem(apiQuotaBatchStorageKey, JSON.stringify(apiQuotaBatchStore))
  window.sessionStorage.setItem(apiQuotaOfferStorageKey, JSON.stringify(apiQuotaOfferStore))
  window.sessionStorage.setItem(apiQuotaRoundStorageKey, JSON.stringify(apiQuotaRoundStore))
  window.sessionStorage.setItem(apiQuotaCredentialSummaryStorageKey, JSON.stringify(apiQuotaCredentialSummaryStore))
}

function persistCarpoolApplicationStores() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(carpoolApplicationStorageKey, JSON.stringify(carpoolApplicationStore))
  window.sessionStorage.setItem(carpoolApplicationEventStorageKey, JSON.stringify(carpoolApplicationEventStore))
}

function persistAdminStores() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(adminAuditLogStorageKey, JSON.stringify(adminAuditLogStore))
}

function persistMarketStores() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(officialPriceStorageKey, JSON.stringify(officialPriceStore))
  window.sessionStorage.setItem(carpoolStorageKey, JSON.stringify(carpoolStore))
  window.sessionStorage.setItem(apiServiceStorageKey, JSON.stringify(apiServiceStore))
  window.sessionStorage.setItem(apiServicePaymentSnapshotStorageKey, JSON.stringify(apiServicePaymentSnapshotStore))
}

function persistApiPaymentAccountSettings() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(apiPaymentAccountSettingsStorageKey, JSON.stringify(apiPaymentAccountSettingsStore))
}

function persistFeedbackTickets() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(feedbackStorageKey, JSON.stringify(feedbackTicketStore))
}

function persistNotificationReadState() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(notificationReadStorageKey, JSON.stringify(notificationReadStore))
}

function persistFavorites() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(favoriteStorageKey, JSON.stringify(favoriteStore))
}

function nowText() {
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date())
}

function minutesFromNow(minutes: number) {
  const date = new Date(Date.now() + minutes * 60_000)
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function compareTimeDesc(a: string, b: string) {
  return new Date(b).getTime() - new Date(a).getTime()
}

function compareNullableNumberAsc(a: number | null, b: number | null) {
  if (a === null) return b === null ? 0 : 1
  if (b === null) return -1
  return a - b
}

function deadlineTime(value?: string) {
  return value ? new Date(value).getTime() : Number.POSITIVE_INFINITY
}

function profileAvatarText(profile: Pick<UserProfile, 'displayName' | 'username'>) {
  return (profile.displayName || profile.username).slice(0, 1).toUpperCase()
}

function hasPublicProfileApiService(username: string) {
  return apiServiceStore.some(item => item.merchantUsername === username && isApiServicePubliclyOrderable(item) && canOpenApiMerchantProfile(item))
}

function publicBadgesForProfile(username: string, badges: PublicUserProfile['badges']) {
  const canShowApiMerchant = hasPublicProfileApiService(username)
  return badges.filter(badge => {
    if (badge.code === 'linuxdo_bound') return false
    if (badge.code === 'api_merchant' && !canShowApiMerchant) return false
    return true
  })
}

function publicBioForProfile(username: string, bio: string | null) {
  if (!bio || hasPublicProfileApiService(username)) return bio
  return bio
    .replace('个人车主和 API 商户，', '个人车主，')
    .replace('和 API 商户', '')
    .replace('API 商户', 'API 服务参与者')
}

function sanitizePublicUserProfile(profile: PublicUserProfile) {
  return {
    ...profile,
    bio: publicBioForProfile(profile.username, profile.bio),
    badges: publicBadgesForProfile(profile.username, profile.badges),
  }
}

function syncPublicCurrentUser() {
  const target = publicUserProfiles.find(item => item.username === myUserProfileStore.username)
  if (!target) return
  target.displayName = myUserProfileStore.displayName
  target.username = myUserProfileStore.username
  target.bio = publicBioForProfile(myUserProfileStore.username, myUserProfileStore.bio)
  target.avatarUrl = myUserProfileStore.avatarUrl
  target.avatarText = profileAvatarText(myUserProfileStore)
  target.linuxDoBound = myUserProfileStore.linuxDoBinding.bound
  target.linuxDoUsername = myUserProfileStore.linuxDoBinding.linuxDoUsername
  target.trustLevel = myUserProfileStore.linuxDoBinding.trustLevel
  target.badges = publicBadgesForProfile(myUserProfileStore.username, clone(myUserProfileStore.badges))
  target.accountStatus = myUserProfileStore.accountStatus
  target.privacy = clone(myUserProfileStore.privacy)
  target.createdAt = myUserProfileStore.privacy.showCreatedAt ? myUserProfileStore.createdAt : null
  target.lastActiveAt = myUserProfileStore.privacy.showLastActiveAt ? myUserProfileStore.lastActiveAt : null
  if (!myUserProfileStore.privacy.showCompletionStats) {
    target.stats.completedCarpools = null
    target.stats.completedApiOrders = null
    target.stats.completedCarpoolsLast90Days = null
    target.stats.completedApiOrdersLast90Days = null
  }
  if (!myUserProfileStore.privacy.showResponseMedian) target.stats.responseMedianMinutes = null
  if (!myUserProfileStore.privacy.showResolvedDisputeSummary) target.stats.resolvedDisputeCountLast90Days = null
}

function contactMaskedValue(type: ContactMethodType, value: string) {
  const trimmed = value.trim()
  if (type === 'email') {
    const [name, domain] = trimmed.split('@')
    if (!name || !domain) return trimmed
    return `${name.slice(0, 2)}***@${domain}`
  }
  if (type === 'wechat' || type === 'telegram') return `${trimmed.slice(0, 3)}***`
  return trimmed
}

function redactContactItem(item: OrderContactSnapshotItem): OrderContactSnapshotItem {
  const { displayValue, actionUrl, ...redacted } = item
  return redacted
}

function contactSnapshotForVisibility(snapshot: OrderContactSnapshot, canView: boolean, unavailableReason: string | null, contactWindowEndsAt: string | null): OrderContactSnapshot {
  return {
    ...snapshot,
    sellerContacts: canView ? snapshot.sellerContacts : snapshot.sellerContacts.map(redactContactItem),
    buyerContacts: canView ? snapshot.buyerContacts : snapshot.buyerContacts.map(redactContactItem),
    contactWindowEndsAt,
    canView,
    unavailableReason,
  }
}

export function apiIntentMerchantContactSnapshot(intent: ApiPurchaseIntent): OrderContactSnapshot {
  const canView = apiContactVisibleStatuses.includes(intent.status)
  return {
    id: `api-intent-merchant-contact-${intent.id}`,
    orderType: 'api_order',
    orderId: intent.id,
    sellerContacts: canView ? intent.contactChannels.map(channel => ({
      type: channel.type,
      label: channel.label,
      maskedValue: contactMaskedValue(channel.type, channel.value),
      displayValue: channel.value,
      verified: channel.type === 'linuxdo',
      usageScope: 'api_merchant',
      actionUrl: channel.type === 'linuxdo' ? linuxDoProfileSummaryUrl(channel.value) : undefined,
    })) : [],
    buyerContacts: [],
    contactWindowEndsAt: null,
    canView,
    unavailableReason: canView ? null : '只有购买意向参与方可以查看冻结联系方式。',
    createdAt: intent.updatedAt,
  }
}

export function apiIntentBuyerContactSnapshot(intent: ApiPurchaseIntent): OrderContactSnapshot {
  const canView = apiContactVisibleStatuses.includes(intent.status)
  return {
    id: `api-intent-buyer-contact-${intent.id}`,
    orderType: 'api_order',
    orderId: intent.id,
    sellerContacts: [],
    buyerContacts: canView ? (intent.buyerContactChannels ?? []).map(channel => ({
      type: channel.type,
      label: channel.label,
      maskedValue: contactMaskedValue(channel.type, channel.value),
      displayValue: channel.value,
      verified: channel.type === 'linuxdo',
      usageScope: 'buyer',
      actionUrl: channel.type === 'linuxdo' ? linuxDoProfileSummaryUrl(channel.value) : undefined,
    })) : [],
    contactWindowEndsAt: null,
    canView,
    unavailableReason: canView ? null : '只有购买意向参与方可以查看冻结联系方式。',
    createdAt: intent.updatedAt,
  }
}

function contactChannelsToSnapshotItems(channels: ApiContactChannel[], usageScope: ContactUsageScope) {
  return channels.map(channel => ({
    type: channel.type,
    label: channel.label,
    maskedValue: contactMaskedValue(channel.type, channel.value),
    displayValue: channel.value,
    verified: channel.type === 'linuxdo',
    usageScope,
    actionUrl: channel.type === 'linuxdo' ? linuxDoProfileSummaryUrl(channel.value) : undefined,
  }))
}

export function apiOrderMerchantContactSnapshot(order: ApiOrder): OrderContactSnapshot {
  return {
    id: `api-order-merchant-contact-${order.id}`,
    orderType: 'api_order',
    orderId: order.id,
    sellerContacts: contactChannelsToSnapshotItems(order.merchantContactChannels, 'api_merchant'),
    buyerContacts: [],
    contactWindowEndsAt: null,
    canView: true,
    unavailableReason: null,
    createdAt: order.updatedAt,
  }
}

export function apiOrderBuyerContactSnapshot(order: ApiOrder): OrderContactSnapshot {
  return {
    id: `api-order-buyer-contact-${order.id}`,
    orderType: 'api_order',
    orderId: order.id,
    sellerContacts: [],
    buyerContacts: contactChannelsToSnapshotItems(order.buyerContactChannels, 'buyer'),
    contactWindowEndsAt: null,
    canView: true,
    unavailableReason: null,
    createdAt: order.updatedAt,
  }
}

function defaultContactLabel(type: ContactMethodType) {
  const labels: Record<ContactMethodType, string> = {
    linuxdo: 'linux.do 私信',
    wechat: '微信',
    email: '邮箱',
    telegram: 'Telegram',
    other: '其他联系',
  }
  return labels[type]
}

type ApiMerchantProfileSource = Pick<ApiService, 'merchantIdentityMode' | 'merchantUsername'>

export function canOpenApiMerchantProfile(source: ApiMerchantProfileSource | Pick<ApiPurchaseIntent['snapshot'], 'merchantIdentityMode'>) {
  return source.merchantIdentityMode === 'public_profile'
}

export function getApiMerchantProfileUrl(source: ApiMerchantProfileSource | Pick<ApiPurchaseIntent['snapshot'], 'merchantIdentityMode' | 'merchantUsername'>) {
  if (!canOpenApiMerchantProfile(source)) return null
  return `/u/${source.merchantUsername}`
}

export function getApiMerchantAvatarText(source: ApiMerchantIdentitySource | ApiIntentMerchantSource) {
  return getApiMerchantDisplayName(source).slice(0, 1).toUpperCase()
}

export function getApiMerchantVisibilityLabel(source: Pick<ApiService, 'merchantIdentityMode'> | Pick<ApiPurchaseIntent['snapshot'], 'merchantIdentityMode'>) {
  return source.merchantIdentityMode === 'store_alias' ? '不公开社区用户名' : '公开个人身份'
}

export type UpdateMyProfileRequest = {
  displayName: string
  username: string
  bio: string | null
  regionCode: string | null
  timezone: string | null
  avatarMode: AvatarMode
  avatarUrl?: string | null
  privacy?: UserPrivacySettings
}

export type SetBackupPasswordRequest = {
  currentPassword?: string
  newPassword: string
}

export type EmailVerificationChallenge = {
  email: string
  expiresAt: string
  devCode?: string
}

export type SaveContactMethodRequest = {
  type: ContactMethodType
  label: string
  displayValue: string
  usageScopes: ContactUsageScope[]
  isDefault: boolean
  enabled: boolean
}

function normalizeMerchantDisplayName(payload: Record<string, unknown>) {
  const mode = payload.merchantIdentityMode === 'public_profile' ? 'public_profile' : 'store_alias'
  const displayName = String(payload.merchantDisplayName ?? '').trim()
  return {
    merchantIdentityMode: mode,
    merchantDisplayName: mode === 'store_alias' ? displayName : currentMerchantName,
  }
}

export function getApiDeliveryModeLabel(_mode: ApiDeliveryMode) {
  return 'API 细节'
}

export function getApiDeliveryModeDescription(mode: ApiDeliveryMode) {
  return mode === 'sub2api_panel_account'
    ? '买家创建订单后，双方站外确认 API 细节；平台不在公开页或列表展示面板账号、密码、token 或登录态。'
    : '买家创建订单后，双方站外确认 API 细节、限速和鉴权边界；平台不在公开页或列表展示 API Key 或 endpoint 密钥。'
}

export function getApiDeliveryModesLabel(modes: ApiDeliveryMode[]) {
  const labels = modes.length ? modes.map(getApiDeliveryModeLabel) : [getApiDeliveryModeLabel('api_key_endpoint')]
  return [...new Set(labels)].join(' / ')
}

export function getApiServiceDefaultPaymentMethod(service: ApiService): ApiPaymentOption['paymentMethod'] | null {
  const supported = service.acceptedPaymentMethods?.find(isApiPaymentMethod)
  if (supported) return supported
  return apiServicePaymentSnapshot(service.id).find(option => option.enabled && isApiPaymentOptionComplete(option))?.paymentMethod ?? null
}

export function getApiIntentDefaultPaymentMethod(intent: ApiPurchaseIntent): ApiPaymentOption['paymentMethod'] | null {
  return intent.snapshot.paymentOptions?.find(option => option.enabled && isApiPaymentOptionComplete(option))?.paymentMethod ?? null
}

export function getApiServicePublicDetailUrl(service: Pick<ApiService, 'id' | 'online' | 'publiclyOrderable'>) {
  return isApiServicePubliclyOrderable(service) ? `/api-market/${service.id}` : null
}

export function getReadableStatus(value: string | null | undefined) {
  if (!value) return '-'
  const labels: Record<string, string> = {
    approved_offline: '审核通过，待上线',
    online: '在线',
    offline: '离线',
    paused: '暂停接单',
    reviewing: '审核中',
    under_review: '申诉复核中',
    need_more_information: '需要补充信息',
    pending_owner: '待车主处理',
    accepted_reserved: '席位已预留',
    open: '意向已创建',
    contacted: '商户已记录联系',
    ordered: '已生成订单',
    buyer_cancelled: '买家已取消',
    owner_closed: '商户已关闭',
  }
  return labels[value] ?? value
}

export function getApiUsageVisibilityLabel(value: ApiService['usageVisibility']) {
  const labels: Record<ApiService['usageVisibility'], string> = {
    none: '未展示用量核对',
    merchant_readonly: '商户说明，买家自行核对',
    panel_realtime: '商户说明，买家自行核对',
  }
  return labels[value]
}

export function getApiStatusLabel(status: ApiPurchaseIntentStatus) {
  const labels: Record<ApiPurchaseIntentStatus, string> = {
    open: '意向已创建',
    contacted: '商户已记录联系',
    ordered: '已生成订单',
    buyer_cancelled: '买家已取消',
    owner_closed: '商户已关闭',
  }
  return labels[status]
}

export function getApiOrderStatusLabel(status: ApiOrderStatus, role: ApiOrderViewerRole = 'admin') {
  const labels: Record<ApiOrderStatus, string> = {
    pending_payment: '待付款',
    payment_submitted: '买家已付款',
    payment_issue: '付款待补充',
    paid_confirmed: '已确认收款',
    delivery_submitted: role === 'buyer' ? '待核验凭证' : role === 'merchant' ? '已完成交付' : '买家核验期',
    completed: '已完成',
    cancelled: '已取消',
  }
  return labels[status]
}

export function getApiOrderDisplayStatus(order: ApiOrder, role: ApiOrderViewerRole) {
  if (order.status !== 'completed') return getApiOrderStatusLabel(order.status, role)
  if (order.completionSource === 'auto_completed') return '已自动完成'
  if (order.completionSource === 'buyer_confirmed') return role === 'buyer' ? '已确认凭证可用' : '买家已确认完成'
  return '已完成'
}

export function getApiOrderCompletionSourceLabel(source?: ApiOrderCompletionSource) {
  if (source === 'buyer_confirmed') return '买家主动确认'
  if (source === 'auto_completed') return '核验期结束后系统自动完成'
  return '尚未完成'
}

export function getApiOrderEventLabel(type: ApiOrderEvent['type']) {
  const labels: Record<ApiOrderEvent['type'], string> = {
    created: '创建订单',
    payment_submitted: '买家提交付款状态',
    payment_issue_reported: '商户报告付款问题',
    payment_confirmed: '商户确认收款',
    delivery_submitted: '商户完成交付',
    completed: '订单完成',
    cancelled: '订单取消',
  }
  return labels[type]
}

export function isApiOrderReceiptConfirmed(status: ApiOrderStatus) {
  return status === 'paid_confirmed' || status === 'delivery_submitted' || status === 'completed'
}

export function getApiOrderDeliveryKindLabel(kind: ApiOrderDeliveryKind) {
  return kind === 'login_account' ? '登录账号接入' : 'API Key 接入'
}

export function getApiOrderPaymentIssueLabel(reason?: ApiOrderPaymentIssueReason) {
  if (reason === 'not_received') return '未到账'
  if (reason === 'amount_mismatch') return '金额不符'
  if (reason === 'remark_mismatch') return '备注不符'
  return '付款信息待补充'
}

export function getApiOrderNextAction(order: ApiOrder, role: 'buyer' | 'merchant') {
  if (role === 'buyer') {
    if (order.status === 'pending_payment') return '查看收款资料并付款'
    if (order.status === 'payment_submitted') return '等待商户确认收款'
    if (order.status === 'payment_issue') return '补充付款信息并重新提交'
    if (order.status === 'paid_confirmed') return '等待商户交付'
    if (order.status === 'delivery_submitted') {
      switch (normalizeApiOrderDisputeStatus(order.disputeStatus)) {
        case 'negotiating': return '继续协商订单问题'
        case 'open': return '等待平台处理凭证问题'
        case 'awaiting_fulfillment': return '等待裁决要求履行'
        case 'fulfillment_confirmation': return '确认履行结果'
        default: return '核验凭证，或报告问题'
      }
    }
    if (order.status === 'completed') return order.completionSource === 'auto_completed' ? '订单已自动完成' : '凭证已确认可用'
    if (order.status === 'cancelled') return '查看取消原因'
  }
  if (order.status === 'pending_payment') return '等待买家付款'
  if (order.status === 'payment_submitted') return '确认已收款'
  if (order.status === 'payment_issue') return '等待买家补充付款信息'
  if (order.status === 'paid_confirmed') return '填写交付信息'
  if (order.status === 'delivery_submitted') return '已完成交付，无需操作'
  if (order.status === 'completed') return order.completionSource === 'auto_completed' ? '订单已自动完成' : '订单已完成'
  return '查看详情'
}

export function isApiOrderBuyerActionRequired(order: ApiOrder) {
  return order.status === 'pending_payment' || order.status === 'payment_issue' || order.status === 'delivery_submitted'
}

export function isApiOrderMerchantActionRequired(order: ApiOrder) {
  return order.status === 'payment_submitted' || order.status === 'paid_confirmed'
}

export function getCarpoolAccessArrangementLabel(mode: Carpool['accessArrangementMode']) {
  if (mode === 'personal_account_cost_share') return '费用分摊'
  if (mode === 'provider_member_invitation') return '成员邀请'
  if (mode === 'owner_managed_access') return '车主管理'
  if (mode === 'other_off_platform') return '站外安排'
  if (mode === 'not_allowed') return '不可上架'
  return '待说明'
}

export function isHighRiskSubscriptionCarpool(carpool: Pick<Carpool, 'product' | 'riskNoticeCode'>) {
  return Boolean(carpool.riskNoticeCode) || /chatgpt|openai/i.test(carpool.product)
}

export function getCarpoolApplicationStatusLabel(status: CarpoolApplicationStatus) {
  const labels: Record<CarpoolApplicationStatus, string> = {
    pending_owner: '等待车主处理',
    accepted_reserved: '席位已预留',
    waiting_contact: '等待买家联系车主',
    contacted: '已联系车主',
    joined_pending_confirmation: '等待车主确认已上车',
    active: '服务中',
    pending_completion: '等待双方确认本次完成',
    completed: '已完成',
    rejected: '已拒绝',
    cancelled_by_buyer: '买家已取消',
    cancelled_by_owner: '车主已取消',
    expired: '联系窗口已过期',
    disputed: '纠纷中',
  }
  return labels[status]
}

export function isCarpoolBuyerActionRequired(application: CarpoolApplication) {
  return ['accepted_reserved', 'waiting_contact', 'contacted', 'pending_completion', 'disputed'].includes(application.status)
}

export function isCarpoolOwnerActionRequired(application: CarpoolApplication) {
  return ['pending_owner', 'joined_pending_confirmation', 'pending_completion', 'disputed'].includes(application.status)
}

export function getCarpoolApplicationNextAction(application: CarpoolApplication, role: 'buyer' | 'owner') {
  if (role === 'buyer') {
    if (application.status === 'pending_owner') return '等待车主处理'
    if (application.status === 'accepted_reserved' || application.status === 'waiting_contact') return '已联系车主'
    if (application.status === 'contacted') return '确认已经上车'
    if (application.status === 'joined_pending_confirmation') return '等待车主确认'
    if (application.status === 'active') return '查看服务记录'
    if (application.status === 'pending_completion') return '确认本次完成'
    if (application.status === 'completed' && !application.buyerReview) return '评价车主'
    if (application.status === 'disputed') return '查看纠纷'
    return '查看详情'
  }

  if (application.status === 'pending_owner') return '处理申请'
  if (application.status === 'accepted_reserved' || application.status === 'waiting_contact') return '等待买家联系'
  if (application.status === 'contacted') return '确认用户已上车'
  if (application.status === 'joined_pending_confirmation') return '确认用户已上车'
  if (application.status === 'pending_completion') return '确认本次完成'
  if (application.status === 'disputed') return '处理纠纷'
  return '查看详情'
}

function isOngoingCarpoolApplication(status: CarpoolApplicationStatus) {
  return !['completed', 'rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'].includes(status)
}

function isReservedCarpoolApplication(status: CarpoolApplicationStatus) {
  return ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation'].includes(status)
}

function isActiveCarpoolApplication(status: CarpoolApplicationStatus) {
  return ['active', 'pending_completion'].includes(status)
}

function buildCarpoolSnapshot(carpool: Carpool): CarpoolApplication['snapshot'] {
  const pricing = getPricingDisplay(carpool)
  return {
    carpoolId: carpool.id,
    productName: carpool.product,
    regionName: carpool.region,
    monthlyPriceCny: pricing.primaryPrice,
    serviceMultiplier: carpool.serviceMultiplier,
    weeklyQuotaAmount: carpool.weeklyQuotaAmount,
    quotaLabel: carpool.quotaLabel,
    quotaUnit: carpool.quotaUnit,
    quotaPeriod: carpool.quotaPeriod,
    priceLabel: pricing.primaryLabel,
    openingChannelName: carpool.openingMethod,
    paymentMethodNames: carpool.openingMethod === 'Apple Store' ? ['Apple Pay'] : carpool.openingMethod === '本地卡' ? ['其他'] : ['站外协商'],
    warrantyText: carpool.warranty,
    rulesVersion: nowText(),
    rulesText: '按车源当前规则申请上车；平台只记录意向和状态，不托管支付或账号。',
    ownerUserId: `owner-${carpool.owner}`,
    ownerUsername: carpool.owner,
    ownerTrustLevel: carpool.trustLevel,
    ownerType: carpool.ownerType,
    accessArrangementMode: carpool.accessArrangementMode,
    accessArrangementNote: carpool.accessArrangementNote,
    riskNoticeCode: carpool.riskNoticeCode,
    riskAcknowledged: carpool.riskAcknowledged,
  }
}

export function getCarpoolSeatSummary(carpool: Carpool): CarpoolSeatSummary {
  const related = carpoolApplicationStore.filter(item => item.carpoolId === carpool.id)
  const reservedSeatCount = related
    .filter(item => isReservedCarpoolApplication(item.status))
    .reduce((sum, item) => sum + item.seatsRequested, 0)
  const activeSessionSeats = related
    .filter(item => isActiveCarpoolApplication(item.status))
    .reduce((sum, item) => sum + item.seatsRequested, 0)
  const activeMemberCount = carpool.currentConfirmedMembers + activeSessionSeats
  return {
    carpoolId: carpool.id,
    totalSeats: carpool.maxMembers,
    activeMemberCount,
    reservedSeatCount,
    availableSeats: Math.max(0, carpool.maxMembers - activeMemberCount - reservedSeatCount),
  }
}

export function getCarpoolApplyDisabledReason(carpool: Carpool, seatSummary?: Pick<CarpoolSeatSummary, 'availableSeats'> | null, hasOngoingApplication = false, currentUserId = '', hasActiveMembership = false) {
  const eligibility = evaluateCarpoolApplicationEligibility(carpool, seatSummary, hasOngoingApplication, currentUserId, hasActiveMembership)
  return eligibility.canApply ? '' : eligibility.reason
}

function appendCarpoolApplicationEvent(event: Omit<CarpoolApplicationEvent, 'id' | 'createdAt'> & { createdAt?: string }) {
  carpoolApplicationEventStore.unshift({
    id: `ride-event-${Date.now()}-${carpoolApplicationEventStore.length + 1}`,
    createdAt: event.createdAt ?? nowText(),
    ...event,
  })
  persistCarpoolApplicationStores()
}

function appendAdminAuditLog(log: Omit<AdminAuditLog, 'id' | 'createdAt'> & { createdAt?: string }) {
  adminAuditLogStore.unshift({
    id: `audit-${Date.now()}-${adminAuditLogStore.length + 1}`,
    createdAt: log.createdAt ?? nowText(),
    ...log,
  })
  persistAdminStores()
}

function findCarpoolApplication(id: string) {
  const application = carpoolApplicationStore.find(item => item.id === id)
  if (!application) throw new Error(`Carpool application not found: ${id}`)
  return application
}

function updateCarpoolApplication(id: string, updater: (application: CarpoolApplication) => void) {
  const application = findCarpoolApplication(id)
  updater(application)
  application.updatedAt = nowText()
  persistCarpoolApplicationStores()
  return clone(application)
}

function startCarpoolServiceIfBothConfirmed(application: CarpoolApplication) {
  if (!application.buyerConfirmedJoinedAt || !application.ownerConfirmedJoinedAt) return false
  application.status = 'active'
  application.startedAt = application.startedAt ?? nowText()
  application.expectedEndAt = application.expectedEndAt ?? minutesFromNow(30 * 24 * 60)
  return true
}

function completeCarpoolIfBothConfirmed(application: CarpoolApplication) {
  if (!application.buyerConfirmedCompletedAt || !application.ownerConfirmedCompletedAt) return false
  application.status = 'completed'
  application.completedAt = application.completedAt ?? nowText()
  application.completionMode = 'mutual'
  return true
}

export function isBuyerActionRequired(intent: ApiPurchaseIntent) {
  return intent.status === 'open' || intent.status === 'contacted'
}

export function isMerchantActionRequired(intent: ApiPurchaseIntent) {
  return intent.status === 'open'
}

export function getApiIntentNextAction(intent: ApiPurchaseIntent, role: 'buyer' | 'merchant') {
  if (role === 'buyer') {
    if (intent.status === 'open' || intent.status === 'contacted') return '查看商户联系方式'
    if (intent.status === 'ordered') return '前往订单继续处理'
    if (intent.status === 'buyer_cancelled') return '查看取消原因'
    if (intent.status === 'owner_closed') return '查看商户关闭原因'
    return '查看详情'
  }

  if (intent.status === 'open') return '记录已联系'
  if (intent.status === 'contacted') return '可关闭意向'
  if (intent.status === 'ordered') return '订单已生成'
  return '查看详情'
}

function appendApiIntentEvent(event: Omit<ApiPurchaseIntentEvent, 'id' | 'createdAt'> & { createdAt?: string }) {
  const row: ApiPurchaseIntentEvent = {
    id: `api-event-${Date.now()}-${apiPurchaseIntentEventStore.length + 1}`,
    createdAt: event.createdAt ?? nowText(),
    ...event,
  }
  apiPurchaseIntentEventStore.unshift(row)
  persistApiPurchaseStores()
}

function findApiPurchaseIntent(id: string) {
  const intent = apiPurchaseIntentStore.find(item => item.id === id)
  if (!intent) throw new Error(`API purchase intent not found: ${id}`)
  return intent
}

function updateApiPurchaseIntent(id: string, updater: (intent: ApiPurchaseIntent) => void) {
  const intent = findApiPurchaseIntent(id)
  updater(intent)
  intent.updatedAt = nowText()
  persistApiPurchaseStores()
  return clone(intent)
}

function apiServicePaymentSnapshot(serviceId: string) {
  return normalizeApiPaymentAccountSettings({
    paymentOptions: apiServicePaymentSnapshotStore[serviceId] ?? [],
  }).paymentOptions.filter(option => option.enabled)
}

function normalizeRawApiPaymentOptions(options: Array<{ paymentMethod?: string, enabled?: boolean, paymentInstructions?: string, paymentQrCodeDataUrl?: string | null }>) {
  return options.flatMap(option => {
    const paymentMethod = String(option.paymentMethod ?? '')
    if (!isApiPaymentMethod(paymentMethod)) return []
    return {
      paymentMethod,
      enabled: Boolean(option.enabled),
      paymentInstructions: String(option.paymentInstructions ?? ''),
      paymentQrCodeDataUrl: normalizeQrCodeDataUrl(option.paymentQrCodeDataUrl),
    }
  })
}

function createSnapshot(service: ApiService): ApiPurchaseIntent['snapshot'] {
  return {
    serviceId: service.id,
    serviceTitle: service.title,
    sourceUrl: service.sourceUrl,
    merchantId: service.merchantId,
    merchant: service.merchant,
    merchantUsername: service.merchantUsername,
    merchantIdentityMode: service.merchantIdentityMode,
    merchantDisplayName: getApiMerchantDisplayName(service),
    trustLevel: service.trustLevel,
    merchantType: service.merchantType,
    models: [...service.models],
    multiplier: service.rate,
    defaultMultiplier: service.defaultMultiplier,
    creditPerCny: service.creditPerCny,
    warranty: service.warranty,
    refundPolicy: service.refundPolicy,
    accountPoolType: service.accountPoolType,
    accountPoolLabel: service.accountPoolLabel,
    declaredMaxConcurrency: service.declaredMaxConcurrency,
    promptAuditEnabled: service.promptAuditEnabled ?? null,
    merchantRefundCommitment: service.merchantRefundCommitment,
    merchantRefundPolicyVersion: service.merchantRefundPolicyVersion,
    serviceValidityExpiresAt: service.quotaExpiresAt ?? null,
    usageVisibility: service.usageVisibility,
    supportedDeliveryModes: [...service.deliveryModes],
    selectedDeliveryMode: service.deliveryModes[0],
    minimumPurchaseCny: service.minimumPurchaseCny,
    panelBaseUrl: service.panelBaseUrl,
    apiBaseUrlVisibility: service.apiBaseUrlVisibility,
    panelLoginUrlVisibility: service.panelLoginUrlVisibility,
    panelRequiresPasswordReset: service.panelRequiresPasswordReset,
    expiresAt: service.expiresAt,
    officialPricingVersion: service.officialPricingVersion,
    officialPricingUpdatedAt: service.officialPricingUpdatedAt,
    modelPrices: clone(getSupportedModelPriceRows(service)),
    paymentOptions: apiServicePaymentSnapshot(service.id),
  }
}

function createPackageSnapshot(item: ApiServicePackage): ApiServicePackageSnapshot {
  return {
    id: item.id,
    name: item.name,
    priceCny: item.priceCny,
    panelAllowance: item.panelAllowance,
    durationDays: item.durationDays,
    description: item.description,
    models: item.models.map(model => ({
      serviceModelId: model.serviceModelId,
      modelCatalogId: model.modelCatalogId,
      modelPriceVersionId: model.modelPriceVersionId,
      modelName: model.modelName,
      merchantMultiplier: model.merchantMultiplier,
    })),
  }
}

function apiServicePublicSearchTerms(item: ApiService) {
  const terms = [item.id, item.title, getApiMerchantDisplayName(item), ...item.models]
  if (canOpenApiMerchantProfile(item)) terms.push(item.merchant, item.merchantUsername)
  if (item.sourceUrl) terms.push(item.sourceUrl)
  return terms
}

function apiIntentPublicSearchTerms(item: ApiPurchaseIntent) {
  return [item.id, item.snapshot.serviceTitle, getApiMerchantDisplayName(item), item.buyer]
}

function userProfileAliases(username: string) {
  const values = new Set([username])
  const userProfile = publicUserProfiles.find(item => item.username === username || item.displayName === username || item.linuxDoUsername === username)
  if (userProfile) {
    values.add(userProfile.username)
    values.add(userProfile.displayName)
    if (userProfile.linuxDoUsername) values.add(userProfile.linuxDoUsername)
  }
  const merchantProfile = publicMerchantProfiles.find(item => item.username === username || item.displayName === username)
  if (merchantProfile) {
    values.add(merchantProfile.username)
    values.add(merchantProfile.displayName)
  }
  return values
}

function profileMatchesUsername(recordUsername: string, profileUsername: string) {
  return userProfileAliases(profileUsername).has(recordUsername)
}

function dateFromDateTime(value: string | null | undefined) {
  if (!value) return nowText().split(' ')[0]
  return value.split(' ')[0]
}

function buildPublicReviewFromCarpoolApplication(application: CarpoolApplication): PublicReviewRecord | null {
  if (application.status !== 'completed' || !application.buyerReview) return null
  return {
    id: `public-carpool-review-${application.id}`,
    username: application.ownerUsername,
    date: dateFromDateTime(application.buyerReview.createdAt ?? application.completedAt ?? application.updatedAt),
    serviceType: application.snapshot.productName,
    tags: application.buyerReview.tags,
    note: application.buyerReview.note,
    verified: true,
  }
}

function publicReviewsForProfile(username: string) {
  const staticReviews = publicReviewRecords.filter(item => item.verified && profileMatchesUsername(item.username, username))
  const carpoolReviews = carpoolApplicationStore
    .map(buildPublicReviewFromCarpoolApplication)
    .filter((item): item is PublicReviewRecord => item !== null && profileMatchesUsername(item.username, username))
  const byId = new Map<string, PublicReviewRecord>()
  for (const review of [...staticReviews, ...carpoolReviews]) {
    byId.set(review.id, review)
  }
  return Array.from(byId.values()).sort((a, b) => compareTimeDesc(a.date, b.date))
}

function adminTargetLink(row: AdminRow) {
  if (row.targetType === 'official-price') return `/official-prices/${row.id}`
  if (row.targetType === 'carpool') return `/carpools/${row.id}`
  if (row.targetType === 'api-intent') return `/my/api-orders/${row.id}`
  if (row.targetType === 'api-order') return `/admin/api-orders/${row.id}`
  if (row.targetType === 'carpool-application') return `/merchant/carpool-applications/${row.id}`
  if (row.targetType === 'feedback-ticket') return `/admin/feedback/${row.id}`
  if (row.targetType === 'user') return `/u/${row.primary}`
  return null
}

function defaultSortForRole(role: 'buyer' | 'merchant') {
  return (a: ApiPurchaseIntent, b: ApiPurchaseIntent) => {
    const aAction = role === 'buyer' ? isBuyerActionRequired(a) : isMerchantActionRequired(a)
    const bAction = role === 'buyer' ? isBuyerActionRequired(b) : isMerchantActionRequired(b)
    return Number(bAction) - Number(aAction)
      || deadlineTime(a.merchantResponseDeadline) - deadlineTime(b.merchantResponseDeadline)
      || compareTimeDesc(a.updatedAt, b.updatedAt)
  }
}

export type ApiPurchaseIntentFilters = {
  buyerId?: string
  merchantId?: string
  status?: ApiPurchaseIntentStatus | ApiPurchaseIntentStatus[]
  deliveryMode?: ApiDeliveryMode
  serviceId?: string
  search?: string
  dateRange?: 'all' | 'today' | '7d' | '30d'
  sort?: 'default_buyer' | 'default_merchant' | 'updated_desc' | 'created_desc' | 'amount_desc' | 'amount_asc'
}

export type ApiOrderFilters = {
  buyerId?: string
  sellerId?: string
  status?: ApiOrderStatus | ApiOrderStatus[]
  serviceId?: string
  search?: string
  dateRange?: 'all' | 'today' | '7d' | '30d'
  sort?: 'default_buyer' | 'default_merchant' | 'updated_desc' | 'created_desc' | 'amount_desc' | 'amount_asc'
  risk?: 'all' | 'high' | 'has_note'
}

export type ApiServiceFilters = {
  model?: string
  maxMultiplier?: number
  deliveryMode?: ApiDeliveryMode
  usageVisibility?: ApiUsageVisibility
  gateway?: ApiService['delivery']
  online?: boolean
  state?: ApiServiceState
  merchantType?: ApiService['merchantType']
  merchantPreference?: 'personal_first' | 'personal' | 'api'
  hasWarranty?: boolean
  trustLevel?: number
  minimumPurchaseCnyMax?: number
  minBalance?: number
  sort?: 'recommended' | 'multiplier_asc' | 'response_fast' | 'recent'
  search?: string
  billingMode?: ApiBillingMode
  packageModelCatalogId?: string
  packageDurationDays?: number
}

export type MinimumPurchaseFilter = 'all' | 'lte_20' | 'between_21_50' | 'gt_50'

export type Sub2ApiMarketSort = 'recommended' | 'credit_price_asc' | 'minimum_purchase_asc' | 'panel_supported' | 'response_fast' | 'recent'

export type OtherApiMarketSort = 'recommended' | 'minimum_purchase_asc' | 'response_fast' | 'recent'

export type Sub2ApiMarketFilters = {
  search?: string
  model?: string
  creditPriceMax?: number
  deliveryMode?: ApiDeliveryMode
  imageCapability?: 'all' | 'supported' | 'none'
  minimumPurchase?: MinimumPurchaseFilter
  online?: boolean
  merchantPreference?: 'personal_first' | 'personal' | 'api'
  trustLevel?: number
  sort?: Sub2ApiMarketSort
}

export type OtherApiMarketFilters = {
  search?: string
  distributionSystem?: ApiService['delivery'] | 'all'
  billingMode?: ApiBillingMode | 'all'
  deliveryMode?: ApiDeliveryMode
  minimumPurchase?: MinimumPurchaseFilter
  online?: boolean
  sort?: OtherApiMarketSort
}

export type CreateApiPurchaseIntentPayload = {
  serviceId: string
  purchaseAmountCny: number
  deliveryMode: ApiDeliveryMode
  targetModel: string
  selectedPackageId?: string
  buyerNote?: string
}

export type ReviewCarpoolApplicationPayload = Pick<CarpoolApplicationReview, 'rating' | 'tags' | 'note'>

export type CarpoolApplicationFilters = {
  buyerId?: string
  ownerId?: string
  status?: CarpoolApplicationStatus | CarpoolApplicationStatus[]
  carpoolId?: string
  search?: string
  sort?: 'default_buyer' | 'default_owner' | 'updated_desc' | 'created_desc'
}

function filterApiPurchaseIntents(filters: ApiPurchaseIntentFilters = {}) {
  const keyword = filters.search?.trim().toLowerCase()
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  const now = Date.now()
  const rangeMs = filters.dateRange === 'today' ? 24 * 60 * 60 * 1000 : filters.dateRange === '7d' ? 7 * 24 * 60 * 60 * 1000 : filters.dateRange === '30d' ? 30 * 24 * 60 * 60 * 1000 : null

  const rows = apiPurchaseIntentStore.filter(item => {
    const createdAt = new Date(item.createdAt).getTime()
    return (!filters.buyerId || item.buyerId === filters.buyerId)
      && (!filters.merchantId || item.merchantId === filters.merchantId)
      && (!statuses || statuses.includes(item.status))
      && (!filters.deliveryMode || item.selectedDeliveryMode === filters.deliveryMode)
      && (!filters.serviceId || item.serviceId === filters.serviceId)
      && (!rangeMs || now - createdAt <= rangeMs)
      && (!keyword || apiIntentPublicSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
  })

  const sort = filters.sort ?? 'updated_desc'
  return rows.sort((a, b) => {
    if (sort === 'default_buyer') return defaultSortForRole('buyer')(a, b)
    if (sort === 'default_merchant') return defaultSortForRole('merchant')(a, b)
    if (sort === 'created_desc') return compareTimeDesc(a.createdAt, b.createdAt)
    if (sort === 'amount_desc') return b.purchaseAmountCny - a.purchaseAmountCny
    if (sort === 'amount_asc') return a.purchaseAmountCny - b.purchaseAmountCny
    return compareTimeDesc(a.updatedAt, b.updatedAt)
  })
}

function apiOrderSearchTerms(order: ApiOrder) {
  return [order.orderNo, order.id, order.apiPurchaseIntentId, order.serviceTitle, order.buyer, order.seller, getApiMerchantDisplayName({ merchant: order.seller, snapshot: order.intentSnapshot })]
}

function defaultApiOrderSortForRole(role: 'buyer' | 'merchant') {
  return (a: ApiOrder, b: ApiOrder) => {
    const aAction = role === 'buyer' ? isApiOrderBuyerActionRequired(a) : isApiOrderMerchantActionRequired(a)
    const bAction = role === 'buyer' ? isApiOrderBuyerActionRequired(b) : isApiOrderMerchantActionRequired(b)
    return Number(bAction) - Number(aAction)
      || compareTimeDesc(a.updatedAt, b.updatedAt)
  }
}

function filterApiOrders(filters: ApiOrderFilters = {}) {
  materializeMockApiOrderReviews()
  const keyword = filters.search?.trim().toLowerCase()
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  const now = Date.now()
  const rangeMs = filters.dateRange === 'today' ? 24 * 60 * 60 * 1000 : filters.dateRange === '7d' ? 7 * 24 * 60 * 60 * 1000 : filters.dateRange === '30d' ? 30 * 24 * 60 * 60 * 1000 : null
  const rows = apiOrderStore.filter(item => {
    const createdAt = new Date(item.createdAt).getTime()
    return (!filters.buyerId || item.buyerId === filters.buyerId)
      && (!filters.sellerId || item.sellerId === filters.sellerId)
      && (!statuses || statuses.includes(item.status))
      && (!filters.serviceId || item.apiServiceId === filters.serviceId)
      && (!rangeMs || now - createdAt <= rangeMs)
      && (!keyword || matchesApiOrderSearch(keyword, apiOrderSearchTerms(item)))
  })

  const sort = filters.sort ?? 'updated_desc'
  return rows.sort((a, b) => {
    if (sort === 'default_buyer') return defaultApiOrderSortForRole('buyer')(a, b)
    if (sort === 'default_merchant') return defaultApiOrderSortForRole('merchant')(a, b)
    if (sort === 'created_desc') return compareTimeDesc(a.createdAt, b.createdAt)
    if (sort === 'amount_desc') return b.amount - a.amount
    if (sort === 'amount_asc') return a.amount - b.amount
    return compareTimeDesc(a.updatedAt, b.updatedAt)
  })
}

function defaultCarpoolSortForRole(role: 'buyer' | 'owner') {
  return (a: CarpoolApplication, b: CarpoolApplication) => {
    const aAction = role === 'buyer' ? isCarpoolBuyerActionRequired(a) : isCarpoolOwnerActionRequired(a)
    const bAction = role === 'buyer' ? isCarpoolBuyerActionRequired(b) : isCarpoolOwnerActionRequired(b)
    return Number(bAction) - Number(aAction)
      || deadlineTime(a.reservedUntil ?? undefined) - deadlineTime(b.reservedUntil ?? undefined)
      || compareTimeDesc(a.updatedAt, b.updatedAt)
  }
}

function filterCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  const keyword = filters.search?.trim().toLowerCase()
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  const rows = carpoolApplicationStore.filter(item => {
    return (!filters.buyerId || item.applicantUserId === filters.buyerId)
      && (!filters.ownerId || item.ownerUserId === filters.ownerId)
      && (!statuses || statuses.includes(item.status))
      && (!filters.carpoolId || item.carpoolId === filters.carpoolId)
      && (!keyword || [item.id, item.snapshot.productName, item.snapshot.regionName, item.applicantUsername, item.ownerUsername].some(value => value.toLowerCase().includes(keyword)))
  })

  const sort = filters.sort ?? 'updated_desc'
  return rows.sort((a, b) => {
    if (sort === 'default_buyer') return defaultCarpoolSortForRole('buyer')(a, b)
    if (sort === 'default_owner') return defaultCarpoolSortForRole('owner')(a, b)
    if (sort === 'created_desc') return compareTimeDesc(a.createdAt, b.createdAt)
    return compareTimeDesc(a.updatedAt, b.updatedAt)
  })
}

export async function getHomeMarket() {
  if (shouldUseRealBackend()) {
    const [officialPrices, carpools, apiServices] = await Promise.all([
      backendOfficialPrices(),
      backendGetCarpools(),
      backendAPIServices({ online: true }),
    ])

    return clone({ categoryRows, officialPrices, carpools, apiServices: apiServices.filter(isApiServicePubliclyOrderable), productTrends, transactionRecords, apiPurchaseIntents: apiPurchaseIntentStore })
  }
  await wait()
  return clone({ categoryRows, officialPrices: officialPriceStore, carpools: carpoolStore, apiServices: apiServiceStore.filter(isApiServicePubliclyOrderable), productTrends, transactionRecords, apiPurchaseIntents: apiPurchaseIntentStore })
}

export async function getTransactionTrendSummary(productId: string, range: TransactionTrendRange): Promise<TransactionTrendSummary | null> {
  await wait()
  const trend = productTrends.find(item => item.slug === productId)
  if (!trend) return null

  const points = trend.points[range]
  const validTransactions = transactionRecords.filter(item => {
    return item.productSlug === productId
      && item.status === 'completed'
      && !item.hasUnresolvedDispute
      && Number.isFinite(item.finalSettlementPrice)
  })
  const latestPoint = [...points].reverse().find(item => item.transactionCount > 0)
  const medianValues = points.map(item => item.medianPrice)
  const p25Values = points.map(item => item.p25Price)
  const p75Values = points.map(item => item.p75Price)

  return clone({
    productId,
    productName: trend.label,
    range,
    latestTransactionPrice: validTransactions[0]?.finalSettlementPrice ?? latestPoint?.medianPrice ?? null,
    medianPrice: medianValues.length ? Math.round(medianValues.reduce((sum, item) => sum + item, 0) / medianValues.length) : null,
    p25Price: p25Values.length ? Math.min(...p25Values) : null,
    p75Price: p75Values.length ? Math.max(...p75Values) : null,
    validSampleCount: points.reduce((sum, item) => sum + item.transactionCount, 0),
    points,
    updatedAt: trend.verifiedAt,
  })
}

export async function getOfficialPrices() {
  if (shouldUseRealBackend()) return backendOfficialPrices()
  await wait()
  return clone(officialPriceStore)
}

export type OfficialPriceListPageFilters = OfficialPricePageFilters & {
  product?: string
  plan?: string
  regionLabel?: string
  channelLabel?: string
  openingMethodLabel?: string
  sourceLabel?: string
  trustFloor?: number
}

export async function getOfficialPricesPage(filters: OfficialPriceListPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<OfficialPrice>> {
  if (shouldUseRealBackend()) return backendOfficialPricesPage(filters, page)
  await wait()
  const query = filters.q?.trim().toLowerCase()
  const rows = officialPriceStore.filter(item => {
    return (!query || [item.product, item.plan, item.region, item.channel, item.submitter, item.source].some(value => value.toLowerCase().includes(query)))
      && (!filters.product || item.product.includes(filters.product))
      && (!filters.plan || item.plan.includes(filters.plan))
      && (!filters.regionLabel || item.region.includes(filters.regionLabel))
      && (!filters.channelLabel || item.channel.includes(filters.channelLabel))
      && (!filters.status || item.status === filters.status)
      && (!filters.openingMethodLabel || item.openingMethod.includes(filters.openingMethodLabel))
      && (!filters.sourceLabel || item.source.includes(filters.sourceLabel))
      && (filters.trustFloor === undefined || item.submitterTrust >= filters.trustFloor)
  })
  rows.sort((a, b) => {
    if (filters.sort === 'cny_asc') return (a.cny ?? Number.POSITIVE_INFINITY) - (b.cny ?? Number.POSITIVE_INFINITY)
    if (filters.sort === 'trust_desc') return b.submitterTrust - a.submitterTrust
    if (filters.sort === 'verified_recent' || filters.sort === 'submitted_desc' || filters.sort === 'updated_desc') return b.updatedAt.localeCompare(a.updatedAt)
    return Number(b.isLowest) - Number(a.isLowest) || b.updatedAt.localeCompare(a.updatedAt)
  })
  return paginateCursorItems(clone(rows), page)
}

export async function getOfficialPriceById(id: string) {
  if (shouldUseRealBackend()) return backendOfficialPriceById(id)
  await wait()
  return clone(officialPriceStore.find(item => item.id === id) ?? null)
}

export async function getMyOfficialPriceLeads() {
  if (shouldUseRealBackend()) return backendMyOfficialPriceLeads()
  await wait()
  return clone(officialPriceStore.filter(item => item.submitter === currentBuyerName || item.status !== '已验证'))
}

export async function getCarpools() {
  if (shouldUseRealBackend()) return backendGetCarpools()
  await wait()
  return clone(carpoolStore.map(item => ({ ...item, seatSummary: getCarpoolSeatSummary(item) })))
}

export type CarpoolListPageFilters = CarpoolPageFilters & {
  category?: ProductCategoryKey
  plan?: string
  openingMethod?: string
}

export async function getCarpoolsPage(filters: CarpoolListPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<CarpoolWithMeta>> {
  if (shouldUseRealBackend()) return backendGetCarpoolsPage(filters, page)
  await wait()
  if (filters.none) return { items: [] }
  const rows = carpoolStore
    .filter(item => (!filters.category || productMatchesCategory(item.product, filters.category)))
    .filter(item => (!filters.plan || productMatchesPlan(item.product, filters.plan)))
    .filter(item => (!filters.region || item.region === filters.region))
    .filter(item => (!filters.ownerType || item.ownerType === filters.ownerType))
    .filter(item => (!filters.warranty || item.warranty === filters.warranty))
    .filter(item => (!filters.openingMethod || item.openingMethod === filters.openingMethod))
    .map(item => ({ ...item, seatSummary: getCarpoolSeatSummary(item) }))
  rows.sort((a, b) => {
    if (filters.sort === 'price_asc') return compareByTradablePrice(a, b)
    if (filters.sort === 'updated_desc') return b.confirmedAt.localeCompare(a.confirmedAt)
    if (filters.sort === 'seats_desc') return getCarpoolSeatSummary(b).availableSeats - getCarpoolSeatSummary(a).availableSeats
    return Number(b.confirmedWithin48h) - Number(a.confirmedWithin48h)
      || Number(a.ownerType !== '商户车源') - Number(b.ownerType !== '商户车源')
      || compareByTradablePrice(a, b)
  })
  return paginateCursorItems(clone(rows), page)
}

export async function getCarpoolById(id: string) {
  if (shouldUseRealBackend()) return backendGetCarpoolById(id)
  await wait()
  const carpool = carpoolStore.find(item => item.id === id)
  return clone(carpool ? { ...carpool, seatSummary: getCarpoolSeatSummary(carpool) } : null)
}

export async function getCarpoolApplicationEligibility(id: string): Promise<CarpoolApplicationEligibility> {
  if (shouldUseRealBackend()) return backendCarpoolApplicationEligibility(id)
  await wait()
  const carpool = carpoolStore.find(item => item.id === id)
  if (!carpool) throw new Error('车源不存在。')
  const related = carpoolApplicationStore.filter(item => item.carpoolId === id && item.applicantUserId === currentBuyerId)
  const hasActiveMembership = related.some(item => ['active', 'pending_completion'].includes(item.status))
  const hasOngoingApplication = related.some(item => !['completed', 'rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'].includes(item.status))
  return clone(evaluateCarpoolApplicationEligibility(carpool, getCarpoolSeatSummary(carpool), hasOngoingApplication, currentBuyerId, hasActiveMembership))
}

export async function getMyCarpools() {
  if (shouldUseRealBackend()) return backendOwnerCarpools()
  await wait()
  return clone(carpoolStore
    .filter(item => item.owner === currentOwnerName)
    .map(item => ({ ...item, seatSummary: getCarpoolSeatSummary(item) })))
}

export async function getMyCarpoolsPage(page: CursorPageRequest = {}): Promise<CursorPage<CarpoolWithMeta>> {
  if (shouldUseRealBackend()) return backendOwnerCarpoolsPage(page)
  return paginateCursorItems(await getMyCarpools(), page)
}

export async function getCarpoolProductCatalog() {
  if (shouldUseRealBackend()) return backendCarpoolProductCatalog()
  await wait()
  return clone(carpoolProductCatalog.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder))
}

export async function getCarpoolRegions() {
  if (shouldUseRealBackend()) return backendCarpoolRegions()
  await wait()
  return clone(carpoolRegions.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder))
}

export async function getCarpoolOpeningChannels() {
  if (shouldUseRealBackend()) return backendCarpoolOpeningChannels()
  await wait()
  return clone(carpoolOpeningChannels.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder))
}

export async function getCarpoolPaymentMethods() {
  if (shouldUseRealBackend()) return backendCarpoolPaymentMethods()
  await wait()
  return clone(carpoolPaymentMethods.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder))
}

export async function getModelCatalog() {
  if (shouldUseRealBackend()) return backendModelCatalog()
  await wait()
  return clone(getMockPublicAPIModels())
}

function filterApiServices(filters: ApiServiceFilters = {}) {
  const keyword = filters.search?.trim().toLowerCase()
  return apiServiceStore
    .filter(item => {
      return (!filters.model || item.models.some(model => model.toLowerCase().includes(filters.model!.toLowerCase())))
        && (!filters.maxMultiplier || item.defaultMultiplier <= filters.maxMultiplier)
        && (!filters.deliveryMode || item.deliveryModes.includes(filters.deliveryMode))
        && (!filters.usageVisibility || item.usageVisibility === filters.usageVisibility)
        && (!filters.gateway || item.delivery === filters.gateway)
        && (filters.online === undefined || isApiServicePubliclyOrderable(item) === filters.online)
        && (!filters.state || item.state === filters.state)
        && (!filters.merchantType || item.merchantType === filters.merchantType)
        && (!filters.merchantPreference || filters.merchantPreference === 'personal_first' || (filters.merchantPreference === 'personal' ? item.merchantType !== '商户' : item.merchantType === '商户'))
        && (filters.hasWarranty === undefined || (filters.hasWarranty ? item.warranty.includes('补') || item.warranty.includes('承诺') || item.warranty.includes('24') : item.warranty.includes('无') || item.warranty.includes('协商')))
        && (!filters.trustLevel || (item.trustLevel !== null && item.trustLevel >= filters.trustLevel))
        && (!filters.minimumPurchaseCnyMax || item.minimumPurchaseCny <= filters.minimumPurchaseCnyMax)
        && (!filters.minBalance || item.balance >= filters.minBalance)
        && (!keyword || apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
        && (!filters.billingMode || item.billingMode === filters.billingMode)
        && ((!filters.packageModelCatalogId && !filters.packageDurationDays) || (item.packages ?? []).some(packageItem => {
          return packageItem.enabled
            && packageItem.stockAvailable > 0
            && (!filters.packageDurationDays || packageItem.durationDays === filters.packageDurationDays)
            && (!filters.packageModelCatalogId || packageItem.models.some(model => model.modelCatalogId === filters.packageModelCatalogId))
        }))
    })
    .sort((a, b) => {
      if (filters.sort === 'multiplier_asc') return a.defaultMultiplier - b.defaultMultiplier || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
      if (filters.sort === 'response_fast') return compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes) || a.defaultMultiplier - b.defaultMultiplier
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      const aPersonal = a.merchantType !== '商户'
      const bPersonal = b.merchantType !== '商户'
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
        || Number(bPersonal) - Number(aPersonal)
        || Number(a.unresolvedDisputes === 0) - Number(b.unresolvedDisputes === 0)
        || Number(b.deliveryModes.length) - Number(a.deliveryModes.length)
        || Number(b.usageVisibility === 'panel_realtime') - Number(a.usageVisibility === 'panel_realtime')
        || a.defaultMultiplier - b.defaultMultiplier
        || compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
    })
}

function matchesMinimumPurchaseFilter(value: number, filter?: MinimumPurchaseFilter) {
  if (!filter || filter === 'all') return true
  if (filter === 'lte_20') return value <= 20
  if (filter === 'between_21_50') return value >= 21 && value <= 50
  return value > 50
}

function filterSub2ApiMarketServices(filters: Sub2ApiMarketFilters = {}) {
  const keyword = filters.search?.trim().toLowerCase()
  return apiServiceStore
    .filter(item => item.delivery === 'Sub2API')
    .filter(item => {
      return (!keyword || apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
        && (!filters.model || item.models.some(model => model.toLowerCase().includes(filters.model!.toLowerCase())))
        && (!filters.creditPriceMax || apiCreditPriceCny(item) <= filters.creditPriceMax)
        && (!filters.deliveryMode || item.deliveryModes.includes(filters.deliveryMode))
        && (!filters.imageCapability || filters.imageCapability === 'all' || (filters.imageCapability === 'supported' ? item.imagePricing.supported : !item.imagePricing.supported))
        && matchesMinimumPurchaseFilter(item.minimumPurchaseCny, filters.minimumPurchase)
        && (filters.online === undefined || isApiServicePubliclyOrderable(item) === filters.online)
        && (!filters.merchantPreference || filters.merchantPreference === 'personal_first' || (filters.merchantPreference === 'personal' ? item.merchantType !== '商户' : item.merchantType === '商户'))
        && (!filters.trustLevel || (item.trustLevel !== null && item.trustLevel >= filters.trustLevel))
    })
    .sort((a, b) => {
      if (filters.sort === 'credit_price_asc') return apiCreditPriceCny(a) - apiCreditPriceCny(b) || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'minimum_purchase_asc') return a.minimumPurchaseCny - b.minimumPurchaseCny || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
      if (filters.sort === 'panel_supported') return Number(b.deliveryModes.includes('sub2api_panel_account')) - Number(a.deliveryModes.includes('sub2api_panel_account')) || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
      if (filters.sort === 'response_fast') return compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes) || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || apiCreditPriceCny(a) - apiCreditPriceCny(b)
        || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
        || Number(b.deliveryModes.includes('sub2api_panel_account')) - Number(a.deliveryModes.includes('sub2api_panel_account'))
        || compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
    })
}

function filterOtherApiMarketServices(filters: OtherApiMarketFilters = {}) {
  const keyword = filters.search?.trim().toLowerCase()
  return apiServiceStore
    .filter(item => item.delivery !== 'Sub2API')
    .filter(item => {
      return (!keyword || apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
        && (!filters.distributionSystem || filters.distributionSystem === 'all' || item.delivery === filters.distributionSystem)
        && (!filters.billingMode || filters.billingMode === 'all' || item.billingMode === filters.billingMode)
        && (!filters.deliveryMode || item.deliveryModes.includes(filters.deliveryMode))
        && matchesMinimumPurchaseFilter(item.minimumPurchaseCny, filters.minimumPurchase)
        && (filters.online === undefined || isApiServicePubliclyOrderable(item) === filters.online)
    })
    .sort((a, b) => {
      if (filters.sort === 'minimum_purchase_asc') return a.minimumPurchaseCny - b.minimumPurchaseCny || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
      if (filters.sort === 'response_fast') return compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes) || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || compareNullableNumberAsc(a.responseMedianMinutes, b.responseMedianMinutes)
        || a.minimumPurchaseCny - b.minimumPurchaseCny
        || compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
    })
}

export async function getApiServices(filters: ApiServiceFilters = {}) {
  if (shouldUseRealBackend()) return backendAPIServices(filters)
  await wait()
  return clone(filterApiServices(filters))
}

export async function getApiServicesPage(filters: ApiServiceFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<ApiService>> {
  if (shouldUseRealBackend()) return backendAPIServicesPage(filters, page)
  await wait()
  return clone(paginateCursorItems(filterApiServices(filters), page))
}

export async function getSub2ApiMarketServices(filters: Sub2ApiMarketFilters = {}) {
  if (shouldUseRealBackend()) return backendSub2APIServices(filters)
  await wait()
  return clone(filterSub2ApiMarketServices(filters))
}

export async function getOtherApiMarketServices(filters: OtherApiMarketFilters = {}) {
  if (shouldUseRealBackend()) return backendOtherAPIServices(filters)
  await wait()
  return clone(filterOtherApiMarketServices(filters))
}

function matchesApiQuotaOfferFilters(item: PublicApiQuotaOffer, filters: ApiQuotaOfferFilters) {
  const keyword = filters.search?.trim().toLowerCase()
  return item.status === 'published'
    && (filters.distributionSystem === undefined || filters.distributionSystem === 'all' || item.distributionSystem === filters.distributionSystem)
    && (!filters.oneMultiplier || item.modelMultiplier === '1.0000')
    && (!filters.onlyOrderable || item.isOrderable)
    && (!filters.excludeSystemSlots || !item.currentRound?.systemSlotKey && !item.nextRound?.systemSlotKey)
    && (!keyword || [item.name, item.serviceTitle, item.sellerDisplayName, item.distributionSystem]
      .some(value => value.toLowerCase().includes(keyword)))
}

function mockApiQuotaSaleSlots(now = new Date()): ApiQuotaSystemSaleSlotList {
  const serverNow = now.toISOString()
  const beijingDate = formatBeijingDateTimeInput(now).slice(0, 10)
  const [year, month, day] = beijingDate.split('-').map(Number)
  const baseDay = Date.UTC(year, month - 1, day)
  const items: ApiQuotaSystemSaleSlot[] = []
  for (let dayOffset = 0; dayOffset < 7; dayOffset += 1) {
    const date = new Date(baseDay + dayOffset * 24 * 60 * 60 * 1000)
    const dateKey = [
      date.getUTCFullYear(),
      String(date.getUTCMonth() + 1).padStart(2, '0'),
      String(date.getUTCDate()).padStart(2, '0'),
    ].join('-')
    for (const hour of [9, 13, 20]) {
      const time = `${String(hour).padStart(2, '0')}:00`
      const startsAt = beijingDateTimeInputToISOString(`${dateKey}T${time}`)
      const startsAtMs = Date.parse(startsAt)
      const endsAtMs = startsAtMs + 30 * 60 * 1000
      const registrationClosesAtMs = startsAtMs - 60 * 60 * 1000
      let state: ApiQuotaSystemSaleSlot['state'] = 'registration_open'
      if (now.getTime() >= endsAtMs) state = 'ended'
      else if (now.getTime() >= startsAtMs) state = 'active'
      else if (now.getTime() >= registrationClosesAtMs) state = 'registration_closed'
      items.push({
        key: `${dateKey}@${time}`,
        startsAt,
        endsAt: new Date(endsAtMs).toISOString(),
        registrationClosesAt: new Date(registrationClosesAtMs).toISOString(),
        state,
      })
    }
  }
  return { serverNow, items }
}

function projectMockSystemRushOffer(item: PublicApiQuotaOffer, now = Date.now()) {
  const round = apiQuotaRoundStore.find(candidate => candidate.allocations.some(allocation => allocation.offerId === item.id) && candidate.systemSlotKey)
  if (!round) return item
  const projected = clone(item)
  const allocation = round.allocations.find(candidate => candidate.offerId === item.id)
  projected.currentRound = undefined
  projected.nextRound = undefined
  projected.availableCopies = allocation?.availableCopies ?? 0
  if (now < Date.parse(round.startsAt)) {
    projected.nextRound = clone(round)
    projected.isOrderable = false
    projected.orderabilityCode = 'not_started'
    projected.orderabilityReason = '本场尚未开抢。'
  } else if (now < Date.parse(round.endsAt)) {
    projected.currentRound = clone(round)
    projected.isOrderable = projected.batchStatus === 'published'
      && projected.status === 'published'
      && projected.availableCopies > 0
      && (projected.deliveryMode !== 'preimported' || projected.credentialAvailableCopies >= projected.availableCopies)
    projected.orderabilityCode = projected.isOrderable
      ? 'orderable'
      : projected.availableCopies <= 0
        ? 'sold_out'
        : projected.deliveryMode === 'preimported'
          ? 'credential_unavailable'
          : 'service_unavailable'
    projected.orderabilityReason = projected.isOrderable
      ? '正在抢购。'
      : projected.availableCopies <= 0
        ? '本场已售罄。'
        : projected.deliveryMode === 'preimported'
          ? '可用交付凭据不足。'
          : '本场暂不可购买。'
  } else {
    projected.availableCopies = 0
    projected.isOrderable = false
    projected.orderabilityCode = 'round_ended'
    projected.orderabilityReason = '本场已结束。'
  }
  return projected
}

export async function getApiQuotaSaleSlots() {
  if (shouldUseRealBackend()) return backendAPIQuotaSaleSlots()
  await wait()
  return mockApiQuotaSaleSlots()
}

export async function getApiQuotaOffers(filters: ApiQuotaOfferFilters = {}) {
  if (shouldUseRealBackend()) return backendPublicAPIQuotaOffers(filters)
  await wait()
  const rows = apiQuotaOfferStore.map(item => projectMockSystemRushOffer(item))
  return clone(rows.filter(item => {
    if (filters.slotKey) {
      const round = apiQuotaRoundStore.find(candidate => candidate.systemSlotKey === filters.slotKey && candidate.allocations.some(allocation => allocation.offerId === item.id))
      if (!round) return false
      item.currentRound = Date.now() >= Date.parse(round.startsAt) && Date.now() < Date.parse(round.endsAt) ? clone(round) : item.currentRound
      item.nextRound = Date.now() < Date.parse(round.startsAt) ? clone(round) : item.nextRound
    }
    return matchesApiQuotaOfferFilters(item, filters)
  }))
}

export async function getApiQuotaOffersPage(filters: ApiQuotaOfferFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<PublicApiQuotaOffer>> {
  if (shouldUseRealBackend()) return backendPublicAPIQuotaOffersPage(filters, page)
  await wait()
  const rows = apiQuotaOfferStore.map(item => projectMockSystemRushOffer(item))
  const filtered = rows.filter(item => {
    if (filters.slotKey) {
      const round = apiQuotaRoundStore.find(candidate => candidate.systemSlotKey === filters.slotKey && candidate.allocations.some(allocation => allocation.offerId === item.id))
      if (!round) return false
      item.currentRound = Date.now() >= Date.parse(round.startsAt) && Date.now() < Date.parse(round.endsAt) ? clone(round) : item.currentRound
      item.nextRound = Date.now() < Date.parse(round.startsAt) ? clone(round) : item.nextRound
    }
    return matchesApiQuotaOfferFilters(item, filters)
  })
  return clone(paginateCursorItems(filtered, page))
}

export async function getApiQuotaOfferById(id: string) {
  if (shouldUseRealBackend()) return backendPublicAPIQuotaOffer(id)
  await wait()
  return clone(apiQuotaOfferStore.find(item => item.id === id && item.status === 'published') ?? null)
}

export async function getOwnerApiQuotaBatches(apiServiceId: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIQuotaBatches(apiServiceId)
  await wait()
  return clone(apiQuotaBatchStore.filter(item => item.apiServiceId === apiServiceId))
}

export async function createApiQuotaBatch(payload: CreateApiQuotaBatchPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIQuotaBatch(payload)
  await wait()
  const expiresAt = Date.parse(payload.expiresAt)
  const saleCutoffAt = Date.parse(payload.saleCutoffAt)
  if (!Number.isFinite(expiresAt) || !Number.isFinite(saleCutoffAt)) throw new Error('请填写有效的停售和失效时间。')
  if (saleCutoffAt > expiresAt - 60 * 60 * 1000) throw new Error('最晚下单时间必须早于失效时间至少 1 小时。')
  if (!Number.isFinite(Number(payload.declaredTotalUsdAllowance)) || Number(payload.declaredTotalUsdAllowance) <= 0) throw new Error('声明美元额度必须大于 0。')
  const batch: ApiQuotaBatch = {
    id: `quota-batch-${Date.now()}`,
    apiServiceId: payload.apiServiceId,
    sourceType: payload.sourceType,
    sourceLabel: payload.sourceLabel?.trim() || undefined,
    status: 'draft',
    declaredTotalUsdAllowance: normalizeDecimalTrimmed(payload.declaredTotalUsdAllowance, 6),
    unallocatedUsdAllowance: normalizeDecimalTrimmed(payload.declaredTotalUsdAllowance, 6),
    saleCutoffAt: new Date(saleCutoffAt).toISOString(),
    expiresAt: new Date(expiresAt).toISOString(),
    sourceConfirmedAt: new Date(payload.sourceConfirmedAt).toISOString(),
    version: 1,
  }
  apiQuotaBatchStore.unshift(batch)
  persistApiQuotaStores()
  return clone(batch)
}

export async function getOwnerApiQuotaOffers(batchId: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIQuotaOffers(batchId)
  await wait()
  return clone(apiQuotaOfferStore.filter(item => item.batchId === batchId).map(item => ({
    id: item.id,
    batchId: item.batchId,
    apiServiceId: item.apiServiceId,
    distributionSystem: item.distributionSystem,
    name: item.name,
    usdAllowance: item.usdAllowance,
    priceCny: item.priceCny,
    cnyPerUsd: item.cnyPerUsd,
    modelMultiplier: item.modelMultiplier,
    quotaUsagePolicy: item.quotaUsagePolicy,
    deliveryMode: item.deliveryMode,
    deliveryEtaMinutes: item.deliveryEtaMinutes,
    saleMode: item.saleMode,
    status: item.status,
    sortOrder: item.sortOrder,
    publishedAt: item.publishedAt,
    version: item.version,
  })))
}

function quotaDistributionFromService(service: ApiService): ApiQuotaDistributionSystem {
  if (service.delivery === 'Sub2API') return 'sub2api'
  if (service.delivery === 'NewAPI Proxy') return 'new_api_proxy'
  return 'other'
}

export async function createApiQuotaOffer(payload: CreateApiQuotaOfferPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIQuotaOffer(payload)
  await wait()
  if (payload.deliveryMode !== 'manual') throw new Error('新额度包只支持卖家手工交付。')
  const batch = apiQuotaBatchStore.find(item => item.id === payload.batchId)
  const service = batch ? apiServiceStore.find(item => item.id === batch.apiServiceId) : undefined
  if (!batch || !service) throw new Error('未找到额度批次或关联服务。')
  if (batch.status !== 'draft') throw new Error('只有草稿批次可以新增额度规格。')
  const usdAllowance = normalizeDecimalTrimmed(payload.usdAllowance, 6)
  const priceCny = normalizeDecimal(payload.priceCny, 2)
  const cnyPerUsd = normalizeDecimalTrimmed(divideDecimal(priceCny, usdAllowance, 6), 6)
  const quotaUsagePolicy = apiQuotaUsagePolicyFromInput(payload.quotaUsagePolicy)
  const offer: PublicApiQuotaOffer = {
    id: `quota-offer-${Date.now()}`,
    batchId: batch.id,
    apiServiceId: service.id,
    distributionSystem: quotaDistributionFromService(service),
    name: payload.name.trim(),
    usdAllowance,
    priceCny,
    cnyPerUsd,
    modelMultiplier: normalizeDecimal(payload.modelMultiplier, 4),
    quotaUsagePolicy,
    deliveryMode: payload.deliveryMode,
    deliveryEtaMinutes: payload.deliveryEtaMinutes,
    saleMode: payload.saleMode,
    status: 'draft',
    sortOrder: payload.sortOrder,
    version: 1,
    batchStatus: 'published',
    serviceTitle: service.title,
    sellerDisplayName: getApiMerchantDisplayName(service),
    sellerIdentityType: service.merchantType === '商户' ? 'merchant' : 'individual',
    sellerLinuxDoBound: true,
    promptAuditEnabled: service.promptAuditEnabled ?? null,
    declaredMaxConcurrency: service.declaredMaxConcurrency ?? 1,
    saleCutoffAt: batch.saleCutoffAt,
    expiresAt: batch.expiresAt,
    availableCopies: payload.saleMode === 'continuous' ? payload.continuousCopies : 0,
    credentialAvailableCopies: 0,
    isOrderable: false,
    orderabilityCode: 'service_unavailable',
    orderabilityReason: '额度批次尚未发布。',
  }
  apiQuotaOfferStore.push(offer)
  persistApiQuotaStores()
  return clone((await getOwnerApiQuotaOffers(batch.id)).find(item => item.id === offer.id)!)
}

export async function createApiQuotaRushOffer(payload: CreateApiQuotaRushOfferPayload): Promise<ApiQuotaRushOfferPublication> {
  if (shouldUseRealBackend()) return backendCreateAPIQuotaRushOffer(payload)
  await wait()
  if (payload.deliveryMode !== 'manual') throw new Error('新额度包只支持卖家手工交付。')
  const service = apiServiceStore.find(item => item.id === payload.apiServiceId && item.merchantUsername === myUserProfileStore.username)
  if (!service) throw new Error('未找到可发布额度包的 API 服务。')
  const slot = mockApiQuotaSaleSlots().items.find(item => item.key === payload.slotKey)
  if (!slot) throw new Error('请选择平台开放的固定场次。')
  if (slot.state !== 'registration_open') throw new Error('该场次已经停止报名，请选择更晚场次。')
  const expiresAt = Date.parse(payload.expiresAt)
  if (!Number.isFinite(expiresAt) || expiresAt < Date.parse(slot.endsAt) + 60 * 60 * 1000) {
    throw new Error('额度失效时间必须至少晚于场次结束 1 小时。')
  }
  const copies = Math.trunc(payload.copies)
  if (copies < 1 || copies > 5000) throw new Error('计划份数必须在 1-5000 之间。')
  const usdAllowance = normalizeDecimalTrimmed(payload.usdAllowance, 6)
  const priceCny = normalizeDecimal(payload.priceCny, 2)
  const modelMultiplier = normalizeDecimal(payload.modelMultiplier, 4)
  const quotaUsagePolicy = apiQuotaUsagePolicyFromInput(payload.quotaUsagePolicy)
  if (Number(usdAllowance) <= 0 || Number(priceCny) <= 0 || Number(modelMultiplier) <= 0) {
    throw new Error('美元额度、人民币总价和模型倍率必须大于 0。')
  }
  if (payload.file || payload.deliveryKind) throw new Error('新额度包不接收预导入凭据。')
  const credentialImported = 0

  const createdAt = nowText()
  const unique = Date.now()
  const batchId = `quota-batch-rush-${unique}`
  const offerId = `quota-offer-rush-${unique}`
  const roundId = `quota-round-rush-${unique}`
  const allocation = {
    id: `quota-allocation-rush-${unique}`,
    offerId,
    saleRoundId: roundId,
    saleMode: 'scheduled' as const,
    copyLimit: copies,
    availableCopies: copies,
    reservedCopies: 0,
    consumedCopies: 0,
    allocatedUsdAllowance: normalizeDecimalTrimmed(String(Number(usdAllowance) * copies), 6),
    returnedUsdAllowance: '0',
    status: 'active' as const,
  }
  const batch: ApiQuotaBatch = {
    id: batchId,
    apiServiceId: service.id,
    sourceType: payload.sourceType,
    sourceLabel: payload.sourceLabel?.trim() || undefined,
    status: 'published',
    declaredTotalUsdAllowance: allocation.allocatedUsdAllowance,
    unallocatedUsdAllowance: '0',
    saleCutoffAt: slot.endsAt,
    expiresAt: new Date(expiresAt).toISOString(),
    sourceConfirmedAt: new Date(payload.sourceConfirmedAt).toISOString(),
    publishedAt: createdAt,
    version: 2,
  }
  const round: ApiQuotaRound = {
    id: roundId,
    batchId,
    systemSlotKey: slot.key,
    name: `${slot.key.slice(0, 10)} ${slot.key.slice(11)} 场`,
    startsAt: slot.startsAt,
    endsAt: slot.endsAt,
    status: 'scheduled',
    allocations: [allocation],
    version: 1,
  }
  const offer: PublicApiQuotaOffer = {
    id: offerId,
    batchId,
    apiServiceId: service.id,
    distributionSystem: quotaDistributionFromService(service),
    name: payload.name.trim(),
    usdAllowance,
    priceCny,
    cnyPerUsd: normalizeDecimalTrimmed(divideDecimal(priceCny, usdAllowance, 6), 6),
    modelMultiplier,
    quotaUsagePolicy,
    deliveryMode: payload.deliveryMode,
    deliveryEtaMinutes: payload.deliveryEtaMinutes,
    saleMode: 'scheduled',
    status: 'published',
    sortOrder: 0,
    publishedAt: createdAt,
    version: 1,
    batchStatus: 'published',
    serviceTitle: service.title,
    sellerDisplayName: getApiMerchantDisplayName(service),
    sellerIdentityType: service.merchantType === '商户' ? 'merchant' : 'individual',
    sellerLinuxDoBound: true,
    promptAuditEnabled: service.promptAuditEnabled ?? null,
    declaredMaxConcurrency: service.declaredMaxConcurrency ?? 1,
    saleCutoffAt: slot.endsAt,
    expiresAt: batch.expiresAt,
    nextRound: round,
    availableCopies: copies,
    credentialAvailableCopies: credentialImported,
    isOrderable: false,
    orderabilityCode: 'not_started',
    orderabilityReason: '本场尚未开抢。',
  }
  const credentialSummary: ApiQuotaCredentialSummary = {
    offerId,
    available: credentialImported,
    reserved: 0,
    delivered: 0,
    retired: 0,
  }
  apiQuotaBatchStore.unshift(batch)
  apiQuotaOfferStore.unshift(offer)
  apiQuotaRoundStore.unshift(round)
  apiQuotaCredentialSummaryStore.unshift(credentialSummary)
  persistApiQuotaStores()
  return clone({
    batch,
    offer,
    round,
    credentialImported,
    credentialSummary,
  })
}

export async function getOwnerApiQuotaRounds(batchId: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIQuotaRounds(batchId)
  await wait()
  return clone(apiQuotaRoundStore.filter(item => item.batchId === batchId))
}

export async function createApiQuotaRound(payload: CreateApiQuotaRoundPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIQuotaRound(payload)
  await wait()
  const batch = apiQuotaBatchStore.find(item => item.id === payload.batchId)
  if (!batch || batch.status !== 'draft') throw new Error('只有草稿批次可以新增放量轮次。')
  const startsAt = new Date(payload.startsAt).toISOString()
  const endsAt = new Date(payload.endsAt).toISOString()
  if (Date.parse(endsAt) <= Date.parse(startsAt)) throw new Error('轮次结束时间必须晚于开始时间。')
  const roundId = `quota-round-${Date.now()}`
  const allocations = payload.offers.map((requested, index) => {
    const offer = apiQuotaOfferStore.find(item => item.id === requested.offerId && item.batchId === payload.batchId)
    if (!offer || offer.saleMode !== 'scheduled') throw new Error('轮次只能分配同批次的定时额度规格。')
    return {
      id: `quota-allocation-${Date.now()}-${index}`,
      offerId: offer.id,
      saleRoundId: roundId,
      saleMode: 'scheduled' as const,
      copyLimit: requested.copies,
      availableCopies: requested.copies,
      reservedCopies: 0,
      consumedCopies: 0,
      allocatedUsdAllowance: normalizeDecimalTrimmed(String(Number(offer.usdAllowance) * requested.copies), 6),
      returnedUsdAllowance: '0',
      status: 'planned' as const,
    }
  })
  const round: ApiQuotaRound = { id: roundId, batchId: payload.batchId, name: payload.name.trim(), startsAt, endsAt, status: 'scheduled', allocations, version: 1 }
  apiQuotaRoundStore.push(round)
  for (const allocation of allocations) {
    const offer = apiQuotaOfferStore.find(item => item.id === allocation.offerId)
    if (offer && (!offer.nextRound || Date.parse(startsAt) < Date.parse(offer.nextRound.startsAt))) offer.nextRound = clone(round)
  }
  persistApiQuotaStores()
  return clone(round)
}

export async function updateApiQuotaBatchStatus(batchId: string, action: 'publish' | 'pause' | 'resume' | 'archive', version: number) {
  if (shouldUseRealBackend()) return backendAPIQuotaBatchAction(batchId, action, version)
  await wait()
  const batch = apiQuotaBatchStore.find(item => item.id === batchId)
  if (!batch) throw new Error('未找到额度批次。')
  if (batch.version !== version) throw new Error('额度批次已更新，请刷新后重试。')
  batch.status = action === 'publish' || action === 'resume' ? 'published' : action === 'pause' ? 'paused' : 'archived'
  if (action === 'publish') batch.publishedAt = nowText()
  batch.version += 1
  for (const offer of apiQuotaOfferStore.filter(item => item.batchId === batchId)) {
    if (action === 'publish' && offer.status === 'draft') {
      offer.status = 'published'
      offer.publishedAt = nowText()
      offer.isOrderable = offer.saleMode === 'continuous' && offer.availableCopies > 0 && (offer.deliveryMode !== 'preimported' || offer.credentialAvailableCopies >= offer.availableCopies)
      offer.orderabilityCode = offer.isOrderable ? 'orderable' : offer.deliveryMode === 'preimported' ? 'credential_unavailable' : 'not_started'
      offer.orderabilityReason = offer.isOrderable ? '当前可购买。' : offer.deliveryMode === 'preimported' ? '可用交付凭据不足。' : '下一轮尚未开始。'
    }
    offer.batchStatus = batch.status === 'paused' ? 'paused' : 'published'
    if (action === 'pause' || action === 'archive') {
      offer.isOrderable = false
      offer.orderabilityCode = 'batch_paused'
      offer.orderabilityReason = action === 'archive' ? '额度批次已归档。' : '商户已暂停额度批次。'
    }
  }
  persistApiQuotaStores()
  return clone(batch)
}

export async function getApiQuotaCredentialSummary(offerId: string) {
  if (shouldUseRealBackend()) return backendAPIQuotaCredentialSummary(offerId)
  await wait()
  return clone(apiQuotaCredentialSummaryStore.find(item => item.offerId === offerId) ?? { offerId, available: 0, reserved: 0, delivered: 0, retired: 0 })
}

export async function importApiQuotaCredentials(offerId: string, deliveryKind: ApiOrderDeliveryKind, file: File) {
  if (shouldUseRealBackend()) return backendImportAPIQuotaCredentials(offerId, deliveryKind, file)
  await wait()
  const offer = apiQuotaOfferStore.find(item => item.id === offerId)
  if (!offer || offer.deliveryMode !== 'preimported') throw new Error('只有预导入交付的额度规格可以导入凭据。')
  const lines = (await file.text()).split(/\r?\n/).map(line => line.trim()).filter(Boolean)
  const expectedHeader = deliveryKind === 'api_key_endpoint' ? 'api_base_url,api_key,instructions' : 'panel_login_url,username,password,instructions'
  if (lines[0]?.toLowerCase() !== expectedHeader) throw new Error(`CSV 表头必须为 ${expectedHeader}`)
  const imported = lines.length - 1
  if (imported < 1 || imported > 5000) throw new Error('CSV 每次需要包含 1-5000 条凭据。')
  let summary = apiQuotaCredentialSummaryStore.find(item => item.offerId === offerId)
  if (!summary) {
    summary = { offerId, available: 0, reserved: 0, delivered: 0, retired: 0 }
    apiQuotaCredentialSummaryStore.push(summary)
  }
  summary.available += imported
  offer.credentialAvailableCopies = summary.available
  if (offer.status === 'published' && offer.availableCopies > 0 && summary.available >= offer.availableCopies) {
    offer.isOrderable = true
    offer.orderabilityCode = 'orderable'
    offer.orderabilityReason = '当前可购买。'
  }
  persistApiQuotaStores()
  return clone({ imported, summary })
}

export async function getApiServiceById(id: string) {
  if (shouldUseRealBackend()) return backendAPIServiceById(id)
  await wait()
  return clone(apiServiceStore.find(item => item.id === id && isApiServicePubliclyOrderable(item)) ?? null)
}

const apiServiceSalesStatePriority: Record<ApiServiceSalesState, number> = {
  selling: 0,
  upcoming: 1,
  paused: 2,
  sold_out: 3,
  expired: 4,
  draft: 5,
  offline: 6,
  archived: 7,
}

function mockServiceSalesFallback(service: ApiService): ApiServiceSalesState {
  if (service.state === 'paused') return 'paused'
  if (service.state === 'reviewing') return 'draft'
  if (service.state === 'offline') return 'offline'
  if (service.publiclyOrderable) return 'selling'
  if (service.balance <= 0) return 'sold_out'
  return 'offline'
}

function mockFlexibleQuotaChannel(service: ApiService): ApiServiceSalesChannel | null {
  if (service.billingMode !== 'metered_credit') return null
  return {
    kind: 'flexible_quota',
    state: mockServiceSalesFallback(service),
    availableUsdAllowance: service.availableUsdAllowance ?? String(service.balance),
    expiresAt: service.quotaExpiresAt,
  }
}

function mockLimitedOfferSalesState(service: ApiService, batch: ApiQuotaBatch, offer?: PublicApiQuotaOffer): ApiServiceSalesState {
  if (batch.status === 'archived' || offer?.status === 'archived') return 'archived'
  if (service.state === 'paused' || batch.status === 'paused' || offer?.status === 'paused') return 'paused'
  if (service.state === 'reviewing' || batch.status === 'draft' || !offer || offer.status === 'draft') return 'draft'
  if (service.state === 'offline') return 'offline'
  if (offer.orderabilityCode === 'orderable') return 'selling'
  if (offer.orderabilityCode === 'not_started') return 'upcoming'
  if (offer.orderabilityCode === 'batch_expired') return 'expired'
  if (offer.orderabilityCode === 'batch_paused' || offer.orderabilityCode === 'offer_paused') return 'paused'
  if (offer.orderabilityCode === 'sold_out' || offer.orderabilityCode === 'round_ended' || offer.orderabilityCode === 'credential_unavailable') return 'sold_out'
  return mockServiceSalesFallback(service)
}

function mockLimitedQuotaChannel(service: ApiService): ApiServiceSalesChannel | null {
  const candidates = apiQuotaBatchStore
    .filter(batch => batch.apiServiceId === service.id)
    .flatMap(batch => {
      const offers = apiQuotaOfferStore.filter(offer => offer.batchId === batch.id)
      const rows = offers.length > 0 ? offers : [undefined]
      return rows.map((offer): ApiServiceSalesChannel => ({
        kind: 'limited_quota',
        state: mockLimitedOfferSalesState(service, batch, offer),
        availableCopies: offer?.orderabilityCode === 'not_started'
          ? offer.nextRound?.allocations.reduce((total, allocation) => total + allocation.availableCopies, 0) ?? 0
          : offer?.availableCopies ?? 0,
        nextStartsAt: offer?.nextRound?.startsAt,
        saleCutoffAt: batch.saleCutoffAt,
        expiresAt: batch.expiresAt,
      }))
    })
  return candidates.sort((left, right) => apiServiceSalesStatePriority[left.state] - apiServiceSalesStatePriority[right.state])[0] ?? null
}

export function buildMockApiServiceSalesSummary(service: ApiService): ApiServiceSalesSummary {
  const channels = [mockFlexibleQuotaChannel(service), mockLimitedQuotaChannel(service)]
    .filter((channel): channel is ApiServiceSalesChannel => channel !== null)
  const overallState = channels
    .map(channel => channel.state)
    .sort((left, right) => apiServiceSalesStatePriority[left] - apiServiceSalesStatePriority[right])[0]
    ?? mockServiceSalesFallback(service)
  return { overallState, channels }
}

export function matchesApiServiceSalesView(state: ApiServiceSalesState, salesView: ApiServiceSalesView) {
  if (salesView === 'active') return state === 'selling' || state === 'upcoming'
  if (salesView === 'expired') return state === 'expired'
  if (salesView === 'paused') return state === 'paused'
  if (salesView === 'draft') return state === 'draft' || state === 'offline'
  return true
}

function mockOwnerApiService(service: ApiService): OwnerApiService {
  return {
    ...service,
    healthSummary: service.healthSummary ?? mockUnconfiguredAPIHealthSummary(),
    salesSummary: buildMockApiServiceSalesSummary(service),
  }
}

function mockUnconfiguredAPIHealthSummary(): ApiServiceHealthSummary {
	const slot = (hour: number): ApiServiceHealthSample => ({
		slotStartedAt: new Date(Date.UTC(2026, 7, 4, hour)).toISOString(),
		state: 'no_sample',
	})
	const bucket = (hour: number): ApiServiceHealthHourlyBucket => ({
		hourStartedAt: new Date(Date.UTC(2026, 7, 4, hour)).toISOString(),
		state: 'no_sample',
		completedCycles: 0,
		firstAttemptSuccesses: 0,
		retryRecoveries: 0,
		finalFailures: 0,
		slowSuccesses: 0,
		finalSuccessPercent: null,
		averageTtftMs: null,
	})
	const hours = Array.from({ length: 24 }, (_, index) => index)
	return {
		state: 'no_sample',
		availabilityReason: 'unconfigured',
		transportSecurity: null,
		stabilityPercent: null,
		finalSuccessPercent: null,
		coveragePercent: '0.0',
		completedCycles: 0,
		theoreticalSlots: 288,
		firstAttemptSuccesses: 0,
		retryRecoveries: 0,
		finalFailures: 0,
		averageTtftMs: null,
		p50TtftMs: null,
		p95TtftMs: null,
		lastSampledAt: null,
		probeModel: null,
		probeProtocol: null,
		probeEnvironment: null,
		probeEnvironmentLabel: null,
		probeModelChangedAt: null,
		accumulatingSamples: false,
		hourlyBuckets: hours.map(bucket),
		cost: {
			knownBaseCostUsd: '0',
			knownRetryCostUsd: '0',
			projectedDailyCostUsd: '',
			hasUnknownUsage: false,
			knownUsageSamples: 0,
		},
		successRatePercent: null,
		successfulSamples: 0,
		totalSamples: 0,
		samples: hours.map(slot),
	}
}

export async function getMyApiServices(salesView: ApiServiceSalesView = 'active') {
  if (shouldUseRealBackend()) return backendOwnerAPIServices(salesView)
  await wait()
  return clone(
    apiServiceStore
      .filter(item => item.merchantUsername === myUserProfileStore.username)
      .map(item => mockOwnerApiService(item))
      .filter(item => matchesApiServiceSalesView(item.salesSummary.overallState, salesView)),
  )
}

export async function getMyApiServicesPage(salesView: ApiServiceSalesView = 'active', page: CursorPageRequest = {}): Promise<CursorPage<OwnerApiService>> {
  if (shouldUseRealBackend()) return backendOwnerAPIServicesPage(salesView, page)
  return paginateCursorItems(await getMyApiServices(salesView), page)
}

export async function getMyApiServiceById(id: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIServiceById(id)
  await wait()
  return clone(apiServiceStore.find(item => item.id === id && item.merchantUsername === myUserProfileStore.username) ?? null)
}

export async function getMyProfile() {
  if (shouldUseRealBackend()) return backendMyProfile()
  await wait()
  return clone(myUserProfileStore)
}

export async function updateMyProfile(payload: UpdateMyProfileRequest) {
  if (shouldUseRealBackend()) return backendUpdateMyProfile(payload)
  await wait()
  if (!payload.displayName.trim()) throw new Error('显示名称不能为空')
  if (payload.displayName.length > 32) throw new Error('显示名称最多 32 字')
  if (!/^[a-zA-Z0-9_-]{3,24}$/.test(payload.username)) throw new Error('站内用户名只允许 3-24 位字母、数字、下划线和短横线')
  myUserProfileStore = {
    ...myUserProfileStore,
    displayName: payload.displayName.trim(),
    username: payload.username.trim(),
    bio: payload.bio?.trim() || null,
    regionCode: payload.regionCode,
    timezone: payload.timezone,
    avatarMode: payload.avatarMode,
    customAvatarUrl: payload.avatarMode === 'custom_url' ? (payload.avatarUrl?.trim() || null) : null,
    avatarUrl: payload.avatarMode === 'custom_url' ? (payload.avatarUrl?.trim() || null) : myUserProfileStore.linuxDoBinding.linuxDoAvatarUrl,
    privacy: payload.privacy ? clone(payload.privacy) : myUserProfileStore.privacy,
  }
  syncPublicCurrentUser()
  return clone(myUserProfileStore)
}

export async function setBackupPassword(payload: SetBackupPasswordRequest) {
  if (shouldUseRealBackend()) return backendSetPassword(payload)
  await wait()
  const validationMessage = getBackupPasswordValidationMessage(payload.newPassword)
  if (validationMessage) throw new Error(validationMessage)
  if (myUserProfileStore.passwordConfigured && !payload.currentPassword?.trim()) throw new Error('修改密码必须输入当前密码')
  myUserProfileStore = {
    ...myUserProfileStore,
    passwordConfigured: true,
  }
}

export async function startEmailVerification(email: string): Promise<EmailVerificationChallenge> {
  if (shouldUseRealBackend()) return backendStartEmailVerification(email)
  await wait()
  const normalized = email.trim().toLowerCase()
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(normalized)) throw new Error('邮箱格式不正确')
  return {
    email: normalized,
    expiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
    devCode: '123456',
  }
}

export async function confirmEmailVerification(payload: { email: string, code: string }) {
  if (shouldUseRealBackend()) return backendConfirmEmailVerification(payload)
  await wait()
  const normalized = payload.email.trim().toLowerCase()
  if (payload.code.trim() !== '123456') throw new Error('验证码无效或已过期')
  myUserProfileStore = {
    ...myUserProfileStore,
    email: normalized,
    emailVerified: true,
    emailVerifiedAt: nowText(),
  }
  syncPublicCurrentUser()
  return clone(myUserProfileStore)
}

export async function uploadMyAvatarMock(file: File) {
  await wait()
  void file
  throw new Error('当前版本不支持本地头像上传，请填写 HTTPS 图片 URL。')
}

export async function deleteCustomAvatar() {
  await wait()
  myUserProfileStore = {
    ...myUserProfileStore,
    avatarMode: 'linuxdo',
    avatarUrl: myUserProfileStore.linuxDoBinding.linuxDoAvatarUrl,
    customAvatarUrl: null,
  }
  syncPublicCurrentUser()
  return clone(myUserProfileStore)
}

export async function useLinuxDoAvatar() {
  await wait()
  myUserProfileStore = {
    ...myUserProfileStore,
    avatarMode: 'linuxdo',
    avatarUrl: myUserProfileStore.linuxDoBinding.linuxDoAvatarUrl,
    customAvatarUrl: null,
  }
  syncPublicCurrentUser()
  return clone(myUserProfileStore)
}

export async function getMyContactMethods() {
  if (shouldUseRealBackend()) return backendMyContactMethods()
  await wait()
  return clone(myContactMethodStore)
}

export async function createContactMethod(payload: SaveContactMethodRequest) {
  if (shouldUseRealBackend()) return backendCreateContact(payload)
  await wait()
  if (payload.type === 'linuxdo') throw new Error('linux.do 联系方式来自绑定账号，不能手动伪造')
  if (!payload.displayValue.trim()) throw new Error('联系方式内容不能为空')
  const createdAt = nowText()
  const contact: UserContactMethod = {
    id: `contact-${Date.now()}`,
    userId: myUserProfileStore.id,
    type: payload.type,
    label: payload.label.trim() || defaultContactLabel(payload.type),
    maskedValue: contactMaskedValue(payload.type, payload.displayValue),
    displayValue: payload.displayValue.trim(),
    usageScopes: [...payload.usageScopes],
    isDefault: payload.isDefault,
    enabled: payload.enabled,
    verified: false,
    createdAt,
    updatedAt: createdAt,
  }
  if (contact.isDefault) {
    myContactMethodStore = myContactMethodStore.map(item => item.usageScopes.some(scope => contact.usageScopes.includes(scope)) ? { ...item, isDefault: false } : item)
  }
  myContactMethodStore = [contact, ...myContactMethodStore]
  return clone(contact)
}

export async function updateContactMethod(contactId: string, payload: SaveContactMethodRequest) {
  if (shouldUseRealBackend()) return backendUpdateContact(contactId, payload)
  await wait()
  const current = myContactMethodStore.find(item => item.id === contactId)
  if (!current) throw new Error('未找到联系方式')
  if (current.type === 'linuxdo' && payload.displayValue !== current.displayValue) throw new Error('linux.do 联系方式不能手动修改')
  const updated: UserContactMethod = {
    ...current,
    type: current.type === 'linuxdo' ? 'linuxdo' : payload.type,
    label: payload.label.trim() || defaultContactLabel(payload.type),
    maskedValue: contactMaskedValue(current.type === 'linuxdo' ? 'linuxdo' : payload.type, payload.displayValue),
    displayValue: payload.displayValue.trim(),
    usageScopes: [...payload.usageScopes],
    isDefault: payload.isDefault,
    enabled: payload.enabled,
    verified: current.type === payload.type ? current.verified : false,
    updatedAt: nowText(),
  }
  myContactMethodStore = myContactMethodStore.map(item => item.id === contactId ? updated : item)
  if (updated.isDefault) {
    myContactMethodStore = myContactMethodStore.map(item => item.id !== updated.id && item.usageScopes.some(scope => updated.usageScopes.includes(scope)) ? { ...item, isDefault: false } : item)
  }
  return clone(updated)
}

export async function deleteContactMethod(contactId: string) {
  if (shouldUseRealBackend()) return backendDeleteContact(contactId)
  await wait()
  const current = myContactMethodStore.find(item => item.id === contactId)
  if (!current) throw new Error('未找到联系方式')
  if (current.type === 'linuxdo') throw new Error('linux.do 绑定联系方式不能删除')
  myContactMethodStore = myContactMethodStore.filter(item => item.id !== contactId)
  return clone(current)
}

export async function setDefaultContactMethod(contactId: string) {
  if (shouldUseRealBackend()) return backendSetDefaultContact(contactId)
  await wait()
  const current = myContactMethodStore.find(item => item.id === contactId)
  if (!current) throw new Error('未找到联系方式')
  myContactMethodStore = myContactMethodStore.map(item => ({
    ...item,
    isDefault: item.id === contactId || (item.isDefault && !item.usageScopes.some(scope => current.usageScopes.includes(scope))),
    updatedAt: item.id === contactId ? nowText() : item.updatedAt,
  }))
  return clone(myContactMethodStore.find(item => item.id === contactId)!)
}

export async function sendContactVerification(contactId: string) {
  await wait()
  const current = myContactMethodStore.find(item => item.id === contactId)
  if (!current) throw new Error('未找到联系方式')
  if (current.type !== 'email') throw new Error('当前仅邮箱支持验证码验证')
  return clone({ contactId, sentAt: nowText() })
}

export async function verifyContactMethod(contactId: string) {
  if (shouldUseRealBackend()) return backendVerifyContact(contactId)
  await wait()
  const current = myContactMethodStore.find(item => item.id === contactId)
  if (!current) throw new Error('未找到联系方式')
  myContactMethodStore = myContactMethodStore.map(item => item.id === contactId ? { ...item, verified: true, updatedAt: nowText() } : item)
  return clone(myContactMethodStore.find(item => item.id === contactId)!)
}

export async function getApiPaymentAccountSettings() {
  if (shouldUseRealBackend()) return backendGetApiPaymentAccountSettings()
  await wait()
  return cloneApiPaymentAccountSettings(apiPaymentAccountSettingsStore)
}

export async function updateApiPaymentAccountSettings(payload: Omit<ApiPaymentAccountSettings, 'updatedAt'>) {
  if (shouldUseRealBackend()) return backendUpdateApiPaymentAccountSettings(payload)
  await wait()
  apiPaymentAccountSettingsStore = normalizeApiPaymentAccountSettings({
    paymentWindowMinutes: payload.paymentWindowMinutes,
    paymentOptions: payload.paymentOptions,
    updatedAt: nowText(),
  })
  persistApiPaymentAccountSettings()
  return cloneApiPaymentAccountSettings(apiPaymentAccountSettingsStore)
}

export async function getPublicMerchantProfile(username: string) {
  if (shouldUseRealBackend()) return backendPublicMerchantProfile(username)
  await wait()
  const profile = publicMerchantProfiles.find(item => item.username === username)
  if (!profile) return null
  return clone({
    profile,
    services: apiServiceStore.filter(item => item.merchantUsername === username && isApiServicePubliclyOrderable(item) && canOpenApiMerchantProfile(item)),
    completions: publicCompletionRecords.filter(item => item.username === username),
    reviews: publicReviewsForProfile(username),
    disputes: publicDisputeRecords.filter(item => item.username === username),
  })
}

export async function getPublicUserProfile(username: string) {
  if (shouldUseRealBackend()) return backendPublicUserProfile(username)
  await wait()
  syncPublicCurrentUser()
  const profile = publicUserProfiles.find(item => item.username === username)
  if (!profile) return null
  const reputation = await mockPublicUserReputation(username, 'overall')
  return clone({
    profile: sanitizePublicUserProfile(profile),
    reputations: reputation.reputations,
    carpools: carpoolStore.filter(item => item.owner === username && item.status === '可上车'),
    services: apiServiceStore.filter(item => item.merchantUsername === username && isApiServicePubliclyOrderable(item) && canOpenApiMerchantProfile(item)),
    completions: publicCompletionRecords.filter(item => item.username === username),
    reviews: publicReviewsForProfile(username),
    disputes: publicDisputeRecords.filter(item => item.username === username),
  })
}

export async function getMyMerchantProfile(): Promise<BackendMerchantProfile | null> {
  return shouldUseRealBackend() ? backendMyMerchantProfile() : null
}

export async function upsertMyMerchantProfile(payload: { slug: string, displayName: string, avatarUrl?: string }): Promise<BackendMerchantProfile> {
  return backendUpsertMerchantProfile(payload)
}

export async function getCarpoolApplicationContacts(applicationId: string): Promise<OrderContactSnapshot> {
  if (shouldUseRealBackend()) return backendCarpoolApplicationContacts(applicationId)
  await wait()
  const application = carpoolApplicationStore.find(item => item.id === applicationId)
  const snapshot = orderContactSnapshots.find(item => item.orderType === 'carpool_application' && item.orderId === applicationId)
  if (!application) throw new Error(`Carpool application not found: ${applicationId}`)
  const canView = carpoolContactVisibleStatuses.includes(application.status)
  if (!canView) {
    return clone({
      id: `contact-snapshot-blocked-${applicationId}`,
      orderType: 'carpool_application',
      orderId: applicationId,
      sellerContacts: [],
      buyerContacts: [],
      contactWindowEndsAt: application.reservedUntil,
      canView: false,
      unavailableReason: '车主接受申请并预留席位后才展示联系窗口联系方式。',
      createdAt: application.createdAt,
    })
  }
  if (snapshot) return clone(contactSnapshotForVisibility(snapshot, canView, null, application.reservedUntil))
  return clone({
    id: `contact-snapshot-${applicationId}`,
    orderType: 'carpool_application',
    orderId: applicationId,
    sellerContacts: [
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: `@${application.ownerUsername}`, displayValue: `@${application.ownerUsername}`, verified: true, usageScope: 'carpool_owner', actionUrl: linuxDoProfileSummaryUrl(application.ownerUsername) },
    ],
    buyerContacts: [],
    contactWindowEndsAt: application.reservedUntil,
    canView: true,
    unavailableReason: null,
    createdAt: application.updatedAt,
  })
}

export async function createContactReport(payload: CreateContactReportRequest) {
  if (shouldUseRealBackend()) return backendCreateReport(payload)
  await wait()
  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '系统',
    action: '联系方式举报',
    targetType: payload.orderType,
    targetId: payload.orderId,
    targetLabel: payload.contactType,
    beforeStatus: null,
    afterStatus: payload.reasonCode,
    reason: payload.note || '用户提交联系方式问题',
  })
  return clone({ id: `contact-report-${Date.now()}`, createdAt: nowText(), ...payload })
}

export async function createManualInterventionReport(payload: CreateManualInterventionReportRequest) {
  if (shouldUseRealBackend()) return backendCreateManualInterventionReport(payload)
  await wait()
  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '系统',
    action: '举报 / 申请人工介入',
    targetType: payload.targetType,
    targetId: payload.targetId,
    targetLabel: payload.targetLabel ?? payload.targetId,
    beforeStatus: null,
    afterStatus: payload.reasonCode,
    reason: payload.description,
  })
  return clone({ id: `manual-intervention-${Date.now()}`, createdAt: nowText(), ...payload })
}

export async function createPublicUserReport(payload: CreatePublicProfileReportRequest) {
  if (shouldUseRealBackend()) return backendCreatePublicUserReport(payload)
  await wait()
  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '系统',
    action: '公开主页举报',
    targetType: 'public_user',
    targetId: payload.username,
    targetLabel: `@${payload.username}`,
    beforeStatus: null,
    afterStatus: payload.reasonCode,
    reason: payload.description || payload.title,
  })
  return clone({ id: `public-user-report-${Date.now()}`, createdAt: nowText(), ...payload })
}

export async function getApiPurchaseIntents(filters: ApiPurchaseIntentFilters = {}) {
  if (shouldUseRealBackend()) return backendMyAPIIntents(filters)
  await wait()
  return clone(filterApiPurchaseIntents(filters))
}

export async function getMyApiPurchaseIntents(filters: ApiPurchaseIntentFilters = {}) {
  if (shouldUseRealBackend()) return backendMyAPIIntents(filters)
  await wait()
  return clone(filterApiPurchaseIntents({ ...filters, buyerId: currentBuyerId, sort: filters.sort ?? 'default_buyer' }))
}

export async function getMerchantApiPurchaseIntents(filters: ApiPurchaseIntentFilters = {}) {
  if (shouldUseRealBackend()) return backendOwnerAPIIntents(filters)
  await wait()
  return clone(filterApiPurchaseIntents({ ...filters, merchantId: currentMerchantId, sort: filters.sort ?? 'default_merchant' }))
}

export async function getApiPurchaseIntentById(id: string) {
  if (shouldUseRealBackend()) return backendAPIIntentById(id)
  await wait()
  return clone(apiPurchaseIntentStore.find(item => item.id === id) ?? null)
}

export async function getApiPurchaseIntentEvents(id: string) {
  if (shouldUseRealBackend()) return backendAPIIntentEvents(id)
  await wait()
  const intent = apiPurchaseIntentStore.find(item => item.id === id)
  const merchantDisplayName = intent ? getApiMerchantDisplayName(intent) : null
  return clone(apiPurchaseIntentEventStore
    .filter(item => item.intentId === id)
    .map(item => item.actorRole === 'merchant' && merchantDisplayName ? { ...item, actorLabel: merchantDisplayName } : item)
    .sort((a, b) => compareTimeDesc(a.createdAt, b.createdAt)))
}

export async function getAdminOverview() {
  await wait()
  return clone(adminCards)
}

function projectMockAdminApiOrder(order: ApiOrder): AdminApiOrderDetail {
  return {
    id: order.id,
    purchaseKind: order.purchaseKind,
    apiPurchaseIntentId: order.apiPurchaseIntentId,
    apiServiceId: order.apiServiceId,
    buyerUserId: order.buyerId,
    sellerUserId: order.sellerId,
    status: order.status,
    disputeStatus: order.disputeStatus,
    disputeCaseId: order.disputeCaseId,
    serviceTitleSnapshot: order.serviceTitle,
    selectedPackageId: order.selectedPackageId,
    selectedPackageSnapshot: order.packageSnapshot ? JSON.stringify(order.packageSnapshot) : undefined,
    amount: order.amountDecimal ?? String(order.amount),
    currency: order.currency,
    requestedUsdAllowanceSnapshot: order.requestedUsdAllowanceDecimal ?? String(order.requestedUsdAllowance),
    selectedPaymentMethod: order.selectedPaymentMethod,
    paymentExpiresAt: order.paymentExpiresAt,
    paymentSubmittedAt: order.paymentSubmittedAt,
    paymentIssueReason: order.paymentIssueReason,
    paymentIssueNote: order.paymentIssueNote,
    paymentIssueReportedAt: order.paymentIssueReportedAt,
    paidConfirmedAt: order.paidConfirmedAt,
    deliveryNote: order.deliveryNote,
    deliverySubmittedAt: order.deliverySubmittedAt,
    deliveryReviewExpiresAt: order.deliveryReviewExpiresAt,
    completionSource: order.completionSource,
    completedAt: order.completedAt,
    cancelledAt: order.cancelledAt,
    cancelReason: order.cancelReason,
    version: order.version,
    createdAt: order.createdAt,
    updatedAt: order.updatedAt,
  }
}

export async function getAdminApiOrderById(id: string): Promise<AdminApiOrderDetail> {
  if (shouldUseRealBackend()) return backendAdminAPIOrder(id)
  await wait()
  materializeMockApiOrderReviews()
  return clone(projectMockAdminApiOrder(findApiOrder(id)))
}

function withAdminRowLinks(rows: AdminRow[]) {
  return rows.map(row => ({ ...row, targetTo: row.targetTo ?? adminTargetLink(row) }))
}

function getOfficialPriceReviewDetails(item: OfficialPrice): AdminRow['detailItems'] {
  const sameProductRows = officialPriceStore.filter(row => row.product === item.product && row.id !== item.id)
  const historySummary = sameProductRows.length
    ? sameProductRows.slice(0, 2).map(row => `${row.plan} ${row.region} ${row.cny ? `¥${row.cny}` : row.originalPrice}（${row.status}）`).join('；')
    : '暂无同产品历史样本'
  const duplicateCount = sameProductRows.filter(row => row.region === item.region || row.originalPrice === item.originalPrice).length
  const submitterRows = officialPriceStore.filter(row => row.submitter === item.submitter)
  const submitterVerifiedCount = submitterRows.filter(row => row.status === '已验证').length
  const evidenceState = item.source.includes('linux.do')
    ? '原帖链接与截图摘要待管理员核对'
    : item.source.includes('官方')
      ? '官网公开页截图摘要待管理员核对'
      : '用户提交截图摘要待管理员核对'
  const regionRestriction = item.region.includes('土耳其') || item.region.includes('菲律宾') || item.region.includes('香港')
    ? `${item.region} 可能需要地区支付方式或当地计费资格`
    : '未标记特殊地区限制'

  return [
    { label: '证据预览', value: evidenceState },
    { label: '来源', value: item.source },
    { label: '历史价格', value: historySummary },
    { label: '汇率时间', value: `${item.updatedAt} · 以公开汇率线索折算，需复核截图时间` },
    { label: '重复线索', value: duplicateCount ? `发现 ${duplicateCount} 条同地区或同原币价格线索` : '未发现明显重复线索' },
    { label: '地区限制', value: regionRestriction },
    { label: '提交者历史', value: `${item.submitter} 共提交 ${submitterRows.length} 条，已验证 ${submitterVerifiedCount} 条，信任等级${item.submitterTrust}` },
    { label: '开通方式', value: item.openingMethod },
    { label: '折合人民币', value: item.cny ? `¥${item.cny}` : '待验证' },
    { label: '操作记录', value: adminAuditLogStore.filter(log => log.targetId === item.id).map(log => `${log.action}：${getReadableStatus(log.beforeStatus)} → ${getReadableStatus(log.afterStatus)}`).join('；') || '暂无管理操作记录' },
  ]
}

function adminDirectoryRow(item: typeof adminDirectoryUsers[number]): AdminRow {
  return {
    id: item.id,
    primary: item.username,
    secondary: `${item.displayName} · ${item.linuxdoBound ? `已绑定 linux.do · ${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}` : '未绑定 linux.do'}`,
    owner: item.isAdmin ? '管理员账号' : '普通账号',
    status: item.accountStatus,
    risk: `注册 ${item.createdAt} · 最近活跃 ${item.lastActiveAt}`,
    targetType: 'user',
    detailItems: [
      { label: '显示名称', value: item.displayName },
      { label: '账号状态', value: item.accountStatus },
      { label: '账号角色', value: item.isAdmin ? '管理员' : '普通用户' },
      { label: 'linux.do 绑定', value: item.linuxdoBound ? `已绑定，${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}` : '未绑定' },
      { label: '注册时间', value: item.createdAt },
      { label: '最近活跃', value: item.lastActiveAt },
    ],
    targetTo: `/u/${item.username}`,
  }
}

const adminUserStatusByLabel: Record<typeof adminDirectoryUsers[number]['accountStatus'], AdminUserStatus> = {
  '正常': 'active',
  '已暂停': 'suspended',
  '已封禁': 'banned',
  '已归档': 'archived',
}

const adminUserMockStore: AdminUser[] = adminDirectoryUsers.map((item, index) => ({
  id: item.id,
  username: item.username,
  displayName: item.displayName,
  accountStatus: adminUserStatusByLabel[item.accountStatus],
  isAdmin: item.isAdmin,
  linuxDoBound: item.linuxdoBound,
  trustLevel: item.trustLevel ?? undefined,
  createdAt: item.createdAt,
  lastActiveAt: item.lastActiveAt,
  version: index + 1,
}))
const adminUserMockAuditEntries = new Map<string, AdminUserDetail['recentAuditEntries']>()

function mockAdminUserSummary() {
  return {
    totalUsers: adminUserMockStore.length,
    adminUsers: adminUserMockStore.filter(item => item.isAdmin).length,
    linuxDoBoundUsers: adminUserMockStore.filter(item => item.linuxDoBound).length,
    activeUsers: adminUserMockStore.filter(item => item.accountStatus === 'active').length,
    suspendedUsers: adminUserMockStore.filter(item => item.accountStatus === 'suspended').length,
    bannedUsers: adminUserMockStore.filter(item => item.accountStatus === 'banned').length,
    archivedUsers: adminUserMockStore.filter(item => item.accountStatus === 'archived').length,
  }
}

function mockAdminUserActions(item: AdminUser): AdminUserDetail['availableActions'] {
  const transitions: Record<AdminUserStatus, AdminUserStatus[]> = {
    active: ['suspended', 'banned', 'archived'],
    suspended: ['active', 'banned', 'archived'],
    banned: ['active', 'archived'],
    archived: ['active'],
  }
  const activeAdminCount = adminUserMockStore.filter(user => user.isAdmin && user.accountStatus === 'active').length
  const statusActions: AdminUserDetail['availableActions'] = transitions[item.accountStatus].map(targetStatus => {
    const action = targetStatus === 'active' ? 'restore' : targetStatus === 'suspended' ? 'suspend' : targetStatus === 'banned' ? 'ban' : 'archive'
    const lastActiveAdmin = item.isAdmin && item.accountStatus === 'active' && activeAdminCount <= 1
    return {
      action,
      kind: 'status',
      targetStatus,
      allowed: !lastActiveAdmin,
      severity: targetStatus === 'active' ? 'normal' : targetStatus === 'suspended' ? 'warning' : 'danger',
      requiresReason: true,
      requiresConfirmation: true,
      blockedCode: lastActiveAdmin ? 'LAST_ACTIVE_ADMIN' : undefined,
      blockedReason: lastActiveAdmin ? '不能停用最后一个有效管理员或撤销其权限。' : undefined,
    }
  })
  const grantBlocked = !item.isAdmin && item.accountStatus !== 'active'
  const revokeBlocked = item.isAdmin && item.accountStatus === 'active' && activeAdminCount <= 1
  return [...statusActions, {
    action: item.isAdmin ? 'revoke_admin' : 'grant_admin',
    kind: 'permission',
    targetIsAdmin: !item.isAdmin,
    allowed: !grantBlocked && !revokeBlocked,
    severity: item.isAdmin ? 'danger' : 'normal',
    requiresReason: true,
    requiresConfirmation: true,
    blockedCode: revokeBlocked ? 'LAST_ACTIVE_ADMIN' : grantBlocked ? 'ACCOUNT_NOT_ACTIVE' : undefined,
    blockedReason: revokeBlocked
      ? '不能停用最后一个有效管理员或撤销其权限。'
      : grantBlocked ? '只能向正常状态的账号授予管理员权限。' : undefined,
  }]
}

function mockAdminUserDetail(item: AdminUser): AdminUserDetail {
	const isActive = item.accountStatus === 'active'
  return {
    user: clone(item),
    updatedAt: item.createdAt,
    linuxDoBinding: {
      bound: item.linuxDoBound,
      username: item.linuxDoBound ? item.username : undefined,
      trustLevel: item.trustLevel,
    },
    emailVerified: item.linuxDoBound,
    backupPasswordConfigured: true,
    providers: item.linuxDoBound ? [{ provider: 'linux.do', createdAt: item.createdAt }] : [],
    sessions: {
      activeCount: isActive ? 1 : 0,
      latestActivityAt: item.lastActiveAt ?? undefined,
    },
    recentAuditEntries: clone(adminUserMockAuditEntries.get(item.id) ?? []),
		availableActions: mockAdminUserActions(item),
		impactPreview: {
			activeSessions: isActive ? 1 : 0,
			activeCarpoolListings: 0,
			onlineApiServices: 0,
			openCarpoolApplications: 0,
			openApiOrders: 0,
			openDisputes: 0,
		},
		accountCapabilities: {
			canLogin: isActive,
			publiclyVisible: isActive,
			canPublish: isActive,
			canCreateOrders: isActive,
			canRevealContact: isActive,
			canAccessHistoricalTransactions: true,
		},
  }
}

export async function getAdminUserDirectory(query: AdminUserDirectoryQuery): Promise<AdminUserList> {
  if (shouldUseRealBackend()) return backendAdminUserDirectory(query)
  await wait()
  const keyword = query.search.trim().toLowerCase()
  const filtered = adminUserMockStore.filter(item => {
    const matchesSearch = !keyword || `${item.username} ${item.displayName}`.toLowerCase().includes(keyword)
    const matchesStatus = query.status === 'all' || item.accountStatus === query.status
    const matchesRole = query.role === 'all' || (query.role === 'admin' ? item.isAdmin : !item.isAdmin)
    const matchesLinuxDo = query.linuxDo === 'all' || (query.linuxDo === 'bound' ? item.linuxDoBound : !item.linuxDoBound)
    return matchesSearch && matchesStatus && matchesRole && matchesLinuxDo
  })
  const items = [...filtered].sort((left, right) => {
    if (query.sort === 'created_asc') return left.createdAt.localeCompare(right.createdAt) || left.id.localeCompare(right.id)
    if (query.sort === 'active_desc') return (right.lastActiveAt ?? '').localeCompare(left.lastActiveAt ?? '') || right.id.localeCompare(left.id)
    if (query.sort === 'username_asc') return left.username.localeCompare(right.username) || left.id.localeCompare(right.id)
    if (query.sort === 'username_desc') return right.username.localeCompare(left.username) || right.id.localeCompare(left.id)
    return right.createdAt.localeCompare(left.createdAt) || right.id.localeCompare(left.id)
  })
  const totalItems = items.length
  const start = (query.page - 1) * query.limit
  return {
    items: clone(items.slice(start, start + query.limit)),
    pagination: {
      page: query.page,
      limit: query.limit,
      totalItems,
      totalPages: Math.ceil(totalItems / query.limit),
    },
    summary: mockAdminUserSummary(),
  }
}

export async function getAdminUserDetail(userId: string): Promise<AdminUserDetail> {
  if (shouldUseRealBackend()) return backendAdminUserDetail(userId)
  await wait()
  const item = adminUserMockStore.find(user => user.id === userId)
  if (!item) throw new Error('用户不存在。')
  return mockAdminUserDetail(item)
}

export async function updateAdminUserStatus(input: {
  userId: string
  version: number
  status: AdminUserStatus
  reason: string
}): Promise<AdminUserDetail> {
  if (shouldUseRealBackend()) return backendUpdateAdminUserStatus(input)
  await wait()
  const item = adminUserMockStore.find(user => user.id === input.userId)
  if (!item) throw new Error('用户不存在。')
  if (item.version !== input.version) throw new Error('账号信息已更新，请刷新后重试。')
  const beforeStatus = item.accountStatus
  item.accountStatus = input.status
  item.version += 1
  adminUserMockAuditEntries.set(item.id, [{
    id: `mock-status-${item.version}`,
    adminUserId: 'mock-admin',
    adminUsername: '管理员',
    action: 'user.account_status_changed',
    reason: input.reason.trim(),
    beforeStatus,
    afterStatus: input.status,
    requestId: `mock-request-${item.version}`,
    createdAt: new Date().toISOString(),
  }, ...(adminUserMockAuditEntries.get(item.id) ?? [])])
  return mockAdminUserDetail(item)
}

export async function updateAdminUserPermission(input: {
  userId: string
  version: number
  isAdmin: boolean
  reason: string
}): Promise<AdminUserDetail> {
  if (shouldUseRealBackend()) return backendUpdateAdminUserPermission(input)
  await wait()
  const item = adminUserMockStore.find(user => user.id === input.userId)
  if (!item) throw new Error('用户不存在。')
  if (item.version !== input.version) throw new Error('账号信息已更新，请刷新后重试。')
  const beforeIsAdmin = item.isAdmin
  item.isAdmin = input.isAdmin
  item.version += 1
  adminUserMockAuditEntries.set(item.id, [{
    id: `mock-permission-${item.version}`,
    adminUserId: 'mock-admin',
    adminUsername: '管理员',
    action: 'user.admin_permission_changed',
    reason: input.reason.trim(),
    beforeIsAdmin,
    afterIsAdmin: input.isAdmin,
    requestId: `mock-request-${item.version}`,
    createdAt: new Date().toISOString(),
  }, ...(adminUserMockAuditEntries.get(item.id) ?? [])])
  return mockAdminUserDetail(item)
}

export async function getAdminSectionRows(section: AdminSection): Promise<AdminRow[]> {
  await wait()

  if (shouldUseRealBackend() && section === 'api-services') {
    return backendAdminAPIServiceRows()
  }

  if (shouldUseRealBackend() && section === 'carpools') {
    return backendAdminCarpoolRows()
  }

  if (shouldUseRealBackend() && (section === 'official-prices' || section === 'price-leads')) {
    return backendAdminOfficialPriceRows()
  }

  if (shouldUseRealBackend() && section === 'reports') {
    return backendAdminReportRows()
  }

  if (shouldUseRealBackend() && section === 'appeals') {
    return backendAdminAppealRows()
  }

  if (shouldUseRealBackend() && section === 'feedback') {
    return backendAdminFeedbackRows()
  }

  if (shouldUseRealBackend() && section === 'trade-intents') {
    return backendAdminAPIOrderRows()
  }

  function apiServiceAdminTargetLink(item: ApiService) {
    return getApiServicePublicDetailUrl(item)
  }

  if (section === 'official-prices' || section === 'price-leads') {
    return withAdminRowLinks(officialPriceStore.map(item => ({
      id: item.id,
      primary: `${item.product} ${item.plan}`,
      secondary: `${item.region} · ${item.channel} · ${item.originalPrice}`,
      owner: `${item.submitter} · 信任等级${item.submitterTrust}`,
      status: item.status,
      risk: item.isLowest ? '当前在售参考' : '普通线索',
      targetType: 'official-price',
      detailItems: getOfficialPriceReviewDetails(item),
    })))
  }

  if (section === 'carpools') {
    return withAdminRowLinks(carpoolStore.map(item => ({
      id: item.id,
      primary: item.product,
      secondary: `${item.region} · ${getPricingDisplay(item).primaryLabel} ¥${getPricingDisplay(item).primaryPrice}/月 · 可申请 ${getCarpoolSeatSummary(item).availableSeats}/${item.maxMembers} 席`,
      owner: `${item.owner} · ${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}`,
      status: item.status,
      risk: item.hasInfoConflict
        ? '信息冲突'
        : item.hasUnresolvedDispute === true
          ? '存在未解决纠纷'
          : item.hasUnresolvedDispute === null
            ? '风险数据暂无'
            : '未发现公开风险',
      targetType: 'carpool',
      detailItems: [
        { label: '车主类型', value: item.ownerType },
        { label: '开通方式', value: item.openingMethod },
        { label: '商户承诺', value: item.warranty },
        { label: '最近确认', value: item.confirmedAt },
      ],
    })))
  }

  if (section === 'api-services') {
    return withAdminRowLinks(apiServiceStore.map(item => ({
      id: item.id,
      primary: item.title,
      secondary: `${item.models.join(' / ')} · ${item.delivery} · 接入细节站外确认`,
      owner: canOpenApiMerchantProfile(item)
        ? `${getApiMerchantDisplayName(item)} · ${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}`
        : `${getApiMerchantDisplayName(item)} → ${item.merchantUsername} · ${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}`,
      status: item.state === 'reviewing'
        ? '待处理'
        : item.state === 'paused'
          ? item.warning === '已下架' ? '已下架' : '暂停'
          : item.online ? '在线' : '已通过',
      risk: item.unresolvedDisputes === null ? '纠纷数据暂无' : item.unresolvedDisputes > 0 ? `${item.unresolvedDisputes} 个未解决纠纷` : item.warranty,
      targetType: 'api-service',
      targetTo: apiServiceAdminTargetLink(item),
      detailItems: [
        { label: '商户身份', value: item.merchantIdentityMode === 'store_alias' ? `店铺名展示，真实用户 ${item.merchantUsername}` : '公开主页展示' },
        { label: '最低订单金额', value: `¥${item.minimumPurchaseCny}` },
        { label: '用量核对', value: getApiUsageVisibilityLabel(item.usageVisibility) },
        { label: '有效期', value: item.expiresAt },
      ],
    })))
  }

  if (section === 'trade-intents') {
    materializeMockApiOrderReviews()
    return withAdminRowLinks(apiOrderStore.map(item => ({
      id: item.id,
      primary: `${item.serviceTitle} API 订单`,
      secondary: `${item.orderNo} · 订单金额 ¥${item.amountDecimal ?? item.amount}`,
      owner: `${item.seller} / 买家 ${item.buyer}`,
      status: item.status === 'completed' ? getApiOrderCompletionSourceLabel(item.completionSource) : getApiOrderStatusLabel(item.status, 'admin'),
      risk: item.disputeStatus || item.cancelReason || `更新于 ${item.updatedAt}`,
      targetType: 'api-order',
      targetTo: `/admin/api-orders/${item.id}`,
      detailItems: [
        { label: '订单号', value: item.orderNo },
        { label: '订单金额', value: `¥${item.amountDecimal ?? item.amount}` },
        { label: '购买额度', value: `${item.requestedUsdAllowanceDecimal ?? item.requestedUsdAllowance} 美元额度` },
        { label: '交付凭证', value: item.deliverySubmittedAt ? '已提交（管理摘要不展示原始凭证）' : '尚未提交' },
        { label: '最近更新', value: item.updatedAt },
      ],
    })))
  }

  if (section === 'feedback') {
    return getAdminFeedbackRows()
  }

  if (section === 'users') {
    return withAdminRowLinks(adminDirectoryUsers.map(adminDirectoryRow))
  }

  if (section === 'reports') {
    return withAdminRowLinks([
      { id: 'report-1', primary: 'API 订单未及时响应', secondary: '买家提交脱敏说明，商户待回应', owner: '买家 木舟 / 商户 小葵 API', status: '处理中', risk: '需 24h 内处理', targetType: 'report', detailItems: [{ label: '处理建议', value: '要求商户补充站外确认记录' }, { label: '敏感信息', value: '仅显示脱敏说明' }] },
      { id: 'report-2', primary: '车源剩余名额争议', secondary: '申请记录与站内展示不一致', owner: '买家 青柠 / 车主 北风', status: '待复核', risk: '信息不一致', targetType: 'report', detailItems: [{ label: '处理建议', value: '核对申请记录与站内剩余席位' }, { label: '敏感信息', value: '不展示联系方式' }] },
      { id: 'report-contact-1', primary: '联系方式无效举报', secondary: '联系快照显示可复制，但买家反馈无法联系', owner: '买家 demo_user / 商户 小葵 API', status: '处理中', risk: '只允许纠纷处理员按订单记录查看必要快照', targetType: 'contact-report', detailItems: [{ label: '处理建议', value: '按联系快照检查必要联系方式' }, { label: '可见范围', value: '仅纠纷处理员' }] },
    ])
  }

  if (section === 'appeals') {
    return withAdminRowLinks([
      { id: 'appeal-1', primary: '雨季 申请解除上车限制', secondary: '用户说明已补充，等待复核', owner: '风险处理', status: '申诉复核中', risk: '关联 ride-app-6', targetType: 'appeal', detailItems: [{ label: '关联记录', value: 'ride-app-6' }, { label: '建议动作', value: '确认纠纷关闭后恢复申请能力' }] },
      { id: 'appeal-2', primary: 'beifeng-api 申请恢复商户资格', secondary: '已提交处理说明', owner: '纠纷处理', status: '需要补充信息', risk: '仍有 1 个未解决纠纷', targetType: 'appeal', detailItems: [{ label: '关联商户', value: 'beifeng-api' }, { label: '建议动作', value: '要求补充未解决纠纷处理结果' }] },
    ])
  }

  return withAdminRowLinks(adminAuditLogStore.map(item => ({
    id: item.id,
    primary: item.action,
    secondary: `${item.targetLabel} · ${getReadableStatus(item.beforeStatus)} → ${getReadableStatus(item.afterStatus)}`,
    owner: item.actorLabel,
    status: '已记录',
    risk: item.reason ?? item.createdAt,
    targetType: 'audit-log',
    detailItems: [
      { label: '目标类型', value: item.targetType },
      { label: '目标 ID', value: item.targetId },
      { label: '操作时间', value: item.createdAt },
      { label: '请求追踪', value: `trace-${item.id}` },
    ],
  })))
}

export type AdminSectionPageFilters = {
  q?: string
  view?: 'public' | 'exceptions'
  activeStatus?: string
  risk?: 'all' | 'high' | 'has_note'
}

function adminSectionBackendStatuses(section: AdminSection, activeStatus: string | undefined) {
  if (!activeStatus || activeStatus === '全部') return undefined
  if (section === 'carpools') {
    if (activeStatus === '待处理') return ['pending_review', 'paused']
    if (activeStatus === '需复核') return ['changes_requested']
  }
  if (section === 'api-services') {
    if (activeStatus === '待处理') return ['pending', 'suspended']
    if (activeStatus === '需复核') return ['changes_requested']
  }
  return undefined
}

function filterAdminSectionPageRows(rows: AdminRow[], filters: AdminSectionPageFilters) {
  const query = filters.q?.trim().toLowerCase()
  return rows.filter((row) => {
    const viewMatched = !filters.view
      || (filters.view === 'public' && (row.targetType === 'carpool' ? !isCarpoolExceptionStatus(row.status) : isApiServicePublicStatus(row.status)))
      || (filters.view === 'exceptions' && (row.targetType === 'carpool' ? isCarpoolExceptionStatus(row.status) : isApiServiceExceptionStatus(row.status)))
    const statusMatched = !filters.activeStatus || filters.activeStatus === '全部'
      || (filters.activeStatus === '待处理' && (row.targetType === 'carpool' ? isCarpoolAdminActionStatus(row.status) : isApiServiceAdminActionStatus(row.status)))
      || (filters.activeStatus === '需复核' && row.status.includes('复核'))
      || row.status === filters.activeStatus
    const riskText = `${row.risk} ${row.status}`
    const riskMatched = !filters.risk || filters.risk === 'all'
      || filters.risk === 'has_note' && Boolean(row.risk.trim())
      || filters.risk === 'high' && /高风险|纠纷|举报|封禁|异常|超时|未解决|危险/i.test(riskText)
    return viewMatched && statusMatched && riskMatched
      && (!query || [row.id, row.primary, row.secondary, row.owner, row.status, row.risk, ...(row.detailItems ?? []).flatMap(item => [item.label, item.value])]
        .some(value => value.toLowerCase().includes(query)))
  })
}

export async function getAdminSectionRowsPage(section: AdminSection, filters: AdminSectionPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<AdminRow>> {
  const statuses = adminSectionBackendStatuses(section, filters.activeStatus)
  if (shouldUseRealBackend() && section === 'carpools') {
    return backendAdminCarpoolRowsPage({ q: filters.q, view: filters.view, statuses, risk: filters.risk }, page)
  }
  if (shouldUseRealBackend() && section === 'api-services') {
    return backendAdminAPIServiceRowsPage({ q: filters.q, view: filters.view, statuses, risk: filters.risk }, page)
  }
  if (shouldUseRealBackend() && (section === 'official-prices' || section === 'price-leads')) {
    return backendAdminOfficialPriceRowsPage({
      q: filters.q,
      none: filters.risk === 'high' || Boolean(filters.activeStatus && filters.activeStatus !== '全部'),
    }, page)
  }
  if (shouldUseRealBackend() && section === 'trade-intents') {
    return backendAdminAPIOrderRowsPage({ search: filters.q, risk: filters.risk }, page)
  }
  if (shouldUseRealBackend()) {
    throw new Error(`管理模块 ${section} 未配置服务端分页适配器。`)
  }
  return paginateCursorItems(filterAdminSectionPageRows(await getAdminSectionRows(section), filters), page)
}

function stringValue(value: unknown, fallback = '') {
  return typeof value === 'string' ? value.trim() : fallback
}

function numberValue(value: unknown, fallback = 0) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function openingMethodFromChannel(channel: OpeningChannelOption | undefined): Carpool['openingMethod'] {
  if (!channel) return '其他'
  if (channel.displayName.includes('Apple')) return 'Apple Store'
  if (channel.displayName.includes('本地')) return '本地卡'
  if (channel.displayName.includes('Web') || channel.displayName.includes('团队')) return '其他'
  return '其他'
}

function carpoolWarrantyLabel(payload: SaveCarpoolDraftPayload): Carpool['warranty'] {
  if (payload.warranty.mode === 'no_warranty') return '售后协商'
  if (payload.warranty.mode === 'fixed_days_warranty' || payload.warranty.mode === 'remaining_days_compensation') return '车主承诺'
  return '售后协商'
}

function carpoolRequiresRiskAck(product: CarpoolProductCatalogItem | undefined, payloadRiskNoticeCode?: string | null) {
  return Boolean(product?.riskAckRequired || payloadRiskNoticeCode)
}

function assertCarpoolAccessArrangement(payload: SaveCarpoolDraftPayload, product: CarpoolProductCatalogItem | undefined) {
  if (payload.status !== 'reviewing') return
  if (!product) throw new Error('请选择产品目录。')
  if (product.publishPolicy !== 'allowed') {
    throw new Error(product.publishPolicy === 'info_only' ? '该产品当前仅允许行情和线索展示，不能发布车源。' : '该产品当前不允许发布车源。')
  }
  if (payload.accessArrangementMode === 'not_allowed') {
    throw new Error('共用账号、密码或登录态方案不能发布。')
  }
  const note = payload.accessArrangementNote?.trim() ?? ''
  if (note.length < 8) throw new Error('请填写成员邀请、订阅费用分摊或站外访问安排说明。')
  if (hasCredentialSharingLanguage(note)) {
    throw new Error('访问安排不能包含共享主账号、密码、API Key、Session、Cookie、token 或登录态。')
  }
  if (!payload.distributionMethod) {
    throw new Error('请选择分发方式。')
  }
  if (payload.distributionMethod !== 'sub2api' && payload.distributionMethod !== 'other') {
    throw new Error('分发方式只能选择 Sub2API 或其他。')
  }
  const distributionNote = payload.distributionMethodNote?.trim() ?? ''
  if (payload.distributionMethod === 'other' && !distributionNote) {
    throw new Error('选择其他分发方式时必须填写说明。')
  }
  if (distributionNote && hasCredentialSharingLanguage(distributionNote)) {
    throw new Error('分发方式说明不能包含共享主账号、密码、API Key、Session、Cookie、token 或登录态。')
  }
  if (typeof payload.providesAdminAccount !== 'boolean') {
    throw new Error('请选择是否提供管理员账号。')
  }
  if (!payload.dailyQuotaAmount || payload.dailyQuotaAmount <= 0 || !payload.weeklyQuotaAmount || payload.weeklyQuotaAmount <= 0) {
    throw new Error('请填写有效的每天额度与每周额度。')
  }
  if (typeof payload.followsOfficialQuotaReset !== 'boolean') throw new Error('请选择额度是否跟随官方重置。')
  if (!payload.vpsRegion.trim()) throw new Error('请填写 VPS 区域。')
  if (typeof payload.supportsMainlandChinaDirectConnection !== 'boolean') throw new Error('请选择是否支持国内直连。')
  if (!payload.openingChannelCode || (payload.openingChannelCode === 'other' && !payload.customOpeningChannel.trim())) {
    throw new Error('请选择或填写开通渠道。')
  }
  if (!payload.paymentMethodCode || (payload.paymentMethodCode === 'other' && !payload.customPaymentMethod.trim())) {
    throw new Error('请选择或填写付款方式。')
  }
  if (carpoolRequiresRiskAck(product, payload.riskNoticeCode) && !payload.riskAcknowledged) {
    throw new Error('请先确认该套餐的发布边界。')
  }
}

function apiGatewayFromDistribution(value: unknown): ApiService['delivery'] {
  if (value === 'sub2api') return 'Sub2API'
  return '其他'
}

function apiUsageVisibility(value: unknown): ApiUsageVisibility {
  if (value === 'panel_realtime') return 'panel_realtime'
  if (value === 'panel_balance_only' || value === 'merchant_confirmed') return 'merchant_readonly'
  if (value === 'fixed_package_only' || value === 'not_available') return 'none'
  return 'none'
}

function apiMerchantIdentityMode(value: unknown): ApiMerchantIdentityMode {
  return value === 'store_alias' ? 'store_alias' : 'public_profile'
}

function apiDeliveryModes(value: unknown): ApiDeliveryMode[] {
  if (!Array.isArray(value)) return ['api_key_endpoint']
  const modes = value.filter((mode): mode is ApiDeliveryMode => mode === 'api_key_endpoint' || mode === 'sub2api_panel_account')
  return modes.length ? modes : ['api_key_endpoint']
}

function buildModelPriceRowsFromPayload(payload: Record<string, unknown>, defaultMultiplier: number): ApiService['modelPriceRows'] {
  const selected = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, enabled?: boolean }> : []
  return selected
    .filter(item => item.enabled !== false)
    .map(item => {
      const model = modelCatalog.find(row => row.id === item.modelId)
      return {
        modelId: model?.id ?? item.modelId ?? 'custom-model',
        modelName: model?.name ?? item.modelId ?? '自定义模型',
        provider: model?.provider === 'openai' ? 'OpenAI' : model?.provider === 'anthropic' ? 'Anthropic' : 'Other',
        officialInputPricePerMillion: model?.officialInputPricePerMillion ?? 0,
        officialCachedInputPricePerMillion: model?.officialCachedInputPricePerMillion ?? null,
        officialOutputPricePerMillion: model?.officialOutputPricePerMillion ?? 0,
        merchantMultiplier: defaultMultiplier,
        actualInputPricePerMillion: Number(((model?.officialInputPricePerMillion ?? 0) * defaultMultiplier).toFixed(3)),
        actualCachedInputPricePerMillion: model?.officialCachedInputPricePerMillion === null || model?.officialCachedInputPricePerMillion === undefined ? null : Number((model.officialCachedInputPricePerMillion * defaultMultiplier).toFixed(3)),
        actualOutputPricePerMillion: Number(((model?.officialOutputPricePerMillion ?? 0) * defaultMultiplier).toFixed(3)),
      }
    })
}

export async function submitOfficialPriceLead(payload: Record<string, unknown>) {
  if (shouldUseRealBackend()) return backendSubmitOfficialPriceLead(payload)
  await wait()
  const id = `lead-${Date.now()}`
  const price: OfficialPrice = {
    id,
    product: stringValue(payload.product, '其他'),
    plan: stringValue(payload.plan, '自定义套餐'),
    region: stringValue(payload.region, '其他'),
    channel: stringValue(payload.channel, 'Web'),
    openingMethod: stringValue(payload.openingMethod, '其他'),
    originalPrice: stringValue(payload.originalPrice, '待补充'),
    cny: null,
    status: '待验证',
    source: stringValue(payload.sourceUrl, '用户线索'),
    submitter: currentBuyerName,
    submitterTrust: 3,
    updatedAt: nowText(),
  }
  officialPriceStore.unshift(price)
  persistMarketStores()
  appendAdminAuditLog({
    actorType: 'system',
    actorLabel: currentBuyerName,
    action: '提交低价线索',
    targetType: 'official-price',
    targetId: id,
    targetLabel: `${price.product} ${price.plan}`,
    beforeStatus: null,
    afterStatus: price.status,
    reason: stringValue(payload.note, '用户提交线索'),
  })
  return clone(price)
}

export async function submitCarpool(payload: SaveCarpoolDraftPayload) {
  if (shouldUseRealBackend()) return backendSubmitCarpool(payload)
  await wait()
  const product = carpoolProductCatalog.find(item => item.id === payload.productId)
  const region = carpoolRegions.find(item => item.code === payload.regionCode)
  const regionName = payload.customRegionName?.trim() || region?.displayName || '其他'
  const channel = carpoolOpeningChannels.find(item => item.code === payload.openingChannelCode)
  assertCarpoolAccessArrangement(payload, product)
  const id = `carpool-${Date.now()}`
  const monthly = payload.monthlyPriceCny ?? 0
  const dailyQuotaAmount = payload.dailyQuotaAmount ?? 0
  const weeklyQuotaAmount = payload.weeklyQuotaAmount ?? 0
  const carpool: Carpool = {
    id,
    product: product?.displayName ?? payload.customProductName?.trim() ?? '自定义产品',
    region: regionName,
    monthly,
    serviceMultiplier: 1,
    dailyQuotaAmount,
    weeklyQuotaAmount,
    followsOfficialQuotaReset: payload.followsOfficialQuotaReset,
    vpsRegion: payload.vpsRegion.trim() || null,
    supportsMainlandChinaDirectConnection: payload.supportsMainlandChinaDirectConnection,
    openingChannelCode: payload.openingChannelCode as Carpool['openingChannelCode'],
    customOpeningChannel: payload.customOpeningChannel.trim() || null,
    paymentMethodCode: payload.paymentMethodCode as Carpool['paymentMethodCode'],
    customPaymentMethod: payload.customPaymentMethod.trim() || null,
    quotaLabel: product?.quotaLabel ?? defaultQuotaLabel,
    quotaUnit: product?.quotaUnit ?? defaultQuotaUnit,
    quotaPeriod: product?.quotaPeriod ?? defaultQuotaPeriod,
    seats: `${payload.occupiedSeats}/${payload.totalSeats}`,
    pricingMode: 'fixed',
    fixedMonthlyPrice: monthly,
    currentConfirmedMembers: payload.occupiedSeats,
    maxMembers: payload.totalSeats,
    owner: currentOwnerName,
    trustLevel: 4,
    ownerType: '个人车主',
    warranty: carpoolWarrantyLabel(payload),
    openingMethod: openingMethodFromChannel(channel),
    status: payload.status === 'reviewing' ? '可上车' : '暂停',
    confirmedAt: nowText(),
    confirmedWithin48h: true,
    linuxdoBound: null,
    sourceAuthorVerification: { status: 'not_submitted' },
    hasInfoConflict: false,
    hasUnresolvedDispute: false,
    distributionMethod: payload.distributionMethod || 'other',
    distributionMethodNote: payload.distributionMethodNote?.trim() || '站外分发方式待确认。',
    providesAdminAccount: Boolean(payload.providesAdminAccount),
    accessArrangementMode: payload.accessArrangementMode ?? 'other_off_platform',
    accessArrangementNote: payload.accessArrangementNote?.trim() || '待管理员复核访问安排',
    riskAcknowledged: carpoolRequiresRiskAck(product, payload.riskNoticeCode) ? Boolean(payload.riskAcknowledged) : undefined,
    riskNoticeCode: carpoolRequiresRiskAck(product, payload.riskNoticeCode) ? product?.riskNoticeCode ?? payload.riskNoticeCode ?? undefined : undefined,
  }
  carpoolStore.unshift(carpool)
  persistMarketStores()
  appendAdminAuditLog({
    actorType: 'system',
    actorLabel: currentOwnerName,
    action: payload.status === 'reviewing' ? '发布车源' : '保存车源草稿',
    targetType: 'carpool',
    targetId: id,
    targetLabel: carpool.product,
    beforeStatus: null,
    afterStatus: carpool.status,
    reason: payload.rulesNote,
  })
  return clone(carpool)
}

export async function submitApiService(payload: Record<string, unknown>) {
  if (shouldUseRealBackend()) return backendSubmitAPIService(payload)
  await wait()
  const billing = requireSupportedApiServiceBillingMode(payload.billingMode)
  const isPublish = payload.status === 'reviewing'
  const probeConnectionId = stringValue(payload.probeConnectionId, '')
  const probeConnection = (await getOwnerAPIProbeConnections()).find(connection => connection.id === probeConnectionId)
  if (isPublish && (!probeConnection || !probeConnection.enabled || probeConnection.verificationStatus !== 'verified')) {
    throw new Error('请选择已验证且启用的探针连接。')
  }
  const id = `api-${Date.now()}`
  const normalized = normalizeMerchantDisplayName(payload)
  const gateway = apiGatewayFromDistribution(payload.distributionSystem)
  const defaultMultiplier = numberValue(payload.defaultMultiplier, 1)
  const cnyPerUsdCredit = numberValue(payload.cnyPerUsdCredit, 1)
  const selectedModels = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, enabled?: boolean }> : []
  const models = selectedModels
    .filter(item => item.enabled !== false)
    .map(item => modelCatalog.find(model => model.id === item.modelId)?.name ?? item.modelId ?? '自定义模型')
    .filter(Boolean)
  const merchantIdentityMode = apiMerchantIdentityMode(normalized.merchantIdentityMode)
  const deliveryModes = apiDeliveryModes(payload.deliveryModes)
  const rawPackages = Array.isArray(payload.packages)
    ? payload.packages as Array<{ id?: string, name?: string, priceCny?: number, panelAllowance?: number, quotaUsagePolicy?: unknown, durationDays?: number, stockTotal?: number, description?: string, enabled?: boolean, modelCatalogIds?: string[] }>
    : []
  const paymentOptions = Array.isArray(payload.paymentOptions)
    ? payload.paymentOptions as Array<{ paymentMethod?: string, enabled?: boolean, paymentInstructions?: string, paymentQrCodeDataUrl?: string | null }>
    : []
  const normalizedPaymentOptions = normalizeRawApiPaymentOptions(paymentOptions)
  const hasEnabledPayment = normalizedPaymentOptions.some(item => item.enabled && isApiPaymentOptionComplete(item))
  const publiclyOrderable = isPublish && hasEnabledPayment
  const responseMinutes = numberValue(payload.paymentWindowMinutes, 10)
  const declaredMaxConcurrency = numberValue(payload.declaredMaxConcurrency, 0)
  const promptAuditEnabled = typeof payload.promptAuditEnabled === 'boolean' ? payload.promptAuditEnabled : null
  if (!Number.isInteger(declaredMaxConcurrency) || declaredMaxConcurrency < 1) throw new Error('商户声明最大并发必须是大于 0 的整数。')
  if (promptAuditEnabled === null) throw new Error('请选择是否开启提示词审计。')
  const state: ApiServiceState = isPublish ? 'online' : 'offline'
  const quotaExpiresAt = beijingDateTimeInputToISOString(String(payload.quotaExpiresAt ?? ''))
  const accountPoolType = String(payload.accountPoolType ?? '') as ApiService['accountPoolType']
  const accountPoolLabels: Record<Exclude<NonNullable<ApiService['accountPoolType']>, 'custom'>, string> = {
    gpt_pro_20x: 'GPT Pro 20x',
    gpt_pro_5x: 'GPT Pro 5x',
    gpt_plus: 'GPT Plus',
  }
  const accountPoolLabel = accountPoolType === 'custom'
    ? stringValue(payload.accountPoolCustomName, '')
    : accountPoolType ? accountPoolLabels[accountPoolType] : ''
  const merchantRefundCommitment = (payload.warranty as { mode?: string } | undefined)?.mode === 'merchant_full_refund'
  const quotaUsagePolicy = apiQuotaUsagePolicyFromInput(payload.quotaUsagePolicy)
  const service: ApiService = {
    id,
    version: 1,
    probeConnectionId,
    probeReady: Boolean(probeConnection?.enabled && probeConnection.verificationStatus === 'verified'),
    title: stringValue(payload.generatedTitle, models.length ? `${models[0]} API 服务` : '新 API 服务'),
    sourceUrl: stringValue(payload.sourceUrl, ''),
    quotaUsagePolicy,
    merchantId: currentMerchantId,
    merchantUsername: currentMerchantName,
    merchant: currentMerchantName,
    merchantIdentityMode,
    merchantDisplayName: normalized.merchantDisplayName,
    trustLevel: 4,
    merchantType: '个人车主',
    models: models.length ? models : ['自定义模型'],
    modelMultipliers: (models.length ? models : ['自定义模型']).map(model => ({ model, multiplier: `${defaultMultiplier.toFixed(2)}x` })),
    rate: `${defaultMultiplier.toFixed(2)}x`,
    defaultMultiplier,
    creditPerCny: cnyPerUsdCredit > 0 ? Number((1 / cnyPerUsdCredit).toFixed(2)) : 1,
    minimumPurchaseCny: numberValue(payload.minimumPurchaseCny, 10),
    maxBuy: numberValue(payload.maximumPurchaseCny, 300),
    balance: numberValue(payload.availableCreditUsd, 0),
    delivery: gateway,
    billingMode: billing,
    deliveryModes,
    usageVisibility: apiUsageVisibility(payload.usageVisibility),
    panelBaseUrl: gateway === 'Sub2API' ? '创建订单后由商户站外确认 API 细节' : null,
    imagePricing: {
      supported: Boolean((payload.imageCapability as { enabled?: boolean } | undefined)?.enabled),
      textToImage: Boolean((payload.imageCapability as { supportsTextToImage?: boolean } | undefined)?.supportsTextToImage),
      imageToImage: Boolean((payload.imageCapability as { supportsImageToImage?: boolean } | undefined)?.supportsImageToImage),
      oneKPriceUsd: null,
      twoKPriceUsd: null,
      fourKPriceUsd: null,
    },
    independentApiKey: deliveryModes.includes('api_key_endpoint'),
    independentPanelAccount: deliveryModes.includes('sub2api_panel_account'),
    panelRequiresPasswordReset: deliveryModes.includes('sub2api_panel_account'),
    apiBaseUrlVisibility: 'after_intent',
    panelLoginUrlVisibility: deliveryModes.includes('sub2api_panel_account') ? 'after_intent' : 'off_platform',
    state,
    online: isPublish,
    publiclyOrderable,
    lastOnlineConfirmedAt: nowText(),
    onlineExpiresAt: nowText(),
    declaredMaxConcurrency,
    promptAuditEnabled,
    expectedResponseMinutes: responseMinutes,
    responseMedianMinutes: null,
    dailyOrderLimit: 5,
    todayOrderCount: 0,
    unresolvedDisputes: 0,
    warning: publiclyOrderable ? undefined : isPublish ? '待配置接单设置' : '草稿尚未上线',
    warranty: merchantRefundCommitment
      ? '商户退款承诺：订单有效期内符合规则时退还全部实付金额；平台不垫付、不代赔'
      : '无额外退款承诺，具体问题由双方站外协商；平台不担保、不代赔',
    refundPolicy: merchantRefundCommitment ? 'api-merchant-refund-v1' : '无额外退款承诺',
    accountPoolType,
    accountPoolLabel,
    merchantRefundCommitment,
    merchantRefundPolicyVersion: 'api-merchant-refund-v1',
    quotaExpiresAt: quotaExpiresAt || undefined,
    expiresAt: formatQuotaExpiresAtLabel(quotaExpiresAt) || '按服务说明',
    completed30d: 0,
    reviewCount: 0,
    officialPricingVersion: '2026-06',
    officialPricingUpdatedAt: nowText(),
    merchantNote: stringValue(payload.merchantNote, '建议首次小额测试。'),
    modelPriceRows: buildModelPriceRowsFromPayload(payload, defaultMultiplier),
    packages: rawPackages.map((item, index) => ({
      id: item.id || `package-${Date.now()}-${index}`,
      name: item.name || `套餐 ${index + 1}`,
      priceCny: numberValue(item.priceCny, 0),
      panelAllowance: numberValue(item.panelAllowance, 0),
      quotaUsagePolicy: apiQuotaUsagePolicyFromInput(toApiQuotaUsagePolicyInput(item.quotaUsagePolicy)),
      durationDays: (item.durationDays ?? 1) as 1 | 3 | 7 | 30,
      stockTotal: numberValue(item.stockTotal, 0),
      stockAvailable: numberValue(item.stockTotal, 0),
      description: item.description || '',
      enabled: item.enabled !== false,
      sortOrder: index,
      models: (item.modelCatalogIds ?? []).map(modelId => {
        const model = modelCatalog.find(row => row.id === modelId)
        return {
          serviceModelId: `service-${modelId}`,
          modelCatalogId: modelId,
          modelPriceVersionId: '',
          modelName: model?.name ?? modelId,
          provider: model?.provider ?? 'other',
          merchantMultiplier: defaultMultiplier,
        }
      }),
    })),
    recommendationResponseMedianMinutes: null,
    serviceUpdatedAt: nowText(),
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: `@${currentMerchantName}` }],
    acceptedPaymentMethods: normalizedPaymentOptions.filter(option => option.enabled).map(option => option.paymentMethod),
  }
  apiServicePaymentSnapshotStore[id] = normalizeApiPaymentAccountSettings({
    paymentWindowMinutes: responseMinutes,
    paymentOptions: normalizedPaymentOptions,
  }).paymentOptions
  apiServiceStore.unshift(service)
  updateMockAPIProbeConnectionReference({
    connectionId: probeConnectionId,
    serviceId: service.id,
    serviceTitle: service.title,
  })
  persistMarketStores()
  appendAdminAuditLog({
    actorType: 'system',
    actorLabel: currentMerchantName,
    action: isPublish ? '发布 API 服务' : '保存 API 服务草稿',
    targetType: 'api-service',
    targetId: id,
    targetLabel: service.title,
    beforeStatus: null,
    afterStatus: state,
    reason: service.merchantNote,
  })
  return clone(service)
}

export async function publishApiService(id: string) {
  if (shouldUseRealBackend()) return backendPublishAPIService(id)
  await wait()
  const target = apiServiceStore.find(item => item.id === id)
  if (!target) throw new Error('API 服务不存在。')
  if (target.state !== 'offline') throw new Error('当前 API 服务不能上线。')
  requireSupportedApiServiceBillingMode(target.billingMode)
  if (!target.probeConnectionId || !target.probeReady) throw new Error('上线前必须绑定已验证且启用的探针连接。')
  target.state = 'online'
  target.online = true
  target.publiclyOrderable = true
  target.warning = undefined
  target.lastOnlineConfirmedAt = nowText()
  persistMarketStores()
  return clone(target)
}

export async function updateApiServiceProbeConnection(input: {
  id: string
  probeConnectionId: string
  version: number
}) {
  if (shouldUseRealBackend()) return backendUpdateAPIServiceProbeConnection(input)
  await wait()
  const target = apiServiceStore.find(item => item.id === input.id)
  if (!target) throw new Error('API 服务不存在。')
  const currentVersion = target.version ?? 1
  if (currentVersion !== input.version) throw new Error('API 服务已更新，请刷新后重试。')
  const connection = input.probeConnectionId
    ? (await getOwnerAPIProbeConnections()).find(item => item.id === input.probeConnectionId)
    : null
  if (input.probeConnectionId && (!connection || !connection.enabled || connection.verificationStatus !== 'verified')) {
    throw new Error('只能绑定已验证且启用的探针连接。')
  }
  const previousConnectionId = target.probeConnectionId
  target.probeConnectionId = connection?.id
  target.probeReady = Boolean(connection)
  target.version = currentVersion + 1
  target.publiclyOrderable = Boolean(connection) && target.online
  target.warning = connection ? undefined : '未绑定可用探针连接'
  target.serviceUpdatedAt = nowText()
  updateMockAPIProbeConnectionReference({
    previousConnectionId,
    connectionId: connection?.id,
    serviceId: target.id,
    serviceTitle: target.title,
  })
  persistMarketStores()
  return clone(target)
}

export async function pauseApiService(id: string) {
  if (shouldUseRealBackend()) return backendPauseAPIService(id)
  await wait()
  const target = apiServiceStore.find(item => item.id === id)
  if (!target) throw new Error('API 服务不存在。')
  if (!target.online) throw new Error('当前 API 服务未上线。')
  target.state = 'paused'
  target.online = false
  target.publiclyOrderable = false
  target.warning = '商户暂停接单'
  target.lastOnlineConfirmedAt = nowText()
  persistMarketStores()
  return clone(target)
}

export async function resumeApiService(id: string) {
  if (shouldUseRealBackend()) return backendResumeAPIService(id)
  await wait()
  const target = apiServiceStore.find(item => item.id === id)
  if (!target) throw new Error('API 服务不存在。')
  if (target.state !== 'paused') throw new Error('当前 API 服务不能恢复。')
  requireSupportedApiServiceBillingMode(target.billingMode)
  if (!target.probeConnectionId || !target.probeReady) throw new Error('恢复接单前必须绑定已验证且启用的探针连接。')
  target.state = 'online'
  target.online = true
  target.publiclyOrderable = true
  target.warning = undefined
  target.lastOnlineConfirmedAt = nowText()
  persistMarketStores()
  return clone(target)
}

export function getFeedbackTypeLabel(value: FeedbackTicketType) {
  return feedbackTypeLabel(value)
}

export function getFeedbackImpactLabel(value: FeedbackImpact) {
  return feedbackImpactLabel(value)
}

export function getFeedbackStatusLabel(value: FeedbackStatus) {
  return feedbackStatusLabel(value)
}

function feedbackUnread(item: FeedbackTicket) {
  if (!item.latestAdminUpdateAt) return false
  if (!item.submitterReadAt) return true
  return new Date(item.submitterReadAt).getTime() < new Date(item.latestAdminUpdateAt).getTime()
}

function normalizeFeedbackTicket(item: FeedbackTicket): FeedbackTicket {
  return { ...item, unread: feedbackUnread(item) }
}

function feedbackNotificationId(id: string) {
  return `feedback-notice-${id}`
}

function addFeedbackEvent(ticket: FeedbackTicket, event: Omit<FeedbackEvent, 'id' | 'createdAt'> & { createdAt?: string }) {
  const createdAt = event.createdAt ?? nowText()
  ticket.events = [
    {
      id: `feedback-event-${Date.now()}-${(ticket.events ?? []).length + 1}`,
      createdAt,
      ...event,
    },
    ...(ticket.events ?? []),
  ]
}

function feedbackAdminRow(item: FeedbackTicket): AdminRow {
  const normalized = normalizeFeedbackTicket(item)
  return {
    id: item.id,
    primary: item.title,
    secondary: `${feedbackTypeLabel(item.type)} · ${item.contextPageLabel}${item.contextTargetLabel ? ` · ${item.contextTargetLabel}` : ''}`,
    owner: item.submitterName || item.submitterUsername || '用户',
    status: feedbackStatusLabel(item.status),
    risk: normalized.unread ? '用户未读处理结果' : feedbackImpactLabel(item.impact),
    targetType: 'feedback-ticket',
    backendKind: 'feedback-ticket',
    backendVersion: item.version,
    targetTo: `/admin/feedback/${item.id}`,
    detailItems: [
      { label: '反馈类型', value: feedbackTypeLabel(item.type) },
      { label: '影响程度', value: feedbackImpactLabel(item.impact) },
      { label: '当前页面', value: item.contextPageLabel },
      { label: '关联内容', value: item.contextTargetLabel || '未指定' },
      { label: '当前身份', value: item.contextRoleLabel || '普通用户' },
      { label: '用户已读', value: normalized.unread ? '否' : '是' },
    ],
  }
}

export async function submitFeedback(payload: SubmitFeedbackPayload): Promise<FeedbackTicket> {
  if (shouldUseRealBackend()) return backendCreateFeedbackTicket(payload)
  await wait()
  const now = nowText()
  const title = payload.title?.trim() || `${feedbackTypeLabel(payload.type)} · ${payload.contextPageLabel}`
  const ticket: FeedbackTicket = {
    id: `feedback-${Date.now()}`,
    submitterUserId: currentBuyerId,
    submitterUsername: myUserProfileStore.username,
    submitterName: myUserProfileStore.displayName || myUserProfileStore.username || currentBuyerName,
    type: payload.type,
    impact: payload.impact,
    status: 'submitted',
    title,
    description: payload.description.trim(),
    contextPageLabel: payload.contextPageLabel.trim(),
    contextTargetType: payload.contextTargetType?.trim() || 'page',
    contextTargetId: payload.contextTargetId?.trim() || '',
    contextTargetLabel: payload.contextTargetLabel?.trim() || '未指定',
    contextRoleLabel: payload.contextRoleLabel?.trim() || '普通用户',
    latestAdminUpdateAt: null,
    submitterReadAt: null,
    unread: false,
    createdAt: now,
    updatedAt: now,
    version: 1,
    events: [],
  }
  addFeedbackEvent(ticket, {
    actorUserId: currentBuyerId,
    actorName: ticket.submitterName,
    actorRole: 'user',
    action: 'submitted',
    publicMessage: payload.description.trim(),
    createdAt: now,
  })
  feedbackTicketStore.unshift(ticket)
  persistFeedbackTickets()
  appendAdminAuditLog({
    actorType: 'system',
    actorLabel: ticket.submitterName,
    action: '提交问题反馈',
    targetType: 'feedback-ticket',
    targetId: ticket.id,
    targetLabel: ticket.title,
    beforeStatus: null,
    afterStatus: feedbackStatusLabel(ticket.status),
    reason: `${feedbackTypeLabel(ticket.type)} · ${feedbackImpactLabel(ticket.impact)}`,
    createdAt: now,
  })
  return clone(normalizeFeedbackTicket(ticket))
}

export async function getMyFeedbackTickets(): Promise<FeedbackTicket[]> {
  if (shouldUseRealBackend()) return backendMyFeedbackTickets()
  await wait()
  return clone(feedbackTicketStore
    .filter(item => item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username)
    .map(normalizeFeedbackTicket)
    .sort((a, b) => compareTimeDesc(a.updatedAt, b.updatedAt)))
}

export async function getMyFeedbackTicket(id: string): Promise<FeedbackTicket | null> {
  if (shouldUseRealBackend()) return backendMyFeedbackTicket(id)
  await wait()
  const item = feedbackTicketStore.find(row => row.id === id && (row.submitterUserId === currentBuyerId || row.submitterUsername === myUserProfileStore.username))
  return clone(item ? normalizeFeedbackTicket(item) : null)
}

export async function getFeedbackUnreadCount(): Promise<number> {
  if (shouldUseRealBackend()) return backendFeedbackUnreadCount()
  await wait()
  return feedbackTicketStore
    .filter(item => item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username)
    .filter(item => normalizeFeedbackTicket(item).unread)
    .length
}

export async function addFeedbackSupplement(id: string, payload: FeedbackSupplementPayload): Promise<FeedbackTicket> {
  if (shouldUseRealBackend()) return backendAddFeedbackSupplement(id, payload)
  await wait()
  const target = feedbackTicketStore.find(item => item.id === id && (item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username))
  if (!target) throw new Error('未找到这条反馈。')
  if (target.status === 'closed') throw new Error('已关闭反馈不能继续补充。')
  const message = payload.message.trim()
  if (message.length < 2) throw new Error('请填写补充说明。')
  const now = nowText()
  if (target.status === 'needs_user_info') target.status = 'submitted'
  target.updatedAt = now
  target.version += 1
  addFeedbackEvent(target, {
    actorUserId: currentBuyerId,
    actorName: target.submitterName,
    actorRole: 'user',
    action: 'user_supplemented',
    publicMessage: message,
    createdAt: now,
  })
  persistFeedbackTickets()
  return clone(normalizeFeedbackTicket(target))
}

export async function markFeedbackRead(id: string): Promise<FeedbackTicket> {
  if (shouldUseRealBackend()) return backendMarkFeedbackRead(id)
  await wait()
  const target = feedbackTicketStore.find(item => item.id === id && (item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username))
  if (!target) throw new Error('未找到这条反馈。')
  if (target.latestAdminUpdateAt && feedbackUnread(target)) {
    const now = nowText()
    target.submitterReadAt = now
    target.updatedAt = now
    target.version += 1
    addFeedbackEvent(target, {
      actorUserId: currentBuyerId,
      actorName: target.submitterName,
      actorRole: 'user',
      action: 'read',
      publicMessage: '用户已查看处理结果',
      createdAt: now,
    })
    const noticeId = feedbackNotificationId(id)
    if (!notificationReadStore.includes(noticeId)) {
      notificationReadStore = [...notificationReadStore, noticeId]
      persistNotificationReadState()
    }
    persistFeedbackTickets()
  }
  return clone(normalizeFeedbackTicket(target))
}

export async function getAdminFeedbackTickets(): Promise<FeedbackTicket[]> {
  if (shouldUseRealBackend()) return backendAdminFeedbackTickets()
  await wait()
  return clone(feedbackTicketStore
    .map(normalizeFeedbackTicket)
    .sort((a, b) => compareTimeDesc(a.updatedAt, b.updatedAt)))
}

export async function getAdminFeedbackTicket(id: string): Promise<FeedbackTicket | null> {
  if (shouldUseRealBackend()) return backendAdminFeedbackTicket(id)
  await wait()
  const item = feedbackTicketStore.find(row => row.id === id)
  return clone(item ? normalizeFeedbackTicket(item) : null)
}

export async function handleFeedbackTicket(id: string, payload: FeedbackAdminHandlePayload, version?: number): Promise<FeedbackTicket> {
  if (shouldUseRealBackend()) return backendHandleFeedbackTicket(id, payload, version ?? 0)
  await wait()
  const target = feedbackTicketStore.find(item => item.id === id)
  if (!target) throw new Error('未找到这条反馈。')
  if (target.status === 'closed') throw new Error('已关闭反馈不能继续处理。')
  if (version && target.version !== version) throw new Error('反馈内容已更新，请刷新后再处理。')
  const response = payload.response.trim()
  if (response.length < 2) throw new Error('请填写给用户看的处理说明。')
  const now = nowText()
  target.status = payload.status
  target.adminResponse = response
  target.adminInternalNote = payload.internalNote?.trim() || undefined
  target.handledByAdminId = 'admin-local'
  target.handledByAdminName = '管理员'
  target.handledAt = now
  target.latestAdminUpdateAt = now
  target.submitterReadAt = null
  target.updatedAt = now
  target.version += 1
  addFeedbackEvent(target, {
    actorUserId: 'admin-local',
    actorName: '管理员',
    actorRole: 'admin',
    action: 'admin_handled',
    publicMessage: response,
    internalNote: target.adminInternalNote,
    createdAt: now,
  })
  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '管理员',
    action: '处理问题反馈',
    targetType: 'feedback-ticket',
    targetId: target.id,
    targetLabel: target.title,
    beforeStatus: '',
    afterStatus: feedbackStatusLabel(target.status),
    reason: response,
    createdAt: now,
  })
  persistFeedbackTickets()
  return clone(normalizeFeedbackTicket(target))
}

export async function getAdminFeedbackRows(): Promise<AdminRow[]> {
  if (shouldUseRealBackend()) return backendAdminFeedbackRows()
  await wait()
  return withAdminRowLinks(feedbackTicketStore
    .map(feedbackAdminRow)
    .sort((a, b) => {
      const sourceA = feedbackTicketStore.find(item => item.id === a.id)
      const sourceB = feedbackTicketStore.find(item => item.id === b.id)
      return compareTimeDesc(sourceA?.updatedAt ?? '', sourceB?.updatedAt ?? '')
    }))
}

function markReadState<T extends { id: string, unread: boolean }>(items: T[]) {
  return items.map(item => ({ ...item, unread: item.unread && !notificationReadStore.includes(item.id) }))
}

async function buildUnifiedNotifications(): Promise<UnifiedNotification[]> {
  const carpoolRows: UnifiedNotification[] = carpoolApplicationStore
    .filter(item => [currentBuyerId, currentOwnerId].includes(item.applicantUserId) || [currentBuyerId, currentOwnerId].includes(item.ownerUserId))
    .filter(item => ['pending_owner', 'accepted_reserved', 'contacted', 'joined_pending_confirmation', 'pending_completion', 'disputed', 'rejected'].includes(item.status))
    .map(item => {
      const isOwner = item.ownerUserId === currentOwnerId
      return {
        id: `carpool-notice-${item.id}`,
        type: '上车申请',
        title: getCarpoolApplicationStatusLabel(item.status),
        detail: `${item.snapshot.productName} · ${item.applicantUsername} / ${item.ownerUsername}`,
        time: item.updatedAt,
        unread: item.status !== 'rejected',
        to: isOwner ? `/merchant/carpool-applications/${item.id}` : `/my/rides/${item.id}`,
      }
    })

  const apiRows: UnifiedNotification[] = apiPurchaseIntentStore
    .filter(item => ['open', 'contacted', 'buyer_cancelled', 'owner_closed'].includes(item.status))
    .map(item => ({
      id: `api-notice-${item.id}`,
      type: 'API 意向',
      title: getApiStatusLabel(item.status),
      detail: `${item.snapshot.serviceTitle} · ${item.buyer} / ${getApiMerchantDisplayName(item)}`,
      time: item.updatedAt,
      unread: item.status === 'open' || item.status === 'contacted',
      to: item.merchantId === currentMerchantId ? `/merchant/api-orders` : `/my/api-orders/${item.id}`,
    }))

  const officialRows: UnifiedNotification[] = officialPriceStore
    .filter(item => item.submitter === currentBuyerName || item.status === '待验证')
    .slice(0, 6)
    .map(item => ({
      id: `official-notice-${item.id}`,
      type: '审核结果',
      title: item.status === '待验证' ? '低价线索待验证' : '低价线索状态更新',
      detail: `${item.product} ${item.plan} · ${item.region} · ${item.status}`,
      time: item.updatedAt,
      unread: item.status === '待验证',
      to: `/official-prices/${item.id}`,
    }))

  const feedbackRows: UnifiedNotification[] = feedbackTicketStore
    .filter(item => item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username)
    .filter(item => item.latestAdminUpdateAt)
    .map(item => {
      const normalized = normalizeFeedbackTicket(item)
      return {
        id: feedbackNotificationId(item.id),
        type: '问题反馈',
        title: item.status === 'needs_user_info' ? '你的问题反馈需要补充' : '你的问题反馈已有处理结果',
        detail: `${feedbackStatusLabel(item.status)} · ${item.title}`,
        time: item.latestAdminUpdateAt ?? item.updatedAt,
        unread: normalized.unread,
        to: `/my/feedback/${item.id}`,
      }
    })

  const adminRows: UnifiedNotification[] = adminAuditLogStore.slice(0, 6).map(item => ({
    id: `audit-notice-${item.id}`,
    type: '管理操作',
    title: item.action,
    detail: `${item.targetLabel} · ${getReadableStatus(item.beforeStatus)} → ${getReadableStatus(item.afterStatus)}`,
    time: item.createdAt,
    unread: false,
    to: '/admin/logs',
  }))

  const fixedRows: UnifiedNotification[] = [{
    id: 'boundary-reminder',
    type: '边界提醒',
    title: '平台不保存密钥',
    detail: '不要在表单或站内说明中提交账号密码、API Key、session 或 refresh token。',
    time: '1 小时前',
    unread: false,
    to: '/my/notifications',
  }]

  return markReadState([...carpoolRows, ...apiRows, ...officialRows, ...feedbackRows, ...adminRows, ...fixedRows]
    .sort((a, b) => compareTimeDesc(a.time, b.time)))
}

export async function getNotifications(): Promise<UnifiedNotification[]> {
  if (shouldUseRealBackend()) return backendNotifications()
  await wait()
  return clone(await buildUnifiedNotifications())
}

export async function getNavigationBadges(): Promise<NavigationBadgeSummary> {
  if (shouldUseRealBackend()) return backendNavigationBadges()

  materializeMockApiOrderReviews()

  const [notifications, importantAnnouncementUnread, reportRows, appealRows] = await Promise.all([
    buildUnifiedNotifications(),
    getImportantAnnouncementUnreadCount(),
    getAdminSectionRows('reports'),
    getAdminSectionRows('appeals'),
  ])
  const currentTime = Date.now()
  const isPendingPaymentActive = (order: ApiOrder) => order.status === 'pending_payment'
    && new Date(order.paymentExpiresAt).getTime() > currentTime
  const reservedStatuses: CarpoolApplicationStatus[] = ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation']
  const hasActiveReservation = (application: CarpoolApplication) => reservedStatuses.includes(application.status)
    && Boolean(application.reservedUntil)
    && new Date(application.reservedUntil!).getTime() > currentTime
  const buyerCarpoolActions = carpoolApplicationStore.filter(item => item.applicantUserId === currentBuyerId)
    .filter(item => (hasActiveReservation(item) && !item.buyerConfirmedJoinedAt)
      || (['active', 'pending_completion'].includes(item.status) && Boolean(item.ownerConfirmedCompletedAt) && !item.buyerConfirmedCompletedAt))
    .length
  const merchantCarpoolActions = carpoolApplicationStore.filter(item => item.ownerUserId === currentOwnerId)
    .filter(item => item.status === 'pending_owner'
      || (hasActiveReservation(item) && Boolean(item.buyerConfirmedJoinedAt) && !item.ownerConfirmedJoinedAt)
      || (['active', 'pending_completion'].includes(item.status) && Boolean(item.buyerConfirmedCompletedAt) && !item.ownerConfirmedCompletedAt))
    .length

  const summary: NavigationBadgeSummary = {
    generatedAt: new Date(currentTime).toISOString(),
    notificationUnread: notifications.filter(item => item.unread).length,
    importantAnnouncementUnread,
    feedbackUnread: feedbackTicketStore
      .filter(item => item.submitterUserId === currentBuyerId || item.submitterUsername === myUserProfileStore.username)
      .filter(feedbackUnread)
      .length,
    buyer: {
      carpoolActions: buyerCarpoolActions,
      apiOrderActions: apiOrderStore
        .filter(item => item.buyerId === currentBuyerId)
        .filter(item => isPendingPaymentActive(item) || item.status === 'payment_issue' || item.status === 'delivery_submitted')
        .length,
    },
    merchant: {
      carpoolActions: merchantCarpoolActions,
      apiOrderActions: apiOrderStore
        .filter(item => item.sellerId === currentMerchantId)
        .filter(item => item.status === 'payment_submitted' || item.status === 'paid_confirmed')
        .length,
    },
    admin: null,
  }

  if (myUserProfileStore.permissions.includes('admin')) {
    const admin = {
      officialPrices: officialPriceStore.filter(item => item.status === '待验证').length,
      carpools: carpoolStore.filter(item => item.status === '审核中' || item.status === '暂停').length,
      apiServices: apiServiceStore.filter(item => item.state === 'reviewing' || (item.state === 'paused' && item.warning === '已下架')).length,
      feedbackTickets: feedbackTicketStore.filter(item => item.status === 'submitted' || item.status === 'following_up').length,
      reports: reportRows.filter(item => !item.status.includes('关闭') && !item.status.includes('需要补充')).length
        + appealRows.filter(item => item.status === '申诉复核中').length,
    }
    summary.admin = {
      ...admin,
      total: admin.officialPrices + admin.carpools + admin.apiServices + admin.feedbackTickets + admin.reports,
    }
  }

  return clone(summary)
}

export async function markNotificationRead(id: string) {
  if (shouldUseRealBackend()) {
    const notification = await backendMarkNotificationRead(id)
    const feedbackId = notification?.to.startsWith('/my/feedback/') ? notification.to.split('/').pop() : null
    if (feedbackId) await backendMarkFeedbackRead(feedbackId)
    return notification
  }
  await wait()
  if (!notificationReadStore.includes(id)) {
    notificationReadStore = [...notificationReadStore, id]
    persistNotificationReadState()
  }
  if (id.startsWith('feedback-notice-')) {
    const feedbackId = id.replace('feedback-notice-', '')
    const target = feedbackTicketStore.find(item => item.id === feedbackId)
    if (target?.latestAdminUpdateAt && feedbackUnread(target)) {
      target.submitterReadAt = nowText()
      target.updatedAt = target.submitterReadAt
      target.version += 1
      persistFeedbackTickets()
    }
  }
  const notifications = await buildUnifiedNotifications()
  return clone(notifications.find(item => item.id === id) ?? null)
}

export async function markAllNotificationsRead() {
  if (shouldUseRealBackend()) {
    const notifications = await backendMarkAllNotificationsRead()
    await Promise.all(notifications
      .map(item => item.to.startsWith('/my/feedback/') ? item.to.split('/').pop() : null)
      .filter((feedbackId): feedbackId is string => Boolean(feedbackId))
      .map(feedbackId => backendMarkFeedbackRead(feedbackId)))
    return notifications
  }
  await wait()
  const notifications = await buildUnifiedNotifications()
  notificationReadStore = Array.from(new Set([...notificationReadStore, ...notifications.map(item => item.id)]))
  persistNotificationReadState()
  const now = nowText()
  feedbackTicketStore = feedbackTicketStore.map(item => feedbackUnread(item)
    ? { ...item, submitterReadAt: now, updatedAt: now, version: item.version + 1 }
    : item)
  persistFeedbackTickets()
  return clone(await buildUnifiedNotifications())
}

export async function toggleFavorite(targetType: FavoriteTargetType, targetId: string) {
  if (shouldUseRealBackend()) return backendToggleFavorite(targetType, targetId)
  await wait()
  const id = `${targetType}:${targetId}`
  const exists = favoriteStore.some(item => item.id === id)
  favoriteStore = exists
    ? favoriteStore.filter(item => item.id !== id)
    : [{ id, targetType, targetId, createdAt: nowText() }, ...favoriteStore]
  persistFavorites()
  return clone({ favorited: !exists })
}

export async function getFavorites(): Promise<FavoriteListItem[]> {
  if (shouldUseRealBackend()) return backendFavorites()
  await wait()
  const rows = favoriteStore.map(item => {
    if (item.targetType === 'carpool') {
      const carpool = carpoolStore.find(row => row.id === item.targetId)
      if (!carpool) return null
      return {
        ...item,
        title: carpool.product,
        subtitle: `${carpool.region} · ${getPricingDisplay(carpool).primaryLabel} ¥${getPricingDisplay(carpool).primaryPrice}/月`,
        status: carpool.status,
        to: `/carpools/${carpool.id}`,
      }
    }
    const service = apiServiceStore.find(row => row.id === item.targetId)
    if (!service) return null
    return {
      ...item,
      title: service.title,
      subtitle: `${getApiMerchantDisplayName(service)} · ${service.models.slice(0, 2).join(' / ')}`,
      status: isApiServicePubliclyOrderable(service) ? '可创建订单' : service.online ? '待配置接单' : service.state === 'reviewing' ? '审核中' : '离线',
      to: getApiServicePublicDetailUrl(service) ?? '/api-market',
    }
  }).filter((item): item is FavoriteListItem => item !== null)
  return clone(rows)
}

export async function isFavorite(targetType: FavoriteTargetType, targetId: string) {
  if (shouldUseRealBackend()) return backendFavoriteStatus(targetType, targetId)
  await wait()
  return favoriteStore.some(item => item.id === `${targetType}:${targetId}`)
}

export async function searchMarket(keyword: string): Promise<SearchResult[]> {
  if (shouldUseRealBackend()) return backendSearchMarket(keyword)
  await wait()
  const q = keyword.trim().toLowerCase()
  if (!q) return []
  const officialResults = officialPriceStore
    .filter(item => [item.product, item.plan, item.region, item.channel, item.submitter].some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `official-${item.id}`, type: '官方价格' as const, title: `${item.product} ${item.plan}`, subtitle: `${item.region} · ${item.originalPrice} · ${item.status}`, badge: item.status, to: `/official-prices/${item.id}` }))
  const carpoolResults = carpoolStore
    .filter(item => [item.product, item.region, item.owner].some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `carpool-${item.id}`, type: '车源' as const, title: item.product, subtitle: `${item.region} · ${item.owner} · ${getPricingDisplay(item).primaryLabel} ¥${getPricingDisplay(item).primaryPrice}/月`, badge: item.status, to: `/carpools/${item.id}` }))
  const apiResults = apiServiceStore
    .filter(isApiServicePubliclyOrderable)
    .filter(item => apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `api-${item.id}`, type: 'API 服务' as const, title: item.title, subtitle: `${getApiMerchantDisplayName(item)} · ${item.models.slice(0, 3).join(' / ')}`, badge: '可创建订单', to: `/api-market/${item.id}` }))
  const merchantResults = publicMerchantProfiles
    .filter(item => [item.username, item.displayName, item.identity, item.merchantId].some(value => value.toLowerCase().includes(q)))
    .map(item => ({
      id: `merchant-${item.username}`,
      type: '商户' as const,
      title: item.displayName,
      subtitle: `@${item.username} · ${item.identity} · 近90天完成 ${item.completedLast90Days ?? '暂无数据'}`,
      badge: item.unresolvedDisputes === null ? '纠纷数据暂无' : item.unresolvedDisputes > 0 ? `${item.unresolvedDisputes} 个未解决纠纷` : item.originalPostBound === true ? '原帖已绑定' : item.originalPostBound === false ? '原帖未绑定' : '原帖数据暂无',
      to: `/u/${item.username}`,
    }))
  const userResults = publicUserProfiles
    .filter(item => [item.username, item.displayName].some(value => value.toLowerCase().includes(q)))
    .map(raw => {
      const item = sanitizePublicUserProfile(raw)
      return {
        id: `user-${item.username}`,
        type: '用户' as const,
        title: item.displayName,
        subtitle: `公开个人主页 · @${item.username} · ${item.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.trustLevel}`}`,
        badge: item.linuxDoBound ? '已绑定 linux.do' : '未绑定',
        to: `/u/${item.username}`,
      }
    })
  return clone([...officialResults, ...carpoolResults, ...apiResults, ...merchantResults, ...userResults])
}

export async function getReviewCenterRows(): Promise<ReviewCenterData> {
  if (shouldUseRealBackend()) return backendReviewCenterRows()
  await wait()
  const reviewWindowMs = 14 * 24 * 60 * 60 * 1000
  const carpoolRows = carpoolApplicationStore
    .filter(item => item.status === 'completed' && (item.applicantUserId === currentBuyerId || item.ownerUserId === currentOwnerId))
    .flatMap((item): ReviewCenterRow[] => {
      const currentIsBuyer = item.applicantUserId === currentBuyerId
      const ownReview = currentIsBuyer ? item.buyerReview : item.ownerReview
      const counterpartyReview = currentIsBuyer ? item.ownerReview : item.buyerReview
      const completedAt = item.completedAt ?? item.updatedAt
      const completedAtMs = Date.parse(completedAt)
      if (!Number.isFinite(completedAtMs)) throw new Error(`评价交易完成时间无效：${item.id}`)
      const reviewDeadlineAt = new Date(completedAtMs + reviewWindowMs).toISOString()
      const deadlinePassed = Date.now() >= Date.parse(reviewDeadlineAt)
      const bothSubmitted = Boolean(ownReview && counterpartyReview)
      const published = bothSubmitted || deadlinePassed
      const visibleAt = published
        ? bothSubmitted
          ? new Date(Math.max(Date.parse(ownReview!.createdAt), Date.parse(counterpartyReview!.createdAt))).toISOString()
          : reviewDeadlineAt
        : null
      const counterparty = currentIsBuyer ? item.ownerUsername : item.applicantUsername
      const ownReviewerRole = currentIsBuyer ? 'buyer' as const : 'seller' as const
      const ownRevieweeRole = currentIsBuyer ? 'seller' as const : 'buyer' as const
      const rows: ReviewCenterRow[] = []

      if (ownReview) {
        rows.push({
          id: `review-carpool-${item.id}-sent`,
          transactionType: 'carpool_membership',
          transactionId: item.id,
          direction: 'sent',
          target: item.snapshot.productName,
          counterparty,
          counterpartyUsername: counterparty,
          reviewerRole: ownReviewerRole,
          revieweeRole: ownRevieweeRole,
          status: published ? 'published' : 'sealed',
          visibility: published ? 'published' : 'sealed',
          counterpartySubmitted: Boolean(counterpartyReview),
          canCreate: false,
          canEdit: !published,
          rating: ownReview.rating,
          tags: [...ownReview.tags],
          note: ownReview.note,
          completedAt,
          reviewDeadlineAt,
          submittedAt: ownReview.createdAt,
          visibleAt,
          frozenAt: visibleAt,
          createdAt: ownReview.createdAt,
          updatedAt: ownReview.createdAt,
          version: 1,
        })
      } else {
        rows.push({
          id: `reviewable-carpool-${item.id}`,
          transactionType: 'carpool_membership',
          transactionId: item.id,
          direction: 'pending',
          target: item.snapshot.productName,
          counterparty,
          counterpartyUsername: counterparty,
          reviewerRole: ownReviewerRole,
          revieweeRole: ownRevieweeRole,
          status: deadlinePassed ? 'expired' : 'reviewable',
          visibility: 'none',
          counterpartySubmitted: Boolean(counterpartyReview),
          canCreate: !deadlinePassed,
          canEdit: false,
          rating: null,
          tags: [],
          note: null,
          completedAt,
          reviewDeadlineAt,
          submittedAt: null,
          visibleAt: null,
          frozenAt: null,
          createdAt: completedAt,
          updatedAt: completedAt,
          version: 0,
        })
      }

      if (counterpartyReview) {
        rows.push({
          id: `review-carpool-${item.id}-received`,
          transactionType: 'carpool_membership',
          transactionId: item.id,
          direction: 'received',
          target: item.snapshot.productName,
          counterparty,
          counterpartyUsername: counterparty,
          reviewerRole: ownRevieweeRole,
          revieweeRole: ownReviewerRole,
          status: published ? 'published' : 'sealed',
          visibility: published ? 'published' : 'sealed',
          counterpartySubmitted: true,
          canCreate: false,
          canEdit: false,
          rating: published ? counterpartyReview.rating : null,
          tags: published ? [...counterpartyReview.tags] : [],
          note: published ? counterpartyReview.note : null,
          completedAt,
          reviewDeadlineAt,
          submittedAt: counterpartyReview.createdAt,
          visibleAt,
          frozenAt: visibleAt,
          createdAt: counterpartyReview.createdAt,
          updatedAt: counterpartyReview.createdAt,
          version: 1,
        })
      }
      return rows
    })
  return {
    items: clone(carpoolRows.sort((a, b) => compareTimeDesc(a.createdAt, b.createdAt))),
    presetTags: ['沟通顺畅', '描述真实', '响应及时', '规则清晰', '付款及时', '确认及时', '交付清晰', '合作愉快', '响应较慢', '与描述不符'],
  }
}

export async function getReviewCenterPage(direction: ReviewCenterRow['direction'] | undefined, page: CursorPageRequest = {}): Promise<CursorPage<ReviewCenterRow> & { presetTags: string[] }> {
  if (shouldUseRealBackend()) return backendReviewCenterPage(direction, page)
  const center = await getReviewCenterRows()
  return {
    ...paginateCursorItems(center.items.filter(item => !direction || item.direction === direction), page),
    presetTags: center.presetTags,
  }
}

export async function submitReview(payload: SubmitReviewPayload) {
  if (shouldUseRealBackend()) return backendSubmitReview(payload)
  await wait()
  await reviewCarpoolApplication(payload.transactionId, {
    rating: payload.rating,
    tags: payload.tags,
    note: payload.note,
  })
  const center = await getReviewCenterRows()
  const row = center.items.find(item => item.transactionId === payload.transactionId && item.direction === 'sent')
  if (!row) throw new Error('评价已保存，但评价中心记录暂不可用。')
  return row
}

export async function getCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  await wait()
  return clone(filterCarpoolApplications(filters))
}

export async function getMyCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  if (shouldUseRealBackend()) return backendMyCarpoolApplications(filters)
  await wait()
  return clone(filterCarpoolApplications({ ...filters, buyerId: currentBuyerId, sort: filters.sort ?? 'default_buyer' }))
}

export async function getMyCarpoolApplicationsPage(filters: CarpoolApplicationFilters = {}, page: CursorPageRequest = {}) {
  if (shouldUseRealBackend()) return backendMyCarpoolApplicationsPage(filters, page)
  return paginateCursorItems(filterCarpoolApplications({ ...filters, buyerId: currentBuyerId, sort: filters.sort ?? 'default_buyer' }), page)
}

export async function getMerchantCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  if (shouldUseRealBackend()) return backendMerchantCarpoolApplications(filters)
  await wait()
  return clone(filterCarpoolApplications({ ...filters, ownerId: currentOwnerId, sort: filters.sort ?? 'default_owner' }))
}

export async function getMerchantCarpoolApplicationsPage(filters: CarpoolApplicationFilters = {}, page: CursorPageRequest = {}) {
  if (shouldUseRealBackend()) return backendMerchantCarpoolApplicationsPage(filters, page)
  return paginateCursorItems(filterCarpoolApplications({ ...filters, ownerId: currentOwnerId, sort: filters.sort ?? 'default_owner' }), page)
}

export async function getCarpoolApplicationById(id: string): Promise<CarpoolApplicationWithMeta | null> {
  if (shouldUseRealBackend()) return backendCarpoolApplicationById(id)
  await wait()
  return clone(carpoolApplicationStore.find(item => item.id === id) ?? null)
}

export async function getCarpoolApplicationEvents(id: string) {
  if (shouldUseRealBackend()) return backendCarpoolApplicationEvents(id)
  await wait()
  return clone(carpoolApplicationEventStore.filter(item => item.applicationId === id).sort((a, b) => compareTimeDesc(a.createdAt, b.createdAt)))
}

export async function createCarpoolApplication(carpoolId: string, payload: { rulesAccepted: boolean }) {
  if (shouldUseRealBackend()) return backendCreateCarpoolApplication(carpoolId, payload)
  await wait()
  if (!payload.rulesAccepted) throw new Error('请先确认已阅读车源规则和车主承诺说明')
  const carpool = carpoolStore.find(item => item.id === carpoolId)
  if (!carpool) throw new Error(`Carpool not found: ${carpoolId}`)
  const related = carpoolApplicationStore.filter(item => item.carpoolId === carpoolId && item.applicantUserId === currentBuyerId)
  const hasActiveMembership = related.some(item => ['active', 'pending_completion'].includes(item.status))
  const hasOngoingApplication = related.some(item => isOngoingCarpoolApplication(item.status))
  const seatSummary = getCarpoolSeatSummary(carpool)
  const eligibility = evaluateCarpoolApplicationEligibility(carpool, seatSummary, hasOngoingApplication, currentBuyerId, hasActiveMembership)
  if (!eligibility.canApply) throw new Error(eligibility.reason)

  const id = `ride-app-${Date.now()}`
  const createdAt = nowText()
  const application: CarpoolApplication = {
    id,
    carpoolId,
    applicantUserId: currentBuyerId,
    applicantUsername: currentBuyerName,
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 2, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: `owner-${carpool.owner}`,
    ownerUsername: carpool.owner,
    status: 'pending_owner',
    seatsRequested: 1,
    snapshot: buildCarpoolSnapshot(carpool),
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
    createdAt,
    updatedAt: createdAt,
  }
  carpoolApplicationStore.unshift(application)
  appendCarpoolApplicationEvent({
    applicationId: id,
    actorId: currentBuyerId,
    actorLabel: currentBuyerName,
    actorRole: 'buyer',
    type: 'application_created',
    toStatus: 'pending_owner',
    note: '买家提交上车申请，等待车主处理。',
    createdAt,
  })
  return clone(application)
}

export async function createCarpoolIntent(carpool: Carpool) {
  return createCarpoolApplication(carpool.id, { rulesAccepted: true })
}

export async function acceptCarpoolApplication(id: string) {
  if (shouldUseRealBackend()) return backendAcceptCarpoolApplication(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (application.status !== 'pending_owner') throw new Error('只有待车主处理的申请可以接受')
    const carpool = carpoolStore.find(item => item.id === application.carpoolId)
    if (!carpool) throw new Error(`Carpool not found: ${application.carpoolId}`)
    const seatSummary = getCarpoolSeatSummary(carpool)
    if (seatSummary.availableSeats < application.seatsRequested) throw new Error('可申请名额不足，无法预留席位')
    const fromStatus = application.status
    application.status = 'accepted_reserved'
    application.reservedUntil = minutesFromNow(30)
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.ownerUserId,
      actorLabel: application.ownerUsername,
      actorRole: 'owner',
      type: 'owner_accepted',
      fromStatus,
      toStatus: 'accepted_reserved',
      note: '车主接受申请，预留 1 席 30 分钟。',
    })
  })
}

export async function rejectCarpoolApplication(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendRejectCarpoolApplication(id, reason)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (application.status !== 'pending_owner') throw new Error('只有待处理申请可以拒绝')
    const fromStatus = application.status
    application.status = 'rejected'
    application.cancellationReasonCode = 'owner_rejected'
    application.cancellationReasonText = reason
    application.responsibility = 'owner'
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.ownerUserId,
      actorLabel: application.ownerUsername,
      actorRole: 'owner',
      type: 'owner_rejected',
      fromStatus,
      toStatus: 'rejected',
      note: reason,
    })
  })
}

export async function cancelCarpoolApplication(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendCancelCarpoolApplication(id, reason)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!isOngoingCarpoolApplication(application.status)) throw new Error('当前状态不能取消')
    const fromStatus = application.status
    application.status = 'cancelled_by_buyer'
    application.reservedUntil = null
    application.cancellationReasonCode = 'buyer_cancelled'
    application.cancellationReasonText = reason
    application.responsibility = fromStatus === 'pending_owner' ? 'mutual' : 'buyer'
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: 'cancelled',
      fromStatus,
      toStatus: 'cancelled_by_buyer',
      note: reason,
    })
  })
}

export async function leaveCarpoolMembership(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendBuyerLeaveCarpool(id, reason)
  return cancelCarpoolApplication(id, reason)
}

export async function markCarpoolApplicationContacted(id: string) {
  if (shouldUseRealBackend()) return backendCarpoolApplicationById(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!['accepted_reserved', 'waiting_contact'].includes(application.status)) throw new Error('当前状态不能标记已联系')
    const fromStatus = application.status
    application.status = 'contacted'
    application.buyerContactedAt = nowText()
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: 'buyer_contacted',
      fromStatus,
      toStatus: 'contacted',
      note: '买家已记录与车主完成联系。',
    })
  })
}

export async function buyerConfirmCarpoolJoined(id: string) {
  if (shouldUseRealBackend()) return backendBuyerConfirmCarpoolJoined(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!['contacted', 'joined_pending_confirmation'].includes(application.status)) throw new Error('请先记录已联系车主')
    const fromStatus = application.status
    application.buyerConfirmedJoinedAt = nowText()
    application.status = 'joined_pending_confirmation'
    startCarpoolServiceIfBothConfirmed(application)
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: 'buyer_confirmed_joined',
      fromStatus,
      toStatus: application.status,
      note: '买家确认已经上车。',
    })
  })
}

export async function ownerConfirmCarpoolJoined(id: string) {
  if (shouldUseRealBackend()) return backendOwnerConfirmCarpoolJoined(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!['contacted', 'joined_pending_confirmation'].includes(application.status)) throw new Error('当前状态不能确认上车')
    const fromStatus = application.status
    application.ownerConfirmedJoinedAt = nowText()
    application.status = 'joined_pending_confirmation'
    const started = startCarpoolServiceIfBothConfirmed(application)
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.ownerUserId,
      actorLabel: application.ownerUsername,
      actorRole: 'owner',
      type: started ? 'service_started' : 'owner_confirmed_joined',
      fromStatus,
      toStatus: application.status,
      note: started ? '双方确认后进入服务中。' : '车主确认用户已上车。',
    })
  })
}

export async function buyerConfirmCarpoolCompleted(id: string) {
  if (shouldUseRealBackend()) return backendBuyerConfirmCarpoolCompleted(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (application.status !== 'pending_completion') throw new Error('只有待完成记录可以确认完成')
    const fromStatus = application.status
    application.buyerConfirmedCompletedAt = nowText()
    const completed = completeCarpoolIfBothConfirmed(application)
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: completed ? 'completed' : 'buyer_confirmed_completed',
      fromStatus,
      toStatus: application.status,
      note: completed ? '双方确认完成。' : '买家确认本次完成。',
    })
  })
}

export async function ownerConfirmCarpoolCompleted(id: string) {
  if (shouldUseRealBackend()) return backendOwnerConfirmCarpoolCompleted(id)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (application.status !== 'pending_completion') throw new Error('只有待完成记录可以确认完成')
    const fromStatus = application.status
    application.ownerConfirmedCompletedAt = nowText()
    const completed = completeCarpoolIfBothConfirmed(application)
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.ownerUserId,
      actorLabel: application.ownerUsername,
      actorRole: 'owner',
      type: completed ? 'completed' : 'owner_confirmed_completed',
      fromStatus,
      toStatus: application.status,
      note: completed ? '双方确认完成。' : '车主确认本次完成。',
    })
  })
}

export async function disputeCarpoolApplication(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendOwnerRemoveCarpool(id, reason)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!isOngoingCarpoolApplication(application.status)) throw new Error('当前状态不能发起纠纷')
    const fromStatus = application.status
    application.status = 'disputed'
    application.disputeReason = reason
    application.responsibility = 'undetermined'
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: 'disputed',
      fromStatus,
      toStatus: 'disputed',
      note: reason,
    })
  })
}

export async function withdrawCarpoolAcceptance(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendWithdrawCarpoolAcceptance(id, reason)
  await wait()
  return updateCarpoolApplication(id, application => {
    if (!['accepted_reserved', 'waiting_contact'].includes(application.status)) throw new Error('只有已预留申请可以撤回接受')
    const fromStatus = application.status
    application.status = 'cancelled_by_owner'
    application.reservedUntil = null
    application.cancellationReasonCode = 'owner_withdrawn'
    application.cancellationReasonText = reason
    application.responsibility = 'owner'
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.ownerUserId,
      actorLabel: application.ownerUsername,
      actorRole: 'owner',
      type: 'cancelled',
      fromStatus,
      toStatus: 'cancelled_by_owner',
      note: reason,
    })
  })
}

export async function reviewCarpoolApplication(id: string, payload: ReviewCarpoolApplicationPayload) {
  await wait()
  return updateCarpoolApplication(id, application => {
    if (application.status !== 'completed') throw new Error('只有已完成记录可以评价')
    const completedAtMs = Date.parse(application.completedAt ?? application.updatedAt)
    if (!Number.isFinite(completedAtMs)) throw new Error('交易完成时间无效')
    if (Date.now() >= completedAtMs + 14 * 24 * 60 * 60 * 1000) throw new Error('评价窗口已截止')
    const currentIsBuyer = application.applicantUserId === currentBuyerId
    const currentIsOwner = application.ownerUserId === currentOwnerId
    if (!currentIsBuyer && !currentIsOwner) throw new Error('只有交易参与方可以评价')
    const review = { ...payload, createdAt: nowText() }
    if (currentIsBuyer) application.buyerReview = review
    else application.ownerReview = review
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: currentIsBuyer ? application.applicantUserId : application.ownerUserId,
      actorLabel: currentIsBuyer ? application.applicantUsername : application.ownerUsername,
      actorRole: currentIsBuyer ? 'buyer' : 'owner',
      type: 'admin_updated',
      note: `${currentIsBuyer ? '买家' : '车主'}已评价：${payload.rating} 星`,
    })
  })
}

export async function createApiPurchaseIntent(payload: CreateApiPurchaseIntentPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIPurchaseIntent(payload)
  await wait()
  const service = apiServiceStore.find(item => item.id === payload.serviceId)
  if (!service) throw new Error(`API service not found: ${payload.serviceId}`)
  requireSupportedApiServiceBillingMode(service.billingMode)
  if (!isApiServicePubliclyOrderable(service) || service.state !== 'online') throw new Error('服务当前不可创建订单。')
  if (!service.deliveryModes.includes(payload.deliveryMode)) throw new Error('选择的 API 细节不属于该服务。')
  if (service.delivery !== 'Sub2API' && payload.deliveryMode === 'sub2api_panel_account') throw new Error('当前服务不支持该 API 细节。')
  const selectedPackage = payload.selectedPackageId ? service.packages?.find(item => item.id === payload.selectedPackageId && item.enabled) : undefined
  if (service.billingMode === 'fixed_package' && (!selectedPackage || selectedPackage.stockAvailable <= 0)) throw new Error('选择的套餐已售罄或不可用。')
  if (selectedPackage && payload.purchaseAmountCny !== selectedPackage.priceCny) throw new Error('订单金额必须与所选套餐价格一致。')
  if (!selectedPackage && payload.purchaseAmountCny < service.minimumPurchaseCny) throw new Error(`最低订单金额为 ¥${service.minimumPurchaseCny}`)
  if (!selectedPackage && payload.purchaseAmountCny > service.maxBuy) throw new Error(`单笔最高订单金额为 ¥${service.maxBuy}`)
  const purchaseAmountCnyDecimal = normalizeDecimal(String(payload.purchaseAmountCny), 2)
  const cnyPerUsdAllowance = service.cnyPerUsdAllowance || divideDecimal('1', String(service.creditPerCny), 4)
  const purchasedCreditDecimal = selectedPackage ? '0' : normalizeDecimalTrimmed(divideDecimal(purchaseAmountCnyDecimal, cnyPerUsdAllowance, 6), 6)
  const availableUsdAllowance = service.availableUsdAllowance || String(service.balance)
  if (!selectedPackage && compareDecimal(purchasedCreditDecimal, availableUsdAllowance) > 0) throw new Error('超过商户当前可售美元额度。')

  const id = `api-intent-${Date.now()}`
  const createdAt = nowText()
  const snapshot: ApiPurchaseIntent['snapshot'] = {
    ...createSnapshot(service),
    selectedDeliveryMode: payload.deliveryMode,
    selectedPackageId: selectedPackage?.id,
    selectedPackageSnapshot: selectedPackage ? createPackageSnapshot(selectedPackage) : undefined,
  }
  const intent: ApiPurchaseIntent = {
    id,
    serviceId: service.id,
    buyerId: currentBuyerId,
    buyer: currentBuyerName,
    merchantId: service.merchantId,
    merchant: getApiMerchantDisplayName(service),
    status: 'open',
    selectedDeliveryMode: payload.deliveryMode,
    selectedPackageId: selectedPackage?.id,
    purchaseAmountCny: payload.purchaseAmountCny,
    purchasedCredit: Number(purchasedCreditDecimal),
    purchaseAmountCnyDecimal,
    purchasedCreditDecimal,
    quotaUsagePolicySnapshot: clone(selectedPackage?.quotaUsagePolicy ?? service.quotaUsagePolicy),
    targetModel: payload.targetModel,
    buyerNote: payload.buyerNote,
    snapshot,
    handoff: {
      intentId: id,
      selectedDeliveryMode: payload.deliveryMode,
      status: 'not_started',
      requiresFirstLoginPasswordReset: payload.deliveryMode === 'sub2api_panel_account' && service.panelRequiresPasswordReset,
      note: '购买意向已提交，商户联系方式和收款确认资料已向买家展示，商户可查看买家选择的联系方式',
    },
    contactChannels: service.contactChannels,
    buyerContactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@buyer' }],
    merchantResponseDeadline: service.online ? minutesFromNow(service.expectedResponseMinutes) : undefined,
    createdAt,
    updatedAt: createdAt,
  }
  apiPurchaseIntentStore.unshift(intent)
  appendApiIntentEvent({
    intentId: id,
    actorId: currentBuyerId,
    actorLabel: currentBuyerName,
    actorRole: 'buyer',
    type: 'intent_created',
    toStatus: 'open',
    metadata: { amount: payload.purchaseAmountCny, deliveryMode: payload.deliveryMode },
    createdAt,
  })
  return clone(intent)
}

export async function markApiPurchaseIntentContacted(id: string) {
  if (shouldUseRealBackend()) return backendMarkAPIIntentContacted(id)
  await wait()
  return updateApiPurchaseIntent(id, intent => {
    if (intent.status !== 'open') throw new Error('只有新购买意向可以记录已联系')
    const fromStatus = intent.status
    intent.status = 'contacted'
    intent.handoff.status = 'contacted'
    intent.handoff.offPlatformContactChannel = intent.contactChannels[0]?.label
    intent.handoff.note = '商户已记录已进行站外联系'
    appendApiIntentEvent({
      intentId: id,
      actorId: currentMerchantId,
      actorLabel: getApiMerchantDisplayName(intent),
      actorRole: 'merchant',
      type: 'contacted',
      fromStatus,
      toStatus: 'contacted',
      metadata: { channel: intent.handoff.offPlatformContactChannel ?? '站外渠道' },
    })
  })
}

export async function closeApiPurchaseIntent(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendCloseAPIIntent(id, reason)
  await wait()
  return updateApiPurchaseIntent(id, intent => {
    if (!['open', 'contacted'].includes(intent.status)) throw new Error('当前购买意向不能关闭')
    const fromStatus = intent.status
    intent.status = 'owner_closed'
    intent.ownerClosedAt = nowText()
    intent.ownerCloseReason = reason
    intent.handoff.status = 'closed'
    intent.handoff.note = '商户已关闭本次购买意向'
    appendApiIntentEvent({
      intentId: id,
      actorId: currentMerchantId,
      actorLabel: getApiMerchantDisplayName(intent),
      actorRole: 'merchant',
      type: 'owner_closed',
      fromStatus,
      toStatus: 'owner_closed',
      metadata: { reason },
    })
  })
}

export async function cancelApiPurchaseIntent(id: string, reason: string) {
  if (shouldUseRealBackend()) return backendCancelAPIIntentById(id, reason)
  await wait()
  return updateApiPurchaseIntent(id, intent => {
    if (!['open', 'contacted'].includes(intent.status)) throw new Error('当前购买意向不能取消')
    const fromStatus = intent.status
    intent.status = 'buyer_cancelled'
    intent.buyerCancelledAt = nowText()
    intent.buyerCancelReason = reason
    intent.handoff.status = 'closed'
    intent.handoff.note = '买家已取消本次购买意向'
    appendApiIntentEvent({
      intentId: id,
      actorId: intent.buyerId,
      actorLabel: intent.buyer,
      actorRole: 'buyer',
      type: 'buyer_cancelled',
      fromStatus,
      toStatus: 'buyer_cancelled',
      metadata: { reason },
    })
  })
}

function findApiOrder(id: string) {
  const order = apiOrderStore.find(item => item.id === id)
  if (!order) throw new Error(`API order not found: ${id}`)
  return order
}

function updateApiOrder(id: string, updater: (order: ApiOrder) => void) {
  materializeMockApiOrderReviews()
  const order = findApiOrder(id)
  updater(order)
  order.updatedAt = nowText()
  order.version += 1
  persistApiOrderStore()
  return clone(order)
}

function mockBuyerContactChannels(intent: ApiPurchaseIntent): ApiContactChannel[] {
  return intent.buyerContactChannels?.length
    ? intent.buyerContactChannels
    : [{ type: 'linuxdo', label: 'linux.do 私信', value: '@buyer' }]
}

export async function createApiOrderFromIntent(intentId: string, paymentMethod: ApiPaymentOption['paymentMethod']) {
  if (shouldUseRealBackend()) return backendCreateAPIOrderFromIntent(intentId, paymentMethod)
  await wait()
  const intent = findApiPurchaseIntent(intentId)
  const service = apiServiceStore.find(item => item.id === intent.serviceId)
  if (!service) throw new Error('API 服务不存在。')
  requireSupportedApiServiceBillingMode(service.billingMode)
  if (apiOrderStore.some(item => item.apiPurchaseIntentId === intentId)) {
    throw new Error('该购买意向已经创建过订单。')
  }
  const option = intent.snapshot.paymentOptions?.find(item => item.paymentMethod === paymentMethod && item.enabled)
  if (!option || !isApiPaymentOptionComplete(option)) {
    throw new Error('选择的收款方式不可用，请联系商户更新收款设置。')
  }
  const selectedPackage = intent.selectedPackageId
    ? service.packages?.find(item => item.id === intent.selectedPackageId && item.enabled)
    : undefined
  if (intent.selectedPackageId && (!selectedPackage || selectedPackage.stockAvailable <= 0)) {
    throw new Error('选择的套餐已售罄或不可用。')
  }
  if (selectedPackage) selectedPackage.stockAvailable -= 1
  const createdAt = nowText()
  const order: ApiOrder = {
    id: `api-order-${Date.now()}`,
    orderNo: createMockApiOrderNo(createdAt),
    purchaseKind: 'api_service',
    apiPurchaseIntentId: intent.id,
    apiServiceId: intent.serviceId,
    buyerId: intent.buyerId,
    buyer: intent.buyer,
    sellerId: intent.merchantId,
    seller: getApiMerchantDisplayName(intent),
    status: 'pending_payment',
    disputeStatus: 'none',
    serviceTitle: intent.snapshot.serviceTitle,
    amount: intent.purchaseAmountCny,
    amountDecimal: intent.purchaseAmountCnyDecimal || normalizeDecimal(String(intent.purchaseAmountCny), 2),
    currency: 'CNY',
    selectedPaymentMethod: paymentMethod,
    paymentWindowMinutes: 10,
    paymentExpiresAt: minutesFromNow(10),
    version: 1,
    intentSnapshot: clone(intent.snapshot),
    selectedDeliveryMode: intent.selectedDeliveryMode,
    selectedPackageId: selectedPackage?.id,
    packageSnapshot: selectedPackage ? createPackageSnapshot(selectedPackage) : undefined,
    packageStockReserved: Boolean(selectedPackage),
    requestedUsdAllowance: intent.purchasedCredit,
    requestedUsdAllowanceDecimal: intent.purchasedCreditDecimal || normalizeDecimalTrimmed(String(intent.purchasedCredit), 6),
    quotaUsagePolicySnapshot: clone(intent.quotaUsagePolicySnapshot),
    merchantContactChannels: clone(intent.contactChannels),
    buyerContactChannels: clone(mockBuyerContactChannels(intent)),
    viewerRole: 'buyer',
    createdAt,
    updatedAt: createdAt,
  }
  apiOrderStore.unshift(order)
  intent.status = 'ordered'
  intent.handoff.status = 'closed'
  intent.handoff.note = '购买意向已生成订单，请前往订单页继续处理。'
  intent.updatedAt = createdAt
  persistApiPurchaseStores()
  persistApiOrderStore()
  if (selectedPackage) persistMarketStores()
  return clone(order)
}

export async function createApiQuotaOrder(payload: CreateApiQuotaOrderPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIQuotaOrder(payload)
  await wait()
  const offer = apiQuotaOfferStore.find(item => item.id === payload.offerId)
  const service = offer ? apiServiceStore.find(item => item.id === offer.apiServiceId) : undefined
  if (!offer || !service) throw new Error('额度包不存在或已下架。')
  if (!offer.isOrderable || offer.availableCopies <= 0) throw new Error(offer.orderabilityReason || '当前额度包不可购买。')
  if (offer.saleMode === 'scheduled' && (!payload.saleRoundId || payload.saleRoundId !== offer.currentRound?.id)) throw new Error('当前放量轮次已变化，请刷新后重试。')
  if (payload.saleRoundId && apiOrderStore.some(order => order.buyerId === currentBuyerId && order.quotaSnapshot?.saleRoundId === payload.saleRoundId)) {
    throw new Error('同一买家每轮最多购买 1 份额度包。')
  }
  const selectedDeliveryMode = service.deliveryModes[0] ?? 'api_key_endpoint'
  const paymentMethod = getApiServiceDefaultPaymentMethod(service)
  if (!paymentMethod) throw new Error('商户尚未配置可用的微信或支付宝收款方式。')
  const credentialSummary = offer.deliveryMode === 'preimported'
    ? apiQuotaCredentialSummaryStore.find(item => item.offerId === offer.id)
    : undefined
  if (offer.deliveryMode === 'preimported' && (!credentialSummary || credentialSummary.available <= 0)) throw new Error('当前没有可用交付凭据。')

  const createdAt = nowText()
  const intentId = `api-intent-quota-${Date.now()}`
  const intentSnapshot = {
    ...createSnapshot(service),
    promptAuditEnabled: offer.promptAuditEnabled ?? null,
    multiplier: `${Number(offer.modelMultiplier).toFixed(2)}x`,
    defaultMultiplier: Number(offer.modelMultiplier),
    selectedDeliveryMode,
    expiresAt: formatQuotaExpiresAtLabel(offer.expiresAt) || offer.expiresAt,
  }
  const intent: ApiPurchaseIntent = {
    id: intentId,
    serviceId: service.id,
    buyerId: currentBuyerId,
    buyer: currentBuyerName,
    merchantId: service.merchantId,
    merchant: getApiMerchantDisplayName(service),
    status: 'ordered',
    selectedDeliveryMode,
    purchaseAmountCny: Number(offer.priceCny),
    purchasedCredit: Number(offer.usdAllowance),
    purchaseAmountCnyDecimal: offer.priceCny,
    purchasedCreditDecimal: offer.usdAllowance,
    quotaUsagePolicySnapshot: clone(offer.quotaUsagePolicy),
    targetModel: service.models[0] ?? '按额度包说明',
    snapshot: intentSnapshot,
    handoff: {
      intentId,
      selectedDeliveryMode,
      status: 'closed',
      requiresFirstLoginPasswordReset: selectedDeliveryMode === 'sub2api_panel_account' && service.panelRequiresPasswordReset,
      note: '限时额度包已直接生成订单。',
    },
    contactChannels: clone(service.contactChannels),
    buyerContactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: '@buyer' }],
    createdAt,
    updatedAt: createdAt,
  }
  const paymentWindowMinutes = offer.saleMode === 'scheduled' ? 5 : 10
  const order: ApiOrder = {
    id: `api-order-quota-${Date.now()}`,
    orderNo: createMockApiOrderNo(createdAt),
    purchaseKind: 'limited_quota_offer',
    apiPurchaseIntentId: intentId,
    apiServiceId: service.id,
    buyerId: currentBuyerId,
    buyer: currentBuyerName,
    sellerId: service.merchantId,
    seller: getApiMerchantDisplayName(service),
    status: 'pending_payment',
    disputeStatus: 'none',
    serviceTitle: service.title,
    amount: Number(offer.priceCny),
    amountDecimal: offer.priceCny,
    currency: 'CNY',
    selectedPaymentMethod: paymentMethod,
    paymentWindowMinutes,
    paymentExpiresAt: minutesFromNow(paymentWindowMinutes),
    version: 1,
    intentSnapshot: clone(intentSnapshot),
    selectedDeliveryMode,
    requestedUsdAllowance: Number(offer.usdAllowance),
    requestedUsdAllowanceDecimal: offer.usdAllowance,
    quotaUsagePolicySnapshot: clone(offer.quotaUsagePolicy),
    quotaSnapshot: {
      batchId: offer.batchId,
      offerId: offer.id,
      saleRoundId: payload.saleRoundId,
      offerName: offer.name,
      usdAllowance: offer.usdAllowance,
      priceCny: offer.priceCny,
      cnyPerUsd: offer.cnyPerUsd,
      modelMultiplier: offer.modelMultiplier,
      saleCutoffAt: offer.saleCutoffAt,
      expiresAt: offer.expiresAt,
      saleMode: offer.saleMode,
      roundStartsAt: offer.currentRound?.startsAt,
      roundEndsAt: offer.currentRound?.endsAt,
      distributionSystem: offer.distributionSystem,
      ttftBand: service.declaredTtftBand ?? '1_to_3s',
      declaredMaxConcurrency: offer.declaredMaxConcurrency,
      promptAuditEnabled: offer.promptAuditEnabled ?? null,
      accountPoolType: service.accountPoolType,
      accountPoolLabel: service.accountPoolLabel,
      merchantRefundCommitment: service.merchantRefundCommitment,
      merchantRefundPolicyVersion: service.merchantRefundPolicyVersion,
      serviceValidityExpiresAt: offer.expiresAt,
      performanceConfirmedAt: service.performanceConfirmedAt,
      performanceUnverified: true,
      deliveryEtaMinutes: offer.deliveryEtaMinutes,
      deliveryMode: offer.deliveryMode,
    },
    merchantContactChannels: clone(service.contactChannels),
    buyerContactChannels: clone(mockBuyerContactChannels(intent)),
    viewerRole: 'buyer',
    createdAt,
    updatedAt: createdAt,
  }

  offer.availableCopies -= 1
  if (credentialSummary) {
    credentialSummary.available -= 1
    credentialSummary.reserved += 1
    offer.credentialAvailableCopies = credentialSummary.available
  }
  if (offer.availableCopies === 0) {
    offer.isOrderable = false
    offer.orderabilityCode = 'sold_out'
    offer.orderabilityReason = '当前轮次已售罄。'
  }
  apiPurchaseIntentStore.unshift(intent)
  apiOrderStore.unshift(order)
  appendApiIntentEvent({
    intentId,
    actorId: currentBuyerId,
    actorLabel: currentBuyerName,
    actorRole: 'buyer',
    type: 'intent_created',
    toStatus: 'ordered',
    metadata: { offerId: offer.id, amount: Number(offer.priceCny) },
    createdAt,
  })
  persistApiPurchaseStores()
  persistApiOrderStore()
  persistApiQuotaStores()
  return clone(order)
}

export async function getMyApiOrders(filters: ApiOrderFilters = {}) {
  if (shouldUseRealBackend()) return backendMyAPIOrders(filters)
  await wait()
  return clone(filterApiOrders({ ...filters, buyerId: currentBuyerId }))
}

export async function getMyApiOrdersPage(filters: ApiOrderFilters = {}, page: CursorPageRequest = {}) {
  if (shouldUseRealBackend()) return backendMyAPIOrdersPage(filters, page)
  return paginateCursorItems(filterApiOrders({ ...filters, buyerId: currentBuyerId }), page)
}

export async function getMerchantApiOrders(filters: ApiOrderFilters = {}) {
  if (shouldUseRealBackend()) return backendOwnerAPIOrders(filters)
  await wait()
  return clone(filterApiOrders({ ...filters, sellerId: currentMerchantId }))
}

export async function getMerchantApiOrdersPage(filters: ApiOrderFilters = {}, page: CursorPageRequest = {}) {
  if (shouldUseRealBackend()) return backendOwnerAPIOrdersPage(filters, page)
  return paginateCursorItems(filterApiOrders({ ...filters, sellerId: currentMerchantId }), page)
}

export async function getApiOrderById(id: string, perspective: 'buyer' | 'merchant' = 'buyer') {
  if (shouldUseRealBackend()) {
    return perspective === 'merchant' ? backendOwnerAPIOrder(id) : backendMyAPIOrder(id)
  }
  await wait()
  materializeMockApiOrderReviews()
  const order = findApiOrder(id)
  if (perspective === 'merchant' && order.sellerId !== currentMerchantId) throw new Error('无权查看该订单。')
  if (perspective === 'buyer' && order.buyerId !== currentBuyerId) throw new Error('无权查看该订单。')
  return clone({ ...order, viewerRole: perspective })
}

export async function readApiOrderPaymentInstructions(id: string) {
  if (shouldUseRealBackend()) return backendReadAPIOrderPaymentInstructions(id)
  await wait()
  const order = findApiOrder(id)
  const option = order.intentSnapshot.paymentOptions?.find(item => item.paymentMethod === order.selectedPaymentMethod)
  return clone({
    orderId: order.id,
    paymentMethod: order.selectedPaymentMethod,
    paymentInstructions: option?.paymentInstructions ?? '',
    paymentQrCodeDataUrl: option?.paymentQrCodeDataUrl ?? null,
    paymentExpiresAt: order.paymentExpiresAt,
  } satisfies ApiOrderPaymentInstructions)
}

export async function submitApiOrderPayment(id: string, paymentSummary: string, version: number) {
  if (shouldUseRealBackend()) return backendSubmitAPIOrderPayment(id, paymentSummary, version)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'pending_payment' && order.status !== 'payment_issue') throw new Error('当前订单不能重新提交付款信息。')
    order.status = 'payment_submitted'
    order.paymentSummary = paymentSummary.trim()
    order.paymentSubmittedAt = nowText()
    order.paymentIssueReason = undefined
    order.paymentIssueNote = undefined
    order.paymentIssueReportedAt = undefined
  })
}

export async function cancelApiOrder(id: string, reason: string, version: number) {
  if (shouldUseRealBackend()) return backendCancelAPIOrder(id, reason, version)
  await wait()
  const updated = updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'pending_payment') throw new Error('只有尚未付款的订单可以取消。')
    const trimmedReason = reason.trim()
    if (!trimmedReason) throw new Error('请选择取消原因。')
    order.status = 'cancelled'
    order.cancelReason = trimmedReason
    order.cancelledAt = nowText()
    if (order.packageStockReserved && order.selectedPackageId) {
      const service = apiServiceStore.find(item => item.id === order.apiServiceId)
      const selectedPackage = service?.packages?.find(item => item.id === order.selectedPackageId)
      if (selectedPackage) selectedPackage.stockAvailable = Math.min(selectedPackage.stockTotal, selectedPackage.stockAvailable + 1)
      order.packageStockReserved = false
      persistMarketStores()
    }
  })
  if (updated.purchaseKind === 'limited_quota_offer' && updated.quotaSnapshot) {
    const offer = apiQuotaOfferStore.find(item => item.id === updated.quotaSnapshot!.offerId)
    if (offer && Date.now() < Date.parse(offer.saleCutoffAt)) {
      offer.availableCopies += 1
      offer.isOrderable = offer.batchStatus === 'published' && offer.status === 'published'
      offer.orderabilityCode = offer.isOrderable ? 'orderable' : offer.orderabilityCode
      offer.orderabilityReason = offer.isOrderable ? '当前可购买。' : offer.orderabilityReason
    }
    const summary = apiQuotaCredentialSummaryStore.find(item => item.offerId === updated.quotaSnapshot!.offerId)
    if (summary && summary.reserved > 0) {
      summary.reserved -= 1
      summary.available += 1
      if (offer) offer.credentialAvailableCopies = summary.available
    }
    persistApiQuotaStores()
  }
  return updated
}

export async function confirmApiOrderComplete(id: string, version: number) {
  if (shouldUseRealBackend()) return backendConfirmAPIOrderComplete(id, version)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'delivery_submitted') throw new Error('只有待核验凭证的订单可以确认可用。')
		if (isApiOrderDisputeActive(order.disputeStatus)) throw new Error('订单问题正在处理中，暂时不能确认凭证可用。')
    order.status = 'completed'
    order.completionSource = 'buyer_confirmed'
    order.completedAt = nowText()
  })
}

export async function openApiOrderDispute(id: string, input: OpenApiOrderDisputeInput, version: number, perspective: 'buyer' | 'merchant') {
  if (shouldUseRealBackend()) return backendOpenAPIOrderDispute(id, input, version, perspective)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (perspective === 'buyer' && order.buyerId !== currentBuyerId) throw new Error('无权操作该订单。')
    if (perspective === 'merchant' && order.sellerId !== currentMerchantId) throw new Error('无权操作该订单。')
		if (order.status === 'cancelled' || order.status === 'completed' || normalizeApiOrderDisputeStatus(order.disputeStatus) !== 'none') {
      throw new Error('当前订单不能再次申请平台介入。')
    }
    if (!input.reason.trim()) throw new Error('请填写订单问题说明。')
    order.disputeStatus = 'negotiating'
  })
}

export async function confirmApiOrderPayment(id: string, version: number) {
  if (shouldUseRealBackend()) return backendConfirmAPIOrderPayment(id, version)
  await wait()
  let shouldPersistQuota = false
  const updated = updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'payment_submitted') throw new Error('只有买家已付款订单可以确认收款。')
    const confirmedAt = nowText()
    order.paidConfirmedAt = confirmedAt
    order.packageStockReserved = false
    if (order.purchaseKind !== 'limited_quota_offer' || order.quotaSnapshot?.deliveryMode !== 'preimported') {
      order.status = 'paid_confirmed'
      return
    }

    const summary = apiQuotaCredentialSummaryStore.find(item => item.offerId === order.quotaSnapshot!.offerId)
    if (!summary || summary.reserved <= 0) throw new Error('预留交付凭据不存在，请刷新后重试。')
    summary.reserved -= 1
    summary.delivered += 1
    shouldPersistQuota = true
    order.status = 'delivery_submitted'
    order.deliverySubmittedAt = confirmedAt
    order.deliveryReviewExpiresAt = new Date(Date.parse(confirmedAt) + apiOrderDeliveryReviewWindowMs).toISOString()
    order.deliveryNote = '确认收款后已分配预导入的买家专属接入信息。'
    order.deliveryCredential = order.selectedDeliveryMode === 'sub2api_panel_account'
      ? {
          deliveryKind: 'login_account',
          panelLoginUrl: 'https://mock-panel.example.test/login',
          username: `buyer-${order.id}`,
          password: `mock-${order.id}`,
          instructions: '演示环境自动分配的买家专属凭证。',
          submittedAt: confirmedAt,
        }
      : {
          deliveryKind: 'api_key_endpoint',
          apiBaseUrl: 'https://mock-api.example.test/v1',
          apiKey: `mock-${order.id}`,
          instructions: '演示环境自动分配的买家专属凭证。',
          submittedAt: confirmedAt,
        }
  })
  if (shouldPersistQuota) persistApiQuotaStores()
  return updated
}

export async function reportApiOrderPaymentIssue(id: string, reason: ApiOrderPaymentIssueReason, note: string, version: number) {
  if (shouldUseRealBackend()) return backendReportAPIOrderPaymentIssue(id, reason, note, version)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'payment_submitted') throw new Error('只有待核对收款的订单可以报告付款问题。')
    order.status = 'payment_issue'
    order.paymentIssueReason = reason
    order.paymentIssueNote = note.trim() || undefined
    order.paymentIssueReportedAt = nowText()
  })
}

function validateMockDeliveryCredential(payload: SubmitApiOrderDeliveryCredentialPayload) {
  if (payload.deliveryKind === 'api_key_endpoint') {
    if (!payload.apiBaseUrl?.trim()) throw new Error('请填写 API Base URL。')
    if (!payload.apiKey?.trim()) throw new Error('请填写买家专属 API Key。')
    return
  }
  if (payload.deliveryKind === 'login_account') {
    if (!payload.panelLoginUrl?.trim()) throw new Error('请填写登录地址。')
    if (!payload.username?.trim()) throw new Error('请填写用户名。')
    if (!payload.password?.trim()) throw new Error('请填写初始密码。')
    return
  }
  throw new Error('请选择交付凭证类型。')
}

export async function submitApiOrderDeliveryCredential(id: string, payload: SubmitApiOrderDeliveryCredentialPayload, version: number) {
  if (shouldUseRealBackend()) return backendSubmitAPIOrderDeliveryCredential(id, payload, version)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'paid_confirmed') throw new Error('只有确认收款后的订单可以交付。')
    if (order.deliveryCredential) throw new Error('交付信息已提交，不能再次修改。')
    validateMockDeliveryCredential(payload)
    const submittedAt = nowText()
    order.status = 'delivery_submitted'
    order.deliverySubmittedAt = submittedAt
    order.deliveryReviewExpiresAt = new Date(Date.parse(submittedAt) + apiOrderDeliveryReviewWindowMs).toISOString()
    if (order.packageSnapshot) {
      const expiresAt = new Date(new Date(submittedAt).getTime() + order.packageSnapshot.durationDays * 86_400_000)
      order.packageExpiresAt = expiresAt.toISOString()
    }
    order.deliveryNote = payload.deliveryKind === 'login_account'
      ? '商户已提交登录账号接入信息。'
      : '商户已提交 API Key 接入信息。'
    order.deliveryCredential = {
      deliveryKind: payload.deliveryKind,
      apiBaseUrl: payload.apiBaseUrl?.trim() || undefined,
      apiKey: payload.apiKey?.trim() || undefined,
      panelLoginUrl: payload.panelLoginUrl?.trim() || undefined,
      username: payload.username?.trim() || undefined,
      password: payload.password?.trim() || undefined,
      instructions: payload.instructions?.trim() || undefined,
      submittedAt,
    }
  })
}

export function getApiOrderEvents(order: ApiOrder): ApiOrderEvent[] {
  const events: ApiOrderEvent[] = [{
    id: `${order.id}-created`,
    orderId: order.id,
    actorLabel: order.buyer,
    actorRole: 'buyer',
    type: 'created',
    toStatus: 'pending_payment',
    createdAt: order.createdAt,
  }]
  if (order.paymentSubmittedAt) {
    events.push({
      id: `${order.id}-payment-submitted`,
      orderId: order.id,
      actorLabel: order.buyer,
      actorRole: 'buyer',
      type: 'payment_submitted',
      fromStatus: 'pending_payment',
      toStatus: 'payment_submitted',
      note: order.paymentSummary,
      createdAt: order.paymentSubmittedAt,
    })
  }
  if (order.paymentIssueReportedAt && order.paymentIssueReason) {
    events.push({
      id: `${order.id}-payment-issue-${order.paymentIssueReportedAt}`,
      orderId: order.id,
      actorLabel: order.seller,
      actorRole: 'merchant',
      type: 'payment_issue_reported',
      fromStatus: 'payment_submitted',
      toStatus: 'payment_issue',
      note: `${getApiOrderPaymentIssueLabel(order.paymentIssueReason)}${order.paymentIssueNote ? `：${order.paymentIssueNote}` : ''}`,
      createdAt: order.paymentIssueReportedAt,
    })
  }
  if (order.paidConfirmedAt) {
    events.push({
      id: `${order.id}-payment-confirmed`,
      orderId: order.id,
      actorLabel: order.seller,
      actorRole: 'merchant',
      type: 'payment_confirmed',
      fromStatus: 'payment_submitted',
      toStatus: 'paid_confirmed',
      createdAt: order.paidConfirmedAt,
    })
  }
  if (order.deliverySubmittedAt) {
    events.push({
      id: `${order.id}-delivery-submitted`,
      orderId: order.id,
      actorLabel: order.seller,
      actorRole: 'merchant',
      type: 'delivery_submitted',
      fromStatus: 'paid_confirmed',
      toStatus: 'delivery_submitted',
      note: order.deliveryNote,
      createdAt: order.deliverySubmittedAt,
    })
  }
  if (order.completedAt) {
    const automaticallyCompleted = order.completionSource === 'auto_completed'
    events.push({
      id: `${order.id}-completed`,
      orderId: order.id,
      actorLabel: automaticallyCompleted ? '系统' : order.buyer,
      actorRole: automaticallyCompleted ? 'system' : 'buyer',
      type: 'completed',
      fromStatus: 'delivery_submitted',
      toStatus: 'completed',
      note: automaticallyCompleted ? '24 小时核验期结束，订单自动完成。' : '买家已确认凭证可用。',
      createdAt: order.completedAt,
    })
  }
  if (order.cancelledAt) {
    events.push({
      id: `${order.id}-cancelled`,
      orderId: order.id,
      actorLabel: '系统',
      actorRole: 'system',
      type: 'cancelled',
      fromStatus: order.status,
      toStatus: 'cancelled',
      note: order.cancelReason,
      createdAt: order.cancelledAt,
    })
  }
  return events.sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime())
}

export async function getApiOrderNotifications(): Promise<ApiOrderNotification[]> {
  await wait()
  materializeMockApiOrderReviews()
  const rows = apiOrderStore
    .filter(item => ['pending_payment', 'payment_issue', 'payment_submitted', 'paid_confirmed', 'delivery_submitted'].includes(item.status))
    .slice(0, 6)
    .map(item => ({
      id: `api-notice-${item.id}`,
      title: getApiOrderStatusLabel(item.status),
      detail: `${item.orderNo} · ${item.serviceTitle} · ${item.buyer} / ${item.seller}`,
      time: item.updatedAt,
      unread: item.status === 'payment_issue' || item.status === 'payment_submitted' || item.status === 'paid_confirmed',
      to: item.sellerId === currentMerchantId ? `/merchant/api-orders/${item.id}` : `/my/api-orders/${item.id}`,
    }))
  return clone(markReadState(rows))
}

export async function getCarpoolNotifications(): Promise<CarpoolNotification[]> {
  await wait()
  const rows = carpoolApplicationStore
    .filter(item => [currentBuyerId, currentOwnerId].includes(item.applicantUserId) || [currentBuyerId, currentOwnerId].includes(item.ownerUserId))
    .filter(item => ['pending_owner', 'accepted_reserved', 'contacted', 'joined_pending_confirmation', 'pending_completion', 'disputed', 'rejected'].includes(item.status))
    .slice(0, 8)
    .map(item => {
      const isOwner = item.ownerUserId === currentOwnerId
      return {
        id: `carpool-notice-${item.id}`,
        title: getCarpoolApplicationStatusLabel(item.status),
        detail: `${item.snapshot.productName} · ${item.applicantUsername} / ${item.ownerUsername}`,
        time: item.updatedAt,
        unread: item.status !== 'rejected',
        to: isOwner ? `/merchant/carpool-applications/${item.id}` : `/my/rides/${item.id}`,
      }
    })
  return clone(markReadState(rows))
}

export async function updateAdminRowStatus(row: AdminRow, status: string, reason = '管理台本地 mock 操作') {
  if (shouldUseRealBackend() && (row.targetType === 'api-service' || row.targetType === 'api-merchant')) {
    return backendUpdateAdminAPIServiceStatus(row, status, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'carpool') {
    return backendUpdateAdminCarpoolStatus(row, status, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'official-price') {
    return backendUpdateOfficialPriceAdminStatus(row, status, reason)
  }
  if (shouldUseRealBackend() && (row.targetType === 'report' || row.targetType === 'dispute' || row.targetType === 'appeal')) {
    return backendUpdateReportAdminStatus(row, status, reason)
  }
  await wait()
  if ((status === '已通过' && ['已通过', '已验证', '在线', '可上车', '匹配中'].some(value => row.status.includes(value)))
    || (status === '待复核' && row.status.includes('复核'))) {
    throw new Error('当前状态已经匹配该操作，不能重复写入审计记录。')
  }
  await applyAdminStatusToTarget(row, status)
  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '管理员',
    action: status,
    targetType: row.targetType ?? 'admin-row',
    targetId: row.id,
    targetLabel: row.primary,
    beforeStatus: row.status,
    afterStatus: status,
    reason,
  })
  return { ...row, status }
}

async function applyAdminStatusToTarget(row: AdminRow, status: string) {
  if (row.targetType === 'official-price') {
    const target = officialPriceStore.find(item => item.id === row.id)
    const nextStatus = status === '已通过' || status === '已恢复'
      ? '已验证'
      : status === '已下架' || status === '已限制'
        ? '已过期'
        : status
    if (target && ['已验证', '待验证', '需复核', '有争议', '已过期'].includes(nextStatus)) {
      target.status = nextStatus as OfficialPrice['status']
      target.updatedAt = nowText()
      persistMarketStores()
    }
  }
  if (row.targetType === 'carpool') {
    const target = carpoolStore.find(item => item.id === row.id)
    if (target) {
      target.status = status === '已通过' || status === '已恢复' ? '可上车' : status === '待复核' ? '审核中' : status === '已下架' ? '暂停' : target.status
      target.confirmedAt = nowText()
      persistMarketStores()
    }
  }
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') {
    const target = apiServiceStore.find(item => item.id === row.id)
    if (target) {
      if (status === '已通过' || status === '已恢复') {
        target.state = 'online'
        target.online = true
        target.warning = undefined
      }
      if (status === '待复核') {
        target.state = 'reviewing'
        target.online = false
        target.warning = '等待管理员复核'
      }
      if (status === '已下架' || status === '已限制') {
        target.state = 'paused'
        target.online = false
        target.warning = status
      }
      target.lastOnlineConfirmedAt = nowText()
      persistMarketStores()
    }
  }
}

export async function runAdminModerationAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string, requestedFromUserId = '') {
  if (shouldUseRealBackend() && (row.targetType === 'api-service' || row.targetType === 'api-merchant')) {
    return backendRunAdminAPIServiceAction(row, action, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'carpool') {
    return backendRunAdminCarpoolAction(row, action, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'official-price') {
    return backendRunOfficialPriceAdminAction(row, action, reason)
  }
  if (shouldUseRealBackend() && (row.targetType === 'report' || row.targetType === 'dispute' || row.targetType === 'appeal')) {
    return backendRunReportAdminAction(row, action, reason, requestedFromUserId)
  }
  await wait()
  if (['take_down', 'restore', 'restrict', 'warn', 'suspend', 'ban'].includes(action) && !reason.trim()) {
    throw new Error('请填写明确的操作原因。')
  }
  const restorableStatuses = ['已下架', '已限制', '暂停', '离线', '临时封禁', '永久封禁', '申诉复核中', '需要补充信息', 'partially_restricted', 'temporarily_suspended', 'permanently_banned', 'under_review']
  if (action === 'restore' && !restorableStatuses.some(status => row.status.includes(status))) {
    throw new Error('当前状态不需要恢复，不能执行恢复操作。')
  }
  const downableStatuses = ['已验证', '已通过', '可上车', '已满', '在线', '匹配中', 'normal']
  if (action === 'take_down' && !downableStatuses.some(status => row.status.includes(status))) {
    throw new Error('当前状态不适合下架，请先复核。')
  }
  const labels: Record<typeof action, string> = {
    approve: '已通过',
    request_changes: '待复核',
    take_down: '已下架',
    restore: '已恢复',
    restrict: '已限制',
    warn: '已警告',
    suspend: '临时封禁',
    ban: '永久封禁',
  }
  const nextStatus = labels[action]
  await applyAdminStatusToTarget(row, nextStatus)

  appendAdminAuditLog({
    actorType: 'admin',
    actorLabel: '管理员',
    action: nextStatus,
    targetType: row.targetType ?? 'admin-row',
    targetId: row.id,
    targetLabel: row.primary,
    beforeStatus: row.status,
    afterStatus: nextStatus,
    reason,
  })
  return { ...row, status: nextStatus, risk: reason || row.risk }
}

export type {
  ApiBillingMode,
  ApiDeliveryMode,
  ApiPurchaseIntent,
  ApiPurchaseIntentEvent,
  ApiPurchaseIntentEventType,
  ApiPurchaseIntentStatus,
  ApiQuotaBatch,
  ApiQuotaCredentialSummary,
  ApiQuotaDeliveryMode,
  ApiQuotaDistributionSystem,
  ApiQuotaOffer,
  ApiQuotaRound,
  ApiQuotaSaleMode,
  ApiQuotaSourceType,
  ApiQuotaSystemSaleSlot,
  ApiQuotaSystemSaleSlotList,
  ApiService,
  ApiServiceSalesChannel,
  ApiServiceSalesChannelKind,
  ApiServiceSalesState,
  ApiServiceSalesSummary,
  ApiServiceSalesView,
  ApiServicePackage,
  ApiServicePackageModel,
  ApiServicePackageSnapshot,
  ApiServiceState,
  ApiTTFTBand,
  ApiUsageVisibility,
  AvatarMode,
  Carpool,
  CarpoolApplication,
  CarpoolApplicationEvent,
  CarpoolApplicationEventType,
  CarpoolApplicationReview,
  CarpoolApplicationStatus,
  CarpoolCancellationResponsibility,
  CarpoolSeatSummary,
  ContactMethodType,
  ContactUsageScope,
  CreateContactReportRequest,
  CreateManualInterventionReportRequest,
  ModelPriceRow,
  OfficialPrice,
  OwnerApiService,
  OrderContactSnapshot,
  OrderContactSnapshotItem,
  PublicMerchantProfile,
  PublicUserProfile,
  ProductTrend,
  PublicApiQuotaOffer,
  TransactionRecord,
  TransactionTrendPoint,
  UserContactMethod,
  UserPrivacySettings,
  UserProfile,
}
