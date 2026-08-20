import { seedVerifiedProbeConnection } from './smoke-support.mjs'

const baseURL = process.env.API_BASE_URL ?? 'http://127.0.0.1:8080'
const runSuffix = process.env.SMOKE_RUN_ID || `${Date.now()}-${Math.random().toString(16).slice(2, 10)}`

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

function idempotencyKey(prefix) {
  return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function cookieFromSetCookie(headers) {
  const setCookie = headers.get('set-cookie')
  if (!setCookie) return ''
  return setCookie.split(',').map(item => item.split(';')[0]).join('; ')
}

function mergeCookies(...cookies) {
  return cookies.filter(Boolean).join('; ')
}

async function decode(response) {
  const text = await response.text()
  const body = text ? JSON.parse(text) : null
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}: ${text}`)
  }
  return body
}

async function decodeProblem(response) {
  const text = await response.text()
  const body = text ? JSON.parse(text) : null
  if (response.ok) {
    throw new Error(`expected problem response, got ${response.status}: ${text}`)
  }
  return body
}

async function session(username, admin = false) {
  const response = await fetch(`${baseURL}/api/v1/auth/dev-session`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, admin }),
  })
  const body = await decode(response)
  return {
    cookie: cookieFromSetCookie(response.headers),
    csrfToken: body.csrfToken,
    user: body.user,
  }
}

async function linuxDoSession(username) {
  const startResponse = await fetch(`${baseURL}/api/v1/auth/oauth/start`)
  const start = await decode(startResponse)
  const startCookie = cookieFromSetCookie(startResponse.headers)
  const startURL = new URL(start.authorizationUrl)
  startURL.searchParams.set('code', username)
  const callbackResponse = await fetch(startURL.toString(), {
    redirect: 'manual',
    headers: startCookie ? { Cookie: startCookie } : {},
  })
  if (callbackResponse.status !== 302) {
    const text = await callbackResponse.text()
    throw new Error(`oauth callback failed ${callbackResponse.status}: ${text}`)
  }
  const cookie = mergeCookies(startCookie, cookieFromSetCookie(callbackResponse.headers))
  const current = await request('/api/v1/auth/session', {}, { cookie })
  assert(current.user.linuxDoBinding?.bound === true, 'owner session should be bound to linux.do')
  return { cookie, csrfToken: current.csrfToken, user: current.user }
}

async function request(path, options = {}, auth) {
  const headers = {
    Accept: 'application/json',
    ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }),
    ...(auth?.cookie ? { Cookie: auth.cookie } : {}),
    ...(auth?.csrfToken && options.method && options.method !== 'GET' ? { 'X-CSRF-Token': auth.csrfToken } : {}),
    ...(options.idempotencyPrefix ? { 'Idempotency-Key': idempotencyKey(options.idempotencyPrefix) } : {}),
    ...(options.ifMatch !== undefined ? { 'If-Match': `"${options.ifMatch}"` } : {}),
    ...(options.headers ?? {}),
  }
  const response = await fetch(`${baseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })
  return decode(response)
}

