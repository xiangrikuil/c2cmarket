import { describe, expect, it } from 'vitest'
import { routes } from '@/router'
import { apiMarketPath, apiMarketQueryForView, apiMarketViewFromPath } from '@/lib/apiMarketRoutes'
import { materializeNuxtPages } from '../../../nuxt.routes'

describe('API market canonical routes', () => {
  it('maps each market mode to a stable child path', () => {
    expect(apiMarketPath('limited')).toBe('/api-market/limited')
    expect(apiMarketPath('packages')).toBe('/api-market/packages')
    expect(apiMarketPath('free')).toBe('/api-market/free')
    expect(apiMarketViewFromPath('/api-market/packages')).toBe('packages')
  })

  it('preserves repeated package models and removes legacy or cross-mode filters', () => {
    expect(apiMarketQueryForView({
      view: 'packages',
      q: 'gpt',
      model: ['model-1', 'model-2'],
      duration: '7',
      distribution: 'sub2api',
    }, 'packages')).toEqual({ q: 'gpt', model: ['model-1', 'model-2'], duration: '7' })
  })

  it('registers static child routes before the dynamic detail route', () => {
    const paths = routes.map(route => route.path)
    const detailIndex = paths.indexOf('/api-market/:id')
    expect(paths.indexOf('/api-market/limited')).toBeLessThan(detailIndex)
    expect(paths.indexOf('/api-market/packages')).toBeLessThan(detailIndex)
    expect(paths.indexOf('/api-market/free')).toBeLessThan(detailIndex)
  })

  it('materializes function redirects as an executable Nuxt page', () => {
    const pages = materializeNuxtPages(routes, '/workspace/frontend')
    const legacyMarket = pages.find(page => page.path === '/api-market')
    expect(legacyMarket?.redirect).toBeUndefined()
    expect(legacyMarket?.file).toBe('/workspace/frontend/src/pages/RouteRedirectPage.vue')

    const routeRecord = routes.find(route => route.path === '/api-market')
    const redirect = routeRecord?.redirect as (to: { query: Record<string, unknown> }) => unknown
    expect(redirect({ query: { view: 'packages', model: ['model-1', 'model-2'], distribution: 'sub2api' } })).toEqual({
      path: '/api-market/packages',
      query: { model: ['model-1', 'model-2'] },
    })
  })
})
