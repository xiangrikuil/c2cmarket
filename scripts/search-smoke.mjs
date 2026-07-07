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

async function decode(response, expectedStatus) {
  const text = await response.text()
  const body = text ? JSON.parse(text) : null
  if (expectedStatus !== undefined) {
    assert(response.status === expectedStatus, `expected HTTP ${expectedStatus}, got ${response.status}: ${text}`)
    return body
  }
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
  return decode(response, options.expectedStatus)
}

async function createContact(auth, value, label) {
  return request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: `search-smoke-contact-${label}`,
    body: { type: 'telegram', label, value },
  }, auth)
}

async function createOfficialPriceRecord(admin, buyer, keyword, plan) {
  const lead = await request('/api/v1/official-price-leads', {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-price-lead',
    body: {
      productText: 'ChatGPT',
      planText: `Search Smoke ${keyword}`,
      regionCode: 'ph',
      channel: keyword,
      openingMethod: 'search_smoke_channel',
      sourceUrl: `https://linux.do/t/search-smoke-price/${Date.now()}`,
      sourceTitle: `Search smoke price ${keyword}`,
      evidenceSummary: '搜索 smoke 验证公开价格聚合。',
      note: '不包含支付、托管、担保或凭据内容。',
      observedAt: new Date().toISOString(),
      billingPeriod: 'monthly',
      currency: 'PHP',
      originalAmount: '7990.00',
      originalPriceText: 'PHP 7,990',
      taxIncluded: true,
    },
  }, buyer)
  const detail = await request(`/api/v1/admin/official-price-leads/${lead.id}`, {}, admin)
  const approved = await request(`/api/v1/admin/official-price-leads/${lead.id}/approve`, {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-price-approve',
    ifMatch: detail.version,
    body: {
      reason: 'search smoke approve',
      resolvedProductPlanId: plan.id,
      validFrom: new Date().toISOString(),
      fxSnapshot: {
        rateToCny: '0.1230',
        source: 'search-smoke-fx',
        observedAt: new Date().toISOString(),
      },
    },
  }, admin)
  return approved.record
}

async function createCarpool(owner, keyword, plan) {
  const ownerContact = await createContact(owner, '@search_smoke_carpool_owner', 'Search smoke carpool owner')
  const listing = await request('/api/v1/carpools', {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-carpool-create',
    body: {
      productPlanId: plan.id,
      ownerContactMethodId: ownerContact.id,
      cycleTerm: {
        billingPeriod: 'monthly',
        cycleStartDay: 1,
        noticeDays: 3,
        exitPolicy: '按月确认，退出需提前站外告知车主，平台不托管支付、不担保。',
        usageRules: '仅按车主说明使用席位，不在平台填写、粘贴或上传任何密码、API Key、token、Cookie 或 Session。',
      },
      title: `Search Smoke Carpool ${keyword}`,
      summary: '搜索 smoke 公开车源',
      accessArrangement: '费用分摊或成员邀请方案，平台不保存、不交付任何凭据。',
      sourceUrl: 'https://linux.do/t/search-smoke-carpool/123',
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
    idempotencyPrefix: 'search-smoke-carpool-submit',
    ifMatch: listing.version,
    body: {},
  }, owner)
  assert(published.status === 'active', 'search carpool should publish directly')
  return published
}

async function createDemand(admin, buyer, keyword) {
  const demand = await request('/api/v1/demands', {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-demand-create',
    body: {
      title: `Search Smoke Demand ${keyword}`,
      maxPriceCny: '199.00',
      regionCode: 'us',
      ownerPreference: 'personal',
      sourceUrl: `https://linux.do/t/search-smoke-demand/${Date.now()}`,
      note: '只记录求车上下文，后续站外确认；平台不处理支付、托管、担保或凭据。',
    },
  }, buyer)
  return request(`/api/v1/admin/demands/${demand.id}/approve`, {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-demand-approve',
    ifMatch: demand.version,
    body: { reason: 'search smoke approve' },
  }, admin)
}

async function createAPIService(owner, keyword) {
  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')
  const ownerContact = await createContact(owner, '@search_smoke_api_owner', 'Search smoke API owner')
  const draft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-api-create',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: ownerContact.id,
      title: `Search Smoke API ${keyword}`,
      shortDescription: '搜索 smoke API 服务',
      distributionSystem: 'sub2api',
      billingMode: 'metered_usd_quota',
      declaredCnyPerUsdAllowance: '0.8',
      declaredMaxUsdAllowancePerIntent: '100',
      quotaExpiresAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
      minimumIntentCny: '20',
      maximumIntentCny: '300',
      usageVisibility: 'offsite_panel_readonly',
      publicAccessNote: '仅展示接入说明，不展示凭据。',
      merchantNote: '站外确认后按说明接入。',
      merchantSupportNote: '平台不担保、不代赔；双方站外确认。',
      accessModes: [{ accessMode: 'buyer_dedicated_sub_key', publicNote: '站外确认接入说明。' }],
      models: [{ modelCatalogId: model.id, merchantMultiplier: '1.0000', enabled: true }],
      packages: [],
    },
  }, owner)
  const autoApproved = await request(`/api/v1/owner/api-services/${draft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-api-submit',
    ifMatch: draft.version,
    body: {},
  }, owner)
  assert(autoApproved.reviewStatus === 'approved', 'search API service should be auto-approved')
  assert(autoApproved.publicationStatus === 'offline', 'search API service should remain offline before publish')
  const online = await request(`/api/v1/owner/api-services/${draft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'search-smoke-api-publish',
    ifMatch: autoApproved.version,
    body: {},
  }, owner)
  return request(`/api/v1/owner/api-services/${draft.id}/order-settings`, {
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
        },
      ],
    },
  }, owner)
}

