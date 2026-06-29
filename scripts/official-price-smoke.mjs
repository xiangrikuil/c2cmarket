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

async function submitLead(auth, suffix, overrides = {}) {
  return request('/api/v1/official-price-leads', {
    method: 'POST',
    idempotencyPrefix: `official-price-smoke-lead-${suffix}`,
    body: {
      productText: 'ChatGPT',
      planText: `Pro Smoke ${suffix}`,
      regionCode: 'ph',
      channel: 'Web',
      openingMethod: 'Apple Store / local card',
      sourceUrl: `https://linux.do/t/official-price-smoke-${suffix}/${Date.now()}`,
      sourceTitle: `Official price smoke ${suffix}`,
      evidenceSummary: 'Smoke 验证真实后端低价线索提交和审核流。',
      note: '不包含支付、托管、担保或凭据内容。',
      observedAt: new Date().toISOString(),
      billingPeriod: 'monthly',
      currency: 'PHP',
      originalAmount: overrides.originalAmount ?? '7990.00',
      originalPriceText: overrides.originalPriceText ?? 'PHP 7,990',
      taxIncluded: true,
      ...overrides,
    },
  }, auth)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const buyer = await session('official-price-smoke-buyer')
  const admin = await session('official-price-smoke-admin', true)

  const productPlans = await request('/api/v1/product-plans')
  const plan = productPlans.items.find(item => item.publishPolicy !== 'blocked') ?? productPlans.items[0]
  assert(plan?.id, 'product plan catalog should not be empty')

  const lead = await submitLead(buyer, 'approve')
  assert(lead.status === 'pending', 'submitted lead should be pending')
  assert(lead.version === 1, 'submitted lead should start at version 1')

  const myLeads = await request('/api/v1/me/official-price-leads', {}, buyer)
  assert(myLeads.items.some(item => item.id === lead.id), 'my official price leads should include submitted lead')

  const adminLeads = await request('/api/v1/admin/official-price-leads', {}, admin)
  assert(adminLeads.items.some(item => item.id === lead.id), 'admin leads should include submitted lead')

  const adminDetail = await request(`/api/v1/admin/official-price-leads/${lead.id}`, {}, admin)
  assert(adminDetail.id === lead.id, 'admin detail should read submitted lead')
  assert(adminDetail.channel === 'Web', 'admin detail should include channel')
  assert(adminDetail.version === 1, 'admin detail should include version for If-Match')

  const approved = await request(`/api/v1/admin/official-price-leads/${lead.id}/approve`, {
    method: 'POST',
    idempotencyPrefix: 'official-price-smoke-approve',
    ifMatch: adminDetail.version,
    body: {
      reason: 'official price smoke approve',
      resolvedProductPlanId: plan.id,
      validFrom: new Date().toISOString(),
      fxSnapshot: {
        rateToCny: '0.1230',
        source: 'smoke-fx',
        observedAt: new Date().toISOString(),
      },
    },
  }, admin)
  assert(approved.lead.status === 'approved', 'lead should be approved')
  assert(approved.record.id, 'approve should create public price record')
  assert(approved.record.normalizedMonthlyCny, 'approved record should include normalized monthly CNY')

  const publicPrices = await request('/api/v1/official-prices')
  assert(publicPrices.items.some(item => item.id === approved.record.id), 'public official prices should include approved record')

  const publicDetail = await request(`/api/v1/official-prices/${approved.record.id}`)
  assert(publicDetail.id === approved.record.id, 'public official price detail should read approved record')
  assert(publicDetail.leadId === lead.id, 'public record should point back to approved lead')

  const changesLead = await submitLead(buyer, 'changes', {
    originalAmount: '6990.00',
    originalPriceText: 'PHP 6,990',
  })
  const changesDetail = await request(`/api/v1/admin/official-price-leads/${changesLead.id}`, {}, admin)
  const changesRequested = await request(`/api/v1/admin/official-price-leads/${changesLead.id}/request-changes`, {
    method: 'POST',
    idempotencyPrefix: 'official-price-smoke-request-changes',
    ifMatch: changesDetail.version,
    body: { reason: 'smoke request changes' },
  }, admin)
  assert(changesRequested.status === 'changes_requested', 'request-changes should move lead to changes_requested')

  const rejectLead = await submitLead(buyer, 'reject', {
    originalAmount: '8990.00',
    originalPriceText: 'PHP 8,990',
  })
  const rejectDetail = await request(`/api/v1/admin/official-price-leads/${rejectLead.id}`, {}, admin)
  const rejected = await request(`/api/v1/admin/official-price-leads/${rejectLead.id}/reject`, {
    method: 'POST',
    idempotencyPrefix: 'official-price-smoke-reject',
    ifMatch: rejectDetail.version,
    body: { reason: 'smoke reject' },
  }, admin)
  assert(rejected.status === 'rejected', 'reject should move lead to rejected')

  console.log(JSON.stringify({
    ok: true,
    approvedLeadId: lead.id,
    recordId: approved.record.id,
    changesLeadId: changesLead.id,
    rejectLeadId: rejectLead.id,
    publicPriceCount: publicPrices.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
