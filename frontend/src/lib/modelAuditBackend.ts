import { backendMutation, backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import type {
  ModelAuditBaseline,
  ModelAuditBaselineInput,
  ModelAuditMonitor,
  ModelAuditMonitorInput,
  ModelAuditReport,
  ModelAuditRun,
  ModelAuditRunInput,
  ModelAuditTarget,
  ModelAuditTargetInput,
} from '@/types/modelAudit'

type ListResponse<T> = { items: T[] }

const targetStorageKey = 'c2cmarket.model-audit.targets'
const baselineStorageKey = 'c2cmarket.model-audit.baselines'
const runStorageKey = 'c2cmarket.model-audit.runs'
const monitorStorageKey = 'c2cmarket.model-audit.monitors'

export async function getModelAuditTargets(): Promise<ModelAuditTarget[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<ModelAuditTarget>>('/api/v1/admin/model-audit/targets')
    return response.items
  }
  return readTargets()
}

export async function createModelAuditTarget(input: ModelAuditTargetInput): Promise<ModelAuditTarget> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditTarget>('/api/v1/admin/model-audit/targets', input)
  }
  const now = new Date().toISOString()
  const target: ModelAuditTarget = {
    id: stableId('target', input.name, readTargets().length),
    name: input.name.trim(),
    baseUrl: input.baseUrl.trim(),
    providerType: input.providerType || 'openai_compatible',
    claimedModel: input.claimedModel.trim(),
    enabled: input.enabled,
    apiServiceId: input.apiServiceId,
    apiServiceModelId: input.apiServiceModelId,
    createdAt: now,
    updatedAt: now,
  }
  writeTargets([target, ...readTargets()])
  return target
}

export async function updateModelAuditTarget(id: string, input: ModelAuditTargetInput): Promise<ModelAuditTarget> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditTarget>(`/api/v1/admin/model-audit/targets/${encodeURIComponent(id)}`, input, { method: 'PATCH' })
  }
  const rows = readTargets()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('审计目标不存在。')
  const updated: ModelAuditTarget = {
    ...previous,
    name: input.name.trim(),
    baseUrl: input.baseUrl.trim(),
    providerType: input.providerType || 'openai_compatible',
    claimedModel: input.claimedModel.trim(),
    enabled: input.enabled,
    apiServiceId: input.apiServiceId,
    apiServiceModelId: input.apiServiceModelId,
    updatedAt: new Date().toISOString(),
  }
  writeTargets(rows.map(item => item.id === id ? updated : item))
  return updated
}

export async function deleteModelAuditTarget(id: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<void>(`/api/v1/admin/model-audit/targets/${encodeURIComponent(id)}`, {}, { method: 'DELETE' })
  }
  writeTargets(readTargets().map(item => item.id === id ? { ...item, enabled: false, updatedAt: new Date().toISOString() } : item))
}

export async function getModelAuditBaselines(): Promise<ModelAuditBaseline[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<ModelAuditBaseline>>('/api/v1/admin/model-audit/baselines')
    return response.items
  }
  return readBaselines()
}

export async function createModelAuditBaseline(input: ModelAuditBaselineInput): Promise<ModelAuditBaseline> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditBaseline>('/api/v1/admin/model-audit/baselines', input)
  }
  const now = new Date().toISOString()
  const baseline: ModelAuditBaseline = {
    id: stableId('baseline', input.baselineName, readBaselines().length),
    baselineName: input.baselineName.trim(),
    sourceTargetId: input.sourceTargetId,
    model: input.model.trim(),
    sourceType: input.sourceType || 'official_api',
    probeSetVersion: input.probeSetVersion.trim(),
    paramsJson: input.paramsJson,
    featureJson: input.featureJson,
    sampleCount: input.sampleCount,
    validFrom: now,
    createdAt: now,
  }
  writeBaselines([baseline, ...readBaselines()])
  return baseline
}

