import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

const myCarpoolsSource = readFileSync(new URL('../../pages/MyCarpoolsPage.vue', import.meta.url), 'utf8')
const managementSource = readFileSync(new URL('../../pages/CarpoolMembershipManagementPage.vue', import.meta.url), 'utf8')
const publishSource = readFileSync(new URL('../../pages/CarpoolPublishPage.vue', import.meta.url), 'utf8')

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  })
}

function createStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem(key: string) { return values.get(key) ?? null },
    setItem(key: string, value: string) { values.set(key, value) },
    removeItem(key: string) { values.delete(key) },
  }
}

function backendSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-carpool',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'owner-id',
      analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
      username: 'owner',
      displayName: 'Owner',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
}

function productPlan() {
  return {
    id: 'plan-id',
    categoryCode: 'gpt',
    providerCode: 'openai',
    slug: 'test-plan',
    displayName: '测试套餐',
    description: '',
    publishPolicy: 'allowed',
    accessMode: 'provider_member_invitation',
    providerPolicyStatus: 'unknown',
    riskLevel: 'normal',
    riskAckRequired: true,
    riskNoticeCode: 'shared-account-risk',
    policyVersion: 7,
    policyNote: '',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    allowCustomVariant: false,
    sortOrder: 1,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  }
}

function backendWechatContactList() {
  return {
    items: [{
      id: 'contact-id',
      userId: 'owner-id',
      type: 'wechat',
      label: '微信',
      maskedValue: 'own***',
      displayValue: 'owner_wechat',
      usageScopes: ['carpool_owner', 'api_merchant', 'buyer', 'dispute'],
      isDefault: true,
      enabled: true,
      verified: false,
      createdAt: '2026-08-01T00:00:00Z',
      updatedAt: '2026-08-01T00:00:00Z',
    }],
    nextCursor: null,
  }
}

