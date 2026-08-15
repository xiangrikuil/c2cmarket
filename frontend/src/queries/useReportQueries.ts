import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  backendRespondDispute,
  backendWithdrawDispute,
  backendSelfResolveDispute,
  backendClaimDisputeRemedy,
  backendConfirmDisputeRemedy,
  backendContestDisputeRemedy,
  backendCreateAppeal,
  backendMyAppeals,
  backendMyDispute,
  backendMyDisputes,
  backendMyReports,
  backendSubmitInfoSupplement,
  type CreateAppealRequest,
  type SubmitInfoSupplementRequest,
} from '@/lib/reportBackend'

export const myReportsQueryKey = ['my-reports'] as const
export const myDisputesQueryKey = ['my-disputes'] as const
export const myAppealsQueryKey = ['my-appeals'] as const
export const myDisputeQueryKey = (id: string) => ['my-dispute', id] as const

export function useMyReportsQuery() {
  return useQuery({
    queryKey: myReportsQueryKey,
    queryFn: backendMyReports,
    refetchOnMount: 'always',
  })
}

export function useMyDisputesQuery() {
  return useQuery({
    queryKey: myDisputesQueryKey,
    queryFn: backendMyDisputes,
    refetchOnMount: 'always',
  })
}

export function useMyDisputeQuery(id: Ref<string>) {
  return useQuery({
    queryKey: computed(() => myDisputeQueryKey(id.value)),
    queryFn: () => backendMyDispute(id.value),
    enabled: computed(() => Boolean(id.value)),
    refetchOnMount: 'always',
  })
}

function useDisputeMutation<T>(mutationFn: (input: T) => ReturnType<typeof backendMyDispute>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess(data) {
      queryClient.setQueryData(myDisputeQueryKey(data.id), data)
      queryClient.invalidateQueries({ queryKey: myDisputesQueryKey })
      queryClient.invalidateQueries({ queryKey: ['api-orders'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
    },
  })
}

export function useRespondDisputeMutation() {
  return useDisputeMutation(({ disputeId, body, evidenceAssetIds }: { disputeId: string, body: string, evidenceAssetIds?: string[] }) => backendRespondDispute(disputeId, { body, evidenceAssetIds }))
}

export function useWithdrawDisputeMutation() {
  return useDisputeMutation(({ disputeId, reason }: { disputeId: string, reason: string }) => backendWithdrawDispute(disputeId, reason))
}

export function useSelfResolveDisputeMutation() {
  return useDisputeMutation(({ disputeId, reason }: { disputeId: string, reason: string }) => backendSelfResolveDispute(disputeId, reason))
}

export function useClaimDisputeRemedyMutation() {
  return useDisputeMutation(({ disputeId, note, evidenceAssetIds }: { disputeId: string, note: string, evidenceAssetIds?: string[] }) => backendClaimDisputeRemedy(disputeId, { note, evidenceAssetIds }))
}

export function useConfirmDisputeRemedyMutation() {
  return useDisputeMutation(({ disputeId, reason = '' }: { disputeId: string, reason?: string }) => backendConfirmDisputeRemedy(disputeId, reason))
}

export function useContestDisputeRemedyMutation() {
  return useDisputeMutation(({ disputeId, reason, evidenceAssetIds }: { disputeId: string, reason: string, evidenceAssetIds?: string[] }) => backendContestDisputeRemedy(disputeId, { reason, evidenceAssetIds }))
}

export function useMyAppealsQuery() {
  return useQuery({
    queryKey: myAppealsQueryKey,
    queryFn: backendMyAppeals,
    refetchOnMount: 'always',
  })
}

export function useCreateAppealMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateAppealRequest) => backendCreateAppeal(payload),
    onSuccess(data) {
      queryClient.setQueryData(myAppealsQueryKey, (current: Awaited<ReturnType<typeof backendMyAppeals>> | undefined) => {
        if (!current) return [data]
        return [data, ...current.filter(item => item.id !== data.id)]
      })
      queryClient.invalidateQueries({ queryKey: myAppealsQueryKey })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

export function useSubmitInfoSupplementMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SubmitInfoSupplementRequest) => backendSubmitInfoSupplement(payload),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myReportsQueryKey })
      queryClient.invalidateQueries({ queryKey: myDisputesQueryKey })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}
