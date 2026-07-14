import type {
  AdminRow,
  CarpoolApplicationEvent,
  CarpoolApplicationEligibility,
  CarpoolApplicationWithMeta,
  CarpoolApplicationFilters,
  CarpoolProductCatalogItem,
  CarpoolWithMeta,
  ContactMethodType,
  OrderContactSnapshot,
  OrderContactSnapshotItem,
  PaymentMethodOption,
  RegionOption,
  SaveCarpoolDraftPayload,
} from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { backendCreateContactMethod } from '@/lib/apiMarketBackend'
import { carpoolOpeningChannels, carpoolPaymentMethods, carpoolRegions } from '@/data/mock'
import { defaultQuotaLabel, defaultQuotaPeriod, defaultQuotaUnit } from '@/lib/quota'

type ListResponse<T> = { items: T[] }

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
  priceMonthlyCny: string
  serviceMultiplier: string
  monthlyQuotaAmount: string
  quotaLabel: string
  quotaUnit: string
  quotaPeriod: 'monthly'
  buyerSeatCapacity: number
  activeBuyerMembers: number
  reservedSeats: number
  availableSeats: number
  status: string
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
  contactSessionId?: string
  reservationExpiresAt?: string
  joinConfirmationDeadline?: string
  buyerConfirmedAt?: string
  ownerConfirmedAt?: string
  joinedAt?: string
  decisionReason?: string
  decidedAt?: string
  version: number
  createdAt: string
  updatedAt: string
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
  buyerCompletedAt?: string
  ownerCompletedAt?: string
  completedAt?: string
  endedAt?: string
  endedReason?: string
  endedByUserId?: string
  version: number
  createdAt: string
  updatedAt: string
}

