import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import {
  bucketPriceCny,
  bucketSeats,
  bucketVisibleSeconds,
  sanitizeAnalyticsEvent,
  trackAnalytics,
} from '../analytics'

afterEach(() => {
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

test('search events drop raw query data and keep only allowed fields', () => {
  const props = sanitizeAnalyticsEvent('search_submit', {
    rawKeyword: 'chatgpt token secret',
    q: 'chatgpt token secret',
    source_route: '/search?q=chatgpt%20token&token=abc',
    has_query: true,
    result_count: 7,
    filters_count: 2,
    unknown: 'leak',
  })

  assert.deepEqual(props, {
    source_route: '/search',
    has_query: true,
    result_count_bucket: '6_20',
    filters_count: 2,
  })
})

test('event sanitization normalizes enum values and drops arbitrary props', () => {
  const props = sanitizeAnalyticsEvent('favorite_toggle', {
    entity_type: 'api-service',
    action: 'delete',
    targetId: 'a1',
    source_route: '/api-market/a1?from=favorites',
    note: 'should not leave the browser',
  })

  assert.deepEqual(props, {
    source_route: '/api-market/:id',
    entity_type: 'api_service',
    action: 'unknown',
  })
})

test('source route normalization removes known dynamic identifiers', () => {
  assert.deepEqual(sanitizeAnalyticsEvent('favorite_toggle', {
    entity_type: 'api-service',
    action: 'add',
    source_route: '/api-market/a1?from=favorites',
  }), {
    source_route: '/api-market/:id',
    entity_type: 'api_service',
    action: 'add',
  })

  assert.deepEqual(sanitizeAnalyticsEvent('report_submit', {
    target_type: 'public_user',
    reason_code: 'other',
    source_route: '/u/orbit',
  }), {
    source_route: '/u/:username',
    entity_type: 'public_user',
    reason_code: 'other',
  })
})

test('subscription carpool analytics use low-cardinality buckets and product categories', () => {
  const props = sanitizeAnalyticsEvent('carpool_detail_view', {
    product: 'ChatGPT Pro 20x Web',
    monthly_price_cny: 88,
    seats: 6,
    access_mode: 'provider_member_invitation',
    risk_ack_required: true,
    risk_notice: 'openai_subscription_carpool',
    accessArrangementNote: 'raw note must be dropped',
  })

  assert.deepEqual(props, {
    product_category: 'gpt',
    access_mode: 'provider_member_invitation',
    price_bucket: '50_99',
    seats_bucket: '6_10',
    risk_ack_required: true,
    risk_notice: 'openai_subscription_carpool',
  })
})

test('bucket helpers keep numeric analytics low-cardinality', () => {
  assert.equal(bucketPriceCny(null), 'unknown')
  assert.equal(bucketPriceCny(19), 'lt_20')
  assert.equal(bucketPriceCny(88), '50_99')
  assert.equal(bucketPriceCny(220), '200_plus')

  assert.equal(bucketSeats(1), '1')
  assert.equal(bucketSeats(4), '2_5')
  assert.equal(bucketSeats(9), '6_10')
  assert.equal(bucketSeats(20), '11_20')

  assert.equal(bucketVisibleSeconds(2), 'lt_3')
  assert.equal(bucketVisibleSeconds(8), '3_9')
  assert.equal(bucketVisibleSeconds(75), '60_179')
  assert.equal(bucketVisibleSeconds(900), '600_plus')
})

test('api service events infer provider category from safe model text', () => {
  const props = sanitizeAnalyticsEvent('api_service_detail_view', {
    title: 'Claude API quota',
    billing_mode: 'metered_credit',
    delivery_mode: 'api_key_endpoint',
    minimum_purchase_cny: 20,
  })

  assert.deepEqual(props, {
    provider_category: 'claude',
    billing_mode: 'metered_credit',
    delivery_mode: 'api_key_endpoint',
    price_bucket: '20_49',
  })
})

test('trackAnalytics is a safe no-op unless analytics is enabled and Umami is loaded', () => {
  const track = vi.fn()
  vi.stubGlobal('window', { umami: { track } })

  trackAnalytics('search_submit', { has_query: true, result_count: 1 })
  assert.equal(track.mock.calls.length, 0)

  vi.stubEnv('VITE_UMAMI_ENABLED', 'true')
  trackAnalytics('search_submit', {
    has_query: true,
    result_count: 1,
    rawKeyword: 'secret',
  })
  assert.equal(track.mock.calls.length, 1)
  assert.deepEqual(track.mock.calls[0], [
    'search_submit',
    { has_query: true, result_count_bucket: '1_5' },
  ])

  vi.stubGlobal('window', {})
  assert.doesNotThrow(() => trackAnalytics('search_submit', { has_query: true }))
})
