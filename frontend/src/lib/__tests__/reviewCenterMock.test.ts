import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type ApiModule = typeof import('../api')

function createStorage() {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

async function loadMockAPI(): Promise<ApiModule> {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage(),
    localStorage: createStorage(),
    setTimeout: globalThis.setTimeout,
  })
  const api: ApiModule = await import('../api')
  await vi.dynamicImportSettled()
  return api
}

async function settle<T>(promise: Promise<T>) {
  await vi.runAllTimersAsync()
  return promise
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('mock 评价中心覆盖双盲、双方公开和截止状态', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI()

  const center = await settle(api.getReviewCenterRows())
  const pending = center.items.find(item => item.transactionId === 'ride-app-7' && item.direction === 'pending')
  const receivedSealed = center.items.find(item => item.transactionId === 'ride-app-7' && item.direction === 'received')
  const sentSealed = center.items.find(item => item.transactionId === 'ride-app-8' && item.direction === 'sent')
  const sentPublished = center.items.find(item => item.transactionId === 'ride-app-5' && item.direction === 'sent')
  const receivedPublished = center.items.find(item => item.transactionId === 'ride-app-5' && item.direction === 'received')
  const expired = center.items.find(item => item.transactionId === 'ride-app-9' && item.direction === 'pending')

  assert.equal(pending?.status, 'reviewable')
  assert.equal(pending?.counterpartySubmitted, true)
  assert.equal(receivedSealed?.visibility, 'sealed')
  assert.equal(receivedSealed?.rating, null)
  assert.equal(receivedSealed?.note, null)
  assert.deepEqual(receivedSealed?.tags, [])
  assert.equal(sentSealed?.visibility, 'sealed')
  assert.equal(sentSealed?.canEdit, true)
  assert.equal(sentSealed?.rating, 4)
  assert.equal(sentPublished?.visibility, 'published')
  assert.equal(sentPublished?.canEdit, false)
  assert.equal(receivedPublished?.visibility, 'published')
  assert.equal(receivedPublished?.rating, 4)
  assert.equal(expired?.status, 'expired')
  assert.equal(expired?.canCreate, false)
})

test('mock 第二方提交后双方评价立即公开冻结', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI()

  const submittedPromise = api.submitReview({
    transactionType: 'carpool_membership',
    transactionId: 'ride-app-7',
    operation: 'create',
    rating: 5,
    tags: ['沟通顺畅'],
    note: '沟通清楚，确认过程顺畅。',
  })
  const submitted = await settle(submittedPromise)
  assert.equal(submitted.visibility, 'published')
  assert.equal(submitted.canEdit, false)

  const center = await settle(api.getReviewCenterRows())
  const received = center.items.find(item => item.transactionId === 'ride-app-7' && item.direction === 'received')
  assert.equal(received?.visibility, 'published')
  assert.equal(received?.rating, 5)
  assert.equal(received?.note, '对方已经提交，双盲期内不应向买家显示。')
})

test('mock 拒绝超过十四天窗口的评价', async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-24T04:00:00.000Z'))
  const api = await loadMockAPI()

  const submission = api.submitReview({
    transactionType: 'carpool_membership',
    transactionId: 'ride-app-9',
    operation: 'create',
    rating: 5,
    tags: ['沟通顺畅'],
    note: '这条评价已经超过允许窗口。',
  })
  const rejection = assert.rejects(submission, /评价窗口已截止/)
  await vi.runAllTimersAsync()
  await rejection
})
