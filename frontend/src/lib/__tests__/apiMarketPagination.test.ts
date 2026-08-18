import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, describe, test, vi } from 'vitest'
import {
  collectCursorPages,
  flattenUniqueCursorPages,
  nextUnseenCursor,
  normalizeNextCursor,
  paginateCursorItems,
} from '../cursorPagination'

const marketPageSource = readFileSync(new URL('../../pages/ApiMarketPage.vue', import.meta.url), 'utf8')
const marketQueriesSource = readFileSync(new URL('../../queries/useMarketQueries.ts', import.meta.url), 'utf8')
const sentinelSource = readFileSync(new URL('../../components/market/InfiniteScrollSentinel.vue', import.meta.url), 'utf8')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('cursor 分页工具', () => {
  test('mock cursor 逐页前进并在末页停止', () => {
    const items = Array.from({ length: 45 }, (_, index) => ({ id: `item-${index + 1}` }))

    const first = paginateCursorItems(items, { limit: 20 })
    const second = paginateCursorItems(items, { limit: 20, cursor: first.nextCursor })
    const last = paginateCursorItems(items, { limit: 20, cursor: second.nextCursor })

    assert.deepEqual(first.items.map(item => item.id), items.slice(0, 20).map(item => item.id))
    assert.equal(first.nextCursor, 'mock:20')
    assert.deepEqual(second.items.map(item => item.id), items.slice(20, 40).map(item => item.id))
    assert.equal(second.nextCursor, 'mock:40')
    assert.deepEqual(last.items.map(item => item.id), items.slice(40).map(item => item.id))
    assert.equal(last.nextCursor, undefined)
  })

  test('mock 分页拒绝格式错误的 cursor', () => {
    for (const cursor of ['not-a-cursor', 'mock:999999999999999999999999']) {
      assert.throws(
        () => paginateCursorItems([{ id: 'item-1' }], { cursor }),
        /分页 cursor 无效/,
      )
    }
  })

  test('规范化空 cursor 并按业务 ID 合并重复页边界', () => {
    assert.equal(normalizeNextCursor('  opaque-next  '), 'opaque-next')
    assert.equal(normalizeNextCursor('   '), undefined)
    assert.equal(normalizeNextCursor(null), undefined)

    const rows = flattenUniqueCursorPages([
      { items: [{ id: 'offer-1', version: 1 }, { id: 'offer-2', version: 1 }], nextCursor: 'page-2' },
      { items: [{ id: 'offer-1', version: 2 }, { id: 'offer-3', version: 1 }] },
    ])
    assert.deepEqual(rows.map(item => item.id), ['offer-1', 'offer-2', 'offer-3'])
    assert.equal(rows[0]?.version, 2)
  })

  test('无限分页在服务端重复 cursor 时停止续页', () => {
    assert.equal(nextUnseenCursor(' next-page ', ['', 'first-page']), 'next-page')
    assert.equal(nextUnseenCursor(' first-page ', ['', 'first-page']), undefined)
  })

  test('兼容全量读取在服务端重复 cursor 时显式失败', async () => {
    await assert.rejects(
      collectCursorPages(async ({ cursor }) => ({
        items: [cursor ?? 'first'],
        nextCursor: 'repeated',
      })),
      /分页 cursor 重复/,
    )
  })
})

describe('API 市场分页适配', () => {
  test('额度包和服务适配器序列化 limit/cursor 并保留 nextCursor', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [], nextCursor: '  quota-next  ' }))
      .mockResolvedValueOnce(jsonResponse({ items: [], nextCursor: null }))
    vi.stubGlobal('fetch', fetchMock)

    const client = await import('../backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'real' })
    const backend = await import('../apiMarketBackend')

    const quotaPage = await backend.backendPublicAPIQuotaOffersPage({
      distributionSystem: 'sub2api',
      modelCatalogId: 'model-1',
      oneMultiplier: true,
      maxMultiplier: 1.2,
      onlyOrderable: true,
      saleMode: 'scheduled',
      sort: 'allowance_desc',
      slotKey: '2026-08-09@13:00',
      search: 'target service',
      excludeSystemSlots: true,
    }, { limit: 20, cursor: 'opaque+/=' })
    const servicePage = await backend.backendAPIServicesPage({
      online: true,
      billingMode: 'fixed_package',
      search: 'target seller',
      packageModelCatalogIds: ['model-1', 'model-2'],
      packageDurationDays: 7,
      packagePriceCnyMax: 20,
      packageMultiplierMax: 1.2,
      sort: 'package_price_asc',
    }, { limit: 20, cursor: 'service-next' })

    const [quotaPath, quotaQuery = ''] = String(fetchMock.mock.calls[0]?.[0]).split('?')
    assert.equal(quotaPath, '/api/v1/api-quota-offers')
    assert.deepEqual(Object.fromEntries(new URLSearchParams(quotaQuery)), {
      distributionSystem: 'sub2api',
      modelCatalogId: 'model-1',
      oneMultiplier: 'true',
      maxMultiplier: '1.2',
      onlyOrderable: 'true',
      saleMode: 'scheduled',
      sort: 'allowance_desc',
      slotKey: '2026-08-09@13:00',
      search: 'target service',
      excludeSystemSlots: 'true',
      limit: '20',
      cursor: 'opaque+/=',
    })
    assert.equal(quotaPage.nextCursor, 'quota-next')

    const [servicePath, serviceQuery = ''] = String(fetchMock.mock.calls[1]?.[0]).split('?')
    assert.equal(servicePath, '/api/v1/api-services')
    const serviceParams = new URLSearchParams(serviceQuery)
    assert.deepEqual(serviceParams.getAll('packageModelCatalogIds'), ['model-1', 'model-2'])
    assert.deepEqual(Object.fromEntries(serviceParams), {
      billingMode: 'fixed_package',
      packageModelCatalogIds: 'model-2',
      packageDurationDays: '7',
      search: 'target seller',
      packagePriceCnyMax: '20',
      packageMultiplierMax: '1.2',
      sort: 'package_price_asc',
      limit: '20',
      cursor: 'service-next',
    })
    assert.equal(servicePage.nextCursor, undefined)
  })
})