type BackendContactSessionContacts = {
  sessionId: string
  endsAt: string
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

function numberFromDecimal(value: string | undefined, fallback = 0) {
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
  return value === 'openai' || value === 'anthropic' ? value : 'other'
}

function mapCategoryCode(value: string): CarpoolProductCatalogItem['categoryCode'] {
  if (value === 'gpt' || value === 'claude' || value === 'cursor' || value === 'gemini' || value === 'perplexity') return value
  return 'other'
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

export async function mapBackendCarpoolListing(listing: BackendCarpoolListing): Promise<CarpoolWithMeta> {
  backendCarpoolListings.set(listing.id, listing)
  const plan = await productPlan(listing.productPlanId)
  const monthly = numberFromDecimal(listing.priceMonthlyCny)
  const serviceMultiplier = numberFromDecimal(listing.serviceMultiplier)
  const monthlyQuotaAmount = numberFromDecimal(listing.monthlyQuotaAmount)
  const activeSeats = Math.max(0, listing.activeBuyerMembers)
  const totalSeats = Math.max(1, listing.buyerSeatCapacity)
  const availableSeats = Math.max(0, listing.availableSeats)
  return {
    id: listing.id,
    product: plan.displayName,
    region: listing.regionName,
    monthly,
    serviceMultiplier,
    monthlyQuotaAmount,
    quotaLabel: listing.quotaLabel || plan.quotaLabel || defaultQuotaLabel,
    quotaUnit: listing.quotaUnit || plan.quotaUnit || defaultQuotaUnit,
    quotaPeriod: listing.quotaPeriod || plan.quotaPeriod || defaultQuotaPeriod,
    seats: `${activeSeats}/${totalSeats}`,
    pricingMode: 'fixed',
    fixedMonthlyPrice: monthly,
    currentConfirmedMembers: activeSeats,
    maxMembers: totalSeats,
    owner: ownerLabel(listing.ownerUserId),
    ownerUserId: listing.ownerUserId,
    trustLevel: 4,
    ownerType: '个人车主',
    warranty: '车主承诺',
    openingMethod: openingMethodFromAccessMode(plan.accessMode),
    status: listingStatus(listing.status, availableSeats),
    confirmedAt: formatTime(listing.updatedAt),
    confirmedWithin48h: true,
    linuxdoBound: Boolean(listing.sourceUrl),
    sourcePostAccessible: Boolean(listing.sourceUrl),
    hasInfoConflict: false,
    hasUnresolvedDispute: false,
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
    seatSummary: {
      carpoolId: listing.id,
      totalSeats,
      activeMemberCount: activeSeats,
      reservedSeatCount: Math.max(0, listing.reservedSeats),
      availableSeats,
    },
  }
}

async function mapListings(rows: BackendCarpoolListing[]) {
  return Promise.all(rows.map(mapBackendCarpoolListing))
}

export async function backendGetCarpools() {
  const response = await backendRequest<ListResponse<BackendCarpoolListing>>('/api/v1/carpools')
  return mapListings(response.items)
}

export async function backendGetCarpoolById(id: string) {
  const listing = await backendRequest<BackendCarpoolListing>(`/api/v1/carpools/${id}`)
  return mapBackendCarpoolListing(listing)
}

export async function backendCarpoolApplicationEligibility(id: string) {
  await ensureBackendSession('buyer', false)
  return backendRequest<BackendCarpoolApplicationEligibility>(`/api/v1/carpools/${id}/eligibility`)
}

export async function backendOwnerCarpools() {
  await ensureBackendSession('owner', false)
  const response = await backendRequest<ListResponse<BackendCarpoolListing>>('/api/v1/me/carpools')
  return mapListings(response.items)
}

function applicationStatus(application: BackendCarpoolApplication, membership?: BackendCarpoolMembership): CarpoolApplicationWithMeta['status'] {
  if (membership?.status === 'completed') return 'completed'
  if (membership?.status === 'left') return 'cancelled_by_buyer'
  if (membership?.status === 'removed') return 'cancelled_by_owner'
  if (membership?.status === 'active') {
    if (membership.buyerCompletedAt || membership.ownerCompletedAt) return 'pending_completion'
    return 'active'
  }
  if (application.status === 'accepted_reserved') {
    if (application.buyerConfirmedAt || application.ownerConfirmedAt) return 'joined_pending_confirmation'
    return 'accepted_reserved'
  }
  if (application.status === 'joined') return 'active'
  if (application.status === 'cancelled_by_buyer') return 'cancelled_by_buyer'
  if (application.status === 'cancelled_by_owner') return 'cancelled_by_owner'
  if (application.status === 'expired') return 'expired'
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
  const monthly = numberFromDecimal(application.priceMonthlyCny)
  const ownerUsername = ownerLabel(application.ownerUserId)
  const buyerUsername = application.buyerUserId ? `买家 ${application.buyerUserId.slice(0, 8)}` : '买家'
  const status = applicationStatus(application, membership)
  return {
    id: application.id,
    carpoolId: application.carpoolListingId,
    applicantUserId: application.buyerUserId,
    applicantUsername: buyerUsername,
    applicantStats: { linuxdoBound: true, trustLevel: 3, completed30d: 0, buyerResponsibleCancellations: 0, ownerResponsibleCancellations: 0, unresolvedDisputes: 0 },
    ownerUserId: application.ownerUserId,
    ownerUsername,
    status,
    seatsRequested: application.seatCount,
    snapshot: {
      carpoolId: application.carpoolListingId,
      productName: application.listingTitleSnapshot || plan.displayName,
      regionName: listing?.regionName || '其他',
      monthlyPriceCny: monthly,
      serviceMultiplier: listing ? numberFromDecimal(listing.serviceMultiplier) : undefined,
      monthlyQuotaAmount: listing ? numberFromDecimal(listing.monthlyQuotaAmount) : undefined,
      quotaLabel: listing?.quotaLabel || plan.quotaLabel || defaultQuotaLabel,
      quotaUnit: listing?.quotaUnit || plan.quotaUnit || defaultQuotaUnit,
      quotaPeriod: listing?.quotaPeriod || plan.quotaPeriod || defaultQuotaPeriod,
      priceLabel: '固定月费',
      openingChannelName: '站外成员安排',
      paymentMethodNames: ['站外确认'],
      warrantyText: '车主承诺',
      rulesVersion: formatTime(application.createdAt),
      rulesText: listing?.cycleTerm?.usageRules || listing?.cycleTerm?.exitPolicy || '规则以车源发布时说明为准，平台不托管支付、不保存凭据。',
      ownerUserId: application.ownerUserId,
      ownerUsername,
      ownerTrustLevel: 4,
      ownerType: '个人车主',
      sourceTopicUrl: listing?.sourceUrl || '',
      accessArrangementMode: mapAccessMode(plan.accessMode),
      accessArrangementNote: listing?.accessArrangement || plan.policyNote,
      riskNoticeCode: application.riskNoticeCode || plan.riskNoticeCode,
      riskAcknowledged: Boolean(application.riskNoticeCode || plan.riskAckRequired),
    },
    reservedUntil: application.reservationExpiresAt ?? null,
    buyerContactedAt: application.contactSessionId ? application.updatedAt : null,
    buyerConfirmedJoinedAt: application.buyerConfirmedAt ?? null,
    ownerConfirmedJoinedAt: application.ownerConfirmedAt ?? null,
    startedAt: application.joinedAt ?? membership?.joinedAt ?? null,
    expectedEndAt: null,
    buyerConfirmedCompletedAt: membership?.buyerCompletedAt ?? null,
    ownerConfirmedCompletedAt: membership?.ownerCompletedAt ?? null,
    completedAt: membership?.completedAt ?? null,
    completionMode: membership?.completedAt ? 'mutual' : null,
    cancellationReasonCode: application.status === 'rejected' ? 'owner_rejected' : membership?.status === 'left' ? 'buyer_left' : membership?.status === 'removed' ? 'owner_removed' : null,
    cancellationReasonText: application.decisionReason || membership?.endedReason || null,
    responsibility: membership?.status === 'left' ? 'buyer' : membership?.status === 'removed' || application.status === 'rejected' ? 'owner' : null,
    disputeReason: null,
    createdAt: application.createdAt,
    updatedAt: application.updatedAt,
    backendVersion: membership && ['active', 'pending_completion', 'completed'].includes(status) ? membership.version : application.version,
    backendContactSessionId: application.contactSessionId,
    backendMembershipId: membership?.id,
    backendStatus: membership?.status ?? application.status,
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
  const response = await backendRequest<ListResponse<BackendCarpoolMembership>>('/api/v1/me/carpool-memberships')
  backendMembershipsByApplication.clear()
  for (const membership of response.items) {
    backendMembershipsByApplication.set(membership.carpoolApplicationId, membership)
  }
  return response.items
}

async function loadOwnerMemberships() {
  const response = await backendRequest<ListResponse<BackendCarpoolMembership>>('/api/v1/owner/carpool-memberships')
  backendMembershipsByApplicationOwner.clear()
  for (const membership of response.items) {
    backendMembershipsByApplicationOwner.set(membership.carpoolApplicationId, membership)
  }
  return response.items
}

export async function backendMyCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  await ensureBackendSession('buyer', false)
  await loadBuyerMemberships()
  const response = await backendRequest<ListResponse<BackendCarpoolApplication>>('/api/v1/me/carpool-applications')
  const rows = await mapApplications(response.items, 'buyer')
  return filterApplications(rows, filters)
}

export async function backendMerchantCarpoolApplications(filters: CarpoolApplicationFilters = {}) {
  await ensureBackendSession('owner', false)
  await loadOwnerMemberships()
  const response = await backendRequest<ListResponse<BackendCarpoolApplication>>('/api/v1/owner/carpool-applications')
  const rows = await mapApplications(response.items, 'owner')
  return filterApplications(rows, filters)
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
    actionUrl: item.type === 'linuxdo' ? `https://linux.do/u/${item.value.replace(/^@/, '')}/messages/new` : undefined,
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
      contactWindowEndsAt: application.reservedUntil,
      canView: false,
      unavailableReason: '车主接受申请并开启联系窗口后才展示联系方式。',
      createdAt: application.createdAt,
    }
  }
  const response = await backendRequest<BackendContactSessionContacts>(`/api/v1/contact-sessions/${application.backendContactSessionId}/contacts`)
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
    distributionMethodNote: payload.distributionMethodNote?.trim() || '站外分发方式待确认。',
    providesAdminAccount: Boolean(payload.providesAdminAccount),
    regionCode: payload.regionCode,
    regionName,
    sourceUrl: payload.linuxDoTopicUrl,
    priceMonthlyCny: String(monthly),
    serviceMultiplier: String(payload.serviceMultiplier ?? 1),
    monthlyQuotaAmount: String(payload.monthlyQuotaAmount ?? 0),
    buyerSeatCapacity: payload.totalSeats,
    activeBuyerMembers: payload.occupiedSeats,
    riskAcknowledgement: riskAcknowledgement(plan, payload.riskNoticeCode, payload.policyVersion, payload.riskAcknowledged),
  }
}