function findType(results, type, id) {
  return results.items.find(item => item.type === type && (!id || item.id.endsWith(id)))
}

function assertPublicSafe(results) {
  const text = JSON.stringify(results)
  const forbidden = ['@search_smoke_api_owner', '@search_smoke_carpool_owner', 'ownerContactMethodId', 'ownerUserId', 'api key', 'password=', 'access_token=', 'cookie=']
  for (const word of forbidden) {
    assert(!text.toLowerCase().includes(word.toLowerCase()), `search result leaked forbidden text: ${word}`)
  }
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const suffix = `${Date.now()}`
  const keyword = `searchsmoke${suffix}`
  const buyer = await session(`search-smoke-buyer-${suffix}`)
  const owner = await linuxDoSession(`search-smoke-owner-${suffix}`)
  const admin = await session(`search-smoke-admin-${suffix}`, true)

  const plans = await request('/api/v1/product-plans')
  const plan = plans.items.find(item => item.riskAckRequired && item.publishPolicy === 'allowed') ?? plans.items[0]
  assert(plan?.id, 'product plan catalog is empty')

  const officialRecord = await createOfficialPriceRecord(admin, buyer, keyword, plan)
  const carpool = await createCarpool(owner, keyword, plan)
  const demand = await createDemand(admin, buyer, keyword)
  const apiService = await createAPIService(owner, keyword)

  const empty = await request('/api/v1/search')
  assert(Array.isArray(empty.items) && empty.items.length === 0, 'empty search should return empty list')

  const results = await request(`/api/v1/search?q=${encodeURIComponent(keyword)}`)
  assert(findType(results, '官方价格', officialRecord.id), 'search should include official price result')
  assert(findType(results, '车源', carpool.id), 'search should include carpool result')
  assert(findType(results, '求车', demand.id), 'search should include demand result')
  assert(findType(results, 'API 服务', apiService.id), 'search should include API service result')
  assert(results.items.every(item => item.title && item.subtitle && item.badge && item.to), 'search results should include display fields')
  assertPublicSafe(results)

  const userResults = await request(`/api/v1/search?q=${encodeURIComponent(owner.user.username)}`)
  assert(findType(userResults, '用户'), 'search should include public user result for active user')
  assert(findType(userResults, '商户'), 'search should include merchant result for public-profile API merchant')
  assertPublicSafe(userResults)

  const tooLong = await request(`/api/v1/search?q=${'x'.repeat(81)}`, { expectedStatus: 422 })
  assert(tooLong.code === 'VALIDATION_FAILED', 'too-long search keyword should return validation problem')

  console.log(JSON.stringify({
    ok: true,
    keyword,
    resultTypes: results.items.map(item => item.type),
    officialRecordId: officialRecord.id,
    carpoolId: carpool.id,
    demandId: demand.id,
    apiServiceId: apiService.id,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
