import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import {
  normalizeGrowthWindowDays,
  validateGrowthOverview,
} from '../growthBackend'

const source = (relativePath: string) => readFileSync(new URL(relativePath, import.meta.url), 'utf8')

test('growth overview accepts only supported windows and the Shanghai snapshot contract', () => {
  assert.equal(normalizeGrowthWindowDays(7), 7)
  assert.equal(normalizeGrowthWindowDays('90'), 90)
  assert.equal(normalizeGrowthWindowDays('14'), 30)

  const overview = {
    windowDays: 30,
    timezone: 'Asia/Shanghai',
    registrationTrend: [],
    activityTrend: [],
    retentionCohorts: [],
  } as Parameters<typeof validateGrowthOverview>[0]
  assert.equal(validateGrowthOverview(overview, 30), overview)
  assert.throws(() => validateGrowthOverview({ ...overview, windowDays: 7 }, 30), /周期与请求不一致/)
  assert.throws(() => validateGrowthOverview({ ...overview, timezone: 'UTC' }, 30), /时区不受支持/)
})

test('growth dashboard remains admin-only, query-backed, and complete across states', () => {
  const backend = source('../growthBackend.ts')
  const queries = source('../../queries/useGrowthQueries.ts')
  const router = source('../../router.ts')
  const shell = source('../../components/layout/AdminShell.vue')
  const page = source('../../pages/AdminGrowthPage.vue')

  assert.match(backend, /ensureBackendSession\('admin', true\)/)
  assert.match(backend, /\/api\/v1\/admin\/growth-overview\?days=\$\{days\}/)
  assert.match(queries, /\['admin', 'growth-overview', days\]/)
  assert.match(router, /path: '\/admin\/growth'.*meta: adminAuthMeta/)
  assert.match(shell, /label: '用户增长', to: '\/admin\/growth'/)
  assert.match(page, /TabsTrigger :value="7"/)
  assert.match(page, /TabsTrigger :value="30"/)
  assert.match(page, /TabsTrigger :value="90"/)
  assert.match(page, /growthQuery\.isLoading\.value/)
  assert.match(page, /growthQuery\.isError\.value/)
  assert.match(page, /当前周期没有新增注册/)
  assert.match(page, /当前周期没有注册 Cohort 数据/)
  assert.match(page, /观察中/)
  assert.match(page, /completedCarpoolTransactions/)
  assert.match(page, /completedApiTransactions/)
})

test('browser analytics consumes OAuth outcomes once without leaking route data', () => {
  const plugin = source('../../plugins/browser.client.ts')
  const login = source('../../pages/LoginPage.vue')
  const backendClient = source('../backendClient.ts')

  assert.match(plugin, /captureRegistrationAttribution\(\)/)
  assert.match(plugin, /router\.afterEach/)
  assert.match(plugin, /trackAnalytics\('normalized_page_view', \{ path: route\.path \}\)/)
  assert.match(plugin, /rawOutcome === 'registered' \? 'registration_success' : 'login_success'/)
  assert.match(plugin, /delete query\.authOutcome/)
  assert.match(plugin, /lastTrackedPath = route\.path/)
  assert.match(login, /trackAnalytics\('login_page_view'/)
  assert.match(login, /trackAnalytics\('oauth_login_start'/)
  assert.match(login, /trackAnalytics\('login_success'/)
  assert.match(backendClient, /identifyAnalyticsUser\(session\.user\.analyticsUserId\)/)
  assert.match(backendClient, /clearAnalyticsIdentity\(\)/)
  assert.doesNotMatch(plugin, /route\.fullPath[^\n]*trackAnalytics/)
})
