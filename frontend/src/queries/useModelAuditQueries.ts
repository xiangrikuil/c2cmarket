import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import {
  cancelModelAuditRun,
  createModelAuditBaseline,
  createModelAuditMonitor,
  createModelAuditRun,
  createModelAuditTarget,
  deleteModelAuditTarget,
  getModelAuditBaselines,
  getModelAuditMonitors,
  getModelAuditReport,
  getModelAuditRuns,
  getModelAuditTargets,
  updateModelAuditTarget,
} from '@/lib/modelAuditBackend'
import type {
  ModelAuditBaselineInput,
  ModelAuditMonitorInput,
  ModelAuditRunInput,
  ModelAuditTargetInput,
} from '@/types/modelAudit'

export const modelAuditQueryKeys = {
  targets: ['model-audit', 'targets'] as const,
  baselines: ['model-audit', 'baselines'] as const,
  runs: ['model-audit', 'runs'] as const,
  monitors: ['model-audit', 'monitors'] as const,
  report: (runId: string) => ['model-audit', 'report', runId] as const,
}

export function useModelAuditTargets() {
  return useQuery({ queryKey: modelAuditQueryKeys.targets, queryFn: getModelAuditTargets, retry: false })
}

export function useModelAuditBaselines() {
  return useQuery({ queryKey: modelAuditQueryKeys.baselines, queryFn: getModelAuditBaselines, retry: false })
}

export function useModelAuditRuns() {
  return useQuery({ queryKey: modelAuditQueryKeys.runs, queryFn: getModelAuditRuns, retry: false })
}

export function useModelAuditMonitors() {
  return useQuery({ queryKey: modelAuditQueryKeys.monitors, queryFn: getModelAuditMonitors, retry: false })
}

export function useModelAuditReport(runId: MaybeRefOrGetter<string>) {
  return useQuery({
    queryKey: computed(() => modelAuditQueryKeys.report(toValue(runId))),
    queryFn: () => getModelAuditReport(toValue(runId)),
    enabled: computed(() => Boolean(toValue(runId))),
    retry: false,
  })
}

export function useCreateModelAuditTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ModelAuditTargetInput) => createModelAuditTarget(input),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useUpdateModelAuditTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: ModelAuditTargetInput }) => updateModelAuditTarget(id, input),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useDeleteModelAuditTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteModelAuditTarget(id),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useCreateModelAuditBaseline() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ModelAuditBaselineInput) => createModelAuditBaseline(input),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useCreateModelAuditRun() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ModelAuditRunInput) => createModelAuditRun(input),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useCancelModelAuditRun() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => cancelModelAuditRun(id),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

export function useCreateModelAuditMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ModelAuditMonitorInput) => createModelAuditMonitor(input),
    onSuccess: () => invalidateModelAudit(queryClient),
  })
}

function invalidateModelAudit(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: modelAuditQueryKeys.targets })
  queryClient.invalidateQueries({ queryKey: modelAuditQueryKeys.baselines })
  queryClient.invalidateQueries({ queryKey: modelAuditQueryKeys.runs })
  queryClient.invalidateQueries({ queryKey: modelAuditQueryKeys.monitors })
}
