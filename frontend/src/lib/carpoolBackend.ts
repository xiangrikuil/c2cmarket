import type {
  AdminRow,
  CarpoolApplicationEvent,
  CarpoolApplicationEligibility,
  CarpoolApplicationWithMeta,
  CarpoolApplicationFilters,
  CarpoolProductCatalogItem,
  CarpoolWithMeta,
  CommunityIdentity,
  ContactMethodType,
  OrderContactSnapshot,
  OrderContactSnapshotItem,
  PaymentMethodOption,
  RegionOption,
  SaveCarpoolDraftPayload,
  OwnerCarpoolEditData,
  OwnerCarpoolView,
} from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { backendBoundLinuxDoContactMethod } from '@/lib/apiMarketBackend'
import { carpoolOpeningChannels, carpoolPaymentMethods, carpoolRegions } from '@/data/mock'
import { defaultQuotaLabel, defaultQuotaPeriod, defaultQuotaUnit } from '@/lib/quota'
import { linuxDoProfileSummaryUrl } from '@/lib/linuxDo'
import { mapBackendReputationSummary } from '@/lib/reputationBackend'
import type { ReputationSummary } from '@/types/reputation'
import { trackAnalytics } from '@/lib/analytics'
import { collectCursorPages, normalizeNextCursor, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'

type ListResponse<T> = { items: T[], nextCursor?: string | null }

export type CarpoolPageFilters = {
  q?: string
  productPlanIds?: string[]
  region?: string
  carpoolId?: string
  ownerType?: string
  warranty?: string
  statuses?: string[]
  view?: 'public' | 'exceptions' | OwnerCarpoolView
  risk?: 'all' | 'high' | 'has_note'
  sort?: 'recommended' | 'updated_desc' | 'created_desc' | 'default_buyer' | 'default_owner' | 'price_asc' | 'seats_desc'
  none?: boolean
}

type BackendProductPlan = {
  id: string
  categoryCode: string
  providerCode: string
  slug: string
  displayName: string
  description: string
  publishPolicy: string
  accessMode: string
  providerPolicyStatus: string
  riskLevel: string
  riskAckRequired: boolean
  riskNoticeCode?: string
  policyVersion: number
  policyNote: string
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: 'monthly'
  allowCustomVariant: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

type BackendCarpoolListing = {
  id: string
  ownerUserId: string
  productPlanId: string
  ownerContactMethodId?: string
  cycleTerm?: BackendCycleTerm
  title: string
  summary: string
  accessArrangement: string
  distributionMethod: CarpoolWithMeta['distributionMethod']
  distributionMethodNote: string
  providesAdminAccount: boolean
  regionCode: string
  regionName: string
  sourceUrl?: string
  sourceAuthorVerification: {
    status: 'not_submitted' | 'pending' | 'verified' | 'mismatch' | 'expired'
    verifiedAt?: string
    expiresAt?: string
  }
  sellerReputation?: ReputationSummary | null
  communityIdentities?: CommunityIdentity[]
  priceMonthlyCny: string
  serviceMultiplier: string
  dailySpendLimitUsd?: string | null
  weeklySpendLimitUsd?: string | null
  dailyQuotaAmount?: string | null
  weeklyQuotaAmount?: string | null
  followsOfficialQuotaReset: boolean | null
  vpsRegion: string | null
  supportsMainlandChinaDirectConnection: boolean | null
  openingChannelCode: CarpoolWithMeta['openingChannelCode']
  customOpeningChannel: string | null
  paymentMethodCode: CarpoolWithMeta['paymentMethodCode']
  customPaymentMethod: string | null
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: 'monthly'
  buyerSeatCapacity: number
  offlineOccupiedSeats?: number
  activeBuyerMembers: number
  availableSeats: number
  status: string
  governanceStatus: string
  recruitmentStopReason?: string
  conditionsVersion?: number
  reviewReason?: string
  reviewedAt?: string
  policyVersion: number
  riskNoticeCode?: string
  riskAckRequired: boolean
  version: number
  createdAt: string
  updatedAt: string
  applicationEligibility?: CarpoolApplicationEligibility
}

type BackendCycleTerm = {
  id: string
  billingPeriod: string
  cycleStartDay?: number
  noticeDays: number
  exitPolicy: string
  usageRules: string
  version: number
  createdAt: string
  updatedAt: string
}

type BackendCarpoolApplication = {
  id: string
  carpoolListingId: string
  buyerUserId: string
  ownerUserId: string
  productPlanId: string
  buyerContactMethodId: string
  status: string
  seatCount: number
  listingTitleSnapshot: string
  priceMonthlyCny: string
  policyVersionSnapshot: number
  riskNoticeCode?: string
  conditionsVersionSnapshot: number
  conditionsSnapshot?: {
    title: string
    priceMonthlyCny: string
    dailySpendLimitUsd: string | null
    weeklySpendLimitUsd: string | null
    followsOfficialQuotaReset: boolean
    buyerSeatCapacity: number
    offlineOccupiedSeats: number
    regionName: string
    accessArrangement: string
    distributionMethod: string
    distributionMethodNote: string
    providesAdminAccount: boolean
    cycleTerm?: { usageRules: string, exitPolicy: string }
  }
  acceptedConditionsVersion: number
  conditionsAcceptedAt: string
  contactSessionId?: string
  joinedAt?: string
  decisionReason?: string
  decidedAt?: string
  version: number
  createdAt: string
  updatedAt: string
  buyerReputation?: ReputationSummary | null
}

type BackendCarpoolApplicationEligibility = CarpoolApplicationEligibility

type BackendCarpoolMembership = {
  id: string
  carpoolListingId: string
  carpoolApplicationId: string
  cycleTermId?: string
  buyerUserId: string
  ownerUserId: string
  productPlanId: string
  status: string
  seatCount: number
  priceMonthlyCny: string
  policyVersionSnapshot: number
  riskNoticeCode?: string
  joinedAt: string
  endedAt?: string
  endedReason?: string
  endedByUserId?: string
  ownerNote?: string
  version: number
  createdAt: string
  updatedAt: string
}

type BackendContactSessionContacts = {
  sessionId: string
  endsAt: string | null
  items: Array<{
    side: string
    type: ContactMethodType
    label: string
    value: string
    maskedValue: string
  }>
}

const backendProductPlans = new Map<string, BackendProductPlan>()
const backendCarpoolListings = new Map<string, BackendCarpoolListing>()
const backendMembershipsByApplication = new Map<string, BackendCarpoolMembership>()
const backendMembershipsByApplicationOwner = new Map<string, BackendCarpoolMembership>()
const PRODUCT_CATALOG_CACHE_TTL_MS = 60_000
let productCatalogCache: { value: CarpoolProductCatalogItem[], cachedAt: number } | null = null
let productCatalogRequest: Promise<CarpoolProductCatalogItem[]> | null = null

function numberFromDecimal(value: string | null | undefined, fallback = 0) {
  if (!value) return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function formatTime(value: string | undefined) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function mapProviderCode(value: string): CarpoolProductCatalogItem['providerCode'] {
  return value
}

function mapCategoryCode(value: string): CarpoolProductCatalogItem['categoryCode'] {
  return value
}

function mapPublishPolicy(value: string): CarpoolProductCatalogItem['publishPolicy'] {
  return value === 'info_only' || value === 'blocked' ? value : 'allowed'
}

function mapAccessMode(value: string): CarpoolProductCatalogItem['accessMode'] {
  if (value === 'personal_account_cost_share' || value === 'provider_member_invitation' || value === 'owner_managed_access') return value
  return 'other_off_platform'
}

function mapProviderPolicyStatus(value: string): CarpoolProductCatalogItem['providerPolicyStatus'] {
  if (value === 'known_restricted' || value === 'possibly_restricted') return value
  return 'unknown'
}

function mapRiskLevel(value: string): CarpoolProductCatalogItem['riskLevel'] {
  if (value === 'high' || value === 'elevated') return value
  return 'normal'
}

function mapProductPlan(plan: BackendProductPlan): CarpoolProductCatalogItem {
  backendProductPlans.set(plan.id, plan)
  return {
    id: plan.id,
    categoryCode: mapCategoryCode(plan.categoryCode),
    providerCode: mapProviderCode(plan.providerCode),
    displayName: plan.displayName,
    slug: plan.slug,
    description: plan.description || null,
    publishPolicy: mapPublishPolicy(plan.publishPolicy),
    accessMode: mapAccessMode(plan.accessMode),
    providerPolicyStatus: mapProviderPolicyStatus(plan.providerPolicyStatus),
    riskLevel: mapRiskLevel(plan.riskLevel),
    riskAckRequired: plan.riskAckRequired,
    policyVersion: plan.policyVersion,
    policyNote: plan.policyNote,
    quotaLabel: plan.quotaLabel || defaultQuotaLabel,
    quotaUnit: plan.quotaUnit || defaultQuotaUnit,
    quotaPeriod: plan.quotaPeriod || defaultQuotaPeriod,
    riskNoticeCode: plan.riskNoticeCode,
    active: true,
    sortOrder: plan.sortOrder,
    allowCustomVariant: plan.allowCustomVariant,
    createdAt: plan.createdAt,
    updatedAt: plan.updatedAt,
  }
}

export async function backendCarpoolProductCatalog() {
  const now = Date.now()
  if (productCatalogCache && now - productCatalogCache.cachedAt <= PRODUCT_CATALOG_CACHE_TTL_MS) {
    return productCatalogCache.value
  }
  if (productCatalogRequest) {
    return productCatalogRequest
  }

  productCatalogRequest = backendRequest<ListResponse<BackendProductPlan>>('/api/v1/product-plans')
    .then(response => {
      const value = response.items.map(mapProductPlan)
      productCatalogCache = { value, cachedAt: Date.now() }
      return value
    })
    .finally(() => {
      productCatalogRequest = null
    })
  return productCatalogRequest
}

export function clearBackendCarpoolProductCatalogCache() {
  productCatalogCache = null
  productCatalogRequest = null
}

export async function backendCarpoolRegions(): Promise<RegionOption[]> {
  return carpoolRegions.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder)
}

export async function backendCarpoolOpeningChannels() {
  return carpoolOpeningChannels.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder)
}

export async function backendCarpoolPaymentMethods(): Promise<PaymentMethodOption[]> {
  return carpoolPaymentMethods.filter(item => item.active).sort((a, b) => a.sortOrder - b.sortOrder)
}

async function productPlan(id: string) {
  if (backendProductPlans.has(id)) return backendProductPlans.get(id)!
  const plan = await backendRequest<BackendProductPlan>(`/api/v1/product-plans/${id}`)
  backendProductPlans.set(id, plan)
  return plan
}

function listingStatus(value: string, availableSeats: number): CarpoolWithMeta['status'] {
  if (value === 'pending_review') return '审核中'
  if (value === 'active') return availableSeats > 0 ? '可上车' : '已满'
  return '暂停'
}

function openingMethodFromAccessMode(value: string): CarpoolWithMeta['openingMethod'] {
  if (value === 'provider_member_invitation') return '其他'
  if (value === 'personal_account_cost_share') return '其他'
  return '其他'
}

function ownerLabel(userId: string) {
  if (!userId) return '车主'
  return userId.length > 8 ? `用户 ${userId.slice(0, 8)}` : userId
}

function projectListingSeats(listing: BackendCarpoolListing) {
  const totalSeats = Math.max(1, listing.buyerSeatCapacity)
  const activeMemberCount = Math.min(Math.max(0, listing.activeBuyerMembers), totalSeats)
  const occupiedSeatCount = Math.min(
    Math.max(0, (listing.offlineOccupiedSeats ?? 0) + listing.activeBuyerMembers),
    totalSeats,
  )

  return {
    carpoolId: listing.id,
    totalSeats,
    activeMemberCount,
    occupiedSeatCount,
    availableSeats: Math.min(Math.max(0, listing.availableSeats), totalSeats),
  }
}

export async function mapBackendCarpoolListing(listing: BackendCarpoolListing): Promise<CarpoolWithMeta> {
  backendCarpoolListings.set(listing.id, listing)
  const plan = await productPlan(listing.productPlanId)
  const monthly = numberFromDecimal(listing.priceMonthlyCny)
  const serviceMultiplier = numberFromDecimal(listing.serviceMultiplier)
  const dailyLimit = listing.dailySpendLimitUsd !== undefined ? listing.dailySpendLimitUsd : listing.dailyQuotaAmount
  const weeklyLimit = listing.weeklySpendLimitUsd !== undefined ? listing.weeklySpendLimitUsd : listing.weeklyQuotaAmount
  const dailyQuotaAmount = dailyLimit ? numberFromDecimal(dailyLimit) : undefined
  const weeklyQuotaAmount = weeklyLimit ? numberFromDecimal(weeklyLimit) : undefined
  const seatSummary = projectListingSeats(listing)
  return {
    id: listing.id,
    product: plan.displayName,
    region: listing.regionName,
    monthly,
    serviceMultiplier,
    dailyQuotaAmount,
    weeklyQuotaAmount,
    followsOfficialQuotaReset: listing.followsOfficialQuotaReset,
    vpsRegion: listing.vpsRegion,
    supportsMainlandChinaDirectConnection: listing.supportsMainlandChinaDirectConnection,
    openingChannelCode: listing.openingChannelCode,
    customOpeningChannel: listing.customOpeningChannel,
    paymentMethodCode: listing.paymentMethodCode,
    customPaymentMethod: listing.customPaymentMethod,
    quotaLabel: listing.quotaLabel || plan.quotaLabel || defaultQuotaLabel,
    quotaUnit: listing.quotaUnit || plan.quotaUnit || defaultQuotaUnit,
    quotaPeriod: listing.quotaPeriod || plan.quotaPeriod || defaultQuotaPeriod,
    seats: `${seatSummary.occupiedSeatCount}/${seatSummary.totalSeats}`,
    pricingMode: 'fixed',
    fixedMonthlyPrice: monthly,
    currentConfirmedMembers: seatSummary.activeMemberCount,
    maxMembers: seatSummary.totalSeats,
    owner: ownerLabel(listing.ownerUserId),
    ownerUserId: listing.ownerUserId,
    trustLevel: null,
    sellerReputation: mapBackendReputationSummary(listing.sellerReputation),
    communityIdentities: listing.communityIdentities ?? [],
    ownerType: '个人车主',
    warranty: '车主承诺',
    openingMethod: openingMethodFromAccessMode(plan.accessMode),
    status: listingStatus(listing.status, seatSummary.availableSeats),
    confirmedAt: formatTime(listing.updatedAt),
    confirmedWithin48h: true,
    linuxdoBound: null,
    sourceUrl: listing.sourceUrl,
    sourceAuthorVerification: listing.sourceAuthorVerification,
    hasInfoConflict: false,
    hasUnresolvedDispute: listing.sellerReputation ? listing.sellerReputation.unresolvedDisputes > 0 : null,
    distributionMethod: listing.distributionMethod,
    distributionMethodNote: listing.distributionMethodNote,
    providesAdminAccount: listing.providesAdminAccount,
    accessArrangementMode: mapAccessMode(plan.accessMode),
    accessArrangementNote: listing.accessArrangement || plan.policyNote,
    riskNoticeCode: listing.riskNoticeCode || plan.riskNoticeCode,
    riskAcknowledged: listing.riskAckRequired ? true : undefined,
    applicationEligibility: listing.applicationEligibility,
    backendVersion: listing.version,
    backendStatus: listing.status,
    offlineOccupiedSeats: listing.offlineOccupiedSeats ?? 0,
    recruitmentStopReason: listing.recruitmentStopReason,
    seatSummary,
  }
}

async function mapListings(rows: BackendCarpoolListing[]) {
  return Promise.all(rows.map(mapBackendCarpoolListing))
}

function carpoolPageQuery(filters: CarpoolPageFilters, page: CursorPageRequest) {
  const params = new URLSearchParams()
  if (filters.q?.trim()) params.set('q', filters.q.trim())
  if (filters.productPlanIds?.length) params.set('productPlanIds', filters.productPlanIds.join(','))
  if (filters.region) params.set('region', filters.region)
  if (filters.carpoolId) params.set('carpoolId', filters.carpoolId)
  if (filters.ownerType) params.set('ownerType', filters.ownerType)
  if (filters.warranty) params.set('warranty', filters.warranty)
  if (filters.statuses?.length) params.set('statuses', filters.statuses.join(','))
  if (filters.view) params.set('view', filters.view)
  if (filters.risk && filters.risk !== 'all') params.set('risk', filters.risk)
  if (filters.sort && filters.sort !== 'recommended') params.set('sort', filters.sort)
  if (filters.none) params.set('none', '1')
  if (page.limit) params.set('limit', String(page.limit))
  if (page.cursor) params.set('cursor', page.cursor)
  const query = params.toString()
  return query ? `?${query}` : ''
}

export async function backendGetCarpoolsPage(filters: CarpoolPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<CarpoolWithMeta>> {
  const response = await backendRequest<ListResponse<BackendCarpoolListing>>(`/api/v1/carpools${carpoolPageQuery(filters, page)}`)
  return {
    items: await mapListings(response.items),
    nextCursor: normalizeNextCursor(response.nextCursor),
  }
}

export async function backendGetCarpools() {
  return collectCursorPages(page => backendGetCarpoolsPage({}, page))
}

export async function backendGetCarpoolById(id: string) {
  const listing = await backendRequest<BackendCarpoolListing>(`/api/v1/carpools/${id}`)
  return mapBackendCarpoolListing(listing)
}

export async function backendCarpoolApplicationEligibility(id: string) {
  await ensureBackendSession('buyer', false)
  return backendRequest<BackendCarpoolApplicationEligibility>(`/api/v1/carpools/${id}/eligibility`)
}

export async function backendOwnerCarpoolsPage(view?: OwnerCarpoolView, page: CursorPageRequest = {}): Promise<CursorPage<CarpoolWithMeta>> {
  await ensureBackendSession('owner', false)
  const response = await backendRequest<ListResponse<BackendCarpoolListing>>(`/api/v1/me/carpools${carpoolPageQuery({ view }, page)}`)
  return {
    items: await mapListings(response.items),
    nextCursor: normalizeNextCursor(response.nextCursor),
  }
}

export async function backendOwnerCarpools(view?: OwnerCarpoolView) {
  return collectCursorPages(page => backendOwnerCarpoolsPage(view, page))
}

function ownerCarpoolEditData(listing: BackendCarpoolListing, plan: BackendProductPlan): OwnerCarpoolEditData {
  return {
    id: listing.id,
    version: listing.version,
    backendStatus: listing.status,
    ownerContactMethodId: listing.ownerContactMethodId ?? '',
    payload: {
      productId: listing.productPlanId,
      customProductName: listing.title !== plan.displayName ? listing.title : null,
      regionCode: listing.regionCode,
      customRegionName: listing.regionCode === 'other' ? listing.regionName : null,
      monthlyPriceCny: numberFromDecimal(listing.priceMonthlyCny),
      serviceMultiplier: numberFromDecimal(listing.serviceMultiplier),
      dailyQuotaAmount: (listing.dailySpendLimitUsd ?? listing.dailyQuotaAmount) ? numberFromDecimal(listing.dailySpendLimitUsd ?? listing.dailyQuotaAmount) : null,
      weeklyQuotaAmount: (listing.weeklySpendLimitUsd ?? listing.weeklyQuotaAmount) ? numberFromDecimal(listing.weeklySpendLimitUsd ?? listing.weeklyQuotaAmount) : null,
      followsOfficialQuotaReset: listing.followsOfficialQuotaReset,
      vpsRegion: listing.vpsRegion ?? '',
      supportsMainlandChinaDirectConnection: listing.supportsMainlandChinaDirectConnection,
      totalSeats: listing.buyerSeatCapacity,
      occupiedSeats: listing.offlineOccupiedSeats ?? listing.activeBuyerMembers,
      openingChannelCode: listing.openingChannelCode ?? '',
      customOpeningChannel: listing.customOpeningChannel ?? '',
      paymentMethodCode: listing.paymentMethodCode ?? '',
      customPaymentMethod: listing.customPaymentMethod ?? '',
      distributionMethod: listing.distributionMethod,
      distributionMethodNote: listing.distributionMethodNote,
      providesAdminAccount: listing.providesAdminAccount,
      accessArrangementMode: mapAccessMode(plan.accessMode),
      accessArrangementNote: listing.accessArrangement,
      riskAcknowledged: listing.riskAckRequired,
      policyVersion: listing.policyVersion,
      riskNoticeCode: listing.riskNoticeCode ?? null,
      warranty: {
        mode: 'remaining_days_compensation',
        fixedWarrantyDays: null,
        compensationMethod: listing.cycleTerm?.exitPolicy ?? '',
        exclusions: '',
      },
      rulesNote: listing.cycleTerm?.usageRules ?? listing.summary,
      status: 'draft',
    },
  }
}

export async function backendOwnerCarpoolForEdit(id: string): Promise<OwnerCarpoolEditData> {
  await ensureBackendSession('owner', false)
  const listing = await backendRequest<BackendCarpoolListing>(`/api/v1/me/carpools/${encodeURIComponent(id)}`)
  return ownerCarpoolEditData(listing, await productPlan(listing.productPlanId))
}

function applicationStatus(application: BackendCarpoolApplication, membership?: BackendCarpoolMembership): CarpoolApplicationWithMeta['status'] {
  if (membership?.status === 'left') return 'cancelled_by_buyer'
  if (membership?.status === 'removed') return 'cancelled_by_owner'
  if (membership?.status === 'active') {
    return 'active'
  }
  if (application.status === 'joined') return 'active'
  if (application.status === 'cancelled_by_buyer') return 'cancelled_by_buyer'
  if (application.status === 'rejected') return 'rejected'
  return 'pending_owner'
}

function membershipForApplication(applicationId: string, perspective: 'buyer' | 'owner') {
  return perspective === 'owner'
    ? backendMembershipsByApplicationOwner.get(applicationId)
    : backendMembershipsByApplication.get(applicationId)
}

async function mapApplication(application: BackendCarpoolApplication, perspective: 'buyer' | 'owner' = 'buyer'): Promise<CarpoolApplicationWithMeta> {
  const plan = await productPlan(application.productPlanId)
  const listing = backendCarpoolListings.get(application.carpoolListingId)
  const membership = membershipForApplication(application.id, perspective)
  const conditions = application.conditionsSnapshot
  const monthly = numberFromDecimal(conditions?.priceMonthlyCny || application.priceMonthlyCny)
  const ownerUsername = ownerLabel(application.ownerUserId)
  const buyerUsername = application.buyerUserId ? `买家 ${application.buyerUserId.slice(0, 8)}` : '买家'
  const status = applicationStatus(application, membership)
  const buyerReputation = mapBackendReputationSummary(application.buyerReputation)
  const ownerReputation = mapBackendReputationSummary(listing?.sellerReputation)
  return {
    id: application.id,
    carpoolId: application.carpoolListingId,
    applicantUserId: application.buyerUserId,
    applicantUsername: buyerUsername,
    applicantStats: {
      linuxdoBound: null,
      trustLevel: null,
      completed30d: buyerReputation?.completedCount ?? null,
      buyerResponsibleCancellations: null,
      ownerResponsibleCancellations: null,
      unresolvedDisputes: buyerReputation?.unresolvedDisputes ?? null,
    },
    buyerReputation,
    ownerUserId: application.ownerUserId,
    ownerUsername,
    status,
    seatsRequested: application.seatCount,
    snapshot: {
      carpoolId: application.carpoolListingId,
      productName: conditions?.title || application.listingTitleSnapshot || plan.displayName,
      regionName: conditions?.regionName || listing?.regionName || '其他',
      monthlyPriceCny: monthly,
      serviceMultiplier: listing ? numberFromDecimal(listing.serviceMultiplier) : undefined,
      dailyQuotaAmount: conditions?.dailySpendLimitUsd ? numberFromDecimal(conditions.dailySpendLimitUsd) : undefined,
      weeklyQuotaAmount: (() => {
        const value = conditions
          ? conditions.weeklySpendLimitUsd
          : listing?.weeklySpendLimitUsd !== undefined
            ? listing.weeklySpendLimitUsd
            : listing?.weeklyQuotaAmount
        return value ? numberFromDecimal(value) : undefined
      })(),
      quotaLabel: listing?.quotaLabel || plan.quotaLabel || defaultQuotaLabel,
      quotaUnit: listing?.quotaUnit || plan.quotaUnit || defaultQuotaUnit,
      quotaPeriod: listing?.quotaPeriod || plan.quotaPeriod || defaultQuotaPeriod,
      priceLabel: '固定月费',
      openingChannelName: '站外成员安排',
      paymentMethodNames: ['站外确认'],
      warrantyText: '车主承诺',
      rulesVersion: formatTime(application.createdAt),
      rulesText: conditions?.cycleTerm?.usageRules || conditions?.cycleTerm?.exitPolicy || listing?.cycleTerm?.usageRules || '规则以申请条件快照为准，平台不托管支付、不保存凭据。',
      ownerUserId: application.ownerUserId,
      ownerUsername,
      ownerTrustLevel: null,
      ownerReputation,
      ownerType: '个人车主',
      accessArrangementMode: mapAccessMode(plan.accessMode),
      accessArrangementNote: conditions?.accessArrangement || listing?.accessArrangement || plan.policyNote,
      riskNoticeCode: application.riskNoticeCode || plan.riskNoticeCode,
      riskAcknowledged: Boolean(application.riskNoticeCode || plan.riskAckRequired),
    },
    startedAt: application.joinedAt ?? membership?.joinedAt ?? null,
    cancellationReasonCode: application.status === 'rejected' ? 'owner_rejected' : membership?.status === 'left' ? 'buyer_left' : membership?.status === 'removed' ? 'owner_removed' : null,
    cancellationReasonText: application.decisionReason || membership?.endedReason || null,
    responsibility: membership?.status === 'left' ? 'buyer' : membership?.status === 'removed' || application.status === 'rejected' ? 'owner' : null,
    disputeReason: null,
    createdAt: application.createdAt,
    updatedAt: application.updatedAt,
    backendVersion: membership ? membership.version : application.version,
    backendContactSessionId: application.contactSessionId,
    backendMembershipId: membership?.id,
    backendStatus: membership?.status ?? application.status,
    backendMembershipJoinedAt: membership?.joinedAt,
    ownerNote: perspective === 'owner' ? membership?.ownerNote : undefined,
    conditionsOutdated: Boolean(listing?.conditionsVersion && application.acceptedConditionsVersion !== listing.conditionsVersion),
    acceptedConditionsVersion: application.acceptedConditionsVersion,
    conditionsVersionSnapshot: application.conditionsVersionSnapshot,
  }
}

async function mapApplications(rows: BackendCarpoolApplication[], perspective: 'buyer' | 'owner' = 'buyer') {
  return Promise.all(rows.map(row => mapApplication(row, perspective)))
}

function filterApplications(rows: CarpoolApplicationWithMeta[], filters: CarpoolApplicationFilters = {}) {
  const statuses = Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : null
  const search = filters.search?.trim().toLowerCase()
  return rows.filter(row => {
    return (!statuses || statuses.includes(row.status))
      && (!filters.carpoolId || row.carpoolId === filters.carpoolId)
      && (!search || [row.id, row.snapshot.productName, row.applicantUsername, row.ownerUsername].some(value => value.toLowerCase().includes(search)))
  }).sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
}

async function loadBuyerMemberships() {
  const items = await collectCursorPages(async (page) => {
    const response = await backendRequest<ListResponse<BackendCarpoolMembership>>(`/api/v1/me/carpool-memberships${carpoolPageQuery({}, page)}`)
    return { items: response.items, nextCursor: normalizeNextCursor(response.nextCursor) }
	})
  backendMembershipsByApplication.clear()
	for (const membership of items) {
    backendMembershipsByApplication.set(membership.carpoolApplicationId, membership)
  }
	return items
}

async function loadOwnerMemberships() {
	const items = await collectCursorPages(async (page) => {
		const response = await backendRequest<ListResponse<BackendCarpoolMembership>>(`/api/v1/owner/carpool-memberships${carpoolPageQuery({}, page)}`)
		return { items: response.items, nextCursor: normalizeNextCursor(response.nextCursor) }
	})
  backendMembershipsByApplicationOwner.clear()
	for (const membership of items) {
    backendMembershipsByApplicationOwner.set(membership.carpoolApplicationId, membership)
  }
	return items
}

function carpoolApplicationPageFilters(filters: CarpoolApplicationFilters): CarpoolPageFilters {
  return {
    q: filters.search,
    statuses: Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : undefined,
    carpoolId: filters.carpoolId,
    sort: filters.sort,
  }
}

export async function backendMyCarpoolApplicationsPage(filters: CarpoolApplicationFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<CarpoolApplicationWithMeta>> {
  await ensureBackendSession('buyer', false)
  await loadBuyerMemberships()
	const response = await backendRequest<ListResponse<BackendCarpoolApplication>>(`/api/v1/me/carpool-applications${carpoolPageQuery(carpoolApplicationPageFilters(filters), page)}`)
	return {
		items: await mapApplications(response.items, 'buyer'),
		nextCursor: normalizeNextCursor(response.nextCursor),
	}
}

export async function backendMyCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  return collectCursorPages(page => backendMyCarpoolApplicationsPage(filters, page))
}

export async function backendMerchantCarpoolApplicationsPage(filters: CarpoolApplicationFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<CarpoolApplicationWithMeta>> {
  await ensureBackendSession('owner', false)
  await loadOwnerMemberships()
	const response = await backendRequest<ListResponse<BackendCarpoolApplication>>(`/api/v1/owner/carpool-applications${carpoolPageQuery(carpoolApplicationPageFilters(filters), page)}`)
	return {
		items: await mapApplications(response.items, 'owner'),
		nextCursor: normalizeNextCursor(response.nextCursor),
	}
}

export async function backendMerchantCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  return collectCursorPages(page => backendMerchantCarpoolApplicationsPage(filters, page))
}

export async function backendCarpoolApplicationById(id: string) {
  try {
    await ensureBackendSession('buyer', false)
    await loadBuyerMemberships()
    const application = await backendRequest<BackendCarpoolApplication>(`/api/v1/me/carpool-applications/${id}`)
    return mapApplication(application, 'buyer')
  } catch {
    await ensureBackendSession('owner', false)
    await loadOwnerMemberships()
    const application = await backendRequest<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}`)
    return mapApplication(application, 'owner')
  }
}

export async function backendCarpoolApplicationEvents(id: string) {
  const application = await backendCarpoolApplicationById(id)
  const events: CarpoolApplicationEvent[] = [{
    id: `backend-carpool-event-${application.id}`,
    applicationId: application.id,
    actorId: application.applicantUserId,
    actorLabel: application.applicantUsername,
    actorRole: 'buyer' as const,
    type: 'application_created' as const,
    toStatus: application.status,
    note: '真实后端申请记录。',
    createdAt: application.createdAt,
  }]
  return events
}

function contactItem(item: BackendContactSessionContacts['items'][number]): OrderContactSnapshotItem {
  const usageScope = item.side === 'seller' ? 'carpool_owner' : 'buyer'
  return {
    type: item.type,
    label: item.label,
    maskedValue: item.maskedValue,
    displayValue: item.value,
    verified: item.type === 'linuxdo',
    usageScope,
    actionUrl: item.type === 'linuxdo' ? linuxDoProfileSummaryUrl(item.value) : undefined,
  }
}

export async function backendCarpoolApplicationContacts(applicationId: string): Promise<OrderContactSnapshot> {
  const application = await backendCarpoolApplicationById(applicationId)
  if (!application.backendContactSessionId) {
    return {
      id: `backend-carpool-contact-blocked-${applicationId}`,
      orderType: 'carpool_application',
      orderId: applicationId,
      sellerContacts: [],
      buyerContacts: [],
      contactWindowEndsAt: null,
      canView: false,
      unavailableReason: '车主确认上车并建立有效成员关系后才展示联系方式。',
      createdAt: application.createdAt,
    }
  }
  const response = await backendRequest<BackendContactSessionContacts>(`/api/v1/contact-sessions/${application.backendContactSessionId}/contacts`)
  trackAnalytics('contact_window_reveal', {
    entity_type: 'carpool_application',
    source_route: '/my/rides/:id',
  })
  return {
    id: response.sessionId,
    orderType: 'carpool_application',
    orderId: applicationId,
    sellerContacts: response.items.filter(item => item.side === 'seller').map(contactItem),
    buyerContacts: response.items.filter(item => item.side === 'buyer').map(contactItem),
    contactWindowEndsAt: response.endsAt,
    canView: true,
    unavailableReason: null,
    createdAt: application.updatedAt,
  }
}

function riskAcknowledgement(plan: BackendProductPlan | undefined, payloadRiskNoticeCode?: string | null, policyVersion?: number | null, acknowledged?: boolean) {
  const riskNoticeCode = payloadRiskNoticeCode || plan?.riskNoticeCode
  const version = policyVersion || plan?.policyVersion
  if (!plan?.riskAckRequired && !riskNoticeCode) return undefined
  if (!acknowledged || !riskNoticeCode || !version) return undefined
  return { riskNoticeCode, policyVersion: version }
}

function toListingRequest(payload: SaveCarpoolDraftPayload, ownerContactMethodId: string, plan: BackendProductPlan | undefined) {
  const monthly = payload.monthlyPriceCny ?? 0
  const regionName = payload.customRegionName?.trim() || carpoolRegions.find(item => item.code === payload.regionCode)?.displayName || '其他'
  return {
    productPlanId: payload.productId,
    ownerContactMethodId,
    cycleTerm: {
      billingPeriod: 'monthly',
      cycleStartDay: null,
      noticeDays: 1,
      exitPolicy: payload.warranty.compensationMethod || '退出与补偿由双方站外确认，平台不托管支付、不担保。',
      usageRules: payload.rulesNote,
    },
    title: payload.customProductName?.trim() || plan?.displayName || '拼车车源',
    summary: payload.rulesNote,
    accessArrangement: payload.accessArrangementNote || '站外成员安排，平台不保存、不提供账号凭据。',
    distributionMethod: payload.distributionMethod || 'other',
    distributionMethodNote: payload.distributionMethod === 'other' ? payload.distributionMethodNote?.trim() || '站外分发方式待确认。' : payload.distributionMethodNote?.trim() || '',
    providesAdminAccount: payload.distributionMethod === 'account_login' ? false : Boolean(payload.providesAdminAccount),
    regionCode: payload.regionCode,
    regionName,
    priceMonthlyCny: String(monthly),
    serviceMultiplier: String(payload.serviceMultiplier ?? 1),
    dailySpendLimitUsd: payload.dailyQuotaAmount === null ? null : String(payload.dailyQuotaAmount),
    weeklySpendLimitUsd: payload.weeklyQuotaAmount === null ? null : String(payload.weeklyQuotaAmount),
    followsOfficialQuotaReset: payload.followsOfficialQuotaReset,
    vpsRegion: payload.vpsRegion.trim() || null,
    supportsMainlandChinaDirectConnection: payload.supportsMainlandChinaDirectConnection,
    openingChannelCode: payload.openingChannelCode,
    customOpeningChannel: payload.openingChannelCode === 'other' ? payload.customOpeningChannel.trim() : '',
    paymentMethodCode: payload.paymentMethodCode,
    customPaymentMethod: payload.paymentMethodCode === 'other' ? payload.customPaymentMethod.trim() : '',
    buyerSeatCapacity: payload.totalSeats,
    offlineOccupiedSeats: payload.occupiedSeats,
    riskAcknowledgement: riskAcknowledgement(plan, payload.riskNoticeCode, payload.policyVersion, payload.riskAcknowledged),
  }
}

export async function backendSubmitCarpool(payload: SaveCarpoolDraftPayload) {
  await ensureBackendSession('owner', false)
  const plan = await productPlan(payload.productId)
	const ownerContact = await backendBoundLinuxDoContactMethod()
  const publish = payload.status === 'reviewing'
  const listing = await backendMutation<BackendCarpoolListing>(publish ? '/api/v1/carpools/publish' : '/api/v1/carpools', toListingRequest(payload, ownerContact.id, plan), {
    idempotencyPrefix: publish ? 'carpool-publish' : 'carpool-listing',
  })
  return mapBackendCarpoolListing(listing)
}

export async function backendUpdateOwnerCarpool(
	id: string,
	payload: SaveCarpoolDraftPayload,
	version: number,
	ownerContactMethodId: string,
	submitForReview: boolean,
) {
	await ensureBackendSession('owner', false)
	const plan = await productPlan(payload.productId)
	let listing = await backendMutation<BackendCarpoolListing>(
		`/api/v1/carpools/${encodeURIComponent(id)}`,
		toListingRequest(payload, ownerContactMethodId, plan),
		{ method: 'PATCH', ifMatch: version, idempotencyPrefix: 'carpool-update' },
	)
	if (submitForReview) {
		listing = await backendMutation<BackendCarpoolListing>(
			`/api/v1/carpools/${encodeURIComponent(id)}/submit-review`,
			{},
			{ ifMatch: listing.version, idempotencyPrefix: 'carpool-submit-review' },
		)
	}
	return mapBackendCarpoolListing(listing)
}

export async function backendCreateCarpoolApplication(carpoolId: string, payload: { rulesAccepted: boolean }) {
  if (!payload.rulesAccepted) throw new Error('请先确认已阅读车源规则和车主承诺说明')
  await ensureBackendSession('buyer', false)
  const listing = await backendRequest<BackendCarpoolListing>(`/api/v1/carpools/${carpoolId}`)
  backendCarpoolListings.set(listing.id, listing)
  const plan = await productPlan(listing.productPlanId)
	const buyerContact = await backendBoundLinuxDoContactMethod()
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/carpools/${carpoolId}/applications`, {
    buyerContactMethodId: buyerContact.id,
    riskAcknowledgement: riskAcknowledgement(plan, listing.riskNoticeCode, listing.policyVersion, true),
  }, { idempotencyPrefix: 'carpool-application' })
  return mapApplication(response, 'buyer')
}

async function ownerApplication(id: string) {
  await ensureBackendSession('owner', false)
  return backendRequest<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}`)
}

async function buyerApplication(id: string) {
  await ensureBackendSession('buyer', false)
  return backendRequest<BackendCarpoolApplication>(`/api/v1/me/carpool-applications/${id}`)
}

export async function backendAcceptCarpoolApplication(id: string) {
  const current = await ownerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}/accept`, {}, {
    idempotencyPrefix: 'carpool-accept',
    ifMatch: current.version,
  })
  await loadOwnerMemberships()
  return mapApplication(response, 'owner')
}

