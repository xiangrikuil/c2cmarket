import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  backendAdminAPIHealthProbes,
  backendCreateAPIHealthChallenge,
  backendDeleteOwnerAPIHealthProbe,
  backendOwnerAPIHealthProbe,
  backendReviewAPIHealthProbe,
  backendSaveOwnerAPIHealthProbe,
  backendVerifyAPIHealthChallenge,
} from '@/lib/apiHealthBackend'
import type {
  AdminAPIHealthProbeReview,
  AdminAPIHealthProbeReviewList,
  ApiHealthAuthorizationMethod,
  ApiHealthAuthorizationStatus,
  APIHealthProbeChallenge,
  OwnerAPIHealthProbeConfig,
  SaveOwnerAPIHealthProbeInput,
} from '@/types/apiHealth'

const mockConfigs = new Map<string, OwnerAPIHealthProbeConfig>()

function now() {
  return new Date().toISOString()
}

function mockOrigin(baseUrl: string) {
  const url = new URL(baseUrl)
  const port = url.port || '443'
  return `${url.protocol}//${url.hostname}:${port}`
}

function mockAdminReview(config: OwnerAPIHealthProbeConfig): AdminAPIHealthProbeReview {
  return {
    id: config.id,
    apiServiceId: config.apiServiceId,
    serviceTitle: null,
    ownerUserId: 'mock-owner',
    ownerDisplayName: '演示卖家',
    ownerUsername: 'mock-owner',
    normalizedOrigin: config.normalizedOrigin,
    authorizationStatus: config.authorizationStatus,
    version: config.version,
    updatedAt: config.updatedAt,
  }
}

export async function getOwnerAPIHealthProbe(apiServiceId: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIHealthProbe(apiServiceId)
  return structuredClone(mockConfigs.get(apiServiceId) ?? null)
}

export async function saveOwnerAPIHealthProbe(input: SaveOwnerAPIHealthProbeInput) {
  if (shouldUseRealBackend()) return backendSaveOwnerAPIHealthProbe(input)
  const current = mockConfigs.get(input.apiServiceId)
  if ((current?.version ?? 0) !== input.version) throw new Error('探针配置已更新，请刷新后重试。')
  const timestamp = now()
  const next: OwnerAPIHealthProbeConfig = {
    id: current?.id ?? `probe-${input.apiServiceId}`,
    apiServiceId: input.apiServiceId,
    protocol: 'openai_chat_completions_v1',
    baseUrl: input.baseUrl.trim(),
    normalizedOrigin: mockOrigin(input.baseUrl.trim()),
    model: input.model.trim(),
    credentialConfigured: current?.credentialConfigured || Boolean(input.credential?.trim()),
    enabled: input.enabled,
    authorizationStatus: current?.baseUrl === input.baseUrl.trim() && current.model === input.model.trim()
      ? current.authorizationStatus
      : 'pending',
    authorizationMethod: current?.baseUrl === input.baseUrl.trim() && current.model === input.model.trim()
      ? current.authorizationMethod
      : null,
    verifiedOrigin: current?.baseUrl === input.baseUrl.trim() && current.model === input.model.trim()
      ? current.verifiedOrigin
      : null,
    verifiedAt: current?.baseUrl === input.baseUrl.trim() && current.model === input.model.trim()
      ? current.verifiedAt
      : null,
    approvedAt: null,
    rejectionReason: null,
    challengeExpiresAt: null,
    measurementVersion: (current?.measurementVersion ?? 0) + (current && current.baseUrl === input.baseUrl.trim() && current.model === input.model.trim() ? 0 : 1),
    lastConfigErrorCode: null,
    version: (current?.version ?? 0) + 1,
    createdAt: current?.createdAt ?? timestamp,
    updatedAt: timestamp,
  }
  mockConfigs.set(input.apiServiceId, next)
  return structuredClone(next)
}

