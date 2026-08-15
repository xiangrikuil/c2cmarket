import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, describe, test, vi } from 'vitest'

const merchantListSource = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const buyerListSource = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const orderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const disputePanelSource = readFileSync(new URL('../../components/api-order/ApiOrderDisputePanel.vue', import.meta.url), 'utf8')
const disputePageSource = readFileSync(new URL('../../pages/MyApiOrderDisputePage.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
const orderContactCardSource = readFileSync(new URL('../../components/profile/OrderContactCard.vue', import.meta.url), 'utf8')
const paymentMethodCardSource = readFileSync(new URL('../../components/contact-payment/PaymentMethodCard.vue', import.meta.url), 'utf8')
const paymentSettingsEditorSource = readFileSync(new URL('../../components/contact-payment/ApiPaymentSettingsEditor.vue', import.meta.url), 'utf8')
const apiFacadeSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')

const quotaPolicy = {
  fiveHour: { mode: 'unlimited', amountUsd: null },
  daily: { mode: 'unlimited', amountUsd: null },
  scope: 'per_buyer_credential',
  dailyReset: 'utc_plus_8_calendar_day',
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('卖家 API 订单详情入口与核款区', () => {
  test('所有订单行可进入详情，待确认收款同时保留查看和确认动作', () => {
    assert.match(merchantListSource, /<tr[^>]*tabindex="0"[^>]*@click="openOrder\(\$event, item\.id\)"[^>]*@keydown\.enter="openOrder\(\$event, item\.id\)"/)
    assert.match(merchantListSource, /<RouterLink :to="`\/merchant\/api-orders\/\$\{item\.id\}`">[\s\S]*?查看详情/)
    assert.match(merchantListSource, /<Button v-if="item\.status === 'payment_submitted'"[\s\S]*?确认已收款/)
    assert.match(merchantListSource, /event\.target instanceof Element && event\.target\.closest\('a,button'\)/)
  })

  test('首个业务区直接展示冻结核款事实、非空备注、买家联系方式和双操作', () => {
    const paymentCheckIndex = orderDetailSource.indexOf('aria-labelledby="merchant-payment-check-title"')
    const workflowIndex = orderDetailSource.indexOf('<Stepper v-if=')
    const detailsIndex = orderDetailSource.indexOf('<Collapsible v-model:open="orderDetailsOpen"')
    assert.ok(paymentCheckIndex > 0)
    assert.ok(paymentCheckIndex < workflowIndex)
    assert.ok(paymentCheckIndex < detailsIndex)
    assert.match(orderDetailSource, /<h2 id="merchant-payment-check-title"[^>]*>收款核对<\/h2>/)
    assert.match(orderDetailSource, /应收金额[\s\S]*?付款方式/)
    assert.match(orderDetailSource, /<div v-if="order\.buyerNote"><dt[^>]*>下单备注<\/dt>/)
    assert.match(orderDetailSource, /<div v-if="order\.paymentSummary"><dt[^>]*>付款备注<\/dt>/)
    assert.match(orderDetailSource, /:snapshot="buyerContactSnapshot"[\s\S]*?side="buyer"[\s\S]*?title="买家联系方式"[\s\S]*?compact/)
    assert.match(orderDetailSource, /<Button v-if="canReportPaymentIssue"[\s\S]*?付款有问题[\s\S]*?<Button :disabled="actionBusy" @click="confirmPayment">[\s\S]*?确认已收款/)
  })
})

describe('订单备注与联系方式快照', () => {
  test('真实卖家订单适配器保留意向中的下单备注', async () => {
    const pricingSnapshot = JSON.stringify({
      models: [{ modelKey: 'gpt-5-mini', merchantMultiplier: '1.0000' }],
      usageVisibility: 'merchant_reported',
      merchantNote: '按订单号核对。',
      merchantSupportNote: '出现问题先联系商户。',
      accountPoolType: null,
      accountPoolLabel: null,
      declaredMaxConcurrency: null,
      merchantRefundCommitment: false,
      merchantRefundPolicyVersion: 'api-merchant-refund-v1',
      serviceValidityExpiresAt: null,
    })
    const order = {
      id: 'order-note-1',
      orderNo: 'API-20260809-K7M4P9Q2XZ',
      purchaseKind: 'api_service',
      apiPurchaseIntentId: 'intent-note-1',
      apiServiceId: 'service-note-1',
      buyerUserId: 'buyer-note-1',
      sellerUserId: 'seller-note-1',
      status: 'payment_submitted',
      serviceTitleSnapshot: 'GPT API 服务',
      amount: '20.00',
      requestedUsdAllowanceSnapshot: '25.000000',
      pricingSnapshot,
      quotaUsagePolicySnapshot: quotaPolicy,
      currency: 'CNY',
      selectedPaymentMethod: 'wechat',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-08-09T06:10:00Z',
      paymentSummary: '已付 ¥20，尾号 1234。',
      version: 2,
      createdAt: '2026-08-09T06:00:00Z',
      updatedAt: '2026-08-09T06:05:00Z',
    }
    const intent = {
      id: 'intent-note-1',
      apiServiceId: 'service-note-1',
      buyerUserId: 'buyer-note-1',
      ownerUserId: 'seller-note-1',
      status: 'ordered',
      requestedCnyAmount: '20.00',
      requestedUsdAllowance: '25.000000',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      serviceVersionSnapshot: 3,
      serviceTitleSnapshot: 'GPT API 服务',
      distributionSystemSnapshot: 'sub2api',
      billingModeSnapshot: 'metered_usd_quota',
      declaredCnyPerUsdAllowanceSnapshot: '0.8000',
      minimumIntentCnySnapshot: '10.00',
      pricingSnapshot,
      quotaUsagePolicySnapshot: quotaPolicy,
      buyerNote: '请在付款备注中保留订单号后 6 位。',
      merchantContact: null,
      buyerContact: null,
      version: 1,
      createdAt: '2026-08-09T05:59:00Z',
      updatedAt: '2026-08-09T06:00:00Z',
    }
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/v1/owner/api-orders/order-note-1') return jsonResponse(order)
      if (url === '/api/v1/me/api-purchase-intents/intent-note-1') return jsonResponse(intent)
      throw new Error(`Unexpected request: ${url}`)
    })
    vi.stubGlobal('fetch', fetchMock)

    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'real' })
    const backend = await import('../apiMarketBackend')
    const mapped = await backend.backendOwnerAPIOrder('order-note-1')

    assert.equal(mapped.buyerNote, intent.buyerNote)
    assert.equal(mapped.paymentSummary, order.paymentSummary)
    assert.equal(mapped.viewerRole, 'merchant')
  })

  test('真实与 mock 映射都冻结 buyerNote，空值由详情模板条件隐藏', () => {
    assert.match(apiFacadeSource, /buyerNote\?: string/)
    assert.match(apiFacadeSource, /buyerNote: payload\.buyerNote/)
    assert.match(apiFacadeSource, /buyerNote: intent\.buyerNote/)
    assert.match(apiMarketBackendSource, /buyerNote: intent\.buyerNote/)
    assert.doesNotMatch(orderDetailSource, /\{\{ order\.buyerNote \?\? ['"]['"] \}\}/)
  })
})

describe('付款联系与平台兜底文案', () => {
  test('紧凑联系区只保留微信复制和 linux.do 私信直接动作', () => {
    const compactStart = orderContactCardSource.indexOf('<template v-if="compact">')
    const compactEnd = orderContactCardSource.indexOf('<template v-else>', compactStart)
    const compactTemplate = orderContactCardSource.slice(compactStart, compactEnd)
    assert.match(compactTemplate, /linux\.do 私信/)
    assert.match(compactTemplate, /复制微信/)
    assert.doesNotMatch(compactTemplate, /我已联系对方|无效|无法联系|疑似冒充|举报/)
    assert.match(orderDetailSource, /:snapshot="merchantContactSnapshot"[\s\S]*?title="付款有疑问？直接联系商户"[\s\S]*?compact[\s\S]*?:show-contacted-action="false"[\s\S]*?:show-issue-actions="false"/)
  })

  test('收款说明只引导脱敏核对，不重复填写付款软件或昵称', () => {
    assert.match(paymentMethodCardSource, /收款核对说明（选填）/)
    assert.match(paymentMethodCardSource, /扫码后请核对收款方“李\*”/)
    assert.doesNotMatch(paymentMethodCardSource, /付款软件昵称|完整实名|手机号|支付账号/)
    assert.match(paymentSettingsEditorSource, /收款核对说明不能包含 API Key、token、密码、Session、Cookie、付款码或面板凭据/)
  })

  test('订单详情只保留纠纷入口，独立页面承载平台处理', () => {
    assert.match(orderDetailSource, /v-model="disputeIssueCode"/)
    assert.match(orderDetailSource, /v-model="disputeRequestedResolution"/)
    assert.match(orderDetailSource, /v-if="disputeRequestedResolution === 'partial_refund'"/)
    assert.match(orderDetailSource, /提交后直接进入平台处理。被申请方可提交一次正式答复/)
    assert.match(orderDetailSource, /发起后直接进入平台处理，被申请方可提交一次正式答复。/)

    assert.match(disputePanelSource, /被申请方正式答复/)
    assert.match(disputePanelSource, /正式答复只能提交一次，提交后不可修改。/)
    assert.match(disputePanelSource, /平台定向补件/)
    assert.match(disputePanelSource, /撤回申请/)
    assert.match(disputePanelSource, /确认线下解决/)
    assert.match(disputePanelSource, /旧流程历史记录/)
    assert.match(disputePanelSource, /仅供查看，不能继续留言或处理方案/)
    assert.match(orderDetailSource, /进入纠纷处理/)
    assert.match(orderDetailSource, /`\/my\/disputes\/\$\{disputePanelId\}`/)
    assert.doesNotMatch(orderDetailSource, /<ApiOrderDisputePanel/)
    assert.match(routerSource, /path: '\/my\/disputes\/:id'/)
    assert.match(disputePageSource, /<ApiOrderDisputePanel :dispute-id="disputeId" \/>/)
    assert.doesNotMatch(disputePanelSource, /结束协商并申请平台介入|const canMessage|pendingFromMe/)
    assert.doesNotMatch(disputePanelSource, /诉求不成立|关闭纠纷|单方面关闭/)
    assert.doesNotMatch(orderDetailSource, /即时客服|实时客服|立即处理/)
  })

  test('买卖订单列表展示纠纷第二状态、待办和案件入口', () => {
    for (const source of [buyerListSource, merchantListSource]) {
      assert.match(source, /disputeCaseId/)
      assert.match(source, /disputeNeedsAction/)
      assert.match(source, /纠纷中/)
      assert.match(source, /纠纷待你处理/)
      assert.match(source, /`\/my\/disputes\/\$\{item\.disputeCaseId\}/)
    }
  })
})