function ownerListing(overrides: Record<string, unknown> = {}) {
  return {
    id: 'listing-id',
    ownerUserId: 'owner-id',
    ownerContactMethodId: 'contact-id',
    productPlanId: 'plan-id',
    cycleTerm: {
      id: 'cycle-id',
      billingPeriod: 'monthly',
      cycleStartDay: 1,
      noticeDays: 2,
      exitPolicy: '按剩余天数补偿',
      usageRules: '不得转售',
      version: 1,
      createdAt: '2026-08-01T00:00:00Z',
      updatedAt: '2026-08-01T00:00:00Z',
    },
    title: '测试套餐',
    summary: '测试摘要',
    accessArrangement: '通过官方邀请加入',
    distributionMethod: 'sub2api',
    distributionMethodNote: '系统分发',
    providesAdminAccount: false,
    regionCode: 'other',
    regionName: '新加坡二区',
    sourceAuthorVerification: { status: 'verified' },
    priceMonthlyCny: '88.50',
    serviceMultiplier: '1.2500',
    dailySpendLimitUsd: '12.500000',
    weeklySpendLimitUsd: '75.000000',
    followsOfficialQuotaReset: true,
    vpsRegion: 'Singapore',
    supportsMainlandChinaDirectConnection: false,
    openingChannelCode: 'other',
    customOpeningChannel: '邀请链接',
    paymentMethodCode: 'other',
    customPaymentMethod: '银行转账',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    buyerSeatCapacity: 5,
    offlineOccupiedSeats: 2,
    activeBuyerMembers: 1,
    reservedSeats: 0,
    availableSeats: 2,
    status: 'changes_requested',
    policyVersion: 7,
    riskNoticeCode: 'shared-account-risk',
    riskAckRequired: true,
    version: 11,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-02T00:00:00Z',
    ...overrides,
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('risk acknowledgement participates in the visible publish checklist', () => {
  assert.match(publishSource, /key: 'riskAcknowledgement' as const,[\s\S]*?complete: taskComplete\('riskAcknowledgement'\)/)
  assert.match(publishSource, /riskAcknowledgement: 'carpool-task-riskAcknowledgement'/)
  assert.match(publishSource, /id="carpool-task-riskAcknowledgement"/)
  assert.match(publishSource, /key === 'accessArrangement'[\s\S]*?\? 'riskAcknowledgement'/)
})

test('carpool management keeps the owner note private and mock-persistent', async () => {
  assert.match(myCarpoolsSource, /title="拼车管理"/)
  assert.match(myCarpoolsSource, /管理车队/)
  assert.match(managementSource, /车主私有备注/)
  assert.match(managementSource, /原因可以留空/)

  vi.stubGlobal('window', { sessionStorage: createStorage(), localStorage: createStorage() })
  const api = await import('../api')
  const member = (await api.getMerchantCarpoolApplications({ carpoolId: 'c1' })).find(item => item.status === 'active')
  assert.ok(member)
  await api.updateCarpoolMembershipOwnerNote(member, '已在站外确认')

  vi.resetModules()
  const reloadedApi = await import('../api')
  const reloadedMember = (await reloadedApi.getMerchantCarpoolApplications({ carpoolId: 'c1' })).find(item => item.id === member.id)
  assert.equal(reloadedMember?.ownerNote, '已在站外确认')
  const buyerApplication = await reloadedApi.getCarpoolApplicationById(member.id)
  assert.equal(buyerApplication?.ownerNote, undefined)
})

test('real carpool adapter leaves unavailable owner reputation facts null', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
    id: '22222222-2222-4222-8222-222222222222',
    categoryCode: 'gpt',
    providerCode: 'openai',
    slug: 'chatgpt-business',
    displayName: 'ChatGPT Business',
    description: '',
    publishPolicy: 'allowed',
    accessMode: 'provider_member_invitation',
    providerPolicyStatus: 'unknown',
    riskLevel: 'normal',
    riskAckRequired: false,
    policyVersion: 1,
    policyNote: '',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    allowCustomVariant: false,
    sortOrder: 1,
    createdAt: '2026-07-24T00:00:00Z',
    updatedAt: '2026-07-24T00:00:00Z',
  })))
  const { mapBackendCarpoolListing } = await import('../carpoolBackend')
  const listing = await mapBackendCarpoolListing({
    id: '33333333-3333-4333-8333-333333333333',
    ownerUserId: '44444444-4444-4444-8444-444444444444',
    productPlanId: '22222222-2222-4222-8222-222222222222',
    title: 'ChatGPT Business 拼车',
    summary: '',
    accessArrangement: '站外确认',
    distributionMethod: 'other',
    distributionMethodNote: '站外确认',
    providesAdminAccount: false,
    regionCode: 'us',
    regionName: '美国区',
    sourceUrl: 'https://linux.do/t/example/1',
    sourceAuthorVerification: {
      status: 'pending',
    },
    priceMonthlyCny: '100.00',
    serviceMultiplier: '1.0000',
    dailyQuotaAmount: null,
    weeklyQuotaAmount: '100.000000',
    followsOfficialQuotaReset: null,
    vpsRegion: null,
    supportsMainlandChinaDirectConnection: null,
    openingChannelCode: null,
    customOpeningChannel: null,
    paymentMethodCode: null,
    customPaymentMethod: null,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    buyerSeatCapacity: 4,
    activeBuyerMembers: 1,
    reservedSeats: 0,
    availableSeats: 3,
    status: 'active',
    policyVersion: 1,
    riskAckRequired: false,
    version: 1,
    createdAt: '2026-07-24T00:00:00Z',
    updatedAt: '2026-07-24T00:00:00Z',
  })

  assert.equal(listing.trustLevel, null)
  assert.equal(listing.linuxdoBound, null)
  assert.equal(listing.sourceAuthorVerification?.status, 'pending')
  assert.equal(listing.sourceUrl, 'https://linux.do/t/example/1')
  assert.equal(listing.hasUnresolvedDispute, null)
})

