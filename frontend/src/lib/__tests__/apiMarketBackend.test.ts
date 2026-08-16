import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import type { Carpool, UserContactMethod } from '../api'
import { apiQuotaOffers } from '../../data/mock'

type ApiModule = typeof import('../api')
type ApiMarketBackendModule = typeof import('../apiMarketBackend')

const writableQuotaPolicy = {
  fiveHour: { mode: 'unlimited' as const },
  daily: { mode: 'unlimited' as const },
}

const backendQuotaPolicy = {
  fiveHour: { mode: 'unlimited' as const, amountUsd: null },
  daily: { mode: 'unlimited' as const, amountUsd: null },
  scope: 'per_buyer_credential' as const,
  dailyReset: 'utc_plus_8_calendar_day' as const,
}

function createStorage() {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
    removeItem: (key: string) => {
      store.delete(key)
    },
    clear: () => {
      store.clear()
    },
  }
}

function buyerContact(overrides: Partial<UserContactMethod> = {}): UserContactMethod {
  return {
    id: 'contact-wechat',
    userId: 'student-buyer',
    type: 'wechat',
    label: '微信',
    maskedValue: 'student_***',
    displayValue: 'student-buyer',
    usageScopes: ['buyer', 'dispute'],
    isDefault: false,
    enabled: true,
    verified: false,
    createdAt: '2026-08-14T00:00:00Z',
    updatedAt: '2026-08-14T00:00:00Z',
    ...overrides,
  }
}

async function loadAPIMarketModules(): Promise<{ api: ApiModule, apiMarketBackend: ApiMarketBackendModule }> {
  vi.resetModules()
  const sessionStorage = createStorage()
  const localStorage = createStorage()
  vi.stubGlobal('window', { sessionStorage, localStorage })
  const [api, apiMarketBackend] = await Promise.all([
    import('../api'),
    import('../apiMarketBackend'),
  ])
  await vi.dynamicImportSettled()
  return { api, apiMarketBackend }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('selects the default eligible buyer contact before linux.do', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const linuxdo = buyerContact({ id: 'contact-linuxdo', type: 'linuxdo', label: 'linux.do', verified: true })
  const wechat = buyerContact({ isDefault: true })

  assert.equal(apiMarketBackend.selectBuyerContactMethod([linuxdo, wechat]), wechat)
})

test('allows a verified email contact for student API purchases', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const email = buyerContact({ id: 'contact-email', type: 'email', label: '邮箱', verified: true })

  assert.equal(apiMarketBackend.selectBuyerContactMethod([email]), email)
})

test('skips unverified email contacts and falls back to another eligible buyer contact', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const email = buyerContact({ id: 'contact-email', type: 'email', label: '邮箱', verified: false, isDefault: true })
  const linuxdo = buyerContact({ id: 'contact-linuxdo', type: 'linuxdo', label: 'linux.do', verified: true })

  assert.equal(apiMarketBackend.selectBuyerContactMethod([email, linuxdo]), linuxdo)
})

test('explains how to configure a contact when no buyer contact is eligible', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()

  assert.throws(
    () => apiMarketBackend.selectBuyerContactMethod([
      buyerContact({ enabled: false }),
      buyerContact({ id: 'contact-seller', usageScopes: ['api_merchant'] }),
      buyerContact({ id: 'contact-email', type: 'email', verified: false }),
    ]),
    /个人中心配置可用于买家交易的联系方式/,
  )
})

