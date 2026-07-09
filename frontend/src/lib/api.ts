import {
  adminCards,
  adminAuditLogs,
  adminUserRiskProfiles,
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
  parsedLinuxDoTopicMock,
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
  type ApiService,
  type ApiServiceState,
  type ApiUsageVisibility,
  type AdminAuditLog,
  type AdminUserRiskProfile,
  type Carpool,
  type CarpoolApplication,
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
  type ParsedLinuxDoTopic,
  type PaymentMethodOption,
  type RegionOption,
  type ModelCatalogItem,
  type ModelPriceRow,
  type OfficialPrice,
  type PublicReviewRecord,
  type PublicMerchantProfile,
  type PublicUserProfile,
  type ProductTrend,
  type TransactionRecord,
  type TransactionTrendPoint,
  type UserContactMethod,
  type UserPrivacySettings,
  type UserProfile,
} from '@/data/mock'
import { getPricingDisplay } from '@/lib/pricing'
import { defaultQuotaLabel, defaultQuotaPeriod, defaultQuotaUnit } from '@/lib/quota'
import { beijingDateTimeInputToISOString, formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'
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
  backendBuyerLeaveCarpool,
  backendCancelCarpoolApplication,
  backendBuyerConfirmCarpoolCompleted,
  backendBuyerConfirmCarpoolJoined,
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
  backendOwnerCarpools,
  backendMerchantCarpoolApplications,
  backendMyCarpoolApplications,
  backendOwnerRemoveCarpool,
  backendOwnerConfirmCarpoolCompleted,
  backendOwnerConfirmCarpoolJoined,
  backendRejectCarpoolApplication,
  backendRunAdminCarpoolAction,
  backendSubmitCarpool,
  backendUpdateAdminCarpoolStatus,
  backendWithdrawCarpoolAcceptance,
} from '@/lib/carpoolBackend'
import {
  backendAPIIntentById,
  backendAPIIntentEvents,
  backendAPIServiceById,
  backendAPIServices,
  backendAdminAPIServiceRows,
  backendConfirmAPIOrderPayment,
  backendCreateAPIOrderFromIntent,
  backendCancelAPIIntentById,
  backendCloseAPIIntent,
  backendCreateAPIPurchaseIntent,
  backendMarkAPIIntentContacted,
  backendModelCatalog,
  backendMyAPIOrder,
  backendMyAPIOrders,
  backendMyAPIIntents,
  backendOtherAPIServices,
  backendOwnerAPIOrder,
  backendOwnerAPIOrders,
  backendOwnerAPIIntents,
  backendOwnerAPIServices,
  backendPauseAPIService,
  backendPublishAPIService,
  backendReadAPIOrderPaymentInstructions,
  backendRunAdminAPIServiceAction,
  backendResumeAPIService,
  backendSubmitAPIOrderDeliveryCredential,
  backendSubmitAPIOrderPayment,
  backendSub2APIServices,
  backendSubmitAPIService,
  backendUpdateAdminAPIServiceStatus,
} from '@/lib/apiMarketBackend'
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
import { backendReviewCenterRows, backendSubmitReview } from '@/lib/reviewBackend'
import { backendSearchMarket } from '@/lib/searchBackend'
import {
  backendAdminOfficialPriceRows,
  backendMyOfficialPriceLeads,
  backendOfficialPriceById,
  backendOfficialPrices,
  backendRunOfficialPriceAdminAction,
  backendSubmitOfficialPriceLead,
  backendUpdateOfficialPriceAdminStatus,
} from '@/lib/officialPriceBackend'
import {
  backendAdminDemandRows,
  backendRunAdminDemandAction,
  backendUpdateAdminDemandStatus,
} from '@/lib/demandBackend'
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
import { shouldUseRealBackend } from '@/lib/backendClient'
import { getBackupPasswordValidationMessage } from '@/lib/passwordPolicy'
import {
  closeDemand,
  getDemandById,
  getDemands,
  submitDemand,
} from '@/features/demand/api'

export {
  closeDemand,
  getDemandById,
  getDemands,
  submitDemand,
}
export type { DemandRecord, DemandStatus, SubmitDemandPayload } from '@/features/demand/api'
export type AdminSection =
  | 'official-prices'
  | 'price-leads'
  | 'carpools'
  | 'demands'
  | 'api-merchants'
  | 'api-services'
  | 'trade-intents'
  | 'carpool-applications'
  | 'certifications'
  | 'users'
  | 'restrictions'
  | 'feedback'
  | 'reports'
  | 'appeals'
  | 'audit-logs'
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
  type: '审核结果' | '上车申请' | 'API 意向' | '求车需求' | '问题反馈' | '管理操作' | '边界提醒'
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
  type: '官方价格' | '车源' | '求车' | 'API 服务' | '用户' | '商户'
  title: string
  subtitle: string
  badge: string
  to: string
}