test('real carpool adapter never treats a source URL as author verification', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
    id: '22222222-2222-4222-8222-222222222222',
    categoryCode: 'gpt',
    providerCode: 'openai',
    slug: 'chatgpt-business',
    displayName: 'ChatGPT Business',
    description: '',
    publishPolicy: 'allowed',
    accessMode: 'provider_member_invitation',
    providerPolicyStatus: 'unknown',
    riskLevel: 'normal',
    riskAckRequired: false,
    policyVersion: 1,
    policyNote: '',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    allowCustomVariant: false,
    sortOrder: 1,
    createdAt: '2026-07-24T00:00:00Z',
    updatedAt: '2026-07-24T00:00:00Z',
  })))
  const { mapBackendCarpoolListing } = await import('../carpoolBackend')
  const listing = await mapBackendCarpoolListing({
    id: '33333333-3333-4333-8333-333333333333',
    ownerUserId: '44444444-4444-4444-8444-444444444444',
    productPlanId: '22222222-2222-4222-8222-222222222222',
    title: '待核验车源',
    summary: '',
    accessArrangement: '站外确认',
    distributionMethod: 'other',
    distributionMethodNote: '站外确认',
    providesAdminAccount: false,
    regionCode: 'us',
    regionName: '美国区',
    sourceUrl: 'https://linux.do/t/example/2',
    sourceAuthorVerification: {
      status: 'not_submitted',
    },
    priceMonthlyCny: '100.00',
    serviceMultiplier: '1.0000',
    dailyQuotaAmount: '25.000000',
    weeklyQuotaAmount: '100.000000',
    followsOfficialQuotaReset: true,
    vpsRegion: '香港',
    supportsMainlandChinaDirectConnection: true,
    openingChannelCode: 'web',
    customOpeningChannel: null,
    paymentMethodCode: 'u_card',
    customPaymentMethod: null,
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    buyerSeatCapacity: 4,
    activeBuyerMembers: 1,
    reservedSeats: 0,
    availableSeats: 3,
    status: 'active',
    policyVersion: 1,
    riskAckRequired: false,
    version: 1,
    createdAt: '2026-07-24T00:00:00Z',
    updatedAt: '2026-07-24T00:00:00Z',
  })

  assert.equal(listing.sourceUrl, 'https://linux.do/t/example/2')
  assert.equal(listing.sourceAuthorVerification?.status, 'not_submitted')
  assert.equal(listing.dailyQuotaAmount, 25)
  assert.equal(listing.followsOfficialQuotaReset, true)
  assert.equal(listing.vpsRegion, '香港')
  assert.equal(listing.supportsMainlandChinaDirectConnection, true)
  assert.equal(listing.openingChannelCode, 'web')
  assert.equal(listing.paymentMethodCode, 'u_card')
})

test('real carpool adapter projects public occupied seats from offline and active seats', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(productPlan())))
  const { mapBackendCarpoolListing } = await import('../carpoolBackend')

  const listing = await mapBackendCarpoolListing(ownerListing({
    buyerSeatCapacity: 5,
    offlineOccupiedSeats: 3,
    activeBuyerMembers: 0,
    availableSeats: 2,
  }))

  assert.equal(listing.currentConfirmedMembers, 0)
  assert.equal(listing.seats, '3/5')
  assert.deepEqual(listing.seatSummary, {
    carpoolId: 'listing-id',
    totalSeats: 5,
    activeMemberCount: 0,
    occupiedSeatCount: 3,
    availableSeats: 2,
  })

  const fullListing = await mapBackendCarpoolListing(ownerListing({
    buyerSeatCapacity: 5,
    offlineOccupiedSeats: 4,
    activeBuyerMembers: 3,
    availableSeats: 0,
  }))
  assert.equal(fullListing.seatSummary?.occupiedSeatCount, 5)
})

