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
    monthlyQuotaAmount: '100.000000',
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
    monthlyQuotaAmount: '100.000000',
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
})
