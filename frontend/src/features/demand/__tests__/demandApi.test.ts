import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { closeDemand, getDemandById, getDemands, submitDemand } from '../api'

function createSessionStorage(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear() {
      store.clear()
    },
    getItem(key: string) {
      return store.get(key) ?? null
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null
    },
    removeItem(key: string) {
      store.delete(key)
    },
    setItem(key: string, value: string) {
      store.set(key, value)
    },
  }
}

beforeEach(() => {
  vi.stubGlobal('window', {
    sessionStorage: createSessionStorage(),
    setTimeout: globalThis.setTimeout,
    clearTimeout: globalThis.clearTimeout,
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

test('creates, reads, and closes demand records through the feature API mock path', async () => {
  const { resetMockDemandsForTest } = await import('@/mocks/demand')
  resetMockDemandsForTest([])

  const created = await submitDemand({
    sourceUrl: 'https://linux.do/t/topic/123456',
    title: 'ChatGPT Business',
    maxPrice: 188,
    region: '美国区',
    ownerPreference: 'personal',
    note: '希望通过官方 workspace 成员席位加入。',
  })

  expect(created.id).toMatch(/^demand-/)
  expect(created.status).toBe('匹配中')

  const rows = await getDemands()
  expect(rows).toHaveLength(1)
  expect(rows[0]?.title).toBe('ChatGPT Business')

  const detail = await getDemandById(created.id)
  expect(detail?.id).toBe(created.id)

  const closed = await closeDemand(created.id)
  expect(closed.status).toBe('已关闭')

  const reopened = await closeDemand(created.id)
  expect(reopened.status).toBe('匹配中')
})
