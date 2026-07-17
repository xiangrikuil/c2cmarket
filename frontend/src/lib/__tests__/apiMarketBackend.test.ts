import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import type { Carpool } from '../api'

type ApiModule = typeof import('../api')
type ApiMarketBackendModule = typeof import('../apiMarketBackend')

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
    distributionSystem: 'sub2api',
    billingMode: 'metered_usd_quota',
    declaredCnyPerUsdAllowance: '0.8000',
    declaredMaxUsdAllowancePerIntent: '500.000000',
    quotaExpiresAt: '2026-08-07T17:05:00Z',
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
      modelNameSnapshot: 'GPT-5 mini',
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
  assert.equal(api.isApiServicePubliclyOrderable(service), true)
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
})

test('builds buyer and merchant API order dispute paths', async () => {
  const { apiMarketBackend } = await loadAPIMarketModules()

  assert.equal(apiMarketBackend.apiOrderDisputePath('order/with space', 'buyer'), '/api/v1/me/api-orders/order%2Fwith%20space/dispute')
  assert.equal(apiMarketBackend.apiOrderDisputePath('order-123', 'merchant'), '/api/v1/owner/api-orders/order-123/dispute')
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
    sourcePostAccessible: true,
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
