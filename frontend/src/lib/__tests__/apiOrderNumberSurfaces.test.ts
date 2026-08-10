import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const buyerList = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const merchantList = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const orderDetail = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const adminList = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
const apiFacade = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')

describe('API order number surfaces', () => {
  it('uses the public business number for buyer and merchant display while preserving UUID routes', () => {
    expect(buyerList).toContain(':value="item.orderNo" full copyable')
    expect(merchantList).toContain(':value="item.orderNo" full copyable')
    expect(orderDetail).toContain(':value="order.orderNo" full copyable')
    expect(orderDetail).toContain('flex min-w-0 flex-wrap items-center')
    expect(orderDetail).not.toContain('truncate text-sm text-muted-foreground" :title="order.serviceTitle"')
    expect(buyerList).toContain('router.push(`/my/api-orders/${id}`)')
    expect(merchantList).toContain(':to="`/merchant/api-orders/${item.id}`"')
  })

  it('uses normalized order search in buyer, merchant, and administrator views', () => {
    expect(buyerList).toContain('search: keyword.value.trim() || undefined')
    expect(merchantList).toContain('search: keyword.value.trim() || undefined')
    expect(adminList).toContain('q: keyword.value.trim() || undefined')
    expect(buyerList).toContain('useMyApiOrdersPage(pageFilters, pageRequest)')
    expect(merchantList).toContain('useMerchantApiOrdersPage(pageFilters, pageRequest)')
    expect(adminList).toContain('useAdminSectionRowsPage(section, pageFilters, pageRequest, supportsServerPagination)')
  })

  it('persists public numbers assigned to legacy mock orders', () => {
    expect(apiFacade).toContain('const loadedApiOrders = readSessionStore<ApiOrder[]>(apiOrderStorageKey, [])')
    expect(apiFacade).toContain('if (loadedApiOrders.some(order => !order.orderNo)) persistApiOrderStore()')
  })
})
