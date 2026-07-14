import { computed, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { useEventListener, useEventSource, useIntervalFn } from '@vueuse/core'
import { useQueryClient } from '@tanstack/vue-query'
import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  REALTIME_EVENT_NAMES,
  hasAllLiveTopic,
  realtimeEventsURL,
  tryDecodeRealtimeEventEnvelope,
  type RealtimeEventEnvelope,
} from '@/lib/realtimeEvents'
import { invalidateAllLiveQueries } from '@/queries/realtimeQueryKeys'

export function useRealtimeSync(enabled: MaybeRefOrGetter<boolean> = true) {
  const queryClient = useQueryClient()
  const active = computed(() => shouldUseRealBackend() && Boolean(toValue(enabled)))
  const url = computed(() => active.value ? realtimeEventsURL() : undefined)
  const reconcile = () => invalidateAllLiveQueries(queryClient)
  const stream = useEventSource<typeof REALTIME_EVENT_NAMES, RealtimeEventEnvelope | null>(
    url,
    REALTIME_EVENT_NAMES,
    {
      withCredentials: true,
      autoReconnect: { retries: -1, delay: 3_000 },
      serializer: { read: tryDecodeRealtimeEventEnvelope },
    },
  )

  watch(stream.status, (status, previousStatus) => {
    if (active.value && status === 'OPEN' && previousStatus !== 'OPEN') {
      void reconcile()
    }
  })

  watch([stream.event, stream.data], ([eventName, envelope]) => {
    if (!active.value || !eventName || !envelope || !hasAllLiveTopic(envelope)) return
    void reconcile()
  })

  useIntervalFn(() => {
    if (!active.value || stream.status.value === 'OPEN') return
    if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
    void reconcile()
  }, 15_000)

  if (typeof document !== 'undefined') {
    useEventListener(document, 'visibilitychange', () => {
      if (active.value && document.visibilityState === 'visible') void reconcile()
    })
  }
  if (typeof window !== 'undefined') {
    useEventListener(window, 'online', () => {
      if (!active.value) return
      if (stream.status.value === 'CLOSED') stream.open()
      void reconcile()
    })
  }

  return {
    ...stream,
    active,
    reconcile,
  }
}
