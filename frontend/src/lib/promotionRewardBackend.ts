import type {
  AdminPromotionCoupon,
  AdminPromotionCouponPage,
  AdminReferralRecord,
  ApplyPromotionCouponRequest,
  GrantPromotionCouponRequest,
  PromotionCoupon,
  PromotionCouponPage,
  PromotionCouponSource,
  PromotionCouponStatusValue,
  PromotionRewardCampaign,
  PromotionRewardPublicConfig,
  ReferralPage,
  ReferralStatus,
  ReferralSummary,
  ReviewActionRequest,
  UpdatePromotionRewardCampaignRequest,
} from '@/api/generated/openapi'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

export type PromotionCouponFilter = 'all' | PromotionCouponStatusValue
export type ReferralFilter = 'all' | ReferralStatus

export type PromotionCouponQuery = {
  page: number
  limit: number
  status: PromotionCouponFilter
}

export type AdminReferralQuery = {
  page: number
  limit: number
  status: ReferralFilter
  search: string
}

export type AdminPromotionCouponQuery = PromotionCouponQuery & {
  source: 'all' | PromotionCouponSource
  search: string
}

export const defaultPromotionCouponQuery: PromotionCouponQuery = {
  page: 1,
  limit: 20,
  status: 'all',
}

export const defaultAdminReferralQuery: AdminReferralQuery = {
  page: 1,
  limit: 20,
  status: 'all',
  search: '',
}

export const defaultAdminPromotionCouponQuery: AdminPromotionCouponQuery = {
  ...defaultPromotionCouponQuery,
  source: 'all',
  search: '',
}

function pageParams(query: PromotionCouponQuery) {
  return new URLSearchParams({
    page: String(query.page),
    limit: String(query.limit),
    status: query.status,
  })
}

export async function backendPromotionRewardPublicConfig() {
  return backendRequest<PromotionRewardPublicConfig>('/api/v1/promotion-rewards/public-config')
}

export async function backendMyReferralSummary() {
  await ensureBackendSession()
  return backendRequest<ReferralSummary>('/api/v1/me/referral')
}

export async function backendMyPromotionCoupons(query: PromotionCouponQuery) {
  await ensureBackendSession()
  return backendRequest<PromotionCouponPage>(`/api/v1/me/promotion-coupons?${pageParams(query).toString()}`)
}

export async function backendApplyPromotionCoupon(input: {
  couponId: string
  apiServiceId: string
}) {
  await ensureBackendSession()
  return backendMutation<PromotionCoupon>(
    `/api/v1/me/promotion-coupons/${encodeURIComponent(input.couponId)}/apply`,
    { apiServiceId: input.apiServiceId } satisfies ApplyPromotionCouponRequest,
    { idempotencyPrefix: 'promotion-coupon-apply' },
  )
}

export async function backendAdminPromotionRewardCampaign() {
  await ensureBackendSession('admin', true)
  return backendRequest<PromotionRewardCampaign>('/api/v1/admin/promotion-reward-campaign')
}

export async function backendUpdatePromotionRewardCampaign(input: {
  version: number
  payload: UpdatePromotionRewardCampaignRequest
}) {
  await ensureBackendSession('admin', true)
  return backendMutation<PromotionRewardCampaign>('/api/v1/admin/promotion-reward-campaign', input.payload, {
    method: 'PATCH',
    ifMatch: input.version,
    idempotencyPrefix: 'promotion-reward-campaign-update',
  })
}

export async function backendAdminReferrals(query: AdminReferralQuery) {
  await ensureBackendSession('admin', true)
  const params = new URLSearchParams({
    page: String(query.page),
    limit: String(query.limit),
    status: query.status,
  })
  if (query.search.trim()) params.set('search', query.search.trim())
  return backendRequest<ReferralPage>(`/api/v1/admin/referrals?${params.toString()}`)
}

export async function backendRevokeAdminReferral(input: {
  referralId: string
  version: number
  reason: string
}) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminReferralRecord>(
    `/api/v1/admin/referrals/${encodeURIComponent(input.referralId)}/revoke`,
    { reason: input.reason.trim() } satisfies ReviewActionRequest,
    {
      ifMatch: input.version,
      idempotencyPrefix: 'admin-referral-revoke',
    },
  )
}

export async function backendAdminPromotionCoupons(query: AdminPromotionCouponQuery) {
  await ensureBackendSession('admin', true)
  const params = pageParams(query)
  if (query.source !== 'all') params.set('source', query.source)
  if (query.search.trim()) params.set('search', query.search.trim())
  return backendRequest<AdminPromotionCouponPage>(`/api/v1/admin/promotion-coupons?${params.toString()}`)
}

export async function backendGrantAdminPromotionCoupon(payload: GrantPromotionCouponRequest) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminPromotionCoupon>('/api/v1/admin/promotion-coupons/grant', payload, {
    idempotencyPrefix: 'admin-promotion-coupon-grant',
  })
}

export async function backendRevokeAdminPromotionCoupon(input: {
  couponId: string
  version: number
  reason: string
}) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminPromotionCoupon>(
    `/api/v1/admin/promotion-coupons/${encodeURIComponent(input.couponId)}/revoke`,
    { reason: input.reason.trim() } satisfies ReviewActionRequest,
    {
      ifMatch: input.version,
      idempotencyPrefix: 'admin-promotion-coupon-revoke',
    },
  )
}

export type {
  AdminPromotionCoupon,
  AdminReferralRecord,
  GrantPromotionCouponRequest,
  PromotionCoupon,
  PromotionCouponSource,
  PromotionCouponStatusValue,
  PromotionRewardCampaign,
  PromotionRewardPublicConfig,
  ReferralRecord,
  ReferralStatus,
  ReferralSummary,
  UpdatePromotionRewardCampaignRequest,
} from '@/api/generated/openapi'
