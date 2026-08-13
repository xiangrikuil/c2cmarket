import assert from 'node:assert/strict'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  adminUserDirectoryRouteQuery,
  normalizeAdminUserDirectoryQuery,
  serializeAdminUserDirectoryQuery,
} from '@/lib/adminUserBackend'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function problemResponse(body: unknown, status: number) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/problem+json' },
  })
}

function adminSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-admin',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'admin-1',
      username: 'admin',
      displayName: 'Admin',
      isAdmin: true,
      permissions: ['admin'],
      capabilities: ['admin.access'],
      linuxDoBinding: { bound: false },
    },
  }
}

async function loadRealAdminUserBackend(fetchMock: ReturnType<typeof vi.fn>) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })
  const backend = await import('../adminUserBackend')
  return { backend, client }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('管理员用户目录查询参数', () => {
  it('规范化 URL 参数并丢弃不受支持的值', () => {
    const query = normalizeAdminUserDirectoryQuery({
      page: '3',
      limit: '50',
      search: '  alice  ',
      status: 'suspended',
      role: 'admin',
      linuxDo: 'bound',
      sort: 'username_asc',
    })

    expect(query).toEqual({
      page: 3,
      limit: 50,
      search: 'alice',
      status: 'suspended',
      role: 'admin',
      linuxDo: 'bound',
      sort: 'username_asc',
    })
    expect(normalizeAdminUserDirectoryQuery({ page: '-1', limit: '200', status: 'unknown' })).toMatchObject({
      page: 1,
      limit: 20,
      status: 'all',
    })
  })

  it('生成可复现的路由状态和服务端分页查询', () => {
    const query = normalizeAdminUserDirectoryQuery({ page: '2', search: 'alice', role: 'user' })
    expect(adminUserDirectoryRouteQuery(query)).toEqual({ search: 'alice', role: 'user', page: '2' })
    expect(serializeAdminUserDirectoryQuery(query)).toBe('page=2&limit=20&status=all&role=user&linuxDo=all&sort=created_desc&search=alice')
  })
})

describe('管理员用户目录真实请求', () => {
  it('真实 API 失败时原样抛错，不回退到 mock 用户', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(problemResponse({
        status: 503,
        code: 'BACKEND_UNAVAILABLE',
        detail: '用户目录暂时不可用。',
      }, 503))
    const { backend, client } = await loadRealAdminUserBackend(fetchMock)

    await assert.rejects(
      () => backend.backendAdminUserDirectory(backend.defaultAdminUserDirectoryQuery),
      (error: unknown) => error instanceof client.BackendProblemError && error.code === 'BACKEND_UNAVAILABLE',
    )
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[1]?.[0]).toBe('https://api.example.test/api/v1/admin/users?page=1&limit=20&status=all&role=all&linuxDo=all&sort=created_desc')
  })

  it('状态治理请求携带 CSRF、幂等键和 If-Match', async () => {
    const detail = {
      user: {
        id: 'user-1', username: 'alice', displayName: 'Alice', accountStatus: 'suspended', isAdmin: false,
        linuxDoBound: false, createdAt: '2026-08-01T00:00:00Z', version: 8,
      },
      updatedAt: '2026-08-01T01:00:00Z',
      linuxDoBinding: { bound: false },
      emailVerified: false,
      backupPasswordConfigured: false,
      providers: [],
      sessions: { activeCount: 0 },
      recentAuditEntries: [],
    }
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(jsonResponse(detail))
    const { backend } = await loadRealAdminUserBackend(fetchMock)

    await backend.backendUpdateAdminUserStatus({
      userId: 'user-1',
      version: 7,
      status: 'suspended',
      reason: '  重复违规，暂停账号  ',
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/admin/users/user-1/status')
    expect(init.method).toBe('POST')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-admin')
    expect(headers.get('If-Match')).toBe('"7"')
    expect(headers.get('Idempotency-Key')).toMatch(/^admin-user-status-/)
    expect(JSON.parse(String(init.body))).toEqual({ status: 'suspended', reason: '重复违规，暂停账号' })
  })
})