export async function deleteOwnerAPIHealthProbe(input: { apiServiceId: string, version: number }) {
  if (shouldUseRealBackend()) return backendDeleteOwnerAPIHealthProbe(input)
  const current = mockConfigs.get(input.apiServiceId)
  if (!current || current.version !== input.version) throw new Error('探针配置已更新，请刷新后重试。')
  mockConfigs.delete(input.apiServiceId)
}

export async function createAPIHealthChallenge(input: {
  apiServiceId: string
  version: number
  method: Exclude<ApiHealthAuthorizationMethod, 'admin_approval'>
}) {
  if (shouldUseRealBackend()) return backendCreateAPIHealthChallenge(input)
  const current = mockConfigs.get(input.apiServiceId)
  if (!current || current.version !== input.version) throw new Error('探针配置已更新，请刷新后重试。')
  const token = `mock-probe-${crypto.randomUUID()}`
  const expiresAt = new Date(Date.now() + 15 * 60_000).toISOString()
  const url = new URL(current.normalizedOrigin)
  const challenge: APIHealthProbeChallenge = {
    token,
    method: input.method,
    dnsRecordName: input.method === 'dns_txt' ? `_c2cmarket-probe.${url.hostname}` : null,
    httpUrl: input.method === 'http_challenge' ? `${current.normalizedOrigin}/.well-known/c2cmarket-probe-verification` : null,
    expiresAt,
    configVersion: current.version + 1,
  }
  mockConfigs.set(input.apiServiceId, {
    ...current,
    authorizationMethod: input.method,
    challengeExpiresAt: expiresAt,
    version: challenge.configVersion,
    updatedAt: now(),
  })
  return challenge
}

export async function verifyAPIHealthChallenge(input: { apiServiceId: string, version: number }) {
  if (shouldUseRealBackend()) return backendVerifyAPIHealthChallenge(input)
  const current = mockConfigs.get(input.apiServiceId)
  if (!current || current.version !== input.version) throw new Error('探针配置已更新，请刷新后重试。')
  const next: OwnerAPIHealthProbeConfig = {
    ...current,
    authorizationStatus: 'verified',
    verifiedOrigin: current.normalizedOrigin,
    verifiedAt: now(),
    challengeExpiresAt: null,
    version: current.version + 1,
    updatedAt: now(),
  }
  mockConfigs.set(input.apiServiceId, next)
  return structuredClone(next)
}

export async function getAdminAPIHealthProbes(status: ApiHealthAuthorizationStatus) {
  if (shouldUseRealBackend()) return backendAdminAPIHealthProbes(status)
  const items = [...mockConfigs.values()]
    .filter(config => config.authorizationStatus === status)
    .map(mockAdminReview)
  return {
    items,
    nextCursor: null,
  } satisfies AdminAPIHealthProbeReviewList
}

export async function reviewAPIHealthProbe(input: {
  id: string
  version: number
  decision: 'approve' | 'reject'
  reason: string
}) {
  if (shouldUseRealBackend()) return backendReviewAPIHealthProbe(input)
  const entry = [...mockConfigs.entries()].find(([, config]) => config.id === input.id)
  if (!entry || entry[1].version !== input.version) throw new Error('探针配置已更新，请刷新后重试。')
  const [apiServiceId, current] = entry
  const next: OwnerAPIHealthProbeConfig = {
    ...current,
    authorizationStatus: input.decision === 'approve' ? 'approved' : 'rejected',
    authorizationMethod: 'admin_approval',
    verifiedOrigin: input.decision === 'approve' ? current.normalizedOrigin : null,
    verifiedAt: input.decision === 'approve' ? now() : null,
    approvedAt: input.decision === 'approve' ? now() : null,
    rejectionReason: input.decision === 'reject' ? input.reason.trim() : null,
    version: current.version + 1,
    updatedAt: now(),
  }
  mockConfigs.set(apiServiceId, next)
  return mockAdminReview(next)
}

export function resetMockAPIHealthProbes() {
  mockConfigs.clear()
}
