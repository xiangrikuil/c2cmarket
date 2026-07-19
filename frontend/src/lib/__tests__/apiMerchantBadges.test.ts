import { describe, expect, it } from 'vitest'
import { getApiMerchantBadges } from '@/lib/apiMerchantBadges'

const merchant = (overrides: Record<string, unknown> = {}) => ({
  trustLevel: 3,
  completed30d: 10,
  unresolvedDisputes: 0,
  publiclyOrderable: true,
  responseMedianMinutes: 8,
  recommendationResponseMedianMinutes: 8,
  ...overrides,
})

describe('getApiMerchantBadges', () => {
  it('grants quality and fast-response badges from public fulfillment data', () => {
    expect(getApiMerchantBadges(merchant()).map(item => item.label)).toEqual(['优质商家', '快速响应'])
  })

  it('withholds quality when the merchant has unresolved disputes', () => {
    expect(getApiMerchantBadges(merchant({ unresolvedDisputes: 1 })).map(item => item.label)).toEqual(['快速响应'])
  })

  it('does not infer fast response when response history is explicitly unavailable', () => {
    expect(getApiMerchantBadges(merchant({ recommendationResponseMedianMinutes: null })).map(item => item.label)).toEqual(['优质商家'])
  })

  it('withholds fast response from a service that cannot accept orders', () => {
    expect(getApiMerchantBadges(merchant({ publiclyOrderable: false })).map(item => item.label)).toEqual(['优质商家'])
  })
})
