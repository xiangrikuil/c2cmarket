import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createAPIModel,
  createAPIModelProvider,
  getAdminAPIModels,
  getAdminAPIModelProviders,
  setAPIModelActive,
  setAPIModelProviderActive,
  updateAPIModel,
  updateAPIModelProvider,
} from '@/lib/apiModelCatalogBackend'
import type { ApiModelInput, ApiModelProviderInput } from '@/types/apiModelCatalog'

export const apiModelCatalogQueryKeys = {
  adminProviders: ['admin-api-model-providers'] as const,
  adminModels: ['admin-api-models'] as const,
  publicActiveModels: ['model-catalog', 'active'] as const,
}

export function useAdminAPIModelProviders() {
  return useQuery({
    queryKey: apiModelCatalogQueryKeys.adminProviders,
    queryFn: getAdminAPIModelProviders,
    retry: false,
  })
}

export function useAdminAPIModels() {
  return useQuery({
    queryKey: apiModelCatalogQueryKeys.adminModels,
    queryFn: getAdminAPIModels,
    retry: false,
  })
}

export function useCreateAPIModelProvider() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ApiModelProviderInput) => createAPIModelProvider(input),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

export function useUpdateAPIModelProvider() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: ApiModelProviderInput }) => updateAPIModelProvider(id, input),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

export function useSetAPIModelProviderActive() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, active }: { id: string, active: boolean }) => setAPIModelProviderActive(id, active),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

export function useCreateAPIModel() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ApiModelInput) => createAPIModel(input),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

export function useUpdateAPIModel() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: ApiModelInput }) => updateAPIModel(id, input),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

export function useSetAPIModelActive() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, active }: { id: string, active: boolean }) => setAPIModelActive(id, active),
    onSuccess() {
      invalidateAPIModelCatalog(queryClient)
    },
  })
}

function invalidateAPIModelCatalog(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: apiModelCatalogQueryKeys.adminProviders })
  queryClient.invalidateQueries({ queryKey: apiModelCatalogQueryKeys.adminModels })
  queryClient.invalidateQueries({ queryKey: apiModelCatalogQueryKeys.publicActiveModels })
}
