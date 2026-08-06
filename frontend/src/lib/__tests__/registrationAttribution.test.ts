import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  captureRegistrationAttribution,
  clearRegistrationAttribution,
  getRegistrationAttribution,
} from '../registrationAttribution'

function createStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
    values,
  }
}

test('captures bounded first-touch registration attribution once per tab', () => {
  const storage = createStorage()
  const first = captureRegistrationAttribution({
    origin: 'https://c2cmarket.shop',
    pathname: '/carpools/12049d7e-7088-4c99-80c6-e6cc0e8eeed1',
    search: '?utm_source=linux.do%00&utm_medium=community&utm_campaign=launch',
  }, 'https://linux.do/t/topic/123?private=value', storage)

  assert.deepEqual(first, {
    source: 'linux.do',
    medium: 'community',
    campaign: 'launch',
    referrerHost: 'linux.do',
    landingPath: '/carpools/:id',
  })

  const second = captureRegistrationAttribution({
    origin: 'https://c2cmarket.shop',
    pathname: '/api-market/private-service-id',
    search: '?utm_source=overwrite',
  }, 'https://example.test/private/path', storage)
  assert.deepEqual(second, first)

  const stored = Array.from(storage.values.values()).join('')
  assert.equal(stored.includes('/t/topic/123'), false)
  assert.equal(stored.includes('private=value'), false)
  assert.equal(stored.includes('12049d7e'), false)
})

test('drops same-origin referrers and normalizes unknown landing paths', () => {
  const storage = createStorage()
  const attribution = captureRegistrationAttribution({
    origin: 'https://c2cmarket.shop',
    pathname: '/invite/private-code',
    search: `?utm_campaign=${'a'.repeat(140)}`,
  }, 'https://c2cmarket.shop/search?q=private', storage)

  assert.equal(attribution?.campaign?.length, 100)
  assert.equal(attribution?.referrerHost, undefined)
  assert.equal(attribution?.landingPath, '/other')
})

test('recovers from malformed stored attribution and supports explicit cleanup', () => {
  const storage = createStorage({
    'c2cmarket.registration-attribution.v1': '{not-json',
  })
  assert.equal(getRegistrationAttribution(storage), null)

  const captured = captureRegistrationAttribution({
    origin: 'https://c2cmarket.shop',
    pathname: '/login',
    search: '',
  }, 'not-a-url', storage)
  assert.deepEqual(captured, { landingPath: '/login' })

  clearRegistrationAttribution(storage)
  assert.equal(getRegistrationAttribution(storage), null)
})
