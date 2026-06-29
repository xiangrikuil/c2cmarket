const baseURL = process.env.API_BASE_URL ?? 'http://127.0.0.1:8080'

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
    idempotencyPrefix: `smoke-contact-${label}`,
    body: {
      type: 'telegram',
      label,
      value,
    },
  }, auth)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession('carpool-smoke-owner')
  const buyer = await session('carpool-smoke-buyer')

  const plans = await request('/api/v1/product-plans')
  const plan = plans.items.find(item => item.riskAckRequired && item.publishPolicy === 'allowed') ?? plans.items[0]
  assert(plan?.id, 'product plan catalog is empty')

  const ownerContact = await createContact(owner, '@carpool_smoke_owner', 'Smoke carpool owner')
  const buyerContact = await createContact(buyer, '@carpool_smoke_buyer', 'Smoke carpool buyer')

  const listing = await request('/api/v1/carpools', {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-listing',
    body: {
      productPlanId: plan.id,
      ownerContactMethodId: ownerContact.id,
      cycleTerm: {
        billingPeriod: 'monthly',
        cycleStartDay: 1,
        noticeDays: 3,
        exitPolicy: '按月确认，退出需提前 3 天站外告知车主，平台不托管支付、不担保。',
        usageRules: '仅按车主说明使用席位，不在平台填写、粘贴或上传任何密码、API Key、token、Cookie 或 Session。',
      },
      title: `Smoke Carpool ${Date.now()}`,
      summary: '真实拼车 smoke 车源',
      accessArrangement: '费用分摊或成员邀请方案，平台不保存、不交付任何凭据。',
      sourceUrl: 'https://linux.do/t/carpool-smoke/123',
      priceMonthlyCny: '68.00',
      serviceMultiplier: '1.3500',
      monthlyQuotaAmount: '200.00',
      buyerSeatCapacity: 1,
      activeBuyerMembers: 0,
      riskAcknowledgement: plan.riskAckRequired ? {
        riskNoticeCode: plan.riskNoticeCode,
        policyVersion: plan.policyVersion,
      } : undefined,
    },
  }, owner)
  assert(listing.status === 'draft', 'listing should start as draft')

  const published = await request(`/api/v1/carpools/${listing.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-submit',
    ifMatch: listing.version,
    body: {},
  }, owner)
  assert(published.status === 'active', 'listing should be active after linux.do owner publish')

  const publicListing = await request(`/api/v1/carpools/${listing.id}`)
  assert(publicListing.id === listing.id, 'published listing should be public')

  const application = await request(`/api/v1/carpools/${listing.id}/applications`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-apply',
    body: {
      buyerContactMethodId: buyerContact.id,
      riskAcknowledgement: plan.riskAckRequired ? {
        riskNoticeCode: plan.riskNoticeCode,
        policyVersion: plan.policyVersion,
      } : undefined,
    },
  }, buyer)
  assert(application.status === 'pending_owner', 'application should wait for owner')

  const accepted = await request(`/api/v1/owner/carpool-applications/${application.id}/accept`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-accept',
    ifMatch: application.version,
    body: {},
  }, owner)
  assert(accepted.status === 'accepted_reserved', 'application should be accepted_reserved')
  assert(accepted.contactSessionId, 'accepted application should expose contact session id')

  const buyerContacts = await request(`/api/v1/contact-sessions/${accepted.contactSessionId}/contacts`, {}, buyer)
  assert(buyerContacts.items.some(item => item.value === '@carpool_smoke_owner'), 'buyer should see owner contact')

  const ownerContacts = await request(`/api/v1/contact-sessions/${accepted.contactSessionId}/contacts`, {}, owner)
  assert(ownerContacts.items.some(item => item.value === '@carpool_smoke_buyer'), 'owner should see buyer contact')

  const buyerJoined = await request(`/api/v1/me/carpool-applications/${application.id}/confirm-join`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-buyer-join',
    ifMatch: accepted.version,
    body: {},
  }, buyer)
  assert(buyerJoined.status === 'accepted_reserved', 'first join confirmation should keep reservation')

  const ownerJoined = await request(`/api/v1/owner/carpool-applications/${application.id}/confirm-join`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-owner-join',
    ifMatch: buyerJoined.version,
    body: {},
  }, owner)
  assert(ownerJoined.status === 'joined', 'second join confirmation should create joined application')

  const memberships = await request('/api/v1/me/carpool-memberships', {}, buyer)
  const membership = memberships.items.find(item => item.carpoolApplicationId === application.id)
  assert(membership?.status === 'active', 'buyer membership should be active')

  const buyerCompleted = await request(`/api/v1/me/carpool-memberships/${membership.id}/confirm-complete`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-buyer-complete',
    ifMatch: membership.version,
    body: {},
  }, buyer)
  assert(buyerCompleted.status === 'active', 'first completion confirmation should keep membership active')

  const ownerCompleted = await request(`/api/v1/owner/carpool-memberships/${membership.id}/confirm-complete`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-owner-complete',
    ifMatch: buyerCompleted.version,
    body: {},
  }, owner)
  assert(ownerCompleted.status === 'completed', 'second completion confirmation should complete membership')

  console.log(JSON.stringify({
    ok: true,
    listingId: listing.id,
    applicationId: application.id,
    contactSessionId: accepted.contactSessionId,
    membershipId: membership.id,
    completedMembershipStatus: ownerCompleted.status,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