export type ReviewCenterRow = {
  id: string
  sourceType: 'carpool'
  sourceId: string
  target: string
  counterparty: string
  status: '可评价' | '已评价'
  rating: number
  tags: string[]
  note: string
  createdAt: string
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
  sourceType: 'carpool'
  sourceId: string
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
  | 'paid_confirmed'
  | 'delivery_submitted'
  | 'completed'
  | 'cancelled'

export type ApiOrderDeliveryKind = 'api_key_endpoint' | 'login_account'

export type ApiOrderDeliveryCredential = {
  deliveryKind: ApiOrderDeliveryKind
  apiBaseUrl?: string
  apiKey?: string
  panelLoginUrl?: string
  username?: string
  password?: string
  instructions?: string
  submittedAt: string
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
  apiPurchaseIntentId: string
  apiServiceId: string
  buyerId: string
  buyer: string
  sellerId: string
  seller: string
  status: ApiOrderStatus
  disputeStatus?: string
  serviceTitle: string
  amount: number
  currency: 'CNY'
  selectedPaymentMethod: ApiPaymentOption['paymentMethod']
  paymentWindowMinutes: number
  paymentExpiresAt: string
  paymentSummary?: string
  paymentSubmittedAt?: string
  paidConfirmedAt?: string
  deliveryNote?: string
  deliverySubmittedAt?: string
  deliveryCredential?: ApiOrderDeliveryCredential
  completedAt?: string
  cancelledAt?: string
  cancelReason?: string
  version: number
  intentSnapshot: ApiPurchaseIntent['snapshot']
  selectedDeliveryMode: ApiDeliveryMode
  requestedUsdAllowance: number
  merchantContactChannels: ApiContactChannel[]
  buyerContactChannels: ApiContactChannel[]
  viewerRole?: 'buyer' | 'merchant'
  createdAt: string
  updatedAt: string
}

export type ApiOrderEvent = {
  id: string
  orderId: string
  actorLabel: string
  actorRole: 'buyer' | 'merchant' | 'system'
  type: 'created' | 'payment_submitted' | 'payment_confirmed' | 'delivery_submitted' | 'completed' | 'cancelled'
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
export type {
  CarpoolProductCatalogItem,
  OpeningChannelOption,
  ParsedLinuxDoTopic,
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
  linuxDoTopicUrl: string
  parsedTopicId: string | null
  productId: string
  customProductName: string | null
  regionCode: string
  customRegionName: string | null
  monthlyPriceCny: number | null
  serviceMultiplier: number | null
  monthlyQuotaAmount: number | null
  totalSeats: number
  occupiedSeats: number
  openingChannelCode: string
  paymentMethodCodes: string[]
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

const wait = () => new Promise(resolve => window.setTimeout(resolve, 80))
const currentBuyerId = 'buyer-demo-user'
const currentBuyerName = 'demo_user'
const currentOwnerId = 'owner-orbit'
const currentOwnerName = 'orbit'
const currentMerchantId = 'merchant-orbit'
const currentMerchantName = 'orbit'
const apiPurchaseIntentStorageKey = 'c2cmarket.apiPurchaseIntents.v2'
const apiPurchaseIntentEventStorageKey = 'c2cmarket.apiPurchaseIntentEvents.v2'
const apiOrderStorageKey = 'c2cmarket.apiOrders.v1'
const carpoolApplicationStorageKey = 'c2cmarket.carpoolApplications.v1'
const carpoolApplicationEventStorageKey = 'c2cmarket.carpoolApplicationEvents.v1'
const adminAuditLogStorageKey = 'c2cmarket.adminAuditLogs.v1'
const adminUserRiskProfileStorageKey = 'c2cmarket.adminUserRiskProfiles.v1'
const officialPriceStorageKey = 'c2cmarket.officialPrices.v1'
const carpoolStorageKey = 'c2cmarket.carpools.v1'
const apiServiceStorageKey = 'c2cmarket.apiServices.v1'
const apiServicePaymentSnapshotStorageKey = 'c2cmarket.apiServicePaymentSnapshots.v1'
const apiPaymentAccountSettingsStorageKey = 'c2cmarket.apiPaymentAccountSettings.v1'
const feedbackStorageKey = 'c2cmarket.feedbackTickets.v1'
const notificationReadStorageKey = 'c2cmarket.notificationReadState.v1'
const favoriteStorageKey = 'c2cmarket.favorites.v1'
const sub2ApiFixedMultiplier = 1
const carpoolApplyAllowedStatuses: Carpool['status'][] = ['可上车']
const carpoolContactVisibleStatuses: CarpoolApplicationStatus[] = ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation', 'active', 'pending_completion', 'completed', 'disputed']
const apiContactVisibleStatuses: ApiPurchaseIntentStatus[] = ['open', 'contacted', 'buyer_cancelled', 'owner_closed']

let apiPurchaseIntentStore = normalizeApiPurchaseIntentStore(readSessionStore(apiPurchaseIntentStorageKey, apiPurchaseIntents))
let apiPurchaseIntentEventStore = normalizeApiPurchaseIntentEventStore(readSessionStore(apiPurchaseIntentEventStorageKey, apiPurchaseIntentEvents))
let apiOrderStore = readSessionStore<ApiOrder[]>(apiOrderStorageKey, [])
let carpoolApplicationStore = readSessionStore(carpoolApplicationStorageKey, carpoolApplications)
let carpoolApplicationEventStore = readSessionStore(carpoolApplicationEventStorageKey, carpoolApplicationEvents)
let adminAuditLogStore = readSessionStore(adminAuditLogStorageKey, adminAuditLogs)
let adminUserRiskProfileStore = readSessionStore(adminUserRiskProfileStorageKey, adminUserRiskProfiles)
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

function isLinuxDoTopicUrl(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' && parsed.hostname === 'linux.do' && parsed.pathname.startsWith('/t/')
  } catch {
    return false
  }
}

function readSessionStore<T>(key: string, seed: T): T {
  const stored = window.sessionStorage.getItem(key)
  if (!stored) return clone(seed)
  const parsed = JSON.parse(stored) as T
  if (isIdRecordArray(seed) && isIdRecordArray(parsed)) {
    return mergeSeedRecords(seed, parsed) as T
  }
  return parsed
}

function readLocalStore<T>(key: string, seed: T): T {
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
  }))
}

