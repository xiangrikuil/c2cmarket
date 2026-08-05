import type {
  ApiHealthProbeAuthorizationMethod,
  ApiHealthProbeAuthorizationStatus,
  ApiHealthProbeChallenge,
  ServiceHealthSample,
  ServiceHealthSummary,
} from '@/api/generated/openapi'

export type ApiHealthState = ServiceHealthSummary['state']
export type ApiHealthAvailabilityReason = ServiceHealthSummary['availabilityReason']
export type ApiHealthSlotState = ServiceHealthSample['state']
export type ApiHealthTransportSecurity = ServiceHealthSummary['transportSecurity']
export type ApiHealthAuthorizationStatus = ApiHealthProbeAuthorizationStatus
export type ApiHealthAuthorizationMethod = Exclude<ApiHealthProbeAuthorizationMethod, null>

export type ApiHealthSafeErrorCode =
  | 'blocked_target'
  | 'authorization_invalid'
  | 'dns_failed'
  | 'connect_failed'
  | 'tls_failed'
  | 'timeout'
  | 'http_4xx'
  | 'http_5xx'
  | 'response_too_large'
  | 'invalid_stream'
  | 'empty_response'
  | 'decrypt_failed'
  | 'internal'
  | 'internal_timeout'
  | 'challenge_mismatch'
  | 'challenge_expired'
  | 'dns_resolution_failed'
  | 'invalid_origin'
  | 'target_blocked'
  | 'http_request_failed'
  | 'http_status'
  | 'http_response_invalid'

export type ApiServiceHealthSample = ServiceHealthSample
export type ApiServiceHealthSummary = ServiceHealthSummary

export type OwnerAPIHealthProbeConfig = {
  id: string
  apiServiceId: string
  protocol: 'openai_chat_completions_v1'
  baseUrl: string
  normalizedOrigin: string
  model: string
  credentialConfigured: boolean
  enabled: boolean
  authorizationStatus: ApiHealthAuthorizationStatus
  authorizationMethod: ApiHealthAuthorizationMethod | null
  verifiedOrigin: string | null
  verifiedAt: string | null
  approvedAt: string | null
  rejectionReason: string | null
  challengeExpiresAt: string | null
  measurementVersion: number
  lastConfigErrorCode: ApiHealthSafeErrorCode | null
  version: number
  createdAt: string
  updatedAt: string
}

export type SaveOwnerAPIHealthProbeInput = {
  apiServiceId: string
  version: number
  baseUrl: string
  model: string
  credential?: string
  enabled: boolean
  acknowledgeInsecureHttp: boolean
}

export type APIHealthProbeChallenge = ApiHealthProbeChallenge

export type AdminAPIHealthProbeReview = {
  id: string
  apiServiceId: string
  serviceTitle: string | null
  ownerUserId: string
  ownerDisplayName: string | null
  ownerUsername: string | null
  normalizedOrigin: string
  authorizationStatus: ApiHealthAuthorizationStatus
  version: number
  updatedAt: string
}

export type AdminAPIHealthProbeReviewList = {
  items: AdminAPIHealthProbeReview[]
  nextCursor: string | null
}
