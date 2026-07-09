import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

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
  assert.equal(api.isApiServicePubliclyOrderable(service), true)
})
