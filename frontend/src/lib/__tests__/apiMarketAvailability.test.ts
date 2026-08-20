import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'
import { getApiMarketAvailability, getApiQuotaOffers, getApiServices } from '../api'

const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const backendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const availabilityQuerySource = readFileSync(new URL('../../queries/useApiMarketAvailability.ts', import.meta.url), 'utf8')
const marketQueriesSource = readFileSync(new URL('../../queries/useMarketQueries.ts', import.meta.url), 'utf8')
const healthQueriesSource = readFileSync(new URL('../../queries/useApiHealthQueries.ts', import.meta.url), 'utf8')
const publishPageSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')

describe('API 市场导航在售徽章', () => {
  test('Mock 汇总按 Offer、套餐和服务三个不同交易单元统计', async () => {
    const [availability, services, offers] = await Promise.all([
      getApiMarketAvailability(),
      getApiServices({ online: true }),
      getApiQuotaOffers({ onlyOrderable: true }),
    ])
    const expectedFixedPackages = services
      .filter(service => service.billingMode === 'fixed_package')
      .reduce((total, service) => {
        const enabledModelIDs = new Set(service.modelPriceRows.map(model => model.modelId))
        return total + (service.packages ?? []).filter(item => item.enabled
          && item.stockAvailable > 0
          && item.models.some(model => enabledModelIDs.has(model.modelCatalogId))).length
      }, 0)

    assert.equal(availability.limitedOffers, offers.length)
    assert.equal(availability.fixedPackages, expectedFixedPackages)
    assert.equal(availability.meteredServices, services.filter(service => service.billingMode === 'metered_credit').length)
  })

  test('真实模式使用独立公开汇总接口并每 30 秒刷新', () => {
    assert.match(backendSource, /backendPublicAPIMarketAvailability[\s\S]*?\/api\/v1\/api-market\/availability/)
    assert.match(availabilityQuerySource, /API_MARKET_AVAILABILITY_REFRESH_MS = 30_000/)
    assert.match(availabilityQuerySource, /staleTime: API_MARKET_AVAILABILITY_REFRESH_MS/)
    assert.match(availabilityQuerySource, /refetchIntervalInBackground: false/)
  })

  test('桌面和移动子菜单都显示明确的零库存与 99+ 上限', () => {
    assert.match(appShellSource, /限量额度包[\s\S]*?limitedOffers/)
    assert.match(appShellSource, /短期流量包[\s\S]*?fixedPackages/)
    assert.match(appShellSource, /自选额度[\s\S]*?meteredServices/)
    assert.equal(appShellSource.match(/child\.count !== null/g)?.length, 2)
    assert.match(appShellSource, /count > 99 \? '99\+' : count/)
  })

  test('发布、库存、订单和探针变化都会使汇总失效', () => {
    assert.match(marketQueriesSource, /invalidateApiQuotaOwnerQueries[\s\S]*?apiMarketAvailabilityQueryKey/)
    assert.match(marketQueriesSource, /invalidateApiOrderQueries[\s\S]*?apiMarketAvailabilityQueryKey/)
    assert.ok((marketQueriesSource.match(/queryKey: apiMarketAvailabilityQueryKey/g)?.length ?? 0) >= 6)
    assert.match(healthQueriesSource, /queryKey: apiMarketAvailabilityQueryKey/)
    assert.match(publishPageSource, /queryKey: apiMarketAvailabilityQueryKey/)
  })
})
