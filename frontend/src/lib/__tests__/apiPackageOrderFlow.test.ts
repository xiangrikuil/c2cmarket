import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import { apiServices } from '@/data/mock'

const createStorage = (initial: Record<string, string> = {}) => {
  const store = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

const paymentOptions = [{
  paymentMethod: 'wechat',
  enabled: true,
  paymentInstructions: '请按订单金额付款。',
  paymentQrCodeDataUrl: 'data:image/png;base64,iVBORw0KGgo=',
}]

const loadApi = async () => {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage({
      'c2cmarket.apiServices.v1': JSON.stringify(apiServices),
      'c2cmarket.apiServicePaymentSnapshots.v1': JSON.stringify({ a2: paymentOptions }),
      'c2cmarket.apiPurchaseIntents.v2': '[]',
      'c2cmarket.apiOrders.v1': '[]',
    }),
    localStorage: createStorage(),
    setTimeout: globalThis.setTimeout,
  })
  const api = await import('../api')
  await vi.dynamicImportSettled()
  return api
}

const createPackageOrder = async (api: Awaited<ReturnType<typeof loadApi>>, packageId = 'a2-package-3d') => {
  const service = await api.getApiServiceById('a2')
  const item = service?.packages?.find(row => row.id === packageId)
  assert.ok(item)
  const intent = await api.createApiPurchaseIntent({
    serviceId: 'a2',
    purchaseAmountCny: item.priceCny,
    deliveryMode: 'api_key_endpoint',
    targetModel: item.models[0].modelName,
    selectedPackageId: item.id,
  })
  const order = await api.createApiOrderFromIntent(intent.id, 'wechat')
  return { item, intent, order }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('reserves package stock and restores it exactly once for an unpaid cancellation', async () => {
  const api = await loadApi()
  const before = await api.getApiServiceById('a2')
  const initialStock = before?.packages?.find(item => item.id === 'a2-package-3d')?.stockAvailable
  assert.equal(initialStock, 8)

  const { order } = await createPackageOrder(api)
  const reserved = await api.getApiServiceById('a2')
  assert.equal(reserved?.packages?.find(item => item.id === 'a2-package-3d')?.stockAvailable, 7)
  assert.equal(order.packageStockReserved, true)

  const cancelled = await api.cancelApiOrder(order.id, '个人原因｜不再需要', order.version)
  const restored = await api.getApiServiceById('a2')
  assert.equal(cancelled.packageStockReserved, false)
  assert.equal(restored?.packages?.find(item => item.id === 'a2-package-3d')?.stockAvailable, 8)
})

test('consumes reserved stock after payment and starts validity at delivery', async () => {
  const api = await loadApi()
  const { order } = await createPackageOrder(api)
  const submitted = await api.submitApiOrderPayment(order.id, '已付款', order.version)
  const paid = await api.confirmApiOrderPayment(order.id, submitted.version)
  assert.equal(paid.packageStockReserved, false)

  const delivered = await api.submitApiOrderDeliveryCredential(paid.id, {
    deliveryKind: 'api_key_endpoint',
    apiBaseUrl: 'https://api.example.test/v1',
    apiKey: 'buyer-dedicated-key',
  }, paid.version)
  assert.ok(delivered.packageExpiresAt)
  assert.equal(
    new Date(delivered.packageExpiresAt).getTime() - new Date(delivered.deliverySubmittedAt!).getTime(),
    3 * 86_400_000,
  )
  const service = await api.getApiServiceById('a2')
  assert.equal(service?.packages?.find(item => item.id === 'a2-package-3d')?.stockAvailable, 7)
})
