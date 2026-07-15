import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { getNavigationBadges } from '@/lib/api'

export const navigationBadgeQueryKey = ['navigation-badges'] as const
export const NAVIGATION_BADGE_POLL_INTERVAL_MS = 15_000

function isPageVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
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
