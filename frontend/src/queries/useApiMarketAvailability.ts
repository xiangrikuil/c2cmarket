import { useQuery } from '@tanstack/vue-query'
import { getApiMarketAvailability } from '@/lib/api'

export const apiMarketAvailabilityQueryKey = ['api-market-availability'] as const
export const API_MARKET_AVAILABILITY_REFRESH_MS = 30_000

function isPageVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

export function useApiMarketAvailability() {
  return useQuery({
    queryKey: apiMarketAvailabilityQueryKey,
    queryFn: getApiMarketAvailability,
    enabled: import.meta.client,
    staleTime: API_MARKET_AVAILABILITY_REFRESH_MS,
    refetchInterval: () => isPageVisible() ? API_MARKET_AVAILABILITY_REFRESH_MS : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  })
}
