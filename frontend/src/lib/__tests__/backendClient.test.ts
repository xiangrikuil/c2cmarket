import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type BackendClientModule = typeof import('../backendClient')

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

async function loadBackendClient(env: Record<string, string> = {}): Promise<BackendClientModule> {
  vi.resetModules()
  vi.unstubAllEnvs()
  for (const [key, value] of Object.entries(env)) {
    vi.stubEnv(key, value)
  }
  return import('../backendClient')
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('real backend mode surfaces expired sessions without dev-session fallback', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockResolvedValueOnce(problemResponse({
    status: 401,
    code: 'SESSION_EXPIRED',
    detail: '请先登录后继续操作。',
  }, 401))

  const client = await loadBackendClient({ VITE_API_MODE: 'real' })

  await assert.rejects(
    () => client.ensureBackendSession('orbit'),
    (error: unknown) => {
      assert.equal(error instanceof client.BackendProblemError, true)
      assert.equal((error as InstanceType<typeof client.BackendProblemError>).code, 'SESSION_EXPIRED')
      return true
    },
  )

  assert.equal(fetchMock.mock.calls.length, 1)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/session')
})

test('decodes Problem Details into BackendProblemError', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockResolvedValueOnce(problemResponse({
    status: 422,
    code: 'VALIDATION_FAILED',
    detail: '字段不符合要求。',
    errors: [{ field: 'q', code: 'too_long', message: '搜索关键词不能超过 80 个字符。' }],
    requestId: 'req_test',
  }, 422))

  const client = await loadBackendClient({ VITE_API_BASE_URL: 'https://api.example.test/' })

  await assert.rejects(
    () => client.backendRequest('/api/v1/search?q=x'),
    (error: unknown) => {
      assert.equal(error instanceof client.BackendProblemError, true)
      const problem = error as InstanceType<typeof client.BackendProblemError>
      assert.equal(problem.status, 422)
      assert.equal(problem.code, 'VALIDATION_FAILED')
      assert.equal(problem.detail, '字段不符合要求。')
      assert.deepEqual(problem.fieldErrors, [
        { field: 'q', code: 'too_long', message: '搜索关键词不能超过 80 个字符。' },
      ])
      return true
    },
  )

  assert.equal(fetchMock.mock.calls[0]?.[0], 'https://api.example.test/api/v1/search?q=x')
})

test('refreshes session and retries mutation after stale CSRF token', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock
    .mockResolvedValueOnce(problemResponse({
      status: 403,
      code: 'CSRF_TOKEN_INVALID',
      detail: 'CSRF token invalid.',
    }, 403))
    .mockResolvedValueOnce(jsonResponse({
      csrfToken: 'fresh-token',
      expiresAt: '2999-01-01T00:00:00Z',
      user: {
        id: 'user-1',
        username: 'orbit',
        displayName: 'Orbit',
        isAdmin: false,
        permissions: [],
        linuxDoBinding: { bound: false },
      },
    }))
    .mockResolvedValueOnce(jsonResponse({ ok: true }))

  const client = await loadBackendClient({ VITE_API_MODE: 'real' })
  client.setBackendCSRFToken('stale-token')

  const result = await client.backendMutation<{ ok: boolean }>('/api/v1/example', { name: 'demo' })

  assert.deepEqual(result, { ok: true })
  assert.equal(fetchMock.mock.calls.length, 3)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/example')
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/auth/session')
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/example')

  const firstMutationHeaders = new Headers((fetchMock.mock.calls[0]?.[1] as RequestInit).headers)
  const retryMutationHeaders = new Headers((fetchMock.mock.calls[2]?.[1] as RequestInit).headers)
  assert.equal(firstMutationHeaders.get('X-CSRF-Token'), 'stale-token')
  assert.equal(retryMutationHeaders.get('X-CSRF-Token'), 'fresh-token')
})

test('logout revokes the backend session and clears the cached session', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  const firstSession = {
    csrfToken: 'csrf-before-logout',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'user-1',
      username: 'orbit',
      displayName: 'Orbit',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const nextSession = {
    ...firstSession,
    csrfToken: 'csrf-after-login',
  }
  fetchMock
    .mockResolvedValueOnce(jsonResponse(firstSession))
    .mockResolvedValueOnce(new Response(null, { status: 204 }))
    .mockResolvedValueOnce(jsonResponse(nextSession))

  const client = await loadBackendClient({ VITE_API_MODE: 'real' })
  assert.deepEqual(await client.getCurrentBackendSession(), firstSession)
  await client.logoutBackendSession()
  assert.deepEqual(await client.getCurrentBackendSession(), nextSession)

  assert.equal(fetchMock.mock.calls.length, 3)
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/auth/logout')
  const logoutHeaders = new Headers((fetchMock.mock.calls[1]?.[1] as RequestInit).headers)
  assert.equal(logoutHeaders.get('X-CSRF-Token'), 'csrf-before-logout')
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/auth/session')
})