function normalizeApiPurchaseIntentEventStore(events: ApiPurchaseIntentEvent[]): ApiPurchaseIntentEvent[] {
  return events.map(event => ({
    ...event,
    fromStatus: event.fromStatus,
    toStatus: event.toStatus,
  }))
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

function applyMultiplierToModelPriceRows(rows: ApiService['modelPriceRows'], multiplier: number): ApiService['modelPriceRows'] {
  return rows.map(row => ({
    ...row,
    merchantMultiplier: multiplier,
    actualInputPricePerMillion: multiplier === 1 ? row.officialInputPricePerMillion : Number((row.officialInputPricePerMillion * multiplier).toFixed(3)),
    actualCachedInputPricePerMillion: row.officialCachedInputPricePerMillion === null ? null : multiplier === 1 ? row.officialCachedInputPricePerMillion : Number((row.officialCachedInputPricePerMillion * multiplier).toFixed(3)),
    actualOutputPricePerMillion: multiplier === 1 ? row.officialOutputPricePerMillion : Number((row.officialOutputPricePerMillion * multiplier).toFixed(3)),
  }))
}

function normalizeSub2ApiService(service: ApiService): ApiService {
  if (service.delivery !== 'Sub2API') return service
  return {
    ...service,
    modelMultipliers: service.modelMultipliers.map(row => ({ ...row, multiplier: `${sub2ApiFixedMultiplier.toFixed(2)}x` })),
    rate: `${sub2ApiFixedMultiplier.toFixed(2)}x`,
    defaultMultiplier: sub2ApiFixedMultiplier,
    modelPriceRows: applyMultiplierToModelPriceRows(service.modelPriceRows, sub2ApiFixedMultiplier),
  }
}

function normalizeApiServiceStore(services: ApiService[]) {
  return services.map(service => {
    const normalized = normalizeSub2ApiService(service)
    return {
      ...normalized,
      publiclyOrderable: normalized.publiclyOrderable ?? normalized.online,
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
  window.sessionStorage.setItem(apiPurchaseIntentStorageKey, JSON.stringify(apiPurchaseIntentStore))
  window.sessionStorage.setItem(apiPurchaseIntentEventStorageKey, JSON.stringify(apiPurchaseIntentEventStore))
}

function persistApiOrderStore() {
  window.sessionStorage.setItem(apiOrderStorageKey, JSON.stringify(apiOrderStore))
}

function persistCarpoolApplicationStores() {
  window.sessionStorage.setItem(carpoolApplicationStorageKey, JSON.stringify(carpoolApplicationStore))
  window.sessionStorage.setItem(carpoolApplicationEventStorageKey, JSON.stringify(carpoolApplicationEventStore))
}

function persistAdminStores() {
  window.sessionStorage.setItem(adminAuditLogStorageKey, JSON.stringify(adminAuditLogStore))
  window.sessionStorage.setItem(adminUserRiskProfileStorageKey, JSON.stringify(adminUserRiskProfileStore))
}

function persistMarketStores() {
  window.sessionStorage.setItem(officialPriceStorageKey, JSON.stringify(officialPriceStore))
  window.sessionStorage.setItem(carpoolStorageKey, JSON.stringify(carpoolStore))
  window.sessionStorage.setItem(apiServiceStorageKey, JSON.stringify(apiServiceStore))
  window.sessionStorage.setItem(apiServicePaymentSnapshotStorageKey, JSON.stringify(apiServicePaymentSnapshotStore))
}

function persistApiPaymentAccountSettings() {
  window.localStorage.setItem(apiPaymentAccountSettingsStorageKey, JSON.stringify(apiPaymentAccountSettingsStore))
}

function persistFeedbackTickets() {
  window.sessionStorage.setItem(feedbackStorageKey, JSON.stringify(feedbackTicketStore))
}

function persistNotificationReadState() {
  window.sessionStorage.setItem(notificationReadStorageKey, JSON.stringify(notificationReadStore))
}

function persistFavorites() {
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
    target.stats.completedCarpoolsLast30Days = null
    target.stats.completedApiOrdersLast30Days = null
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
      actionUrl: channel.type === 'linuxdo' ? `https://linux.do/u/${channel.value.replace(/^@/, '')}/messages/new` : undefined,
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
      actionUrl: channel.type === 'linuxdo' ? `https://linux.do/u/${channel.value.replace(/^@/, '')}/messages/new` : undefined,
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
    actionUrl: channel.type === 'linuxdo' ? `https://linux.do/u/${channel.value.replace(/^@/, '')}/messages/new` : undefined,
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

type ApiMerchantIdentitySource = Pick<ApiService, 'merchant' | 'merchantIdentityMode' | 'merchantDisplayName'>
type ApiMerchantProfileSource = Pick<ApiService, 'merchantIdentityMode' | 'merchantUsername'>
type ApiIntentMerchantSource = Pick<ApiPurchaseIntent, 'merchant' | 'snapshot'>

export function getApiMerchantDisplayName(source: ApiMerchantIdentitySource | ApiIntentMerchantSource) {
  if ('snapshot' in source) {
    return source.snapshot.merchantDisplayName || source.snapshot.merchant
  }
  return source.merchantDisplayName || source.merchant
}

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
    ? '买家提交购买意向后，双方站外确认 API 细节；平台不保存面板账号、密码、token 或登录态。'
    : '买家提交购买意向后，双方站外确认 API 细节、限速和鉴权边界；平台不保存 API Key 或 endpoint 密钥。'
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

export function isApiServicePubliclyOrderable(service: Pick<ApiService, 'online' | 'publiclyOrderable'>) {
  return service.online && service.publiclyOrderable
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
    buyer_cancelled: '买家已取消',
    owner_closed: '商户已关闭',
  }
  return labels[status]
}

export function getApiOrderStatusLabel(status: ApiOrderStatus) {
  const labels: Record<ApiOrderStatus, string> = {
    pending_payment: '待付款',
    payment_submitted: '买家已付款',
    paid_confirmed: '已确认收款',
    delivery_submitted: '已交付',
    completed: '已完成',
    cancelled: '已取消',
  }
  return labels[status]
}

export function getApiOrderDeliveryKindLabel(kind: ApiOrderDeliveryKind) {
  return kind === 'login_account' ? '登录账号接入' : 'API Key 接入'
}

export function getApiOrderNextAction(order: ApiOrder, role: 'buyer' | 'merchant') {
  if (role === 'buyer') {
    if (order.status === 'pending_payment') return '查看收款资料并付款'
    if (order.status === 'payment_submitted') return '等待商户确认收款'
    if (order.status === 'paid_confirmed') return '等待商户交付'
    if (order.status === 'delivery_submitted' || order.status === 'completed') return '查看交付凭证'
    if (order.status === 'cancelled') return '查看取消原因'
  }
  if (order.status === 'pending_payment') return '等待买家付款'
  if (order.status === 'payment_submitted') return '确认已收款'
  if (order.status === 'paid_confirmed') return '填写交付信息'
  if (order.status === 'delivery_submitted' || order.status === 'completed') return '已交付'
  return '查看详情'
}

export function isApiOrderBuyerActionRequired(order: ApiOrder) {
  return order.status === 'pending_payment' || order.status === 'delivery_submitted' || order.status === 'completed'
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
    pending_completion: '等待双方确认本期完成',
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
    if (application.status === 'pending_completion') return '确认本期完成'
    if (application.status === 'completed' && !application.buyerReview) return '评价车主'
    if (application.status === 'disputed') return '查看纠纷'
    return '查看详情'
  }

  if (application.status === 'pending_owner') return '处理申请'
  if (application.status === 'accepted_reserved' || application.status === 'waiting_contact') return '等待买家联系'
  if (application.status === 'contacted') return '确认用户已上车'
  if (application.status === 'joined_pending_confirmation') return '确认用户已上车'
  if (application.status === 'pending_completion') return '确认本期完成'
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
    monthlyQuotaAmount: carpool.monthlyQuotaAmount,
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
    sourceTopicUrl: `https://linux.do/t/carpool-${carpool.id}`,
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

export function getCarpoolApplyDisabledReason(carpool: Carpool, seatSummary?: Pick<CarpoolSeatSummary, 'availableSeats'> | null, hasOngoingApplication = false) {
  if (carpool.owner === currentBuyerName) return '不能申请自己的车源'
  if (!carpoolApplyAllowedStatuses.includes(carpool.status)) return carpool.status === '已满' ? '车位已满' : '车源暂不可申请'
  if (carpool.accessArrangementMode === 'not_allowed') return '访问安排不符合平台边界'
  const note = carpool.accessArrangementNote?.trim() ?? ''
  if (!note) return '缺少访问安排说明'
  if (hasCredentialSharingLanguage(note)) return '访问安排包含共享凭据风险'
  if (/chatgpt|openai/i.test(carpool.product) && !carpool.riskAcknowledged) return '需要先确认订阅拼车风险'
  if (carpool.hasUnresolvedDispute) return '车源存在未解决纠纷'
  if (hasOngoingApplication) return '已有进行中的申请'
  const availableSeats = seatSummary?.availableSeats ?? getCarpoolSeatSummary(carpool).availableSeats
  if (availableSeats < 1) return '车位已满'
  return ''
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

function registerMockDemandAuditListener() {
  if (shouldUseRealBackend()) return
  void import('@/mocks/demand').then(({ setMockDemandCreatedListener }) => {
    setMockDemandCreatedListener(demand => {
      appendAdminAuditLog({
        actorType: 'system',
        actorLabel: currentBuyerName,
        action: '提交求车需求',
        targetType: 'demand',
        targetId: demand.id,
        targetLabel: demand.title,
        beforeStatus: null,
        afterStatus: demand.status,
        reason: demand.note || '用户提交求车需求',
      })
    })
  })
}

registerMockDemandAuditListener()

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
    if (intent.status === 'buyer_cancelled') return '查看取消原因'
    if (intent.status === 'owner_closed') return '查看商户关闭原因'
    return '查看详情'
  }

  if (intent.status === 'open') return '记录已联系'
  if (intent.status === 'contacted') return '可关闭意向'
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
  if (row.targetType === 'demand') return `/demands/${row.id}`
  if (row.targetType === 'api-intent') return `/my/api-orders/${row.id}`
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
  return [order.id, order.apiPurchaseIntentId, order.serviceTitle, order.buyer, order.seller, getApiMerchantDisplayName({ merchant: order.seller, snapshot: order.intentSnapshot })]
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
      && (!keyword || apiOrderSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
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
    const [officialPrices, carpools, apiServices, demands] = await Promise.all([
      backendOfficialPrices(),
      backendGetCarpools(),
      backendAPIServices({ online: true }),
      getDemands(),
    ])

    return clone({ categoryRows, officialPrices, carpools, apiServices: apiServices.filter(isApiServicePubliclyOrderable), demands, productTrends, transactionRecords, apiPurchaseIntents: apiPurchaseIntentStore })
  }
  await wait()
  const demands = await getDemands()
  return clone({ categoryRows, officialPrices: officialPriceStore, carpools: carpoolStore, apiServices: apiServiceStore.filter(isApiServicePubliclyOrderable), demands, productTrends, transactionRecords, apiPurchaseIntents: apiPurchaseIntentStore })
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

export async function getCarpoolById(id: string) {
  if (shouldUseRealBackend()) return backendGetCarpoolById(id)
  await wait()
  const carpool = carpoolStore.find(item => item.id === id)
  return clone(carpool ? { ...carpool, seatSummary: getCarpoolSeatSummary(carpool) } : null)
}

export async function getMyCarpools() {
  if (shouldUseRealBackend()) return backendOwnerCarpools()
  await wait()
  return clone(carpoolStore
    .filter(item => item.owner === currentOwnerName)
    .map(item => ({ ...item, seatSummary: getCarpoolSeatSummary(item) })))
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

export async function parseLinuxDoTopic(topicUrl: string) {
  await wait()
  if (!isLinuxDoTopicUrl(topicUrl)) {
    throw new Error('只能读取 https://linux.do/t/* 原帖链接')
  }

  return clone({
    ...parsedLinuxDoTopicMock,
    topicUrl,
  })
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
        && (!filters.trustLevel || item.trustLevel >= filters.trustLevel)
        && (!filters.minimumPurchaseCnyMax || item.minimumPurchaseCny <= filters.minimumPurchaseCnyMax)
        && (!filters.minBalance || item.balance >= filters.minBalance)
        && (!keyword || apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(keyword)))
    })
    .sort((a, b) => {
      if (filters.sort === 'multiplier_asc') return a.defaultMultiplier - b.defaultMultiplier || a.responseMedianMinutes - b.responseMedianMinutes
      if (filters.sort === 'response_fast') return a.responseMedianMinutes - b.responseMedianMinutes || a.defaultMultiplier - b.defaultMultiplier
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      const aPersonal = a.merchantType !== '商户'
      const bPersonal = b.merchantType !== '商户'
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || a.responseMedianMinutes - b.responseMedianMinutes
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
        && (!filters.trustLevel || item.trustLevel >= filters.trustLevel)
    })
    .sort((a, b) => {
      if (filters.sort === 'credit_price_asc') return apiCreditPriceCny(a) - apiCreditPriceCny(b) || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'minimum_purchase_asc') return a.minimumPurchaseCny - b.minimumPurchaseCny || a.responseMedianMinutes - b.responseMedianMinutes
      if (filters.sort === 'panel_supported') return Number(b.deliveryModes.includes('sub2api_panel_account')) - Number(a.deliveryModes.includes('sub2api_panel_account')) || a.responseMedianMinutes - b.responseMedianMinutes
      if (filters.sort === 'response_fast') return a.responseMedianMinutes - b.responseMedianMinutes || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || apiCreditPriceCny(a) - apiCreditPriceCny(b)
        || a.responseMedianMinutes - b.responseMedianMinutes
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
      if (filters.sort === 'minimum_purchase_asc') return a.minimumPurchaseCny - b.minimumPurchaseCny || a.responseMedianMinutes - b.responseMedianMinutes
      if (filters.sort === 'response_fast') return a.responseMedianMinutes - b.responseMedianMinutes || a.minimumPurchaseCny - b.minimumPurchaseCny
      if (filters.sort === 'recent') return compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
      return Number(isApiServicePubliclyOrderable(b)) - Number(isApiServicePubliclyOrderable(a))
        || a.responseMedianMinutes - b.responseMedianMinutes
        || a.minimumPurchaseCny - b.minimumPurchaseCny
        || compareTimeDesc(a.lastOnlineConfirmedAt, b.lastOnlineConfirmedAt)
    })
}

export async function getApiServices(filters: ApiServiceFilters = {}) {
  if (shouldUseRealBackend()) return backendAPIServices(filters)
  await wait()
  return clone(filterApiServices(filters))
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

export async function getApiServiceById(id: string) {
  if (shouldUseRealBackend()) return backendAPIServiceById(id)
  await wait()
  return clone(apiServiceStore.find(item => item.id === id && isApiServicePubliclyOrderable(item)) ?? null)
}

export async function getMyApiServices() {
  if (shouldUseRealBackend()) return backendOwnerAPIServices()
  await wait()
  return clone(apiServiceStore.filter(item => item.merchantUsername === myUserProfileStore.username))
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
  await wait()
  return cloneApiPaymentAccountSettings(apiPaymentAccountSettingsStore)
}

export async function updateApiPaymentAccountSettings(payload: Omit<ApiPaymentAccountSettings, 'updatedAt'>) {
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
  return clone({
    profile: sanitizePublicUserProfile(profile),
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
      { type: 'linuxdo', label: 'linux.do 私信', maskedValue: `@${application.ownerUsername}`, displayValue: `@${application.ownerUsername}`, verified: true, usageScope: 'carpool_owner', actionUrl: `https://linux.do/u/${application.ownerUsername}/messages/new` },
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

export async function getAdminSectionRows(section: AdminSection): Promise<AdminRow[]> {
  await wait()

  if (shouldUseRealBackend() && (section === 'api-merchants' || section === 'api-services')) {
    return backendAdminAPIServiceRows()
  }

  if (shouldUseRealBackend() && section === 'carpools') {
    return backendAdminCarpoolRows()
  }

  if (shouldUseRealBackend() && (section === 'official-prices' || section === 'price-leads')) {
    return backendAdminOfficialPriceRows()
  }

  if (shouldUseRealBackend() && section === 'demands') {
    return backendAdminDemandRows()
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
      owner: `${item.owner} · 信任等级${item.trustLevel}`,
      status: item.status,
      risk: item.linuxdoBound ? '原帖已绑定' : '缺少原帖',
      targetType: 'carpool',
      detailItems: [
        { label: '车主类型', value: item.ownerType },
        { label: '开通方式', value: item.openingMethod },
        { label: '商户承诺', value: item.warranty },
        { label: '最近确认', value: item.confirmedAt },
      ],
    })))
  }

  if (section === 'demands') {
    const demands = await getDemands()
    return withAdminRowLinks(demands.map(item => ({
      id: item.id,
      primary: item.title,
      secondary: `最高 ¥${item.maxPrice}/月 · ${item.require}`,
      owner: `${item.poster} · 信任等级${item.trustLevel}`,
      status: item.status,
      risk: item.linuxdoPost,
      targetType: 'demand',
      detailItems: [
        { label: '预算', value: `¥${item.maxPrice}/月` },
        { label: '地区', value: item.region },
        { label: '车主偏好', value: item.ownerPreference === 'only-personal' ? '只看个人车主' : item.ownerPreference === 'personal' ? '优先个人车主' : '不限' },
        { label: '更新时间', value: item.updatedAt },
      ],
    })))
  }

  if (section === 'api-merchants' || section === 'api-services') {
    return withAdminRowLinks(apiServiceStore.map(item => ({
      id: item.id,
      primary: item.title,
      secondary: `${item.models.join(' / ')} · ${item.delivery} · 接入细节站外确认`,
      owner: canOpenApiMerchantProfile(item)
        ? `${getApiMerchantDisplayName(item)} · 信任等级${item.trustLevel}`
        : `${getApiMerchantDisplayName(item)} → ${item.merchantUsername} · 信任等级${item.trustLevel}`,
      status: item.online ? '在线' : '离线',
      risk: item.unresolvedDisputes ? `${item.unresolvedDisputes} 个未解决纠纷` : item.warranty,
      targetType: section === 'api-services' ? 'api-service' : 'api-merchant',
      targetTo: apiServiceAdminTargetLink(item),
      detailItems: [
        { label: '商户身份', value: item.merchantIdentityMode === 'store_alias' ? `店铺名展示，真实用户 ${item.merchantUsername}` : '公开主页展示' },
        { label: '最低意向金额', value: `¥${item.minimumPurchaseCny}` },
        { label: '用量核对', value: getApiUsageVisibilityLabel(item.usageVisibility) },
        { label: '有效期', value: item.expiresAt },
      ],
    })))
  }

  if (section === 'trade-intents') {
    return withAdminRowLinks(filterApiPurchaseIntents({ sort: 'updated_desc' }).map(item => ({
      id: item.id,
      primary: `${item.snapshot.serviceTitle} 购买意向`,
      secondary: `${item.id} · 意向金额 ¥${item.purchaseAmountCny} · 联系方式按参与方详情展示`,
      owner: `${getApiMerchantDisplayName(item)} / 买家 ${item.buyer}`,
      status: getApiStatusLabel(item.status),
      risk: item.ownerCloseReason
        ? `商户关闭：${item.ownerCloseReason}`
        : item.buyerCancelReason
          ? `买家取消：${item.buyerCancelReason}`
          : getApiUsageVisibilityLabel(item.snapshot.usageVisibility),
      targetType: 'api-intent',
      detailItems: [
        { label: '意向金额', value: `¥${item.purchaseAmountCny}` },
        { label: '目标模型', value: item.targetModel },
        { label: '联系方式', value: '仅参与方详情可见' },
        { label: '最近更新', value: item.updatedAt },
      ],
    })))
  }

  if (section === 'carpool-applications') {
    return withAdminRowLinks(filterCarpoolApplications({ sort: 'updated_desc' }).map(item => ({
      id: item.id,
      primary: `${item.snapshot.productName} 上车申请`,
      secondary: `${item.snapshot.regionName} · ${item.snapshot.priceLabel} ¥${item.snapshot.monthlyPriceCny}/月 · ${item.seatsRequested} 席`,
      owner: `${item.ownerUsername} / 申请人 ${item.applicantUsername}`,
      status: getCarpoolApplicationStatusLabel(item.status),
      risk: item.status === 'disputed'
        ? item.disputeReason ?? '纠纷待处理'
        : item.responsibility
          ? `责任：${item.responsibility}`
          : `更新 ${item.updatedAt}`,
      targetType: 'carpool-application',
      detailItems: [
        { label: '申请席位', value: `${item.seatsRequested} 席` },
        { label: '申请人信任等级', value: String(item.applicantStats.trustLevel) },
        { label: '预留截止', value: item.reservedUntil ?? '无' },
        { label: '最近更新', value: item.updatedAt },
      ],
    })))
  }

  if (section === 'feedback') {
    return getAdminFeedbackRows()
  }

  if (section === 'certifications') {
    return withAdminRowLinks(carpoolStore.map(item => ({
      id: `cert-${item.id}`,
      primary: item.owner,
      secondary: `${item.ownerType} · ${item.product}`,
      owner: `linux.do 信任等级${item.trustLevel}`,
      status: item.linuxdoBound ? '已绑定' : '待补充',
      risk: item.ownerType,
      targetType: 'certification',
      detailItems: [
        { label: '车源', value: item.product },
        { label: '地区', value: item.region },
        { label: '原帖状态', value: item.sourcePostAccessible ? '可访问' : '不可访问' },
      ],
    })))
  }

  if (section === 'users' || section === 'restrictions') {
    return withAdminRowLinks(adminUserRiskProfileStore.map(item => ({
      id: item.id,
      primary: item.username,
      secondary: `${item.identity} · ${item.linuxdoBound ? '已绑定 linux.do' : '未绑定 linux.do'} · 信任等级${item.trustLevel}`,
      owner: `完成 拼车${item.carpoolCompletions} / API${item.apiCompletions}`,
      status: item.accountStatus,
      risk: item.restrictions.length
        ? `${item.restrictions.join(' / ')} · 可限制资料修改或冻结联系方式`
        : item.unresolvedDisputes
          ? `${item.unresolvedDisputes} 个未解决纠纷 · 纠纷处理员按联系快照查看必要联系方式`
          : '可下架头像、重置昵称/用户名、隐藏简介；普通审核员不查看完整联系方式',
      targetType: 'user',
      detailItems: [
        { label: '账号状态', value: item.accountStatus },
        { label: '限制项', value: item.restrictions.length ? item.restrictions.join(' / ') : '无' },
        { label: '责任取消', value: `买家 ${item.buyerResponsibleCancellations} / 车主 ${item.ownerResponsibleCancellations}` },
        { label: '最近活跃', value: item.lastActiveAt },
      ],
    })))
  }

  if (section === 'reports') {
    return withAdminRowLinks([
      { id: 'report-1', primary: 'API 意向未及时响应', secondary: '买家提交脱敏说明，商户待回应', owner: '买家 木舟 / 商户 小葵 API', status: '处理中', risk: '需 24h 内处理', targetType: 'report', detailItems: [{ label: '处理建议', value: '要求商户补充站外确认记录' }, { label: '敏感信息', value: '仅显示脱敏说明' }] },
      { id: 'report-2', primary: '车源剩余名额争议', secondary: '原帖信息与站内展示不一致', owner: '买家 青柠 / 车主 北风', status: '待复核', risk: '信息不一致', targetType: 'report', detailItems: [{ label: '处理建议', value: '核对原帖与站内剩余席位' }, { label: '敏感信息', value: '不展示联系方式' }] },
      { id: 'report-contact-1', primary: '联系方式无效举报', secondary: '联系快照显示可复制，但买家反馈无法联系', owner: '买家 demo_user / 商户 小葵 API', status: '处理中', risk: '只允许纠纷处理员按意向记录查看必要快照', targetType: 'contact-report', detailItems: [{ label: '处理建议', value: '按联系快照检查必要联系方式' }, { label: '可见范围', value: '仅纠纷处理员' }] },
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
    ],
  })))
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

