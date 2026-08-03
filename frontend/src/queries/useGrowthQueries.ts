import type { MaybeRefOrGetter } from 'vue'
import { computed, toValue } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { backendAdminGrowthOverview, type GrowthWindowDays } from '@/lib/growthBackend'

export function growthOverviewQueryKey(days: GrowthWindowDays) {
  return ['admin', 'growth-overview', days] as const
}

export function useAdminGrowthOverview(days: MaybeRefOrGetter<GrowthWindowDays>) {
  return useQuery({
    queryKey: computed(() => growthOverviewQueryKey(toValue(days))),
    queryFn: () => backendAdminGrowthOverview(toValue(days)),
    refetchOnMount: 'always',
    staleTime: 60_000,
  })
}
