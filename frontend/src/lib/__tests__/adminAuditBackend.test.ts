import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const adminPageSource = readFileSync(new URL('../../pages/AdminPage.vue', import.meta.url), 'utf8')
const adminSectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': status >= 400 ? 'application/problem+json' : 'application/json' },
  })
}

function adminSession() {
  return {
    csrfToken: 'csrf-admin-audit',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: '11111111-1111-4111-8111-111111111111',
      analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
      username: 'admin',
      displayName: 'Admin',
      isAdmin: true,
      permissions: ['admin'],
      linuxDoBinding: { bound: true },
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('real admin audit adapter serializes filters and maps safe rows', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(adminSession()))
    .mockResolvedValueOnce(jsonResponse({
      items: [{
        id: '22222222-2222-4222-8222-222222222222',
        actorUserId: '11111111-1111-4111-8111-111111111111',
        actorUsername: 'admin',
        action: 'user.account_status_changed',
        targetType: 'user',
        targetId: '33333333-3333-4333-8333-333333333333',
        reason: '异常登录核查',
        requestId: 'request-audit-1',
        beforeStatus: 'active',
        afterStatus: 'suspended',
        createdAt: '2026-08-01T12:00:00Z',
      }],
      nextCursor: ' next-audit-page ',
    }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendAdminAuditLogRowsPage } = await import('../adminAuditBackend')
  const page = await backendAdminAuditLogRowsPage({
    search: '异常登录',
    action: 'user.account_status_changed',
    targetType: 'user',
    actorUserId: '11111111-1111-4111-8111-111111111111',
    targetId: '33333333-3333-4333-8333-333333333333',
  }, { limit: 20, cursor: 'opaque+/=' })

  const [path, query = ''] = String(fetchMock.mock.calls[1]?.[0]).split('?')
  assert.equal(path, '/api/v1/admin/audit-logs')
  assert.deepEqual(Object.fromEntries(new URLSearchParams(query)), {
    search: '异常登录',
    action: 'user.account_status_changed',
    targetType: 'user',
    actorUserId: '11111111-1111-4111-8111-111111111111',
    targetId: '33333333-3333-4333-8333-333333333333',
    limit: '20',
    cursor: 'opaque+/=',
  })
  assert.equal(page.nextCursor, 'next-audit-page')
  assert.deepEqual(page.items[0], {
    id: '22222222-2222-4222-8222-222222222222',
    primary: 'user.account_status_changed',
    secondary: 'user · 33333333-3333-4333-8333-333333333333 · active → suspended',
    owner: 'admin',
    status: '已记录',
    risk: '异常登录核查',
    targetType: 'audit-log',
    backendKind: 'admin-audit-log',
    detailItems: [
      { label: '目标类型', value: 'user' },
      { label: '目标 ID', value: '33333333-3333-4333-8333-333333333333' },
      { label: '管理员 ID', value: '11111111-1111-4111-8111-111111111111' },
      { label: '操作时间', value: '2026-08-01T12:00:00Z' },
      { label: '请求追踪', value: 'request-audit-1' },
    ],
  })
})

test('real admin audit reads never fall back to session mock logs', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(adminSession()))
    .mockResolvedValueOnce(jsonResponse({ title: 'Database unavailable', status: 503, detail: '审计库暂不可用' }, 503))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { getAdminSectionRows } = await import('../api')
  await assert.rejects(() => getAdminSectionRows('logs'), /审计库暂不可用/)
  assert.equal(fetchMock.mock.calls.length, 2)
})

test('admin overview and log page share the server audit source', () => {
  assert.match(apiSource, /section === 'logs'[\s\S]*backendAdminAuditLogRows\(\)/)
  assert.match(apiSource, /section === 'logs'[\s\S]*backendAdminAuditLogRowsPage\(/)
  assert.match(adminPageSource, /useAdminSectionRows\('logs'\)/)
  assert.match(adminSectionSource, /serverPagedSections[\s\S]*'logs'/)
})