function hasCredentialSharingLanguage(value: string) {
  const hasRiskyCredentialText = /(共享|共用|转交|借用).*(账号|密码|主账号|session|cookie|token|登录态)|主账号|主 key|主key|session|cookie|refresh token|api key/i.test(value)
  const statesProhibition = /(不得|不能|不可|禁止|不允许|拒绝|避免|不保存|不交付|不提供|不会保存|不会交付|不会提供).{0,16}(共享|共用|转交|借用|填写|粘贴|上传|提供|交换|索要|账号|密码|主账号|session|cookie|token|登录态|api key)/i.test(value)
  return hasRiskyCredentialText && !statesProhibition
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
  if (carpoolRequiresRiskAck(product, payload.riskNoticeCode) && !payload.riskAcknowledged) {
    throw new Error('请先确认该套餐的发布边界。')
  }
}

function apiGatewayFromDistribution(value: unknown): ApiService['delivery'] {
  if (value === 'sub2api') return 'Sub2API'
  return '其他'
}

function apiBillingMode(value: unknown): ApiBillingMode {
  return value === 'fixed_package' || value === 'manual_credit' || value === 'metered_credit' ? value : 'metered_credit'
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

function buildModelPriceRowsFromPayload(payload: Record<string, unknown>, defaultMultiplier: number, lockedMultiplier = false): ApiService['modelPriceRows'] {
  const selected = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, multiplierOverride?: number | null, enabled?: boolean }> : []
  return selected
    .filter(item => item.enabled !== false)
    .map(item => {
      const model = modelCatalog.find(row => row.id === item.modelId)
      const multiplier = !lockedMultiplier && typeof item.multiplierOverride === 'number' && Number.isFinite(item.multiplierOverride) ? item.multiplierOverride : defaultMultiplier
      return {
        modelId: model?.id ?? item.modelId ?? 'custom-model',
        modelName: model?.displayName ?? item.modelId ?? '自定义模型',
        provider: model?.provider === 'openai' ? 'OpenAI' : model?.provider === 'anthropic' ? 'Anthropic' : 'Other',
        officialInputPricePerMillion: model?.officialInputPricePerMillion ?? 0,
        officialCachedInputPricePerMillion: model?.officialCachedInputPricePerMillion ?? null,
        officialOutputPricePerMillion: model?.officialOutputPricePerMillion ?? 0,
        merchantMultiplier: multiplier,
        actualInputPricePerMillion: Number(((model?.officialInputPricePerMillion ?? 0) * multiplier).toFixed(3)),
        actualCachedInputPricePerMillion: model?.officialCachedInputPricePerMillion === null || model?.officialCachedInputPricePerMillion === undefined ? null : Number((model.officialCachedInputPricePerMillion * multiplier).toFixed(3)),
        actualOutputPricePerMillion: Number(((model?.officialOutputPricePerMillion ?? 0) * multiplier).toFixed(3)),
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
  const serviceMultiplier = payload.serviceMultiplier ?? 1
  const monthlyQuotaAmount = payload.monthlyQuotaAmount ?? 0
  const carpool: Carpool = {
    id,
    product: product?.displayName ?? payload.customProductName?.trim() ?? '自定义产品',
    region: regionName,
    monthly,
    serviceMultiplier,
    monthlyQuotaAmount,
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
    linuxdoBound: Boolean(payload.linuxDoTopicUrl),
    sourcePostAccessible: Boolean(payload.linuxDoTopicUrl),
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
  const id = `api-${Date.now()}`
  const normalized = normalizeMerchantDisplayName(payload)
  const gateway = apiGatewayFromDistribution(payload.distributionSystem)
  const defaultMultiplier = gateway === 'Sub2API' ? sub2ApiFixedMultiplier : numberValue(payload.defaultMultiplier, 1)
  const cnyPerUsdCredit = numberValue(payload.cnyPerUsdCredit, 1)
  const selectedModels = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, enabled?: boolean }> : []
  const models = selectedModels
    .filter(item => item.enabled !== false)
    .map(item => modelCatalog.find(model => model.id === item.modelId)?.displayName ?? item.modelId ?? '自定义模型')
    .filter(Boolean)
  const merchantIdentityMode = apiMerchantIdentityMode(normalized.merchantIdentityMode)
  const deliveryModes = apiDeliveryModes(payload.deliveryModes)
  const billing = apiBillingMode(payload.billingMode)
  const isPublish = payload.status === 'reviewing'
  const paymentOptions = Array.isArray(payload.paymentOptions)
    ? payload.paymentOptions as Array<{ paymentMethod?: string, enabled?: boolean, paymentInstructions?: string, paymentQrCodeDataUrl?: string | null }>
    : []
  const normalizedPaymentOptions = normalizeRawApiPaymentOptions(paymentOptions)
  const hasEnabledPayment = normalizedPaymentOptions.some(item => item.enabled && isApiPaymentOptionComplete(item))
  const publiclyOrderable = isPublish && hasEnabledPayment
  const responseMinutes = numberValue(payload.paymentWindowMinutes, 10)
  const state: ApiServiceState = isPublish ? 'online' : 'offline'
  const quotaExpiresAt = beijingDateTimeInputToISOString(String(payload.quotaExpiresAt ?? ''))
  const service: ApiService = {
    id,
    title: stringValue(payload.generatedTitle, models.length ? `${models[0]} API 服务` : '新 API 服务'),
    sourceUrl: stringValue(payload.sourceUrl, ''),
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
    panelBaseUrl: gateway === 'Sub2API' ? '提交意向后由商户站外确认 API 细节' : null,
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
    expectedResponseMinutes: responseMinutes,
    responseMedianMinutes: responseMinutes,
    dailyOrderLimit: 5,
    todayOrderCount: 0,
    unresolvedDisputes: 0,
    warning: publiclyOrderable ? undefined : isPublish ? '待配置接单设置' : '草稿尚未上线',
    warranty: (payload.warranty as { mode?: string, warrantyDays?: number | null } | undefined)?.mode === 'merchant_warranty' ? `商户承诺：${(payload.warranty as { warrantyDays?: number | null }).warrantyDays ?? 7} 天可用性处理，平台不担保、不代赔` : '按商户备注站外协商，平台不担保、不代赔',
    refundPolicy: stringValue((payload.warranty as { refundNote?: string } | undefined)?.refundNote, '按服务说明站外协商'),
    quotaExpiresAt: quotaExpiresAt || undefined,
    expiresAt: formatQuotaExpiresAtLabel(quotaExpiresAt) || '按服务说明',
    completed30d: 0,
    reviewCount: 0,
    officialPricingVersion: '2026-06',
    officialPricingUpdatedAt: nowText(),
    merchantNote: stringValue(payload.merchantNote, '建议首次小额测试。'),
    modelPriceRows: buildModelPriceRowsFromPayload(payload, defaultMultiplier, gateway === 'Sub2API'),
    contactChannels: [{ type: 'linuxdo', label: 'linux.do 私信', value: `@${currentMerchantName}` }],
    acceptedPaymentMethods: normalizedPaymentOptions.filter(option => option.enabled).map(option => option.paymentMethod),
  }
  apiServicePaymentSnapshotStore[id] = normalizeApiPaymentAccountSettings({
    paymentWindowMinutes: responseMinutes,
    paymentOptions: normalizedPaymentOptions,
  }).paymentOptions
  apiServiceStore.unshift(service)
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
  target.state = 'online'
  target.online = true
  target.publiclyOrderable = true
  target.warning = undefined
  target.lastOnlineConfirmedAt = nowText()
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

  const demands = await getDemands()
  const demandRows: UnifiedNotification[] = demands
    .filter(item => item.poster === currentBuyerName || item.status === '匹配中')
    .slice(0, 6)
    .map(item => ({
      id: `demand-notice-${item.id}`,
      type: '求车需求',
      title: item.status === '匹配中' ? '求车需求匹配中' : '求车需求状态更新',
      detail: `${item.title} · 预算 ¥${item.maxPrice}/月`,
      time: item.updatedAt,
      unread: item.status === '匹配中',
      to: `/demands/${item.id}`,
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
    to: '/admin/audit-logs',
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

  return markReadState([...carpoolRows, ...apiRows, ...officialRows, ...demandRows, ...feedbackRows, ...adminRows, ...fixedRows]
    .sort((a, b) => compareTimeDesc(a.time, b.time)))
}

export async function getNotifications(): Promise<UnifiedNotification[]> {
  if (shouldUseRealBackend()) return backendNotifications()
  await wait()
  return clone(await buildUnifiedNotifications())
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
      status: isApiServicePubliclyOrderable(service) ? '可提交意向' : service.online ? '待配置接单' : service.state === 'reviewing' ? '审核中' : '离线',
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
  const demandRows = await getDemands()
  const officialResults = officialPriceStore
    .filter(item => [item.product, item.plan, item.region, item.channel, item.submitter].some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `official-${item.id}`, type: '官方价格' as const, title: `${item.product} ${item.plan}`, subtitle: `${item.region} · ${item.originalPrice} · ${item.status}`, badge: item.status, to: `/official-prices/${item.id}` }))
  const carpoolResults = carpoolStore
    .filter(item => [item.product, item.region, item.owner].some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `carpool-${item.id}`, type: '车源' as const, title: item.product, subtitle: `${item.region} · ${item.owner} · ${getPricingDisplay(item).primaryLabel} ¥${getPricingDisplay(item).primaryPrice}/月`, badge: item.status, to: `/carpools/${item.id}` }))
  const demandResults = demandRows
    .filter(item => [item.title, item.require, item.poster, item.region].some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `demand-${item.id}`, type: '求车' as const, title: item.title, subtitle: `${item.region} · 预算 ¥${item.maxPrice}/月 · ${item.poster}`, badge: item.status, to: `/demands/${item.id}` }))
  const apiResults = apiServiceStore
    .filter(isApiServicePubliclyOrderable)
    .filter(item => apiServicePublicSearchTerms(item).some(value => value.toLowerCase().includes(q)))
    .map(item => ({ id: `api-${item.id}`, type: 'API 服务' as const, title: item.title, subtitle: `${getApiMerchantDisplayName(item)} · ${item.models.slice(0, 3).join(' / ')}`, badge: '可提交意向', to: `/api-market/${item.id}` }))
  const merchantResults = publicMerchantProfiles
    .filter(item => [item.username, item.displayName, item.identity, item.merchantId].some(value => value.toLowerCase().includes(q)))
    .map(item => ({
      id: `merchant-${item.username}`,
      type: '商户' as const,
      title: item.displayName,
      subtitle: `@${item.username} · ${item.identity} · 近30天完成 ${item.completed30d}`,
      badge: item.unresolvedDisputes ? `${item.unresolvedDisputes} 个未解决纠纷` : item.originalPostBound ? '原帖已绑定' : '待补充原帖',
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
        subtitle: `公开个人主页 · @${item.username} · 信任等级${item.trustLevel}`,
        badge: item.linuxDoBound ? '已绑定 linux.do' : '未绑定',
        to: `/u/${item.username}`,
      }
    })
  return clone([...officialResults, ...carpoolResults, ...demandResults, ...apiResults, ...merchantResults, ...userResults])
}

