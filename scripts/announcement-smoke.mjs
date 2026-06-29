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
    ...(options.headers ?? {}),
  }
  const response = await fetch(`${baseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })
  return decode(response, options.expectedStatus)
}

function announcementInput(overrides = {}) {
  const now = new Date()
  const publishAt = new Date(now.getTime() - 60_000).toISOString()
  const expireAt = new Date(now.getTime() + 86_400_000).toISOString()
  return {
    title: `Smoke 公告 ${Date.now()}`,
    summary: '这是一条用于真实后端闭环验证的公告摘要。',
    contentMarkdown: '## 公告 smoke\n\n这条公告只验证平台通知，不承载支付、托管或凭据交付。',
    category: 'platform',
    level: 'important',
    channels: ['message_center', 'home_banner'],
    isPinned: true,
    isDismissible: true,
    ctaLabel: '查看首页',
    ctaUrl: '/',
    publishAt,
    expireAt,
    ...overrides,
  }
}

function ids(items) {
  return new Set(items.map(item => item.id))
}

async function main() {
  const health = await request('/health')
  assert(health.status === 'ok', 'backend health check failed')

  const admin = await session('announcement-smoke-admin', true)
  const user = await session('announcement-smoke-user')
  const nonAdmin = await session('announcement-smoke-non-admin')

  const initialUserList = await request('/api/v1/announcements', {}, user)
  assert(Array.isArray(initialUserList.items), 'announcement list should return items')

  const forbidden = await request('/api/v1/admin/announcements', { expectedStatus: 403 }, nonAdmin)
  assert(forbidden.code === 'PERMISSION_DENIED', 'non-admin should be rejected from admin announcement list')

  const draft = await request('/api/v1/admin/announcements', {
    method: 'POST',
    idempotencyPrefix: 'announcement-smoke-create',
    body: announcementInput(),
  }, admin)
  assert(draft.status === 'draft', 'created announcement should start as draft')
  assert(draft.slug, 'created announcement should include slug')

  const draftUserList = await request('/api/v1/announcements', {}, user)
  assert(!ids(draftUserList.items).has(draft.id), 'draft announcement should not be user-visible')

  const adminDetail = await request(`/api/v1/admin/announcements/${draft.id}`, {}, admin)
  assert(adminDetail.id === draft.id, 'admin detail should read created announcement')

  const updated = await request(`/api/v1/admin/announcements/${draft.id}`, {
    method: 'PATCH',
    body: announcementInput({
      title: draft.title,
      summary: '这是一条更新后的真实后端公告 smoke 摘要。',
      contentMarkdown: '## 更新后的公告 smoke\n\n更新已发布公告后，receipt 版本应重新生效。',
    }),
  }, admin)
  assert(updated.version === draft.version + 1, 'update should increment version')

  const published = await request(`/api/v1/admin/announcements/${draft.id}/publish`, {
    method: 'POST',
    body: {},
  }, admin)
  assert(published.status === 'published', 'publish should publish immediate announcement')

  const userList = await request('/api/v1/announcements', {}, user)
  assert(ids(userList.items).has(published.id), 'published announcement should be user-visible')
  const listed = userList.items.find(item => item.id === published.id)
  assert(listed.receipt === undefined, 'unseen announcement should not include a receipt')

  const activeHomeList = await request('/api/v1/announcements/active?channel=home_banner', {}, user)
  assert(ids(activeHomeList.items).has(published.id), 'home banner active list should include published announcement')

  const home = await request('/api/v1/announcements/home', {}, user)
  assert(home?.id === published.id, 'home endpoint should return the smoke announcement')

  const detail = await request(`/api/v1/announcements/${published.slug}`, {}, user)
  assert(detail.id === published.id, 'slug detail should return published announcement')

  const unreadBefore = await request('/api/v1/me/announcements/unread-count', {}, user)
  const importantUnreadBefore = await request('/api/v1/me/announcements/important-unread-count', {}, user)
  assert(unreadBefore.count >= 1, 'unread count should include smoke announcement before reading')
  assert(importantUnreadBefore.count >= 1, 'important unread count should include smoke announcement before reading')

  const seenReceipt = await request(`/api/v1/me/announcements/${published.id}/seen`, {
    method: 'POST',
    body: {},
  }, user)
  assert(seenReceipt.announcementId === published.id, 'seen receipt should point to announcement')
  assert(seenReceipt.firstSeenAt, 'seen receipt should include firstSeenAt')

  const readReceipt = await request(`/api/v1/me/announcements/${published.id}/read`, {
    method: 'POST',
    body: {},
  }, user)
  assert(readReceipt.readAt, 'read receipt should include readAt')

  const afterReadList = await request('/api/v1/announcements', {}, user)
  const afterReadItem = afterReadList.items.find(item => item.id === published.id)
  assert(afterReadItem?.receipt?.readAt, 'announcement list should include current user read receipt')

  const unreadAfterRead = await request('/api/v1/me/announcements/unread-count', {}, user)
  assert(unreadAfterRead.count === unreadBefore.count - 1, 'reading announcement should reduce unread count by one')

  const editedPublished = await request(`/api/v1/admin/announcements/${published.id}`, {
    method: 'PATCH',
    body: announcementInput({
      title: published.title,
      summary: '发布后再次编辑，验证 receipt 版本失效。',
      contentMarkdown: '## 发布后编辑\n\n版本变化后旧已读 receipt 不应让新版公告保持已读。',
    }),
  }, admin)
  assert(editedPublished.version === published.version + 1, 'editing published announcement should increment version')

  const unreadAfterVersionChange = await request('/api/v1/me/announcements/unread-count', {}, user)
  assert(unreadAfterVersionChange.count === unreadAfterRead.count + 1, 'version change should make announcement unread again')

  const duplicate = await request(`/api/v1/admin/announcements/${published.id}/duplicate`, {
    method: 'POST',
    idempotencyPrefix: 'announcement-smoke-duplicate',
    body: {},
  }, admin)
  assert(duplicate.status === 'draft', 'duplicated announcement should be a draft')

  const userListAfterDuplicate = await request('/api/v1/announcements', {}, user)
  assert(!ids(userListAfterDuplicate.items).has(duplicate.id), 'duplicated draft should not be user-visible')

  const dismissedReceipt = await request(`/api/v1/me/announcements/${published.id}/dismiss`, {
    method: 'POST',
    body: {},
  }, user)
  assert(dismissedReceipt.dismissedAt, 'dismiss receipt should include dismissedAt')

  const homeAfterDismiss = await request('/api/v1/announcements/home', {}, user)
  assert(homeAfterDismiss === null || homeAfterDismiss.id !== published.id, 'dismissed announcement should leave home banner')

  const offlined = await request(`/api/v1/admin/announcements/${published.id}/offline`, {
    method: 'POST',
    body: { reason: 'announcement smoke offline' },
  }, admin)
  assert(offlined.status === 'offline', 'offline action should mark announcement offline')

  await request(`/api/v1/announcements/${published.slug}`, { expectedStatus: 404 }, user)
  const userListAfterOffline = await request('/api/v1/announcements', {}, user)
  assert(!ids(userListAfterOffline.items).has(published.id), 'offline announcement should not be user-visible')

  const audit = await request('/api/v1/admin/announcement-audit-logs', {}, admin)
  const auditActions = audit.items
    .filter(item => item.announcementId === published.id || item.announcementId === duplicate.id)
    .map(item => item.action)
  for (const action of [
    'announcement_created',
    'announcement_updated',
    'announcement_published',
    'announcement_duplicated',
    'announcement_offlined',
  ]) {
    assert(auditActions.includes(action), `audit logs should include ${action}`)
  }

  const adminList = await request('/api/v1/admin/announcements', {}, admin)
  assert(ids(adminList.items).has(published.id), 'admin list should still include offline announcement')
  assert(ids(adminList.items).has(duplicate.id), 'admin list should include duplicated draft')

  console.log(JSON.stringify({
    ok: true,
    announcementId: published.id,
    duplicateId: duplicate.id,
    slug: published.slug,
    unreadBefore: unreadBefore.count,
    unreadAfterRead: unreadAfterRead.count,
    unreadAfterVersionChange: unreadAfterVersionChange.count,
    auditActions,
  }, null, 2))
}

main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
