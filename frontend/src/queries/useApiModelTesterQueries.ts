import { useMutation, useQuery } from '@tanstack/vue-query'
import {
  discoverAPIModels,
  getAPIModelTesterOrderSources,
  testAPIModel,
} from '@/lib/apiModelTesterFacade'
import type { ApiModelTesterCredentialSource } from '@/types/apiModelTester'

export const apiModelTesterQueryKeys = {
  orderSources: ['api-model-tester', 'order-sources'] as const,
}

export function useAPIModelTesterOrderSources() {
  return useQuery({
    queryKey: apiModelTesterQueryKeys.orderSources,
    queryFn: getAPIModelTesterOrderSources,
    retry: false,
    refetchOnMount: 'always',
  })
}

export function useDiscoverAPIModelsMutation() {
  return useMutation({
    mutationFn: (input: { source: ApiModelTesterCredentialSource, signal?: AbortSignal }) => discoverAPIModels(input.source, input.signal),
  })
}

export function useTestAPIModelMutation() {
  return useMutation({
    mutationFn: (input: { source: ApiModelTesterCredentialSource, model: string, signal?: AbortSignal }) => testAPIModel(input.source, input.model, input.signal),
  })
}