export async function getModelAuditRuns(): Promise<ModelAuditRun[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<ModelAuditRun>>('/api/v1/admin/model-audit/runs')
    return response.items
  }
  return readRuns()
}

export async function createModelAuditRun(input: ModelAuditRunInput): Promise<ModelAuditRun> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditRun>('/api/v1/admin/model-audit/runs', input)
  }
  const target = readTargets().find(item => item.id === input.targetId)
  if (!target) throw new Error('审计目标不存在。')
  const now = new Date().toISOString()
  const run: ModelAuditRun = {
    id: stableId('run', `${target.id}-${now}`, readRuns().length),
    targetId: target.id,
    targetName: target.name,
    claimedModel: input.claimedModel || target.claimedModel,
    baselineId: input.baselineId,
    status: 'completed',
    mode: input.mode,
    riskLevel: input.mode === 'quick' ? 'suspicious' : 'insufficient_data',
    confidence: input.mode === 'quick' ? 0.62 : 0.28,
    overallScore: input.mode === 'quick' ? 0.36 : 0.18,
    probeScores: seedProbeScores(input.mode),
    startedAt: now,
    finishedAt: now,
    createdAt: now,
  }
  writeRuns([run, ...readRuns()])
  writeTargets(readTargets().map(item => item.id === target.id ? { ...item, lastRiskLevel: run.riskLevel, lastRunId: run.id, updatedAt: now } : item))
  return run
}

export async function cancelModelAuditRun(id: string): Promise<ModelAuditRun> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditRun>(`/api/v1/admin/model-audit/runs/${encodeURIComponent(id)}/cancel`, {})
  }
  const rows = readRuns()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('审计运行不存在。')
  const updated = { ...previous, status: 'cancelled' as const, finishedAt: new Date().toISOString() }
  writeRuns(rows.map(item => item.id === id ? updated : item))
  return updated
}

export async function getModelAuditReport(runId: string): Promise<ModelAuditReport> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendRequest<ModelAuditReport>(`/api/v1/admin/model-audit/runs/${encodeURIComponent(runId)}/report`)
  }
  const run = readRuns().find(item => item.id === runId)
  if (!run) throw new Error('审计运行不存在。')
  return reportFromRun(run)
}

export async function getModelAuditMonitors(): Promise<ModelAuditMonitor[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<ModelAuditMonitor>>('/api/v1/admin/model-audit/scheduled-monitors')
    return response.items
  }
  return readMonitors()
}

export async function createModelAuditMonitor(input: ModelAuditMonitorInput): Promise<ModelAuditMonitor> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ModelAuditMonitor>('/api/v1/admin/model-audit/scheduled-monitors', input)
  }
  const now = new Date().toISOString()
  const monitor: ModelAuditMonitor = {
    id: stableId('monitor', `${input.targetId}-${now}`, readMonitors().length),
    targetId: input.targetId,
    baselineId: input.baselineId,
    mode: input.mode,
    enabled: input.enabled,
    cronSpec: input.cronSpec,
    createdAt: now,
    updatedAt: now,
  }
  writeMonitors([monitor, ...readMonitors()])
  return monitor
}

function readTargets() {
  return readStore<ModelAuditTarget>(targetStorageKey, seedTargets())
}

function writeTargets(items: ModelAuditTarget[]) {
  writeStore(targetStorageKey, items)
}

function readBaselines() {
  return readStore<ModelAuditBaseline>(baselineStorageKey, seedBaselines())
}

function writeBaselines(items: ModelAuditBaseline[]) {
  writeStore(baselineStorageKey, items)
}

function readRuns() {
  return readStore<ModelAuditRun>(runStorageKey, seedRuns())
}

function writeRuns(items: ModelAuditRun[]) {
  writeStore(runStorageKey, items)
}

function readMonitors() {
  return readStore<ModelAuditMonitor>(monitorStorageKey, [])
}

function writeMonitors(items: ModelAuditMonitor[]) {
  writeStore(monitorStorageKey, items)
}

