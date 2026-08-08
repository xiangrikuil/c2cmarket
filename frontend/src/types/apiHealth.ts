export type ApiHealthState = 'normal' | 'fluctuating' | 'abnormal' | 'no_sample'
export type ApiHealthAvailabilityReason =
  | 'unconfigured'
  | 'disabled'
  | 'unverified'
  | 'insufficient'
  | 'stale'
  | 'temporarily_unavailable'
  | 'runner_disabled'
  | null
export type ApiHealthSlotState = 'smooth' | 'fluctuating' | 'abnormal' | 'no_sample'
export type ApiHealthTransportSecurity = 'secure_https' | 'insecure_http' | 'unknown' | null
export type ApiProbeProtocol = 'openai_responses_v1' | 'openai_chat_completions_v1'

export type ApiHealthSafeErrorCode =
  | 'blocked_target'
  | 'authorization_invalid'
  | 'dns_failed'
  | 'connect_failed'
  | 'tls_failed'
  | 'timeout'
  | 'http_4xx'
  | 'http_5xx'
  | 'rate_limited'
  | 'response_too_large'
  | 'invalid_response'
  | 'stream_interrupted'
  | 'model_unavailable'
  | 'protocol_unavailable'
  | 'decrypt_failed'
  | 'internal'
  | 'internal_timeout'

export type ApiServiceHealthSample = {
  slotStartedAt: string
  state: ApiHealthSlotState
}

export type ApiServiceHealthHourlyBucket = {
  hourStartedAt: string
  state: ApiHealthSlotState
  completedCycles: number
  firstAttemptSuccesses: number
  retryRecoveries: number
  finalFailures: number
  slowSuccesses: number
  finalSuccessPercent: string | null
  averageTtftMs: number | null
}

export type ApiServiceHealthCost = {
  knownBaseCostUsd: string
  knownRetryCostUsd: string
  projectedDailyCostUsd: string
  hasUnknownUsage: boolean
  knownUsageSamples: number
}

export type ApiServiceHealthSummary = {
  state: ApiHealthState
  availabilityReason: ApiHealthAvailabilityReason
  transportSecurity: ApiHealthTransportSecurity
  stabilityPercent: string | null
  finalSuccessPercent: string | null
  coveragePercent: string
  completedCycles: number
  theoreticalSlots: number
  firstAttemptSuccesses: number
  retryRecoveries: number
  finalFailures: number
  averageTtftMs: number | null
  p50TtftMs: number | null
  p95TtftMs: number | null
  lastSampledAt: string | null
  probeModel: string | null
  probeProtocol: ApiProbeProtocol | null
  probeEnvironment: string | null
  probeEnvironmentLabel: string | null
  probeModelChangedAt: string | null
  accumulatingSamples: boolean
  hourlyBuckets: ApiServiceHealthHourlyBucket[]
  cost: ApiServiceHealthCost
  // Temporary backend compatibility aliases.
  successRatePercent?: string | null
  successfulSamples?: number
  totalSamples?: number
  samples?: ApiServiceHealthSample[]
}

export type ApiProbeConnectionVerificationStatus = 'unverified' | 'verified' | 'failed'

export type ApiProbeConnectionServiceReference = {
  id: string
  title: string
}

export type OwnerAPIProbeConnection = {
  id: string
  name: string
  baseUrl: string
  normalizedBaseUrl: string
  credentialConfigured: boolean
  enabled: boolean
  verificationStatus: ApiProbeConnectionVerificationStatus
  verifiedAt: string | null
  lastVerificationErrorCode: ApiHealthSafeErrorCode | null
  probeModel: string | null
  probeProtocol: ApiProbeProtocol | null
  availableModels: string[]
  probeEnvironment: string
  probeModelChangedAt: string | null
  dailyBaseCostUpperBoundUsd: string | null
  priceUnavailable: boolean
  measurementVersion: number
  version: number
  referencedServices: ApiProbeConnectionServiceReference[]
  healthSummary: ApiServiceHealthSummary
  createdAt: string
  updatedAt: string
}

export type SaveOwnerAPIProbeConnectionInput = {
  id?: string
  version?: number
  name: string
  baseUrl: string
  credential?: string
  probeModel?: string
  preflightToken?: string
  enabled: boolean
  acknowledgeInsecureHttp: boolean
}

export type ApiProbeConnectionPreflight = {
  errorCode: ApiHealthSafeErrorCode | null
  availableModels: string[]
  probeModel: string | null
  probeProtocol: ApiProbeProtocol | null
  probeEnvironment: string
  dailyBaseCostUpperBoundUsd: string | null
  priceUnavailable: boolean
  preflightToken: string | null
}

export type ApiProbeCalibration = {
  model: string
  protocol: ApiProbeProtocol
  environment: string
  environmentLabel: string
  observationStartedAt: string
  observationEndedAt: string
  completeCalendarDays: number
  connectionCount: number
  sampleCount: number
  p50TtftMs: number | null
  p90TtftMs: number | null
  p95TtftMs: number | null
  p99TtftMs: number | null
  ready: boolean
}

export type ApiProbeLatencyRulePreview = {
  calibration: ApiProbeCalibration
  slowTtftMs: number
  hardTimeoutMs: number
  slowSampleCount: number
  slowPercent: string
  overTimeoutCount: number
  overTimeoutPercent: string
}

export type ApiProbeLatencyRule = Omit<ApiProbeCalibration, 'ready'> & {
  id: string
  version: number
  slowTtftMs: number
  hardTimeoutMs: number
  status: 'active' | 'superseded'
  publishedAt: string
  supersededAt: string | null
}

export type ApiProbeLatencyRuleInput = {
  model: string
  protocol: ApiProbeProtocol
  environment: string
  slowTtftMs: number
  hardTimeoutMs: number
}
