import type {
  AdminRow,
  ApiBillingMode,
  ApiDeliveryMode,
  ApiOrder,
  ApiOrderDeliveryCredential,
  ApiOrderFilters,
  ApiOrderPaymentInstructions,
  ApiOrderPaymentIssueReason,
  ApiOrderStatus,
  ApiQuotaBatch,
  ApiQuotaCredentialSummary,
  ApiQuotaOffer,
  ApiQuotaOfferFilters,
  ApiQuotaOrderSnapshot,
  ApiQuotaRound,
  ApiQuotaSaleMode,
  ApiQuotaRushOfferPublication,
  ApiQuotaSystemSaleSlotList,
  ApiTTFTBand,
  ApiPurchaseIntent,
  ApiPurchaseIntentEvent,
  ApiPurchaseIntentFilters,
  ApiService,
  ApiServiceCommercialSnapshot,
  ApiServiceSalesChannel,
  ApiServiceSalesView,
  ApiServicePackageSnapshot,
  ApiServiceFilters,
  ApiUsageVisibility,
  ContactMethodType,
  CreateApiQuotaBatchPayload,
  CreateApiQuotaOfferPayload,
  CreateApiQuotaOrderPayload,
  CreateApiQuotaRushOfferPayload,
  CreateApiQuotaRoundPayload,
  CreateApiPurchaseIntentPayload,
  ModelCatalogItem,
  ModelPriceRow,
  OtherApiMarketFilters,
  OwnerApiService,
  PublicApiQuotaOffer,
  SaveContactMethodRequest,
  SubmitApiOrderDeliveryCredentialPayload,
  Sub2ApiMarketFilters,
  UserContactMethod,
} from '@/lib/api'
import type {
  AdminApiServicePromotion,
  AdminApiServicePromotionList,
  ApiServicePromotionAvailability,
  ApiService as GeneratedApiService,
  ApiServiceSalesChannel as BackendApiServiceSalesChannel,
  ApiServiceSalesSummary as BackendApiServiceSalesSummary,
  CreateApiServicePromotionRequest,
  PublicApiService,
  PublicApiServicePromotionList,
} from '@/api/generated/openapi'
import { backendFormDataMutation, backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { apiPaymentMethodRequiresQrCode, isApiPaymentMethod, normalizeQrCodeDataUrl } from '@/lib/apiPaymentSettings'
import { beijingDateTimeInputToISOString, formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'
import { backendMyMerchantProfile, backendUpsertMerchantProfile } from '@/lib/profileBackend'
import { compareDecimal, divideDecimal, normalizeDecimal, normalizeDecimalTrimmed } from '@/lib/decimal'
import { mapBackendReputationSummary } from '@/lib/reputationBackend'
import type { ReputationSummary } from '@/types/reputation'

type ListResponse<T> = { items: T[] }

type BackendAccessMode = {
  accessMode: string
  publicNote?: string
}

type BackendServiceModel = {
  id?: string
  modelCatalogId: string
  modelPriceVersionId?: string
  modelNameSnapshot: string
  providerSnapshot: string
  capabilitiesSnapshot: string[]
  merchantMultiplier: string
  effectiveInputPricePerMillion?: string
  effectiveCachedInputPricePerMillion?: string
  effectiveOutputPricePerMillion?: string
  enabled: boolean
}

type BackendServicePackage = {
  id?: string
  name: string
  priceCny: string
  panelAllowance: string
  durationDays?: number | null
  stockTotal: number
  stockAvailable: number
  description: string
  enabled: boolean
  sortOrder: number
  models: BackendServicePackageModel[]
}

type BackendServicePackageModel = {
  serviceModelId: string
  modelCatalogId: string
  modelPriceVersionId?: string
  modelNameSnapshot: string
  providerSnapshot: string
  merchantMultiplier: string
}

type BackendPaymentOption = {
  id?: string
  paymentMethod: string
  enabled: boolean
  paymentInstructions: string
  paymentQrCodeDataUrl?: string
}

type BackendAPIService = {
  id: string
  ownerUserId?: string
  merchantProfileId?: string
  merchantIdentityMode: string
  merchantDisplayName?: string
  merchantProfileSlug?: string
  merchantAvatarUrl?: string
  ownerContactMethodId?: string
  title: string
  shortDescription: string
  sourceUrl?: string
  sourceAuthorVerification: {
    status: 'not_submitted' | 'pending' | 'verified' | 'mismatch' | 'expired'
    verifiedAt?: string
    expiresAt?: string
  }
  sellerReputation?: ReputationSummary | null
  distributionSystem: string
  billingMode: string
  declaredCnyPerUsdAllowance?: string
  declaredMaxUsdAllowancePerIntent?: string
  availableUsdAllowance?: string
  quotaExpiresAt?: string
  declaredTtftBand?: string
  declaredMaxConcurrency?: number
  performanceConfirmedAt?: string
  minimumIntentCny: string
  maximumIntentCny?: string
  usageVisibility: string
  publicAccessNote?: string
  merchantNote?: string
  merchantSupportNote?: string
  accountPoolType?: 'gpt_pro_20x' | 'gpt_pro_5x' | 'gpt_plus' | 'custom' | null
  accountPoolLabel?: string | null
  merchantRefundCommitment?: boolean
  merchantRefundPolicyVersion?: string
  reviewStatus?: string
  publicationStatus?: string
  moderationStatus?: string
  acceptingOrders?: boolean
  paymentWindowMinutes?: number
  acceptedPaymentMethods?: string[]
  paymentOptions?: BackendPaymentOption[]
  isOrderable?: boolean
  orderableReasons?: string[]
  accessModes: BackendAccessMode[]
  models: BackendServiceModel[]
  packages: BackendServicePackage[]
  completed30d?: number
  unresolvedDisputes?: number
  responseMedianMinutes?: number | null
  version: number
  createdAt: string
  updatedAt: string
}

type BackendOwnerAPIService = BackendAPIService & {
  salesSummary: BackendApiServiceSalesSummary
}

export type ApiServicePromotion = Omit<PublicApiServicePromotionList['items'][number], 'service'> & {
  service: ApiService
}

export type AdminApiServiceOption = Pick<GeneratedApiService, 'id' | 'title' | 'reviewStatus' | 'publicationStatus' | 'moderationStatus'> & {
  merchantDisplayName?: string
}

type ContactDisclosure = {
  side: string
  type: ContactMethodType
  label: string
  value: string
  maskedValue: string
}

export type BackendAPIPurchaseIntent = {
  id: string
  apiServiceId: string
  buyerUserId?: string
  ownerUserId?: string
  buyerContactMethodId?: string
  status: ApiPurchaseIntent['status']
  requestedCnyAmount: string
  requestedUsdAllowance?: string
  selectedAccessMode: string
  selectedPackageId?: string
  selectedPackageSnapshot?: string
  serviceVersionSnapshot: number
  serviceTitleSnapshot: string
  distributionSystemSnapshot: string
  billingModeSnapshot: string
  declaredCnyPerUsdAllowanceSnapshot?: string
  declaredMaxUsdAllowancePerIntentSnapshot?: string
  minimumIntentCnySnapshot: string
  maximumIntentCnySnapshot?: string
  pricingSnapshot: string
  buyerNote?: string
  contactedAt?: string | null
  buyerCancelledAt?: string | null
  buyerCancelReason?: string
  ownerClosedAt?: string | null
  ownerCloseReason?: string
  merchantContact?: ContactDisclosure | null
  buyerContact?: ContactDisclosure | null
  version: number
  createdAt: string
  updatedAt: string
}

type BackendAPIOrderDeliveryCredential = {
  deliveryKind: string
  apiBaseUrl?: string
  apiKey?: string
  panelLoginUrl?: string
  username?: string
  password?: string
  instructions?: string
  submittedAt: string
}

export type BackendAPIOrder = {
  id: string
  purchaseKind: string
  apiPurchaseIntentId: string
  apiServiceId: string
  buyerUserId?: string
  sellerUserId?: string
  buyerReputation?: ReputationSummary | null
  sellerReputation?: ReputationSummary | null
  status: string
  disputeStatus?: string
  serviceTitleSnapshot: string
  selectedPackageId?: string
  selectedPackageSnapshot?: string
  packageStockReserved?: boolean
  packageExpiresAt?: string | null
  amount: string
  requestedUsdAllowanceSnapshot?: string
  cnyPerUsdAllowanceSnapshot?: string
  pricingSnapshot?: string
  apiQuotaBatchId?: string
  apiQuotaOfferId?: string
  apiQuotaSaleRoundId?: string
  quotaOfferNameSnapshot?: string
  quotaUsdAllowanceSnapshot?: string
  quotaPriceCnySnapshot?: string
  quotaCnyPerUsdSnapshot?: string
  quotaModelMultiplierSnapshot?: string
  quotaSaleCutoffAtSnapshot?: string
  quotaExpiresAtSnapshot?: string
  quotaSaleModeSnapshot?: string
  quotaRoundStartsAtSnapshot?: string
  quotaRoundEndsAtSnapshot?: string
  quotaDistributionSystemSnapshot?: string
  quotaTtftBandSnapshot?: string
  quotaDeclaredMaxConcurrencySnapshot?: number
  quotaPerformanceConfirmedAtSnapshot?: string
  quotaPerformanceUnverifiedSnapshot?: boolean
  quotaDeliveryEtaMinutesSnapshot?: number
  quotaDeliveryModeSnapshot?: string
  currency: string
  selectedPaymentMethod: string
  paymentWindowMinutesSnapshot: number
  paymentExpiresAt: string
  paymentSummary?: string
  paymentSubmittedAt?: string | null
  paymentIssueReason?: string
  paymentIssueNote?: string
  paymentIssueReportedAt?: string | null
  paidConfirmedAt?: string | null
  deliveryNote?: string
  deliverySubmittedAt?: string | null
  deliveryCredential?: BackendAPIOrderDeliveryCredential | null
  completedAt?: string | null
  cancelledAt?: string | null
  cancelReason?: string
  version: number
  createdAt: string
  updatedAt: string
}

type BackendAPIOrderPaymentInstructions = {
  orderId: string
  paymentMethod: string
  paymentInstructions: string
  paymentQrCodeDataUrl?: string
  paymentExpiresAt: string
}

type BackendIntentPricingSnapshotModel = {
  modelNameSnapshot?: unknown
  merchantMultiplier?: unknown
}

type BackendIntentPricingSnapshot = {
  models?: unknown
  usageVisibility?: unknown
  merchantNote?: unknown
  merchantSupportNote?: unknown
  accountPoolType?: unknown
  accountPoolLabel?: unknown
  declaredMaxConcurrency?: unknown
  recommendedConcurrency?: unknown
  merchantRefundCommitment?: unknown
  merchantRefundPolicyVersion?: unknown
  serviceValidityExpiresAt?: unknown
}

export type APIIntentPricingSnapshotProjection = ApiServiceCommercialSnapshot & {
  models: string[]
  multiplier: string
  defaultMultiplier: number
  usageVisibility: ApiUsageVisibility
  usageVisibilitySnapshotMissing: boolean
  merchantNote: string
  merchantSupportNote: string
  issue?: 'missing' | 'invalid'
}

const legacyMerchantSupportNote = '历史订单未冻结商户售后说明'
const invalidMerchantSupportNote = '订单快照不可用，无法读取商户售后说明'
const apiOrderPlatformTradeBoundary = '售后由双方站外确认；平台不代收、不托管、不担保、不代赔。'
const commercialSnapshotKeys = [
  'accountPoolType',
  'accountPoolLabel',
  'declaredMaxConcurrency',
  'merchantRefundCommitment',
  'merchantRefundPolicyVersion',
  'serviceValidityExpiresAt',
] as const

type BackendAPIModel = {
  id: string
  providerCategory: string
  provider: string
  modelKey: string
  displayName: string
  capabilities: string[]
  inputPricePerMillion?: string
  cachedInputPricePerMillion?: string
  outputPricePerMillion?: string
}

function numberFromDecimal(value: string | undefined, fallback = 0) {
  if (!value) return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function nonEmptyString(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function apiAccountPoolType(value: unknown): ApiService['accountPoolType'] | undefined {
  if (value === 'gpt_pro_20x' || value === 'gpt_pro_5x' || value === 'gpt_plus' || value === 'custom') return value
  return undefined
}

function commercialSnapshotIssue(snapshot: BackendIntentPricingSnapshot): ApiServiceCommercialSnapshot['commercialFactsSnapshotIssue'] {
  if (!commercialSnapshotKeys.every(key => Object.prototype.hasOwnProperty.call(snapshot, key))) return 'missing'
  const validity = snapshot.serviceValidityExpiresAt
  const poolIsHistoricalNull = snapshot.accountPoolType === null && snapshot.accountPoolLabel === null
  const poolIsValid = Boolean(apiAccountPoolType(snapshot.accountPoolType) && nonEmptyString(snapshot.accountPoolLabel))
  const concurrencyIsValid = snapshot.declaredMaxConcurrency === null
    || (Number.isInteger(snapshot.declaredMaxConcurrency) && Number(snapshot.declaredMaxConcurrency) > 0)
  if (
    (!poolIsHistoricalNull && !poolIsValid)
    || !concurrencyIsValid
    || typeof snapshot.merchantRefundCommitment !== 'boolean'
    || !nonEmptyString(snapshot.merchantRefundPolicyVersion)
    || (validity !== null && !nonEmptyString(validity))
  ) {
    return 'invalid'
  }
  return undefined
}

function snapshotMultiplier(models: BackendIntentPricingSnapshotModel[]) {
  const values = [...new Set(models.map(model => nonEmptyString(model.merchantMultiplier)).filter(Boolean))]
  if (!values.length) return { label: '未冻结倍率信息', value: 1 }
  const normalized = values.map(value => numberFromDecimal(value, Number.NaN)).filter(Number.isFinite)
  if (normalized.length !== values.length) return { label: '订单倍率快照不可用', value: 1 }
  if (normalized.length > 1) return { label: '按模型分别计算', value: normalized[0] }
  return { label: `${normalized[0].toFixed(2)}x`, value: normalized[0] }
}

export function projectAPIIntentPricingSnapshot(value: string): APIIntentPricingSnapshotProjection {
  if (!value.trim()) {
    return {
      models: [],
      multiplier: '未冻结倍率信息',
      defaultMultiplier: 1,
      usageVisibility: 'none',
      usageVisibilitySnapshotMissing: true,
      merchantNote: '',
      merchantSupportNote: legacyMerchantSupportNote,
      commercialFactsSnapshotIssue: 'missing',
      issue: 'missing',
    }
  }

  let snapshot: BackendIntentPricingSnapshot
  try {
    const parsed = JSON.parse(value) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('invalid pricing snapshot')
    snapshot = parsed as BackendIntentPricingSnapshot
  } catch {
    return {
      models: [],
      multiplier: '订单倍率快照不可用',
      defaultMultiplier: 1,
      usageVisibility: 'none',
      usageVisibilitySnapshotMissing: true,
      merchantNote: '',
      merchantSupportNote: invalidMerchantSupportNote,
      commercialFactsSnapshotIssue: 'invalid',
      issue: 'invalid',
    }
  }

  const rawModels = Array.isArray(snapshot.models) ? snapshot.models : []
  const modelRows = rawModels.filter((model): model is BackendIntentPricingSnapshotModel => Boolean(model) && typeof model === 'object' && !Array.isArray(model))
  const models = [...new Set(modelRows.map(model => nonEmptyString(model.modelNameSnapshot)).filter(Boolean))]
  const multiplier = snapshotMultiplier(modelRows)
  const rawUsageVisibility = nonEmptyString(snapshot.usageVisibility)
  const accountPoolType = apiAccountPoolType(snapshot.accountPoolType)
  const accountPoolLabel = nonEmptyString(snapshot.accountPoolLabel) || undefined
  const frozenConcurrency = Object.prototype.hasOwnProperty.call(snapshot, 'declaredMaxConcurrency')
    ? snapshot.declaredMaxConcurrency
    : snapshot.recommendedConcurrency
  const rawConcurrency = Number(frozenConcurrency)
  const declaredMaxConcurrency = Number.isInteger(rawConcurrency) && rawConcurrency > 0 ? rawConcurrency : undefined
  const merchantRefundCommitment = typeof snapshot.merchantRefundCommitment === 'boolean' ? snapshot.merchantRefundCommitment : undefined
  const merchantRefundPolicyVersion = nonEmptyString(snapshot.merchantRefundPolicyVersion) || undefined
  const serviceValidityExpiresAt = snapshot.serviceValidityExpiresAt === null
    ? null
    : nonEmptyString(snapshot.serviceValidityExpiresAt) || undefined

  return {
    models,
    multiplier: multiplier.label,
    defaultMultiplier: multiplier.value,
    usageVisibility: usageVisibility(rawUsageVisibility),
    usageVisibilitySnapshotMissing: !rawUsageVisibility,
    merchantNote: nonEmptyString(snapshot.merchantNote),
    merchantSupportNote: nonEmptyString(snapshot.merchantSupportNote) || legacyMerchantSupportNote,
    accountPoolType,
    accountPoolLabel,
    declaredMaxConcurrency,
    merchantRefundCommitment,
    merchantRefundPolicyVersion,
    serviceValidityExpiresAt,
    commercialFactsSnapshotIssue: commercialSnapshotIssue(snapshot),
  }
}

export function merchantSupportNoteFromPublishPayload(value: unknown) {
  const warranty = value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
  const mode = nonEmptyString(warranty.mode)
  if (mode === 'merchant_full_refund') {
    return '商户退款承诺：订单服务有效期内，如未交付、实际号池/模型/额度与订单快照不符，或交付后连续不可用超过 1 小时且不属于排除情形，商户承诺退还订单全部实付金额。'
  }
  if (mode === 'upstream_refund_only') {
    const note = nonEmptyString(warranty.refundNote)
    return note ? `仅在上游退款后处理：${note}` : '仅在上游退款后处理。'
  }
  if (mode === 'merchant_warranty') {
    const days = Number(warranty.warrantyDays)
    const parts = [Number.isFinite(days) && days > 0 ? `商户承诺 ${days} 天` : '商户承诺']
    const coverage = nonEmptyString(warranty.coverage)
    const compensation = nonEmptyString(warranty.compensation)
    const exclusions = nonEmptyString(warranty.exclusions)
    if (coverage) parts.push(`适用范围：${coverage}`)
    if (compensation) parts.push(`补偿方式：${compensation}`)
    if (exclusions) parts.push(`不适用情形：${exclusions}`)
    return `${parts.join('；')}。`
  }
  return '无额外售后承诺，具体问题由双方站外协商。'
}

function apiTTFTBand(value?: string): ApiTTFTBand | undefined {
  if (value === 'under_1s' || value === '1_to_3s' || value === '3_to_5s' || value === '5_to_10s' || value === 'over_10s') return value
  if (!value) return undefined
  throw new Error(`Unsupported API TTFT band: ${value}`)
}

function deliveryMode(value: string): ApiDeliveryMode {
  return value === 'buyer_dedicated_panel_subaccount' || value === 'sub2api_panel_account' ? 'sub2api_panel_account' : 'api_key_endpoint'
}

function deliveryModes(modes: BackendAccessMode[]): ApiDeliveryMode[] {
  const rows = modes.map(item => deliveryMode(item.accessMode))
  return rows.length ? [...new Set(rows)] : ['api_key_endpoint']
}

function distributionLabel(value: string): ApiService['delivery'] {
  if (value === 'sub2api') return 'Sub2API'
  if (value === 'fixed_package') return '固定套餐'
  if (value === 'manual_usage_check') return '商户手工核对'
  return '其他'
}

function billingMode(value: string): ApiBillingMode {
  if (value === 'fixed_package') return 'fixed_package'
  if (value === 'manual_usage_check') return 'manual_credit'
  return 'metered_credit'
}

function usageVisibility(value: string): ApiUsageVisibility {
  if (value === 'offsite_panel_readonly' || value === 'panel_realtime') return 'panel_realtime'
  if (value === 'merchant_reported' || value === 'merchant_readonly') return 'merchant_readonly'
  return 'none'
}

function serviceState(service: BackendAPIService): ApiService['state'] {
  // 公开列表不返回审核/发布/治理状态，isOrderable 已是后端公开可接单契约。
  if (service.isOrderable) return 'online'
  if (service.moderationStatus === 'removed' || service.publicationStatus === 'archived') return 'offline'
  if (service.moderationStatus === 'admin_suspended' || service.publicationStatus === 'owner_paused') return 'paused'
  if (service.reviewStatus === 'pending_review') return 'reviewing'
  if (service.publicationStatus === 'online') return 'online'
  return 'offline'
}

function modelPriceRows(models: BackendServiceModel[]): ModelPriceRow[] {
  return models.filter(item => item.enabled).map(item => ({
    modelId: item.modelCatalogId,
    modelName: item.modelNameSnapshot,
    provider: item.providerSnapshot,
    officialInputPricePerMillion: numberFromDecimal(item.effectiveInputPricePerMillion),
    officialCachedInputPricePerMillion: item.effectiveCachedInputPricePerMillion ? numberFromDecimal(item.effectiveCachedInputPricePerMillion) : null,
    officialOutputPricePerMillion: numberFromDecimal(item.effectiveOutputPricePerMillion),
    merchantMultiplier: numberFromDecimal(item.merchantMultiplier, 1),
    actualInputPricePerMillion: numberFromDecimal(item.effectiveInputPricePerMillion),
    actualCachedInputPricePerMillion: item.effectiveCachedInputPricePerMillion ? numberFromDecimal(item.effectiveCachedInputPricePerMillion) : null,
    actualOutputPricePerMillion: numberFromDecimal(item.effectiveOutputPricePerMillion),
  }))
}

export function mapBackendAPIService(service: BackendAPIService): ApiService {
  const cnyPerUsd = numberFromDecimal(service.declaredCnyPerUsdAllowance, 1)
  const creditPerCny = cnyPerUsd > 0 ? Number((1 / cnyPerUsd).toFixed(4)) : 1
  const modes = deliveryModes(service.accessModes)
  const state = serviceState(service)
  const isStoreAlias = service.merchantIdentityMode === 'store_alias'
  const displayName = service.merchantDisplayName || (isStoreAlias ? 'API 商户' : '公开商户')
  const merchantUsername = service.merchantProfileSlug || (isStoreAlias ? service.merchantProfileId : service.ownerUserId) || 'merchant'
  const online = state === 'online'
  const publiclyOrderable = Boolean(service.isOrderable)
  const declaredTtftBand = apiTTFTBand(service.declaredTtftBand)
  const sellerReputation = mapBackendReputationSummary(service.sellerReputation)
  return {
    id: service.id,
    title: service.title.replace(/意向服务/g, '服务').replace(/API 意向/g, 'API 订单'),
    sourceUrl: service.sourceUrl ?? '',
    sourceAuthorVerification: service.sourceAuthorVerification,
    sellerReputation,
    merchantId: service.merchantProfileId ?? service.ownerUserId ?? 'merchant',
    merchantUsername,
    merchant: displayName,
    merchantIdentityMode: isStoreAlias ? 'store_alias' : 'public_profile',
    merchantDisplayName: displayName,
    merchantAvatarUrl: service.merchantAvatarUrl?.trim() || undefined,
    trustLevel: null,
    merchantType: '商户',
    models: service.models.filter(item => item.enabled).map(item => item.modelNameSnapshot),
    modelMultipliers: service.models.filter(item => item.enabled).map(item => ({ model: item.modelNameSnapshot, multiplier: `${numberFromDecimal(item.merchantMultiplier, 1).toFixed(2)}x` })),
    rate: `${numberFromDecimal(service.models[0]?.merchantMultiplier, 1).toFixed(2)}x`,
    defaultMultiplier: numberFromDecimal(service.models[0]?.merchantMultiplier, 1),
    creditPerCny,
    cnyPerUsdAllowance: service.declaredCnyPerUsdAllowance || '1.0000',
    availableUsdAllowance: service.availableUsdAllowance || service.declaredMaxUsdAllowancePerIntent || '0',
    maxUsdAllowancePerOrder: service.declaredMaxUsdAllowancePerIntent || service.availableUsdAllowance || '0',
    minimumPurchaseCny: numberFromDecimal(service.minimumIntentCny, 1),
    maxBuy: numberFromDecimal(service.maximumIntentCny, 999999),
    balance: numberFromDecimal(service.declaredMaxUsdAllowancePerIntent, 0),
    delivery: distributionLabel(service.distributionSystem),
    billingMode: billingMode(service.billingMode),
    deliveryModes: modes,
    usageVisibility: usageVisibility(service.usageVisibility),
    panelBaseUrl: null,
    imagePricing: {
      supported: service.models.some(item => item.capabilitiesSnapshot.includes('image_generation') || item.capabilitiesSnapshot.includes('image_edit')),
      textToImage: service.models.some(item => item.capabilitiesSnapshot.includes('image_generation')),
      imageToImage: service.models.some(item => item.capabilitiesSnapshot.includes('image_edit')),
      oneKPriceUsd: null,
      twoKPriceUsd: null,
      fourKPriceUsd: null,
    },
    independentApiKey: modes.includes('api_key_endpoint'),
    independentPanelAccount: modes.includes('sub2api_panel_account'),
    panelRequiresPasswordReset: modes.includes('sub2api_panel_account'),
    apiBaseUrlVisibility: 'after_intent',
    panelLoginUrlVisibility: modes.includes('sub2api_panel_account') ? 'after_intent' : 'off_platform',
    state,
    online,
    publiclyOrderable,
    lastOnlineConfirmedAt: service.updatedAt,
    onlineExpiresAt: service.quotaExpiresAt ?? service.updatedAt,
    declaredTtftBand,
    declaredMaxConcurrency: service.declaredMaxConcurrency,
    performanceConfirmedAt: service.performanceConfirmedAt,
    expectedResponseMinutes: service.paymentWindowMinutes ?? 10,
    responseMedianMinutes: service.responseMedianMinutes ?? null,
    dailyOrderLimit: 10,
    todayOrderCount: 0,
    unresolvedDisputes: service.unresolvedDisputes ?? sellerReputation?.unresolvedDisputes ?? null,
    warning: state === 'reviewing' ? '等待管理员审核' : online && !publiclyOrderable ? '待配置接单设置' : undefined,
    warranty: service.merchantSupportNote || '按商户备注站外协商，平台不担保、不代赔',
    refundPolicy: service.merchantRefundCommitment
      ? '订单有效期内符合商户退款承诺条件时，由商户退还全部实付金额；平台记录但不垫付、不代赔'
      : '无额外退款承诺，具体问题由双方站外协商；平台不处理支付或托管',
    accountPoolType: service.accountPoolType ?? undefined,
    accountPoolLabel: service.accountPoolLabel ?? undefined,
    merchantRefundCommitment: Boolean(service.merchantRefundCommitment),
    merchantRefundPolicyVersion: service.merchantRefundPolicyVersion,
    quotaExpiresAt: service.quotaExpiresAt,
    expiresAt: formatQuotaExpiresAtLabel(service.quotaExpiresAt) || '按服务说明',
    completed30d: service.completed30d ?? sellerReputation?.completedCount ?? null,
    reviewCount: sellerReputation?.verifiedReviewCount ?? null,
    officialPricingVersion: 'backend',
    officialPricingUpdatedAt: service.updatedAt,
    merchantNote: service.merchantNote || service.publicAccessNote || service.shortDescription,
    modelPriceRows: modelPriceRows(service.models),
    packages: (service.packages ?? []).map(item => ({
      id: item.id ?? '',
      name: item.name,
      priceCny: numberFromDecimal(item.priceCny),
      panelAllowance: numberFromDecimal(item.panelAllowance),
      durationDays: item.durationDays as 1 | 3 | 7 | 30,
      stockTotal: item.stockTotal,
      stockAvailable: item.stockAvailable,
      description: item.description,
      enabled: item.enabled,
      sortOrder: item.sortOrder,
      models: (item.models ?? []).map(model => ({
        serviceModelId: model.serviceModelId,
        modelCatalogId: model.modelCatalogId,
        modelPriceVersionId: model.modelPriceVersionId ?? '',
        modelName: model.modelNameSnapshot,
        provider: model.providerSnapshot,
        merchantMultiplier: numberFromDecimal(model.merchantMultiplier, 1),
      })),
    })),
    recommendationResponseMedianMinutes: service.responseMedianMinutes ?? null,
    serviceUpdatedAt: service.updatedAt,
    contactChannels: [],
    acceptedPaymentMethods: (service.acceptedPaymentMethods ?? []).filter(isApiPaymentMethod),
  }
}

function mapBackendPublicAPIService(service: PublicApiService): ApiService {
  return mapBackendAPIService(service)
}

function mapBackendAPIServiceSalesChannel(channel: BackendApiServiceSalesChannel): ApiServiceSalesChannel {
  return {
    kind: channel.kind,
    state: channel.state,
    availableUsdAllowance: channel.availableUsdAllowance,
    availableCopies: channel.availableCopies,
    nextStartsAt: channel.nextStartsAt,
    saleCutoffAt: channel.saleCutoffAt,
    expiresAt: channel.expiresAt,
  }
}

export function mapBackendOwnerAPIService(service: BackendOwnerAPIService): OwnerApiService {
  return {
    ...mapBackendAPIService(service),
    salesSummary: {
      overallState: service.salesSummary.overallState,
      channels: service.salesSummary.channels.map(mapBackendAPIServiceSalesChannel),
    },
  }
}

function filterServices(rows: ApiService[], filters: ApiServiceFilters | Sub2ApiMarketFilters | OtherApiMarketFilters = {}) {
  const search = 'search' in filters ? filters.search?.trim().toLowerCase() : undefined
  return rows.filter(row => {
    if (search && ![row.title, row.merchant, row.merchantDisplayName, ...row.models].some(value => value.toLowerCase().includes(search))) return false
    if ('deliveryMode' in filters && filters.deliveryMode && !row.deliveryModes.includes(filters.deliveryMode)) return false
    if ('online' in filters && filters.online !== undefined && row.publiclyOrderable !== filters.online) return false
    return true
  })
}

function mapAPIQuotaAllocation(item: ApiQuotaRound['allocations'][number]): ApiQuotaRound['allocations'][number] {
  return {
    id: item.id,
    offerId: item.offerId,
    saleRoundId: item.saleRoundId,
    saleMode: item.saleMode,
    copyLimit: item.copyLimit,
    availableCopies: item.availableCopies,
    reservedCopies: item.reservedCopies,
    consumedCopies: item.consumedCopies,
    allocatedUsdAllowance: item.allocatedUsdAllowance,
    returnedUsdAllowance: item.returnedUsdAllowance,
    status: item.status,
  }
}

function mapAPIQuotaRound(item: ApiQuotaRound): ApiQuotaRound {
  return {
    id: item.id,
    batchId: item.batchId,
    systemSlotKey: item.systemSlotKey,
    name: item.name,
    startsAt: item.startsAt,
    endsAt: item.endsAt,
    status: item.status,
    allocations: item.allocations.map(mapAPIQuotaAllocation),
    version: item.version,
  }
}

function mapAPIQuotaOffer(item: ApiQuotaOffer): ApiQuotaOffer {
  return {
    id: item.id,
    batchId: item.batchId,
    apiServiceId: item.apiServiceId,
    distributionSystem: item.distributionSystem,
    name: item.name,
    usdAllowance: item.usdAllowance,
    priceCny: item.priceCny,
    cnyPerUsd: item.cnyPerUsd,
    modelMultiplier: item.modelMultiplier,
    deliveryMode: item.deliveryMode,
    deliveryEtaMinutes: item.deliveryEtaMinutes,
    saleMode: item.saleMode,
    status: item.status,
    sortOrder: item.sortOrder,
    publishedAt: item.publishedAt,
    version: item.version,
  }
}

export function mapBackendPublicAPIQuotaOffer(item: PublicApiQuotaOffer): PublicApiQuotaOffer {
  return {
    ...mapAPIQuotaOffer(item),
    batchStatus: item.batchStatus,
    serviceTitle: item.serviceTitle,
    sellerDisplayName: item.sellerDisplayName,
    sellerIdentityType: item.sellerIdentityType,
    sellerLinuxDoBound: item.sellerLinuxDoBound,
    declaredTtftBand: item.declaredTtftBand,
    declaredMaxConcurrency: item.declaredMaxConcurrency,
    performanceConfirmedAt: item.performanceConfirmedAt,
    performanceDisclaimer: item.performanceDisclaimer,
    saleCutoffAt: item.saleCutoffAt,
    expiresAt: item.expiresAt,
    currentRound: item.currentRound ? mapAPIQuotaRound(item.currentRound) : undefined,
    nextRound: item.nextRound ? mapAPIQuotaRound(item.nextRound) : undefined,
    availableCopies: item.availableCopies,
    credentialAvailableCopies: item.credentialAvailableCopies,
    isOrderable: item.isOrderable,
    orderabilityCode: item.orderabilityCode,
    orderabilityReason: item.orderabilityReason,
  }
}

function mapAPIQuotaBatch(item: ApiQuotaBatch): ApiQuotaBatch {
  return {
    id: item.id,
    apiServiceId: item.apiServiceId,
    sourceType: item.sourceType,
    sourceLabel: item.sourceLabel,
    status: item.status,
    declaredTotalUsdAllowance: item.declaredTotalUsdAllowance,
    unallocatedUsdAllowance: item.unallocatedUsdAllowance,
    saleCutoffAt: item.saleCutoffAt,
    expiresAt: item.expiresAt,
    sourceConfirmedAt: item.sourceConfirmedAt,
    publishedAt: item.publishedAt,
    version: item.version,
  }
}

function quotaOfferQuery(filters: ApiQuotaOfferFilters) {
  const params = new URLSearchParams()
  if (filters.distributionSystem && filters.distributionSystem !== 'all') params.set('distributionSystem', filters.distributionSystem)
  if (filters.oneMultiplier) params.set('oneMultiplier', 'true')
  if (filters.onlyOrderable) params.set('onlyOrderable', 'true')
  if (filters.slotKey) params.set('slotKey', filters.slotKey)
  const query = params.toString()
  return query ? `?${query}` : ''
}

export async function backendPublicAPIQuotaOffers(filters: ApiQuotaOfferFilters = {}) {
  const response = await backendRequest<ListResponse<PublicApiQuotaOffer>>(`/api/v1/api-quota-offers${quotaOfferQuery(filters)}`)
  return response.items.map(mapBackendPublicAPIQuotaOffer)
}

export async function backendAPIQuotaSaleSlots() {
  return backendRequest<ApiQuotaSystemSaleSlotList>('/api/v1/api-quota-sale-slots')
}

export async function backendPublicAPIQuotaOffer(id: string) {
  return mapBackendPublicAPIQuotaOffer(await backendRequest<PublicApiQuotaOffer>(`/api/v1/api-quota-offers/${id}`))
}

export async function backendCreateAPIQuotaOrder(payload: CreateApiQuotaOrderPayload) {
  await ensureBackendSession('buyer', false)
  const service = await backendAPIServiceById((await backendPublicAPIQuotaOffer(payload.offerId)).apiServiceId)
  const paymentMethod = service.acceptedPaymentMethods?.[0]
  const selectedAccessMode = service.deliveryModes[0]
  if (!paymentMethod) throw new Error('商户尚未配置可用的微信或支付宝收款方式。')
  if (!selectedAccessMode) throw new Error('商户尚未配置可用接入方式。')
  const contact = await backendCreateContactMethod({
    type: 'linuxdo',
    label: 'linux.do 私信',
    displayValue: '@buyer',
    usageScopes: ['buyer'],
    isDefault: true,
    enabled: true,
  })
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/api-quota-offers/${payload.offerId}/orders`, {
    ...(payload.saleRoundId ? { saleRoundId: payload.saleRoundId } : {}),
    buyerContactMethodId: contact.id,
    selectedAccessMode: toBackendAccessMode(selectedAccessMode),
    paymentMethod,
    buyerNote: '',
  }, { idempotencyPrefix: 'api-quota-order-create' })
  return mapBackendAPIOrder(response, 'buyer')
}

export async function backendOwnerAPIQuotaBatches(apiServiceId: string) {
  const response = await backendRequest<ListResponse<ApiQuotaBatch>>(`/api/v1/owner/api-services/${apiServiceId}/quota-batches`)
  return response.items.map(mapAPIQuotaBatch)
}

export async function backendCreateAPIQuotaBatch(payload: CreateApiQuotaBatchPayload) {
  const response = await backendMutation<ApiQuotaBatch>(`/api/v1/owner/api-services/${payload.apiServiceId}/quota-batches`, {
    sourceType: payload.sourceType,
    sourceLabel: payload.sourceLabel ?? '',
    declaredTotalUsdAllowance: payload.declaredTotalUsdAllowance,
    saleCutoffAt: payload.saleCutoffAt,
    expiresAt: payload.expiresAt,
    sourceConfirmedAt: payload.sourceConfirmedAt,
  }, { idempotencyPrefix: 'api-quota-batch-create' })
  return mapAPIQuotaBatch(response)
}

export async function backendOwnerAPIQuotaOffers(batchId: string) {
  const response = await backendRequest<ApiQuotaOffer[]>(`/api/v1/owner/api-quota-batches/${batchId}/offers`)
  return response.map(mapAPIQuotaOffer)
}

export async function backendCreateAPIQuotaOffer(payload: CreateApiQuotaOfferPayload) {
  const response = await backendMutation<ApiQuotaOffer>(`/api/v1/owner/api-quota-batches/${payload.batchId}/offers`, {
    name: payload.name,
    usdAllowance: payload.usdAllowance,
    priceCny: payload.priceCny,
    modelMultiplier: payload.modelMultiplier,
    deliveryMode: payload.deliveryMode,
    deliveryEtaMinutes: payload.deliveryEtaMinutes,
    saleMode: payload.saleMode,
    continuousCopies: payload.continuousCopies,
    sortOrder: payload.sortOrder,
  }, { idempotencyPrefix: 'api-quota-offer-create' })
  return mapAPIQuotaOffer(response)
}

export async function backendCreateAPIQuotaRushOffer(payload: CreateApiQuotaRushOfferPayload) {
  const form = new FormData()
  const { apiServiceId, file, ...request } = payload
  form.set('payload', JSON.stringify(request))
  if (file) form.set('file', file)
  const response = await backendFormDataMutation<ApiQuotaRushOfferPublication>(
    `/api/v1/owner/api-services/${apiServiceId}/quota-rush-offers`,
    form,
    { idempotencyPrefix: 'api-quota-rush-offer-create' },
  )
  return {
    batch: mapAPIQuotaBatch(response.batch),
    offer: mapAPIQuotaOffer(response.offer),
    round: mapAPIQuotaRound(response.round),
    credentialImported: response.credentialImported,
    credentialSummary: response.credentialSummary,
  }
}

export async function backendOwnerAPIQuotaRounds(batchId: string) {
  const response = await backendRequest<ApiQuotaRound[]>(`/api/v1/owner/api-quota-batches/${batchId}/rounds`)
  return response.map(mapAPIQuotaRound)
}

export async function backendCreateAPIQuotaRound(payload: CreateApiQuotaRoundPayload) {
  const response = await backendMutation<ApiQuotaRound>(`/api/v1/owner/api-quota-batches/${payload.batchId}/rounds`, {
    name: payload.name,
    startsAt: payload.startsAt,
    endsAt: payload.endsAt,
    offers: payload.offers,
  }, { idempotencyPrefix: 'api-quota-round-create' })
  return mapAPIQuotaRound(response)
}

export async function backendAPIQuotaBatchAction(batchId: string, action: 'publish' | 'pause' | 'resume' | 'archive', version: number) {
  const response = await backendMutation<ApiQuotaBatch>(`/api/v1/owner/api-quota-batches/${batchId}/${action}`, {}, {
    idempotencyPrefix: `api-quota-batch-${action}`,
    ifMatch: version,
  })
  return mapAPIQuotaBatch(response)
}

export async function backendAPIQuotaCredentialSummary(offerId: string) {
  return backendRequest<ApiQuotaCredentialSummary>(`/api/v1/owner/api-quota-offers/${offerId}/credentials/summary`)
}

export async function backendImportAPIQuotaCredentials(offerId: string, deliveryKind: ApiOrderDeliveryCredential['deliveryKind'], file: File) {
  const form = new FormData()
  form.set('deliveryKind', deliveryKind)
  form.set('file', file)
  return backendFormDataMutation<{ imported: number, summary: ApiQuotaCredentialSummary }>(`/api/v1/owner/api-quota-offers/${offerId}/credentials/import`, form, {
    idempotencyPrefix: 'api-quota-credential-import',
  })
}

export async function backendAPIServices(filters: ApiServiceFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIService>>('/api/v1/api-services')
  return filterServices(response.items.map(mapBackendAPIService).filter(row => row.publiclyOrderable), filters)
}

export async function backendPublicAPIPromotions(): Promise<ApiServicePromotion[]> {
  const response = await backendRequest<PublicApiServicePromotionList>('/api/v1/api-service-promotions?placement=api_market_top')
  return response.items.map(item => ({
    ...item,
    service: mapBackendPublicAPIService(item.service),
  }))
}

export async function backendAdminAPIPromotions(): Promise<AdminApiServicePromotion[]> {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<AdminApiServicePromotionList>('/api/v1/admin/api-service-promotions?limit=100')
  return response.items
}

export async function backendAdminAPIServiceOptions(): Promise<AdminApiServiceOption[]> {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<GeneratedApiService>>('/api/v1/admin/api-services?limit=100')
  return response.items.map(item => ({
    id: item.id,
    title: item.title,
    reviewStatus: item.reviewStatus,
    publicationStatus: item.publicationStatus,
    moderationStatus: item.moderationStatus,
    merchantDisplayName: item.merchantDisplayName,
  }))
}

export async function backendAPIPromotionAvailability(apiServiceId: string, startsAt: string, endsAt: string) {
  await ensureBackendSession('admin', true)
  const params = new URLSearchParams({
    apiServiceId,
    placement: 'api_market_top',
    startsAt,
    endsAt,
  })
  return backendRequest<ApiServicePromotionAvailability>(`/api/v1/admin/api-service-promotions/availability?${params.toString()}`)
}

export async function backendCreateAPIPromotion(payload: CreateApiServicePromotionRequest) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminApiServicePromotion>('/api/v1/admin/api-service-promotions', payload, {
    idempotencyPrefix: 'api-service-promotion-create',
  })
}

export async function backendStopAPIPromotion(id: string, version: number, reason: string) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminApiServicePromotion>(`/api/v1/admin/api-service-promotions/${encodeURIComponent(id)}/stop`, { reason }, {
    idempotencyPrefix: 'api-service-promotion-stop',
    ifMatch: version,
  })
}

export async function backendSub2APIServices(filters: Sub2ApiMarketFilters = {}) {
  const rows = await backendAPIServices({})
  return filterServices(rows.filter(row => row.delivery === 'Sub2API'), filters)
}

export async function backendOtherAPIServices(filters: OtherApiMarketFilters = {}) {
  const rows = await backendAPIServices({})
  return filterServices(rows.filter(row => row.delivery !== 'Sub2API'), filters)
}

export async function backendAPIServiceById(id: string) {
  const service = await backendRequest<BackendAPIService>(`/api/v1/api-services/${id}`)
  return mapBackendAPIService(service)
}

export async function backendOwnerAPIServices(salesView: ApiServiceSalesView = 'active') {
  await ensureBackendSession('merchant', false)
  const params = new URLSearchParams({ salesView })
  const response = await backendRequest<ListResponse<BackendOwnerAPIService>>(`/api/v1/owner/api-services?${params.toString()}`)
  return response.items.map(mapBackendOwnerAPIService)
}

export async function backendOwnerAPIServiceById(id: string) {
  await ensureBackendSession('merchant', false)
  const service = await backendRequest<BackendAPIService>(`/api/v1/owner/api-services/${encodeURIComponent(id)}`)
  return mapBackendAPIService(service)
}

function providerFromBackend(value: string): ModelCatalogItem['provider'] {
  const normalized = value.trim().toLowerCase()
  if (normalized === 'openai' || normalized === 'gpt') return 'openai'
  if (normalized === 'anthropic' || normalized === 'claude') return 'anthropic'
  return 'other'
}

function capabilitiesFromBackend(values: string[]): ModelCatalogItem['capabilities'] {
  const capabilities = new Set<ModelCatalogItem['capabilities'][number]>()
  for (const value of values) {
    if (value === 'text' || value === 'chat') capabilities.add('chat')
    if (value === 'vision') capabilities.add('vision')
    if (value === 'image_generation') capabilities.add('image_generation')
    if (value === 'image_edit') capabilities.add('image_edit')
    if (value === 'reasoning') capabilities.add('reasoning')
  }
  return capabilities.size ? [...capabilities] : ['chat']
}

function mapBackendModel(model: BackendAPIModel): ModelCatalogItem {
  return {
    id: model.id,
    provider: providerFromBackend(model.providerCategory || model.provider),
    name: model.modelKey,
    displayName: model.displayName,
    capabilities: capabilitiesFromBackend(model.capabilities),
    officialInputPricePerMillion: model.inputPricePerMillion ? numberFromDecimal(model.inputPricePerMillion) : null,
    officialCachedInputPricePerMillion: model.cachedInputPricePerMillion ? numberFromDecimal(model.cachedInputPricePerMillion) : null,
    officialOutputPricePerMillion: model.outputPricePerMillion ? numberFromDecimal(model.outputPricePerMillion) : null,
    active: true,
  }
}

export async function backendModelCatalog() {
  const response = await backendRequest<ListResponse<BackendAPIModel>>('/api/v1/api-models')
  return response.items.map(mapBackendModel)
}

function contactToChannel(contact?: ContactDisclosure | null) {
  if (!contact) return []
  return [{ type: contact.type, label: contact.label, value: contact.value }]
}

type ApiIntentViewerRole = 'buyer' | 'merchant'

function parsePackageSnapshot(value?: string): ApiServicePackageSnapshot | undefined {
  if (!value) return undefined
  try {
    const source = JSON.parse(value) as Record<string, unknown>
    const durationDays = Number(source.durationDays)
    if (![1, 3, 7, 30].includes(durationDays)) return undefined
    const rawModels = Array.isArray(source.models) ? source.models as Array<Record<string, unknown>> : []
    return {
      id: String(source.id ?? ''),
      name: String(source.name ?? ''),
      priceCny: numberFromDecimal(String(source.priceCny ?? '0')),
      panelAllowance: numberFromDecimal(String(source.panelAllowance ?? '0')),
      durationDays: durationDays as 1 | 3 | 7 | 30,
      description: String(source.description ?? ''),
      models: rawModels.map(model => ({
        serviceModelId: String(model.serviceModelId ?? ''),
        modelCatalogId: String(model.modelCatalogId ?? ''),
        modelPriceVersionId: String(model.modelPriceVersionId ?? ''),
        modelName: String(model.modelNameSnapshot ?? model.modelName ?? ''),
        merchantMultiplier: numberFromDecimal(String(model.merchantMultiplier ?? '1')),
      })),
    }
  } catch {
    return undefined
  }
}

function mapIntent(intent: BackendAPIPurchaseIntent, viewerRole: ApiIntentViewerRole): ApiPurchaseIntent {
  const amount = numberFromDecimal(intent.requestedCnyAmount)
  const credit = numberFromDecimal(intent.requestedUsdAllowance)
  const mode = deliveryMode(intent.selectedAccessMode)
  const merchantName = 'API 商户'
  const pricingSnapshot = projectAPIIntentPricingSnapshot(intent.pricingSnapshot)
  return {
    id: intent.id,
    serviceId: intent.apiServiceId,
    version: intent.version,
    buyerId: intent.buyerUserId ?? 'buyer',
    buyer: intent.buyerUserId ? `买家 ${intent.buyerUserId.slice(0, 8)}` : '买家',
    merchantId: intent.ownerUserId ?? 'merchant',
    merchant: merchantName,
    status: intent.status,
    selectedDeliveryMode: mode,
    selectedPackageId: intent.selectedPackageId,
    purchaseAmountCny: amount,
    purchasedCredit: credit,
    purchaseAmountCnyDecimal: intent.requestedCnyAmount,
    purchasedCreditDecimal: intent.requestedUsdAllowance || '0',
    targetModel: pricingSnapshot.models[0] ?? '',
    buyerNote: intent.buyerNote,
    snapshot: {
      serviceId: intent.apiServiceId,
      serviceTitle: intent.serviceTitleSnapshot,
      merchantId: intent.ownerUserId ?? 'merchant',
      merchant: merchantName,
      merchantUsername: intent.ownerUserId ?? 'merchant',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: merchantName,
      trustLevel: null,
      merchantType: '商户',
      models: pricingSnapshot.models,
      multiplier: pricingSnapshot.multiplier,
      defaultMultiplier: pricingSnapshot.defaultMultiplier,
      creditPerCny: amount > 0 && credit > 0 ? Number((credit / amount).toFixed(4)) : 1,
      cnyPerUsdAllowance: intent.declaredCnyPerUsdAllowanceSnapshot || '1.0000',
      warranty: pricingSnapshot.merchantSupportNote,
      refundPolicy: apiOrderPlatformTradeBoundary,
      merchantNote: pricingSnapshot.merchantNote,
      pricingSnapshotIssue: pricingSnapshot.issue,
      usageVisibilitySnapshotMissing: pricingSnapshot.usageVisibilitySnapshotMissing,
      usageVisibility: pricingSnapshot.usageVisibility,
      accountPoolType: pricingSnapshot.accountPoolType,
      accountPoolLabel: pricingSnapshot.accountPoolLabel,
      declaredMaxConcurrency: pricingSnapshot.declaredMaxConcurrency,
      merchantRefundCommitment: pricingSnapshot.merchantRefundCommitment,
      merchantRefundPolicyVersion: pricingSnapshot.merchantRefundPolicyVersion,
      serviceValidityExpiresAt: pricingSnapshot.serviceValidityExpiresAt,
      commercialFactsSnapshotIssue: pricingSnapshot.commercialFactsSnapshotIssue,
      supportedDeliveryModes: [mode],
      selectedDeliveryMode: mode,
      selectedPackageId: intent.selectedPackageId,
      selectedPackageSnapshot: parsePackageSnapshot(intent.selectedPackageSnapshot),
      minimumPurchaseCny: numberFromDecimal(intent.minimumIntentCnySnapshot, 1),
      panelBaseUrl: null,
      apiBaseUrlVisibility: 'after_intent',
      panelLoginUrlVisibility: 'off_platform',
      panelRequiresPasswordReset: mode === 'sub2api_panel_account',
      expiresAt: '按服务说明',
      officialPricingVersion: 'backend',
      officialPricingUpdatedAt: intent.updatedAt,
      modelPrices: [],
    },
    handoff: {
      intentId: intent.id,
      selectedDeliveryMode: mode,
      status: intent.status === 'contacted' ? 'contacted' : ['ordered', 'buyer_cancelled', 'owner_closed'].includes(intent.status) ? 'closed' : 'not_started',
      requiresFirstLoginPasswordReset: mode === 'sub2api_panel_account',
      note: '真实后端购买意向记录',
    },
    contactChannels: contactToChannel(intent.merchantContact),
    buyerContactChannels: contactToChannel(intent.buyerContact),
    viewerRole,
    createdAt: intent.createdAt,
    updatedAt: intent.updatedAt,
    buyerCancelledAt: intent.buyerCancelledAt ?? undefined,
    buyerCancelReason: intent.buyerCancelReason,
    ownerClosedAt: intent.ownerClosedAt ?? undefined,
    ownerCloseReason: intent.ownerCloseReason,
  }
}

function sortIntents(rows: ApiPurchaseIntent[], filters: ApiPurchaseIntentFilters = {}) {
  const search = filters.search?.trim().toLowerCase()
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  return rows.filter(row => {
    return (!statuses || statuses.includes(row.status))
      && (!filters.deliveryMode || row.selectedDeliveryMode === filters.deliveryMode)
      && (!filters.serviceId || row.serviceId === filters.serviceId)
      && (!search || [row.id, row.snapshot.serviceTitle, row.merchant, row.buyer].some(value => value.toLowerCase().includes(search)))
  }).sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
}

export async function backendMyAPIIntents(filters: ApiPurchaseIntentFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIPurchaseIntent>>('/api/v1/me/api-purchase-intents')
  return sortIntents(response.items.map(item => mapIntent(item, 'buyer')), filters)
}

export async function backendOwnerAPIIntents(filters: ApiPurchaseIntentFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIPurchaseIntent>>('/api/v1/owner/api-purchase-intents')
  return sortIntents(response.items.map(item => mapIntent(item, 'merchant')), filters)
}

function adminIntentStatusLabel(value: BackendAPIPurchaseIntent['status']) {
  const labels: Record<BackendAPIPurchaseIntent['status'], string> = {
    open: '待处理',
    contacted: '已联系',
    ordered: '已下单',
    buyer_cancelled: '买家已取消',
    owner_closed: '商户已关闭',
  }
  return labels[value]
}

export function mapBackendAdminAPIIntent(item: BackendAPIPurchaseIntent): AdminRow {
  return {
    id: item.id,
    primary: `${item.serviceTitleSnapshot} 购买意向`,
    secondary: `${item.id} · 意向金额 ¥${numberFromDecimal(item.requestedCnyAmount)}`,
    owner: `买家 ${item.buyerUserId?.slice(0, 8) ?? '未知'} / 商户 ${item.ownerUserId?.slice(0, 8) ?? '未知'}`,
    status: adminIntentStatusLabel(item.status),
    risk: item.ownerCloseReason || item.buyerCancelReason || `更新于 ${item.updatedAt}`,
    targetType: 'api-intent',
    backendKind: 'api-purchase-intent',
    backendVersion: item.version,
    detailItems: [
      { label: '后端状态', value: item.status },
      { label: '服务', value: item.serviceTitleSnapshot },
      { label: '意向金额', value: `¥${numberFromDecimal(item.requestedCnyAmount)}` },
      { label: '接入方式', value: item.selectedAccessMode },
      { label: '最近更新', value: item.updatedAt },
    ],
  }
}

export async function backendAdminAPIIntentRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendAPIPurchaseIntent>>('/api/v1/admin/api-purchase-intents')
  return response.items.map(mapBackendAdminAPIIntent)
}

function adminOrderStatusLabel(value: string) {
	const labels: Record<string, string> = {
		pending_payment: '待买家付款',
		payment_submitted: '待确认收款',
		paid_confirmed: '待商户交付',
		delivery_submitted: '待买家验收',
		completed: '已完成',
		cancelled: '已取消',
	}
	return labels[value] ?? value
}

export async function backendAdminAPIOrderRows(): Promise<AdminRow[]> {
	await ensureBackendSession('admin', true)
	const response = await backendRequest<ListResponse<BackendAPIOrder>>('/api/v1/admin/api-orders')
	return response.items.map(mapBackendAdminAPIOrder)
}

export function mapBackendAdminAPIOrder(order: BackendAPIOrder): AdminRow {
	return {
		id: order.id,
		primary: `${order.serviceTitleSnapshot} API 订单`,
		secondary: `${order.id} · 订单金额 ¥${order.amount}`,
		owner: `买家 ${order.buyerUserId?.slice(0, 8) ?? '未知'} / 商户 ${order.sellerUserId?.slice(0, 8) ?? '未知'}`,
		status: adminOrderStatusLabel(order.status),
		risk: order.disputeStatus || order.cancelReason || `更新于 ${order.updatedAt}`,
		targetType: 'api-order',
		backendKind: 'api-order',
		backendVersion: order.version,
		targetTo: null,
		detailItems: [
			{ label: '订单状态', value: order.status },
			{ label: '订单金额', value: `¥${order.amount}` },
			{ label: '购买额度', value: order.requestedUsdAllowanceSnapshot ? `${order.requestedUsdAllowanceSnapshot} 美元额度` : '不适用' },
			{ label: '定价快照', value: order.cnyPerUsdAllowanceSnapshot ? `¥${order.cnyPerUsdAllowanceSnapshot} / $1` : '按套餐快照' },
			{ label: '交付凭证', value: order.deliverySubmittedAt ? '已提交（管理摘要不展示原始凭证）' : '尚未提交' },
			{ label: '最近更新', value: order.updatedAt },
		],
	}
}

export async function backendAPIIntentById(id: string) {
  try {
    return mapIntent(await backendRequest<BackendAPIPurchaseIntent>(`/api/v1/me/api-purchase-intents/${id}`), 'buyer')
  } catch {
    return mapIntent(await backendRequest<BackendAPIPurchaseIntent>(`/api/v1/owner/api-purchase-intents/${id}`), 'merchant')
  }
}

export async function backendAPIIntentEvents(id: string): Promise<ApiPurchaseIntentEvent[]> {
  const intent = await backendAPIIntentById(id)
  return [{
    id: `backend-api-event-${intent.id}`,
    intentId: intent.id,
    actorId: intent.buyerId,
    actorLabel: intent.buyer,
    actorRole: 'buyer',
    type: 'intent_created',
    toStatus: 'open',
    createdAt: intent.createdAt,
  }]
}

export async function backendCreateContactMethod(payload: SaveContactMethodRequest): Promise<UserContactMethod> {
  const response = await backendMutation<{
    id: string
    type: ContactMethodType
    label: string
    maskedValue: string
    createdAt: string
  }>('/api/v1/contact-methods', {
    type: payload.type,
    label: payload.label,
    value: payload.displayValue,
  }, { idempotencyPrefix: 'contact-method' })
  return {
    id: response.id,
    userId: 'backend',
    type: response.type,
    label: response.label,
    maskedValue: response.maskedValue,
    displayValue: payload.displayValue,
    usageScopes: payload.usageScopes,
    isDefault: payload.isDefault,
    enabled: payload.enabled,
    verified: false,
    createdAt: response.createdAt,
    updatedAt: response.createdAt,
  }
}

export async function backendCreateAPIPurchaseIntent(payload: CreateApiPurchaseIntentPayload) {
  await ensureBackendSession('buyer', false)
  const service = await backendAPIServiceById(payload.serviceId)
  const contact = await backendCreateContactMethod({
    type: 'linuxdo',
    label: 'linux.do 私信',
    displayValue: '@buyer',
    usageScopes: ['buyer'],
    isDefault: true,
    enabled: true,
  })
  const requestedCnyAmount = normalizeDecimal(String(payload.purchaseAmountCny), 2)
  const requestedUsdAllowance = service.billingMode === 'fixed_package'
    ? ''
    : normalizeDecimalTrimmed(divideDecimal(requestedCnyAmount, service.cnyPerUsdAllowance || '1', 6), 6)
  const response = await backendMutation<BackendAPIPurchaseIntent>(`/api/v1/api-services/${payload.serviceId}/purchase-intents`, {
    buyerContactMethodId: contact.id,
    requestedCnyAmount,
    requestedUsdAllowance,
    selectedAccessMode: service.billingMode === 'fixed_package' ? 'fixed_package_offsite' : toBackendAccessMode(payload.deliveryMode),
    selectedPackageId: payload.selectedPackageId ?? '',
    buyerNote: payload.buyerNote ?? '',
  }, { idempotencyPrefix: 'api-intent' })
  return mapIntent(response, 'buyer')
}

export async function backendCancelAPIIntent(intent: ApiPurchaseIntent, reason: string) {
  const response = await backendMutation<BackendAPIPurchaseIntent>(`/api/v1/me/api-purchase-intents/${intent.id}/cancel`, { reason }, {
    idempotencyPrefix: 'api-intent-cancel',
    ifMatch: intent.version,
  })
  return mapIntent(response, 'buyer')
}

export async function backendCancelAPIIntentById(id: string, reason: string) {
  const intent = await backendAPIIntentById(id)
  return backendCancelAPIIntent(intent, reason)
}

export async function backendMarkAPIIntentContacted(id: string) {
  const intent = await backendRequest<BackendAPIPurchaseIntent>(`/api/v1/owner/api-purchase-intents/${id}`)
  const response = await backendMutation<BackendAPIPurchaseIntent>(`/api/v1/owner/api-purchase-intents/${id}/mark-contacted`, {}, {
    idempotencyPrefix: 'api-intent-contacted',
    ifMatch: intent.version,
  })
  return mapIntent(response, 'merchant')
}

export async function backendCloseAPIIntent(id: string, reason: string) {
  const intent = await backendRequest<BackendAPIPurchaseIntent>(`/api/v1/owner/api-purchase-intents/${id}`)
  const response = await backendMutation<BackendAPIPurchaseIntent>(`/api/v1/owner/api-purchase-intents/${id}/close`, { reason }, {
    idempotencyPrefix: 'api-intent-close',
    ifMatch: intent.version,
  })
  return mapIntent(response, 'merchant')
}

function apiOrderStatus(value: string): ApiOrderStatus {
  if (
    value === 'pending_payment'
	    || value === 'payment_submitted'
	    || value === 'payment_issue'
    || value === 'paid_confirmed'
    || value === 'delivery_submitted'
    || value === 'completed'
    || value === 'cancelled'
  ) {
    return value
  }
  throw new Error(`Unsupported API order status: ${value}`)
}

function apiOrderPurchaseKind(value: string): ApiOrder['purchaseKind'] {
  if (value === 'api_service' || value === 'limited_quota_offer') return value
  throw new Error(`Unsupported API order purchase kind: ${value}`)
}

function apiQuotaSaleMode(value?: string): ApiQuotaSaleMode {
  if (value === 'continuous' || value === 'scheduled') return value
  throw new Error(`Unsupported API quota sale mode: ${value}`)
}

function mapAPIQuotaOrderSnapshot(order: BackendAPIOrder, pricingSnapshot: APIIntentPricingSnapshotProjection): ApiQuotaOrderSnapshot | undefined {
  if (order.purchaseKind !== 'limited_quota_offer') return undefined
  const ttftBand = apiTTFTBand(order.quotaTtftBandSnapshot)
  if (
    !order.apiQuotaBatchId
    || !order.apiQuotaOfferId
    || !order.quotaOfferNameSnapshot
    || !order.quotaUsdAllowanceSnapshot
    || !order.quotaPriceCnySnapshot
    || !order.quotaCnyPerUsdSnapshot
    || !order.quotaModelMultiplierSnapshot
    || !order.quotaSaleCutoffAtSnapshot
    || !order.quotaExpiresAtSnapshot
    || !order.quotaDistributionSystemSnapshot
    || !ttftBand
    || !order.quotaDeclaredMaxConcurrencySnapshot
    || !order.quotaDeliveryEtaMinutesSnapshot
    || !order.quotaDeliveryModeSnapshot
  ) {
    throw new Error(`Incomplete API quota order snapshot: ${order.id}`)
  }
  if (order.quotaDistributionSystemSnapshot !== 'sub2api' && order.quotaDistributionSystemSnapshot !== 'new_api_proxy' && order.quotaDistributionSystemSnapshot !== 'other') {
    throw new Error(`Unsupported API quota distribution system: ${order.quotaDistributionSystemSnapshot}`)
  }
  if (order.quotaDeliveryModeSnapshot !== 'manual' && order.quotaDeliveryModeSnapshot !== 'preimported') {
    throw new Error(`Unsupported API quota delivery mode: ${order.quotaDeliveryModeSnapshot}`)
  }
  return {
    batchId: order.apiQuotaBatchId,
    offerId: order.apiQuotaOfferId,
    saleRoundId: order.apiQuotaSaleRoundId,
    offerName: order.quotaOfferNameSnapshot,
    usdAllowance: order.quotaUsdAllowanceSnapshot,
    priceCny: order.quotaPriceCnySnapshot,
    cnyPerUsd: order.quotaCnyPerUsdSnapshot,
    modelMultiplier: order.quotaModelMultiplierSnapshot,
    saleCutoffAt: order.quotaSaleCutoffAtSnapshot,
    expiresAt: order.quotaExpiresAtSnapshot,
    saleMode: apiQuotaSaleMode(order.quotaSaleModeSnapshot),
    roundStartsAt: order.quotaRoundStartsAtSnapshot,
    roundEndsAt: order.quotaRoundEndsAtSnapshot,
    distributionSystem: order.quotaDistributionSystemSnapshot,
    ttftBand,
    declaredMaxConcurrency: order.quotaDeclaredMaxConcurrencySnapshot,
    performanceConfirmedAt: order.quotaPerformanceConfirmedAtSnapshot,
    performanceUnverified: order.quotaPerformanceUnverifiedSnapshot === true,
    deliveryEtaMinutes: order.quotaDeliveryEtaMinutesSnapshot,
    deliveryMode: order.quotaDeliveryModeSnapshot,
    accountPoolType: pricingSnapshot.accountPoolType,
    accountPoolLabel: pricingSnapshot.accountPoolLabel,
    merchantRefundCommitment: pricingSnapshot.merchantRefundCommitment,
    merchantRefundPolicyVersion: pricingSnapshot.merchantRefundPolicyVersion,
    serviceValidityExpiresAt: pricingSnapshot.serviceValidityExpiresAt,
    commercialFactsSnapshotIssue: pricingSnapshot.commercialFactsSnapshotIssue,
  }
}

function apiOrderPaymentMethod(value: string): ApiOrderPaymentInstructions['paymentMethod'] {
  if (isApiPaymentMethod(value)) return value
  throw new Error(`Unsupported API order payment method: ${value}`)
}

function apiOrderPaymentIssueReason(value?: string): ApiOrderPaymentIssueReason | undefined {
  if (value === 'not_received' || value === 'amount_mismatch' || value === 'remark_mismatch') return value
  if (!value) return undefined
  throw new Error(`Unsupported API order payment issue reason: ${value}`)
}

function mapDeliveryCredential(value?: BackendAPIOrderDeliveryCredential | null): ApiOrderDeliveryCredential | undefined {
  if (!value) return undefined
  if (value.deliveryKind !== 'api_key_endpoint' && value.deliveryKind !== 'login_account') {
    throw new Error(`Unsupported API order delivery kind: ${value.deliveryKind}`)
  }
  return {
    deliveryKind: value.deliveryKind,
    apiBaseUrl: value.apiBaseUrl,
    apiKey: value.apiKey,
    panelLoginUrl: value.panelLoginUrl,
    username: value.username,
    password: value.password,
    instructions: value.instructions,
    submittedAt: value.submittedAt,
  }
}

function apiOrderSearchTerms(order: ApiOrder) {
  return [order.id, order.apiPurchaseIntentId, order.serviceTitle, order.buyer, order.seller]
}

function filterAndSortOrders(rows: ApiOrder[], filters: ApiOrderFilters = {}, role: 'buyer' | 'merchant') {
  const search = filters.search?.trim().toLowerCase()
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  const now = Date.now()
  const rangeMs = filters.dateRange === 'today' ? 24 * 60 * 60 * 1000 : filters.dateRange === '7d' ? 7 * 24 * 60 * 60 * 1000 : filters.dateRange === '30d' ? 30 * 24 * 60 * 60 * 1000 : null
  const filtered = rows.filter(row => {
    const createdAt = new Date(row.createdAt).getTime()
    return (!filters.buyerId || row.buyerId === filters.buyerId)
      && (!filters.sellerId || row.sellerId === filters.sellerId)
      && (!statuses || statuses.includes(row.status))
      && (!filters.serviceId || row.apiServiceId === filters.serviceId)
      && (!rangeMs || now - createdAt <= rangeMs)
      && (!search || apiOrderSearchTerms(row).some(value => value.toLowerCase().includes(search)))
  })
  const sort = filters.sort ?? 'updated_desc'
  return filtered.sort((a, b) => {
    if (sort === 'default_buyer' || sort === 'default_merchant') {
	      const buyerAction = (item: ApiOrder) => item.status === 'pending_payment' || item.status === 'payment_issue' || item.status === 'delivery_submitted' || item.status === 'completed'
      const merchantAction = (item: ApiOrder) => item.status === 'payment_submitted' || item.status === 'paid_confirmed'
      const aAction = role === 'buyer' ? buyerAction(a) : merchantAction(a)
      const bAction = role === 'buyer' ? buyerAction(b) : merchantAction(b)
      return Number(bAction) - Number(aAction) || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    }
    if (sort === 'created_desc') return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
    if (sort === 'amount_desc') return compareDecimal(b.amountDecimal || String(b.amount), a.amountDecimal || String(a.amount))
    if (sort === 'amount_asc') return compareDecimal(a.amountDecimal || String(a.amount), b.amountDecimal || String(b.amount))
    return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
  })
}

async function mapBackendAPIOrder(order: BackendAPIOrder, viewerRole: 'buyer' | 'merchant'): Promise<ApiOrder> {
  const intent = await backendAPIIntentById(order.apiPurchaseIntentId)
  if (order.currency !== 'CNY') throw new Error(`Unsupported API order currency: ${order.currency}`)
  const pricingSnapshot = projectAPIIntentPricingSnapshot(order.pricingSnapshot ?? '')
  return {
    id: order.id,
    purchaseKind: apiOrderPurchaseKind(order.purchaseKind),
    apiPurchaseIntentId: order.apiPurchaseIntentId,
    apiServiceId: order.apiServiceId,
    buyerId: order.buyerUserId ?? intent.buyerId,
    buyer: intent.buyer,
    sellerId: order.sellerUserId ?? intent.merchantId,
    seller: intent.snapshot.merchantDisplayName || intent.merchant,
    buyerReputation: mapBackendReputationSummary(order.buyerReputation),
    sellerReputation: mapBackendReputationSummary(order.sellerReputation),
    status: apiOrderStatus(order.status),
    disputeStatus: order.disputeStatus,
    serviceTitle: order.serviceTitleSnapshot || intent.snapshot.serviceTitle,
    amount: numberFromDecimal(order.amount),
    amountDecimal: order.amount,
    currency: 'CNY',
    selectedPaymentMethod: apiOrderPaymentMethod(order.selectedPaymentMethod),
    paymentWindowMinutes: order.paymentWindowMinutesSnapshot,
    paymentExpiresAt: order.paymentExpiresAt,
    paymentSummary: order.paymentSummary,
    paymentSubmittedAt: order.paymentSubmittedAt ?? undefined,
    paymentIssueReason: apiOrderPaymentIssueReason(order.paymentIssueReason),
    paymentIssueNote: order.paymentIssueNote,
    paymentIssueReportedAt: order.paymentIssueReportedAt ?? undefined,
    paidConfirmedAt: order.paidConfirmedAt ?? undefined,
    deliveryNote: order.deliveryNote,
    deliverySubmittedAt: order.deliverySubmittedAt ?? undefined,
    deliveryCredential: mapDeliveryCredential(order.deliveryCredential),
    completedAt: order.completedAt ?? undefined,
    cancelledAt: order.cancelledAt ?? undefined,
    cancelReason: order.cancelReason,
    version: order.version,
    intentSnapshot: {
      ...intent.snapshot,
      models: pricingSnapshot.models,
      multiplier: pricingSnapshot.multiplier,
      defaultMultiplier: pricingSnapshot.defaultMultiplier,
      warranty: pricingSnapshot.merchantSupportNote,
      refundPolicy: apiOrderPlatformTradeBoundary,
      merchantNote: pricingSnapshot.merchantNote,
      pricingSnapshotIssue: pricingSnapshot.issue,
      usageVisibilitySnapshotMissing: pricingSnapshot.usageVisibilitySnapshotMissing,
      usageVisibility: pricingSnapshot.usageVisibility,
      accountPoolType: pricingSnapshot.accountPoolType,
      accountPoolLabel: pricingSnapshot.accountPoolLabel,
      declaredMaxConcurrency: pricingSnapshot.declaredMaxConcurrency,
      merchantRefundCommitment: pricingSnapshot.merchantRefundCommitment,
      merchantRefundPolicyVersion: pricingSnapshot.merchantRefundPolicyVersion,
      serviceValidityExpiresAt: pricingSnapshot.serviceValidityExpiresAt,
      commercialFactsSnapshotIssue: pricingSnapshot.commercialFactsSnapshotIssue,
    },
    selectedDeliveryMode: intent.selectedDeliveryMode,
    selectedPackageId: order.selectedPackageId ?? intent.selectedPackageId,
    packageSnapshot: parsePackageSnapshot(order.selectedPackageSnapshot) ?? intent.snapshot.selectedPackageSnapshot,
    packageStockReserved: order.packageStockReserved,
    packageExpiresAt: order.packageExpiresAt ?? undefined,
    requestedUsdAllowance: numberFromDecimal(order.requestedUsdAllowanceSnapshot || intent.purchasedCreditDecimal),
    requestedUsdAllowanceDecimal: order.requestedUsdAllowanceSnapshot || intent.purchasedCreditDecimal || String(intent.purchasedCredit),
    quotaSnapshot: mapAPIQuotaOrderSnapshot(order, pricingSnapshot),
    merchantContactChannels: intent.contactChannels,
    buyerContactChannels: intent.buyerContactChannels ?? [],
    viewerRole,
    createdAt: order.createdAt,
    updatedAt: order.updatedAt,
  }
}

export async function backendCreateAPIOrderFromIntent(intentId: string, paymentMethod: ApiOrderPaymentInstructions['paymentMethod']) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/me/api-purchase-intents/${intentId}/orders`, { paymentMethod }, {
    idempotencyPrefix: 'api-order-create',
  })
  return mapBackendAPIOrder(response, 'buyer')
}

export async function backendMyAPIOrders(filters: ApiOrderFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIOrder>>('/api/v1/me/api-orders')
  const orders = await Promise.all(response.items.map(item => mapBackendAPIOrder(item, 'buyer')))
  return filterAndSortOrders(orders, filters, 'buyer')
}

export async function backendOwnerAPIOrders(filters: ApiOrderFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIOrder>>('/api/v1/owner/api-orders')
  const orders = await Promise.all(response.items.map(item => mapBackendAPIOrder(item, 'merchant')))
  return filterAndSortOrders(orders, filters, 'merchant')
}

export async function backendMyAPIOrder(id: string) {
  return mapBackendAPIOrder(await backendRequest<BackendAPIOrder>(`/api/v1/me/api-orders/${id}`), 'buyer')
}

export async function backendOwnerAPIOrder(id: string) {
  return mapBackendAPIOrder(await backendRequest<BackendAPIOrder>(`/api/v1/owner/api-orders/${id}`), 'merchant')
}

export async function backendReadAPIOrderPaymentInstructions(id: string): Promise<ApiOrderPaymentInstructions> {
  const response = await backendMutation<BackendAPIOrderPaymentInstructions>(`/api/v1/me/api-orders/${id}/payment-instructions`, {})
  return {
    orderId: response.orderId,
    paymentMethod: apiOrderPaymentMethod(response.paymentMethod),
    paymentInstructions: response.paymentInstructions,
    paymentQrCodeDataUrl: normalizeQrCodeDataUrl(response.paymentQrCodeDataUrl),
    paymentExpiresAt: response.paymentExpiresAt,
  }
}

export async function backendSubmitAPIOrderPayment(id: string, paymentSummary: string, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/me/api-orders/${id}/submit-payment`, { paymentSummary }, {
    idempotencyPrefix: 'api-order-submit-payment',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'buyer')
}

export async function backendCancelAPIOrder(id: string, reason: string, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/me/api-orders/${id}/cancel`, { reason }, {
    idempotencyPrefix: 'api-order-cancel',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'buyer')
}

export async function backendConfirmAPIOrderComplete(id: string, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/me/api-orders/${id}/confirm-complete`, {}, {
    idempotencyPrefix: 'api-order-confirm-complete',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'buyer')
}

export function apiOrderDisputePath(id: string, perspective: 'buyer' | 'merchant') {
  const scope = perspective === 'merchant' ? 'owner' : 'me'
  return `/api/v1/${scope}/api-orders/${encodeURIComponent(id)}/dispute`
}

export async function backendOpenAPIOrderDispute(id: string, reason: string, version: number, perspective: 'buyer' | 'merchant') {
  const response = await backendMutation<BackendAPIOrder>(apiOrderDisputePath(id, perspective), { reason }, {
    idempotencyPrefix: `api-order-${perspective}-dispute`,
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, perspective)
}

export async function backendConfirmAPIOrderPayment(id: string, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/owner/api-orders/${id}/confirm-payment`, {}, {
    idempotencyPrefix: 'api-order-confirm-payment',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'merchant')
}

export async function backendReportAPIOrderPaymentIssue(id: string, reason: ApiOrderPaymentIssueReason, note: string, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/owner/api-orders/${id}/report-payment-issue`, { reason, note }, {
    idempotencyPrefix: 'api-order-report-payment-issue',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'merchant')
}

export async function backendSubmitAPIOrderDeliveryCredential(id: string, payload: SubmitApiOrderDeliveryCredentialPayload, version: number) {
  const response = await backendMutation<BackendAPIOrder>(`/api/v1/owner/api-orders/${id}/submit-delivery`, payload, {
    idempotencyPrefix: 'api-order-submit-delivery',
    ifMatch: version,
  })
  return mapBackendAPIOrder(response, 'merchant')
}

export async function backendSubmitAPIService(payload: Record<string, unknown>) {
  await ensureBackendSession('merchant', false)
  const merchantProfile = await ensureMerchantProfile(payload)
  let ownerContactMethodId = String(payload.ownerContactMethodId ?? '')
  if (!ownerContactMethodId) {
    const contact = await backendCreateContactMethod({
      type: 'linuxdo',
      label: 'linux.do 私信',
      displayValue: '@merchant',
      usageScopes: ['api_merchant'],
      isDefault: true,
      enabled: true,
    })
    ownerContactMethodId = contact.id
  }
  let response = await backendMutation<BackendAPIService>('/api/v1/owner/api-services', toBackendServiceRequest({
    ...payload,
    ownerContactMethodId,
    merchantProfileId: merchantProfile.id,
    merchantIdentityMode: 'store_alias',
  }), {
    idempotencyPrefix: 'api-service',
  })
  if (payload.status === 'reviewing') {
    response = await backendOwnerAPIServiceAction(response.id, 'submit-review', response.version)
    response = await backendOwnerAPIServiceAction(response.id, 'publish', response.version)
    response = await backendUpdateAPIServiceOrderSettings(response.id, payload, response.version)
  }
  return mapBackendAPIService(response)
}

async function backendUpdateAPIServiceOrderSettings(id: string, payload: Record<string, unknown>, version: number) {
  return backendMutation<BackendAPIService>(`/api/v1/owner/api-services/${id}/order-settings`, toBackendOrderSettingsRequest(payload), {
    method: 'PATCH',
    idempotencyPrefix: 'api-service-order-settings',
    ifMatch: version,
  })
}

async function backendOwnerAPIServiceAction(id: string, action: 'submit-review' | 'publish' | 'pause' | 'resume' | 'start-revision', version?: number) {
  const current = version === undefined
    ? await backendRequest<BackendAPIService>(`/api/v1/owner/api-services/${id}`)
    : null
  return backendMutation<BackendAPIService>(`/api/v1/owner/api-services/${id}/${action}`, {}, {
    idempotencyPrefix: `api-service-${action}`,
    ifMatch: version ?? current?.version,
  })
}

async function backendAdminAPIServiceAction(id: string, action: 'approve' | 'request-changes' | 'reject' | 'suspend' | 'restore' | 'remove', reason: string, version?: number) {
  await ensureBackendSession('admin', true)
  const current = version === undefined
    ? await backendRequest<BackendAPIService>(`/api/v1/admin/api-services/${id}`)
    : null
  return backendMutation<BackendAPIService>(`/api/v1/admin/api-services/${id}/${action}`, { reason }, {
    idempotencyPrefix: `api-service-admin-${action}`,
    ifMatch: version ?? current?.version,
  })
}

export function toBackendServiceRequest(payload: Record<string, unknown>) {
  const distributionSystem = payload.distributionSystem === 'new_api_proxy' ? 'new_api_proxy' : payload.distributionSystem === 'sub2api' ? 'sub2api' : 'other'
  const billing = payload.billingMode === 'fixed_package' ? 'fixed_package' : payload.billingMode === 'manual_credit' ? 'manual_usage_check' : 'metered_usd_quota'
  const modes = Array.isArray(payload.deliveryModes) ? payload.deliveryModes as string[] : ['api_key_endpoint']
  const selectedModels = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, enabled?: boolean }> : []
  const packages = Array.isArray(payload.packages) ? payload.packages as Array<{ id?: string, name?: string, priceCny?: number, panelAllowance?: number, durationDays?: number, stockTotal?: number, description?: string, enabled?: boolean, modelCatalogIds?: string[] }> : []

  const fixedPackage = billing === 'fixed_package'
  return {
    merchantProfileId: String(payload.merchantProfileId ?? ''),
    merchantIdentityMode: String(payload.merchantIdentityMode ?? 'public_profile'),
    ownerContactMethodId: String(payload.ownerContactMethodId ?? ''),
    title: String(payload.generatedTitle ?? 'API 服务'),
    shortDescription: String(payload.shortDescription ?? 'API 服务'),
    sourceUrl: String(payload.sourceUrl ?? ''),
    distributionSystem,
    billingMode: billing,
    declaredCnyPerUsdAllowance: fixedPackage ? '' : String(payload.cnyPerUsdCredit ?? '1'),
    declaredMaxUsdAllowancePerIntent: fixedPackage ? '' : String(payload.availableCreditUsd ?? '20'),
    availableUsdAllowance: fixedPackage ? '' : String(payload.availableCreditUsd ?? '20'),
    quotaExpiresAt: fixedPackage ? '' : beijingDateTimeInputToISOString(String(payload.quotaExpiresAt ?? '')),
    minimumIntentCny: String(payload.minimumPurchaseCny ?? '10'),
    maximumIntentCny: String(payload.maximumPurchaseCny ?? '300'),
    usageVisibility: toBackendUsageVisibility(payload.usageVisibility),
    publicAccessNote: String(payload.distributionSystemNote ?? ''),
    merchantNote: String(payload.merchantNote ?? ''),
    accountPoolType: String(payload.accountPoolType ?? ''),
    accountPoolCustomName: payload.accountPoolType === 'custom' ? String(payload.accountPoolCustomName ?? '') : '',
    merchantRefundCommitment: Boolean(payload.warranty && typeof payload.warranty === 'object' && !Array.isArray(payload.warranty) && (payload.warranty as Record<string, unknown>).mode === 'merchant_full_refund'),
    declaredTtftBand: String(payload.declaredTtftBand ?? ''),
    declaredMaxConcurrency: Number(payload.declaredMaxConcurrency ?? 0),
    performanceConfirmedAt: beijingDateTimeInputToISOString(String(payload.performanceConfirmedAt ?? '')),
    accessModes: fixedPackage
      ? [{ accessMode: 'fixed_package_offsite', publicNote: '交付后开始计算套餐有效期，具体接入信息按订单权限展示。' }]
      : modes.map(accessMode => ({ accessMode: toBackendAccessMode(accessMode), publicNote: '仅展示接入说明，不展示凭据。' })),
    models: selectedModels.filter(item => item.enabled !== false).map(item => ({
      modelCatalogId: item.modelId ?? '',
      modelPriceVersionId: '',
      merchantMultiplier: String(payload.defaultMultiplier ?? '1.0000'),
      enabled: true,
    })),
    packages: packages.map((item, index) => ({
      id: item.id || undefined,
      name: item.name ?? `套餐 ${index + 1}`,
      priceCny: String(item.priceCny ?? 20),
      panelAllowance: String(item.panelAllowance ?? 1),
      durationDays: item.durationDays,
      stockTotal: item.stockTotal ?? 0,
      description: item.description ?? '',
      enabled: item.enabled !== false,
      sortOrder: index,
      modelCatalogIds: item.modelCatalogIds ?? [],
    })),
  }
}

function toBackendOrderSettingsRequest(payload: Record<string, unknown>) {
  const paymentOptions = Array.isArray(payload.paymentOptions)
    ? payload.paymentOptions as Array<{ paymentMethod?: string, enabled?: boolean, paymentInstructions?: string, paymentQrCodeDataUrl?: string | null }>
    : []
  return {
    acceptingOrders: true,
    paymentWindowMinutes: Number(payload.paymentWindowMinutes ?? 10),
    paymentOptions: paymentOptions.map(option => {
      const paymentMethod = String(option.paymentMethod ?? '')
      const enabled = Boolean(option.enabled)
      const paymentInstructions = String(option.paymentInstructions ?? '').trim()
      const paymentQrCodeDataUrl = normalizeQrCodeDataUrl(option.paymentQrCodeDataUrl)
      return {
        paymentMethod,
        enabled,
        paymentInstructions: paymentInstructions || (enabled && isApiPaymentMethod(paymentMethod) && apiPaymentMethodRequiresQrCode(paymentMethod) && paymentQrCodeDataUrl
          ? '买家创建订单后查看收款码并站外确认。'
          : ''),
        paymentQrCodeDataUrl: paymentQrCodeDataUrl ?? '',
      }
    }).filter(option => option.enabled || option.paymentInstructions),
  }
}

async function ensureMerchantProfile(payload: Record<string, unknown>) {
  const existing = await backendMyMerchantProfile()
  if (existing) return existing
  const requestedName = String(payload.merchantDisplayName ?? payload.generatedTitle ?? 'API Store').trim()
  const displayName = requestedName.length >= 2 ? requestedName.slice(0, 32) : 'API Store'
  const slug = displayName
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 24)
  return backendUpsertMerchantProfile({
    slug: slug.length >= 3 ? slug : 'api-store',
    displayName,
  })
}

function toBackendAccessMode(mode: string) {
  if (mode === 'sub2api_panel_account') return 'buyer_dedicated_panel_subaccount'
  return 'buyer_dedicated_sub_key'
}

function toBackendUsageVisibility(value: unknown) {
  if (value === 'panel_realtime' || value === 'panel_balance_only') return 'offsite_panel_readonly'
  if (value === 'merchant_confirmed') return 'merchant_reported'
  if (value === 'fixed_package_only') return 'fixed_package_only'
  return 'none'
}

function backendServiceStatus(service: BackendAPIService) {
  if (service.moderationStatus === 'removed') return '已移除'
  if (service.moderationStatus === 'admin_suspended') return '已下架'
  if (service.reviewStatus === 'pending_review') return '待处理'
  if (service.reviewStatus === 'changes_requested') return '待复核'
  if (service.reviewStatus === 'rejected') return '已拒绝'
  if (service.reviewStatus === 'approved' && service.publicationStatus === 'online') return '在线'
  if (service.reviewStatus === 'approved' && service.publicationStatus === 'owner_paused') return '暂停'
  if (service.reviewStatus === 'approved') return '已通过'
  return '草稿'
}

function serviceAdminRow(service: BackendAPIService): AdminRow {
  const mapped = mapBackendAPIService(service)
  return {
    id: service.id,
    primary: service.title,
    secondary: `${mapped.models.join(' / ')} · ${mapped.delivery} · 接入细节站外确认`,
    owner: `${mapped.merchantDisplayName} · ${service.ownerUserId ? `用户 ${service.ownerUserId.slice(0, 8)}` : '真实后端用户'}`,
    status: backendServiceStatus(service),
    risk: service.moderationStatus === 'clear' ? mapped.warranty : service.moderationStatus ?? 'clear',
    targetType: 'api-service',
    detailItems: [
      { label: '审核状态', value: service.reviewStatus ?? 'draft' },
      { label: '发布状态', value: service.publicationStatus ?? 'offline' },
      { label: '治理状态', value: service.moderationStatus ?? 'clear' },
      { label: '版本', value: String(service.version) },
      { label: '最低订单金额', value: `¥${mapped.minimumPurchaseCny}` },
      { label: '用量核对', value: service.usageVisibility },
    ],
    targetTo: mapped.publiclyOrderable ? `/api-market/${service.id}` : null,
  }
}

export async function backendAdminAPIServiceRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendAPIService>>('/api/v1/admin/api-services')
  return response.items.map(serviceAdminRow)
}

export async function backendUpdateAdminAPIServiceStatus(row: AdminRow, status: string, reason: string) {
  if (row.targetType !== 'api-service' && row.targetType !== 'api-merchant') return row
  const action = status === '已通过' ? 'approve' : 'request-changes'
  const service = await backendAdminAPIServiceAction(row.id, action, reason || '管理台审核操作')
  return serviceAdminRow(service)
}

export async function backendRunAdminAPIServiceAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (row.targetType !== 'api-service' && row.targetType !== 'api-merchant') return row
  const backendAction = action === 'request_changes'
    ? 'request-changes'
    : action === 'take_down' || action === 'suspend'
      ? 'suspend'
      : action === 'restore'
        ? 'restore'
        : action === 'approve'
          ? 'approve'
          : 'remove'
  const service = await backendAdminAPIServiceAction(row.id, backendAction, reason)
  return serviceAdminRow(service)
}

export async function backendPublishAPIService(id: string) {
  await ensureBackendSession('merchant', false)
  const service = await backendOwnerAPIServiceAction(id, 'publish')
  return mapBackendAPIService(service)
}

export async function backendPauseAPIService(id: string) {
  await ensureBackendSession('merchant', false)
  const service = await backendOwnerAPIServiceAction(id, 'pause')
  return mapBackendAPIService(service)
}

export async function backendResumeAPIService(id: string) {
  await ensureBackendSession('merchant', false)
  const service = await backendOwnerAPIServiceAction(id, 'resume')
  return mapBackendAPIService(service)
}
