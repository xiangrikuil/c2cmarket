import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import {
  bucketPriceCny,
  bucketSeats,
  bucketVisibleSeconds,
  clearAnalyticsIdentity,
  identifyAnalyticsUser,
  normalizeAnalyticsPath,
  sanitizeAnalyticsEvent,
  setAnalyticsRuntimeConfig,
  trackAnalytics,
} from '../analytics'

afterEach(() => {
  setAnalyticsRuntimeConfig({})
  vi.useRealTimers()
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

  assert.deepEqual(sanitizeAnalyticsEvent('report_submit', {
    target_type: 'api_purchase_intent',
    reason_code: 'other',
    source_route: '/merchant/api-orders/12049d7e-7088-4c99-80c6-e6cc0e8eeed1',
  }), {
    source_route: '/merchant/api-orders/:id',
    entity_type: 'api_purchase_intent',
    reason_code: 'other',
  })

  assert.deepEqual(sanitizeAnalyticsEvent('report_submit', {
    target_type: 'api_purchase_intent',
    reason_code: 'other',
    source_route: '/api-intents/12049d7e-7088-4c99-80c6-e6cc0e8eeed1',
  }), {
    source_route: '/api-intents/:id',
    entity_type: 'api_purchase_intent',
    reason_code: 'other',
  })
})

test('auth and normalized page events keep only low-cardinality route fields', () => {
  assert.deepEqual(sanitizeAnalyticsEvent('oauth_login_start', {
    method: 'oauth_linux_do',
    source_route: '/login?returnTo=/my/rides/private-id',
    user_id: 'business-user-id',
    username: 'private-user',
    returnTo: '/my/rides/private-id',
    token: 'secret',
  }), {
    source_route: '/login',
    method: 'oauth_linux_do',
  })

  assert.deepEqual(sanitizeAnalyticsEvent('login_success', {
    method: 'untrusted-method',
    source_route: '/my/rides/12049d7e-7088-4c99-80c6-e6cc0e8eeed1?token=secret',
  }), {
    source_route: '/my/rides/:id',
    method: 'unknown',
  })

  assert.deepEqual(sanitizeAnalyticsEvent('normalized_page_view', {
    path: '/merchant/api-orders/12049d7e-7088-4c99-80c6-e6cc0e8eeed1?credential=secret',
    page_class: 'forged',
    entity_id: '12049d7e-7088-4c99-80c6-e6cc0e8eeed1',
  }), {
    path: '/merchant/api-orders/:id',
    page_class: 'merchant',
  })

  assert.equal(normalizeAnalyticsPath('/unknown/private-value'), '/other')
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

test('promotion events keep only low-cardinality placement fields', () => {
  const props = sanitizeAnalyticsEvent('api_promotion_impression', {
    placement: 'api_market_top',
    display_position: 'middle',
    category: 'Claude',
    billing_mode: 'fixed_package',
    target_type: 'api_service',
    source_route: '/api-market?view=free',
    promotion_id: 'secret-promotion-id',
    service_id: 'secret-service-id',
    title: 'raw title',
  })

  assert.deepEqual(props, {
    placement: 'api_market_top',
    display_position: 'middle',
    provider_category: 'claude',
    billing_mode: 'fixed_package',
    target_type: 'api_service',
    source_route: '/api-market',
  })
})

test('promotion benefit actions drop invite and business identifiers', () => {
  assert.deepEqual(sanitizeAnalyticsEvent('promotion_benefit_action', {
    action: 'coupon_apply',
    source_route: '/my/promotion-benefits?coupon=secret-coupon-id',
    invite_code: '2ABCDE89',
    coupon_id: 'secret-coupon-id',
    service_id: 'secret-service-id',
    user_id: 'secret-user-id',
    username: 'private-user',
  }), {
    action: 'coupon_apply',
    source_route: '/my/promotion-benefits',
  })

  assert.deepEqual(sanitizeAnalyticsEvent('promotion_benefit_action', {
    action: 'unknown-action',
    source_route: '/admin/growth-promotions?search=private-user',
  }), {
    action: 'unknown',
    source_route: '/admin/growth-promotions',
  })
})

test('trackAnalytics is a safe no-op unless analytics is enabled and Umami is loaded', () => {
  const track = vi.fn()
  vi.stubGlobal('window', { umami: { track } })

  trackAnalytics('search_submit', { has_query: true, result_count: 1 })
  assert.equal(track.mock.calls.length, 0)

  setAnalyticsRuntimeConfig({ enabled: true })
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

test('analytics failures never escape into business behavior', async () => {
  setAnalyticsRuntimeConfig({ enabled: true })
  vi.stubGlobal('window', {
    umami: {
      track: vi.fn(() => {
        throw new Error('blocked tracker')
      }),
    },
  })
  assert.doesNotThrow(() => trackAnalytics('login_page_view', { source_route: '/login' }))

  vi.stubGlobal('window', {
    umami: {
      track: vi.fn(() => Promise.reject(new Error('async tracker failure'))),
    },
  })
  assert.doesNotThrow(() => trackAnalytics('login_page_view', { source_route: '/login' }))
  await Promise.resolve()
})

test('analytics identity waits for Umami and clears the distinct ID on logout', async () => {
  vi.useFakeTimers()
  const browser: {
    umami: {
      track: ReturnType<typeof vi.fn>
      identify?: ReturnType<typeof vi.fn>
    }
  } = { umami: { track: vi.fn() } }
  vi.stubGlobal('window', browser)
  setAnalyticsRuntimeConfig({ enabled: true })

  identifyAnalyticsUser('A1111111-1111-4111-8111-111111111111')
  assert.equal(browser.umami.identify, undefined)

  browser.umami.identify = vi.fn()
  await vi.advanceTimersByTimeAsync(250)
  assert.deepEqual(browser.umami.identify.mock.calls, [
    ['a1111111-1111-4111-8111-111111111111'],
  ])

  clearAnalyticsIdentity()
  assert.deepEqual(browser.umami.identify.mock.calls.at(-1), [''])
})
