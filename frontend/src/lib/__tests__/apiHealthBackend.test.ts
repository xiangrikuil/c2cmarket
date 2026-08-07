import assert from 'node:assert/strict'
import { afterEach, describe, expect, it, vi } from 'vitest'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json', ETag: '"8"' },
  })
}

function adminSession(admin = false) {
  return {
    csrfToken: admin ? 'csrf-admin' : 'csrf-owner',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: admin ? 'admin-1' : 'owner-1',
      username: admin ? 'admin' : 'owner',
      displayName: admin ? 'Admin' : 'Owner',
      isAdmin: admin,
      permissions: admin ? ['admin'] : [],
      linuxDoBinding: { bound: false },
    },
  }
}

function ownerProbe(overrides: Record<string, unknown> = {}) {
  return {
    id: 'probe-1',
    apiServiceId: 'service-1',
    protocol: 'openai_chat_completions_v1',
    baseUrl: 'https://api.example.test/v1',
    normalizedOrigin: 'https://api.example.test:443',
    model: 'gpt-5-mini',
    credentialConfigured: true,
    enabled: true,
    authorizationStatus: 'pending',
    authorizationMethod: null,
    verifiedOrigin: null,
    verifiedAt: null,
    approvedAt: null,
    rejectionReason: null,
    challengeExpiresAt: null,
    measurementVersion: 1,
    lastConfigErrorCode: null,
    version: 8,
    createdAt: '2026-08-04T00:00:00Z',
    updatedAt: '2026-08-04T01:00:00Z',
    ...overrides,
  }
}

async function loadBackend(fetchMock: ReturnType<typeof vi.fn>, admin = false) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })
  const sessionFetch = fetchMock.mock.calls.length === 0
  if (sessionFetch) fetchMock.mockResolvedValueOnce(jsonResponse(adminSession(admin)))
  const backend = await import('../apiHealthBackend')
  return { backend, client }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('API 健康探针真实请求', () => {
  it('Owner 保存使用 CSRF 与 If-Match，留空 credential 时不发送该字段', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(jsonResponse(ownerProbe({ version: 9 })))
    const { backend } = await loadBackend(fetchMock)

    await backend.backendSaveOwnerAPIHealthProbe({
      apiServiceId: 'service-1',
      version: 8,
      baseUrl: ' https://api.example.test/v1 ',
      model: ' gpt-5-mini ',
      credential: '   ',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/owner/api-services/service-1/health-probe')
    expect(init.method).toBe('PUT')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-owner')
    expect(headers.get('If-Match')).toBe('"8"')
    expect(JSON.parse(String(init.body))).toEqual({
      baseUrl: 'https://api.example.test/v1',
      model: 'gpt-5-mini',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
  })

  it('Owner 轮换 credential 只在当前 PUT 请求中发送', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(jsonResponse(ownerProbe({ version: 9 })))
    const { backend } = await loadBackend(fetchMock)

    const saved = await backend.backendSaveOwnerAPIHealthProbe({
      apiServiceId: 'service-1',
      version: 8,
      baseUrl: 'https://api.example.test/v1',
      model: 'gpt-5-mini',
      credential: 'probe-key-once',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })

    const body = JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))
    expect(body.credential).toBe('probe-key-once')
    expect(saved).not.toHaveProperty('credential')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('Admin 审批只提交理由并携带 CSRF、幂等键和 If-Match', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession(true)))
      .mockResolvedValueOnce(jsonResponse({
        ...ownerProbe({ authorizationStatus: 'approved', version: 9 }),
        ownerUserId: 'owner-1',
        serviceTitle: 'Example API',
        ownerDisplayName: 'Owner',
        ownerUsername: 'owner',
      }))
    const { backend } = await loadBackend(fetchMock, true)

    await backend.backendReviewAPIHealthProbe({
      id: 'probe-1',
      version: 8,
      decision: 'approve',
      reason: '  已核对精确域名归属  ',
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/admin/api-service-health-probes/probe-1/approve')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-admin')
    expect(headers.get('If-Match')).toBe('"8"')
    expect(headers.get('Idempotency-Key')).toMatch(/^api-health-approve-/)
    expect(JSON.parse(String(init.body))).toEqual({ reason: '已核对精确域名归属' })
  })

  it('Admin 列表只映射页面所需的精确 Origin 信息', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession(true)))
      .mockResolvedValueOnce(jsonResponse({
        items: [{
          id: 'probe-1',
          apiServiceId: 'service-1',
          serviceTitle: 'Example API',
          ownerUserId: 'owner-1',
          ownerUsername: 'owner',
          ownerDisplayName: 'Owner',
          protocol: 'openai_chat_completions_v1',
          normalizedOrigin: 'https://api.example.test:443',
          model: 'private-review-model',
          enabled: true,
          authorizationStatus: 'pending',
          authorizationMethod: null,
          verifiedOrigin: null,
          verifiedAt: null,
          approvedAt: null,
          rejectionReason: null,
          version: 8,
          updatedAt: '2026-08-04T01:00:00Z',
        }],
        nextCursor: null,
      }))
    const { backend } = await loadBackend(fetchMock, true)

    const result = await backend.backendAdminAPIHealthProbes('pending')

    expect(fetchMock.mock.calls[1]?.[0]).toBe('https://api.example.test/api/v1/admin/api-service-health-probes?status=pending&limit=100')
    expect(result.items[0]).toEqual({
      id: 'probe-1',
      apiServiceId: 'service-1',
      serviceTitle: 'Example API',
      ownerUserId: 'owner-1',
      ownerDisplayName: 'Owner',
      ownerUsername: 'owner',
      normalizedOrigin: 'https://api.example.test:443',
      authorizationStatus: 'pending',
      version: 8,
      updatedAt: '2026-08-04T01:00:00Z',
    })
    expect(result.items[0]).not.toHaveProperty('model')
    expect(result.items[0]).not.toHaveProperty('credentialConfigured')
  })

  it('Owner GET 的 404 映射为未配置，其余错误继续抛出', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: 404, code: 'OBJECT_NOT_FOUND' }), {
        status: 404,
        headers: { 'content-type': 'application/problem+json' },
      }))
    const { backend } = await loadBackend(fetchMock)
    await expect(backend.backendOwnerAPIHealthProbe('service-1')).resolves.toBeNull()

    const failingFetch = vi.fn()
      .mockResolvedValueOnce(jsonResponse(adminSession()))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: 503, code: 'BACKEND_UNAVAILABLE' }), {
        status: 503,
        headers: { 'content-type': 'application/problem+json' },
      }))
    const loaded = await loadBackend(failingFetch)
    await assert.rejects(() => loaded.backend.backendOwnerAPIHealthProbe('service-1'))
  })
})

