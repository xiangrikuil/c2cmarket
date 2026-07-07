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
  const start = await request('/api/v1/auth/oauth/start')
  const startURL = new URL(start.authorizationUrl)
  startURL.searchParams.set('code', username)
  const callbackResponse = await fetch(startURL.toString(), { redirect: 'manual' })
  if (callbackResponse.status !== 302) {
    const text = await callbackResponse.text()
    throw new Error(`oauth callback failed ${callbackResponse.status}: ${text}`)
  }
  const cookie = cookieFromSetCookie(callbackResponse.headers)
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

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const owner = await linuxDoSession('profile-smoke-owner')
  const buyer = await session('profile-smoke-buyer')

  const originalProfile = await request('/api/v1/me/profile', {}, owner)
  assert(originalProfile.username === 'profile-smoke-owner', 'owner profile should match dev session')

  const updatedProfile = await request('/api/v1/me/profile', {
    method: 'PATCH',
    body: {
      displayName: 'Profile Smoke Owner',
      username: 'profile-smoke-owner',
      bio: '只公开必要业务资料。',
      regionCode: 'cn',
      timezone: 'Asia/Shanghai',
      avatarMode: 'linuxdo',
      privacy: {
        showCreatedAt: true,
        showLastActiveAt: false,
        showCompletedCarpoolCount: true,
        showCompletedApiIntentCount: true,
        showResponseMedian: true,
        showResolvedDisputeSummary: true,
        allowPublicProfileReport: true,
      },
    },
  }, owner)
  assert(updatedProfile.displayName === 'Profile Smoke Owner', 'profile update should persist display name')
  assert(updatedProfile.privacy.showLastActiveAt === false, 'profile privacy update should persist')

  const ownerContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-owner-contact',
    body: {
      type: 'telegram',
      label: 'Profile Smoke Owner TG',
      value: '@profile_smoke_owner',
      usageScopes: ['api_merchant'],
      isDefault: true,
      enabled: true,
    },
  }, owner)
  assert(ownerContact.maskedValue, 'created contact should include masked value')

  const emailContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-owner-email',
    body: {
      type: 'email',
      label: 'Profile Smoke Email',
      value: 'profile-smoke@example.com',
      usageScopes: ['buyer'],
      isDefault: false,
      enabled: true,
    },
  }, owner)

  const listedContacts = await request('/api/v1/me/contact-methods', {}, owner)
  const listedEmail = listedContacts.items.find(item => item.id === emailContact.id)
  assert(listedEmail?.displayValue === 'profile-smoke@example.com', 'self contact list should expose full contact value')

  const updatedContact = await request(`/api/v1/contact-methods/${emailContact.id}`, {
    method: 'PATCH',
    body: {
      type: 'email',
      label: 'Profile Smoke Email Updated',
      displayValue: 'profile-smoke-updated@example.com',
      usageScopes: ['buyer', 'api_merchant'],
      isDefault: true,
      enabled: true,
    },
  }, owner)
  assert(updatedContact.isDefault === true, 'updated contact should become default')
  assert(updatedContact.displayValue === 'profile-smoke-updated@example.com', 'updated contact should return full value')

  const verifiedContact = await request(`/api/v1/contact-methods/${emailContact.id}/verify`, {
    method: 'POST',
    body: {},
  }, owner)
  assert(verifiedContact.verified === true, 'verify action should mark contact verified')
  assert(verifiedContact.displayValue === 'profile-smoke-updated@example.com', 'verified contact should preserve full value for owner')

  const deletedContact = await request(`/api/v1/contact-methods/${ownerContact.id}`, {
    method: 'DELETE',
    body: {},
  }, owner)
  assert(deletedContact.enabled === false, 'delete contact should disable contact')

  const merchantProfile = await request('/api/v1/me/merchant-profile', {
    method: 'POST',
    body: {
      slug: 'profile-smoke-store',
      displayName: 'Profile Smoke Store',
      avatarUrl: '',
    },
  }, owner)
  assert(merchantProfile.ownerUserId === owner.user.id, 'self merchant profile should include owner id')

  const myMerchantProfile = await request('/api/v1/me/merchant-profile', {}, owner)
  assert(myMerchantProfile.slug === 'profile-smoke-store', 'my merchant profile should be readable')

  const publicUser = await request('/api/v1/users/profile-smoke-owner/public-profile')
  assert(publicUser.profile.displayName === 'Profile Smoke Owner', 'public user profile should reflect profile update')
  assert(publicUser.profile.lastActiveAt === null, 'public user profile should respect lastActive privacy')
  const publicUserText = JSON.stringify(publicUser)
  assert(!publicUserText.includes('profile-smoke-updated@example.com'), 'public user profile must not leak contact value')
  assert(!publicUserText.includes(emailContact.id), 'public user profile must not leak contact id')

  const publicMerchant = await request('/api/v1/merchant-profiles/profile-smoke-store')
  assert(publicMerchant.profile.username === 'profile-smoke-store', 'public merchant profile should use slug as public username')
  assert(publicMerchant.profile.displayName === 'Profile Smoke Store', 'public merchant profile should expose display name')
  const publicMerchantText = JSON.stringify(publicMerchant)
  assert(!publicMerchantText.includes(owner.user.id), 'public merchant profile must not expose owner user id')
  assert(!publicMerchantText.includes('profile-smoke-updated@example.com'), 'public merchant profile must not leak contact value')

  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')

  const serviceDraft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-api-service',
    body: {
      merchantProfileId: merchantProfile.id,
      merchantIdentityMode: 'store_alias',
      ownerContactMethodId: emailContact.id,
      title: `Profile Smoke API ${Date.now()}`,
      shortDescription: 'Profile smoke store alias API 服务',
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
      accessModes: [
        { accessMode: 'buyer_dedicated_sub_key', publicNote: '站外确认接入说明。' },
      ],
      models: [
        { modelCatalogId: model.id, merchantMultiplier: '1.0000', enabled: true },
      ],
      packages: [],
    },
  }, owner)
  assert(serviceDraft.merchantIdentityMode === 'store_alias', 'service should use store alias identity')
  assert(serviceDraft.merchantDisplayName === 'Profile Smoke Store', 'owner service response should expose store display name')

  const autoApprovedService = await request(`/api/v1/owner/api-services/${serviceDraft.id}/submit-review`, {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-api-submit',
    ifMatch: serviceDraft.version,
    body: {},
  }, owner)
  assert(autoApprovedService.reviewStatus === 'approved', 'service should be auto-approved')
  assert(autoApprovedService.publicationStatus === 'offline', 'auto-approved service should remain offline')

  const onlineService = await request(`/api/v1/owner/api-services/${serviceDraft.id}/publish`, {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-api-publish',
    ifMatch: autoApprovedService.version,
    body: {},
  }, owner)
  assert(onlineService.publicationStatus === 'online', 'service should be online')

  const publicService = await request(`/api/v1/api-services/${serviceDraft.id}`)
  assert(publicService.merchantIdentityMode === 'store_alias', 'public service should keep store alias identity')
  assert(publicService.merchantDisplayName === 'Profile Smoke Store', 'public service should show merchant profile display name')
  assert(publicService.merchantProfileSlug === 'profile-smoke-store', 'public service should show merchant profile slug')
  const publicServiceText = JSON.stringify(publicService)
  assert(!publicServiceText.includes(owner.user.id), 'public service must not leak owner user id')
  assert(!publicServiceText.includes(emailContact.id), 'public service must not leak owner contact method id')
  assert(!publicServiceText.includes('profile-smoke-updated@example.com'), 'public service must not leak owner contact value')

  const buyerContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-buyer-contact',
    body: {
      type: 'telegram',
      label: 'Profile Smoke Buyer',
      value: '@profile_smoke_buyer',
    },
  }, buyer)
  const intent = await request(`/api/v1/api-services/${serviceDraft.id}/purchase-intents`, {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-api-intent',
    body: {
      buyerContactMethodId: buyerContact.id,
      requestedCnyAmount: '20',
      requestedUsdAllowance: '25',
      selectedAccessMode: 'buyer_dedicated_sub_key',
      selectedPackageId: '',
      buyerNote: 'profile smoke intent',
    },
  }, buyer)
  assert(intent.merchantContact?.value === 'profile-smoke-updated@example.com', 'buyer should see frozen store alias merchant contact after intent')

  console.log(JSON.stringify({
    ok: true,
    profileUserId: owner.user.id,
    merchantProfileId: merchantProfile.id,
    merchantSlug: merchantProfile.slug,
    apiServiceId: serviceDraft.id,
    intentId: intent.id,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
