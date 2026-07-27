import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import { apiQuotaBatches, apiQuotaOffers } from '../../data/mock'
import type { PublicApiQuotaOffer } from '../api'

type ApiModule = typeof import('../api')

function createStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial))
  return {
    storage: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => store.set(key, value),
      removeItem: (key: string) => store.delete(key),
      clear: () => store.clear(),
    },
    serialized: () => [...store.values()].join('\n'),
  }
}

async function loadMockAPI(initialSession: Record<string, string> = {}) {
  vi.resetModules()
  const session = createStorage(initialSession)
  vi.stubGlobal('window', {
    sessionStorage: session.storage,
    localStorage: createStorage().storage,
    setTimeout: globalThis.setTimeout,
  })
  const api: ApiModule = await import('../api')
  await vi.dynamicImportSettled()
  return { api, session }
}

function scheduledOffer(id: string, roundId: string): PublicApiQuotaOffer {
  const source = structuredClone(apiQuotaOffers[1]!)
  const currentRound = {
    ...source.nextRound!,
    id: roundId,
    startsAt: '2026-07-19T00:00:00.000Z',
    endsAt: '2026-12-30T12:20:00.000Z',
  }
  return {
    ...source,
    id,
    currentRound,
    nextRound: undefined,
    availableCopies: 1,
    isOrderable: true,
    orderabilityCode: 'orderable',
    orderabilityReason: '当前可购买。',
  }
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('mock 固定场次按北京时间生成，并原子发布一个场次额度包', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T00:30:00.000Z'))
  const { api, session } = await loadMockAPI()

  const slotsPromise = api.getApiQuotaSaleSlots()
  await vi.runAllTimersAsync()
  const slots = await slotsPromise
  assert.equal(slots.items.length, 21)
  assert.equal(slots.items[0]?.key, '2026-07-24@09:00')
  assert.equal(slots.items[0]?.state, 'registration_closed')
  assert.equal(slots.items[1]?.key, '2026-07-24@13:00')
  assert.equal(slots.items[1]?.state, 'registration_open')

  const publicationPromise = api.createApiQuotaRushOffer({
    apiServiceId: 'a1',
    sourceType: 'sub2api',
    name: '$25 午间额度',
    usdAllowance: '25',
    priceCny: '3.50',
    modelMultiplier: '1.0000',
    copies: 8,
    deliveryMode: 'manual',
    deliveryEtaMinutes: 10,
    slotKey: '2026-07-24@13:00',
    expiresAt: '2026-07-25T05:30:00.000Z',
    sourceConfirmedAt: '2026-07-24T00:30:00.000Z',
  })
  await vi.runAllTimersAsync()
  const publication = await publicationPromise
  assert.equal(publication.round.systemSlotKey, '2026-07-24@13:00')
  assert.equal(publication.round.allocations.length, 1)
  assert.equal(publication.round.allocations[0]?.copyLimit, 8)
  assert.equal(publication.offer.saleMode, 'scheduled')
  assert.equal(publication.batch.status, 'published')

  const offersPromise = api.getApiQuotaOffers({ slotKey: '2026-07-24@13:00' })
  await vi.runAllTimersAsync()
  const offers = await offersPromise
  assert.equal(offers.some(item => item.id === publication.offer.id), true)
  assert.equal(session.serialized().includes('api_key'), false)
})

test('mock 预导入发布拒绝少于计划份数的凭据', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T00:30:00.000Z'))
  const { api } = await loadMockAPI()
  const file = new File([
    'api_base_url,api_key,instructions\nhttps://example.test/v1,secret-one,buyer only',
  ], 'credentials.csv', { type: 'text/csv' })

  const publication = api.createApiQuotaRushOffer({
    apiServiceId: 'a1',
    sourceType: 'sub2api',
    name: '$25 午间额度',
    usdAllowance: '25',
    priceCny: '3.50',
    modelMultiplier: '1.0000',
    copies: 2,
    deliveryMode: 'preimported',
    deliveryEtaMinutes: 2,
    deliveryKind: 'api_key_endpoint',
    file,
    slotKey: '2026-07-24@13:00',
    expiresAt: '2026-07-25T05:30:00.000Z',
    sourceConfirmedAt: '2026-07-24T00:30:00.000Z',
  })
  const rejection = assert.rejects(publication, /凭据数量至少需要 2 条/)
  await vi.runAllTimersAsync()
  await rejection
})

