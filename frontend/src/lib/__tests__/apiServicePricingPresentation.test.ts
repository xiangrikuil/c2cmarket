import { describe, expect, it } from 'vitest'
import type { ApiService, ApiServicePackage } from '@/data/mock'
import {
  getApiServicePricePresentation,
} from '@/lib/apiServicePricingPresentation'

const packageItem = (overrides: Partial<ApiServicePackage> = {}): ApiServicePackage => ({
  id: 'package-1',
  name: '3 天套餐',
  priceCny: 9.9,
  panelAllowance: 5,
  durationDays: 3,
  stockTotal: 10,
  stockAvailable: 4,
  description: '测试套餐',
  enabled: true,
  sortOrder: 0,
  models: [],
  quotaUsagePolicy: {
    fiveHour: { mode: 'unlimited', amountUsd: null },
    daily: { mode: 'unlimited', amountUsd: null },
    scope: 'per_buyer_credential',
    dailyReset: 'utc_plus_8_calendar_day',
  },
  ...overrides,
})

const service = (overrides: Partial<ApiService> = {}) => ({
  billingMode: 'metered_credit',
  creditPerCny: 1.25,
  cnyPerUsdAllowance: '0.8000',
  minimumPurchaseCny: 10,
  packages: [],
  ...overrides,
} as ApiService)

describe('API service price presentation', () => {
  it('keeps the USD unit price for metered services', () => {
    expect(getApiServicePricePresentation(service())).toMatchObject({
      fixedPackage: false,
      label: '购买价格',
      value: '¥0.80 / $1',
      secondary: '最低 ¥10 起购',
      minimumPriceCny: 10,
    })
  })

  it('uses package prices instead of the compatibility USD unit price', () => {
    const presentation = getApiServicePricePresentation(service({
      billingMode: 'fixed_package',
      creditPerCny: 1,
      cnyPerUsdAllowance: '1.0000',
      packages: [
        packageItem(),
        packageItem({ id: 'package-2', priceCny: 20, stockAvailable: 2 }),
        packageItem({ id: 'package-3', priceCny: 1, stockAvailable: 0 }),
      ],
    }))

    expect(presentation).toEqual({
      fixedPackage: true,
      label: '套餐价格',
      value: '¥9.9–¥20',
      secondary: '2 款套餐 · 剩余 6 份',
      minimumPriceCny: 9.9,
      packageCount: 2,
      stockAvailable: 6,
    })
  })

  it('shows the selected package price and package facts on detail pages', () => {
    const selected = packageItem()
    expect(getApiServicePricePresentation(service({
      billingMode: 'fixed_package',
      packages: [selected],
    }), selected)).toMatchObject({
      label: '套餐价格',
      value: '¥9.9',
      secondary: '3 天 · 面板额度 $5',
    })
  })

  it('does not fall back to a USD unit price when no package is orderable', () => {
    expect(getApiServicePricePresentation(service({
      billingMode: 'fixed_package',
      cnyPerUsdAllowance: '1.0000',
      packages: [packageItem({ stockAvailable: 0 })],
    }))).toMatchObject({
      value: '暂无可售套餐',
      secondary: '当前没有可购买的套餐',
      minimumPriceCny: null,
    })
  })
})
