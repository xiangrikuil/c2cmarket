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

async function loadBackendClient(config: { apiMode?: string, apiBaseUrl?: string } = {}): Promise<BackendClientModule> {
  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig(config)
  return client
}

afterEach(() => {
  vi.unstubAllGlobals()
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

  const client = await loadBackendClient({ apiMode: 'real' })

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

test('runtime backend mode requires an explicit supported value', async () => {
  vi.resetModules()
  const client = await import('../backendClient')

  assert.throws(
    () => client.setBackendRuntimeConfig({}),
    /NUXT_PUBLIC_API_MODE must be explicitly set to "real" or "mock"/,
  )
  assert.throws(
    () => client.setBackendRuntimeConfig({ apiMode: 'development' }),
    /NUXT_PUBLIC_API_MODE must be explicitly set to "real" or "mock"/,
  )

  client.setBackendRuntimeConfig({ apiMode: 'mock' })
  assert.equal(client.shouldUseRealBackend(), false)
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  assert.equal(client.shouldUseRealBackend(), true)
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

  const client = await loadBackendClient({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })

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

  const client = await loadBackendClient({ apiMode: 'real' })
  const invalidationHandler = vi.fn()
  client.subscribeToBackendSessionInvalidation(invalidationHandler)
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
  assert.equal(invalidationHandler.mock.calls.length, 0)
})

for (const code of ['SESSION_EXPIRED', 'SESSION_REVOKED']) {
  test(`notifies session invalidation subscribers for ${code}`, async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    fetchMock
      .mockResolvedValueOnce(jsonResponse({
        csrfToken: 'active-token',
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
      .mockResolvedValueOnce(problemResponse({
        status: 401,
        code,
        detail: '请重新登录。',
      }, 401))

    const client = await loadBackendClient({ apiMode: 'real' })
    const invalidationHandler = vi.fn()
    client.subscribeToBackendSessionInvalidation(invalidationHandler)

    await client.getCurrentBackendSession()
    await assert.rejects(() => client.backendRequest('/api/v1/private'))

    assert.equal(invalidationHandler.mock.calls.length, 1)
    assert.equal(invalidationHandler.mock.calls[0]?.[0].code, code)
  })
}

test('does not notify an unsubscribed session invalidation handler', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock
    .mockResolvedValueOnce(jsonResponse({
      csrfToken: 'active-token',
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
    .mockResolvedValueOnce(problemResponse({
      status: 401,
      code: 'SESSION_EXPIRED',
      detail: '请重新登录。',
    }, 401))

  const client = await loadBackendClient({ apiMode: 'real' })
  const invalidationHandler = vi.fn()
  const unsubscribe = client.subscribeToBackendSessionInvalidation(invalidationHandler)
  await client.getCurrentBackendSession()
  unsubscribe()

  await assert.rejects(() => client.backendRequest('/api/v1/private'))

  assert.equal(invalidationHandler.mock.calls.length, 0)
})

test('does not treat an anonymous public-page session probe as session invalidation', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockResolvedValueOnce(problemResponse({
    status: 401,
    code: 'SESSION_EXPIRED',
    detail: '请先登录。',
  }, 401))

  const client = await loadBackendClient({ apiMode: 'real' })
  const invalidationHandler = vi.fn()
  client.subscribeToBackendSessionInvalidation(invalidationHandler)

  await assert.rejects(() => client.getCurrentBackendSession())

  assert.equal(invalidationHandler.mock.calls.length, 0)
})

test('real backend mode preserves network errors during session checks', async () => {
  const fetchMock = vi.fn().mockRejectedValue(new TypeError('network unavailable'))
  vi.stubGlobal('fetch', fetchMock)
  const client = await loadBackendClient({ apiMode: 'real' })

  await assert.rejects(
    () => client.ensureBackendSession('orbit'),
    error => error instanceof TypeError && error.message === 'network unavailable',
  )
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

  const client = await loadBackendClient({ apiMode: 'real' })
  assert.deepEqual(await client.getCurrentBackendSession(), firstSession)
  await client.logoutBackendSession()
  assert.deepEqual(await client.getCurrentBackendSession(), nextSession)

  assert.equal(fetchMock.mock.calls.length, 3)
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/auth/logout')
  const logoutHeaders = new Headers((fetchMock.mock.calls[1]?.[1] as RequestInit).headers)
  assert.equal(logoutHeaders.get('X-CSRF-Token'), 'csrf-before-logout')
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/auth/session')
})

test('development persona login replaces the cached session and CSRF token', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  const originalSession = {
    csrfToken: 'buyer-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'buyer-id',
      analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
      username: 'dev-buyer',
      displayName: '开发买家',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const sellerSession = {
    persona: 'seller',
    csrfToken: 'seller-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'seller-id',
      analyticsUserId: 'a2222222-2222-4222-8222-222222222222',
      username: 'dev-seller',
      displayName: '开发卖家',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  fetchMock
    .mockResolvedValueOnce(jsonResponse(originalSession))
    .mockResolvedValueOnce(jsonResponse(sellerSession))

  const client = await loadBackendClient({ apiMode: 'real' })
  await client.getCurrentBackendSession()
  const switched = await client.createDevPersonaSession('seller')

  assert.deepEqual(switched, sellerSession)
  assert.equal(client.getBackendCSRFToken(), 'seller-csrf')
  assert.deepEqual(await client.getCurrentBackendSession(), sellerSession)
  assert.equal(fetchMock.mock.calls.length, 2)
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/auth/dev-persona-session')
  assert.equal((fetchMock.mock.calls[1]?.[1] as RequestInit).body, JSON.stringify({ persona: 'seller' }))
})

test('development persona login cannot be overwritten by an older session request', async () => {
  let resolveOldSession!: (response: Response) => void
  const oldSessionResponse = new Promise<Response>((resolve) => {
    resolveOldSession = resolve
  })
  const sellerSession = {
    persona: 'seller' as const,
    csrfToken: 'seller-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'seller-id',
      analyticsUserId: 'a2222222-2222-4222-8222-222222222222',
      username: 'dev-seller',
      displayName: '开发卖家',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const oldBuyerSession = {
    ...sellerSession,
    persona: undefined,
    csrfToken: 'old-buyer-csrf',
    user: { ...sellerSession.user, id: 'buyer-id', username: 'dev-buyer', displayName: '开发买家' },
  }
  const fetchMock = vi.fn()
    .mockImplementationOnce(() => oldSessionResponse)
    .mockResolvedValueOnce(jsonResponse(sellerSession))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  const staleRead = client.getCurrentBackendSession()
  const switched = await client.createDevPersonaSession('seller')
  resolveOldSession(jsonResponse(oldBuyerSession))

  assert.deepEqual(await staleRead, sellerSession)
  assert.deepEqual(switched, sellerSession)
  assert.equal(client.getBackendCSRFToken(), 'seller-csrf')
  assert.deepEqual(await client.getCurrentBackendSession(), sellerSession)
  assert.equal(fetchMock.mock.calls.length, 2)
})

test('an older authenticated failure cannot clear a replacement persona session', async () => {
  let resolveOldRequest!: (response: Response) => void
  const oldRequestResponse = new Promise<Response>((resolve) => {
    resolveOldRequest = resolve
  })
  const sellerSession = {
    persona: 'seller' as const,
    csrfToken: 'seller-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'seller-id',
      analyticsUserId: 'a2222222-2222-4222-8222-222222222222',
      username: 'dev-seller',
      displayName: '开发卖家',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const fetchMock = vi.fn()
    .mockImplementationOnce(() => oldRequestResponse)
    .mockResolvedValueOnce(jsonResponse(sellerSession))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  const oldRequest = client.backendRequest('/api/v1/private')
  await client.createDevPersonaSession('seller')
  resolveOldRequest(problemResponse({
    status: 401,
    code: 'SESSION_REVOKED',
    detail: '旧会话已失效。',
  }, 401))

  await assert.rejects(oldRequest)
  assert.equal(client.getBackendCSRFToken(), 'seller-csrf')
  assert.deepEqual(await client.getCurrentBackendSession(), sellerSession)
  assert.equal(fetchMock.mock.calls.length, 2)
})

test('a replacement persona does not coalesce GETs with the previous session generation', async () => {
  let resolveOldProfile!: (response: Response) => void
  const oldProfileResponse = new Promise<Response>((resolve) => {
    resolveOldProfile = resolve
  })
  const sellerSession = {
    persona: 'seller' as const,
    csrfToken: 'seller-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'seller-id',
      analyticsUserId: 'a2222222-2222-4222-8222-222222222222',
      username: 'dev-seller',
      displayName: '开发卖家',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const newProfile = { id: 'seller-id', username: 'dev-seller', displayName: '开发卖家' }
  const oldProfile = { id: 'buyer-id', username: 'dev-buyer', displayName: '开发买家' }
  const fetchMock = vi.fn()
    .mockImplementationOnce(() => oldProfileResponse)
    .mockResolvedValueOnce(jsonResponse(sellerSession))
    .mockResolvedValueOnce(jsonResponse(newProfile))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  const previousProfileRead = client.backendRequest('/api/v1/me/profile')
  await client.createDevPersonaSession('seller')
  const replacementProfile = await client.backendRequest('/api/v1/me/profile')
  resolveOldProfile(jsonResponse(oldProfile))

  assert.deepEqual(replacementProfile, newProfile)
  assert.deepEqual(await previousProfileRead, oldProfile)
  assert.equal(fetchMock.mock.calls.length, 3)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/me/profile')
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/me/profile')
})

test('development persona login has no mock-mode fallback', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  const client = await loadBackendClient({ apiMode: 'mock' })

  await assert.rejects(
    () => client.createDevPersonaSession('buyer'),
    /requires real API mode/,
  )
  assert.equal(fetchMock.mock.calls.length, 0)
})

test('OAuth start sends only the stored bounded registration attribution', async () => {
  const values = new Map<string, string>()
  const sessionStorage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
  vi.stubGlobal('window', {
    sessionStorage,
    location: {
      origin: 'https://c2cmarket.shop',
      pathname: '/carpools/private-listing-id',
      search: '?utm_source=linux.do&utm_medium=community&utm_campaign=launch',
    },
  })
  vi.stubGlobal('document', { referrer: 'https://linux.do/t/private-topic/123?token=secret' })
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
    authorizationUrl: 'https://connect.linux.do/oauth2/authorize',
  }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real', apiBaseUrl: 'https://api.example.test' })
  await client.startOAuthLogin('/my/rides/private-application-id?tab=contact')

  const requestURL = new URL(fetchMock.mock.calls[0]?.[0] as string)
  assert.equal(requestURL.pathname, '/api/v1/auth/oauth/start')
  assert.equal(requestURL.searchParams.get('returnTo'), '/my/rides/private-application-id?tab=contact')
  assert.equal(requestURL.searchParams.get('utmSource'), 'linux.do')
  assert.equal(requestURL.searchParams.get('utmMedium'), 'community')
  assert.equal(requestURL.searchParams.get('utmCampaign'), 'launch')
  assert.equal(requestURL.searchParams.get('referrerHost'), 'linux.do')
  assert.equal(requestURL.searchParams.get('landingPath'), '/carpools/:id')
  assert.equal(requestURL.search.includes('private-topic'), false)
  assert.equal(requestURL.search.includes('token'), false)
})

test('restricted-account appeals use dedicated CSRF and preserve the normal session cache', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock
    .mockResolvedValueOnce(jsonResponse({
      csrfToken: 'normal-csrf',
      expiresAt: '2999-01-01T00:00:00Z',
      user: {
        id: 'user-1',
        analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
        username: 'orbit',
        displayName: 'Orbit',
        isAdmin: false,
        permissions: [],
        linuxDoBinding: { bound: true },
      },
    }))
    .mockResolvedValueOnce(jsonResponse({
      accountStatus: 'suspended',
      csrfToken: 'appeal-csrf',
      expiresAt: '2026-08-03T10:15:00Z',
    }))
    .mockResolvedValueOnce(jsonResponse({
      id: 'appeal-1',
      targetType: 'account_governance',
      targetId: 'user-1',
      title: '账号治理申诉',
      status: 'submitted',
      createdAt: '2026-08-03T10:01:00Z',
      updatedAt: '2026-08-03T10:01:00Z',
      version: 1,
    }, 201))
    .mockResolvedValueOnce(jsonResponse({ ok: true }))

  const client = await loadBackendClient({ apiMode: 'real' })
  await client.getCurrentBackendSession()
  const appealSession = await client.getAccountAppealSession()
  assert.equal(appealSession.accountStatus, 'suspended')
  assert.equal(client.getBackendCSRFToken(), 'normal-csrf')

  const appeal = await client.submitAccountGovernanceAppeal('请复核本次账号限制。')
  assert.equal(appeal.targetType, 'account_governance')
  await client.backendMutation('/api/v1/example', {})

  const appealHeaders = new Headers((fetchMock.mock.calls[2]?.[1] as RequestInit).headers)
  assert.equal(fetchMock.mock.calls[2]?.[0], '/api/v1/account-appeal/appeals')
  assert.equal(appealHeaders.get('X-Account-Appeal-CSRF'), 'appeal-csrf')
  assert.equal(appealHeaders.get('X-CSRF-Token'), null)
  assert.match(appealHeaders.get('Idempotency-Key') ?? '', /^account-appeal-create-/)
  assert.equal((fetchMock.mock.calls[2]?.[1] as RequestInit).body, JSON.stringify({ statement: '请复核本次账号限制。' }))

  const normalHeaders = new Headers((fetchMock.mock.calls[3]?.[1] as RequestInit).headers)
  assert.equal(normalHeaders.get('X-CSRF-Token'), 'normal-csrf')
  assert.equal(normalHeaders.get('X-Account-Appeal-CSRF'), null)
})

test('dedicated appeal-session errors do not invalidate the normal cached session', async () => {
  const normalSession = {
    csrfToken: 'normal-csrf',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'user-1',
      analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
      username: 'orbit',
      displayName: 'Orbit',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(normalSession))
    .mockResolvedValueOnce(problemResponse({
      status: 401,
      code: 'SESSION_REVOKED',
      detail: '专用申诉会话已失效。',
    }, 401))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  await client.getCurrentBackendSession()
  await assert.rejects(() => client.getAccountAppealSession())
  assert.deepEqual(await client.getCurrentBackendSession(), normalSession)
  assert.equal(fetchMock.mock.calls.length, 2)
  assert.equal(client.getBackendCSRFToken(), 'normal-csrf')
})

test('restricted-account appeal submission refreshes only its dedicated CSRF and reuses idempotency', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({
      accountStatus: 'banned',
      csrfToken: 'stale-appeal-csrf',
      expiresAt: '2026-08-03T10:15:00Z',
    }))
    .mockResolvedValueOnce(problemResponse({
      status: 403,
      code: 'CSRF_TOKEN_INVALID',
      detail: 'CSRF token 无效或缺失。',
    }, 403))
    .mockResolvedValueOnce(jsonResponse({
      accountStatus: 'banned',
      csrfToken: 'fresh-appeal-csrf',
      expiresAt: '2026-08-03T10:15:00Z',
    }))
    .mockResolvedValueOnce(jsonResponse({
      id: 'appeal-1',
      targetType: 'account_governance',
      targetId: 'user-1',
      title: '账号治理申诉',
      status: 'submitted',
      createdAt: '2026-08-03T10:01:00Z',
      updatedAt: '2026-08-03T10:01:00Z',
      version: 1,
    }, 201))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  await client.getAccountAppealSession()
  await client.submitAccountGovernanceAppeal('请复核本次账号限制。')

  assert.deepEqual(fetchMock.mock.calls.map(call => call[0]), [
    '/api/v1/account-appeal/session',
    '/api/v1/account-appeal/appeals',
    '/api/v1/account-appeal/session',
    '/api/v1/account-appeal/appeals',
  ])
  const firstHeaders = new Headers((fetchMock.mock.calls[1]?.[1] as RequestInit).headers)
  const retryHeaders = new Headers((fetchMock.mock.calls[3]?.[1] as RequestInit).headers)
  assert.equal(firstHeaders.get('X-Account-Appeal-CSRF'), 'stale-appeal-csrf')
  assert.equal(retryHeaders.get('X-Account-Appeal-CSRF'), 'fresh-appeal-csrf')
  assert.equal(firstHeaders.get('Idempotency-Key'), retryHeaders.get('Idempotency-Key'))
})

test('restricted-account appeal verification starts only the dedicated OAuth flow', async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse({
    authorizationUrl: 'https://connect.linux.do/oauth2/authorize?state=appeal',
  }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  const response = await client.startAccountAppealVerification()

  assert.equal(response.authorizationUrl.includes('state=appeal'), true)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/account-appeal/start')
})

test('cached sessions identify with the opaque analytics ID and logout clears it', async () => {
  const identify = vi.fn()
  vi.stubGlobal('window', { umami: { identify } })
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({
      csrfToken: 'csrf-token',
      expiresAt: '2999-01-01T00:00:00Z',
      user: {
        id: 'business-user-id',
        analyticsUserId: 'a1111111-1111-4111-8111-111111111111',
        username: 'private-username',
        displayName: 'Private User',
        isAdmin: false,
        permissions: [],
        linuxDoBinding: { bound: true },
      },
    }))
    .mockResolvedValueOnce(new Response(null, { status: 204 }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await loadBackendClient({ apiMode: 'real' })
  const analytics = await import('../analytics')
  analytics.setAnalyticsRuntimeConfig({ enabled: true })

  await client.getCurrentBackendSession()
  await client.logoutBackendSession()

  assert.deepEqual(identify.mock.calls, [
    ['a1111111-1111-4111-8111-111111111111'],
    [''],
  ])
  assert.equal(JSON.stringify(identify.mock.calls).includes('business-user-id'), false)
  assert.equal(JSON.stringify(identify.mock.calls).includes('private-username'), false)
})
