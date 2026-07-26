import { describe, expect, it, vi } from 'vitest'
import type { DemandRecord } from '@/features/demand/types'
import type { ApiOrder, ApiService, Carpool, CarpoolApplication, UserProfile } from '@/lib/api'

function createStorage(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: key => store.get(key) ?? null,
    key: index => Array.from(store.keys())[index] ?? null,
    removeItem: key => store.delete(key),
    setItem: (key, value) => store.set(key, value),
  }
}

vi.stubGlobal('window', {
  sessionStorage: createStorage(),
  localStorage: createStorage(),
  setTimeout: globalThis.setTimeout,
})

const {
  buildAccountCompleteness,
  buildPendingTasks,
  buildPublishedContent,
  countActivePublishedContent,
  dashboardTimestamp,
  getPrimaryAccountAlert,
  uniqueRelatedApiOrderCount,
} = await import('@/lib/personalCenterDashboard')

function carpoolApplication(id: string, status: CarpoolApplication['status'], updatedAt: string): CarpoolApplication {
  return {
    id,
    status,
    updatedAt,
    snapshot: { productName: `车源 ${id}` },
  } as CarpoolApplication
}

function apiOrder(id: string, status: ApiOrder['status'], updatedAt: string): ApiOrder {
  return {
    id,
    status,
    updatedAt,
    serviceTitle: `服务 ${id}`,
  } as ApiOrder
}

function profile(overrides: Partial<UserProfile> = {}): UserProfile {
  return {
    displayName: 'orbit',
    bio: 'API 服务与订阅拼车',
    emailVerified: true,
    passwordConfigured: true,
    linuxDoBinding: { bound: true },
    ...overrides,
  } as UserProfile
}

describe('个人中心待办聚合', () => {
  it('只保留当前买家或商户需要行动的真实状态', () => {
    const tasks = buildPendingTasks({
      buyerCarpoolApplications: [
        carpoolApplication('ride-buyer-action', 'accepted_reserved', '2026-07-26 09:00'),
        carpoolApplication('ride-buyer-wait', 'joined_pending_confirmation', '2026-07-26 10:00'),
      ],
      ownerCarpoolApplications: [
        carpoolApplication('ride-owner-action', 'pending_owner', '2026-07-26 11:00'),
        carpoolApplication('ride-owner-wait', 'accepted_reserved', '2026-07-26 12:00'),
      ],
      buyerApiOrders: [
        apiOrder('order-buyer-action', 'payment_issue', '2026-07-26 13:00'),
        apiOrder('order-buyer-wait', 'payment_submitted', '2026-07-26 14:00'),
      ],
      merchantApiOrders: [
        apiOrder('order-merchant-action', 'paid_confirmed', '2026-07-26 15:00'),
        apiOrder('order-merchant-wait', 'pending_payment', '2026-07-26 16:00'),
      ],
    })

    expect(tasks.map(item => item.id)).toEqual([
      'order-buyer-action',
      'order-merchant-action',
      'ride-owner-action',
      'ride-buyer-action',
    ])
    expect(tasks.find(item => item.id === 'ride-owner-action')?.to).toBe('/merchant/carpool-applications/ride-owner-action')
    expect(tasks.find(item => item.id === 'order-buyer-action')?.to).toBe('/my/api-orders/order-buyer-action')
  })

  it('先按业务优先级再按更新时间排序', () => {
    const tasks = buildPendingTasks({
      buyerCarpoolApplications: [
        carpoolApplication('dispute-old', 'disputed', '2026-07-25 10:00'),
        carpoolApplication('dispute-new', 'disputed', '2026-07-26 10:00'),
        carpoolApplication('buyer-payment-later', 'accepted_reserved', '2026-07-26 12:00'),
      ],
      ownerCarpoolApplications: [],
      buyerApiOrders: [apiOrder('payment-issue', 'payment_issue', '2026-07-26 11:00')],
      merchantApiOrders: [],
    })

    expect(tasks.map(item => item.id)).toEqual([
      'dispute-new',
      'dispute-old',
      'payment-issue',
      'buyer-payment-later',
    ])
  })
})

describe('个人中心发布内容投影', () => {
  it('合并真实对象、按更新时间排序并统计发布中对象', () => {
    const content = buildPublishedContent({
      carpools: [{
        id: 'carpool-1',
        product: 'ChatGPT Team',
        region: '美国区',
        seats: '剩余 2 位',
        monthly: 120,
        status: '可上车',
        confirmedAt: '2026-07-24 10:00',
      } as Carpool],
      apiServices: [{
        id: 'api-1',
        title: 'GPT API 美元额度',
        cnyPerUsdAllowance: '0.8000',
        availableUsdAllowance: '500',
        delivery: 'Sub2API',
        state: 'paused',
        online: false,
        lastOnlineConfirmedAt: '2026-07-26T10:00:00+08:00',
      } as ApiService],
      demands: [{
        id: 'demand-1',
        title: '求 ChatGPT Team 年付位置',
        maxPrice: 120,
        region: '不限',
        require: '个人车主优先',
        status: '匹配中',
        updatedAt: '2026-07-25 10:00',
      } as DemandRecord],
    })

    expect(content.map(item => item.key)).toEqual(['api-service:api-1', 'demand:demand-1', 'carpool:carpool-1'])
    expect(content[0]).toMatchObject({ status: '已暂停', manageTo: '/my/api-services/api-1' })
    expect(countActivePublishedContent(content)).toBe(2)
  })

  it('无效时间排到有效时间之后', () => {
    expect(dashboardTimestamp('not-a-time')).toBe(0)
    expect(dashboardTimestamp('2026-07-26 10:00')).toBeGreaterThan(0)
  })
})

describe('个人中心账户完整度', () => {
  it('普通用户不需要配置 API 收款方式', () => {
    const completeness = buildAccountCompleteness({
      profile: profile(),
      enabledContactCount: 1,
      hasApiServices: false,
      apiPaymentComplete: false,
    })

    expect(completeness.percentage).toBe(100)
    expect(completeness.tasks.map(item => item.id)).not.toContain('api-payment')
    expect(getPrimaryAccountAlert(completeness)).toBeNull()
  })

  it('已发布 API 服务时将收款设置加入分母和提醒', () => {
    const completeness = buildAccountCompleteness({
      profile: profile(),
      enabledContactCount: 1,
      hasApiServices: true,
      apiPaymentComplete: false,
    })

    expect(completeness).toMatchObject({ completedCount: 6, missingCount: 1, percentage: 86 })
    expect(getPrimaryAccountAlert(completeness)).toMatchObject({
      id: 'api-payment',
      to: '/my/contacts',
    })
  })

  it('提醒按身份绑定、邮箱、密码、联系方式和收款设置排序', () => {
    const completeness = buildAccountCompleteness({
      profile: profile({
        emailVerified: false,
        passwordConfigured: false,
        linuxDoBinding: { bound: false } as UserProfile['linuxDoBinding'],
      }),
      enabledContactCount: 0,
      hasApiServices: true,
      apiPaymentComplete: false,
    })

    expect(getPrimaryAccountAlert(completeness)?.id).toBe('linuxdo')
  })

  it('相关 API 订单按 ID 去重', () => {
    const buyerOrders = [apiOrder('shared', 'pending_payment', '2026-07-26 10:00')]
    const merchantOrders = [
      apiOrder('shared', 'payment_submitted', '2026-07-26 11:00'),
      apiOrder('merchant-only', 'paid_confirmed', '2026-07-26 12:00'),
    ]

    expect(uniqueRelatedApiOrderCount(buyerOrders, merchantOrders)).toBe(2)
  })
})
