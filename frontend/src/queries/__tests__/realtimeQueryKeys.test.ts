import assert from 'node:assert/strict'
import { QueryClient } from '@tanstack/vue-query'
import { test, vi } from 'vitest'
import { ALL_LIVE_QUERY_PREFIXES, invalidateAllLiveQueries } from '../realtimeQueryKeys'

test('all-live invalidation covers summaries, notifications, workflows, details, and admin queues', async () => {
  const queryClient = new QueryClient()
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue()

  await invalidateAllLiveQueries(queryClient)

  const invalidated = invalidate.mock.calls.map(call => call[0]?.queryKey?.[0])
  for (const requiredPrefix of [
    'navigation-badges',
    'notifications',
    'announcements',
    'my-api-orders',
    'merchant-api-orders',
    'api-orders',
    'api-order-payment-instructions',
    'my-carpool-applications',
    'merchant-carpool-applications',
    'carpool-application',
    'order-contacts',
    'admin-section',
    'admin-overview',
  ]) {
    assert.equal(invalidated.includes(requiredPrefix), true, `missing ${requiredPrefix}`)
  }
  assert.equal(invalidate.mock.calls.length, ALL_LIVE_QUERY_PREFIXES.length)
})
