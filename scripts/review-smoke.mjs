import { seedVerifiedProbeConnection } from './smoke-support.mjs'

const baseURL = process.env.API_BASE_URL ?? 'http://127.0.0.1:8080'
const runSuffix = process.env.SMOKE_RUN_ID || `${Date.now()}-${Math.random().toString(16).slice(2, 10)}`
const userSuffix = runSuffix.replace(/[^a-z0-9]/gi, '').slice(-8).toLowerCase()

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
  const startResponse = await fetch(`${baseURL}/api/v1/auth/oauth/start?returnTo=/my`)
  const start = await decode(startResponse)
  const startCookie = cookieFromSetCookie(startResponse.headers)
  const callbackURL = new URL(start.authorizationUrl)
  callbackURL.searchParams.set('code', username)
  const callbackResponse = await fetch(callbackURL.toString(), {
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

async function createContact(auth, value, label) {
  return request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: `review-smoke-contact-${label}`,
    body: {
      type: 'telegram',
      label,
      value,
    },
  }, auth)
}

async function createCompletedAPIOrder(owner, buyer) {
  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')
  const probeConnectionId = seedVerifiedProbeConnection(owner.user.id)
  const ownerContactValue = `@review_owner_${runSuffix.replaceAll('-', '_')}`
  const buyerContactValue = `@review_buyer_${runSuffix.replaceAll('-', '_')}`
  const ownerContact = await createContact(owner, ownerContactValue, 'Review smoke owner')
  const buyerContact = await createContact(buyer, buyerContactValue, 'Review smoke buyer')

  const draft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-service',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: ownerContact.id,
      probeConnectionId,
      title: `Review Smoke API Service ${Date.now()}`,
      shortDescription: '评价 smoke API 服务',
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
      accountPoolCustomName: 'Review Smoke API Pool',
      merchantRefundCommitment: false,
      declaredMaxConcurrency: 4,
      promptAuditEnabled: false,
      accessModes: [
        { accessMode: 'buyer_dedicated_sub_key', publicNote: '站外确认接入说明。' },
      ],
      models: [
        { modelCatalogId: model.id, merchantMultiplier: '1.0000', enabled: true },
      ],
      packages: [],
    },
  }, owner)
  const approved = await request(`/api/v1/owner/api-services/${draft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-service-submit',
    ifMatch: draft.version,
    body: {},
  }, owner)
  const published = await request(`/api/v1/owner/api-services/${draft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-service-publish',
    ifMatch: approved.version,
    body: {},
  }, owner)
  const orderable = await request(`/api/v1/owner/api-services/${draft.id}/order-settings`, {
    method: 'PATCH',
    ifMatch: published.version,
    body: {
      acceptingOrders: true,
      paymentWindowMinutes: 10,
      paymentOptions: [{
        paymentMethod: 'wechat',
        enabled: true,
        paymentInstructions: '站外确认付款后填写付款摘要。',
        paymentQrCodeDataUrl: 'data:image/png;base64,aGVsbG8=',
      }],
    },
  }, owner)
  assert(orderable.isOrderable === true, 'review smoke service should be orderable')

  const intent = await request(`/api/v1/api-services/${draft.id}/purchase-intents`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-intent',
    body: {
      buyerContactMethodId: buyerContact.id,
      requestedCnyAmount: '20',
      requestedUsdAllowance: '25',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      selectedPackageId: '',
      buyerNote: 'review smoke intent',
    },
  }, buyer)
  const order = await request(`/api/v1/me/api-purchase-intents/${intent.id}/orders`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-order',
    body: { paymentMethod: 'wechat' },
  }, buyer)
  const paid = await request(`/api/v1/me/api-orders/${order.id}/submit-payment`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-submit-payment',
    ifMatch: order.version,
    body: { paymentSummary: '已按站外确认金额完成付款。' },
  }, buyer)
  const confirmed = await request(`/api/v1/owner/api-orders/${order.id}/confirm-payment`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-confirm-payment',
    ifMatch: paid.version,
    body: {},
  }, owner)
  const delivered = await request(`/api/v1/owner/api-orders/${order.id}/submit-delivery`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-submit-delivery',
    ifMatch: confirmed.version,
    body: {
      deliveryKind: 'login_account',
      panelLoginUrl: 'https://panel.example.com/login',
      username: `review-smoke-${runSuffix}`,
      password: `review-smoke-password-${runSuffix}`,
      instructions: '买家专属接入信息；提交后不可修改。',
    },
  }, owner)
  const completed = await request(`/api/v1/me/api-orders/${order.id}/confirm-complete`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-confirm-complete',
    ifMatch: delivered.version,
    body: {},
  }, buyer)
  assert(completed.status === 'completed', 'API order should be completed')
  return { service: orderable, order: completed }
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const ownerUsername = `review-${userSuffix}`
  const owner = await linuxDoSession(ownerUsername)
  const buyer = await session(`rev-buy-${userSuffix}`)

  await request('/api/v1/me/profile', {
    method: 'PATCH',
    body: {
      displayName: 'Review Smoke Owner',
      username: ownerUsername,
      bio: '评价 smoke API 商户公开主页。',
      regionCode: 'cn',
      timezone: 'Asia/Shanghai',
      avatarMode: 'linuxdo',
      privacy: {
        showCreatedAt: true,
        showLastActiveAt: true,
        showCompletedCarpoolCount: true,
        showCompletedApiIntentCount: true,
        showResponseMedian: true,
        showResolvedDisputeSummary: true,
        allowPublicProfileReport: true,
      },
    },
  }, owner)

  const { service, order } = await createCompletedAPIOrder(owner, buyer)

  const beforeRows = await request('/api/v1/me/reviews', {}, buyer)
  const before = beforeRows.items.find(item => item.sourceId === order.id)
  assert(before?.status === 'reviewable', 'completed API order should be reviewable')
  assert(before.target === service.title, 'review center row should include service title')

  const firstReview = await request(`/api/v1/me/transactions/api_order/${order.id}/review`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-create-first',
    body: {
      rating: 5,
      tags: ['沟通顺畅', '规则清晰'],
      note: '商户说明清楚，接入规则透明，交付顺畅。',
    },
  }, buyer)
  assert(firstReview.status === 'sealed', 'first submitted review should remain sealed')
  assert(firstReview.visibility === 'sealed', 'first submitted review should not be public')
  assert(firstReview.rating === 5, 'submitted review rating mismatch')

  const afterRows = await request('/api/v1/me/reviews', {}, buyer)
  const after = afterRows.items.find(item => item.sourceId === order.id)
  assert(after?.status === 'sealed', 'review center should show sealed status')
  assert(after.note.includes('交付顺畅'), 'review center should keep note')

  const publicReviews = await request(`/api/v1/users/${ownerUsername}/reviews`)
  const publicReview = publicReviews.items.find(item => item.id === firstReview.id)
  assert(publicReview === undefined, 'sealed review must not appear publicly')

  const publicProfile = await request(`/api/v1/users/${ownerUsername}/public-profile`)
  assert(!JSON.stringify(publicProfile).includes('@review_owner_'), 'public profile must not leak owner contact')

  const updatedReview = await request(`/api/v1/me/transactions/api_order/${order.id}/review`, {
    method: 'PUT',
    idempotencyPrefix: 'review-smoke-edit',
    body: {
      rating: 4,
      tags: ['响应及时', '规则清晰'],
      note: '修改后的评价：沟通响应及时，账期规则明确。',
    },
  }, buyer)
  assert(updatedReview.id === firstReview.id, 'review update should keep the same review id')
  assert(updatedReview.rating === 4, 'updated review rating mismatch')
  assert(updatedReview.visibility === 'sealed', 'updated review should remain sealed until the counterparty submits')

  const ownerReview = await request(`/api/v1/me/transactions/api_order/${order.id}/review`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-create-owner',
    body: {
      rating: 5,
      tags: ['付款及时', '确认及时'],
      note: '买家付款和确认都很及时。',
    },
  }, owner)
  assert(ownerReview.visibility === 'published', 'counterparty review should publish both reviews')

  const updatedPublicReviews = await request(`/api/v1/users/${ownerUsername}/reviews`)
  const updatedPublicReview = updatedPublicReviews.items.find(item => item.id === firstReview.id)
  assert(updatedPublicReview?.note.includes('修改后的评价'), 'public review should reflect updated note')
  assert(updatedPublicReview.rating === 4, 'public review should reflect updated rating')

  console.log(JSON.stringify({
    ok: true,
    serviceId: service.id,
    orderId: order.id,
    reviewId: updatedReview.id,
    publicReviewCount: updatedPublicReviews.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
