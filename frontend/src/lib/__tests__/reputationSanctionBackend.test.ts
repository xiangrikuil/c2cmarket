import assert from 'node:assert/strict'
import { afterEach, describe, it, vi } from 'vitest'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

async function loadBackend(fetchMock: ReturnType<typeof vi.fn>) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const backend = await import('../reputationBackend')
  return { backend, client }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('API 订单逾期处罚真实后端适配器', () => {
  it('读取服务端重算的资格、180 天次数和主体版本', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      eligible: true,
      reasonCode: 'eligible',
      remedyId: 'remedy-1',
      outcomeId: 'outcome-1',
      subjectUserId: 'seller-1',
      confirmedBreaches180Days: 2,
      recommendedDays: 30,
      alreadyApplied: false,
      subjectUserVersion: 7,
    }))
    const { backend } = await loadBackend(fetchMock)

    const result = await backend.backendAPIOrderSanctionRecommendation('dispute/1')

    assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/admin/disputes/dispute%2F1/sanction-recommendation')
    assert.equal(result.confirmedBreaches180Days, 2)
    assert.equal(result.recommendedDays, 30)
    assert.equal(result.subjectUserVersion, 7)
  })

  it('只提交内部说明并用主体用户版本保护显式处罚', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        restriction: {
          id: 'restriction-1',
          userId: 'seller-1',
          restrictionType: 'api_order_remedy_overdue',
          roleScope: 'seller',
          actionCode: 'api_service_publish',
          reasonCode: 'api_order_remedy_overdue',
          publicReason: '暂停新接单、发布和恢复；已成立订单仍可继续履约。',
          internalReason: '管理员核对逾期事实。',
          startsAt: '2026-08-09T12:00:00Z',
          endsAt: '2026-09-08T12:00:00Z',
          sourceDisputeOutcomeId: 'outcome-1',
          sourceDisputeRemedyId: 'remedy-1',
          createdByAdminId: 'admin-1',
          revokedAt: null,
          createdAt: '2026-08-09T12:00:00Z',
          updatedAt: '2026-08-09T12:00:00Z',
          version: 1,
          userVersion: 8,
        },
      }, 201))
    const { backend, client } = await loadBackend(fetchMock)
    client.setBackendCSRFToken('csrf-sanction-admin')

    await backend.backendApplyAPIOrderSanction({
      disputeCaseId: 'dispute-1',
      subjectUserId: 'seller-1',
      internalReason: '管理员核对逾期事实。',
      expectedUserVersion: 7,
    })

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(init.headers)
    assert.equal(url, '/api/v1/admin/disputes/dispute-1/sanction')
    assert.equal(init.method, 'POST')
    assert.equal(headers.get('X-CSRF-Token'), 'csrf-sanction-admin')
    assert.equal(headers.get('If-Match'), '"7"')
    assert.match(headers.get('Idempotency-Key') ?? '', /^api-order-sanction-/)
    assert.deepEqual(JSON.parse(String(init.body)), { internalReason: '管理员核对逾期事实。' })
    assert.equal(String(init.body).includes('subjectUserId'), false)
    assert.equal(String(init.body).includes('recommendedDays'), false)
    assert.equal(String(init.body).includes('remedyId'), false)
  })
})
