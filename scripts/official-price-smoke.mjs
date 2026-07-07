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

function recordPayload(planId, suffix, overrides = {}) {
  const now = new Date().toISOString()
  return {
    productPlanId: planId,
    productText: 'ChatGPT',
    planText: `Pro Smoke ${suffix}`,
    regionCode: 'ph',
    channel: 'web',
    openingMethod: `official_web_smoke_${suffix}`,
    sourceUrl: `https://linux.do/t/official-price-smoke-${suffix}/${Date.now()}`,
    observedAt: now,
    billingPeriod: 'monthly',
    currency: 'PHP',
    originalAmount: '7990.00',
    taxIncluded: true,
    fxRateToCny: '0.1230',
    fxSource: 'smoke-fx',
    fxObservedAt: now,
    validFrom: now,
    reason: 'official price admin maintenance smoke',
    ...overrides,
  }
}

function disabledSubmitPayload(suffix) {
  return {
    productText: 'ChatGPT',
    planText: `Pro Smoke ${suffix}`,
    regionCode: 'ph',
    channel: 'web',
    openingMethod: 'official_web',
    sourceUrl: `https://linux.do/t/official-price-disabled-${suffix}`,
    sourceTitle: 'disabled user submit smoke',
    evidenceSummary: '用户提交入口禁用验证。',
    observedAt: new Date().toISOString(),
    billingPeriod: 'monthly',
    currency: 'PHP',
    originalAmount: '7990.00',
    originalPriceText: 'PHP 7,990',
    taxIncluded: true,
  }
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const buyer = await session('official-price-smoke-buyer')
  const admin = await session('official-price-smoke-admin', true)
  const suffix = Date.now().toString(36)

  const productPlans = await request('/api/v1/product-plans')
  const plan = productPlans.items.find(item => item.publishPolicy !== 'blocked') ?? productPlans.items[0]
  assert(plan?.id, 'product plan catalog should not be empty')

  const disabled = await request('/api/v1/official-price-leads', {
    method: 'POST',
    idempotencyPrefix: `official-price-smoke-disabled-${suffix}`,
    expectedStatus: 403,
    body: disabledSubmitPayload(suffix),
  }, buyer)
  assert(disabled.code === 'OFFICIAL_PRICE_USER_SUBMIT_DISABLED', 'user lead submit should be disabled')

  const created = await request('/api/v1/admin/official-price-records', {
    method: 'POST',
    idempotencyPrefix: `official-price-smoke-create-${suffix}`,
    body: recordPayload(plan.id, suffix),
  }, admin)
  assert(created.id, 'admin create should return record id')
  assert(created.status === 'active', 'created record should be active')
  assert(created.normalizedMonthlyCny === '982.77', `unexpected normalized CNY ${created.normalizedMonthlyCny}`)

  const publicPrices = await request('/api/v1/official-prices')
  assert(publicPrices.items.some(item => item.id === created.id), 'public official prices should include created record')

  const publicDetail = await request(`/api/v1/official-prices/${created.id}`)
  assert(publicDetail.id === created.id, 'public official price detail should read created record')
  assert(publicDetail.leadId, 'admin-created record should keep an internal lead reference')

  const updated = await request(`/api/v1/admin/official-price-records/${created.id}`, {
    method: 'PUT',
    idempotencyPrefix: `official-price-smoke-update-${suffix}`,
    ifMatch: created.version,
    body: recordPayload(plan.id, suffix, {
      originalAmount: '6990.00',
      reason: 'official price admin maintenance smoke update',
    }),
  }, admin)
  assert(updated.id && updated.id !== created.id, 'update should create a replacement active record')
  assert(updated.status === 'active', 'updated record should be active')
  assert(updated.normalizedMonthlyCny === '859.77', `unexpected updated normalized CNY ${updated.normalizedMonthlyCny}`)

  const oldPublicDetail = await request(`/api/v1/official-prices/${created.id}`, {
    expectedStatus: 404,
  })
  assert(oldPublicDetail.code === 'OBJECT_NOT_FOUND', 'superseded record should be hidden from public detail')

  const adminRecords = await request('/api/v1/admin/official-price-records', {}, admin)
  assert(adminRecords.items.some(item => item.id === created.id && item.status === 'superseded'), 'admin list should include superseded record')
  assert(adminRecords.items.some(item => item.id === updated.id && item.status === 'active'), 'admin list should include replacement active record')

  const takenDown = await request(`/api/v1/admin/official-price-records/${updated.id}/take-down`, {
    method: 'POST',
    idempotencyPrefix: `official-price-smoke-take-down-${suffix}`,
    ifMatch: updated.version,
    body: { reason: 'official price admin maintenance smoke take down' },
  }, admin)
  assert(takenDown.status === 'taken_down', 'take-down should mark record taken_down')

  const hiddenDetail = await request(`/api/v1/official-prices/${updated.id}`, {
    expectedStatus: 404,
  })
  assert(hiddenDetail.code === 'OBJECT_NOT_FOUND', 'taken-down record should be hidden from public detail')

  console.log(JSON.stringify({
    ok: true,
    disabledSubmitCode: disabled.code,
    createdRecordId: created.id,
    updatedRecordId: updated.id,
    takenDownStatus: takenDown.status,
    adminRecordCount: adminRecords.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
