import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
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

test('contact reveal analytics fires only after authoritative disclosure succeeds', async () => {
  const track = vi.fn()
  vi.stubGlobal('window', { umami: { track, identify: vi.fn() } })
  let contactShouldFail = false
  const fetchMock = vi.fn(async (input: string | URL | Request) => {
    const path = String(input)
    if (path.endsWith('/api/v1/auth/session')) {
      return jsonResponse({
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
        status: 'accepted_reserved',
        seatCount: 1,
        listingTitleSnapshot: '测试车源',
        priceMonthlyCny: '88.00',
        policyVersionSnapshot: 1,
        contactSessionId: 'contact-session-id',
        reservationExpiresAt: '2026-08-03T00:00:00Z',
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
