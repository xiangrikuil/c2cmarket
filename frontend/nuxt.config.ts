import tailwindcss from '@tailwindcss/vite'
import { defineNuxtConfig } from 'nuxt/config'
import { resolve } from 'node:path'
import { routes } from './src/router'

const apiMode = process.env.NUXT_PUBLIC_API_MODE ?? ''
const publicApiBaseURL = process.env.NUXT_PUBLIC_API_BASE_URL ?? ''
const devApiProxyTarget = process.env.NUXT_DEV_API_PROXY_TARGET ?? 'http://127.0.0.1:8080'
const serverApiBaseURL = process.env.NUXT_API_BASE_URL
  || publicApiBaseURL
  || devApiProxyTarget
const siteURL = process.env.NUXT_PUBLIC_SITE_URL ?? 'https://c2cmarket.shop'
const isProductionBuild = process.env.NODE_ENV === 'production' && process.argv.includes('build')
const privateRouteRule = {
  cache: false,
  ssr: false,
  headers: { 'x-robots-tag': 'noindex, nofollow' },
} as const

if (isProductionBuild && apiMode !== 'real') {
  throw new Error('Production frontend builds must set NUXT_PUBLIC_API_MODE=real.')
}

if (isProductionBuild && !publicApiBaseURL) {
  throw new Error('Production frontend builds must set NUXT_PUBLIC_API_BASE_URL.')
}

if (isProductionBuild && !process.env.NUXT_API_BASE_URL) {
  throw new Error('Production frontend builds must set NUXT_API_BASE_URL.')
}

export default defineNuxtConfig({
  compatibilityDate: '2026-07-15',
  srcDir: 'src/',
  devtools: { enabled: false },
  components: [],
  typescript: {
    tsConfig: {
      compilerOptions: {
        allowImportingTsExtensions: true,
        noUncheckedIndexedAccess: false,
      },
      exclude: ['../src/**/__tests__/**'],
    },
  },
  modules: ['@nuxtjs/sitemap'],
  hooks: {
    'pages:extend'(pages) {
      pages.splice(0, pages.length, ...routes.map((route) => {
        const componentName = typeof route.component === 'function' ? route.component.name : ''
        return {
          path: route.path,
          name: typeof route.name === 'string' ? route.name : undefined,
          ...(componentName ? { file: resolve(process.cwd(), 'src/pages', `${componentName}.vue`) } : {}),
          ...(typeof route.redirect === 'string' ? { redirect: route.redirect } : {}),
          ...(route.meta ? { meta: route.meta } : {}),
        }
      }))
    },
  },
  css: ['~/styles.css', 'vue-sonner/style.css'],
  runtimeConfig: {
    apiBaseUrl: serverApiBaseURL,
    public: {
      apiMode,
      apiBaseUrl: publicApiBaseURL,
      siteUrl: siteURL,
      umamiEnabled: false,
      umamiScriptUrl: '',
      umamiWebsiteId: '',
      umamiDomains: '',
      umamiHostUrl: '',
      umamiDebug: false,
    },
  },
  site: {
    url: siteURL,
    name: 'C2CMarket',
  },
  sitemap: {
    sources: ['/api/__sitemap__/urls'],
    exclude: [
      '/search/**',
      '/login',
      '/auth/**',
      '/my/**',
      '/merchant/**',
      '/admin/**',
      '/api-intents/**',
      '/carpools/new',
      '/demands/new',
      '/api-market/new',
      '/announcements/**',
      '/u/**',
    ],
  },
  routeRules: {
    '/': { cache: { maxAge: 300, swr: true } },
    '/official-prices': { cache: { maxAge: 300, swr: true } },
    '/official-prices/**': { cache: { maxAge: 300, swr: true } },
    '/carpools': { cache: { maxAge: 120, swr: true } },
    '/carpools/**': { cache: { maxAge: 120, swr: true } },
    '/demands': { cache: { maxAge: 120, swr: true } },
    '/demands/**': { cache: { maxAge: 120, swr: true } },
    '/api-market': { cache: { maxAge: 120, swr: true } },
    '/api-market/**': { cache: { maxAge: 120, swr: true } },
    '/announcements/**': privateRouteRule,
    '/u/**': privateRouteRule,
    '/search/**': privateRouteRule,
    '/login': privateRouteRule,
    '/auth/**': privateRouteRule,
    '/my/**': privateRouteRule,
    '/merchant/**': privateRouteRule,
    '/admin/**': privateRouteRule,
    '/api-intents/**': privateRouteRule,
    '/carpools/new': privateRouteRule,
    '/demands/new': privateRouteRule,
    '/api-market/new': privateRouteRule,
  },
  nitro: {
    preset: 'cloudflare_module',
    compressPublicAssets: true,
  },
  vite: {
    plugins: [tailwindcss()],
    server: {
      allowedHosts: ['c2cmarket.shop', 'staging.c2cmarket.shop'],
      proxy: {
        '^/api(?:/|\\?|$)': {
          target: devApiProxyTarget,
          changeOrigin: true,
        },
        '/health': {
          target: devApiProxyTarget,
          changeOrigin: true,
        },
        '/readyz': {
          target: devApiProxyTarget,
          changeOrigin: true,
        },
      },
    },
  },
})
