export type ApiHealthState = 'normal' | 'fluctuating' | 'abnormal' | 'no_sample'
export type ApiHealthAvailabilityReason =
  | 'unconfigured'
  | 'disabled'
  | 'unverified'
  | 'insufficient'
  | 'stale'
  | 'temporarily_unavailable'
  | null
export type ApiHealthSlotState = 'smooth' | 'fluctuating' | 'abnormal' | 'no_sample'
export type ApiHealthTransportSecurity = 'secure_https' | 'insecure_http' | 'unknown' | null

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
  | 'decrypt_failed'
  | 'internal'
  | 'internal_timeout'

export type ApiServiceHealthSample = {
  slotStartedAt: string
  state: ApiHealthSlotState
}

export type ApiServiceHealthSummary = {
  state: ApiHealthState
  availabilityReason: ApiHealthAvailabilityReason
  successRatePercent: string | null
  successfulSamples: number
  totalSamples: number
  transportSecurity: ApiHealthTransportSecurity
  lastSampledAt: string | null
  samples: ApiServiceHealthSample[]
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
  measurementVersion: number
  version: number
  referencedServices: ApiProbeConnectionServiceReference[]
  healthSummary?: ApiServiceHealthSummary
  createdAt: string
  updatedAt: string
}

export type SaveOwnerAPIProbeConnectionInput = {
  id?: string
  version?: number
  name: string
  baseUrl: string
  credential?: string
  enabled: boolean
  acknowledgeInsecureHttp: boolean
}
