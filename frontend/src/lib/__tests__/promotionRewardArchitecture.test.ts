import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

describe('promotion reward architecture', () => {
  it('keeps the user benefit workflow on real query adapters with local poster generation', () => {
    const page = readFileSync(new URL('../../pages/MyPromotionBenefitsPage.vue', import.meta.url), 'utf8')
    const queries = readFileSync(new URL('../../queries/usePromotionRewardQueries.ts', import.meta.url), 'utf8')
    const backend = readFileSync(new URL('../promotionRewardBackend.ts', import.meta.url), 'utf8')

    expect(page).toContain('useMyReferralSummary')
    expect(page).toContain('useMyPromotionCoupons')
    expect(page).toContain('useApplyPromotionCouponMutation')
    expect(page).toContain('QRCode.toDataURL(inviteLink.value')
    expect(page).toContain("trackAnalytics('promotion_benefit_action'")
    expect(page).not.toMatch(/trackAnalytics\([^\n]*(?:invite|coupon|service|user)(?:Code|Id|_id)/)
    expect(queries).toContain('backendApplyPromotionCoupon')
    expect(backend).toContain('/api/v1/me/promotion-coupons/')
    expect(backend).not.toMatch(/catch[\s\S]{0,200}mock/i)
  })

  it('keeps administrator campaign, referral, and coupon workflows server-backed', () => {
    const page = readFileSync(new URL('../../pages/AdminGrowthPromotionsPage.vue', import.meta.url), 'utf8')
    const queries = readFileSync(new URL('../../queries/usePromotionRewardQueries.ts', import.meta.url), 'utf8')
    const backend = readFileSync(new URL('../promotionRewardBackend.ts', import.meta.url), 'utf8')

    expect(page).toContain('useAdminPromotionRewardCampaign')
    expect(page).toContain('useAdminReferrals')
    expect(page).toContain('useAdminPromotionCoupons')
    expect(page).toContain('useUpdatePromotionRewardCampaignMutation')
    expect(page).toContain('useGrantAdminPromotionCouponMutation')
    expect(page).toContain('useRevokeAdminReferralMutation')
    expect(page).toContain('useRevokeAdminPromotionCouponMutation')
    expect(page).toContain('class="[&_table]:min-w-[820px]"')
    expect(page).toContain('class="[&_table]:min-w-[980px]"')
    expect(queries).toContain('backendUpdatePromotionRewardCampaign')
    expect(backend).toContain('ifMatch: input.version')
    expect(backend).toContain("idempotencyPrefix: 'promotion-reward-campaign-update'")
    expect(backend).toContain("idempotencyPrefix: 'admin-referral-revoke'")
    expect(backend).toContain("idempotencyPrefix: 'admin-promotion-coupon-grant'")
    expect(backend).toContain("idempotencyPrefix: 'admin-promotion-coupon-revoke'")
  })

  it('registers private user and administrator routes in router and navigation', () => {
    const router = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
    const appShell = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
    const adminShell = readFileSync(new URL('../../components/layout/AdminShell.vue', import.meta.url), 'utf8')

    expect(router).toContain("path: '/my/promotion-benefits'")
    expect(router).toContain("path: '/admin/growth-promotions'")
    expect(appShell).toContain("to: '/my/promotion-benefits'")
    expect(adminShell).toContain("to: '/admin/growth-promotions'")
  })
})