export async function getReviewCenterRows(): Promise<ReviewCenterRow[]> {
  if (shouldUseRealBackend()) return backendReviewCenterRows()
  await wait()
  const carpoolRows: ReviewCenterRow[] = carpoolApplicationStore
    .filter(item => item.status === 'completed' && (item.applicantUserId === currentBuyerId || item.ownerUserId === currentOwnerId))
    .map(item => ({
      id: `review-carpool-${item.id}`,
      sourceType: 'carpool',
      sourceId: item.id,
      target: item.snapshot.productName,
      counterparty: item.applicantUserId === currentBuyerId ? item.ownerUsername : item.applicantUsername,
      status: item.buyerReview ? '已评价' : '可评价',
      rating: item.buyerReview?.rating ?? 0,
      tags: item.buyerReview?.tags ?? [],
      note: item.buyerReview?.note ?? '',
      createdAt: item.buyerReview?.createdAt ?? item.completedAt ?? item.updatedAt,
    }))
  return clone(carpoolRows.sort((a, b) => compareTimeDesc(a.createdAt, b.createdAt)))
}

export async function submitReview(payload: SubmitReviewPayload) {
  if (shouldUseRealBackend()) return backendSubmitReview(payload)
  await wait()
  return reviewCarpoolApplication(payload.sourceId, {
    rating: payload.rating,
    tags: payload.tags,
    note: payload.note,
  })
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

export async function getMerchantCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  if (shouldUseRealBackend()) return backendMerchantCarpoolApplications(filters)
  await wait()
  return clone(filterCarpoolApplications({ ...filters, ownerId: currentOwnerId, sort: filters.sort ?? 'default_owner' }))
}