test('contact reveal analytics fires only after authoritative disclosure succeeds', async () => {
  const track = vi.fn()
  vi.stubGlobal('window', { umami: { track, identify: vi.fn() } })
  let contactShouldFail = false
  const fetchMock = vi.fn(async (input: string | URL | Request) => {
    const path = String(input)
    if (path.endsWith('/api/v1/auth/session')) {
      return jsonResponse({
			audience: 'normal',
        csrfToken: 'csrf-token',
        expiresAt: '2999-01-01T00:00:00Z',
        user: {
          id: 'buyer-id',
          analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
          username: 'buyer',
          displayName: 'Buyer',
          isAdmin: false,
          permissions: [],
          linuxDoBinding: { bound: true },
        },
      })
    }
    if (path.includes('/api/v1/me/carpool-memberships?limit=100')) return jsonResponse({ items: [] })
    if (path.includes('/api/v1/owner/carpool-memberships?limit=100')) return jsonResponse({ items: [] })
    if (path.endsWith('/api/v1/me/carpool-applications/application-id')) {
      return jsonResponse({
        id: 'application-id',
        carpoolListingId: 'listing-id',
        buyerUserId: 'buyer-id',
        ownerUserId: 'owner-id',
        productPlanId: 'plan-id',
        buyerContactMethodId: 'contact-method-id',
        status: 'joined',
        seatCount: 1,
        listingTitleSnapshot: '测试车源',
        priceMonthlyCny: '88.00',
        policyVersionSnapshot: 1,
        contactSessionId: 'contact-session-id',
        joinedAt: '2026-08-02T01:00:00Z',
        version: 1,
        createdAt: '2026-08-02T00:00:00Z',
        updatedAt: '2026-08-02T01:00:00Z',
      })
    }
    if (path.endsWith('/api/v1/product-plans/plan-id')) {
      return jsonResponse({
        id: 'plan-id',
        categoryCode: 'gpt',
        providerCode: 'openai',
        slug: 'test-plan',
        displayName: '测试套餐',
        description: '',
        publishPolicy: 'allowed',
        accessMode: 'provider_member_invitation',
        providerPolicyStatus: 'unknown',
        riskLevel: 'normal',
        riskAckRequired: false,
        policyVersion: 1,
        policyNote: '',
        quotaLabel: '额度',
        quotaUnit: 'USD',
        quotaPeriod: 'monthly',
        allowCustomVariant: false,
        sortOrder: 1,
        createdAt: '2026-08-02T00:00:00Z',
        updatedAt: '2026-08-02T00:00:00Z',
      })
    }
    if (path.endsWith('/api/v1/contact-sessions/contact-session-id/contacts')) {
      if (contactShouldFail) throw new TypeError('contact request failed')
      return jsonResponse({
        sessionId: 'contact-session-id',
        endsAt: '2026-08-03T00:00:00Z',
        items: [{
          side: 'seller',
          type: 'linuxdo',
          label: 'linux.do 私信',
          value: '@seller',
          maskedValue: '@s***r',
        }],
      })
    }
    throw new Error(`unexpected request: ${path}`)
  })
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const analytics = await import('../analytics')
  analytics.setAnalyticsRuntimeConfig({ enabled: true })
  const { backendCarpoolApplicationContacts } = await import('../carpoolBackend')

  const contacts = await backendCarpoolApplicationContacts('application-id')
  assert.equal(contacts.canView, true)
  assert.equal(contacts.sellerContacts[0]?.actionUrl, 'https://linux.do/u/seller/summary')
  assert.deepEqual(track.mock.calls, [[
    'contact_window_reveal',
    {
      source_route: '/my/rides/:id',
      entity_type: 'carpool_application',
    },
  ]])

  contactShouldFail = true
  await assert.rejects(
    () => backendCarpoolApplicationContacts('application-id'),
    /contact request failed/,
  )
  assert.equal(track.mock.calls.length, 1)
})

