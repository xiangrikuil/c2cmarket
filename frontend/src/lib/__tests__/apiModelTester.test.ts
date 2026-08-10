import { readFileSync } from 'node:fs'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiModelTesterProtocolPresentation } from '@/lib/apiModelTesterPresentation'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function session() {
  return {
    csrfToken: 'csrf-user',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'user-1',
      username: 'buyer',
      displayName: 'Buyer',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: false },
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('API 模型测试器 adapter', () => {
  it('手填凭据只进入发现请求体，不进入 URL，并透传取消信号', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(session()))
      .mockResolvedValueOnce(jsonResponse({
        baseUrl: 'https://api.example.test/v1',
        models: ['gpt-4.1-mini'],
        discoveredAt: '2026-08-08T00:00:00Z',
      }))
    vi.stubGlobal('fetch', fetchMock)
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })
    const backend = await import('../apiModelTesterBackend')
    const controller = new AbortController()

    await backend.backendDiscoverAPIModels({
      kind: 'manual',
      baseUrl: ' https://target.example/v1 ',
      apiKey: ' secret-key ',
      acknowledgeInsecureHttp: false,
    }, controller.signal)

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(url).toBe('https://api.example.test/api/v1/tools/api-model-tester/discover')
    expect(url).not.toContain('secret-key')
    expect(init.signal).toBe(controller.signal)
    expect(JSON.parse(String(init.body))).toEqual({
      credentialSource: {
        kind: 'manual',
        baseUrl: 'https://target.example/v1',
        apiKey: 'secret-key',
        acknowledgeInsecureHttp: false,
      },
    })
  })

  it('订单来源测试只提交订单 ID 和所选模型', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(session()))
      .mockResolvedValueOnce(jsonResponse({
        model: 'gpt-4.1-mini',
        responsesResult: { succeeded: true, httpStatusClass: 2, durationMs: 80, errorCode: '' },
        chatCompletionsResult: { succeeded: false, httpStatusClass: 4, durationMs: 60, errorCode: 'protocol_unsupported' },
        testedAt: '2026-08-08T00:00:00Z',
      }))
    vi.stubGlobal('fetch', fetchMock)
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'real', apiBaseUrl: 'https://api.example.test/' })
    const backend = await import('../apiModelTesterBackend')

    await backend.backendTestAPIModel({ kind: 'order', orderId: 'order-1', acknowledgeInsecureHttp: true }, 'gpt-4.1-mini')
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toEqual({
      credentialSource: { kind: 'order', orderId: 'order-1', acknowledgeInsecureHttp: true },
      model: 'gpt-4.1-mini',
    })
  })
})

describe('API 模型测试器展示与临时状态', () => {
  it('分别呈现成功、额度警告和协议失败', () => {
    expect(apiModelTesterProtocolPresentation({ succeeded: true, httpStatusClass: 2, durationMs: 10, errorCode: '' })).toEqual({ label: '实际调用通过', tone: 'success' })
    expect(apiModelTesterProtocolPresentation({ succeeded: false, httpStatusClass: 4, durationMs: 10, errorCode: 'rate_limited' })).toEqual({ label: '额度或频率受限', tone: 'waiting' })
    expect(apiModelTesterProtocolPresentation({ succeeded: false, httpStatusClass: 4, durationMs: 10, errorCode: 'protocol_unsupported' })).toEqual({ label: '协议不支持', tone: 'risk' })
  })

  it('Mock 发现全部模型，并为两个公网协议返回独立结果', async () => {
    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: 'https://api.example.test/' })
    const facade = await import('../apiModelTesterFacade')
    const source = { kind: 'manual' as const, baseUrl: 'https://target.example/v1', apiKey: 'test-key', acknowledgeInsecureHttp: false }
    const discovery = await facade.discoverAPIModels(source)
    expect(discovery.models).toEqual(['gpt-4.1-mini', 'gpt-5-mini', 'claude-sonnet-4-5'])
    const result = await facade.testAPIModel(source, 'claude-sonnet-4-5')
    expect(result.responsesResult.errorCode).toBe('protocol_unsupported')
    expect(result.chatCompletionsResult.succeeded).toBe(true)
  })

  it('页面限制三路并发、只测试发现列表并在离开时清空 Key', () => {
    const page = readFileSync(new URL('../../pages/ApiModelTesterPage.vue', import.meta.url), 'utf8')
    expect(page).toContain('Math.min(3, uniqueModels.length)')
    expect(page).toContain('discovery.value?.models.includes(model)')
    expect(page).toContain('manual.apiKey = \'\'')
    expect(page).toContain('确认使用未加密 HTTP')
    expect(page).toContain('acknowledgeInsecureHttp')
    expect(page).toContain('onBeforeUnmount')
    expect(page).toContain('测试选中（{{ selectedCount * 2 }} 次调用）')
    expect(page).toContain('测试全部（{{ discovery.models.length * 2 }} 次调用）')
    expect(page.match(/class="flex-col items-stretch gap-2"/g)).toHaveLength(2)
    expect(page).not.toContain('<main class=')
    expect(page).not.toContain('localStorage')
    expect(page).not.toContain('sessionStorage')
    expect(page).not.toContain('analytics')
  })
})