async function problemRequest(path, options = {}, auth) {
  const headers = {
    Accept: 'application/json',
    ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }),
    ...(auth?.cookie ? { Cookie: auth.cookie } : {}),
    ...(auth?.csrfToken && options.method && options.method !== 'GET' ? { 'X-CSRF-Token': auth.csrfToken } : {}),
    ...(options.idempotencyPrefix ? { 'Idempotency-Key': idempotencyKey(options.idempotencyPrefix) } : {}),
    ...(options.ifMatch !== undefined ? { 'If-Match': `"${options.ifMatch}"` } : {}),
    ...(options.headers ?? {}),
  }
  const response = await fetch(`${baseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })
  return decodeProblem(response)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession(`api-smoke-owner-${runSuffix}`)
  const buyer = await session(`api-smoke-buyer-${runSuffix}`)
  const admin = await session(`api-smoke-admin-${runSuffix}`, true)
  const probeConnectionId = seedVerifiedProbeConnection(owner.user.id)
  const ownerContactValue = `@api_smoke_owner_${runSuffix.replaceAll('-', '_')}`
  const buyerContactValue = `@api_smoke_buyer_${runSuffix.replaceAll('-', '_')}`

  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')

  const ownerContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'smoke-owner-contact',
    body: {
      type: 'wechat',
      label: 'Smoke API owner',
      value: ownerContactValue,
    },
  }, owner)

  const serviceDraft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-service',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: ownerContact.id,
      probeConnectionId,
      title: `Smoke API Service ${Date.now()}`,
      shortDescription: '真实 API 集市 smoke 服务',
      distributionSystem: 'sub2api',
      billingMode: 'metered_usd_quota',
      declaredCnyPerUsdAllowance: '0.8',
      declaredMaxUsdAllowancePerIntent: '100',
      availableUsdAllowance: '1000',
      quotaExpiresAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
      quotaUsagePolicy: {
        fiveHour: { mode: 'limited', amountUsd: '5' },
        daily: { mode: 'unlimited' },
      },
      minimumIntentCny: '20',
      maximumIntentCny: '300',
      usageVisibility: 'offsite_panel_readonly',
      publicAccessNote: '仅展示接入说明，不展示凭据。',
      merchantNote: '站外确认后按说明接入。',
      accountPoolType: 'custom',
      accountPoolCustomName: 'Smoke API Pool',
      merchantRefundCommitment: false,
      declaredMaxConcurrency: 4,
      promptAuditEnabled: false,
      accessModes: [
        { accessMode: 'buyer_dedicated_sub_key', publicNote: '站外确认接入说明。' },
        { accessMode: 'buyer_dedicated_panel_subaccount', publicNote: '站外确认面板说明。' },
      ],
      models: [
        { modelCatalogId: model.id, merchantMultiplier: '1.0000', enabled: true },
      ],
      packages: [],
    },
  }, owner)
  assert(serviceDraft.reviewStatus === 'draft', 'service should start as draft')

  const autoApprovedService = await request(`/api/v1/owner/api-services/${serviceDraft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-service-submit',
    ifMatch: serviceDraft.version,
    body: {},
  }, owner)
  assert(autoApprovedService.reviewStatus === 'approved', 'service should be auto-approved')
  assert(autoApprovedService.publicationStatus === 'offline', 'auto-approved service should remain offline')

  const onlineService = await request(`/api/v1/owner/api-services/${serviceDraft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-service-publish',
    ifMatch: autoApprovedService.version,
    body: {},
  }, owner)
  assert(onlineService.publicationStatus === 'online', 'service should be online')

  const hiddenDetailProblem = await problemRequest(`/api/v1/api-services/${serviceDraft.id}`)
  assert(hiddenDetailProblem.code === 'OBJECT_NOT_FOUND', 'service without order settings should not have a public detail')

  const orderableService = await request(`/api/v1/owner/api-services/${serviceDraft.id}/order-settings`, {
    method: 'PATCH',
    ifMatch: onlineService.version,
    body: {
      acceptingOrders: true,
      paymentWindowMinutes: 10,
      paymentOptions: [
        {
          paymentMethod: 'wechat',
          enabled: true,
          paymentInstructions: '微信收款二维码请按商户站外确认展示，付款后填写付款摘要。',
          paymentQrCodeDataUrl: 'data:image/png;base64,aGVsbG8=',
        },
      ],
    },
  }, owner)
  assert(orderableService.acceptingOrders === true, 'service should accept orders after settings')
  assert(orderableService.isOrderable === true, 'service should be orderable after settings')
  assert(orderableService.acceptedPaymentMethods.includes('wechat'), 'service should expose wechat payment label')

  const publicDetail = await request(`/api/v1/api-services/${serviceDraft.id}`)
  assert(publicDetail.id === serviceDraft.id, 'public service detail should be available after order settings')

  const publicList = await request('/api/v1/api-services?paymentMethod=wechat')
  assert(publicList.items.some(item => item.id === serviceDraft.id), 'orderable service should appear in payment-filtered public list')

  const buyerContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'smoke-buyer-contact',
    body: {
      type: 'wechat',
      label: 'Smoke API buyer',
      value: buyerContactValue,
    },
  }, buyer)

  const intent = await request(`/api/v1/api-services/${serviceDraft.id}/purchase-intents`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-intent',
    body: {
      buyerContactMethodId: buyerContact.id,
      requestedCnyAmount: '20',
      requestedUsdAllowance: '25',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      selectedPackageId: '',
      buyerNote: 'smoke intent',
    },
  }, buyer)
  assert(intent.status === 'open', 'intent should be open')
  assert(intent.merchantContact?.value === ownerContactValue, 'buyer should see frozen merchant contact')

  const buyerDetail = await request(`/api/v1/me/api-purchase-intents/${intent.id}`, {}, buyer)
  assert(buyerDetail.merchantContact?.value === ownerContactValue, 'buyer detail should include merchant contact')

  const ownerDetail = await request(`/api/v1/owner/api-purchase-intents/${intent.id}`, {}, owner)
  assert(ownerDetail.buyerContact?.value === buyerContactValue, 'owner detail should include buyer contact')

  const contacted = await request(`/api/v1/owner/api-purchase-intents/${intent.id}/mark-contacted`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-intent-contacted',
    ifMatch: ownerDetail.version,
    body: {},
  }, owner)
  assert(contacted.status === 'contacted', 'intent should be marked contacted')

  const order = await request(`/api/v1/me/api-purchase-intents/${intent.id}/orders`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order',
    body: { paymentMethod: 'wechat' },
  }, buyer)
  assert(order.status === 'pending_payment', 'order should start pending payment')
  assert(order.amount === '20.00', 'metered API order should freeze requested CNY amount')
  assert(order.currency === 'CNY', 'metered API order should freeze CNY currency')
  assert(order.selectedPaymentMethod === 'wechat', 'order should freeze selected payment method')

  const instructions = await request(`/api/v1/me/api-orders/${order.id}/payment-instructions`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-instructions',
    body: {},
  }, buyer)
  assert(instructions.orderId === order.id, 'payment instructions should belong to order')
  assert(instructions.paymentInstructions.includes('微信收款二维码'), 'payment instructions should be private order instructions')

  const paid = await request(`/api/v1/me/api-orders/${order.id}/submit-payment`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-submit-payment',
    ifMatch: order.version,
    body: { paymentSummary: '已按站外确认金额完成微信付款，尾号 1234。' },
  }, buyer)
  assert(paid.status === 'payment_submitted', 'order should accept buyer payment summary')

  const disputeIntent = await request(`/api/v1/api-services/${serviceDraft.id}/purchase-intents`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-dispute-intent',
    body: {
      buyerContactMethodId: buyerContact.id,
      requestedCnyAmount: '21',
      requestedUsdAllowance: '26.25',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      selectedPackageId: '',
      buyerNote: 'smoke dispute intent',
    },
  }, buyer)
  const disputeOrder = await request(`/api/v1/me/api-purchase-intents/${disputeIntent.id}/orders`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-dispute-order',
    body: { paymentMethod: 'wechat' },
  }, buyer)
  const disputePaid = await request(`/api/v1/me/api-orders/${disputeOrder.id}/submit-payment`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-dispute-submit-payment',
    ifMatch: disputeOrder.version,
    body: { paymentSummary: '已付款但站外确认存在争议。' },
  }, buyer)
  const disputed = await request(`/api/v1/me/api-orders/${disputeOrder.id}/dispute`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-dispute-open',
    ifMatch: disputePaid.version,
    body: {
      issueCode: 'not_delivered',
      requestedResolution: 'continue_fulfillment',
      reason: '付款后商户未按站外确认说明继续处理。',
    },
  }, buyer)
  assert(disputed.disputeStatus === 'negotiating', 'API order dispute should start in negotiation')
  assert(disputed.disputeCaseId, 'API order dispute should bind disputeCaseId')
  const adminDisputes = await request('/api/v1/admin/disputes', {}, admin)
  assert(adminDisputes.items.some(item => item.id === disputed.disputeCaseId && item.targetType === 'api_order' && item.targetId === disputed.id), 'admin disputes should include API order dispute case')

  const confirmed = await request(`/api/v1/owner/api-orders/${order.id}/confirm-payment`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-confirm-payment',
    ifMatch: paid.version,
    body: {},
  }, owner)
  assert(confirmed.status === 'paid_confirmed', 'owner should confirm payment manually')

  const secretDeliveryProblem = await problemRequest(`/api/v1/owner/api-orders/${order.id}/submit-delivery`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-secret-delivery',
    ifMatch: confirmed.version,
    body: {
      deliveryKind: 'api_key_endpoint',
      apiBaseUrl: 'https://example.com/api/v1/client/subscribe?token=abc',
      apiKey: 'sk-proj-smoke',
      instructions: '买家专属接入信息。',
    },
  }, owner)
  assert(secretDeliveryProblem.code === 'SECRET_CONTENT_DETECTED', 'delivery must reject subscription URLs')

  const delivered = await request(`/api/v1/owner/api-orders/${order.id}/submit-delivery`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-submit-delivery',
    ifMatch: confirmed.version,
    body: {
      deliveryKind: 'login_account',
      panelLoginUrl: 'https://panel.example.com/login',
      username: `smoke-buyer-${runSuffix}`,
      password: `smoke-password-${runSuffix}`,
      instructions: '买家专属；提交后不可修改，后续更换请站外联系。',
    },
  }, owner)
	assert(delivered.status === 'completed', 'seller delivery should complete the order')
	assert(delivered.completionSource === 'seller_delivered', 'seller delivery completion source should be explicit')

  const duplicateOrderProblem = await problemRequest(`/api/v1/me/api-purchase-intents/${intent.id}/orders`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-api-order-duplicate',
    body: { paymentMethod: 'wechat' },
  }, buyer)
  assert(duplicateOrderProblem.code === 'API_PURCHASE_INTENT_HAS_ORDER', 'same purchase intent must not create a second order')

  console.log(JSON.stringify({
    ok: true,
    serviceId: serviceDraft.id,
    intentId: intent.id,
    orderId: order.id,
    publicServiceCount: (await request('/api/v1/api-services')).items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
