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
  ApiPurchaseIntent,
  ApiPurchaseIntentEvent,
  ApiPurchaseIntentFilters,
  ApiService,
  ApiServicePackageSnapshot,
  ApiServiceFilters,
  ApiUsageVisibility,
  ContactMethodType,
  CreateApiPurchaseIntentPayload,
  ModelCatalogItem,
  ModelPriceRow,
  OtherApiMarketFilters,
  SaveContactMethodRequest,
  SubmitApiOrderDeliveryCredentialPayload,
  Sub2ApiMarketFilters,
  UserContactMethod,
} from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { apiPaymentMethodRequiresQrCode, isApiPaymentMethod, normalizeQrCodeDataUrl } from '@/lib/apiPaymentSettings'
import { beijingDateTimeInputToISOString, formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'
import { backendMyMerchantProfile, backendUpsertMerchantProfile } from '@/lib/profileBackend'
import { compareDecimal, divideDecimal, normalizeDecimal, normalizeDecimalTrimmed } from '@/lib/decimal'

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
  durationDays?: number
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
  distributionSystem: string
  billingMode: string
  declaredCnyPerUsdAllowance?: string
  declaredMaxUsdAllowancePerIntent?: string
  availableUsdAllowance?: string
  quotaExpiresAt?: string
  minimumIntentCny: string
  maximumIntentCny?: string
  usageVisibility: string
  publicAccessNote?: string
  merchantNote?: string
  merchantSupportNote?: string
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
  apiPurchaseIntentId: string
  apiServiceId: string
  buyerUserId?: string
  sellerUserId?: string
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
  return {
    id: service.id,
    title: service.title.replace(/意向服务/g, '服务').replace(/API 意向/g, 'API 订单'),
    sourceUrl: service.sourceUrl ?? '',
    merchantId: service.merchantProfileId ?? service.ownerUserId ?? 'merchant',
    merchantUsername,
    merchant: displayName,
    merchantIdentityMode: isStoreAlias ? 'store_alias' : 'public_profile',
    merchantDisplayName: displayName,
    merchantAvatarUrl: service.merchantAvatarUrl?.trim() || undefined,
    trustLevel: 4,
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
    expectedResponseMinutes: service.paymentWindowMinutes ?? 10,
    responseMedianMinutes: service.responseMedianMinutes ?? service.paymentWindowMinutes ?? 10,
    dailyOrderLimit: 10,
    todayOrderCount: 0,
    unresolvedDisputes: service.unresolvedDisputes ?? 0,
    warning: state === 'reviewing' ? '等待管理员审核' : online && !publiclyOrderable ? '待配置接单设置' : undefined,
    warranty: service.merchantSupportNote || '按商户备注站外协商，平台不担保、不代赔',
    refundPolicy: '最终金额和售后由双方站外确认，平台不处理支付或托管',
    quotaExpiresAt: service.quotaExpiresAt,
    expiresAt: formatQuotaExpiresAtLabel(service.quotaExpiresAt) || '按服务说明',
    completed30d: service.completed30d ?? 0,
    reviewCount: 0,
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

function filterServices(rows: ApiService[], filters: ApiServiceFilters | Sub2ApiMarketFilters | OtherApiMarketFilters = {}) {
  const search = 'search' in filters ? filters.search?.trim().toLowerCase() : undefined
  return rows.filter(row => {
    if (search && ![row.title, row.merchant, row.merchantDisplayName, ...row.models].some(value => value.toLowerCase().includes(search))) return false
    if ('deliveryMode' in filters && filters.deliveryMode && !row.deliveryModes.includes(filters.deliveryMode)) return false
    if ('online' in filters && filters.online !== undefined && row.publiclyOrderable !== filters.online) return false
    return true
  })
}

export async function backendAPIServices(filters: ApiServiceFilters = {}) {
  const response = await backendRequest<ListResponse<BackendAPIService>>('/api/v1/api-services')
  return filterServices(response.items.map(mapBackendAPIService).filter(row => row.publiclyOrderable), filters)
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

export async function backendOwnerAPIServices() {
  await ensureBackendSession('merchant', false)
  const response = await backendRequest<ListResponse<BackendAPIService>>('/api/v1/owner/api-services')
  return response.items.map(mapBackendAPIService)
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
    targetModel: intent.serviceTitleSnapshot,
    buyerNote: intent.buyerNote,
    snapshot: {
      serviceId: intent.apiServiceId,
      serviceTitle: intent.serviceTitleSnapshot,
      merchantId: intent.ownerUserId ?? 'merchant',
      merchant: merchantName,
      merchantUsername: intent.ownerUserId ?? 'merchant',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: merchantName,
      trustLevel: 4,
      merchantType: '商户',
      models: [intent.serviceTitleSnapshot],
      multiplier: '1.00x',
      defaultMultiplier: 1,
      creditPerCny: amount > 0 && credit > 0 ? Number((credit / amount).toFixed(4)) : 1,
      cnyPerUsdAllowance: intent.declaredCnyPerUsdAllowanceSnapshot || '1.0000',
      warranty: '商户按服务说明站外处理，平台不担保、不代赔',
      refundPolicy: '最终金额和售后由双方站外确认',
      usageVisibility: 'none',
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
  return {
    id: order.id,
    apiPurchaseIntentId: order.apiPurchaseIntentId,
    apiServiceId: order.apiServiceId,
    buyerId: order.buyerUserId ?? intent.buyerId,
    buyer: intent.buyer,
    sellerId: order.sellerUserId ?? intent.merchantId,
    seller: intent.snapshot.merchantDisplayName || intent.merchant,
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
    intentSnapshot: intent.snapshot,
    selectedDeliveryMode: intent.selectedDeliveryMode,
    selectedPackageId: order.selectedPackageId ?? intent.selectedPackageId,
    packageSnapshot: parsePackageSnapshot(order.selectedPackageSnapshot) ?? intent.snapshot.selectedPackageSnapshot,
    packageStockReserved: order.packageStockReserved,
    packageExpiresAt: order.packageExpiresAt ?? undefined,
    requestedUsdAllowance: numberFromDecimal(order.requestedUsdAllowanceSnapshot || intent.purchasedCreditDecimal),
    requestedUsdAllowanceDecimal: order.requestedUsdAllowanceSnapshot || intent.purchasedCreditDecimal || String(intent.purchasedCredit),
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

function toBackendServiceRequest(payload: Record<string, unknown>) {
  const distributionSystem = payload.distributionSystem === 'new_api_proxy' ? 'new_api_proxy' : payload.distributionSystem === 'sub2api' ? 'sub2api' : 'other'
  const billing = payload.billingMode === 'fixed_package' ? 'fixed_package' : payload.billingMode === 'manual_credit' ? 'manual_usage_check' : 'metered_usd_quota'
  const modes = Array.isArray(payload.deliveryModes) ? payload.deliveryModes as string[] : ['api_key_endpoint']
  const selectedModels = Array.isArray(payload.selectedModels) ? payload.selectedModels as Array<{ modelId?: string, multiplierOverride?: number | null, enabled?: boolean }> : []
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
    merchantSupportNote: '平台不担保、不代赔；双方站外确认。',
    accessModes: fixedPackage
      ? [{ accessMode: 'fixed_package_offsite', publicNote: '交付后开始计算套餐有效期，具体接入信息按订单权限展示。' }]
      : modes.map(accessMode => ({ accessMode: toBackendAccessMode(accessMode), publicNote: '仅展示接入说明，不展示凭据。' })),
    models: selectedModels.filter(item => item.enabled !== false).map(item => ({
      modelCatalogId: item.modelId ?? '',
      modelPriceVersionId: '',
      merchantMultiplier: String(item.multiplierOverride ?? payload.defaultMultiplier ?? '1.0000'),
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
