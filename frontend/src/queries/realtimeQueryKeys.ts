import type { QueryClient, QueryKey } from '@tanstack/vue-query'

export const ALL_LIVE_QUERY_PREFIXES = [
  ['navigation-badges'],
  ['notifications'],
  ['api-order-notifications'],
  ['carpool-notifications'],
  ['announcements'],
  ['feedback'],
  ['feedback-unread-count'],
  ['admin-feedback'],
  ['my-api-purchase-intents'],
  ['merchant-api-purchase-intents'],
  ['api-purchase-intents'],
  ['api-purchase-intent-events'],
  ['my-api-orders'],
  ['merchant-api-orders'],
  ['api-orders'],
  ['api-order-payment-instructions'],
  ['my-carpool-applications'],
  ['merchant-carpool-applications'],
  ['carpool-application'],
  ['carpool-application-events'],
  ['order-contacts'],
  ['carpools'],
  ['admin-section'],
  ['admin-overview'],
  ['admin-official-price-records'],
] as const satisfies readonly QueryKey[]

export async function invalidateAllLiveQueries(queryClient: Pick<QueryClient, 'invalidateQueries'>) {
  await Promise.all(ALL_LIVE_QUERY_PREFIXES.map(queryKey => queryClient.invalidateQueries({ queryKey })))
}
