import assert from 'node:assert/strict'
import { test, vi } from 'vitest'
import { createPromotionImpressionTracker, type PromotionAnalyticsProperties } from '../usePromotionImpression'

const properties: PromotionAnalyticsProperties = {
  placement: 'api_market_top',
  display_position: 'first',
  provider_category: 'gpt',
  billing_mode: 'metered_credit',
  target_type: 'api_service',
  source_route: '/api-market',
}

test('records one impression after continuous qualifying visibility', () => {
  vi.useFakeTimers()
  const track = vi.fn()
  const element = {} as Element
  const documentStub = {
    visibilityState: 'visible',
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  } as unknown as Document
  const observer = { observe: vi.fn(), unobserve: vi.fn(), disconnect: vi.fn() } as unknown as IntersectionObserver
  const tracker = createPromotionImpressionTracker({
    document: documentStub,
    observerFactory: () => observer,
    track,
  })

  tracker.observe(element, 'promotion-1', properties)
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 0.5 }])
  vi.advanceTimersByTime(999)
  assert.equal(track.mock.calls.length, 0)
  vi.advanceTimersByTime(1)
  assert.equal(track.mock.calls.length, 1)

  tracker.handleEntries([{ target: element, isIntersecting: false, intersectionRatio: 0 }])
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 1 }])
  vi.advanceTimersByTime(1000)
  assert.equal(track.mock.calls.length, 1)
  tracker.destroy()
  vi.useRealTimers()
})

test('cancels pending impression when visibility drops below threshold', () => {
  vi.useFakeTimers()
  const track = vi.fn()
  const element = {} as Element
  const tracker = createPromotionImpressionTracker({
    document: {
      visibilityState: 'visible',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as Document,
    observerFactory: () => ({ observe() {}, unobserve() {}, disconnect() {} } as IntersectionObserver),
    track,
  })

  tracker.observe(element, 'promotion-1', properties)
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 0.8 }])
  vi.advanceTimersByTime(500)
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 0.49 }])
  vi.advanceTimersByTime(1000)
  assert.equal(track.mock.calls.length, 0)
  tracker.destroy()
  vi.useRealTimers()
})

test('restarts the full impression delay when a qualifying element returns to a visible page', () => {
  vi.useFakeTimers()
  const track = vi.fn()
  const element = {} as Element
  const documentStub = {
    visibilityState: 'visible',
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  } as unknown as Document
  const tracker = createPromotionImpressionTracker({
    document: documentStub,
    observerFactory: () => ({ observe() {}, unobserve() {}, disconnect() {} } as IntersectionObserver),
    track,
  })

  tracker.observe(element, 'promotion-1', properties)
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 0.8 }])
  vi.advanceTimersByTime(500)

  Object.assign(documentStub, { visibilityState: 'hidden' })
  tracker.handleVisibilityChange()
  vi.advanceTimersByTime(1_000)
  assert.equal(track.mock.calls.length, 0)

  Object.assign(documentStub, { visibilityState: 'visible' })
  tracker.handleVisibilityChange()
  vi.advanceTimersByTime(999)
  assert.equal(track.mock.calls.length, 0)
  vi.advanceTimersByTime(1)
  assert.equal(track.mock.calls.length, 1)

  tracker.destroy()
  vi.useRealTimers()
})

test('keeps one timer when the same promotion element is registered again', () => {
  vi.useFakeTimers()
  const track = vi.fn()
  const element = {} as Element
  const observer = { observe: vi.fn(), unobserve: vi.fn(), disconnect: vi.fn() } as unknown as IntersectionObserver
  const tracker = createPromotionImpressionTracker({
    document: {
      visibilityState: 'visible',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as Document,
    observerFactory: () => observer,
    track,
  })
  const updatedProperties: PromotionAnalyticsProperties = {
    ...properties,
    display_position: 'middle',
  }

  tracker.observe(element, 'promotion-1', properties)
  tracker.handleEntries([{ target: element, isIntersecting: true, intersectionRatio: 0.8 }])
  vi.advanceTimersByTime(500)
  tracker.observe(element, 'promotion-1', updatedProperties)
  vi.advanceTimersByTime(499)
  assert.equal(track.mock.calls.length, 0)
  vi.advanceTimersByTime(1)

  assert.deepEqual(track.mock.calls, [[updatedProperties]])
  assert.equal((observer.observe as ReturnType<typeof vi.fn>).mock.calls.length, 1)
  tracker.destroy()
  vi.useRealTimers()
})

test('can be created without browser globals during SSR', () => {
  assert.equal(typeof Element, 'undefined')
  assert.equal(typeof IntersectionObserver, 'undefined')

  const tracker = createPromotionImpressionTracker({ track: vi.fn() })
  assert.doesNotThrow(() => tracker.observe({} as Element, 'promotion-1', properties))
  tracker.destroy()
})
