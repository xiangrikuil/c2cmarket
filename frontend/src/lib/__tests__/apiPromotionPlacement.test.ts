import { describe, expect, it } from 'vitest'
import { apiServices, type ApiService } from '@/data/mock'
import type { ApiServicePromotion } from '@/lib/apiMarketBackend'
import { placePromotions, promotionsForBillingMode } from '@/lib/apiPromotionPlacement'

function service(id: string, billingMode: ApiService['billingMode'] = 'metered_usd_quota'): ApiService {
  return { ...structuredClone(apiServices[0]!), id, billingMode }
}

function promotion(id: string, promotedService: ApiService, kind: ApiServicePromotion['kind']): ApiServicePromotion {
  return {
    promotionId: id,
    kind,
    placement: kind === 'operator' ? 'api_market_top' : 'api_market_reward',
    label: '推广',
    startsAt: '2026-08-02T00:00:00Z',
    endsAt: '2026-08-09T00:00:00Z',
    service: promotedService,
  }
}

type Row = {
  serviceId: string
  promotion?: ApiServicePromotion
  promotionPosition?: 'first' | 'middle' | 'last'
}

const resolveRow = (rows: Row[], item: ApiServicePromotion) => (
  rows.find(row => row.serviceId === item.service.id) ?? { serviceId: item.service.id }
)

describe('API 市场分类内推广排序', () => {
  it('按账单模式和当前筛选分别选择第一条运营与奖励推广', () => {
    const freeOperator = promotion('operator-free', service('free-operator'), 'operator')
    const packageOperator = promotion('operator-package', service('package-operator', 'fixed_package'), 'operator')
    const freeReward = promotion('reward-free', service('free-reward'), 'reward')

    expect(promotionsForBillingMode([freeOperator, packageOperator, freeReward], false)).toEqual({
      operator: freeOperator,
      reward: freeReward,
    })
    expect(promotionsForBillingMode([freeOperator, packageOperator], true).operator).toBe(packageOperator)
    expect(promotionsForBillingMode([packageOperator], true, () => false)).toEqual({ operator: undefined, reward: undefined })
  })

  it('运营推广在首位，奖励推广在三条自然结果后，且自然顺序不变', () => {
    const operator = promotion('operator', service('service-operator'), 'operator')
    const reward = promotion('reward', service('service-reward'), 'reward')
    const rows: Row[] = ['natural-1', 'service-reward', 'natural-2', 'service-operator', 'natural-3', 'natural-4']
      .map(serviceId => ({ serviceId }))

    const result = placePromotions(rows, { operator, reward }, resolveRow, row => row.serviceId)

    expect(result.map(row => row.serviceId)).toEqual([
      'service-operator', 'natural-1', 'natural-2', 'natural-3', 'service-reward', 'natural-4',
    ])
    expect(result.filter(row => !row.promotion).map(row => row.serviceId)).toEqual(['natural-1', 'natural-2', 'natural-3', 'natural-4'])
    expect(result[0]?.promotionPosition).toBe('first')
    expect(result[4]?.promotionPosition).toBe('middle')
  })

  it('短列表把奖励放到末尾且不让两张推广相邻', () => {
    const operator = promotion('operator', service('operator'), 'operator')
    const reward = promotion('reward', service('reward'), 'reward')
    const twoRows = placePromotions<Row>([{ serviceId: 'one' }, { serviceId: 'two' }], { operator, reward }, resolveRow, row => row.serviceId)
    expect(twoRows.map(row => row.serviceId)).toEqual(['operator', 'one', 'two', 'reward'])
    expect(twoRows.at(-1)?.promotionPosition).toBe('last')

    const noNaturalRows = placePromotions<Row>([], { operator, reward }, resolveRow, row => row.serviceId)
    expect(noNaturalRows.map(row => row.serviceId)).toEqual(['operator'])
  })

  it('无运营推广时奖励仍在三条自然结果后，空列表时可单独展示', () => {
    const reward = promotion('reward', service('reward'), 'reward')
    const rows: Row[] = ['one', 'two', 'three', 'four'].map(serviceId => ({ serviceId }))
    expect(placePromotions(rows, { reward }, resolveRow, row => row.serviceId).map(row => row.serviceId))
      .toEqual(['one', 'two', 'three', 'reward', 'four'])
    expect(placePromotions<Row>([], { reward }, resolveRow, row => row.serviceId).map(row => row.serviceId))
      .toEqual(['reward'])
  })

  it('同一服务被异常投放到两个池时只保留运营推广', () => {
    const shared = service('shared')
    const operator = promotion('operator', shared, 'operator')
    const reward = promotion('reward', shared, 'reward')
    const result = placePromotions<Row>([{ serviceId: 'shared' }, { serviceId: 'natural' }], { operator, reward }, resolveRow, row => row.serviceId)
    expect(result.map(row => row.promotion?.promotionId).filter(Boolean)).toEqual(['operator'])
    expect(result.map(row => row.serviceId)).toEqual(['shared', 'natural'])
  })
})
