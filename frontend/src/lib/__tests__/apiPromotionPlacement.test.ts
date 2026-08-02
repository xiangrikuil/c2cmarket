import { describe, expect, it } from 'vitest'
import { apiServices, type ApiService } from '@/data/mock'
import type { ApiServicePromotion } from '@/lib/apiMarketBackend'
import { firstPromotionForBillingMode, placePromotionFirst } from '@/lib/apiPromotionPlacement'

function service(id: string, billingMode: ApiService['billingMode']): ApiService {
  return { ...structuredClone(apiServices[0]!), id, billingMode }
}

function promotion(id: string, promotedService: ApiService): ApiServicePromotion {
  return {
    promotionId: id,
    placement: 'api_market_top',
    label: '推广',
    startsAt: '2026-08-02T00:00:00Z',
    endsAt: '2026-08-09T00:00:00Z',
    service: promotedService,
  }
}

describe('API 市场分类内推广排序', () => {
  it('每天后端顺序中每个分类只选择第一条匹配推广', () => {
    const freeFirst = promotion('promotion-free-first', service('free-first', 'metered_usd_quota'))
    const packageFirst = promotion('promotion-package-first', service('package-first', 'fixed_package'))
    const freeSecond = promotion('promotion-free-second', service('free-second', 'manual_credit'))

    expect(firstPromotionForBillingMode([freeFirst, packageFirst, freeSecond], false)?.promotionId).toBe('promotion-free-first')
    expect(firstPromotionForBillingMode([freeFirst, packageFirst, freeSecond], true)?.promotionId).toBe('promotion-package-first')
  })

  it('跳过不符合当前套餐筛选的推广并选择下一条匹配活动', () => {
    const packageFirst = promotion('promotion-package-first', service('package-first', 'fixed_package'))
    const packageSecond = promotion('promotion-package-second', service('package-second', 'fixed_package'))

    expect(firstPromotionForBillingMode(
      [packageFirst, packageSecond],
      true,
      item => item.service.id === 'package-second',
    )?.promotionId).toBe('promotion-package-second')
  })

  it('将推广移到首位并按服务 ID 从后续自然结果去重', () => {
    const promoted = promotion('promotion-free', service('service-promoted', 'metered_usd_quota'))
    const rows = [
      { serviceId: 'service-natural-1', promotion: undefined as ApiServicePromotion | undefined },
      { serviceId: 'service-promoted', promotion: undefined as ApiServicePromotion | undefined },
      { serviceId: 'service-natural-2', promotion: undefined as ApiServicePromotion | undefined },
    ]

    const result = placePromotionFirst(
      rows,
      promoted,
      items => items.find(item => item.serviceId === promoted.service.id),
      item => item.serviceId,
    )

    expect(result.map(item => item.serviceId)).toEqual(['service-promoted', 'service-natural-1', 'service-natural-2'])
    expect(result.filter(item => item.promotion).map(item => item.promotion?.promotionId)).toEqual(['promotion-free'])
  })

  it('推广服务不在自然结果时可注入自由额度，筛选不匹配时可保持套餐自然结果', () => {
    const promoted = promotion('promotion-free', service('service-promoted', 'metered_usd_quota'))
    const rows = [{ serviceId: 'service-natural', promotion: undefined as ApiServicePromotion | undefined }]
    const injected = placePromotionFirst(
      rows,
      promoted,
      () => ({ serviceId: promoted.service.id, promotion: undefined }),
      item => item.serviceId,
    )
    const filteredPackageRows = placePromotionFirst(rows, promoted, () => undefined, item => item.serviceId)

    expect(injected.map(item => item.serviceId)).toEqual(['service-promoted', 'service-natural'])
    expect(filteredPackageRows).toEqual(rows)
  })
})
