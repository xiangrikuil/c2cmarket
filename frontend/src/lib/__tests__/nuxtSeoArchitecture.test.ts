import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { resolveRouteSeo } from '@/seo/routeSeo'

function route(path: string, name: string = 'test') {
  return {
    path,
    name,
    meta: {},
  } as RouteLocationNormalizedLoaded
}

describe('Nuxt hybrid rendering and SEO architecture', () => {
  it('indexes public discovery routes and blocks private application routes', () => {
    expect(resolveRouteSeo(route('/carpools')).indexable).toBe(true)
    expect(resolveRouteSeo(route('/api-market/service-1')).indexable).toBe(true)
    expect(resolveRouteSeo(route('/search')).indexable).toBe(false)
    expect(resolveRouteSeo(route('/my/api-orders')).indexable).toBe(false)
    expect(resolveRouteSeo(route('/announcements/release-notes')).indexable).toBe(false)
    expect(resolveRouteSeo(route('/u/orbit')).indexable).toBe(false)
  })

  it('marks the catch-all route as a non-indexable not-found page', () => {
    const seo = resolveRouteSeo(route('/missing-page', 'not-found'))
    expect(seo.indexable).toBe(false)
    expect(seo.title).toContain('页面不存在')
  })

  it('keeps public SSR and private CSR route rules explicit', () => {
    const source = readFileSync(new URL('../../../nuxt.config.ts', import.meta.url), 'utf8')

    expect(source).toContain("'/carpools': { cache: { maxAge: 120, swr: true } }")
    expect(source).toContain("'/carpools/**': { cache: { maxAge: 120, swr: true } }")
    expect(source).toContain("'/api-market': { cache: { maxAge: 120, swr: true } }")
    expect(source).toContain("'/api-market/**': { cache: { maxAge: 120, swr: true } }")
    expect(source).toContain('cache: false')
    expect(source).toContain("'/search/**': privateRouteRule")
    expect(source).toContain("'/my/**': privateRouteRule")
    expect(source).toContain("'/u/**': privateRouteRule")
    expect(source).toContain("'x-robots-tag': 'noindex, nofollow'")
    expect(source).toContain("preset: 'cloudflare_module'")
    expect(source).toContain("sources: ['/api/__sitemap__/urls']")
  })

  it('keeps query hydration and public server prefetch wired into Nuxt', () => {
    const plugin = readFileSync(new URL('../../plugins/vue-query.ts', import.meta.url), 'utf8')
    const home = readFileSync(new URL('../../pages/HomePage.vue', import.meta.url), 'utf8')

    expect(plugin).toContain('dehydrate(queryClient)')
    expect(plugin).toContain('hydrate(queryClient, vueQueryState.value)')
    expect(home).toContain('prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)')
  })

  it('uses only Nuxt runtime variables in current frontend deployment surfaces', () => {
    const config = readFileSync(new URL('../../../nuxt.config.ts', import.meta.url), 'utf8')
    const currentDeploymentFiles = [
      '../../../../.github/workflows/ci.yml',
      '../../../../.env.example',
      '../../../../.env.production.example',
      '../../../../.env.staging.example',
    ].map(path => readFileSync(new URL(path, import.meta.url), 'utf8'))

    expect(config).not.toMatch(/\bVITE_[A-Z0-9_]+\b/)
    expect(currentDeploymentFiles.join('\n')).not.toMatch(/\bVITE_[A-Z0-9_]+\b/)
    expect(config).toContain("apiMode = process.env.NUXT_PUBLIC_API_MODE ?? ''")
    expect(config).toContain("process.argv.includes('build')")
    expect(config).toContain('!publicApiBaseURL')
    expect(config).toContain('!process.env.NUXT_API_BASE_URL')
    expect(config).toContain('NUXT_DEV_API_PROXY_TARGET')
  })
})
