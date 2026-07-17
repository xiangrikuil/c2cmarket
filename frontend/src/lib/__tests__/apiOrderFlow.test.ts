import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import type { ApiOrder, ApiOrderStatus } from '../api'

function createStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

function orderWithStatus(status: ApiOrderStatus): ApiOrder {
  return {
    id: 'api-order-flow-1',
    apiPurchaseIntentId: 'intent-1',
    apiServiceId: 'service-1',
    buyerId: 'buyer-demo-user',
    buyer: 'buyer',
    sellerId: 'merchant-orbit',
    seller: 'merchant',
    status,
    serviceTitle: 'API 美元额度',
    amount: 80,
    currency: 'CNY',
    selectedPaymentMethod: 'wechat',
    paymentWindowMinutes: 15,
    paymentExpiresAt: '2026-07-11T12:00:00Z',
    version: 3,
    intentSnapshot: {
      serviceId: 'service-1',
      serviceTitle: 'API 美元额度',
      billingMode: 'metered_usd_quota',
      usageVisibility: 'merchant_reported',
      models: ['GPT-5'],
      multiplier: '1x',
      warranty: '站外确认',
      refundPolicy: '站外协商',
      trustLevel: 3,
      linuxdoBound: true,
    },
    selectedDeliveryMode: 'api_key_endpoint',
    requestedUsdAllowance: 100,
    merchantContactChannels: [],
    buyerContactChannels: [],
    createdAt: '2026-07-11T10:00:00Z',
    updatedAt: '2026-07-11T10:00:00Z',
  }
}

async function loadApiWithOrder(status: ApiOrderStatus) {
  vi.resetModules()
  const order = orderWithStatus(status)
  const sessionStorage = createStorage({
    'c2cmarket.apiOrders.v1': JSON.stringify([order]),
  })
  vi.stubGlobal('window', {
    sessionStorage,
    localStorage: createStorage(),
    setTimeout: globalThis.setTimeout,
  })
  const api = await import('../api')
  await vi.dynamicImportSettled()
  return api
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('exposes the missing buyer completion action after seller delivery', async () => {
  const api = await loadApiWithOrder('delivery_submitted')
  const order = orderWithStatus('delivery_submitted')

  assert.equal(api.getApiOrderNextAction(order, 'buyer'), '核对交付并确认完成')
  assert.equal(api.getApiOrderNextAction(order, 'merchant'), '等待买家确认完成')
  assert.equal(api.isApiOrderBuyerActionRequired(order), true)

  const completed = await api.confirmApiOrderComplete(order.id, order.version)
  assert.equal(completed.status, 'completed')
  assert.equal(completed.version, order.version + 1)
  assert.ok(completed.completedAt)
  assert.equal(api.getApiOrderNextAction(completed, 'buyer'), '交易已完成')
  assert.equal(api.isApiOrderBuyerActionRequired(completed), false)
})

test('rejects completion before the seller submits delivery', async () => {
  const api = await loadApiWithOrder('paid_confirmed')

  await assert.rejects(
    api.confirmApiOrderComplete('api-order-flow-1', 3),
    /只有商户已交付的订单可以确认完成/,
  )
})

test('labels an order-backed purchase intent as ordered', async () => {
  const api = await loadApiWithOrder('pending_payment')

  assert.equal(api.getApiStatusLabel('ordered'), '已生成订单')
})

test('cancels only an unpaid order and preserves the selected reason', async () => {
  const api = await loadApiWithOrder('pending_payment')
  const order = orderWithStatus('pending_payment')
  const reason = '个人原因｜我不再需要该服务'

  const cancelled = await api.cancelApiOrder(order.id, reason, order.version)
  assert.equal(cancelled.status, 'cancelled')
  assert.equal(cancelled.cancelReason, reason)
  assert.ok(cancelled.cancelledAt)

  const paidApi = await loadApiWithOrder('payment_submitted')
  await assert.rejects(
    paidApi.cancelApiOrder(order.id, reason, order.version),
    /只有尚未付款的订单可以取消/,
  )
})

test('routes a payment mismatch back to the buyer and accepts a supplemented resubmission', async () => {
  const api = await loadApiWithOrder('payment_submitted')
  const order = orderWithStatus('payment_submitted')

  const issue = await api.reportApiOrderPaymentIssue(
    order.id,
    'amount_mismatch',
    '实收金额与订单金额不一致。',
    order.version,
  )
  assert.equal(issue.status, 'payment_issue')
  assert.equal(issue.paymentIssueReason, 'amount_mismatch')
  assert.equal(issue.paymentIssueNote, '实收金额与订单金额不一致。')
  assert.equal(api.getApiOrderNextAction(issue, 'buyer'), '补充付款信息并重新提交')
  assert.equal(api.getApiOrderNextAction(issue, 'merchant'), '等待买家补充付款信息')
  assert.equal(api.isApiOrderBuyerActionRequired(issue), true)

  const resubmitted = await api.submitApiOrderPayment(
    issue.id,
    '实际付款 ¥80.00，交易尾号 1234。',
    issue.version,
  )
  assert.equal(resubmitted.status, 'payment_submitted')
  assert.equal(resubmitted.paymentIssueReason, undefined)
  assert.equal(resubmitted.paymentIssueNote, undefined)
  assert.equal(resubmitted.paymentIssueReportedAt, undefined)
})

test('lets both order participants request platform intervention once', async () => {
  const buyerApi = await loadApiWithOrder('payment_submitted')
  const order = orderWithStatus('payment_submitted')

  const buyerDispute = await buyerApi.openApiOrderDispute(order.id, '付款后商户未继续处理。', order.version, 'buyer')
  assert.equal(buyerDispute.disputeStatus, 'open')
  assert.equal(buyerDispute.version, order.version + 1)
  await assert.rejects(
    buyerApi.openApiOrderDispute(order.id, '重复提交。', buyerDispute.version, 'buyer'),
    /不能再次申请平台介入/,
  )

  const merchantApi = await loadApiWithOrder('payment_submitted')
  const merchantDispute = await merchantApi.openApiOrderDispute(order.id, '收款记录与买家说明不一致。', order.version, 'merchant')
  assert.equal(merchantDispute.disputeStatus, 'open')
})