export async function backendConfirmCarpoolApplicationConditions(id: string) {
  const current = await buyerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/me/carpool-applications/${id}/confirm-conditions`, {}, {
    ifMatch: current.version,
  })
  return mapApplication(response, 'buyer')
}

export async function backendUpdateCarpoolRecruitment(id: string, action: 'stop' | 'resume') {
  await ensureBackendSession('owner', false)
  const current = await backendRequest<BackendCarpoolListing>(`/api/v1/me/carpools/${encodeURIComponent(id)}`)
  const response = await backendMutation<BackendCarpoolListing>(`/api/v1/me/carpools/${encodeURIComponent(id)}/${action}-recruiting`, {}, {
    ifMatch: current.version,
  })
  return mapBackendCarpoolListing(response)
}

export async function backendRejectCarpoolApplication(id: string, reason: string) {
  const current = await ownerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}/reject`, { reason }, {
    idempotencyPrefix: 'carpool-reject',
    ifMatch: current.version,
  })
  return mapApplication(response, 'owner')
}

export async function backendCancelCarpoolApplication(id: string, reason: string) {
  const current = await buyerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/me/carpool-applications/${id}/cancel`, { reason }, {
    idempotencyPrefix: 'carpool-cancel',
    ifMatch: current.version,
  })
  return mapApplication(response, 'buyer')
}

async function membershipForAction(applicationId: string, perspective: 'buyer' | 'owner') {
  if (perspective === 'owner') {
    await ensureBackendSession('owner', false)
    await loadOwnerMemberships()
    const membership = backendMembershipsByApplicationOwner.get(applicationId)
    if (!membership) throw new Error('该申请还没有形成有效成员关系。')
    return membership
  }
  await ensureBackendSession('buyer', false)
  await loadBuyerMemberships()
  const membership = backendMembershipsByApplication.get(applicationId)
  if (!membership) throw new Error('该申请还没有形成有效成员关系。')
  return membership
}

export async function backendBuyerLeaveCarpool(applicationId: string, reason: string) {
  const membership = await membershipForAction(applicationId, 'buyer')
  const response = await backendMutation<BackendCarpoolMembership>(`/api/v1/me/carpool-memberships/${membership.id}/leave`, { reason }, {
    idempotencyPrefix: 'carpool-buyer-leave',
    ifMatch: membership.version,
  })
  backendMembershipsByApplication.set(response.carpoolApplicationId, response)
  return mapApplication(await buyerApplication(applicationId), 'buyer')
}

export async function backendOwnerRemoveCarpool(applicationId: string, reason: string) {
  const membership = await membershipForAction(applicationId, 'owner')
  const response = await backendMutation<BackendCarpoolMembership>(`/api/v1/owner/carpool-memberships/${membership.id}/remove`, { reason }, {
    idempotencyPrefix: 'carpool-owner-remove',
    ifMatch: membership.version,
  })
  backendMembershipsByApplicationOwner.set(response.carpoolApplicationId, response)
  return mapApplication(await ownerApplication(applicationId), 'owner')
}

export async function backendUpdateCarpoolMembershipOwnerNote(membershipId: string, note: string, version: number) {
  await ensureBackendSession('owner', false)
  const response = await backendMutation<BackendCarpoolMembership>(`/api/v1/owner/carpool-memberships/${membershipId}/note`, { note }, {
    method: 'PATCH',
    idempotencyPrefix: 'carpool-owner-note',
    ifMatch: version,
  })
  backendMembershipsByApplicationOwner.set(response.carpoolApplicationId, response)
  return mapApplication(await ownerApplication(response.carpoolApplicationId), 'owner')
}

function carpoolStatusLabel(listing: BackendCarpoolListing) {
  if (listing.governanceStatus === 'removed') return '已下架'
  if (listing.status === 'pending_review') return '待处理'
  if (listing.status === 'changes_requested') return '待复核'
  if (listing.status === 'active') return '招募中'
  if (listing.status === 'stopped') return '已停止招募'
  if (listing.status === 'rejected') return '已拒绝'
  return '草稿'
}

async function carpoolAdminRow(listing: BackendCarpoolListing): Promise<AdminRow> {
  backendCarpoolListings.set(listing.id, listing)
  const plan = await productPlan(listing.productPlanId)
  return {
    id: listing.id,
    primary: listing.title || plan.displayName,
    secondary: `${plan.displayName} · ¥${numberFromDecimal(listing.priceMonthlyCny)}/月 · 可申请 ${listing.availableSeats}/${listing.buyerSeatCapacity} 席`,
    owner: `${ownerLabel(listing.ownerUserId)} · 真实后端用户`,
    status: carpoolStatusLabel(listing),
    risk: listing.riskAckRequired ? `风险确认 ${listing.riskNoticeCode || plan.riskNoticeCode || 'required'}` : '普通车源',
    targetType: 'carpool',
    detailItems: [
      { label: '后端状态', value: listing.status },
      { label: '治理状态', value: listing.governanceStatus },
      { label: '版本', value: String(listing.version) },
      { label: '访问安排', value: listing.accessArrangement },
      { label: '规则说明', value: listing.cycleTerm?.usageRules || listing.summary },
    ],
    targetTo: `/carpools/${listing.id}`,
  }
}

export async function backendAdminCarpoolRowsPage(filters: CarpoolPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<AdminRow>> {
  await ensureBackendSession('admin', true)
	const response = await backendRequest<ListResponse<BackendCarpoolListing>>(`/api/v1/admin/carpools${carpoolPageQuery(filters, page)}`)
	return {
		items: await Promise.all(response.items.map(carpoolAdminRow)),
		nextCursor: normalizeNextCursor(response.nextCursor),
	}
}

export async function backendAdminCarpoolRows() {
  return collectCursorPages(page => backendAdminCarpoolRowsPage({}, page))
}

async function backendAdminCarpoolAction(id: string, action: 'approve' | 'request-changes' | 'reject' | 'pause' | 'restore', reason: string) {
  await ensureBackendSession('admin', true)
  const current = await backendRequest<BackendCarpoolListing>(`/api/v1/admin/carpools/${id}`)
  const response = await backendMutation<BackendCarpoolListing>(`/api/v1/admin/carpools/${id}/${action}`, { reason }, {
    idempotencyPrefix: `carpool-admin-${action}`,
    ifMatch: current.version,
  })
  return carpoolAdminRow(response)
}

export async function backendUpdateAdminCarpoolStatus(row: AdminRow, status: string, reason: string) {
  if (row.targetType !== 'carpool') return row
  const action = status === '已下架' ? 'pause' : status === '已恢复' ? 'restore' : status === '已通过' ? 'approve' : 'request-changes'
  return backendAdminCarpoolAction(row.id, action, reason || '管理台发布治理操作')
}

export async function backendRunAdminCarpoolAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (row.targetType !== 'carpool') return row
  const backendAction = action === 'request_changes'
    ? 'request-changes'
    : action === 'take_down' || action === 'suspend'
      ? 'pause'
      : action === 'restore'
        ? 'restore'
        : action === 'approve'
          ? 'approve'
          : 'reject'
  return backendAdminCarpoolAction(row.id, backendAction, reason || '管理台操作')
}
