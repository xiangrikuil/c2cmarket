import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  getAdminAPIProbeCalibration,
  getAdminAPIProbeLatencyRules,
  previewAdminAPIProbeLatencyRule,
  publishAdminAPIProbeLatencyRule,
} from '@/lib/apiHealthFacade'
import type { ApiProbeProtocol } from '@/types/apiHealth'

export type ApiProbeCalibrationDimension = {
  model: string
  protocol: ApiProbeProtocol
  environment: string
}

export const adminApiHealthKeys = {
  all: ['admin-api-health'] as const,
  calibration: (value: ApiProbeCalibrationDimension) => ['admin-api-health', 'calibration', value] as const,
  rules: () => ['admin-api-health', 'rules'] as const,
}

export function useAdminAPIProbeCalibration(dimension: Ref<ApiProbeCalibrationDimension>) {
  return useQuery({
    queryKey: computed(() => adminApiHealthKeys.calibration(dimension.value)),
    queryFn: () => getAdminAPIProbeCalibration(dimension.value),
    retry: false,
  })
}

export function useAdminAPIProbeLatencyRules() {
  return useQuery({ queryKey: adminApiHealthKeys.rules(), queryFn: getAdminAPIProbeLatencyRules, retry: false })
}

export function usePreviewAdminAPIProbeLatencyRuleMutation() {
  return useMutation({ mutationFn: previewAdminAPIProbeLatencyRule })
}

export function usePublishAdminAPIProbeLatencyRuleMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: publishAdminAPIProbeLatencyRule,
    onSuccess() {
      return queryClient.invalidateQueries({ queryKey: adminApiHealthKeys.all })
    },
  })
}