test('owner carpool page serializes view and cursor without forcing a default view', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse({ items: [], nextCursor: 'next-owner-page' }))
    .mockResolvedValueOnce(jsonResponse({ items: [] }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendOwnerCarpools, backendOwnerCarpoolsPage } = await import('../carpoolBackend')

  const page = await backendOwnerCarpoolsPage('needs_edit', { limit: 20, cursor: 'opaque+/=' })
  assert.equal(page.nextCursor, 'next-owner-page')
  const [pagePath, pageQuery = ''] = String(fetchMock.mock.calls[1]?.[0]).split('?')
  assert.equal(pagePath, '/api/v1/me/carpools')
  assert.deepEqual(Object.fromEntries(new URLSearchParams(pageQuery)), {
    view: 'needs_edit',
    limit: '20',
    cursor: 'opaque+/=',
  })

  await backendOwnerCarpools()
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/me/carpools?limit=100')
})

test('owner note adapter patches the membership version and maps the updated note', async () => {
  const membership = {
    id: 'membership-id',
    carpoolListingId: 'listing-id',
    carpoolApplicationId: 'application-id',
    buyerUserId: 'buyer-id',
    ownerUserId: 'owner-id',
    productPlanId: 'plan-id',
    status: 'active',
    seatCount: 1,
    priceMonthlyCny: '88.00',
    policyVersionSnapshot: 1,
    joinedAt: '2026-08-02T01:00:00Z',
    ownerNote: '仅车主可见',
    version: 3,
    createdAt: '2026-08-02T01:00:00Z',
    updatedAt: '2026-08-02T02:00:00Z',
  }
  const application = {
    id: 'application-id',
    carpoolListingId: 'listing-id',
    buyerUserId: 'buyer-id',
    ownerUserId: 'owner-id',
    productPlanId: 'plan-id',
    buyerContactMethodId: 'contact-id',
    status: 'joined',
    seatCount: 1,
    listingTitleSnapshot: '测试车源',
    priceMonthlyCny: '88.00',
    policyVersionSnapshot: 1,
    conditionsVersionSnapshot: 1,
    acceptedConditionsVersion: 1,
    conditionsAcceptedAt: '2026-08-02T01:00:00Z',
    joinedAt: '2026-08-02T01:00:00Z',
    version: 2,
    createdAt: '2026-08-02T00:00:00Z',
    updatedAt: '2026-08-02T01:00:00Z',
  }
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(membership))
    .mockResolvedValueOnce(jsonResponse(application))
    .mockResolvedValueOnce(jsonResponse(productPlan()))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendUpdateCarpoolMembershipOwnerNote } = await import('../carpoolBackend')
  const updated = await backendUpdateCarpoolMembershipOwnerNote('membership-id', '新备注', 2)

  assert.equal(updated.ownerNote, '仅车主可见')
  assert.equal(updated.backendMembershipId, 'membership-id')
  assert.equal(new Headers(fetchMock.mock.calls[1]?.[1]?.headers).get('If-Match'), '"2"')
  assert.match(new Headers(fetchMock.mock.calls[1]?.[1]?.headers).get('Idempotency-Key') ?? '', /^carpool-owner-note-/)
  assert.deepEqual(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body)), { note: '新备注' })
})

test('owner carpool edit detail maps every persisted publish field', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(ownerListing()))
    .mockResolvedValueOnce(jsonResponse(productPlan()))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendOwnerCarpoolForEdit } = await import('../carpoolBackend')

  const detail = await backendOwnerCarpoolForEdit('listing/id')
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/me/carpools/listing%2Fid')
  assert.deepEqual(detail, {
    id: 'listing-id',
    version: 11,
    backendStatus: 'changes_requested',
    ownerContactMethodId: 'contact-id',
    payload: {
      productId: 'plan-id',
      customProductName: null,
      regionCode: 'other',
      customRegionName: '新加坡二区',
      monthlyPriceCny: 88.5,
      serviceMultiplier: 1.25,
      dailyQuotaAmount: 12.5,
      weeklyQuotaAmount: 75,
      followsOfficialQuotaReset: true,
      vpsRegion: 'Singapore',
      supportsMainlandChinaDirectConnection: false,
      totalSeats: 5,
      occupiedSeats: 2,
      openingChannelCode: 'other',
      customOpeningChannel: '邀请链接',
      paymentMethodCode: 'other',
      customPaymentMethod: '银行转账',
      distributionMethod: 'sub2api',
      distributionMethodNote: '系统分发',
      providesAdminAccount: false,
      accessArrangementMode: 'provider_member_invitation',
      accessArrangementNote: '通过官方邀请加入',
      riskAcknowledged: true,
      policyVersion: 7,
      riskNoticeCode: 'shared-account-risk',
      warranty: {
        mode: 'remaining_days_compensation',
        fixedWarrantyDays: null,
        compensationMethod: '按剩余天数补偿',
        exclusions: '',
      },
      rulesNote: '不得转售',
      status: 'draft',
    },
  })
})

