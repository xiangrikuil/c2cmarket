import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createOwnerAPIProbeConnection,
  deleteOwnerAPIProbeConnection,
  getOwnerAPIProbeConnection,
  getOwnerAPIProbeConnections,
  updateOwnerAPIProbeConnection,
  verifyOwnerAPIProbeConnection,
} from '@/lib/apiHealthFacade'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export const apiHealthQueryKeys = {
  all: ['api-probe-connections'] as const,
  list: () => ['api-probe-connections', 'owner'] as const,
  detail: (id: string) => ['api-probe-connections', 'owner', id] as const,
}

export function useOwnerAPIProbeConnections(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: apiHealthQueryKeys.list(),
    queryFn: getOwnerAPIProbeConnections,
    enabled: computed(() => valueOf(enabled)),
    retry: false,
    refetchOnMount: 'always',
  })
}

export function useOwnerAPIProbeConnection(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => apiHealthQueryKeys.detail(valueOf(id))),
    queryFn: () => getOwnerAPIProbeConnection(valueOf(id)),
    enabled: computed(() => Boolean(valueOf(id))),
    retry: false,
  })
}

function invalidateConnections(queryClient: ReturnType<typeof useQueryClient>, id?: string) {
  const requests = [
    queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.all }),
    queryClient.invalidateQueries({ queryKey: ['my-api-services'] }),
    queryClient.invalidateQueries({ queryKey: ['api-services'] }),
  ]
  if (id) requests.push(queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.detail(id) }))
  return Promise.all(requests)
}

export function useCreateOwnerAPIProbeConnectionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createOwnerAPIProbeConnection,
    onSuccess(connection) {
      queryClient.setQueryData(apiHealthQueryKeys.detail(connection.id), connection)
      return invalidateConnections(queryClient, connection.id)
    },
  })
}

export function useUpdateOwnerAPIProbeConnectionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateOwnerAPIProbeConnection,
    onSuccess(connection) {
      queryClient.setQueryData(apiHealthQueryKeys.detail(connection.id), connection)
      return invalidateConnections(queryClient, connection.id)
    },
    onError(_error, input) {
      return invalidateConnections(queryClient, input.id)
    },
  })
}

export function useDeleteOwnerAPIProbeConnectionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: deleteOwnerAPIProbeConnection,
    onSuccess(_data, input) {
      queryClient.removeQueries({ queryKey: apiHealthQueryKeys.detail(input.id) })
      return invalidateConnections(queryClient)
    },
    onError(_error, input) {
      return invalidateConnections(queryClient, input.id)
    },
  })
}

export function useVerifyOwnerAPIProbeConnectionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: verifyOwnerAPIProbeConnection,
    onSuccess(connection) {
      queryClient.setQueryData(apiHealthQueryKeys.detail(connection.id), connection)
      return invalidateConnections(queryClient, connection.id)
    },
    onError(_error, input) {
      return invalidateConnections(queryClient, input.id)
    },
  })
}
