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
      type: 'wechat',
      label,
      value,
    },
  }, auth)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession(`carpool-smoke-owner-${runSuffix}`)
  const buyer = await session(`carpool-smoke-buyer-${runSuffix}`)

  const plans = await request('/api/v1/product-plans')
  const plan = plans.items.find(item => item.riskAckRequired && item.publishPolicy === 'allowed') ?? plans.items[0]
  assert(plan?.id, 'product plan catalog is empty')

  const ownerContactValue = `@carpool_owner_${runSuffix.replaceAll('-', '_')}`
  const buyerContactValue = `@carpool_buyer_${runSuffix.replaceAll('-', '_')}`
  const ownerContact = await createContact(owner, ownerContactValue, 'Smoke carpool owner')
  const buyerContact = await createContact(buyer, buyerContactValue, 'Smoke carpool buyer')

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
      distributionMethod: 'sub2api',
      distributionMethodNote: 'Sub2API 托管管理，具体方式站外确认。',
      providesAdminAccount: true,
      regionCode: 'other',
      regionName: 'Smoke 测试区',
      priceMonthlyCny: '68.00',
      serviceMultiplier: '1.0000',
      dailySpendLimitUsd: '10.00',
      weeklySpendLimitUsd: '50.00',
      followsOfficialQuotaReset: true,
      vpsRegion: '香港',
      supportsMainlandChinaDirectConnection: true,
      openingChannelCode: 'web',
      customOpeningChannel: '',
      paymentMethodCode: 'u_card',
      customPaymentMethod: '',
      buyerSeatCapacity: 1,
      offlineOccupiedSeats: 0,
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
  assert(accepted.status === 'joined', 'owner acceptance should join the application immediately')
  assert(accepted.contactSessionId, 'accepted application should expose contact session id')

  const buyerContacts = await request(`/api/v1/contact-sessions/${accepted.contactSessionId}/contacts`, {}, buyer)
  assert(buyerContacts.items.some(item => item.value === ownerContactValue), 'buyer should see owner contact')

  const ownerContacts = await request(`/api/v1/contact-sessions/${accepted.contactSessionId}/contacts`, {}, owner)
  assert(ownerContacts.items.some(item => item.value === buyerContactValue), 'owner should see buyer contact')

  const memberships = await request('/api/v1/me/carpool-memberships', {}, buyer)
  const membership = memberships.items.find(item => item.carpoolApplicationId === application.id)
  assert(membership?.status === 'active', 'buyer membership should be active')

  const fullListing = await request(`/api/v1/me/carpools/${listing.id}`, {}, owner)
  assert(fullListing.status === 'stopped', 'full listing should stop recruiting automatically')

  const leftMembership = await request(`/api/v1/me/carpool-memberships/${membership.id}/leave`, {
    method: 'POST',
    idempotencyPrefix: 'smoke-carpool-buyer-leave',
    ifMatch: membership.version,
    body: { reason: 'Smoke 验证买家主动退出' },
  }, buyer)
  assert(leftMembership.status === 'left', 'buyer should be able to leave an active membership')

  const stoppedAfterLeave = await request(`/api/v1/me/carpools/${listing.id}`, {}, owner)
  assert(stoppedAfterLeave.status === 'stopped', 'member exit must not resume recruiting automatically')

  const resumedListing = await request(`/api/v1/me/carpools/${listing.id}/resume-recruiting`, {
    method: 'POST',
    ifMatch: stoppedAfterLeave.version,
    body: {},
  }, owner)
  assert(resumedListing.status === 'active', 'owner should explicitly resume recruiting after a seat is released')

  console.log(JSON.stringify({
    ok: true,
    listingId: listing.id,
    applicationId: application.id,
    contactSessionId: accepted.contactSessionId,
    membershipId: membership.id,
    endedMembershipStatus: leftMembership.status,
    resumedListingStatus: resumedListing.status,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