function backendPublicAPIService(overrides: Record<string, unknown> = {}) {
  return {
    id: 'service-public-1',
    merchantIdentityMode: 'store_alias',
    merchantDisplayName: '小葵 API',
    merchantProfileSlug: 'xiaokui-api',
    merchantAvatarUrl: 'https://cdn.example.com/xiaokui-api.webp',
    title: 'GPT · API 美元额度',
    shortDescription: '建议首次小额测试',
    sourceUrl: 'https://linux.do/t/api-quota-sub2api/123456',
    sourceAuthorVerification: {
      status: 'pending',
    },
    distributionSystem: 'sub2api',
    billingMode: 'metered_usd_quota',
    declaredCnyPerUsdAllowance: '0.8000',
    declaredMaxUsdAllowancePerIntent: '500.000000',
    quotaExpiresAt: '2026-08-07T17:05:00Z',
    accountPoolType: 'gpt_pro_5x',
    accountPoolLabel: 'GPT Pro 5x',
    declaredMaxConcurrency: 12,
    quotaUsagePolicy: backendQuotaPolicy,
    merchantRefundCommitment: true,
    merchantRefundPolicyVersion: 'api-merchant-refund-v1',
    minimumIntentCny: '10.00',
    maximumIntentCny: '300.00',
    usageVisibility: 'merchant_reported',
    publicAccessNote: 'Sub2API 标准美元额度，接入细节由双方站外确认。',
    merchantSupportNote: '平台不担保、不代赔；双方站外确认。',
    acceptingOrders: true,
    paymentWindowMinutes: 10,
    acceptedPaymentMethods: ['wechat'],
    isOrderable: true,
    accessModes: [{ accessMode: 'buyer_dedicated_sub_key', publicNote: '仅展示接入说明，不展示凭据。' }],
    models: [{
      id: 'model-row-1',
      modelCatalogId: 'gpt-5-mini',
      modelKeySnapshot: 'gpt-5-mini',
      providerSnapshot: 'OpenAI',
      capabilitiesSnapshot: ['text', 'chat'],
      merchantMultiplier: '1.0000',
      enabled: true,
    }],
    packages: [],
    version: 4,
    createdAt: '2026-07-08T17:06:02Z',
    updatedAt: '2026-07-08T17:06:02Z',
    ...overrides,
  }
}

test('maps public orderable API service responses as online services', async () => {
  const { api, apiMarketBackend } = await loadAPIMarketModules()
  const service = apiMarketBackend.mapBackendAPIService(backendPublicAPIService())

  assert.equal(service.state, 'online')
  assert.equal(service.online, true)
  assert.equal(service.publiclyOrderable, true)
  assert.equal(service.merchantAvatarUrl, 'https://cdn.example.com/xiaokui-api.webp')
  assert.equal(service.merchantType, '商户')
  assert.equal(api.isApiServicePubliclyOrderable(service), true)
  assert.equal(service.trustLevel, null)
  assert.equal(service.completed30d, null)
  assert.equal(service.reviewCount, null)
  assert.equal(service.unresolvedDisputes, null)
  assert.equal(service.sourceAuthorVerification?.status, 'pending')
  assert.equal(service.accountPoolType, 'gpt_pro_5x')
  assert.equal(service.accountPoolLabel, 'GPT Pro 5x')
  assert.equal(service.declaredMaxConcurrency, 12)
  assert.equal(service.merchantRefundCommitment, true)
  assert.equal(service.merchantRefundPolicyVersion, 'api-merchant-refund-v1')
})

test('keeps historical manual billing rows readable', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const service = apiMarketBackend.mapBackendAPIService(backendPublicAPIService({
    billingMode: 'manual_usage_check',
    isOrderable: false,
  }))

  assert.equal(service.billingMode, 'manual_credit')
  assert.equal(service.publiclyOrderable, false)
})

test('serializes structured commercial facts without writing the legacy merchant support note', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const request = apiMarketBackend.toBackendServiceRequest({
    billingMode: 'metered_credit',
    accountPoolType: 'custom',
    accountPoolCustomName: 'Claude Max',
    declaredMaxConcurrency: 16,
    promptAuditEnabled: false,
    quotaUsagePolicy: writableQuotaPolicy,
    warranty: { mode: 'merchant_full_refund' },
  })

  assert.equal(request.accountPoolType, 'custom')
  assert.equal(request.accountPoolCustomName, 'Claude Max')
  assert.equal(request.declaredMaxConcurrency, 16)
  assert.equal(request.promptAuditEnabled, false)
  assert.equal(request.merchantRefundCommitment, true)
  assert.equal('merchantSupportNote' in request, false)
})

