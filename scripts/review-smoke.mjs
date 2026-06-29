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
    idempotencyPrefix: `review-smoke-contact-${label}`,
    body: {
      type: 'telegram',
      label,
      value,
    },
  }, auth)
}

async function createCompletedMembership(owner, buyer) {
  const plans = await request('/api/v1/product-plans')
  const plan = plans.items.find(item => item.riskAckRequired && item.publishPolicy === 'allowed') ?? plans.items[0]
  assert(plan?.id, 'product plan catalog is empty')

  const ownerContact = await createContact(owner, '@review_smoke_owner', 'Review smoke owner')
  const buyerContact = await createContact(buyer, '@review_smoke_buyer', 'Review smoke buyer')
  const listing = await request('/api/v1/carpools', {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-listing',
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
      title: `Review Smoke Carpool ${Date.now()}`,
      summary: '评价 smoke 车源',
      accessArrangement: '费用分摊或成员邀请方案，平台不保存、不交付任何凭据。',
      sourceUrl: 'https://linux.do/t/review-carpool-smoke/123',
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

  const published = await request(`/api/v1/carpools/${listing.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-submit',
    ifMatch: listing.version,
    body: {},
  }, owner)
  assert(published.status === 'active', 'listing should be active')

  const application = await request(`/api/v1/carpools/${listing.id}/applications`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-apply',
    body: {
      buyerContactMethodId: buyerContact.id,
      riskAcknowledgement: plan.riskAckRequired ? {
        riskNoticeCode: plan.riskNoticeCode,
        policyVersion: plan.policyVersion,
      } : undefined,
    },
  }, buyer)
  const accepted = await request(`/api/v1/owner/carpool-applications/${application.id}/accept`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-accept',
    ifMatch: application.version,
    body: {},
  }, owner)
  const buyerJoined = await request(`/api/v1/me/carpool-applications/${application.id}/confirm-join`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-buyer-join',
    ifMatch: accepted.version,
    body: {},
  }, buyer)
  const ownerJoined = await request(`/api/v1/owner/carpool-applications/${application.id}/confirm-join`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-owner-join',
    ifMatch: buyerJoined.version,
    body: {},
  }, owner)
  assert(ownerJoined.status === 'joined', 'application should be joined')

  const memberships = await request('/api/v1/me/carpool-memberships', {}, buyer)
  const membership = memberships.items.find(item => item.carpoolApplicationId === application.id)
  assert(membership?.status === 'active', 'membership should be active')

  const buyerCompleted = await request(`/api/v1/me/carpool-memberships/${membership.id}/confirm-complete`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-buyer-complete',
    ifMatch: membership.version,
    body: {},
  }, buyer)
  const ownerCompleted = await request(`/api/v1/owner/carpool-memberships/${membership.id}/confirm-complete`, {
    method: 'POST',
    idempotencyPrefix: 'review-smoke-owner-complete',
    ifMatch: buyerCompleted.version,
    body: {},
  }, owner)
  assert(ownerCompleted.status === 'completed', 'membership should be completed')
  return { listing: published, application, membership: ownerCompleted }
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession('review-smoke-owner')
  const buyer = await session('review-smoke-buyer')

  await request('/api/v1/me/profile', {
    method: 'PATCH',
    body: {
      displayName: 'Review Smoke Owner',
      username: 'review-smoke-owner',
      bio: '评价 smoke 车主公开主页。',
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

  const { listing, membership } = await createCompletedMembership(owner, buyer)

  const beforeRows = await request('/api/v1/me/reviews', {}, buyer)
  const before = beforeRows.items.find(item => item.sourceId === membership.id)
  assert(before?.status === 'reviewable', 'completed membership should be reviewable')
  assert(before.target === listing.title, 'review center row should include listing title')

  const firstReview = await request(`/api/v1/me/reviews/carpool-memberships/${membership.id}`, {
    method: 'PUT',
    idempotencyPrefix: 'review-smoke-put-first',
    body: {
      rating: 5,
      tags: ['沟通清楚', '规则明确'],
      note: '车主说明清楚，拼车规则透明，服务稳定。',
    },
  }, buyer)
  assert(firstReview.status === 'reviewed', 'submitted review should be reviewed')
  assert(firstReview.rating === 5, 'submitted review rating mismatch')

  const afterRows = await request('/api/v1/me/reviews', {}, buyer)
  const after = afterRows.items.find(item => item.sourceId === membership.id)
  assert(after?.status === 'reviewed', 'review center should show reviewed status')
  assert(after.note.includes('服务稳定'), 'review center should keep note')

  const publicReviews = await request('/api/v1/users/review-smoke-owner/reviews')
  const publicReview = publicReviews.items.find(item => item.id === firstReview.id)
  assert(publicReview?.verified === true, 'public review should be verified')
  assert(publicReview.note.includes('服务稳定'), 'public review should include submitted note')

  const publicProfile = await request('/api/v1/users/review-smoke-owner/public-profile')
  assert(!JSON.stringify(publicProfile).includes('@review_smoke_owner'), 'public profile must not leak owner contact')

  const updatedReview = await request(`/api/v1/me/reviews/carpool-memberships/${membership.id}`, {
    method: 'PUT',
    idempotencyPrefix: 'review-smoke-put-update',
    body: {
      rating: 4,
      tags: ['响应及时', '规则明确'],
      note: '修改后的评价：沟通响应及时，账期规则明确。',
    },
  }, buyer)
  assert(updatedReview.id === firstReview.id, 'review update should keep the same review id')
  assert(updatedReview.rating === 4, 'updated review rating mismatch')

  const updatedPublicReviews = await request('/api/v1/users/review-smoke-owner/reviews')
  const updatedPublicReview = updatedPublicReviews.items.find(item => item.id === firstReview.id)
  assert(updatedPublicReview?.note.includes('修改后的评价'), 'public review should reflect updated note')
  assert(updatedPublicReview.rating === 4, 'public review should reflect updated rating')

  console.log(JSON.stringify({
    ok: true,
    listingId: listing.id,
    membershipId: membership.id,
    reviewId: updatedReview.id,
    publicReviewCount: updatedPublicReviews.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
