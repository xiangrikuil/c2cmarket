import { BackendProblemError, backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import type {
  AdminApiHealthProbe,
  AdminApiHealthProbeDecisionRequest,
  AdminApiHealthProbeList,
  ApiHealthProbeChallenge,
  ApiHealthProbeChallengeRequest,
  ApiHealthProbeConfigRequestWritable,
  OwnerApiHealthProbeConfig,
} from '@/api/generated/openapi'
import type {
  AdminAPIHealthProbeReview,
  AdminAPIHealthProbeReviewList,
  ApiHealthAuthorizationMethod,
  ApiHealthAuthorizationStatus,
  ApiHealthSafeErrorCode,
  APIHealthProbeChallenge,
  OwnerAPIHealthProbeConfig,
  SaveOwnerAPIHealthProbeInput,
} from '@/types/apiHealth'

const safeErrorCodes: readonly ApiHealthSafeErrorCode[] = [
  'blocked_target',
  'authorization_invalid',
  'dns_failed',
  'connect_failed',
  'tls_failed',
  'timeout',
  'http_4xx',
  'http_5xx',
  'response_too_large',
  'invalid_stream',
  'empty_response',
  'decrypt_failed',
  'internal',
  'internal_timeout',
  'challenge_mismatch',
  'challenge_expired',
  'dns_resolution_failed',
  'invalid_origin',
  'target_blocked',
  'http_request_failed',
  'http_status',
  'http_response_invalid',
]

function safeErrorCode(value: string | null): ApiHealthSafeErrorCode | null {
  if (!value) return null
  return safeErrorCodes.includes(value as ApiHealthSafeErrorCode) ? value as ApiHealthSafeErrorCode : 'internal'
}

function mapOwnerProbe(dto: OwnerApiHealthProbeConfig): OwnerAPIHealthProbeConfig {
  return {
    id: dto.id,
    apiServiceId: dto.apiServiceId,
    protocol: dto.protocol,
    baseUrl: dto.baseUrl,
    normalizedOrigin: dto.normalizedOrigin,
    model: dto.model,
    credentialConfigured: dto.credentialConfigured,
    enabled: dto.enabled,
    authorizationStatus: dto.authorizationStatus,
    authorizationMethod: dto.authorizationMethod,
    verifiedOrigin: dto.verifiedOrigin,
    verifiedAt: dto.verifiedAt,
    approvedAt: dto.approvedAt,
    rejectionReason: dto.rejectionReason,
    challengeExpiresAt: dto.challengeExpiresAt,
    measurementVersion: dto.measurementVersion,
    lastConfigErrorCode: safeErrorCode(dto.lastConfigErrorCode),
    version: dto.version,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
  }
}

function mapAdminProbe(dto: AdminApiHealthProbe): AdminAPIHealthProbeReview {
  return {
    id: dto.id,
    apiServiceId: dto.apiServiceId,
    serviceTitle: dto.serviceTitle.trim() || null,
    ownerUserId: dto.ownerUserId,
    ownerDisplayName: dto.ownerDisplayName.trim() || null,
    ownerUsername: dto.ownerUsername.trim() || null,
    normalizedOrigin: dto.normalizedOrigin,
    authorizationStatus: dto.authorizationStatus,
    version: dto.version,
    updatedAt: dto.updatedAt,
  }
}

export async function backendOwnerAPIHealthProbe(apiServiceId: string) {
  await ensureBackendSession()
  try {
    const dto = await backendRequest<OwnerApiHealthProbeConfig>(
      `/api/v1/owner/api-services/${encodeURIComponent(apiServiceId)}/health-probe`,
    )
    return mapOwnerProbe(dto)
  } catch (error) {
    if (error instanceof BackendProblemError && error.status === 404) return null
    throw error
  }
}

export async function backendSaveOwnerAPIHealthProbe(input: SaveOwnerAPIHealthProbeInput) {
  await ensureBackendSession()
  const credential = input.credential?.trim()
  const body = {
    baseUrl: input.baseUrl.trim(),
    model: input.model.trim(),
    enabled: input.enabled,
    ...(credential ? { credential } : {}),
  } satisfies ApiHealthProbeConfigRequestWritable
  const dto = await backendMutation<OwnerApiHealthProbeConfig>(
    `/api/v1/owner/api-services/${encodeURIComponent(input.apiServiceId)}/health-probe`,
    body,
    { method: 'PUT', ifMatch: input.version },
  )
  return mapOwnerProbe(dto)
}

export async function backendDeleteOwnerAPIHealthProbe(input: { apiServiceId: string, version: number }) {
  await ensureBackendSession()
  await backendMutation<void>(
    `/api/v1/owner/api-services/${encodeURIComponent(input.apiServiceId)}/health-probe`,
    {},
    { method: 'DELETE', ifMatch: input.version },
  )
}

export async function backendCreateAPIHealthChallenge(input: {
  apiServiceId: string
  version: number
  method: Exclude<ApiHealthAuthorizationMethod, 'admin_approval'>
}) {
  await ensureBackendSession()
  const body = { method: input.method } satisfies ApiHealthProbeChallengeRequest
  const dto = await backendMutation<ApiHealthProbeChallenge>(
    `/api/v1/owner/api-services/${encodeURIComponent(input.apiServiceId)}/health-probe/challenges`,
    body,
    { ifMatch: input.version, idempotencyPrefix: 'api-health-challenge' },
  )
  return {
    token: dto.token,
    method: dto.method,
    dnsRecordName: dto.dnsRecordName ?? null,
    httpUrl: dto.httpUrl ?? null,
    expiresAt: dto.expiresAt,
    configVersion: dto.configVersion,
  } satisfies APIHealthProbeChallenge
}

export async function backendVerifyAPIHealthChallenge(input: { apiServiceId: string, version: number }) {
  await ensureBackendSession()
  const dto = await backendMutation<OwnerApiHealthProbeConfig>(
    `/api/v1/owner/api-services/${encodeURIComponent(input.apiServiceId)}/health-probe/verify`,
    {},
    { ifMatch: input.version, idempotencyPrefix: 'api-health-verify' },
  )
  return mapOwnerProbe(dto)
}

export async function backendAdminAPIHealthProbes(status: ApiHealthAuthorizationStatus) {
  await ensureBackendSession('admin', true)
  const params = new URLSearchParams({ status, limit: '100' })
  const dto = await backendRequest<AdminApiHealthProbeList>(`/api/v1/admin/api-service-health-probes?${params.toString()}`)
  return {
    items: dto.items.map(mapAdminProbe),
    nextCursor: dto.nextCursor ?? null,
  } satisfies AdminAPIHealthProbeReviewList
}

export async function backendReviewAPIHealthProbe(input: {
  id: string
  version: number
  decision: 'approve' | 'reject'
  reason: string
}) {
  await ensureBackendSession('admin', true)
  const body = { reason: input.reason.trim() } satisfies AdminApiHealthProbeDecisionRequest
  const dto = await backendMutation<AdminApiHealthProbe>(
    `/api/v1/admin/api-service-health-probes/${encodeURIComponent(input.id)}/${input.decision}`,
    body,
    { ifMatch: input.version, idempotencyPrefix: `api-health-${input.decision}` },
  )
  return mapAdminProbe(dto)
}