describe('API 健康探针 Mock facade', () => {
  it('只保存 credentialConfigured，不持久化 credential 内容', async () => {
    vi.resetModules()
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiHealthFacade')
    facade.resetMockAPIHealthProbes()

    const saved = await facade.saveOwnerAPIHealthProbe({
      apiServiceId: 'service-1',
      version: 0,
      baseUrl: 'https://api.example.test',
      model: 'gpt-5-mini',
      credential: 'probe-key-once',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    const loaded = await facade.getOwnerAPIHealthProbe('service-1')

    expect(saved.credentialConfigured).toBe(true)
    expect(saved.baseUrl).toBe('https://api.example.test/v1')
    expect(saved.baseUrl).not.toContain('/v1/v1')
    expect(JSON.stringify(saved)).not.toContain('probe-key-once')
    expect(JSON.stringify(loaded)).not.toContain('probe-key-once')
    facade.resetMockAPIHealthProbes()
  })

  it('HTTP Mock 配置要求确认风险并使用 80 端口 Origin', async () => {
    vi.resetModules()
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiHealthFacade')
    facade.resetMockAPIHealthProbes()

    const input = {
      apiServiceId: 'service-http',
      version: 0,
      baseUrl: 'http://api.example.test',
      model: 'gpt-5-mini',
      credential: 'probe-key-once',
      enabled: true,
      acknowledgeInsecureHttp: false,
    }
    await expect(facade.saveOwnerAPIHealthProbe(input)).rejects.toThrow('确认未加密传输风险')

    const saved = await facade.saveOwnerAPIHealthProbe({ ...input, acknowledgeInsecureHttp: true })
    expect(saved.baseUrl).toBe('http://api.example.test/v1')
    expect(saved.normalizedOrigin).toBe('http://api.example.test:80')
    facade.resetMockAPIHealthProbes()
  })

  it('Mock 仅在 Origin 变化时重置授权，模型和路径变化只更新测量版本', async () => {
    vi.resetModules()
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiHealthFacade')
    facade.resetMockAPIHealthProbes()

    let config = await facade.saveOwnerAPIHealthProbe({
      apiServiceId: 'service-auth',
      version: 0,
      baseUrl: 'https://api.example.test/v1',
      model: 'gpt-5-mini',
      credential: 'probe-key-once',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    const challenge = await facade.createAPIHealthChallenge({
      apiServiceId: 'service-auth',
      version: config.version,
      method: 'dns_txt',
    })
    config = await facade.verifyAPIHealthChallenge({ apiServiceId: 'service-auth', version: challenge.configVersion })

    config = await facade.saveOwnerAPIHealthProbe({
      apiServiceId: 'service-auth',
      version: config.version,
      baseUrl: config.baseUrl,
      model: 'gpt-5.1',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    expect(config.authorizationStatus).toBe('verified')
    expect(config.measurementVersion).toBe(2)

    config = await facade.saveOwnerAPIHealthProbe({
      apiServiceId: 'service-auth',
      version: config.version,
      baseUrl: 'https://api.example.test/openai/v1',
      model: config.model,
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    expect(config.authorizationStatus).toBe('verified')
    expect(config.measurementVersion).toBe(3)

    config = await facade.saveOwnerAPIHealthProbe({
      apiServiceId: 'service-auth',
      version: config.version,
      baseUrl: 'https://other.example.test/v1',
      model: config.model,
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    expect(config.authorizationStatus).toBe('pending')
    expect(config.measurementVersion).toBe(4)
    facade.resetMockAPIHealthProbes()
  })
})