test('serializes only supported API service billing modes', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const otherMetered = apiMarketBackend.toBackendServiceRequest({
    distributionSystem: 'other',
    billingMode: 'metered_credit',
    promptAuditEnabled: true,
    quotaUsagePolicy: writableQuotaPolicy,
  })
  assert.equal(otherMetered.distributionSystem, 'other')
  assert.equal(otherMetered.billingMode, 'metered_usd_quota')

  const fixedPackage = apiMarketBackend.toBackendServiceRequest({
    billingMode: 'fixed_package',
    promptAuditEnabled: false,
    quotaUsagePolicy: writableQuotaPolicy,
  })
  assert.equal(fixedPackage.billingMode, 'fixed_package')

  assert.throws(
    () => apiMarketBackend.toBackendServiceRequest({ billingMode: 'manual_credit' }),
    /Unsupported API billing mode/,
  )
  assert.throws(
    () => apiMarketBackend.toBackendServiceRequest({ billingMode: 'unknown_mode' }),
    /Unsupported API billing mode/,
  )
  assert.throws(
    () => apiMarketBackend.toBackendServiceRequest({}),
    /Unsupported API billing mode/,
  )
  assert.throws(
    () => apiMarketBackend.toBackendServiceRequest({ billingMode: 'metered_credit' }),
    /Prompt audit selection required/,
  )
})

test('maps API source-author verification independently from source URL presence', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const service = apiMarketBackend.mapBackendAPIService(backendPublicAPIService({
    sourceUrl: 'https://linux.do/t/api-quota-sub2api/123456',
    sourceAuthorVerification: {
      status: 'mismatch',
    },
  }))

  assert.equal(service.sourceUrl, 'https://linux.do/t/api-quota-sub2api/123456')
  assert.equal(service.sourceAuthorVerification?.status, 'mismatch')
})

test('maps platform health for public quota offers without projecting seller TTFT', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const source = {
    ...structuredClone(apiQuotaOffers[1]!),
    merchantAvatarUrl: 'https://cdn.example.com/quota-seller.webp',
    healthSummary: {
      state: 'no_sample' as const,
      availabilityReason: 'unconfigured' as const,
      successRatePercent: null,
      successfulSamples: 0,
      totalSamples: 0,
      transportSecurity: null,
      lastSampledAt: null,
      samples: Array.from({ length: 12 }, (_, index) => ({
        slotStartedAt: `2026-08-04T00:${String(index * 5).padStart(2, '0')}:00Z`,
        state: 'no_sample' as const,
      })) as [
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
        { slotStartedAt: string, state: 'no_sample' },
      ],
    },
  }
  const mapped = apiMarketBackend.mapBackendPublicAPIQuotaOffer(source)

  assert.equal(mapped.id, source.id)
  assert.equal(mapped.usdAllowance, '100.000000')
  assert.equal(mapped.priceCny, '8.80')
  assert.equal(mapped.modelMultiplier, '1.0000')
  assert.equal(mapped.merchantAvatarUrl, 'https://cdn.example.com/quota-seller.webp')
  assert.deepEqual(mapped.healthSummary, source.healthSummary)
  assert.equal(mapped.declaredTtftBand, undefined)
  assert.equal(mapped.declaredMaxConcurrency, 5)
  assert.equal(mapped.nextRound?.id, source.nextRound?.id)
})

test('maps public-profile merchant identity and avatar from the backend projection', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const service = apiMarketBackend.mapBackendAPIService(backendPublicAPIService({
    merchantIdentityMode: 'public_profile',
    merchantDisplayName: 'Profile Owner',
    merchantProfileSlug: 'profile-owner',
    merchantAvatarUrl: 'https://cdn.example.com/profile-owner.png',
  }))

  assert.equal(service.merchantDisplayName, 'Profile Owner')
  assert.equal(service.merchantUsername, 'profile-owner')
  assert.equal(service.merchantAvatarUrl, 'https://cdn.example.com/profile-owner.png')
  assert.equal(service.merchantType, '个人卖家')
})

test('preserves the selected API merchant identity mode in service requests', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const base = {
    billingMode: 'metered_credit',
    promptAuditEnabled: false,
    quotaUsagePolicy: writableQuotaPolicy,
  }

  assert.equal(apiMarketBackend.toBackendServiceRequest({ ...base, merchantIdentityMode: 'public_profile' }).merchantIdentityMode, 'public_profile')
  assert.equal(apiMarketBackend.toBackendServiceRequest({ ...base, merchantIdentityMode: 'store_alias' }).merchantIdentityMode, 'store_alias')
})