export async function backendSubmitCarpool(payload: SaveCarpoolDraftPayload) {
  await ensureBackendSession('owner', false)
  const plan = await productPlan(payload.productId)
  const ownerContact = await backendCreateContactMethod({
    type: 'linuxdo',
    label: 'linux.do 私信',
    displayValue: '@owner',
    usageScopes: ['carpool_owner'],
    isDefault: true,
    enabled: true,
  })
  const publish = payload.status === 'reviewing'
  const listing = await backendMutation<BackendCarpoolListing>(publish ? '/api/v1/carpools/publish' : '/api/v1/carpools', toListingRequest(payload, ownerContact.id, plan), {
    idempotencyPrefix: publish ? 'carpool-publish' : 'carpool-listing',
  })
  return mapBackendCarpoolListing(listing)
}

export async function backendCreateCarpoolApplication(carpoolId: string, payload: { rulesAccepted: boolean }) {
  if (!payload.rulesAccepted) throw new Error('请先确认已阅读车源规则和车主承诺说明')
  await ensureBackendSession('buyer', false)
  const listing = await backendRequest<BackendCarpoolListing>(`/api/v1/carpools/${carpoolId}`)
  backendCarpoolListings.set(listing.id, listing)
  const plan = await productPlan(listing.productPlanId)
  const buyerContact = await backendCreateContactMethod({
    type: 'linuxdo',
    label: 'linux.do 私信',
    displayValue: '@buyer',
    usageScopes: ['buyer'],
    isDefault: true,
    enabled: true,
  })
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
  return mapApplication(response, 'owner')
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

export async function backendWithdrawCarpoolAcceptance(id: string, reason: string) {
  const current = await ownerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}/withdraw-acceptance`, { reason }, {
    idempotencyPrefix: 'carpool-withdraw-acceptance',
    ifMatch: current.version,
  })
  return mapApplication(response, 'owner')
}

export async function backendBuyerConfirmCarpoolJoined(id: string) {
  const current = await buyerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/me/carpool-applications/${id}/confirm-join`, {}, {
    idempotencyPrefix: 'carpool-buyer-join',
    ifMatch: current.version,
  })
  await loadBuyerMemberships()
  return mapApplication(response, 'buyer')
}

