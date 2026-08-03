import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { getApiMerchantDisplayName, isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const homePage = source('../../pages/HomePage.vue')
const app = source('../../App.vue')
const appShell = source('../../components/layout/AppShell.vue')
const adminShell = source('../../components/layout/AdminShell.vue')
const homeMarket = source('../homeMarket.ts')
const homeQuery = source('../../queries/useHomeMarketQuery.ts')
const appShellQueries = source('../../queries/useAppShellQueries.ts')
const realtimeQueries = source('../../queries/useRealtimeQueries.ts')
const marketQueries = source('../../queries/useMarketQueries.ts')
const apiFacade = source('../api.ts')
const nuxtConfig = source('../../../nuxt.config.ts')

describe('development first-page architecture', () => {
  it('keeps homepage and app-shell entry points off the monolithic market facade', () => {
    expect(app).toContain("defineAsyncComponent(() => import('@/components/layout/AdminShell.vue'))")
    expect(app).not.toContain("import AdminShell from '@/components/layout/AdminShell.vue'")
    expect(homePage).toContain("from '@/queries/useHomeMarketQuery'")
    expect(homePage).toContain("from '@/lib/apiServicePresentation'")
    expect(homePage).not.toMatch(/from ['"]@\/lib\/api['"]/)
    expect(homePage).not.toContain("from '@/queries/useMarketQueries'")

    expect(appShell).toContain("from '@/queries/useAppShellQueries'")
    expect(appShell).not.toContain("from '@/queries/useMarketQueries'")
    expect(adminShell).toContain("from '@/queries/useAppShellQueries'")
    expect(adminShell).not.toContain("from '@/queries/useMarketQueries'")
    expect(appShellQueries).not.toMatch(/from ['"]@\/lib\/api['"]/)
    expect(realtimeQueries).not.toMatch(/from ['"]@\/lib\/api['"]/)
    for (const entry of [homePage, appShell, adminShell]) {
      expect(entry).not.toContain("from 'lucide-vue-next'")
      expect(entry).toContain("from 'lucide-vue-next/dist/esm/icons/")
    }
  })

  it('uses focused real adapters and loads the compatibility facade only for Mock mode', () => {
    expect(homeMarket).toContain('Promise.all([')
    expect(homeMarket).toContain('backendOfficialPrices()')
    expect(homeMarket).toContain('backendGetCarpools()')
    expect(homeMarket).toContain('backendAPIServices({ online: true })')
    expect(homeMarket).toContain("await import('@/lib/api')")
    expect(homeMarket).toContain('import.meta.dev && developmentCache')
    expect(homeMarket).toContain('developmentCacheDurationMs = 2_000')
    expect(homeQuery).toContain("queryKey: ['home-market']")

    expect(appShellQueries).toContain('shouldUseRealBackend()')
    expect(appShellQueries).toContain("await import('@/lib/api')")
    expect(realtimeQueries).toContain("await import('@/lib/navigationBadgeBackend')")
    expect(realtimeQueries).toContain("await import('@/lib/api')")
  })

  it('preserves facade exports for existing consumers', () => {
    expect(apiFacade).toContain("export { getApiMerchantDisplayName, isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'")
    expect(marketQueries).toContain("export { useHomeMarket } from '@/queries/useHomeMarketQuery'")
    expect(marketQueries).toContain('useMyApiServices, useMyCarpools, useMyProfileQuery, useNotifications')
  })

  it('disables public response caching only in development and pre-bundles first-page dependencies', () => {
    expect(nuxtConfig).toContain("process.env.NODE_ENV === 'development'")
    expect(nuxtConfig).toContain("[path, { cache: false }]")
    expect(nuxtConfig).toContain("'vite:serverCreated'")
    expect(nuxtConfig).toContain('server.transformRequest(`/pages/${name}.vue?macro=true`, { ssr: true })')
    expect(nuxtConfig).toContain("'/': { cache: { maxAge: 300, swr: true } }")
    expect(nuxtConfig).toContain("'/carpools': { cache: { maxAge: 120, swr: true } }")
    expect(nuxtConfig).toContain("'/api-market': { cache: { maxAge: 120, swr: true } }")

    for (const dependency of [
      '@radix-icons/vue',
      '@tanstack/vue-query',
      '@vueuse/core',
      'class-variance-authority',
      'clsx',
      'decimal.js',
      'dompurify',
      'lucide-vue-next',
      'marked',
      'pinia',
      'reka-ui',
      'tailwind-merge',
      'vue-sonner',
    ]) {
      expect(nuxtConfig).toContain(`'${dependency}'`)
    }
  })

  it('keeps merchant display and orderability behavior unchanged', () => {
    expect(getApiMerchantDisplayName({
      merchant: 'merchant-id',
      merchantIdentityMode: 'store_alias',
      merchantDisplayName: 'Public Store',
    })).toBe('Public Store')
    expect(getApiMerchantDisplayName({
      merchant: 'merchant-id',
      snapshot: {
        merchant: 'Snapshot Store',
        merchantDisplayName: '',
      },
    })).toBe('Snapshot Store')
    expect(isApiServicePubliclyOrderable({ online: true, publiclyOrderable: true })).toBe(true)
    expect(isApiServicePubliclyOrderable({ online: true, publiclyOrderable: false })).toBe(false)
  })
})
