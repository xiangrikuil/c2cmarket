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

async function createContact(auth, label, value) {
  return request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: `notification-smoke-contact-${label}`,
    body: {
      type: 'telegram',
      label,
      value,
    },
  }, auth)
}

async function createPublicAPIService(owner) {
  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')

  const ownerContact = await createContact(owner, 'Notification smoke API owner', `@notification_owner_${runSuffix.replaceAll('-', '_')}`)
  const probeConnectionId = seedVerifiedProbeConnection(owner.user.id)
  const draft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'notification-smoke-api-service',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: ownerContact.id,
      probeConnectionId,
      title: `Notification Smoke API Service ${Date.now()}`,
      shortDescription: '通知中心 smoke API 服务',
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
      accountPoolCustomName: 'Notification Smoke Pool',
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
  assert(draft.reviewStatus === 'draft', 'service should start as draft')

  const autoApproved = await request(`/api/v1/owner/api-services/${draft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'notification-smoke-api-submit',
    ifMatch: draft.version,
    body: {},
  }, owner)
  assert(autoApproved.reviewStatus === 'approved', 'service should be auto-approved')
  assert(autoApproved.publicationStatus === 'offline', 'auto-approved service should remain offline')

  const online = await request(`/api/v1/owner/api-services/${draft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'notification-smoke-api-publish',
    ifMatch: autoApproved.version,
    body: {},
  }, owner)
  assert(online.publicationStatus === 'online', 'service should be online')

  const orderable = await request(`/api/v1/owner/api-services/${draft.id}/order-settings`, {
    method: 'PATCH',
    ifMatch: online.version,
    body: {
      acceptingOrders: true,
      paymentWindowMinutes: 10,
      paymentOptions: [
        {
          paymentMethod: 'wechat',
          enabled: true,
          paymentInstructions: '微信收款方式由商户站外确认，平台不处理支付。',
          paymentQrCodeDataUrl: 'data:image/png;base64,aGVsbG8=',
        },
      ],
    },
  }, owner)
  assert(orderable.isOrderable === true, 'service should be orderable after settings')
  return orderable
}

async function createPurchaseIntent(buyer, service) {
  const buyerContact = await createContact(buyer, 'Notification smoke buyer', `@notification_buyer_${runSuffix.replaceAll('-', '_')}`)
  const intent = await request(`/api/v1/api-services/${service.id}/purchase-intents`, {
    method: 'POST',
    idempotencyPrefix: 'notification-smoke-api-intent',
    body: {
      buyerContactMethodId: buyerContact.id,
      requestedCnyAmount: '20',
      requestedUsdAllowance: '25',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      selectedPackageId: '',
      buyerNote: 'notification smoke intent',
    },
  }, buyer)
  assert(intent.status === 'open', 'intent should be open')
  return intent
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession(`notification-smoke-owner-${runSuffix}`)
  const buyer = await session(`notification-smoke-buyer-${runSuffix}`)

  const before = await request('/api/v1/me/notifications/unread-count', {}, owner)
  assert(Number.isInteger(before.count), 'unread count should be numeric before action')

  const service = await createPublicAPIService(owner)
  const intent = await createPurchaseIntent(buyer, service)

  const list = await request('/api/v1/me/notifications', {}, owner)
  const notice = list.items.find(item => item.targetType === 'api_purchase_intent' && item.targetId === intent.id)
  assert(notice, 'owner notification list should include API purchase intent notification')
  assert(notice.title === '收到新的购买意向', 'notification title should match API intent creation')
  assert(notice.unread === true, 'new notification should be unread')
  assert(notice.to === '/merchant/api-orders', 'owner API intent notification should link to merchant order list')

  const afterCreate = await request('/api/v1/me/notifications/unread-count', {}, owner)
  assert(afterCreate.count >= before.count + 1, 'unread count should increase after notification creation')

  const read = await request(`/api/v1/me/notifications/${notice.id}/read`, {
    method: 'POST',
    body: {},
  }, owner)
  assert(read.id === notice.id, 'read response should return target notification')
  assert(read.unread === false, 'read response should mark notification read')
  assert(typeof read.readAt === 'string' && read.readAt.length > 0, 'read response should include readAt')

  const afterRead = await request('/api/v1/me/notifications/unread-count', {}, owner)
  assert(afterRead.count === afterCreate.count - 1, 'unread count should decrease by one after single read')

  const readAll = await request('/api/v1/me/notifications/read-all', {
    method: 'POST',
    body: {},
  }, owner)
  assert(Number.isInteger(readAll.count), 'read-all count should be numeric')
  assert(Array.isArray(readAll.items), 'read-all should return notification list')

  const afterReadAll = await request('/api/v1/me/notifications/unread-count', {}, owner)
  assert(afterReadAll.count === 0, 'read-all should clear owner unread notifications')

  console.log(JSON.stringify({
    ok: true,
    serviceId: service.id,
    intentId: intent.id,
    notificationId: notice.id,
    unreadBefore: before.count,
    unreadAfterCreate: afterCreate.count,
    readAllCount: readAll.count,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