export async function backendOwnerConfirmCarpoolJoined(id: string) {
  const current = await ownerApplication(id)
  const response = await backendMutation<BackendCarpoolApplication>(`/api/v1/owner/carpool-applications/${id}/confirm-join`, {}, {
    idempotencyPrefix: 'carpool-owner-join',
    ifMatch: current.version,
  })
  await loadOwnerMemberships()
  return mapApplication(response, 'owner')
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

export async function backendBuyerConfirmCarpoolCompleted(applicationId: string) {
  const membership = await membershipForAction(applicationId, 'buyer')
  const response = await backendMutation<BackendCarpoolMembership>(`/api/v1/me/carpool-memberships/${membership.id}/confirm-complete`, {}, {
    idempotencyPrefix: 'carpool-buyer-complete',
    ifMatch: membership.version,
  })
  backendMembershipsByApplication.set(response.carpoolApplicationId, response)
  return mapApplication(await buyerApplication(applicationId), 'buyer')
}

export async function backendOwnerConfirmCarpoolCompleted(applicationId: string) {
  const membership = await membershipForAction(applicationId, 'owner')
  const response = await backendMutation<BackendCarpoolMembership>(`/api/v1/owner/carpool-memberships/${membership.id}/confirm-complete`, {}, {
    idempotencyPrefix: 'carpool-owner-complete',
    ifMatch: membership.version,
  })
  backendMembershipsByApplicationOwner.set(response.carpoolApplicationId, response)
  return mapApplication(await ownerApplication(applicationId), 'owner')
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

function carpoolStatusLabel(listing: BackendCarpoolListing) {
  if (listing.status === 'pending_review') return '待处理'
  if (listing.status === 'changes_requested') return '待复核'
  if (listing.status === 'active') return '可上车'
  if (listing.status === 'paused') return '暂停'
  if (listing.status === 'rejected') return '已拒绝'
  if (listing.status === 'removed') return '已移除'
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
      { label: '版本', value: String(listing.version) },
      { label: '访问安排', value: listing.accessArrangement },
      { label: '规则说明', value: listing.cycleTerm?.usageRules || listing.summary },
    ],
    targetTo: `/carpools/${listing.id}`,
  }
}

export async function backendAdminCarpoolRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendCarpoolListing>>('/api/v1/admin/carpools')
  return Promise.all(response.items.map(carpoolAdminRow))
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
