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

async function submitDemand(auth, suffix, overrides = {}) {
  return request('/api/v1/demands', {
    method: 'POST',
    idempotencyPrefix: `demand-smoke-create-${suffix}`,
    body: {
      title: `Smoke 求 ChatGPT Business ${suffix}`,
      maxPriceCny: '199.00',
      regionCode: 'us',
      ownerPreference: 'personal',
      sourceUrl: `https://linux.do/t/demand-smoke-${suffix}/${Date.now()}`,
      note: '只记录求车上下文，后续站外确认；平台不处理支付、托管、担保或凭据。',
      ...overrides,
    },
  }, auth)
}

async function adminAction(admin, demand, action, reason, expectedStatus) {
  return request(`/api/v1/admin/demands/${demand.id}/${action}`, {
    method: 'POST',
    idempotencyPrefix: `demand-smoke-${action}`,
    ifMatch: demand.version,
    body: { reason },
    expectedStatus,
  }, admin)
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const buyer = await session('demand-smoke-buyer')
  const otherBuyer = await session('demand-smoke-other')
  const admin = await session('demand-smoke-admin', true)

  const created = await submitDemand(buyer, 'approve')
  assert(created.status === 'pending_review', 'created demand should wait for review')
  assert(created.version === 1, 'created demand should start at version 1')

  await request(`/api/v1/demands/${created.id}`, { expectedStatus: 404 })

  const myList = await request('/api/v1/me/demands', {}, buyer)
  assert(myList.items.some(item => item.id === created.id), 'my demands should include submitted demand')

  const myDetail = await request(`/api/v1/me/demands/${created.id}`, {}, buyer)
  assert(myDetail.id === created.id && myDetail.version === 1, 'my demand detail should include version')

  await request(`/api/v1/me/demands/${created.id}`, { expectedStatus: 404 }, otherBuyer)

  const adminList = await request('/api/v1/admin/demands', {}, admin)
  assert(adminList.items.some(item => item.id === created.id), 'admin list should include pending demand')

  const adminDetail = await request(`/api/v1/admin/demands/${created.id}`, {}, admin)
  assert(adminDetail.publisherUserId === buyer.user.id, 'admin detail should include publisher user id')

  const approved = await adminAction(admin, adminDetail, 'approve', 'demand smoke approve', 200)
  assert(approved.status === 'active', 'admin approve should activate demand')

  const publicList = await request('/api/v1/demands')
  assert(publicList.items.some(item => item.id === created.id), 'public list should include approved demand')
  const publicDetail = await request(`/api/v1/demands/${created.id}`)
  assert(publicDetail.id === created.id, 'public detail should read approved demand')
  assert(!publicDetail.publisherUserId, 'public demand must not expose publisher user id')

  const closed = await request(`/api/v1/me/demands/${created.id}/close`, {
    method: 'POST',
    idempotencyPrefix: 'demand-smoke-close',
    ifMatch: approved.version,
    body: {},
  }, buyer)
  assert(closed.status === 'closed', 'publisher close should close demand')

  await request(`/api/v1/demands/${created.id}`, { expectedStatus: 404 })

  const reopened = await request(`/api/v1/me/demands/${created.id}/reopen`, {
    method: 'POST',
    idempotencyPrefix: 'demand-smoke-reopen',
    ifMatch: closed.version,
    body: {},
  }, buyer)
  assert(reopened.status === 'pending_review', 'reopen should return demand to review')

  const reapproved = await adminAction(admin, reopened, 'approve', 'demand smoke reapprove', 200)
  assert(reapproved.status === 'active', 'reopened demand can be reapproved')

  const takenDown = await adminAction(admin, reapproved, 'take-down', 'demand smoke take down', 200)
  assert(takenDown.status === 'taken_down', 'take-down should hide active demand')
  await request(`/api/v1/demands/${created.id}`, { expectedStatus: 404 })

  const restored = await adminAction(admin, takenDown, 'restore', 'demand smoke restore', 200)
  assert(restored.status === 'active', 'restore should return demand to active')

  const changesDemand = await submitDemand(buyer, 'changes')
  const changesRequested = await adminAction(admin, changesDemand, 'request-changes', 'demand smoke request changes', 200)
  assert(changesRequested.status === 'changes_requested', 'request-changes should move demand to changes_requested')

  const rejectedDemand = await submitDemand(buyer, 'reject')
  const rejected = await adminAction(admin, rejectedDemand, 'reject', 'demand smoke reject', 200)
  assert(rejected.status === 'rejected', 'reject should reject demand')

  const duplicateApprove = await adminAction(admin, restored, 'approve', 'demand smoke duplicate approve', 409)
  assert(duplicateApprove.code === 'INVALID_STATE_TRANSITION', 'duplicate approve should fail with invalid transition')

  console.log(JSON.stringify({
    ok: true,
    demandId: created.id,
    finalStatus: restored.status,
    changesDemandId: changesDemand.id,
    rejectedDemandId: rejectedDemand.id,
    publicDemandCount: publicList.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
