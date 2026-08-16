import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from '../featureFlags'
import { routes } from '../../router'

const marketPageSource = readFileSync(new URL('../../pages/ApiMarketPage.vue', import.meta.url), 'utf8')
const servicesPageSource = readFileSync(new URL('../../pages/MyApiServicesPage.vue', import.meta.url), 'utf8')

describe('限量额度包功能开关', () => {
  test('当前关闭用户入口并保留集中恢复点', () => {
    assert.equal(LIMITED_API_QUOTA_OFFERS_ENABLED, false)
    const quotaPublishRoute = routes.find(route => route.path === '/api-market/quota/new')
    assert.deepEqual(quotaPublishRoute && 'redirect' in quotaPublishRoute ? quotaPublishRoute.redirect : undefined, {
      path: '/api-market',
      query: { view: 'free' },
    })
    assert.match(marketPageSource, /v-if="LIMITED_API_QUOTA_OFFERS_ENABLED" value="limited"/)
    assert.match(marketPageSource, /useApiQuotaSaleSlots\(limitedViewEnabled\)/)
    assert.match(servicesPageSource, /visibleSalesChannels[\s\S]*?channel\.kind !== 'limited_quota'/)
  })
})
