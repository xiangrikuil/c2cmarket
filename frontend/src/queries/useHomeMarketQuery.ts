import { useQuery } from '@tanstack/vue-query'
import { getHomeMarket } from '@/lib/homeMarket'

export function useHomeMarket() {
  return useQuery({
    queryKey: ['home-market'],
    queryFn: getHomeMarket,
  })
}