test('owner carpool update uses If-Match and submits with the patched version', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(productPlan()))
    .mockResolvedValueOnce(jsonResponse(backendWechatContactList()))
    .mockResolvedValueOnce(jsonResponse(ownerListing({ version: 12, status: 'draft' })))
    .mockResolvedValueOnce(jsonResponse(ownerListing({ version: 13, status: 'pending_review' })))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendUpdateOwnerCarpool } = await import('../carpoolBackend')
  const payload = {
    productId: 'plan-id',
    customProductName: null,
    regionCode: 'other',
    customRegionName: '新加坡二区',
    monthlyPriceCny: 88.5,
    serviceMultiplier: 1.25,
    dailyQuotaAmount: 12.5,
    weeklyQuotaAmount: 75,
    followsOfficialQuotaReset: true,
    vpsRegion: 'Singapore',
    supportsMainlandChinaDirectConnection: false,
    totalSeats: 5,
    occupiedSeats: 2,
    openingChannelCode: 'other',
    customOpeningChannel: '邀请链接',
    paymentMethodCode: 'other',
    customPaymentMethod: '银行转账',
    distributionMethod: 'sub2api' as const,
    distributionMethodNote: '系统分发',
    providesAdminAccount: false,
    accessArrangementMode: 'provider_member_invitation' as const,
    accessArrangementNote: '通过官方邀请加入',
    riskAcknowledged: true,
    policyVersion: 7,
    riskNoticeCode: 'shared-account-risk',
    warranty: {
      mode: 'remaining_days_compensation',
      fixedWarrantyDays: null,
      compensationMethod: '按剩余天数补偿',
      exclusions: '',
    },
    rulesNote: '不得转售',
    status: 'reviewing' as const,
  }

  const updated = await backendUpdateOwnerCarpool('listing/id', payload, 11, 'contact-id', true)
  assert.equal(updated.backendVersion, 13)

  const [patchPath, patchInit] = fetchMock.mock.calls[3] ?? []
  assert.equal(patchPath, '/api/v1/carpools/listing%2Fid')
  assert.equal(patchInit?.method, 'PATCH')
  assert.equal(new Headers(patchInit?.headers).get('If-Match'), '"11"')
  assert.match(new Headers(patchInit?.headers).get('Idempotency-Key') ?? '', /^carpool-update-/)
  assert.equal(new Headers(patchInit?.headers).get('X-CSRF-Token'), 'csrf-carpool')
  const patchBody = JSON.parse(String(patchInit?.body))
  assert.equal(patchBody.serviceMultiplier, '1.25')
  assert.equal(patchBody.dailySpendLimitUsd, '12.5')
  assert.equal(patchBody.weeklySpendLimitUsd, '75')
  assert.equal(patchBody.offlineOccupiedSeats, 2)
  assert.equal(patchBody.ownerContactMethodId, 'contact-id')
  assert.deepEqual(patchBody.riskAcknowledgement, {
    riskNoticeCode: 'shared-account-risk',
    policyVersion: 7,
  })

  const [submitPath, submitInit] = fetchMock.mock.calls[4] ?? []
  assert.equal(submitPath, '/api/v1/carpools/listing%2Fid/submit-review')
  assert.equal(submitInit?.method, 'POST')
  assert.equal(new Headers(submitInit?.headers).get('If-Match'), '"12"')
  assert.match(new Headers(submitInit?.headers).get('Idempotency-Key') ?? '', /^carpool-submit-review-/)
})