describe('API 市场无限滚动接线', () => {
  test('无限查询用筛选和市场视图隔离 cursor 链并读取服务端 nextCursor', () => {
    assert.match(marketQueriesSource, /queryKey: computed\(\(\) => \['api-services', 'infinite', valueOf\(scope\), valueOf\(filters\)\]\)/)
    assert.match(marketQueriesSource, /queryFn: \(\{ pageParam \}\) => getApiServicesPage\(valueOf\(filters\), \{ limit: 20, cursor: pageParam \|\| undefined \}\)/)
    assert.match(marketPageSource, /useInfiniteApiServices\(serviceFilters, serviceViewEnabled, activeView\)/)
    assert.match(marketPageSource, /billingMode: activeView\.value === 'packages' \? 'fixed_package' : 'metered_credit'/)
    assert.match(marketPageSource, /packageModelCatalogIds: activeView\.value === 'packages' \? packageModels\.value : undefined/)
    assert.match(marketPageSource, /packageDurationDays: activeView\.value === 'packages' && packageDuration\.value \? Number\(packageDuration\.value\) : undefined/)
    assert.match(marketPageSource, /search: debouncedSearch\.value\.trim\(\) \|\| undefined/)
    assert.match(marketPageSource, /modelCatalogId: packageModel\.value \|\| undefined/)
    assert.match(marketPageSource, /packagePriceCnyMax:/)
    assert.match(marketPageSource, /minimumPurchaseCnyMax:/)
    assert.match(marketPageSource, /excludeSystemSlots: true/)
    assert.match(marketQueriesSource, /queryKey: computed\(\(\) => \['api-quota-offers', 'infinite', valueOf\(filters\)\]\)/)
    assert.equal((marketQueriesSource.match(/getNextPageParam: \(lastPage, _pages, _lastPageParam, pageParams\) => nextUnseenCursor\(lastPage\.nextCursor, pageParams\)/g) ?? []).length, 2)
    assert.equal((marketQueriesSource.match(/enabled: computed\(\(\) => valueOf\(enabled\)\)/g) ?? []).length >= 2, true)
    assert.match(marketPageSource, /const visibleMarketQuery = activeView\.value === 'limited' \? quotaQuery : freeServicesQuery/)
    assert.match(marketPageSource, /useApiPackageFilterOptions\(computed\(\(\) => activeView\.value === 'packages'\)\)/)
    assert.match(marketPageSource, /\.\.\.\(activeView\.value === 'packages' \? \[packageFilterOptionsQuery\] : \[\]\)/)
    assert.doesNotMatch(marketPageSource, /prefetchQueriesOnServer\(quotaQuery, freeServicesQuery/)
  })

  test('四类商品接入独立 sentinel，续页失败保留已加载卡片并允许重试', () => {
    assert.equal((marketPageSource.match(/<InfiniteScrollSentinel\b/g) ?? []).length, 4)
    assert.match(marketPageSource, /flattenUniqueCursorPages\(quotaQuery\.data\.value\?\.pages\)/)
    assert.match(marketPageSource, /flattenUniqueCursorPages\(rushQuery\.data\.value\?\.pages\)/)
    assert.match(marketPageSource, /flattenUniqueCursorPages\(freeServicesQuery\.data\.value\?\.pages\)/)
    assert.match(marketPageSource, /const quotaHasLoadedPages = computed\(\(\) => Boolean\(quotaQuery\.data\.value\?\.pages\.length\)\)/)
    assert.match(marketPageSource, /const rushHasLoadedPages = computed\(\(\) => Boolean\(rushQuery\.data\.value\?\.pages\.length\)\)/)
    assert.match(marketPageSource, /const servicesHaveLoadedPages = computed\(\(\) => Boolean\(freeServicesQuery\.data\.value\?\.pages\.length\)\)/)
    assert.match(marketPageSource, /quotaQuery\.error\.value && !quotaHasLoadedPages/)
    assert.match(marketPageSource, /rushQuery\.error\.value && !rushHasLoadedPages/)
    assert.match(marketPageSource, /freeServicesQuery\.error\.value && !servicesHaveLoadedPages/)
    assert.equal((marketPageSource.match(/:error="[^"]+\.isFetchNextPageError\.value"/g) ?? []).length, 4)
    assert.equal((marketPageSource.match(/@retry="[^"]+\.fetchNextPage\(\)"/g) ?? []).length, 4)
  })

  test('sentinel 只在接近视口且可续页时加载，并显示加载、失败和结束状态', () => {
    assert.match(sentinelSource, /rootMargin: '400px 0px'/)
    assert.match(sentinelSource, /isVisible && hasMore && !loading && !error/)
    assert.match(sentinelSource, /正在加载更多/)
    assert.match(sentinelSource, /重试加载/)
    assert.match(sentinelSource, /v-else-if="!hasMore">已加载全部/)
    assert.match(sentinelSource, /:aria-busy="loading"/)
  })
})
