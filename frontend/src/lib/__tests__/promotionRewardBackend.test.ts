import { describe, expect, it } from 'vitest'
import {
  defaultAdminPromotionCouponQuery,
  defaultAdminReferralQuery,
  defaultPromotionCouponQuery,
} from '@/lib/promotionRewardBackend'

describe('推广权益真实后端契约', () => {
  it('所有分页查询以服务端默认值开始', () => {
    expect(defaultPromotionCouponQuery).toEqual({ page: 1, limit: 20, status: 'all' })
    expect(defaultAdminReferralQuery).toEqual({ page: 1, limit: 20, status: 'all', search: '' })
    expect(defaultAdminPromotionCouponQuery).toEqual({
      page: 1,
      limit: 20,
      status: 'all',
      source: 'all',
      search: '',
    })
  })

  it('适配器不包含真实模式 mock 回退', async () => {
    const source = await import('node:fs/promises').then(fs => fs.readFile(
      new URL('../promotionRewardBackend.ts', import.meta.url),
      'utf8',
    ))
    expect(source).not.toContain('catch')
    expect(source).not.toContain('mock')
    expect(source).toContain("'/api/v1/me/referral'")
    expect(source).toContain("'/api/v1/admin/promotion-reward-campaign'")
  })
})