test('owner carpool update preserves unlimited limits and normalizes account login fields', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(productPlan()))
    .mockResolvedValueOnce(jsonResponse(backendWechatContactList()))
    .mockResolvedValueOnce(jsonResponse(ownerListing({
      distributionMethod: 'account_login',
      providesAdminAccount: false,
      dailySpendLimitUsd: null,
      weeklySpendLimitUsd: null,
      vpsRegion: null,
      supportsMainlandChinaDirectConnection: null,
      version: 12,
    })))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendUpdateOwnerCarpool } = await import('../carpoolBackend')
  await backendUpdateOwnerCarpool('listing-id', {
    productId: 'plan-id', customProductName: null, regionCode: 'other', customRegionName: '新加坡二区',
    monthlyPriceCny: 88.5, serviceMultiplier: 1, dailyQuotaAmount: null, weeklyQuotaAmount: null,
    followsOfficialQuotaReset: true, vpsRegion: '', supportsMainlandChinaDirectConnection: null,
    totalSeats: 5, occupiedSeats: 2, openingChannelCode: 'web', customOpeningChannel: '',
    paymentMethodCode: 'credit_card', customPaymentMethod: '', distributionMethod: 'account_login',
    distributionMethodNote: '', providesAdminAccount: true, accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: '通过官方邀请加入', riskAcknowledged: true, policyVersion: 7,
    riskNoticeCode: 'shared-account-risk', warranty: { mode: 'no_warranty', fixedWarrantyDays: null, compensationMethod: null, exclusions: null },
    rulesNote: '不得转售', status: 'draft',
  }, 11, 'contact-id', false)

  const body = JSON.parse(String(fetchMock.mock.calls[3]?.[1]?.body))
  assert.equal(body.dailySpendLimitUsd, null)
  assert.equal(body.weeklySpendLimitUsd, null)
  assert.equal(body.vpsRegion, null)
  assert.equal(body.supportsMainlandChinaDirectConnection, null)
  assert.equal(body.distributionMethod, 'account_login')
  assert.equal(body.providesAdminAccount, false)
})

test('owner carpool edit data preserves unlimited daily and weekly limits', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(ownerListing({
      dailySpendLimitUsd: null,
      weeklySpendLimitUsd: null,
    })))
    .mockResolvedValueOnce(jsonResponse(productPlan()))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendOwnerCarpoolForEdit } = await import('../carpoolBackend')
  const editData = await backendOwnerCarpoolForEdit('listing-id')

  assert.equal(editData.payload.dailyQuotaAmount, null)
  assert.equal(editData.payload.weeklyQuotaAmount, null)
})

test('owner carpool pages bind real tabs, edit routing, and version conflict recovery', () => {
  assert.match(myCarpoolsSource, /<StatusTabs v-model="activeTab"/)
  assert.match(myCarpoolsSource, /`\/my\/carpools\/\$\{item\.id\}\/edit`/)
  assert.match(myCarpoolsSource, /draft: '草稿'/)
  assert.match(myCarpoolsSource, /changes_requested: '待修改'/)
  assert.match(myCarpoolsSource, /stopped: '已停止'/)
  assert.match(myCarpoolsSource, /ownerStatusLabel\(item\)/)
  assert.doesNotMatch(myCarpoolsSource, /toast\.(?:info|success)\([^)]*编辑/)

  assert.match(publishSource, /route\.name === 'my-carpool-edit'/)
  assert.match(publishSource, /error\.status === 412/)
  assert.match(publishSource, /error\.code === 'VERSION_CONFLICT'/)
  assert.match(publishSource, /editQuery\.refetch\(\)/)
  assert.match(publishSource, /updateMyCarpool\(/)
})
