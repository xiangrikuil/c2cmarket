import { onBeforeUnmount, onMounted } from 'vue'
import { trackAnalytics } from '@/lib/analytics'

export type PromotionDisplayPosition = 'first' | 'middle' | 'last'

export type PromotionAnalyticsProperties = {
  placement: 'api_market_top'
  display_position: PromotionDisplayPosition
  provider_category: string
  billing_mode: string
  target_type: 'api_service'
  source_route: '/api-market'
}

type VisibleEntry = Pick<IntersectionObserverEntry, 'target' | 'isIntersecting' | 'intersectionRatio'>

type TrackerOptions = {
  document?: Document
  delayMs?: number
  threshold?: number
  observerFactory?: (callback: IntersectionObserverCallback, options: IntersectionObserverInit) => IntersectionObserver
  track?: (properties: PromotionAnalyticsProperties) => void
}

type TrackedElement = {
  promotionId: string
  properties: PromotionAnalyticsProperties
  qualifiesForImpression: boolean
  timer?: ReturnType<typeof setTimeout>
}

export function createPromotionImpressionTracker(options: TrackerOptions = {}) {
  const doc = options.document ?? (typeof document === 'undefined' ? undefined : document)
  const delayMs = options.delayMs ?? 1000
  const threshold = options.threshold ?? 0.5
  const seen = new Set<string>()
  const tracked = new Map<Element, TrackedElement>()
  const elementByPromotion = new Map<string, Element>()
  const track = options.track ?? (properties => trackAnalytics('api_promotion_impression', properties))
  const factory = options.observerFactory ?? ((callback, init) => new IntersectionObserver(callback, init))
  const observer = typeof IntersectionObserver === 'undefined' && !options.observerFactory
    ? null
    : factory(entries => handleEntries(entries), { threshold: [threshold] })

  function cancel(item: TrackedElement) {
    if (item.timer) clearTimeout(item.timer)
    item.timer = undefined
  }

  function schedule(item: TrackedElement) {
    if (!item.qualifiesForImpression || seen.has(item.promotionId) || item.timer || doc?.visibilityState !== 'visible') return
    item.timer = setTimeout(() => {
      item.timer = undefined
      if (!item.qualifiesForImpression || seen.has(item.promotionId) || doc?.visibilityState !== 'visible') return
      seen.add(item.promotionId)
      track(item.properties)
    }, delayMs)
  }

  function handleEntries(entries: VisibleEntry[]) {
    for (const entry of entries) {
      const item = tracked.get(entry.target)
      if (!item) continue
      item.qualifiesForImpression = entry.isIntersecting && entry.intersectionRatio >= threshold
      if (item.qualifiesForImpression) schedule(item)
      else cancel(item)
    }
  }

  function handleVisibilityChange() {
    if (doc?.visibilityState === 'visible') {
      for (const item of tracked.values()) schedule(item)
      return
    }
    for (const item of tracked.values()) cancel(item)
  }

  function observe(element: Element, promotionId: string, properties: PromotionAnalyticsProperties) {
    const previous = elementByPromotion.get(promotionId)
    if (previous && previous !== element) unobserve(promotionId)

    const current = tracked.get(element)
    if (current?.promotionId === promotionId) {
      current.properties = properties
      return
    }
    if (current) {
      cancel(current)
      if (elementByPromotion.get(current.promotionId) === element) elementByPromotion.delete(current.promotionId)
      observer?.unobserve(element)
    }

    tracked.set(element, { promotionId, properties, qualifiesForImpression: false })
    elementByPromotion.set(promotionId, element)
    observer?.observe(element)
  }

  function unobserve(promotionId: string) {
    const element = elementByPromotion.get(promotionId)
    if (!element) return
    const item = tracked.get(element)
    if (item) cancel(item)
    observer?.unobserve(element)
    tracked.delete(element)
    elementByPromotion.delete(promotionId)
  }

  function destroy() {
    for (const item of tracked.values()) cancel(item)
    observer?.disconnect()
    tracked.clear()
    elementByPromotion.clear()
    doc?.removeEventListener('visibilitychange', handleVisibilityChange)
  }

  doc?.addEventListener('visibilitychange', handleVisibilityChange)
  return { observe, unobserve, handleEntries, handleVisibilityChange, destroy, seen }
}

export function usePromotionImpression() {
  let tracker: ReturnType<typeof createPromotionImpressionTracker> | null = null
  const pending = new Map<string, { element: Element, properties: PromotionAnalyticsProperties }>()

  onMounted(() => {
    tracker = createPromotionImpressionTracker()
    for (const [promotionId, item] of pending) tracker.observe(item.element, promotionId, item.properties)
  })

  onBeforeUnmount(() => {
    tracker?.destroy()
    tracker = null
    pending.clear()
  })

  function setPromotionElement(element: Element | null, promotionId: string, properties: PromotionAnalyticsProperties) {
    if (!element) {
      pending.delete(promotionId)
      tracker?.unobserve(promotionId)
      return
    }
    pending.set(promotionId, { element, properties })
    tracker?.observe(element, promotionId, properties)
  }

  function trackPromotionClick(properties: PromotionAnalyticsProperties) {
    trackAnalytics('api_promotion_click', properties)
  }

  return { setPromotionElement, trackPromotionClick }
}
