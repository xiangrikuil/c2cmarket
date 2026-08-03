import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  backendCreateAppeal,
  backendMyAppeals,
  backendMyDisputes,
  backendMyReports,
  type CreateAppealRequest,
} from '@/lib/reportBackend'

export const myReportsQueryKey = ['my-reports'] as const
export const myDisputesQueryKey = ['my-disputes'] as const
export const myAppealsQueryKey = ['my-appeals'] as const

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
