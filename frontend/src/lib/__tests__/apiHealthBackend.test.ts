import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, describe, expect, it, vi } from 'vitest'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json', ETag: '"8"' },
  })
}

function ownerSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-owner',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'owner-1',
      username: 'owner',
      displayName: 'Owner',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: false },
    },
  }
}

function connection(overrides: Record<string, unknown> = {}) {
  return {
    id: 'connection-1',
    name: '主 Sub2API',
    baseUrl: 'https://api.example.test/v1',
    normalizedBaseUrl: 'https://api.example.test/v1',
    credentialConfigured: true,
    enabled: true,
    verificationStatus: 'verified',
    verifiedAt: '2026-08-08T00:00:00Z',
    lastVerificationErrorCode: null,
    measurementVersion: 1,
    version: 8,
    referencedServices: [{ id: 'service-1', title: '示例 API' }],
    healthSummary: {
      state: 'no_sample',
      availabilityReason: 'insufficient',
      successRatePercent: null,
      successfulSamples: 0,
      totalSamples: 0,
      transportSecurity: 'secure_https',
      lastSampledAt: null,
      samples: [],
    },
    createdAt: '2026-08-08T00:00:00Z',
    updatedAt: '2026-08-08T01:00:00Z',
    ...overrides,
  }
}

async function loadBackend(fetchMock: ReturnType<typeof vi.fn>) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })
  const backend = await import('../apiHealthBackend')
  return { backend, client }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('共享探针连接真实请求', () => {
  it('快捷重新启用先预检已存凭据，再携带一次性 token 更新', () => {
    const page = readFileSync(new URL('../../pages/MyApiProbeConnectionsPage.vue', import.meta.url), 'utf8')
    const toggleFlow = page.slice(page.indexOf('async function setEnabled'), page.indexOf('async function removeConnection'))
    expect(toggleFlow).toContain('await preflightMutation.mutateAsync')
    expect(toggleFlow).toContain('preflightToken = verification.preflightToken')
    expect(toggleFlow).toContain('preflightToken,')
  })

  it('预检读取模型并固定协议，不使用创建幂等键', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(jsonResponse({
        errorCode: null,
        availableModels: ['gpt-5.6-luna', 'gpt-5.6-sol'],
        probeModel: 'gpt-5.6-luna',
        probeProtocol: 'openai_responses_v1',
        probeEnvironment: 'us-west-v1',
        dailyBaseCostUpperBoundUsd: '0.0123000000',
        priceUnavailable: false,
        preflightToken: 'preflight-token',
      }))
    const { backend } = await loadBackend(fetchMock)

    const result = await backend.backendPreflightOwnerAPIProbeConnection({
      name: '主 Sub2API',
      baseUrl: 'https://api.example.test/v1',
      credential: 'probe-key-once',
      probeModel: 'gpt-5.6-luna',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/owner/api-probe-connections/preflight')
    expect(init.method).toBe('POST')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-owner')
    expect(headers.get('Idempotency-Key')).toBeNull()
    expect(result.probeProtocol).toBe('openai_responses_v1')
    expect(result.availableModels).toEqual(['gpt-5.6-luna', 'gpt-5.6-sol'])
    expect(result.preflightToken).toBe('preflight-token')
  })

  it('创建连接携带 CSRF 和幂等键，Key 只出现在当前请求体', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(jsonResponse(connection(), 201))
    const { backend } = await loadBackend(fetchMock)

    const saved = await backend.backendCreateOwnerAPIProbeConnection({
      name: ' 主 Sub2API ',
      baseUrl: ' https://api.example.test/v1 ',
      credential: ' probe-key-once ',
      probeModel: 'gpt-5.6-luna',
      preflightToken: 'preflight-token',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/owner/api-probe-connections')
    expect(init.method).toBe('POST')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-owner')
    expect(headers.get('Idempotency-Key')).toMatch(/^api-probe-connection-create-/)
    expect(JSON.parse(String(init.body))).toEqual({
      name: '主 Sub2API',
      baseUrl: 'https://api.example.test/v1',
      credential: 'probe-key-once',
      probeModel: 'gpt-5.6-luna',
      preflightToken: 'preflight-token',
      enabled: true,
      acknowledgeInsecureHttp: false,
    })
    expect(JSON.stringify(saved)).not.toContain('probe-key-once')
  })

  it('更新连接留空 Key 时不发送 credential，并携带 If-Match', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(jsonResponse(connection({ version: 9 })))
    const { backend } = await loadBackend(fetchMock)

    await backend.backendUpdateOwnerAPIProbeConnection({
      id: 'connection-1',
      version: 8,
      name: '主 Sub2API',
      baseUrl: 'https://api.example.test/v1',
      credential: '   ',
      enabled: false,
      acknowledgeInsecureHttp: false,
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('https://api.example.test/api/v1/owner/api-probe-connections/connection-1')
    expect(init.method).toBe('PUT')
    expect(headers.get('If-Match')).toBe('"8"')
    expect(JSON.parse(String(init.body))).not.toHaveProperty('credential')
  })

  it('重新验证使用版本与幂等键，删除使用 DELETE', async () => {
    const verifyFetch = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(jsonResponse(connection({ version: 9 })))
    const { backend } = await loadBackend(verifyFetch)
    await backend.backendVerifyOwnerAPIProbeConnection({ id: 'connection-1', version: 8 })

    const verifyInit = verifyFetch.mock.calls[1]?.[1] as RequestInit
    expect(verifyFetch.mock.calls[1]?.[0]).toBe('https://api.example.test/api/v1/owner/api-probe-connections/connection-1/verify')
    expect(new Headers(verifyInit.headers).get('If-Match')).toBe('"8"')
    expect(new Headers(verifyInit.headers).get('Idempotency-Key')).toMatch(/^api-probe-connection-verify-/)

    const deleteFetch = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    const loaded = await loadBackend(deleteFetch)
    await loaded.backend.backendDeleteOwnerAPIProbeConnection({ id: 'connection-1', version: 9 })
    expect((deleteFetch.mock.calls[1]?.[1] as RequestInit).method).toBe('DELETE')
  })

  it('列表补齐引用数组并把未知上游错误收敛为 internal', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(ownerSession()))
      .mockResolvedValueOnce(jsonResponse({ items: [connection({ referencedServices: undefined, lastVerificationErrorCode: 'raw-provider-message' })] }))
    const { backend } = await loadBackend(fetchMock)
    const result = await backend.backendOwnerAPIProbeConnections()
    expect(result[0]?.referencedServices).toEqual([])
    expect(result[0]?.lastVerificationErrorCode).toBe('internal')
  })
})

describe('共享探针连接 Mock facade', () => {
  it('保留卖家输入的完整 Base URL，不自动补 /v1，也不保存明文 Key', async () => {
    vi.resetModules()
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiHealthFacade')
    facade.resetMockAPIProbeConnections()

    const createInput = {
      name: '低额度探针',
      baseUrl: 'https://api.example.test',
      credential: 'probe-key-once',
      probeModel: 'gpt-5.6-luna',
      enabled: true,
      acknowledgeInsecureHttp: false,
    }
    const preflight = await facade.preflightOwnerAPIProbeConnection(createInput)
    const saved = await facade.createOwnerAPIProbeConnection({ ...createInput, preflightToken: preflight.preflightToken ?? undefined })
    const loaded = await facade.getOwnerAPIProbeConnection(saved.id)

    expect(saved.baseUrl).toBe('https://api.example.test')
    expect(saved.normalizedBaseUrl).toBe('https://api.example.test')
    expect(saved.verificationStatus).toBe('verified')
    expect(JSON.stringify(loaded)).not.toContain('probe-key-once')
    facade.resetMockAPIProbeConnections()
  })

  it('HTTP 必须显式确认，仍被服务引用的连接不能删除', async () => {
    vi.resetModules()
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiHealthFacade')
    facade.resetMockAPIProbeConnections()

    const input = {
      name: 'HTTP 探针',
      baseUrl: 'http://api.example.test:31238',
      credential: 'probe-key-once',
      enabled: true,
      acknowledgeInsecureHttp: false,
    }
    await expect(facade.createOwnerAPIProbeConnection(input)).rejects.toThrow('HTTP 未加密传输风险')
    const acknowledged = { ...input, probeModel: 'gpt-5.6-luna', acknowledgeInsecureHttp: true }
    const preflight = await facade.preflightOwnerAPIProbeConnection(acknowledged)
    const saved = await facade.createOwnerAPIProbeConnection({ ...acknowledged, preflightToken: preflight.preflightToken ?? undefined })
    facade.updateMockAPIProbeConnectionReference({ connectionId: saved.id, serviceId: 'service-1', serviceTitle: '示例 API' })
    await assert.rejects(() => facade.deleteOwnerAPIProbeConnection({ id: saved.id, version: saved.version }), /仍被 API 服务引用/)
    facade.resetMockAPIProbeConnections()
  })
})
