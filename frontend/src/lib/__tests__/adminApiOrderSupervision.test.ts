import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { filterApiOrderRows, paginateMockAdminAPIOrderRows, type AdminRow, type ApiOrder } from '@/lib/api'
import { apiOrderPageQuery } from '@/lib/apiMarketBackend'
import { normalizeApiOrderAmountFilter } from '@/lib/apiOrderUi'

const adminSectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
const adminShellSource = readFileSync(new URL('../../components/layout/AdminShell.vue', import.meta.url), 'utf8')

describe('API 订单监管', () => {
  it('maps every meaningful filter to the backend query and omits empty/all values', () => {
    expect(apiOrderPageQuery({ search: ' ', buyerId: ' ', sellerId: ' ', serviceId: ' ', dateRange: 'all', dispute: 'all' }, {})).toBe('')

    const query = apiOrderPageQuery({
      search: ' API-20260810 ',
      status: ['pending_payment', 'completed'],
      dateRange: '7d',
      buyerId: ' buyer-id ',
      sellerId: ' seller-id ',
      serviceId: 'service-id',
      dispute: 'active',
      minAmount: ' 10.50 ',
      maxAmount: '20',
      sort: 'amount_asc',
    }, { limit: 20, cursor: 'next-page' })
    const params = new URLSearchParams(query.slice(1))

    expect(Object.fromEntries(params)).toEqual({
      statuses: 'pending_payment,completed',
      buyerId: 'buyer-id',
      sellerId: 'seller-id',
      serviceId: 'service-id',
      q: 'API-20260810',
      dateRange: '7d',
      sort: 'amount_asc',
      dispute: 'active',
      minAmount: '10.50',
      maxAmount: '20',
      limit: '20',
      cursor: 'next-page',
    })
  })

  it('applies exact mock order filters before pagination and binds cursors to sort', async () => {
    const now = new Date('2026-08-10T12:00:00+08:00')
    const orders = [
      { id: 'order-c', orderNo: 'API-20260810-K7M4-P9Q2XZ', apiServiceId: 'service-1', serviceTitle: '目标服务', buyerId: 'buyer-1', buyer: '买家一', sellerId: 'seller-1', seller: '商户一', status: 'pending_payment', disputeStatus: 'none', amount: 20, amountDecimal: '20.00', intentSnapshot: { merchant: '商户一' }, createdAt: '2026-08-10T10:00:00+08:00', updatedAt: '2026-08-10T10:00:00+08:00' },
      { id: 'order-b', orderNo: 'API-20260809-AAAAAAAAAA', apiServiceId: 'service-2', serviceTitle: '其他服务', buyerId: 'buyer-2', buyer: '买家二', sellerId: 'seller-2', seller: '商户二', status: 'completed', disputeStatus: 'open', amount: 10, amountDecimal: '10.00', createdAt: '2026-08-09T10:00:00+08:00', updatedAt: '2026-08-09T10:00:00+08:00' },
      { id: 'order-a', orderNo: 'API-20260808-BBBBBBBBBB', apiServiceId: 'service-3', serviceTitle: '其他服务', buyerId: 'buyer-3', buyer: '买家三', sellerId: 'seller-3', seller: '商户三', status: 'completed', disputeStatus: 'none', amount: 10, amountDecimal: '10.00', createdAt: '2026-08-08T10:00:00+08:00', updatedAt: '2026-08-08T10:00:00+08:00' },
    ] as ApiOrder[]

    const filtered = filterApiOrderRows(orders, {
      search: 'k7m4p9q2xz',
      status: 'pending_payment',
      buyerId: 'buyer-1',
      sellerId: 'seller-1',
      serviceId: 'service-1',
      dispute: 'none',
      minAmount: '20',
      maxAmount: '20.00',
      sort: 'amount_asc',
    }, now)
    expect(filtered.map(item => item.id)).toEqual(['order-c'])
    expect(filterApiOrderRows(orders, { sort: 'amount_asc' }, now).map(item => item.id)).toEqual(['order-a', 'order-b', 'order-c'])

    const rows = orders.map(order => ({ id: order.id } as AdminRow))
    const first = paginateMockAdminAPIOrderRows(rows, { limit: 1 }, 'updated_desc')
    expect(first.nextCursor).toBeTruthy()
    expect(() => paginateMockAdminAPIOrderRows(rows, { limit: 1, cursor: first.nextCursor }, 'amount_asc')).toThrow('分页 cursor 无效。')
  })

  it('uses the Shanghai calendar-day boundary and decimal strings in Mock mode', () => {
    const now = new Date('2026-08-09T16:30:00.000Z')
    const orders = [
      { id: 'later', amount: 9007199254740992, amountDecimal: '9007199254740992.20', createdAt: '2026-08-09T16:01:00.000Z', updatedAt: '2026-08-09T16:01:00.000Z' },
      { id: 'earlier', amount: 9007199254740992, amountDecimal: '9007199254740992.10', createdAt: '2026-08-09T15:59:00.000Z', updatedAt: '2026-08-09T15:59:00.000Z' },
    ] as ApiOrder[]

    expect(filterApiOrderRows(orders, { dateRange: 'today', sort: 'amount_asc' }, now).map(item => item.id)).toEqual(['later'])
    expect(filterApiOrderRows(orders, { minAmount: '9007199254740992.15', sort: 'amount_asc' }, now).map(item => item.id)).toEqual(['later'])
    expect(filterApiOrderRows(orders, { sort: 'amount_asc' }, now).map(item => item.id)).toEqual(['earlier', 'later'])
  })

  it('normalizes text and numeric amount inputs without assuming trim is available', () => {
    expect(normalizeApiOrderAmountFilter(' 10.50 ')).toBe('10.50')
    expect(normalizeApiOrderAmountFilter(100.01)).toBe('100.01')
    expect(normalizeApiOrderAmountFilter(null)).toBe('')
    expect(adminSectionSource).not.toContain('orderMinAmount.value.trim()')
    expect(adminSectionSource).not.toContain('orderMaxAmount.value.trim()')
  })

  it('renders the dedicated controls under the preserved admin route and updates visible naming', () => {
    for (const expected of [
      'API 订单监管',
      'orderStatus',
      'orderDateRange',
      'orderBuyerId',
      'orderSellerId',
      'orderServiceId',
      'orderDispute',
      'orderMinAmount',
      'orderMaxAmount',
      'orderSort',
      'clearOrderFilters',
      '清空筛选',
    ]) {
      expect(adminSectionSource).toContain(expected)
    }
    expect(routerSource).toContain("['trade-intents', 'API 订单监管'")
    expect(adminShellSource).toContain("to: '/admin/trade-intents'")
    expect(adminShellSource).toContain("label: 'API 订单监管'")
  })
})