export async function getCarpoolApplicationById(id: string) {
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
  const duplicate = carpoolApplicationStore.find(item => item.carpoolId === carpoolId && item.applicantUserId === currentBuyerId && isOngoingCarpoolApplication(item.status))
  if (duplicate) throw new Error('已有进行中的上车申请')
  const seatSummary = getCarpoolSeatSummary(carpool)
  const disabledReason = getCarpoolApplyDisabledReason(carpool, seatSummary)
  if (disabledReason) throw new Error(disabledReason)

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
      note: '买家已记录完成站外联系。',
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
      note: completed ? '双方确认完成。' : '买家确认本期完成。',
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
      note: completed ? '双方确认完成。' : '车主确认本期完成。',
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
    application.buyerReview = { ...payload, createdAt: nowText() }
    appendCarpoolApplicationEvent({
      applicationId: id,
      actorId: application.applicantUserId,
      actorLabel: application.applicantUsername,
      actorRole: 'buyer',
      type: 'admin_updated',
      note: `买家已评价：${payload.rating} 星`,
    })
  })
}

export async function createApiPurchaseIntent(payload: CreateApiPurchaseIntentPayload) {
  if (shouldUseRealBackend()) return backendCreateAPIPurchaseIntent(payload)
  await wait()
  const service = apiServiceStore.find(item => item.id === payload.serviceId)
  if (!service) throw new Error(`API service not found: ${payload.serviceId}`)
  if (!isApiServicePubliclyOrderable(service) || service.state !== 'online') throw new Error('服务当前不可提交购买意向。')
  if (!service.deliveryModes.includes(payload.deliveryMode)) throw new Error('选择的 API 细节不属于该服务。')
  if (service.delivery !== 'Sub2API' && payload.deliveryMode === 'sub2api_panel_account') throw new Error('当前服务不支持该 API 细节。')
  if (payload.purchaseAmountCny < service.minimumPurchaseCny) throw new Error(`最低意向金额为 ¥${service.minimumPurchaseCny}`)
  if (payload.purchaseAmountCny > service.maxBuy) throw new Error(`单笔最高意向金额为 ¥${service.maxBuy}`)
  if (payload.purchaseAmountCny > service.balance / service.creditPerCny) throw new Error('超过商户当前可售美元额度上限。')

  const id = `api-intent-${Date.now()}`
  const createdAt = nowText()
  const snapshot: ApiPurchaseIntent['snapshot'] = {
    ...createSnapshot(service),
    selectedDeliveryMode: payload.deliveryMode,
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
    purchaseAmountCny: payload.purchaseAmountCny,
    purchasedCredit: Math.round(payload.purchaseAmountCny * service.creditPerCny),
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
  if (apiOrderStore.some(item => item.apiPurchaseIntentId === intentId)) {
    throw new Error('该购买意向已经创建过订单。')
  }
  const option = intent.snapshot.paymentOptions?.find(item => item.paymentMethod === paymentMethod && item.enabled)
  if (!option || !isApiPaymentOptionComplete(option)) {
    throw new Error('选择的收款方式不可用，请联系商户更新收款设置。')
  }
  const createdAt = nowText()
  const order: ApiOrder = {
    id: `api-order-${Date.now()}`,
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
    currency: 'CNY',
    selectedPaymentMethod: paymentMethod,
    paymentWindowMinutes: 10,
    paymentExpiresAt: minutesFromNow(10),
    version: 1,
    intentSnapshot: clone(intent.snapshot),
    selectedDeliveryMode: intent.selectedDeliveryMode,
    requestedUsdAllowance: intent.purchasedCredit,
    merchantContactChannels: clone(intent.contactChannels),
    buyerContactChannels: clone(mockBuyerContactChannels(intent)),
    viewerRole: 'buyer',
    createdAt,
    updatedAt: createdAt,
  }
  apiOrderStore.unshift(order)
  persistApiOrderStore()
  return clone(order)
}

export async function getMyApiOrders(filters: ApiOrderFilters = {}) {
  if (shouldUseRealBackend()) return backendMyAPIOrders(filters)
  await wait()
  return clone(filterApiOrders({ ...filters, buyerId: currentBuyerId }))
}

export async function getMerchantApiOrders(filters: ApiOrderFilters = {}) {
  if (shouldUseRealBackend()) return backendOwnerAPIOrders(filters)
  await wait()
  return clone(filterApiOrders({ ...filters, sellerId: currentMerchantId }))
}

export async function getApiOrderById(id: string, perspective: 'buyer' | 'merchant' = 'buyer') {
  if (shouldUseRealBackend()) {
    return perspective === 'merchant' ? backendOwnerAPIOrder(id) : backendMyAPIOrder(id)
  }
  await wait()
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
    if (order.status !== 'pending_payment') throw new Error('只有待付款订单可以标记已付款。')
    order.status = 'payment_submitted'
    order.paymentSummary = paymentSummary.trim()
    order.paymentSubmittedAt = nowText()
  })
}

