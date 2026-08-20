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

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const ownerUsername = `profile-${userSuffix}`
  const merchantSlug = `profile-store-${userSuffix}`
  const emailValue = `profile-smoke-${runSuffix}@example.com`
  const updatedEmailValue = `profile-updated-${runSuffix}@example.com`
  const ownerContactValue = `@profile_owner_${runSuffix.replaceAll('-', '_')}`
  const buyerContactValue = `@profile_buyer_${runSuffix.replaceAll('-', '_')}`
  const owner = await linuxDoSession(ownerUsername)
  const buyer = await session(`prof-buy-${userSuffix}`)
  const probeConnectionId = seedVerifiedProbeConnection(owner.user.id)

  const originalProfile = await request('/api/v1/me/profile', {}, owner)
  assert(originalProfile.username === ownerUsername, 'owner profile should match dev session')

  const updatedProfile = await request('/api/v1/me/profile', {
    method: 'PATCH',
    body: {
      displayName: 'Profile Smoke Owner',
      username: ownerUsername,
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
      value: ownerContactValue,
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
      value: emailValue,
      isDefault: false,
      enabled: true,
    },
  }, owner)

  const listedContacts = await request('/api/v1/me/contact-methods', {}, owner)
  const listedEmail = listedContacts.items.find(item => item.id === emailContact.id)
  assert(listedEmail?.displayValue === emailValue, 'self contact list should expose full contact value')

  const updatedContact = await request(`/api/v1/contact-methods/${emailContact.id}`, {
    method: 'PATCH',
    body: {
      type: 'email',
      label: 'Profile Smoke Email Updated',
      displayValue: updatedEmailValue,
      isDefault: true,
      enabled: true,
    },
  }, owner)
  assert(updatedContact.isDefault === true, 'updated contact should become default')
  assert(updatedContact.displayValue === updatedEmailValue, 'updated contact should return full value')

  const verifiedContact = await request(`/api/v1/contact-methods/${emailContact.id}/verify`, {
    method: 'POST',
    body: {},
  }, owner)
  assert(verifiedContact.verified === true, 'verify action should mark contact verified')
  assert(verifiedContact.displayValue === updatedEmailValue, 'verified contact should preserve full value for owner')

  const deletedContact = await request(`/api/v1/contact-methods/${ownerContact.id}`, {
    method: 'DELETE',
    body: {},
  }, owner)
  assert(deletedContact.enabled === false, 'delete contact should disable contact')

  const merchantProfile = await request('/api/v1/me/merchant-profile', {
    method: 'POST',
    body: {
      slug: merchantSlug,
      displayName: 'Profile Smoke Store',
      avatarUrl: '',
    },
  }, owner)
  assert(merchantProfile.ownerUserId === owner.user.id, 'self merchant profile should include owner id')

  const myMerchantProfile = await request('/api/v1/me/merchant-profile', {}, owner)
  assert(myMerchantProfile.slug === merchantSlug, 'my merchant profile should be readable')

  const publicUser = await request(`/api/v1/users/${ownerUsername}/public-profile`)
  assert(publicUser.profile.displayName === 'Profile Smoke Owner', 'public user profile should reflect profile update')
  assert(publicUser.profile.lastActiveAt === null, 'public user profile should respect lastActive privacy')
  const publicUserText = JSON.stringify(publicUser)
  assert(!publicUserText.includes(updatedEmailValue), 'public user profile must not leak contact value')
  assert(!publicUserText.includes(emailContact.id), 'public user profile must not leak contact id')
  for (const field of ['carpools', 'services', 'completions', 'reviews', 'disputes']) {
    assert(Array.isArray(publicUser[field]), `public user ${field} should be a typed array`)
  }

  const publicMerchant = await request(`/api/v1/merchant-profiles/${merchantSlug}`)
  assert(publicMerchant.username === merchantSlug, 'public merchant profile should use slug as public username')
  assert(publicMerchant.displayName === 'Profile Smoke Store', 'public merchant profile should expose display name')
  for (const removedField of ['profile', 'services', 'completions', 'reviews', 'disputes']) {
    assert(!(removedField in publicMerchant), `public merchant profile must not include ${removedField}`)
  }
  const publicMerchantText = JSON.stringify(publicMerchant)
  assert(!publicMerchantText.includes(owner.user.id), 'public merchant profile must not expose owner user id')
  assert(!publicMerchantText.includes(updatedEmailValue), 'public merchant profile must not leak contact value')

  const models = await request('/api/v1/api-models')
  const model = models.items[0]
  assert(model?.id, 'api model catalog is empty')

  const serviceDraft = await request('/api/v1/owner/api-services', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-api-service',
    body: {
      merchantProfileId: '',
      merchantIdentityMode: 'public_profile',
      ownerContactMethodId: emailContact.id,
      probeConnectionId,
      title: `Profile Smoke API ${Date.now()}`,
      shortDescription: 'Profile smoke 公开主页 API 服务',
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
      accountPoolCustomName: 'Profile Smoke Pool',
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
  assert(serviceDraft.merchantIdentityMode === 'public_profile', 'service should use public profile identity')
  assert(serviceDraft.merchantDisplayName === 'Profile Smoke Owner', 'owner service response should expose public profile display name')

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
          paymentInstructions: '微信收款方式由商户站外确认，平台不处理支付。',
          paymentQrCodeDataUrl: 'data:image/png;base64,aGVsbG8=',
        },
      ],
    },
  }, owner)
  assert(orderableService.isOrderable === true, 'service should be orderable after settings')

  const publicService = await request(`/api/v1/api-services/${serviceDraft.id}`)
  assert(publicService.merchantIdentityMode === 'public_profile', 'public service should keep public profile identity')
  assert(publicService.merchantDisplayName === 'Profile Smoke Owner', 'public service should show public profile display name')
  const publicServiceText = JSON.stringify(publicService)
  assert(!publicServiceText.includes(owner.user.id), 'public service must not leak owner user id')
  assert(!publicServiceText.includes(emailContact.id), 'public service must not leak owner contact method id')
  assert(!publicServiceText.includes(updatedEmailValue), 'public service must not leak owner contact value')

  const enrichedPublicUser = await request(`/api/v1/users/${ownerUsername}/public-profile`)
  assert(enrichedPublicUser.services.some(item => item.id === serviceDraft.id), 'public user profile should aggregate the online public-profile API service')
  assert(!JSON.stringify(enrichedPublicUser).includes(emailContact.id), 'aggregated public profile must not leak contact ids')

  const buyerContact = await request('/api/v1/contact-methods', {
    method: 'POST',
    idempotencyPrefix: 'profile-smoke-buyer-contact',
    body: {
      type: 'wechat',
      label: 'Profile Smoke Buyer',
      value: buyerContactValue,
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
  assert(intent.merchantContact?.value === updatedEmailValue, 'buyer should see frozen public-profile merchant contact after intent')

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