function readStore<T extends { id: string }>(key: string, seed: T[]) {
  if (typeof window === 'undefined') return seed
  try {
    const raw = window.sessionStorage.getItem(key)
    if (!raw) return seed
    const stored = JSON.parse(raw) as T[]
    const storedIds = new Set(stored.map(item => item.id))
    return [...stored, ...seed.filter(item => !storedIds.has(item.id))]
  } catch {
    return seed
  }
}

function writeStore<T>(key: string, items: T[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(key, JSON.stringify(items))
}

function seedTargets(): ModelAuditTarget[] {
  const now = '2026-07-07T00:00:00.000Z'
  return [{
    id: 'mock-model-audit-target-openai-relay',
    name: 'OpenAI Relay 样例',
    baseUrl: 'https://relay.example.test/v1',
    providerType: 'openai_compatible',
    claimedModel: 'gpt-4.1',
    enabled: true,
    lastRiskLevel: 'suspicious',
    lastRunId: 'mock-model-audit-run-1',
    createdAt: now,
    updatedAt: now,
  }]
}

function seedBaselines(): ModelAuditBaseline[] {
  const now = '2026-07-07T00:00:00.000Z'
  return [{
    id: 'mock-model-audit-baseline-gpt41',
    baselineName: 'gpt-4.1 official 2026-07',
    model: 'gpt-4.1',
    sourceType: 'official_api',
    probeSetVersion: '2026-07-v1',
    paramsJson: { temperature: 0.7 },
    featureJson: { random_distance_reference: 0 },
    sampleCount: 220,
    validFrom: now,
    createdAt: now,
  }]
}

function seedRuns(): ModelAuditRun[] {
  const now = '2026-07-07T00:20:00.000Z'
  return [{
    id: 'mock-model-audit-run-1',
    targetId: 'mock-model-audit-target-openai-relay',
    targetName: 'OpenAI Relay 样例',
    claimedModel: 'gpt-4.1',
    baselineId: 'mock-model-audit-baseline-gpt41',
    status: 'completed',
    mode: 'standard',
    riskLevel: 'suspicious',
    confidence: 0.78,
    overallScore: 0.43,
    probeScores: seedProbeScores('standard'),
    startedAt: now,
    finishedAt: now,
    createdAt: now,
  }]
}

function seedProbeScores(mode: ModelAuditRunInput['mode']) {
  return [
    { probe: 'random_fingerprint', risk: 'suspicious' as const, confidence: 0.81, score: 0.42, evidence: { js_distance: 0.27, mode } },
    { probe: 'kbf_knowledge_boundary', risk: 'insufficient_data' as const, confidence: 0.22, score: 0.18, evidence: { reason: 'mock_baseline_seed' } },
    { probe: 'billing_latency_protocol', risk: 'consistent' as const, confidence: 0.66, score: 0.08, evidence: { p95_ms: 1800 } },
  ]
}

function reportFromRun(run: ModelAuditRun): ModelAuditReport {
  return {
    runId: run.id,
    targetId: run.targetId,
    targetName: run.targetName,
    claimedModel: run.claimedModel,
    baselineId: run.baselineId,
    mode: run.mode,
    riskLevel: run.riskLevel ?? 'insufficient_data',
    confidence: run.confidence,
    overallRiskScore: run.overallScore,
    summary: '目标 API 与可信基线存在可观测漂移，建议扩大样本复核。',
    probeScores: run.probeScores ?? [],
    recommendations: ['运行 strict 模式复核。', '开启 scheduled 巡检观察漂移趋势。'],
    caveats: ['本报告为统计风险审计，不是法律、密码学或绝对证明。'],
    createdAt: run.createdAt,
    markdown: `# AI API 一致性审计报告\n\n- Run ID: ${run.id}\n- Risk Level: ${run.riskLevel}\n\n本报告为统计风险审计，不是法律、密码学或绝对证明。`,
  }
}

function stableId(prefix: string, value: string, index: number) {
  return `${prefix}-${value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'item'}-${index + 1}`
}