test('定时额度包固定金额下单，取消释放库存但同轮不能重抢其他规格', async () => {
  const roundId = 'quota-round-limit-test'
  const first = scheduledOffer('quota-offer-limit-50', roundId)
  const second = scheduledOffer('quota-offer-limit-100', roundId)
  const { api } = await loadMockAPI({
    'c2cmarket.apiQuotaOffers.v1': JSON.stringify([first, second]),
  })

  const order = await api.createApiQuotaOrder({ offerId: first.id, saleRoundId: roundId })
  assert.equal(order.purchaseKind, 'limited_quota_offer')
  assert.equal(order.amountDecimal, first.priceCny)
  assert.equal(order.quotaSnapshot?.usdAllowance, first.usdAllowance)
  assert.equal(order.paymentWindowMinutes, 5)

  await api.cancelApiOrder(order.id, '个人原因｜我不再需要该服务', order.version)
  const released = (await api.getApiQuotaOffers()).find(item => item.id === first.id)
  assert.equal(released?.availableCopies, 1)

  await assert.rejects(
    api.createApiQuotaOrder({ offerId: second.id, saleRoundId: roundId }),
    /同一买家每轮最多购买 1 份额度包/,
  )
})

test('CSV mock 只保存摘要，并在确认收款后自动交付预导入凭据', async () => {
  const { api, session } = await loadMockAPI()
  const offer = (await api.getApiQuotaOffers()).find(item => item.deliveryMode === 'preimported' && item.isOrderable)
  assert.ok(offer)

  const rawKey = 'sk-raw-import-must-not-persist'
  const file = new File([
    `api_base_url,api_key,instructions\nhttps://upstream.example/v1,${rawKey},buyer only`,
  ], 'credentials.csv', { type: 'text/csv' })
  const imported = await api.importApiQuotaCredentials(offer.id, 'api_key_endpoint', file)
  assert.equal(imported.imported, 1)
  assert.equal(session.serialized().includes(rawKey), false)

  const order = await api.createApiQuotaOrder({ offerId: offer.id })
  assert.equal(order.amountDecimal, offer.priceCny)
  assert.equal(order.quotaSnapshot?.priceCny, offer.priceCny)
  assert.equal(order.paymentWindowMinutes, 10)
  const submitted = await api.submitApiOrderPayment(order.id, '已付款，尾号 1234。', order.version)
  const delivered = await api.confirmApiOrderPayment(submitted.id, submitted.version)

  assert.equal(delivered.status, 'delivery_submitted')
  assert.equal(delivered.deliveryCredential?.deliveryKind, 'api_key_endpoint')
  assert.match(delivered.deliveryCredential?.apiKey ?? '', /^mock-api-order-quota-/)
  assert.notEqual(delivered.deliveryCredential?.apiKey, rawKey)
  const summary = await api.getApiQuotaCredentialSummary(offer.id)
  assert.equal(summary.reserved, 0)
  assert.equal(summary.delivered, 1)
  assert.equal(session.serialized().includes(rawKey), false)
})

test('Sub2API 额度包默认一倍但允许卖家声明其他固定倍率', async () => {
  const draftBatch = { ...structuredClone(apiQuotaBatches[0]!), status: 'draft' as const, publishedAt: undefined }
  const { api } = await loadMockAPI({
    'c2cmarket.apiQuotaBatches.v1': JSON.stringify([draftBatch]),
  })

  const offer = await api.createApiQuotaOffer({
    batchId: draftBatch.id,
    name: '$50 商业额度',
    usdAllowance: '50',
    priceCny: '6.00',
    modelMultiplier: '1.2500',
    deliveryMode: 'manual',
    deliveryEtaMinutes: 10,
    saleMode: 'continuous',
    continuousCopies: 10,
    sortOrder: 20,
  })

  assert.equal(offer.distributionSystem, 'sub2api')
  assert.equal(offer.modelMultiplier, '1.2500')
})
