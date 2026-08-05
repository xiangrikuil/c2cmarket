import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createAPIHealthChallenge,
  deleteOwnerAPIHealthProbe,
  getAdminAPIHealthProbes,
  getOwnerAPIHealthProbe,
  reviewAPIHealthProbe,
  saveOwnerAPIHealthProbe,
  verifyAPIHealthChallenge,
} from '@/lib/apiHealthFacade'
import type { ApiHealthAuthorizationStatus } from '@/types/apiHealth'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export const apiHealthQueryKeys = {
  all: ['api-health-probes'] as const,
  owner: (apiServiceId: string) => ['api-health-probes', 'owner', apiServiceId] as const,
  admin: (status: ApiHealthAuthorizationStatus) => ['api-health-probes', 'admin', status] as const,
}

export function useOwnerAPIHealthProbe(apiServiceId: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => apiHealthQueryKeys.owner(valueOf(apiServiceId))),
    queryFn: () => getOwnerAPIHealthProbe(valueOf(apiServiceId)),
    enabled: computed(() => Boolean(valueOf(apiServiceId))),
    retry: false,
    refetchOnMount: 'always',
  })
}

function invalidateOwnerProbe(queryClient: ReturnType<typeof useQueryClient>, apiServiceId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.owner(apiServiceId) }),
    queryClient.invalidateQueries({ queryKey: ['my-api-services', apiServiceId] }),
    queryClient.invalidateQueries({ queryKey: ['api-services', apiServiceId] }),
    queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.all }),
  ])
}

export function useSaveOwnerAPIHealthProbeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: saveOwnerAPIHealthProbe,
    onSuccess(config) {
      queryClient.setQueryData(apiHealthQueryKeys.owner(config.apiServiceId), config)
      return invalidateOwnerProbe(queryClient, config.apiServiceId)
    },
    onError(_error, input) {
      return invalidateOwnerProbe(queryClient, input.apiServiceId)
    },
  })
}

export function useDeleteOwnerAPIHealthProbeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: deleteOwnerAPIHealthProbe,
    onSuccess(_data, input) {
      queryClient.setQueryData(apiHealthQueryKeys.owner(input.apiServiceId), null)
      return invalidateOwnerProbe(queryClient, input.apiServiceId)
    },
    onError(_error, input) {
      return invalidateOwnerProbe(queryClient, input.apiServiceId)
    },
  })
}

export function useCreateAPIHealthChallengeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createAPIHealthChallenge,
    onSettled(_data, _error, input) {
      return invalidateOwnerProbe(queryClient, input.apiServiceId)
    },
  })
}

export function useVerifyAPIHealthChallengeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: verifyAPIHealthChallenge,
    onSuccess(config) {
      queryClient.setQueryData(apiHealthQueryKeys.owner(config.apiServiceId), config)
      return invalidateOwnerProbe(queryClient, config.apiServiceId)
    },
    onError(_error, input) {
      return invalidateOwnerProbe(queryClient, input.apiServiceId)
    },
  })
}

export function useAdminAPIHealthProbes(status: Ref<ApiHealthAuthorizationStatus> | ApiHealthAuthorizationStatus) {
  return useQuery({
    queryKey: computed(() => apiHealthQueryKeys.admin(valueOf(status))),
    queryFn: () => getAdminAPIHealthProbes(valueOf(status)),
    refetchOnMount: 'always',
  })
}

export function useReviewAPIHealthProbeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: reviewAPIHealthProbe,
    onSettled() {
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: ['api-services'] }),
      ])
    },
  })
}
