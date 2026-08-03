import assert from 'node:assert/strict'
import { afterEach, describe, it, vi } from 'vitest'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function problemResponse(detail: string, status = 503) {
  return new Response(JSON.stringify({ status, code: 'BACKEND_UNAVAILABLE', detail }), {
    status,
    headers: { 'content-type': 'application/problem+json' },
  })
}

function sessionResponse() {
  return jsonResponse({
    csrfToken: 'csrf-report-center',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'user-1',
      analyticsUserId: 'analytics-user-1',
      username: 'orbit',
      displayName: 'Orbit',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  })
}

function adminSessionResponse() {
  return jsonResponse({
    csrfToken: 'csrf-admin-report',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'admin-1',
      analyticsUserId: 'analytics-admin-1',
      username: 'moderator',
      displayName: 'Moderator',
      isAdmin: true,
      permissions: ['admin:reports'],
      linuxDoBinding: { bound: true },
    },
  })
}

async function loadBackend(fetchMock: ReturnType<typeof vi.fn>) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const backend = await import('../reportBackend')
  return { backend, client }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('举报与申诉中心真实后端适配器', () => {
  it('读取三类用户记录且保留服务端 canAppeal', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/v1/auth/session') return sessionResponse()
      if (url === '/api/v1/me/reports?limit=100') {
        return jsonResponse({ items: [{ id: 'report-1', status: 'closed' }], nextCursor: 'report-next' })
      }
      if (url === '/api/v1/me/reports?limit=100&cursor=report-next') {
        return jsonResponse({ items: [{ id: 'report-2', status: 'rejected' }] })
      }
      if (url === '/api/v1/me/disputes?limit=100') return jsonResponse({ items: [{ id: 'dispute-1', status: 'resolved', canAppeal: true }] })
      if (url === '/api/v1/me/appeals?limit=100') return jsonResponse({ items: [{ id: 'appeal-1', status: 'submitted' }] })
      throw new Error(`Unexpected request: ${url}`)
    })
    const { backend } = await loadBackend(fetchMock)

    const [reports, disputes, appeals] = await Promise.all([
      backend.backendMyReports(),
      backend.backendMyDisputes(),
      backend.backendMyAppeals(),
    ])

    assert.equal(reports[0]?.id, 'report-1')
    assert.equal(reports[1]?.id, 'report-2')
    assert.equal(disputes[0]?.canAppeal, true)
    assert.equal(appeals[0]?.id, 'appeal-1')
    assert.deepEqual(fetchMock.mock.calls.map(call => String(call[0])).sort(), [
      '/api/v1/auth/session',
      '/api/v1/me/appeals?limit=100',
      '/api/v1/me/disputes?limit=100',
      '/api/v1/me/reports?limit=100',
      '/api/v1/me/reports?limit=100&cursor=report-next',
    ])
  })

  it('提交申诉只发送服务端关联 ID 和脱敏说明', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(sessionResponse())
      .mockResolvedValueOnce(jsonResponse({ id: 'appeal-1', status: 'submitted' }, 201))
    const { backend } = await loadBackend(fetchMock)

    await backend.backendCreateAppeal({
      disputeId: 'dispute-1',
      title: '请求复核处理结果',
      statement: '公开结果与现有记录不一致，请复核。',
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    assert.equal(url, '/api/v1/me/appeals')
    assert.equal(init.method, 'POST')
    assert.equal(headers.get('X-CSRF-Token'), 'csrf-report-center')
    assert.match(headers.get('Idempotency-Key') ?? '', /^appeal-create-/)
    assert.deepEqual(JSON.parse(String(init.body)), {
      disputeId: 'dispute-1',
      title: '请求复核处理结果',
      statement: '公开结果与现有记录不一致，请复核。',
    })
    assert.equal(String(init.body).includes('targetType'), false)
    assert.equal(String(init.body).includes('targetId'), false)
  })

  it('补充材料只发送开放请求 ID 和脱敏纯文本并使用幂等键', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(sessionResponse())
      .mockResolvedValueOnce(jsonResponse({ dispute: { id: 'dispute-1', status: 'waiting_info', canSupplement: false } }))
    const { backend } = await loadBackend(fetchMock)

    await backend.backendSubmitInfoSupplement({
      entityType: 'dispute',
      entityId: 'dispute-1',
      openInfoRequestId: 'request-1',
      body: '订单状态与付款记录时间不一致，请复核。',
    })

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    assert.equal(url, '/api/v1/me/disputes/dispute-1/supplements')
    assert.equal(init.method, 'POST')
    assert.equal(headers.get('X-CSRF-Token'), 'csrf-report-center')
    assert.match(headers.get('Idempotency-Key') ?? '', /^dispute-supplement-/)
    assert.deepEqual(JSON.parse(String(init.body)), {
      openInfoRequestId: 'request-1',
      body: '订单状态与付款记录时间不一致，请复核。',
    })
  })

  it('管理员要求举报人补充时显式发送所选用户 ID', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(adminSessionResponse())
      .mockResolvedValueOnce(jsonResponse({
        id: 'report-1', reporterUserId: 'reporter-1', reporterUsername: 'orbit', reporterName: 'Orbit',
        title: '举报说明', targetLabel: 'API 订单', version: 2,
      }))
      .mockResolvedValueOnce(jsonResponse({ report: { id: 'report-1', status: 'needs_info', version: 3 } }))
    const { backend } = await loadBackend(fetchMock)

    await backend.backendRunReportAdminAction({
      id: 'report-1', primary: '举报说明', secondary: '', owner: '', status: '已分诊', risk: '', targetType: 'report',
    }, 'request_changes', '请补充订单时间线。', 'reporter-1')

    const [url, init] = fetchMock.mock.calls[2] as [string, RequestInit]
    assert.equal(url, '/api/v1/admin/reports/report-1/request-info')
    assert.deepEqual(JSON.parse(String(init.body)), {
      reason: '请补充订单时间线。',
      publicSummary: '请补充订单时间线。',
      publicResultCode: 'no_action',
      publicResult: '',
      requestedFromUserId: 'reporter-1',
    })
  })

  it('管理员要求纠纷补充时拒绝非参与者 ID', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(adminSessionResponse())
      .mockResolvedValueOnce(jsonResponse({
        id: 'dispute-1', primaryUserId: 'seller-1', counterpartyUserId: 'buyer-1', subjectUserId: 'buyer-1',
        publicSummary: '订单说明争议', targetLabel: 'API 订单', version: 2,
      }))
    const { backend } = await loadBackend(fetchMock)

    await assert.rejects(
      () => backend.backendRunReportAdminAction({
        id: 'dispute-1', primary: '订单说明争议', secondary: '', owner: '', status: '处理中', risk: '', targetType: 'dispute',
      }, 'request_changes', '请补充付款记录。', 'outsider-1'),
      /请选择当前纠纷中的有效参与者补充信息/,
    )
    assert.equal(fetchMock.mock.calls.length, 2)
  })

  it('真实列表失败时抛出 Problem Details，不回退本地记录', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(sessionResponse())
      .mockResolvedValueOnce(problemResponse('举报记录暂时不可用。'))
    const { backend, client } = await loadBackend(fetchMock)

    await assert.rejects(
      () => backend.backendMyReports(),
      (error: unknown) => error instanceof client.BackendProblemError && error.code === 'BACKEND_UNAVAILABLE',
    )
    assert.equal(fetchMock.mock.calls.length, 2)
  })
})
