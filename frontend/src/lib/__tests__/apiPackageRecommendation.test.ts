import { describe, expect, it } from 'vitest'
import type { ApiService, ApiServicePackage } from '@/lib/api'
import { rankApiPackages } from '@/lib/apiPackageRecommendation'

const packageRow = (overrides: Partial<ApiServicePackage> = {}): ApiServicePackage => ({
  id: 'package-1',
  name: '3 天 GPT-5.6 套餐',
  priceCny: 10,
  panelAllowance: 5,
  durationDays: 3,
  stockTotal: 5,
  stockAvailable: 5,
  description: '交付后开始计算有效期。',
  enabled: true,
  sortOrder: 0,
  models: [{
    serviceModelId: 'service-model-1',
    modelCatalogId: 'model-1',
    modelPriceVersionId: 'price-version-1',
    modelName: 'GPT-5.6',
    provider: 'OpenAI',
    merchantMultiplier: 1,
  }],
  ...overrides,
})

const service = (id: string, pack: ApiServicePackage, overrides: Partial<ApiService> = {}): ApiService => ({
  id,
  title: id,
  merchantId: id,
  merchantUsername: id,
  merchant: id,
  merchantIdentityMode: 'store_alias',
  merchantDisplayName: id,
  trustLevel: 3,
  merchantType: '商户',
  models: ['GPT-5.6'],
  modelMultipliers: [{ model: 'GPT-5.6', multiplier: '1.00x' }],
  rate: '1.00x',
  defaultMultiplier: 1,
  creditPerCny: 1,
  minimumPurchaseCny: pack.priceCny,
  maxBuy: pack.priceCny,
  balance: 0,
  delivery: 'Sub2API',
  billingMode: 'fixed_package',
  deliveryModes: ['api_key_endpoint'],
  usageVisibility: 'none',
  panelBaseUrl: null,
  imagePricing: { supported: false, textToImage: false, imageToImage: false, oneKPriceUsd: null, twoKPriceUsd: null, fourKPriceUsd: null },
  independentApiKey: true,
  independentPanelAccount: false,
  panelRequiresPasswordReset: false,
  apiBaseUrlVisibility: 'after_intent',
  panelLoginUrlVisibility: 'off_platform',
  state: 'online',
  online: true,
  publiclyOrderable: true,
  lastOnlineConfirmedAt: '2026-07-16T00:00:00Z',
  onlineExpiresAt: '2026-07-19T00:00:00Z',
  expectedResponseMinutes: 10,
  responseMedianMinutes: 10,
  dailyOrderLimit: 10,
  todayOrderCount: 0,
  unresolvedDisputes: 0,
  warranty: '站外协商',
  refundPolicy: '站外协商',
  expiresAt: '交付后 3 天',
  completed30d: 10,
  reviewCount: 0,
  officialPricingVersion: 'test',
  officialPricingUpdatedAt: '2026-07-16T00:00:00Z',
  merchantNote: '',
  modelPriceRows: [],
  packages: [pack],
  recommendationResponseMedianMinutes: 10,
  serviceUpdatedAt: '2026-07-16T00:00:00Z',
  contactChannels: [],
  ...overrides,
})

describe('rankApiPackages', () => {
  it('ranks only matching in-stock packages and favors lower declared unit cost', () => {
    const lowerValue = packageRow({ id: 'higher-cost', priceCny: 20 })
    const betterValue = packageRow({ id: 'better-value', priceCny: 8 })
    const soldOut = packageRow({ id: 'sold-out', stockAvailable: 0 })
    const rows = rankApiPackages([
      service('higher-cost', lowerValue),
      service('better-value', betterValue),
      service('sold-out', soldOut),
    ], 'model-1', 3, new Date('2026-07-16T00:00:00Z'))

    expect(rows.map(row => row.package.id)).toEqual(['better-value', 'higher-cost'])
    expect(rows[0].valueScore).toBe(100)
  })

  it('uses a neutral response score when the merchant has no response history', () => {
    const rows = rankApiPackages([
      service('new-merchant', packageRow(), { recommendationResponseMedianMinutes: null, completed30d: 0 }),
    ], 'model-1', 3, new Date('2026-07-16T00:00:00Z'))

    expect(rows[0].responseScore).toBe(50)
    expect(rows[0].fulfillmentScore).toBe(50)
  })
})
