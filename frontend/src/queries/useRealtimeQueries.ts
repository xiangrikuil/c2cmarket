import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { shouldUseRealBackend } from '@/lib/backendClient'

export const navigationBadgeQueryKey = ['navigation-badges'] as const
export const NAVIGATION_BADGE_POLL_INTERVAL_MS = 15_000

function isPageVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

async function getNavigationBadges() {
  if (shouldUseRealBackend()) {
    const { backendNavigationBadges } = await import('@/lib/navigationBadgeBackend')
    return backendNavigationBadges()
  }
  const api = await import('@/lib/api')
  return api.getNavigationBadges()
}

export function useNavigationBadges(enabled: MaybeRefOrGetter<boolean> = true) {
  return useQuery({
    queryKey: navigationBadgeQueryKey,
    queryFn: getNavigationBadges,
    enabled: computed(() => Boolean(toValue(enabled))),
    refetchInterval: () => isPageVisible() ? NAVIGATION_BADGE_POLL_INTERVAL_MS : false,
    refetchIntervalInBackground: false,
    refetchOnMount: 'always',
    refetchOnWindowFocus: 'always',
    refetchOnReconnect: 'always',
  })
}
