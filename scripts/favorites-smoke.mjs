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

async function createContact(auth, value, label) {
  return request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: `favorites-smoke-contact-${label}`,
    body: {
      type: 'telegram',
      label,
      value,
    },
  }, auth)
}

async function createPublicCarpool(owner) {
  const plans = await request('/api/v1/product-plans')
  const plan = plans.items.find(item => item.riskAckRequired && item.publishPolicy === 'allowed') ?? plans.items[0]
  assert(plan?.id, 'product plan catalog is empty')

  const ownerContact = await createContact(owner, '@favorites_smoke_carpool_owner', 'Favorites smoke carpool owner')
  const listing = await request('/api/v1/carpools', {
    method: 'POST',
    idempotencyPrefix: 'favorites-smoke-carpool-listing',
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
      title: `Favorites Smoke Carpool ${Date.now()}`,
      summary: '收藏 smoke 车源',
      accessArrangement: '费用分摊或成员邀请方案，平台不保存、不交付任何凭据。',
      sourceUrl: 'https://linux.do/t/favorites-carpool-smoke/123',
      priceMonthlyCny: '68.00',
      serviceMultiplier: '1.3500',
      monthlyQuotaAmount: '200.00',
      buyerSeatCapacity: 2,
      activeBuyerMembers: 0,
      riskAcknowledgement: plan.riskAckRequired ? {
        riskNoticeCode: plan.riskNoticeCode,
        policyVersion: plan.policyVersion,
      } : undefined,
    },
  }, owner)

  const published = await request(`/api/v1/carpools/${listing.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'favorites-smoke-carpool-submit',
    ifMatch: listing.version,
    body: {},
  }, owner)
  assert(published.status === 'active', 'favorite carpool target should be active')
  return published
}

async function createPublicAPIService(owner) {
  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')

  const ownerContact = await createContact(owner, '@favorites_smoke_api_owner', 'Favorites smoke API owner')
  const draft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'favorites-smoke-api-service',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: ownerContact.id,
      title: `Favorites Smoke API Service ${Date.now()}`,
      shortDescription: '收藏 smoke API 服务',
      distributionSystem: 'sub2api',
      billingMode: 'metered_usd_quota',
      declaredCnyPerUsdAllowance: '0.8',
      declaredMaxUsdAllowancePerIntent: '100',
      minimumIntentCny: '20',
      maximumIntentCny: '300',
      usageVisibility: 'offsite_panel_readonly',
      publicAccessNote: '仅展示接入说明，不展示凭据。',
      merchantNote: '站外确认后按说明接入。',
      merchantSupportNote: '平台不担保、不代赔；双方站外确认。',
      accessModes: [
        { accessMode: 'buyer_dedicated_sub_key', publicNote: '站外确认接入说明。' },
      ],
      models: [
        { modelCatalogId: model.id, merchantMultiplier: '1.0000', enabled: true },
      ],
      packages: [],
    },
  }, owner)

  const autoApproved = await request(`/api/v1/owner/api-services/${draft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'favorites-smoke-api-submit',
    ifMatch: draft.version,
    body: {},
  }, owner)
  assert(autoApproved.reviewStatus === 'approved', 'favorite API target should be auto-approved')
  assert(autoApproved.publicationStatus === 'offline', 'favorite API target should remain offline before publish')
  const online = await request(`/api/v1/owner/api-services/${draft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'favorites-smoke-api-publish',
    ifMatch: autoApproved.version,
    body: {},
  }, owner)
  assert(online.reviewStatus === 'approved', 'favorite API target should be approved')
  assert(online.publicationStatus === 'online', 'favorite API target should be online')
  assert(online.moderationStatus === 'clear', 'favorite API target should be clear')
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
        },
      ],
    },
  }, owner)
  assert(orderable.isOrderable === true, 'favorite API target should be orderable')
  return orderable
}

async function favoriteStatus(auth, targetType, targetId) {
  return request(`/api/v1/me/favorites/${targetType}/${targetId}`, {}, auth)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const carpoolOwner = await linuxDoSession('favorites-smoke-carpool-owner')
  const apiOwner = await linuxDoSession('favorites-smoke-api-owner')
  const buyer = await session('favorites-smoke-buyer')

  const carpool = await createPublicCarpool(carpoolOwner)
  const apiService = await createPublicAPIService(apiOwner)

  const initialCarpoolStatus = await favoriteStatus(buyer, 'carpool', carpool.id)
  const initialAPIStatus = await favoriteStatus(buyer, 'api-service', apiService.id)
  assert(initialCarpoolStatus.favorited === false, 'carpool favorite should start false')
  assert(initialAPIStatus.favorited === false, 'api service favorite should start false')

  const carpoolFavorite = await request(`/api/v1/me/favorites/carpool/${carpool.id}`, {
    method: 'PUT',
    idempotencyPrefix: 'favorites-smoke-put-carpool',
    body: {},
  }, buyer)
  assert(carpoolFavorite.favorited === true, 'carpool favorite should become true')
  assert(carpoolFavorite.favorite?.targetType === 'carpool', 'carpool favorite target type mismatch')

  const apiFavorite = await request(`/api/v1/me/favorites/api-service/${apiService.id}`, {
    method: 'PUT',
    idempotencyPrefix: 'favorites-smoke-put-api',
    body: {},
  }, buyer)
  assert(apiFavorite.favorited === true, 'api service favorite should become true')
  assert(apiFavorite.favorite?.targetType === 'api_service', 'api service favorite target type mismatch')

  const listWithFavorites = await request('/api/v1/me/favorites', {}, buyer)
  const favoriteTargets = new Set(listWithFavorites.items.map(item => `${item.targetType}:${item.targetId}`))
  assert(favoriteTargets.has(`carpool:${carpool.id}`), 'favorites list should contain carpool')
  assert(favoriteTargets.has(`api_service:${apiService.id}`), 'favorites list should contain api service')

  const deletedCarpool = await request(`/api/v1/me/favorites/carpool/${carpool.id}`, {
    method: 'DELETE',
    body: {},
  }, buyer)
  assert(deletedCarpool.favorited === false, 'deleted carpool favorite should become false')
  assert((await favoriteStatus(buyer, 'carpool', carpool.id)).favorited === false, 'carpool status should stay false after delete')

  const deletedAPI = await request(`/api/v1/me/favorites/api-service/${apiService.id}`, {
    method: 'DELETE',
    body: {},
  }, buyer)
  assert(deletedAPI.favorited === false, 'deleted api favorite should become false')
  assert((await favoriteStatus(buyer, 'api-service', apiService.id)).favorited === false, 'api service status should stay false after delete')

  const finalList = await request('/api/v1/me/favorites', {}, buyer)
  const finalTargets = new Set(finalList.items.map(item => `${item.targetType}:${item.targetId}`))
  assert(!finalTargets.has(`carpool:${carpool.id}`), 'final favorites list should not contain carpool')
  assert(!finalTargets.has(`api_service:${apiService.id}`), 'final favorites list should not contain api service')

  console.log(JSON.stringify({
    ok: true,
    carpoolId: carpool.id,
    apiServiceId: apiService.id,
    listCountAfterCreate: listWithFavorites.items.length,
    listCountAfterDelete: finalList.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
