import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type ReviewBackendModule = typeof import('../reviewBackend')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function sessionResponse() {
  return jsonResponse({
    csrfToken: 'csrf-review',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: '11111111-1111-4111-8111-111111111111',
      username: 'reviewer',
      displayName: 'Reviewer',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  })
}

function backendReviewRow(overrides: Record<string, unknown> = {}) {
  return {
    id: '22222222-2222-4222-8222-222222222222',
    transactionType: 'api_order',
    transactionId: '33333333-3333-4333-8333-333333333333',
    direction: 'received',
    target: 'API 额度订单',
    counterpartyUsername: 'seller',
    counterpartyName: '卖家',
    reviewerRole: 'seller',
    revieweeRole: 'buyer',
    status: 'sealed',
    visibility: 'sealed',
    counterpartySubmitted: true,
    canCreate: false,
    canEdit: false,
    rating: null,
    tags: [],
    note: null,
    completedAt: '2026-07-23T08:00:00Z',
    reviewDeadlineAt: '2026-08-06T08:00:00Z',
    submittedAt: '2026-07-24T08:00:00Z',
    visibleAt: null,
    frozenAt: null,
    createdAt: '2026-07-24T08:00:00Z',
    updatedAt: '2026-07-24T08:00:00Z',
    version: 1,
    ...overrides,
  }
}

async function loadReviewBackend(): Promise<ReviewBackendModule> {
  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  return import('../reviewBackend')
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('real review center preserves sealed content and backend preset tags', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(sessionResponse())
    .mockResolvedValueOnce(jsonResponse({
      items: [backendReviewRow()],
      presetTags: ['沟通顺畅', '付款及时'],
    }))
  vi.stubGlobal('fetch', fetchMock)

  const { backendReviewCenterRows } = await loadReviewBackend()
  const result = await backendReviewCenterRows()

  assert.deepEqual(result.presetTags, ['沟通顺畅', '付款及时'])
  assert.equal(result.items[0]?.transactionType, 'api_order')
  assert.equal(result.items[0]?.direction, 'received')
  assert.equal(result.items[0]?.visibility, 'sealed')
  assert.equal(result.items[0]?.rating, null)
  assert.equal(result.items[0]?.note, null)
  assert.deepEqual(result.items[0]?.tags, [])
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/me/reviews')
})

test('real review create uses the generic carpool transaction POST route', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(sessionResponse())
    .mockResolvedValueOnce(jsonResponse(backendReviewRow({
      transactionType: 'carpool_membership',
      transactionId: '44444444-4444-4444-8444-444444444444',
      direction: 'sent',
      reviewerRole: 'buyer',
      revieweeRole: 'seller',
      status: 'sealed',
      rating: 5,
      tags: ['规则清晰'],
      note: '规则说明清楚。',
    }), 201))
  vi.stubGlobal('fetch', fetchMock)

  const { backendSubmitReview } = await loadReviewBackend()
  await backendSubmitReview({
    transactionType: 'carpool_membership',
    transactionId: '44444444-4444-4444-8444-444444444444',
    operation: 'create',
    rating: 5,
    tags: ['规则清晰'],
    note: '规则说明清楚。',
  })

  const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
  const headers = new Headers(init.headers)
  assert.equal(url, '/api/v1/me/transactions/carpool_membership/44444444-4444-4444-8444-444444444444/review')
  assert.equal(init.method, 'POST')
  assert.equal(headers.get('X-CSRF-Token'), 'csrf-review')
  assert.match(headers.get('Idempotency-Key') ?? '', /^review-create-carpool_membership-/)
  assert.deepEqual(JSON.parse(String(init.body)), {
    rating: 5,
    tags: ['规则清晰'],
    note: '规则说明清楚。',
  })
})

test('real review edit uses the generic API order PUT route', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(sessionResponse())
    .mockResolvedValueOnce(jsonResponse(backendReviewRow({
      direction: 'sent',
      reviewerRole: 'buyer',
      revieweeRole: 'seller',
      rating: 4,
      tags: ['沟通顺畅'],
      note: '修改后的评价。',
      version: 2,
    })))
  vi.stubGlobal('fetch', fetchMock)

  const { backendSubmitReview } = await loadReviewBackend()
  const result = await backendSubmitReview({
    transactionType: 'api_order',
    transactionId: '33333333-3333-4333-8333-333333333333',
    operation: 'edit',
    rating: 4,
    tags: ['沟通顺畅'],
    note: '修改后的评价。',
  })

  const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
  const headers = new Headers(init.headers)
  assert.equal(url, '/api/v1/me/transactions/api_order/33333333-3333-4333-8333-333333333333/review')
  assert.equal(init.method, 'PUT')
  assert.match(headers.get('Idempotency-Key') ?? '', /^review-edit-api_order-/)
  assert.equal(result.version, 2)
  assert.equal(result.rating, 4)
})
