import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  backendAdminPromotionCoupons,
  backendAdminPromotionRewardCampaign,
  backendAdminReferrals,
  backendApplyPromotionCoupon,
  backendGrantAdminPromotionCoupon,
  backendMyPromotionCoupons,
  backendMyReferralSummary,
  backendPromotionRewardPublicConfig,
  backendRevokeAdminPromotionCoupon,
  backendRevokeAdminReferral,
  backendUpdatePromotionRewardCampaign,
  type AdminPromotionCouponQuery,
  type AdminReferralQuery,
  type PromotionCouponQuery,
} from '@/lib/promotionRewardBackend'

export const promotionRewardQueryKeys = {
  all: ['promotion-rewards'] as const,
  publicConfig: ['promotion-rewards', 'public-config'] as const,
  myReferral: ['promotion-rewards', 'me', 'referral'] as const,
  myCoupons: (query: PromotionCouponQuery) => ['promotion-rewards', 'me', 'coupons', query] as const,
  adminCampaign: ['promotion-rewards', 'admin', 'campaign'] as const,
  adminReferrals: (query: AdminReferralQuery) => ['promotion-rewards', 'admin', 'referrals', query] as const,
  adminCoupons: (query: AdminPromotionCouponQuery) => ['promotion-rewards', 'admin', 'coupons', query] as const,
}

export function usePromotionRewardPublicConfig() {
  return useQuery({
    queryKey: promotionRewardQueryKeys.publicConfig,
    queryFn: backendPromotionRewardPublicConfig,
    staleTime: 60_000,
  })
}

export function useMyReferralSummary(enabled: MaybeRefOrGetter<boolean> = true) {
  return useQuery({
    queryKey: promotionRewardQueryKeys.myReferral,
    queryFn: backendMyReferralSummary,
    enabled: computed(() => toValue(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyPromotionCoupons(query: MaybeRefOrGetter<PromotionCouponQuery>) {
  return useQuery({
    queryKey: computed(() => promotionRewardQueryKeys.myCoupons(toValue(query))),
    queryFn: () => backendMyPromotionCoupons(toValue(query)),
    placeholderData: keepPreviousData,
    refetchOnMount: 'always',
  })
}

export function useApplyPromotionCouponMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: backendApplyPromotionCoupon,
    async onSettled() {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: promotionRewardQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: ['api-service-promotions'] }),
      ])
    },
  })
}

export function useAdminPromotionRewardCampaign() {
  return useQuery({
    queryKey: promotionRewardQueryKeys.adminCampaign,
    queryFn: backendAdminPromotionRewardCampaign,
    refetchOnMount: 'always',
  })
}

export function useAdminReferrals(query: MaybeRefOrGetter<AdminReferralQuery>) {
  return useQuery({
    queryKey: computed(() => promotionRewardQueryKeys.adminReferrals(toValue(query))),
    queryFn: () => backendAdminReferrals(toValue(query)),
    placeholderData: keepPreviousData,
    refetchOnMount: 'always',
  })
}

export function useAdminPromotionCoupons(query: MaybeRefOrGetter<AdminPromotionCouponQuery>) {
  return useQuery({
    queryKey: computed(() => promotionRewardQueryKeys.adminCoupons(toValue(query))),
    queryFn: () => backendAdminPromotionCoupons(toValue(query)),
    placeholderData: keepPreviousData,
    refetchOnMount: 'always',
  })
}

function useAdminPromotionRewardMutation<TInput, TResult>(mutationFn: (input: TInput) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    async onSettled() {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: promotionRewardQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: ['api-service-promotions'] }),
      ])
    },
  })
}

export function useUpdatePromotionRewardCampaignMutation() {
  return useAdminPromotionRewardMutation(backendUpdatePromotionRewardCampaign)
}

export function useRevokeAdminReferralMutation() {
  return useAdminPromotionRewardMutation(backendRevokeAdminReferral)
}

export function useGrantAdminPromotionCouponMutation() {
  return useAdminPromotionRewardMutation(backendGrantAdminPromotionCoupon)
}

export function useRevokeAdminPromotionCouponMutation() {
  return useAdminPromotionRewardMutation(backendRevokeAdminPromotionCoupon)
}
