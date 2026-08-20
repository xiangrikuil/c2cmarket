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
    purchaseKind: 'api_service',
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
    ...(status === 'delivery_submitted'
      ? {
          deliverySubmittedAt: '2099-07-11T10:00:00Z',
          deliveryReviewExpiresAt: '2099-07-12T10:00:00Z',
        }
      : {}),
  }
}

async function loadApiWithStoredOrder(order: ApiOrder) {
  vi.resetModules()
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

async function loadApiWithOrder(status: ApiOrderStatus) {
  return loadApiWithStoredOrder(orderWithStatus(status))
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('completes the order when the seller submits delivery', async () => {
  const api = await loadApiWithOrder('paid_confirmed')
  const order = orderWithStatus('paid_confirmed')

	const completed = await api.submitApiOrderDeliveryCredential(order.id, {
		deliveryKind: 'api_key_endpoint',
		apiBaseUrl: 'https://api.example.com/v1',
		apiKey: 'sk-test',
	}, order.version)
  assert.equal(completed.status, 'completed')
	assert.equal(completed.completionSource, 'seller_delivered')
  assert.equal(completed.version, order.version + 1)
  assert.ok(completed.completedAt)
	assert.equal(api.getApiOrderDisplayStatus(completed, 'buyer'), '商家已交付，订单完成')
	assert.equal(api.getApiOrderNextAction(completed, 'buyer'), '查看交付凭证；有问题可联系商家或发起纠纷')
  assert.equal(api.isApiOrderBuyerActionRequired(completed), false)
})

test('keeps seller-delivered completion stable when the after-sales window expires', async () => {
  const expired = {
		...orderWithStatus('completed'),
		completionSource: 'seller_delivered' as const,
    deliverySubmittedAt: '2026-07-11T10:00:00Z',
    deliveryReviewExpiresAt: '2026-07-12T10:00:00Z',
		completedAt: '2026-07-11T10:00:00Z',
  }
  const api = await loadApiWithStoredOrder(expired)

  const [completed] = await api.getMyApiOrders()
  assert.equal(completed.status, 'completed')
	assert.equal(completed.completionSource, 'seller_delivered')
	assert.equal(completed.canOpenDispute, false)
	assert.equal(completed.disputeEligibilityReason, 'after_sales_expired')
	assert.equal(api.getApiOrderEvents(completed).some(event => event.actorRole === 'system' && event.type === 'completed'), false)
})

test('keeps a seller-delivered order completed when a credential dispute exists', async () => {
  const disputed = {
		...orderWithStatus('completed'),
		completionSource: 'seller_delivered' as const,
		completedAt: '2026-07-11T10:00:00Z',
    disputeStatus: 'open',
    deliverySubmittedAt: '2026-07-11T10:00:00Z',
    deliveryReviewExpiresAt: '2026-07-12T10:00:00Z',
  }
  const api = await loadApiWithStoredOrder(disputed)

  const order = await api.getApiOrderById(disputed.id, 'buyer')
	assert.equal(order.status, 'completed')
	assert.equal(order.completionSource, 'seller_delivered')
	assert.equal(order.canOpenDispute, false)
})

test('keeps historical remedy completion attribution truthful', async () => {
	const remedied = {
		...orderWithStatus('completed'),
		completionSource: 'remedy_confirmed' as const,
		completedAt: '2026-07-11T11:00:00Z',
		deliverySubmittedAt: '2026-07-11T10:00:00Z',
		deliveryReviewExpiresAt: '2026-07-12T10:00:00Z',
	}
	const api = await loadApiWithStoredOrder(remedied)

	const completedEvent = api.getApiOrderEvents(remedied).find(event => event.type === 'completed')
	assert.equal(completedEvent?.actorLabel, '纠纷处理')
	assert.equal(completedEvent?.actorRole, 'system')
	assert.equal(completedEvent?.note, '纠纷补救履行确认后，订单完成。')
})

test('counts active disputes once in navigation actions and exposes their risk count', async () => {
  const disputed = {
    ...orderWithStatus('payment_submitted'),
    disputeStatus: 'open' as const,
  }
  const api = await loadApiWithStoredOrder(disputed)

  const summary = await api.getNavigationBadges()
  assert.equal(summary.buyer.apiOrderActions, 1)
  assert.equal(summary.buyer.apiOrderDisputes, 1)
  assert.equal(summary.merchant.apiOrderActions, 1)
  assert.equal(summary.merchant.apiOrderDisputes, 1)
})

test('keeps terminal dispute history out of navigation risk counts', async () => {
  const resolved = {
    ...orderWithStatus('completed'),
    disputeStatus: 'closed' as const,
    hasDisputeHistory: true,
  }
  const api = await loadApiWithStoredOrder(resolved)

  const summary = await api.getNavigationBadges()
  assert.equal(summary.buyer.apiOrderDisputes, 0)
  assert.equal(summary.merchant.apiOrderDisputes, 0)
})

test('labels an order-backed purchase intent as ordered', async () => {
  const api = await loadApiWithOrder('pending_payment')

  assert.equal(api.getApiStatusLabel('ordered'), '已生成订单')
})

test('counts revenue only after the merchant confirms receipt', async () => {
  const api = await loadApiWithOrder('pending_payment')

  for (const status of ['paid_confirmed', 'delivery_submitted', 'completed'] as ApiOrderStatus[]) {
    assert.equal(api.isApiOrderReceiptConfirmed(status), true, `${status} should count as confirmed receipt`)
  }
  for (const status of ['pending_payment', 'payment_submitted', 'payment_issue', 'cancelled'] as ApiOrderStatus[]) {
    assert.equal(api.isApiOrderReceiptConfirmed(status), false, `${status} must not count as confirmed receipt`)
  }
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

test('lets only the buyer open one structured after-sales application', async () => {
  const buyerApi = await loadApiWithOrder('payment_submitted')
  const order = orderWithStatus('payment_submitted')

  const buyerRequest = {
    issueCode: 'not_delivered' as const,
    requestedResolution: 'continue_fulfillment' as const,
    requestedAmountCny: null,
    reason: '付款后商户未继续处理。',
  }
  const buyerDispute = await buyerApi.openApiOrderDispute(order.id, buyerRequest, order.version, 'buyer')
  assert.equal(buyerDispute.disputeStatus, 'pending_seller_response')
  assert.equal(buyerDispute.version, order.version + 1)
  await assert.rejects(
    buyerApi.openApiOrderDispute(order.id, { ...buyerRequest, reason: '重复提交。' }, buyerDispute.version, 'buyer'),
    /不能再次申请平台介入/,
  )

  const merchantApi = await loadApiWithOrder('payment_submitted')
  await assert.rejects(
    merchantApi.openApiOrderDispute(order.id, {
      issueCode: 'payment_dispute',
      requestedResolution: 'other',
      requestedAmountCny: null,
      reason: '收款记录与买家说明不一致。',
    }, order.version, 'merchant'),
    /只有买家可以发起订单售后申请/,
  )
})
