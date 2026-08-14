import { computed, unref, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  applyAPIOrderSanction,
  createDisputeReputationOutcome,
  createReputationRestriction,
  getAdminUserReputation,
  getAPIOrderSanctionRecommendation,
  getMyReputation,
  getPublicUserReputation,
  getReputationRules,
  getSourceAuthorVerification,
  recalculateAllReputation,
  recalculateUserReputation,
  revokeReputationRestriction,
  updateSourceAuthorVerification,
} from '@/lib/reputationApi'
import type {
  ApplyAPIOrderSanctionInput,
  CreateDisputeOutcomeInput,
  CreateReputationRestrictionInput,
  ReputationScope,
  UpdateSourceAuthorVerificationInput,
} from '@/types/reputation'

type MaybeRef<T> = T | Ref<T>

function valueOf<T>(value: MaybeRef<T>) {
  return unref(value)
}

export const reputationQueryKeys = {
  all: ['reputation'] as const,
  rules: () => ['reputation', 'rules'] as const,
  my: () => ['reputation', 'my'] as const,
  public: (username: string, scope: ReputationScope) => ['reputation', 'public', username, scope] as const,
  admin: (userId: string, historyLimit: number) => ['reputation', 'admin', userId, historyLimit] as const,
  sanction: (disputeCaseId: string) => ['reputation', 'sanction', disputeCaseId] as const,
  sourceAuthor: (resourceType: 'carpool' | 'api_service', resourceId: string) =>
    ['reputation', 'source-author', resourceType, resourceId] as const,
}

export function useReputationRulesQuery() {
  return useQuery({
    queryKey: reputationQueryKeys.rules(),
    queryFn: getReputationRules,
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
  })
}

export function useMyReputationQuery() {
  return useQuery({
    queryKey: reputationQueryKeys.my(),
    queryFn: getMyReputation,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function usePublicUserReputationQuery(
  username: MaybeRef<string>,
  scope: MaybeRef<ReputationScope>,
) {
  return useQuery({
    queryKey: computed(() => reputationQueryKeys.public(valueOf(username), valueOf(scope))),
    queryFn: () => getPublicUserReputation(valueOf(username), valueOf(scope)),
    enabled: computed(() => Boolean(valueOf(username))),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useAdminUserReputationQuery(
  userId: MaybeRef<string>,
  options: { enabled?: MaybeRef<boolean>, historyLimit?: number } = {},
) {
  const historyLimit = options.historyLimit ?? 50
  return useQuery({
    queryKey: computed(() => reputationQueryKeys.admin(valueOf(userId), historyLimit)),
    queryFn: () => getAdminUserReputation(valueOf(userId), historyLimit),
    enabled: computed(() => Boolean(valueOf(userId)) && (options.enabled === undefined || valueOf(options.enabled))),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })
}

export function useAPIOrderSanctionRecommendationQuery(
  disputeCaseId: MaybeRef<string>,
  enabled: MaybeRef<boolean> = true,
) {
  return useQuery({
    queryKey: computed(() => reputationQueryKeys.sanction(valueOf(disputeCaseId))),
    queryFn: () => getAPIOrderSanctionRecommendation(valueOf(disputeCaseId)),
    enabled: computed(() => Boolean(valueOf(disputeCaseId)) && valueOf(enabled)),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })
}

export function useApplyAPIOrderSanctionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ApplyAPIOrderSanctionInput) => applyAPIOrderSanction(input),
    onSuccess(_data, input) {
      queryClient.invalidateQueries({ queryKey: reputationQueryKeys.sanction(input.disputeCaseId) })
      queryClient.invalidateQueries({ queryKey: reputationQueryKeys.my() })
      if (input.subjectUserId) queryClient.invalidateQueries({ queryKey: ['reputation', 'admin', input.subjectUserId] })
      queryClient.invalidateQueries({ queryKey: ['admin-dispute-resolution', input.disputeCaseId] })
      queryClient.invalidateQueries({ queryKey: ['admin-section', 'reports'] })
    },
  })
}

export function useRecalculateUserReputationMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) => recalculateUserReputation(userId),
    onSuccess(_data, userId) {
      queryClient.invalidateQueries({ queryKey: ['reputation', 'admin', userId] })
      queryClient.invalidateQueries({ queryKey: reputationQueryKeys.my() })
      queryClient.invalidateQueries({ queryKey: ['reputation', 'public'] })
    },
  })
}

export function useRecalculateAllReputationMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: recalculateAllReputation,
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: reputationQueryKeys.all })
    },
  })
}

export function useCreateReputationRestrictionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateReputationRestrictionInput) => createReputationRestriction(input),
    onSuccess(_data, input) {
      queryClient.invalidateQueries({ queryKey: ['reputation', 'admin', input.userId] })
      queryClient.invalidateQueries({ queryKey: ['admin-section', 'users'] })
    },
  })
}

export function useRevokeReputationRestrictionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { userId: string, restrictionId: string, version: number, reason: string }) =>
      revokeReputationRestriction(input.restrictionId, input.version, input.reason),
    onSuccess(_data, input) {
      queryClient.invalidateQueries({ queryKey: ['reputation', 'admin', input.userId] })
    },
  })
}

export function useCreateDisputeReputationOutcomeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateDisputeOutcomeInput) => createDisputeReputationOutcome(input),
    onSuccess(_data, input) {
      queryClient.invalidateQueries({ queryKey: ['reputation', 'admin', input.subjectUserId] })
      queryClient.invalidateQueries({ queryKey: ['admin-section', 'reports'] })
    },
  })
}

export function useSourceAuthorVerificationQuery(
  resourceType: MaybeRef<'carpool' | 'api_service'>,
  resourceId: MaybeRef<string>,
  enabled: MaybeRef<boolean> = true,
) {
  return useQuery({
    queryKey: computed(() => reputationQueryKeys.sourceAuthor(valueOf(resourceType), valueOf(resourceId))),
    queryFn: () => getSourceAuthorVerification(valueOf(resourceType), valueOf(resourceId)),
    enabled: computed(() => Boolean(valueOf(resourceId)) && valueOf(enabled)),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })
}

export function useUpdateSourceAuthorVerificationMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: UpdateSourceAuthorVerificationInput) => updateSourceAuthorVerification(input),
    onSuccess(_data, input) {
      queryClient.invalidateQueries({
        queryKey: reputationQueryKeys.sourceAuthor(input.resourceType, input.resourceId),
      })
      queryClient.invalidateQueries({ queryKey: ['reputation', 'admin'] })
    },
  })
}
