import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import { apiPurchaseIntents, apiServices } from '@/data/mock'
import type { ApiPurchaseIntent, ApiService } from '@/data/mock'

type ApiModule = typeof import('../api')

const completePaymentOptions = [{
  paymentMethod: 'wechat' as const,
  enabled: true,
  paymentInstructions: '请按订单金额付款。',
  paymentQrCodeDataUrl: 'data:image/png;base64,iVBORw0KGgo=',
}]

const writableQuotaPolicy = {
  fiveHour: { mode: 'unlimited' as const },
  daily: { mode: 'unlimited' as const },
}

function createStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

function serviceWithBilling(id: string, billingMode: unknown, state: ApiService['state'] = 'online'): ApiService {
  return {
    ...structuredClone(apiServices[0]!),
    id,
    billingMode: billingMode as ApiService['billingMode'],
    state,
    online: state === 'online',
    publiclyOrderable: true,
    probeConnectionId: 'probe-connection-ready',
    probeReady: true,
  }
}

function intentForService(id: string, serviceId: string): ApiPurchaseIntent {
  const intent = structuredClone(apiPurchaseIntents[0]!)
  intent.id = id
  intent.serviceId = serviceId
  intent.snapshot.serviceId = serviceId
  intent.snapshot.paymentOptions = completePaymentOptions
  return intent
}

async function loadMockApi(services: ApiService[] = [], intents: ApiPurchaseIntent[] = []): Promise<ApiModule> {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage({
      'c2cmarket.apiServices.v1': JSON.stringify(services),
      'c2cmarket.apiPurchaseIntents.v2': JSON.stringify(intents),
      'c2cmarket.apiOrders.v1': '[]',
    }),
    localStorage: createStorage(),
    setTimeout: globalThis.setTimeout,
  })
  const api = await import('../api')
  await vi.dynamicImportSettled()
  return api
}

async function settle<T>(promise: Promise<T>) {
  await vi.runAllTimersAsync()
  return promise
}

async function expectRejection(promise: Promise<unknown>) {
  const assertion = assert.rejects(promise, /当前版本不支持该 API 服务计费方式/)
  await vi.runAllTimersAsync()
  await assertion
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('rejects unsupported mock service writes and accepts supported modes', async () => {
  vi.useFakeTimers()
  const api = await loadMockApi()

  await expectRejection(api.submitApiService({ billingMode: 'manual_credit' }))
  await expectRejection(api.submitApiService({ billingMode: 'unknown_mode' }))

  const requiredServiceDeclarations = {
    declaredMaxConcurrency: 1,
    promptAuditEnabled: false,
    ownerContactMethodId: 'contact-wechat-orbit',
  }
  const metered = await settle(api.submitApiService({
    billingMode: 'metered_credit',
    quotaUsagePolicy: writableQuotaPolicy,
    ...requiredServiceDeclarations,
  }))
  const fixed = await settle(api.submitApiService({
    billingMode: 'fixed_package',
    quotaUsagePolicy: writableQuotaPolicy,
    ...requiredServiceDeclarations,
  }))
  assert.equal(metered.billingMode, 'metered_credit')
  assert.equal(fixed.billingMode, 'fixed_package')
})

test('keeps historical manual rows readable but never publicly orderable', async () => {
  vi.useFakeTimers()
  const manual = serviceWithBilling('manual-history', 'manual_credit')
  const api = await loadMockApi([manual])

  const ownerView = await settle(api.getMyApiServiceById(manual.id))
  assert.equal(ownerView?.billingMode, 'manual_credit')
  assert.equal(ownerView?.publiclyOrderable, false)
  assert.equal(await settle(api.getApiServiceById(manual.id)), null)
})

test('keeps sold-out packages in owner management but hides public detail', async () => {
  vi.useFakeTimers()
  const soldOut = structuredClone(apiServices.find(service => service.id === 'a2')!)
  soldOut.packages?.forEach((item) => {
    item.stockAvailable = 0
  })
  soldOut.merchantId = apiServices[0]!.merchantId
  soldOut.merchantUsername = apiServices[0]!.merchantUsername
  soldOut.publiclyOrderable = false
  const api = await loadMockApi([soldOut])

  assert.equal((await settle(api.getMyApiServiceById(soldOut.id)))?.packages?.every(item => item.stockAvailable === 0), true)
  assert.equal(await settle(api.getApiServiceById(soldOut.id)), null)
})

test('blocks publish and resume for unsupported modes without changing supported behavior', async () => {
  vi.useFakeTimers()
  const manual = serviceWithBilling('manual-offline', 'manual_credit', 'offline')
  const unknown = serviceWithBilling('unknown-paused', 'unknown_mode', 'paused')
  const metered = serviceWithBilling('metered-offline', 'metered_credit', 'offline')
  const fixed = serviceWithBilling('fixed-paused', 'fixed_package', 'paused')
  const api = await loadMockApi([manual, unknown, metered, fixed])

  await expectRejection(api.publishApiService(manual.id))
  await expectRejection(api.resumeApiService(unknown.id))

  assert.equal((await settle(api.getMyApiServiceById(manual.id)))?.state, 'offline')
  assert.equal((await settle(api.getMyApiServiceById(unknown.id)))?.state, 'paused')
  assert.equal((await settle(api.publishApiService(metered.id))).publiclyOrderable, true)
  assert.equal((await settle(api.resumeApiService(fixed.id))).publiclyOrderable, true)
})

test('rejects unsupported billing when creating an intent or an order from a historical intent', async () => {
  vi.useFakeTimers()
  const manual = serviceWithBilling('manual-order-service', 'manual_credit')
  const unknown = serviceWithBilling('unknown-order-service', 'unknown_mode')
  const manualIntent = intentForService('manual-existing-intent', manual.id)
  const api = await loadMockApi([manual, unknown], [manualIntent])

  await expectRejection(api.createApiPurchaseIntent({
    serviceId: unknown.id,
    buyerContactMethodId: 'contact-wechat-orbit',
    purchaseAmountCny: 20,
    deliveryMode: 'api_key_endpoint',
    targetModel: 'GPT-5 mini',
  }))
  await expectRejection(api.createApiOrderFromIntent(manualIntent.id, 'wechat'))

  const supportedIntent = await settle(api.createApiPurchaseIntent({
    serviceId: 'a1',
    buyerContactMethodId: 'contact-wechat-orbit',
    purchaseAmountCny: 20,
    deliveryMode: 'api_key_endpoint',
    targetModel: 'GPT-5 mini',
  }))
  supportedIntent.snapshot.paymentOptions = completePaymentOptions
  const orderSeedApi = await loadMockApi([], [supportedIntent])
  const order = await settle(orderSeedApi.createApiOrderFromIntent(supportedIntent.id, 'wechat'))
  assert.equal(order.intentSnapshot.serviceId, 'a1')
})