test('maps required owner sales and health summaries without changing the public service projection', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()
  const healthSummary = {
    state: 'no_sample' as const,
    availabilityReason: 'unconfigured' as const,
    successRatePercent: null,
    successfulSamples: 0,
    totalSamples: 0,
    transportSecurity: null,
    lastSampledAt: null,
    samples: [],
  }
  const service = apiMarketBackend.mapBackendOwnerAPIService({
    ...backendPublicAPIService({ healthSummary }),
    salesSummary: {
      overallState: 'selling',
      channels: [
        {
          kind: 'flexible_quota',
          state: 'selling',
          availableUsdAllowance: '420.000000',
        },
        {
          kind: 'limited_quota',
          state: 'upcoming',
          availableCopies: 48,
          nextStartsAt: '2026-07-30T12:00:00Z',
          saleCutoffAt: '2026-07-31T14:00:00Z',
          expiresAt: '2026-07-31T15:00:00Z',
        },
      ],
    },
  })

  assert.deepEqual(service.healthSummary, healthSummary)
  assert.equal(service.salesSummary.overallState, 'selling')
  assert.deepEqual(service.salesSummary.channels, [
    {
      kind: 'flexible_quota',
      state: 'selling',
      availableUsdAllowance: '420.000000',
      availableCopies: undefined,
      nextStartsAt: undefined,
      saleCutoffAt: undefined,
      expiresAt: undefined,
    },
    {
      kind: 'limited_quota',
      state: 'upcoming',
      availableUsdAllowance: undefined,
      availableCopies: 48,
      nextStartsAt: '2026-07-30T12:00:00Z',
      saleCutoffAt: '2026-07-31T14:00:00Z',
      expiresAt: '2026-07-31T15:00:00Z',
    },
  ])
})

test('matches mock owner sales views with the backend filter contract', async () => {
  const { api } = await loadAPIMarketModules()

  assert.equal(api.matchesApiServiceSalesView('selling', 'active'), true)
  assert.equal(api.matchesApiServiceSalesView('upcoming', 'active'), true)
  assert.equal(api.matchesApiServiceSalesView('expired', 'active'), false)
  assert.equal(api.matchesApiServiceSalesView('expired', 'expired'), true)
  assert.equal(api.matchesApiServiceSalesView('paused', 'paused'), true)
  assert.equal(api.matchesApiServiceSalesView('draft', 'draft'), true)
  assert.equal(api.matchesApiServiceSalesView('offline', 'draft'), true)
  assert.equal(api.matchesApiServiceSalesView('sold_out', 'all'), true)
  assert.equal(api.matchesApiServiceSalesView('archived', 'all'), true)
})

test('builds only the buyer API order dispute path', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()

  assert.equal(apiMarketBackend.apiOrderDisputePath('order/with space'), '/api/v1/me/api-orders/order%2Fwith%20space/dispute')
})

test('disables applications to a backend carpool owned by the current user', async () => {
  const { api } = await loadAPIMarketModules()
  const carpool: Carpool = {
    id: 'carpool-self-1',
    product: 'ChatGPT Pro',
    region: '印度区',
    monthly: 260,
    seats: '1/5',
    pricingMode: 'fixed',
    fixedMonthlyPrice: 260,
    currentConfirmedMembers: 1,
    maxMembers: 5,
    owner: '用户 owner-1',
    ownerUserId: 'owner-1',
    trustLevel: 4,
    ownerType: '个人车主',
    warranty: '车主承诺',
    openingMethod: '其他',
    status: '可上车',
    confirmedAt: '2026-07-11 13:00',
    confirmedWithin48h: true,
    linuxdoBound: true,
    sourceAuthorVerification: { status: 'verified' },
    hasInfoConflict: false,
    hasUnresolvedDispute: false,
    distributionMethod: 'other',
    distributionMethodNote: '具体安排站外确认。',
    providesAdminAccount: false,
    accessArrangementMode: 'other_off_platform',
    accessArrangementNote: '通过站外渠道确认成员安排。',
    riskAcknowledged: true,
  }

  assert.equal(
    api.getCarpoolApplyDisabledReason(carpool, { availableSeats: 4 }, false, 'owner-1'),
    '不能申请自己的车源。',
  )
})
