import { onBeforeUnmount, onMounted, watch, type Ref } from 'vue'
import { trackAnalytics } from '@/lib/analytics'

type VisibleEntityType = 'carpool' | 'api_service'

type VisibleDurationTrackerOptions = {
  emit: (seconds: number) => void
  now?: () => number
  minSeconds?: number
  maxSeconds?: number
}

export const createVisibleDurationTracker = ({
  emit,
  now = Date.now,
  minSeconds = 3,
  maxSeconds = 1800,
}: VisibleDurationTrackerOptions) => {
  let startedAt: number | null = null
  let elapsedMs = 0
  let sent = false

  const pause = () => {
    if (sent || startedAt === null) return
    elapsedMs += Math.max(0, now() - startedAt)
    startedAt = null
  }

  return {
    start() {
      if (sent || startedAt !== null) return
      startedAt = now()
    },
    pause,
    flush() {
      if (sent) return
      pause()
      const seconds = Math.floor(elapsedMs / 1000)
      elapsedMs = 0
      startedAt = null
      if (seconds < minSeconds) return
      sent = true
      emit(Math.min(maxSeconds, seconds))
    },
    reset() {
      startedAt = null
      elapsedMs = 0
      sent = false
    },
  }
}

export const useDetailVisibleAnalytics = (options: {
  enabled: Ref<boolean>
  entityType: VisibleEntityType
  sourceRoute: () => string
}) => {
  const tracker = createVisibleDurationTracker({
    emit: seconds => trackAnalytics('detail_visible_time', {
      entity_type: options.entityType,
      visible_seconds: seconds,
      source_route: options.sourceRoute(),
    }),
  })

  const stopEnabledWatch = watch(options.enabled, enabled => {
    if (enabled) {
      tracker.start()
      return
    }
    tracker.flush()
    tracker.reset()
  }, { immediate: true })

  const handleVisibilityChange = () => {
    if (typeof document === 'undefined') return
    if (document.visibilityState === 'hidden') {
      tracker.pause()
      return
    }
    if (options.enabled.value) tracker.start()
  }

  const flush = () => tracker.flush()

  onMounted(() => {
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
    if (typeof window !== 'undefined') window.addEventListener('pagehide', flush)
  })

  onBeforeUnmount(() => {
    flush()
    stopEnabledWatch()
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
    if (typeof window !== 'undefined') window.removeEventListener('pagehide', flush)
  })
}
