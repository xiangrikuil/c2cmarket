import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const adminPageSource = readFileSync(new URL('../../pages/AdminPage.vue', import.meta.url), 'utf8')
const adminAuditPageSource = readFileSync(new URL('../../pages/AdminAuditLogsPage.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': status >= 400 ? 'application/problem+json' : 'application/json' },
  })
}

function adminSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-admin-audit',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: '11111111-1111-4111-8111-111111111111',
      analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
      username: 'admin',
      displayName: 'Admin',
      isAdmin: true,
      permissions: ['admin'],
      capabilities: ['admin.access'],
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
        sourceKind: 'admin',
        domain: 'account',
        actorKind: 'admin',
        actorUserId: '11111111-1111-4111-8111-111111111111',
        actorUsername: 'admin',
        action: 'user.account_status_changed',
        actionLabel: '调整账号状态',
        targetType: 'user',
        targetId: '33333333-3333-4333-8333-333333333333',
        targetLabel: 'orbit',
        outcome: 'status_changed',
        summary: '管理员完成账号状态调整。',
        detailPath: '/admin/users',
        requestId: 'request-audit-1',
        createdAt: '2026-08-01T12:00:00Z',
      }],
      nextCursor: ' next-audit-page ',
    }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendAdminAuditLogRowsPage } = await import('../adminAuditBackend')
  const page = await backendAdminAuditLogRowsPage({
    sourceKind: 'admin',
    domain: 'account',
    search: '异常登录',
    action: 'user.account_status_changed',
    actorKind: 'admin',
    targetType: 'user',
    actorUserId: '11111111-1111-4111-8111-111111111111',
    targetId: '33333333-3333-4333-8333-333333333333',
    outcome: 'status_changed',
    from: '2026-08-01T00:00:00Z',
    to: '2026-08-02T00:00:00Z',
  }, { limit: 20, cursor: 'opaque+/=' })

  const [path, query = ''] = String(fetchMock.mock.calls[1]?.[0]).split('?')
  assert.equal(path, '/api/v1/admin/audit-logs')
  assert.deepEqual(Object.fromEntries(new URLSearchParams(query)), {
    sourceKind: 'admin',
    domain: 'account',
    actorKind: 'admin',
    search: '异常登录',
    action: 'user.account_status_changed',
    targetType: 'user',
    actorUserId: '11111111-1111-4111-8111-111111111111',
    targetId: '33333333-3333-4333-8333-333333333333',
    outcome: 'status_changed',
    from: '2026-08-01T00:00:00Z',
    to: '2026-08-02T00:00:00Z',
    limit: '20',
    cursor: 'opaque+/=',
  })
  assert.equal(page.nextCursor, 'next-audit-page')
  assert.deepEqual(page.items[0], {
    id: '22222222-2222-4222-8222-222222222222',
    primary: '调整账号状态',
    secondary: 'account · 管理员完成账号状态调整。',
    owner: 'admin',
    status: 'status_changed',
    risk: '2026-08-01T12:00:00Z',
    targetType: 'audit-log',
    backendKind: 'admin-audit-log',
    targetTo: '/admin/users',
    detailItems: [
      { label: '来源', value: 'admin' },
      { label: '领域', value: 'account' },
      { label: '动作', value: 'user.account_status_changed' },
      { label: '对象', value: 'orbit' },
      { label: '结果', value: 'status_changed' },
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

test('admin overview and dedicated log page share the unified server audit source', () => {
  assert.match(apiSource, /section === 'logs'[\s\S]*backendAdminAuditLogRows\(\)/)
  assert.match(adminPageSource, /useAdminSectionRows\('logs'\)/)
  assert.match(routerSource, /path: '\/admin\/logs'[\s\S]*component: AdminAuditLogsPage[\s\S]*meta: adminAuthMeta/)
  assert.match(adminAuditPageSource, /useAdminAuditLogsPage\(filters, pageRequest\)/)
  assert.match(adminAuditPageSource, /CursorTablePagination/)
  assert.doesNotMatch(adminAuditPageSource, /beforeStatus|afterStatus|reason/)
})