export async function confirmApiOrderPayment(id: string, version: number) {
  if (shouldUseRealBackend()) return backendConfirmAPIOrderPayment(id, version)
  await wait()
  return updateApiOrder(id, order => {
    if (order.version !== version) throw new Error('订单已更新，请刷新后重试。')
    if (order.status !== 'payment_submitted') throw new Error('只有买家已付款订单可以确认收款。')
    order.status = 'paid_confirmed'
    order.paidConfirmedAt = nowText()
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
    events.push({
      id: `${order.id}-completed`,
      orderId: order.id,
      actorLabel: order.buyer,
      actorRole: 'buyer',
      type: 'completed',
      fromStatus: 'delivery_submitted',
      toStatus: 'completed',
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
  const rows = apiOrderStore
    .filter(item => ['pending_payment', 'payment_submitted', 'paid_confirmed', 'delivery_submitted'].includes(item.status))
    .slice(0, 6)
    .map(item => ({
      id: `api-notice-${item.id}`,
      title: getApiOrderStatusLabel(item.status),
      detail: `${item.serviceTitle} · ${item.buyer} / ${item.seller}`,
      time: item.updatedAt,
      unread: item.status === 'payment_submitted' || item.status === 'paid_confirmed',
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
  if (shouldUseRealBackend() && row.targetType === 'demand') {
    return backendUpdateAdminDemandStatus(row, status, reason)
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
  if (row.targetType === 'demand') {
    const { updateMockDemandAdminStatus } = await import('@/mocks/demand')
    updateMockDemandAdminStatus(row.id, status)
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

export async function runAdminModerationAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (shouldUseRealBackend() && (row.targetType === 'api-service' || row.targetType === 'api-merchant')) {
    return backendRunAdminAPIServiceAction(row, action, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'carpool') {
    return backendRunAdminCarpoolAction(row, action, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'official-price') {
    return backendRunOfficialPriceAdminAction(row, action, reason)
  }
  if (shouldUseRealBackend() && row.targetType === 'demand') {
    return backendRunAdminDemandAction(row, action, reason)
  }
  if (shouldUseRealBackend() && (row.targetType === 'report' || row.targetType === 'dispute' || row.targetType === 'appeal')) {
    return backendRunReportAdminAction(row, action, reason)
  }
  await wait()
  if (['take_down', 'restore', 'restrict', 'warn', 'suspend', 'ban'].includes(action) && !reason.trim()) {
    throw new Error('请填写明确的操作原因。')
  }
  const restorableStatuses = ['已下架', '已限制', '暂停', '离线', '临时封禁', '永久封禁', '申诉复核中', '需要补充信息', 'partially_restricted', 'temporarily_suspended', 'permanently_banned', 'under_review']
  if (action === 'restore' && !restorableStatuses.some(status => row.status.includes(status))) {
    throw new Error('当前状态不需要恢复，不能执行恢复操作。')
  }
  const downableStatuses = ['已验证', '已通过', '可上车', '在线', '匹配中', 'normal']
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

  if (row.targetType === 'user') {
    const target = adminUserRiskProfileStore.find(item => item.id === row.id)
    if (target) {
      target.accountStatus = action === 'warn'
        ? 'warning'
        : action === 'restrict'
          ? 'partially_restricted'
          : action === 'suspend'
            ? 'temporarily_suspended'
            : action === 'ban'
              ? 'permanently_banned'
              : action === 'restore'
                ? 'normal'
                : target.accountStatus
      if (action === 'restrict' && !target.restrictions.includes('禁止申请上车')) target.restrictions.push('禁止申请上车')
      if (action === 'restore') target.restrictions = []
      persistAdminStores()
    }
  }

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
  ApiService,
  ApiServiceState,
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
  OrderContactSnapshot,
  OrderContactSnapshotItem,
  PublicMerchantProfile,
  PublicUserProfile,
  ProductTrend,
  TransactionRecord,
  TransactionTrendPoint,
  UserContactMethod,
  UserPrivacySettings,
  UserProfile,
}
