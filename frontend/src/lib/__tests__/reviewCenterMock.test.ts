import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type ApiModule = typeof import('../api')

function createStorage(entries: Record<string, string> = {}) {
  const store = new Map(Object.entries(entries))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

function completedApiOrder(completedAt: string) {
  return {
    id: 'api-order-reviewable',
    orderNo: 'API-20260723-REVIEWTEST',
    purchaseKind: 'api_service',
    apiPurchaseIntentId: 'api-intent-reviewable',
    apiServiceId: 'api-service-reviewable',
    buyerId: 'buyer-demo-user',
    buyer: 'demo_user',
    sellerId: 'merchant-orbit',
    seller: 'orbit',
    status: 'completed',
    disputeStatus: 'none',
    hasDisputeHistory: false,
    serviceTitle: 'API 额度订单',
    amount: 100,
    currency: 'CNY',
    selectedPaymentMethod: 'wechat',
    paymentWindowMinutes: 10,
    paymentExpiresAt: '2026-07-23T07:10:00.000Z',
    commercialOutcome: 'normal_fulfillment',
    completionSource: 'buyer_confirmed',
    completedAt,
    version: 1,
    intentSnapshot: {},
    selectedDeliveryMode: 'api_key_endpoint',
    requestedUsdAllowance: 10,
    merchantContactChannels: [],
    buyerContactChannels: [],
    createdAt: '2026-07-23T07:00:00.000Z',
    updatedAt: completedAt,
  }
}

async function loadMockAPI(completedAt = '2026-07-23T08:00:00.000Z'): Promise<ApiModule> {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage({
      'c2cmarket.apiOrders.v1': JSON.stringify([completedApiOrder(completedAt)]),
    }),
    localStorage: createStorage(),
    setTimeout: globalThis.setTimeout,
  })
  const api: ApiModule = await import('../api')
  await vi.dynamicImportSettled()
  return api
}

async function settle<T>(promise: Promise<T>) {
  await vi.runAllTimersAsync()
  return promise
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('mock 评价中心只投影已完成的 API 订单', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI()

  const center = await settle(api.getReviewCenterRows())
  assert.equal(center.items.length, 1)
  assert.equal(center.items[0]?.transactionType, 'api_order')
  assert.equal(center.items[0]?.status, 'reviewable')
  assert.equal(center.items[0]?.canCreate, true)
  assert.equal(center.items[0]?.allowedTags.some(tag => tag.code === 'true_desc'), true)
})

test('mock API 订单评价提交后进入双盲状态并允许窗口内编辑', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI()

  const submitted = await settle(api.submitReview({
    transactionType: 'api_order',
    transactionId: 'api-order-reviewable',
    operation: 'create',
    rating: 5,
    tags: ['沟通顺畅'],
    note: '交付说明清楚。',
  }))

  assert.equal(submitted.visibility, 'sealed')
  assert.equal(submitted.canEdit, true)
  assert.equal(submitted.rating, 5)
})

test('mock 拒绝拼车评价和超过十四天窗口的 API 订单评价', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI('2026-07-01T08:00:00.000Z')

  const carpoolSubmission = api.submitReview({
    transactionType: 'carpool_membership' as 'api_order',
    transactionId: 'ride-app-5',
    operation: 'create',
    rating: 5,
    tags: ['沟通顺畅'],
    note: '拼车不应进入评价体系。',
  })
  const carpoolRejection = assert.rejects(carpoolSubmission, /拼车不支持评价/)
  await vi.runAllTimersAsync()
  await carpoolRejection

  const expiredSubmission = api.submitReview({
    transactionType: 'api_order',
    transactionId: 'api-order-reviewable',
    operation: 'create',
    rating: 5,
    tags: ['沟通顺畅'],
    note: '这条评价已经超过允许窗口。',
  })
  const expiredRejection = assert.rejects(expiredSubmission, /评价窗口已截止/)
  await vi.runAllTimersAsync()
  await expiredRejection
})
