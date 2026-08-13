import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import type {
  ApiHealthSafeErrorCode,
  ApiProbeConnectionPreflight,
  ApiProbeLatencyRule,
  ApiProbeLatencyRuleInput,
  ApiProbeLatencyRulePreview,
  ApiProbeCalibration,
  OwnerAPIProbeConnection,
  SaveOwnerAPIProbeConnectionInput,
} from '@/types/apiHealth'

type ListResponse<T> = { items: T[] }

type OwnerAPIProbeConnectionDTO = Omit<OwnerAPIProbeConnection, 'lastVerificationErrorCode'> & {
  lastVerificationErrorCode: string | null
}

const safeErrorCodes: readonly ApiHealthSafeErrorCode[] = [
  'blocked_target',
  'authorization_invalid',
  'dns_failed',
  'connect_failed',
  'tls_failed',
  'timeout',
  'http_4xx',
  'http_5xx',
  'rate_limited',
  'response_too_large',
  'invalid_response',
  'stream_interrupted',
  'model_unavailable',
  'protocol_unavailable',
  'decrypt_failed',
  'internal',
  'internal_timeout',
]

function safeErrorCode(value: string | null): ApiHealthSafeErrorCode | null {
  if (!value) return null
  return safeErrorCodes.includes(value as ApiHealthSafeErrorCode)
    ? value as ApiHealthSafeErrorCode
    : 'internal'
}

function mapConnection(dto: OwnerAPIProbeConnectionDTO): OwnerAPIProbeConnection {
  return {
    ...dto,
    lastVerificationErrorCode: safeErrorCode(dto.lastVerificationErrorCode),
    referencedServices: dto.referencedServices ?? [],
  }
}

function connectionBody(input: SaveOwnerAPIProbeConnectionInput) {
  const credential = input.credential?.trim()
  return {
    name: input.name.trim(),
    baseUrl: input.baseUrl.trim(),
    enabled: input.enabled,
    acknowledgeInsecureHttp: input.acknowledgeInsecureHttp,
    probeModel: input.probeModel?.trim() ?? '',
    ...(credential ? { credential } : {}),
    ...(input.preflightToken?.trim() ? { preflightToken: input.preflightToken.trim() } : {}),
  }
}

export async function backendPreflightOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput) {
  await ensureBackendSession()
  const path = input.id
    ? `/api/v1/owner/api-probe-connections/${encodeURIComponent(input.id)}/preflight`
    : '/api/v1/owner/api-probe-connections/preflight'
  const dto = await backendMutation<ApiProbeConnectionPreflight>(path, connectionBody(input), {
    ...(input.id && input.version ? { ifMatch: input.version } : {}),
  })
  return { ...dto, errorCode: safeErrorCode(dto.errorCode), availableModels: dto.availableModels ?? [] }
}

export async function backendOwnerAPIProbeConnections() {
  await ensureBackendSession()
  const response = await backendRequest<ListResponse<OwnerAPIProbeConnectionDTO>>('/api/v1/owner/api-probe-connections')
  return response.items.map(mapConnection)
}

export async function backendOwnerAPIProbeConnection(id: string) {
  await ensureBackendSession()
  return mapConnection(await backendRequest<OwnerAPIProbeConnectionDTO>(`/api/v1/owner/api-probe-connections/${encodeURIComponent(id)}`))
}

export async function backendCreateOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput) {
  await ensureBackendSession()
  const dto = await backendMutation<OwnerAPIProbeConnectionDTO>(
    '/api/v1/owner/api-probe-connections',
    connectionBody(input),
    { idempotencyPrefix: 'api-probe-connection-create' },
  )
  return mapConnection(dto)
}

export async function backendUpdateOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput & { id: string, version: number }) {
  await ensureBackendSession()
  const dto = await backendMutation<OwnerAPIProbeConnectionDTO>(
    `/api/v1/owner/api-probe-connections/${encodeURIComponent(input.id)}`,
    connectionBody(input),
    { method: 'PUT', ifMatch: input.version, idempotencyPrefix: 'api-probe-connection-update' },
  )
  return mapConnection(dto)
}

export async function backendDeleteOwnerAPIProbeConnection(input: { id: string, version: number }) {
  await ensureBackendSession()
  await backendMutation<void>(
    `/api/v1/owner/api-probe-connections/${encodeURIComponent(input.id)}`,
    {},
    { method: 'DELETE', ifMatch: input.version, idempotencyPrefix: 'api-probe-connection-delete' },
  )
}

export async function backendVerifyOwnerAPIProbeConnection(input: { id: string, version: number }) {
  await ensureBackendSession()
  const dto = await backendMutation<OwnerAPIProbeConnectionDTO>(
    `/api/v1/owner/api-probe-connections/${encodeURIComponent(input.id)}/verify`,
    {},
    { ifMatch: input.version, idempotencyPrefix: 'api-probe-connection-verify' },
  )
  return mapConnection(dto)
}

export async function backendAdminAPIProbeCalibration(input: { model: string, protocol: string, environment: string }) {
  await ensureBackendSession()
  const query = new URLSearchParams(input)
  return backendRequest<ApiProbeCalibration>(`/api/v1/admin/api-health/latency-calibration?${query}`)
}

export async function backendPreviewAdminAPIProbeLatencyRule(input: ApiProbeLatencyRuleInput) {
  await ensureBackendSession()
  return backendMutation<ApiProbeLatencyRulePreview>('/api/v1/admin/api-health/latency-rules/preview', input)
}

export async function backendPublishAdminAPIProbeLatencyRule(input: ApiProbeLatencyRuleInput) {
  await ensureBackendSession()
  return backendMutation<ApiProbeLatencyRule>('/api/v1/admin/api-health/latency-rules', input, {
    idempotencyPrefix: 'api-probe-latency-rule-publish',
  })
}

export async function backendAdminAPIProbeLatencyRules() {
  await ensureBackendSession()
  const response = await backendRequest<ListResponse<ApiProbeLatencyRule>>('/api/v1/admin/api-health/latency-rules')
  return response.items
}
