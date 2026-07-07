export type ModelAuditRiskLevel = 'consistent' | 'suspicious' | 'high_risk' | 'insufficient_data'
export type ModelAuditProbeRiskLevel = ModelAuditRiskLevel | 'not_applicable'
export type ModelAuditMode = 'quick' | 'standard' | 'strict' | 'scheduled'
export type ModelAuditRunStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'

export type ModelAuditTarget = {
  id: string
  name: string
  baseUrl: string
  providerType: string
  claimedModel: string
  enabled: boolean
  apiServiceId?: string
  apiServiceModelId?: string
  lastRiskLevel?: ModelAuditRiskLevel
  lastRunId?: string
  createdAt: string
  updatedAt: string
}

export type ModelAuditTargetInput = {
  name: string
  baseUrl: string
  providerType: string
  claimedModel: string
  apiKey: string
  enabled: boolean
  apiServiceId?: string
  apiServiceModelId?: string
}

export type ModelAuditBaseline = {
  id: string
  baselineName: string
  sourceTargetId?: string
  model: string
  sourceType: string
  probeSetVersion: string
  paramsJson: Record<string, unknown>
  featureJson: Record<string, unknown>
  sampleCount: number
  validFrom: string
  validTo?: string
  createdAt: string
}

export type ModelAuditBaselineInput = {
  baselineName: string
  sourceTargetId?: string
  model: string
  sourceType: string
  probeSetVersion: string
  paramsJson: Record<string, unknown>
  featureJson: Record<string, unknown>
  sampleCount: number
}

export type ModelAuditProbeScore = {
  probe: string
  risk: ModelAuditProbeRiskLevel
  confidence: number
  score: number
  evidence: Record<string, unknown>
}

export type ModelAuditRun = {
  id: string
  targetId: string
  targetName: string
  claimedModel: string
  baselineId?: string
  status: ModelAuditRunStatus
  mode: ModelAuditMode
  riskLevel?: ModelAuditRiskLevel
  confidence: number
  overallScore: number
  errorMessage?: string
  probeScores?: ModelAuditProbeScore[]
  startedAt?: string
  finishedAt?: string
  createdAt: string
}

export type ModelAuditRunInput = {
  targetId: string
  baselineId?: string
  claimedModel?: string
  mode: ModelAuditMode
  enableModelEquality: boolean
  enableLogprobs: 'auto' | 'enabled' | 'disabled'
  storePromptText: boolean
  storeResponseText: boolean
}

export type ModelAuditMonitor = {
  id: string
  targetId: string
  baselineId?: string
  mode: ModelAuditMode
  enabled: boolean
  cronSpec?: string
  lastRunId?: string
  lastRisk?: ModelAuditRiskLevel
  lastRunAt?: string
  createdAt: string
  updatedAt: string
}

export type ModelAuditMonitorInput = {
  targetId: string
  baselineId?: string
  mode: ModelAuditMode
  enabled: boolean
  cronSpec?: string
}

export type ModelAuditReport = {
  runId: string
  targetId: string
  targetName: string
  claimedModel: string
  baselineId?: string
  mode: ModelAuditMode
  riskLevel: ModelAuditRiskLevel
  confidence: number
  overallRiskScore: number
  summary: string
  probeScores: ModelAuditProbeScore[]
  recommendations: string[]
  caveats: string[]
  createdAt: string
  markdown: string
}
