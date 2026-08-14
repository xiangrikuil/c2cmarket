import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { nextTick, ref } from 'vue'
import { afterEach, describe, test, vi } from 'vitest'
import { useCursorPagination } from '../../composables/useCursorPagination'

const cursorPageSources = [
  'OfficialPricesPage.vue',
  'OfficialPriceManagePage.vue',
  'CarpoolsPage.vue',
  'MyRidesPage.vue',
  'MerchantCarpoolApplicationsPage.vue',
  'MyCarpoolsPage.vue',
  'MyApiServicesPage.vue',
  'MyApiOrdersPage.vue',
  'MerchantApiOrdersPage.vue',
  'MyReviewsPage.vue',
  'AdminAnnouncementsPage.vue',
  'AdminSectionPage.vue',
].map(name => readFileSync(new URL(`../../pages/${name}`, import.meta.url), 'utf8'))

const allPageSources = readdirSync(new URL('../../pages', import.meta.url))
  .filter(name => name.endsWith('.vue'))
  .map(name => ({ name, source: readFileSync(new URL(`../../pages/${name}`, import.meta.url), 'utf8') }))

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('表格游标分页', () => {
  test('支持前进、返回，并在筛选变化后回到第一页', async () => {
    const filter = ref('all')
    const pagination = useCursorPagination([filter], 10)

    assert.equal(pagination.page.value, 1)
    assert.equal(pagination.cursor.value, undefined)
    assert.equal(pagination.pageSize, 10)

    pagination.next('cursor-2')
    assert.equal(pagination.page.value, 2)
    assert.equal(pagination.cursor.value, 'cursor-2')

    pagination.next('cursor-3')
    pagination.previous()
    assert.equal(pagination.page.value, 2)
    assert.equal(pagination.cursor.value, 'cursor-2')

    filter.value = 'exceptions'
    await nextTick()
    assert.equal(pagination.page.value, 1)
    assert.equal(pagination.cursor.value, undefined)
  })

  test('业务列表页面不再引用前端数组分页', () => {
    for (const source of cursorPageSources) {
      assert.doesNotMatch(source, /usePagination/)
      assert.doesNotMatch(source, /pagination\.paginatedRows/)
    }

    const allowedServerPageNumberTables = new Set([
      'AdminGrowthPromotionsPage.vue',
      'AdminUsersPage.vue',
      'MyPromotionBenefitsPage.vue',
    ])
    for (const { name, source } of allPageSources) {
      assert.doesNotMatch(source, /usePagination/)
      if (/import TablePagination from/.test(source)) {
        assert.equal(allowedServerPageNumberTables.has(name), true, `${name} 必须使用服务端分页元数据`)
      }
    }
  })

  test('真实后端模式不会回退到 Mock 数组分页', () => {
    const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
    const announcementSource = readFileSync(new URL('../announcementsApi.ts', import.meta.url), 'utf8')
    for (const source of [apiSource, announcementSource]) {
      let callOffset = source.indexOf('paginateCursorItems(')
      while (callOffset >= 0) {
        const functionOffset = source.lastIndexOf('export async function ', callOffset)
        const functionPrefix = source.slice(functionOffset, callOffset)
        assert.match(functionPrefix, /shouldUseRealBackend\(\)/)
        callOffset = source.indexOf('paginateCursorItems(', callOffset + 1)
      }
    }
    assert.match(apiSource, /if \(shouldUseRealBackend\(\)\) \{\s*throw new Error\(`管理模块 \$\{section\} 未配置服务端分页适配器。`\)/)
  })

  test('公开列表适配器透传筛选、limit 和 cursor，并规范化 nextCursor', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [], nextCursor: '  official-next  ' }))
      .mockResolvedValueOnce(jsonResponse({ items: [], nextCursor: null }))
    vi.stubGlobal('fetch', fetchMock)

    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'real' })
    const official = await import('../officialPriceBackend')
    const carpool = await import('../carpoolBackend')

    const officialPage = await official.backendOfficialPricesPage({
      productPlanIds: ['plan-1', 'plan-2'],
      region: 'ph',
      sort: 'cny_asc',
    }, { limit: 20, cursor: 'official-current' })
    const carpoolPage = await carpool.backendGetCarpoolsPage({
      productPlanIds: ['plan-3'],
      view: 'exceptions',
      risk: 'high',
    }, { limit: 10, cursor: 'carpool-current' })

    const [, officialQuery = ''] = String(fetchMock.mock.calls[0]?.[0]).split('?')
    assert.deepEqual(Object.fromEntries(new URLSearchParams(officialQuery)), {
      productPlanIds: 'plan-1,plan-2',
      region: 'ph',
      sort: 'cny_asc',
      limit: '20',
      cursor: 'official-current',
    })
    assert.equal(officialPage.nextCursor, 'official-next')

    const [, carpoolQuery = ''] = String(fetchMock.mock.calls[1]?.[0]).split('?')
    assert.deepEqual(Object.fromEntries(new URLSearchParams(carpoolQuery)), {
      productPlanIds: 'plan-3',
      view: 'exceptions',
      risk: 'high',
      limit: '10',
      cursor: 'carpool-current',
    })
    assert.equal(carpoolPage.nextCursor, undefined)
  })
})
