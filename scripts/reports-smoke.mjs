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

async function updatePublicProfile(auth, displayName) {
  return request('/api/v1/me/profile', {
    method: 'PATCH',
    body: {
      displayName,
      username: auth.user.username,
      bio: '举报纠纷 smoke 公开主页。',
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
  }, auth)
}

function assertPublicSafe(value) {
  const text = JSON.stringify(value).toLowerCase()
  const forbidden = [
    'reporteruserid',
    'handledbyadminid',
    'openedbyadminid',
    'adminreason',
    'password',
    'api key',
    'token',
    'session',
    'cookie',
    'recovery',
    '@reports_smoke',
  ]
  for (const word of forbidden) {
    assert(!text.includes(word), `public dispute payload leaked forbidden text: ${word}`)
  }
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const reporter = await session('reports-smoke-reporter')
  const reported = await session('reports-smoke-target')
  const admin = await session('reports-smoke-admin', true)

  await updatePublicProfile(reporter, 'Reports Smoke Reporter')
  await updatePublicProfile(reported, 'Reports Smoke Target')

  const publicReport = await request('/api/v1/reports', {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-public-user',
    body: {
      targetType: 'public_user',
      targetId: reported.user.username,
      targetLabel: `公开主页 @${reported.user.username}`,
      reportedUsername: reported.user.username,
      reasonCode: 'other',
      title: '公开主页信息需要复核',
      description: '公开资料描述和实际沟通不一致，请管理员复核脱敏上下文。',
    },
  }, reporter)
  assert(publicReport.status === 'submitted', 'public user report should be submitted')

  const contactReport = await request('/api/v1/reports', {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-contact',
    body: {
      targetType: 'contact_snapshot',
      targetId: `contact-snapshot-${Date.now()}`,
      targetLabel: '联系快照 smoke',
      reportedUsername: reported.user.username,
      reasonCode: 'unreachable',
      title: '联系方式无法联系',
      description: '站内联系快照显示可联系，但实际没有回应；这里只提交脱敏说明。',
    },
  }, reporter)
  assert(contactReport.status === 'submitted', 'contact report should be submitted')

  const adminReports = await request('/api/v1/admin/reports', {}, admin)
  assert(adminReports.items.some(item => item.id === publicReport.id), 'admin reports should include public report')
  assert(adminReports.items.some(item => item.id === contactReport.id), 'admin reports should include contact report')

  const reportDetail = await request(`/api/v1/admin/reports/${publicReport.id}`, {}, admin)
  assert(reportDetail.version === publicReport.version, 'admin report detail should expose version')

  const opened = await request(`/api/v1/admin/reports/${publicReport.id}/open-dispute`, {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-open-dispute',
    ifMatch: reportDetail.version,
    body: {
      reason: '公开主页举报进入纠纷处理。',
      publicSummary: '公开主页信息争议',
      publicResult: '已进入人工处理中',
    },
  }, admin)
  assert(opened.report?.status === 'dispute_opened', 'report should be linked to dispute')
  assert(opened.dispute?.status === 'open', 'dispute should be open')
  assert(opened.dispute.publicSummary === '公开主页信息争议', 'dispute public summary mismatch')

  const publicDisputes = await request(`/api/v1/users/${reported.user.username}/disputes`)
  const publicDispute = publicDisputes.items.find(item => item.id === opened.dispute.id)
  assert(publicDispute?.unresolved === true, 'public user disputes should show unresolved dispute')
  assert(publicDispute.type === '公开主页信息争议', 'public dispute summary should be sanitized')
  assertPublicSafe(publicDisputes)

  const publicProfile = await request(`/api/v1/users/${reported.user.username}/public-profile`)
  assert(publicProfile.profile.stats.unresolvedDisputeCount >= 1, 'public profile stats should include unresolved dispute')
  assert(publicProfile.disputes.some(item => item.id === opened.dispute.id), 'public profile should include public dispute summary')
  assertPublicSafe(publicProfile.disputes)

  const disputeDetail = await request(`/api/v1/admin/disputes/${opened.dispute.id}`, {}, admin)
  const resolved = await request(`/api/v1/admin/disputes/${opened.dispute.id}/resolve`, {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-resolve-dispute',
    ifMatch: disputeDetail.version,
    body: {
      reason: '双方补充说明后记录处理结果。',
      publicSummary: '公开主页信息争议',
      publicResult: '已记录处理结果',
    },
  }, admin)
  assert(resolved.dispute?.status === 'resolved', 'dispute should resolve')

  const appeal = await request('/api/v1/me/appeals', {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-appeal',
    body: {
      reportId: publicReport.id,
      disputeId: opened.dispute.id,
      title: '申请复核公开纠纷记录',
      statement: '用户已补充脱敏说明，申请管理员复核处理结果。',
    },
  }, reported)
  assert(appeal.status === 'submitted', 'appeal should be submitted')

  const myAppeals = await request('/api/v1/me/appeals', {}, reported)
  assert(myAppeals.items.some(item => item.id === appeal.id), 'my appeals should include submitted appeal')

  const adminAppeals = await request('/api/v1/admin/appeals', {}, admin)
  assert(adminAppeals.items.some(item => item.id === appeal.id), 'admin appeals should include submitted appeal')

  const appealDetail = await request(`/api/v1/admin/appeals/${appeal.id}`, {}, admin)
  const approvedAppeal = await request(`/api/v1/admin/appeals/${appeal.id}/approve`, {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-approve-appeal',
    ifMatch: appealDetail.version,
    body: {
      reason: '申诉材料已补充，记录通过。',
    },
  }, admin)
  assert(approvedAppeal.appeal?.status === 'approved', 'appeal should be approved')

  const rejected = await request(`/api/v1/admin/reports/${contactReport.id}/reject`, {
    method: 'POST',
    idempotencyPrefix: 'reports-smoke-reject-contact',
    ifMatch: contactReport.version,
    body: {
      reason: '联系快照举报信息不足，先关闭该条。',
    },
  }, admin)
  assert(rejected.report?.status === 'rejected', 'contact report should be rejected')

  console.log(JSON.stringify({
    ok: true,
    reportId: publicReport.id,
    contactReportId: contactReport.id,
    disputeId: opened.dispute.id,
    appealId: appeal.id,
    publicDisputeCount: publicDisputes.items.length,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
