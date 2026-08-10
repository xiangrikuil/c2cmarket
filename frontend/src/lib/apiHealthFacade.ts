import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  backendAdminAPIProbeCalibration,
  backendAdminAPIProbeLatencyRules,
  backendCreateOwnerAPIProbeConnection,
  backendDeleteOwnerAPIProbeConnection,
  backendOwnerAPIProbeConnection,
  backendOwnerAPIProbeConnections,
  backendPreflightOwnerAPIProbeConnection,
  backendPreviewAdminAPIProbeLatencyRule,
  backendPublishAdminAPIProbeLatencyRule,
  backendUpdateOwnerAPIProbeConnection,
  backendVerifyOwnerAPIProbeConnection,
} from '@/lib/apiHealthBackend'
import type { ApiProbeLatencyRuleInput, OwnerAPIProbeConnection, SaveOwnerAPIProbeConnectionInput } from '@/types/apiHealth'

const mockConnections = new Map<string, OwnerAPIProbeConnection>()
const mockCredentials = new Map<string, string>()
const mockPreflightTokens = new Map<string, string>()

function now() {
  return new Date().toISOString()
}

function normalizeMockBaseURL(value: string) {
  const raw = value.trim()
  const url = new URL(raw)
  if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new Error('Base URL 必须使用 HTTP 或 HTTPS。')
  if (url.username || url.password || url.search || url.hash) throw new Error('Base URL 不能包含用户信息、查询参数或锚点。')
  url.hostname = url.hostname.toLowerCase()
  if ((url.protocol === 'https:' && url.port === '443') || (url.protocol === 'http:' && url.port === '80')) url.port = ''
  url.pathname = url.pathname.replace(/\/+$/, '') || '/'
  const canonical = url.toString().replace(/\/$/, '')
  return { raw, canonical }
}

function clone<T>(value: T): T {
  return structuredClone(value)
}

function mockVerification(credential: string | undefined) {
  const failed = credential?.toLowerCase().includes('invalid') ?? false
  return {
    verificationStatus: failed ? 'failed' as const : 'verified' as const,
    verifiedAt: failed ? null : now(),
    lastVerificationErrorCode: failed ? 'authorization_invalid' as const : null,
  }
}

function mockPreflightSignature(input: SaveOwnerAPIProbeConnectionInput, canonicalBaseUrl: string, credential: string, model: string) {
  return [input.id ?? '', input.version ?? 0, canonicalBaseUrl, credential, model, 'openai_responses_v1'].join('|')
}

function consumeMockPreflight(input: SaveOwnerAPIProbeConnectionInput, canonicalBaseUrl: string, credential: string, model: string) {
  const token = input.preflightToken?.trim() ?? ''
  const expected = mockPreflightTokens.get(token)
  mockPreflightTokens.delete(token)
  if (!token || expected !== mockPreflightSignature(input, canonicalBaseUrl, credential, model)) {
    throw new Error('验证结果已过期、已使用或与当前配置不一致，请重新验证。')
  }
}

function emptySummary(model: string | null) {
	const start = new Date()
	start.setUTCMinutes(0, 0, 0)
	start.setUTCHours(start.getUTCHours() - 23)
	const lastReachedSlot = new Date()
	lastReachedSlot.setUTCSeconds(0, 0)
	lastReachedSlot.setUTCMinutes(Math.floor(lastReachedSlot.getUTCMinutes() / 5) * 5)
	const theoreticalSlots = Math.min(288, Math.floor((lastReachedSlot.getTime() - start.getTime()) / 300_000) + 1)
	const hourlyBuckets = Array.from({ length: 24 }, (_, index) => ({
    hourStartedAt: new Date(start.getTime() + index * 3_600_000).toISOString(),
    state: 'no_sample' as const,
    completedCycles: 0,
    firstAttemptSuccesses: 0,
    retryRecoveries: 0,
    finalFailures: 0,
    slowSuccesses: 0,
    finalSuccessPercent: null,
    averageTtftMs: null,
  }))
  return {
    state: 'no_sample' as const,
    availabilityReason: 'insufficient' as const,
    transportSecurity: 'secure_https' as const,
    stabilityPercent: null,
    finalSuccessPercent: null,
    coveragePercent: '0.0',
    completedCycles: 0,
		theoreticalSlots,
    firstAttemptSuccesses: 0,
    retryRecoveries: 0,
    finalFailures: 0,
    averageTtftMs: null,
    p50TtftMs: null,
    p95TtftMs: null,
    lastSampledAt: null,
    probeModel: model,
    probeProtocol: 'openai_responses_v1' as const,
    probeEnvironment: 'us-west-v1',
    probeEnvironmentLabel: '平台美西',
    probeModelChangedAt: null,
    accumulatingSamples: true,
    hourlyBuckets,
    cost: { knownBaseCostUsd: '0.0000000000', knownRetryCostUsd: '0.0000000000', projectedDailyCostUsd: '', hasUnknownUsage: false, knownUsageSamples: 0 },
  }
}

export async function preflightOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput) {
  if (shouldUseRealBackend()) return backendPreflightOwnerAPIProbeConnection(input)
  const credential = input.credential?.trim() || (input.id ? mockCredentials.get(input.id) : undefined)
  if (!input.id && !credential) throw new Error('首次创建必须填写探针专用 API Key。')
  if (!credential) throw new Error('探针专用 API Key 不可用。')
  const target = normalizeMockBaseURL(input.baseUrl)
  if (target.raw.startsWith('http://') && !input.acknowledgeInsecureHttp) throw new Error('请先确认 HTTP 未加密传输风险。')
  const models = ['gpt-5.6-luna', 'gpt-5.6-sol']
  const model = input.probeModel || 'gpt-5.6-luna'
  const errorCode = credential.toLowerCase().includes('invalid')
    ? 'authorization_invalid' as const
    : models.includes(model) ? null : 'model_unavailable' as const
  const preflightToken = errorCode ? null : crypto.randomUUID()
  if (preflightToken) mockPreflightTokens.set(preflightToken, mockPreflightSignature(input, target.canonical, credential, model))
  return {
    errorCode,
    availableModels: models,
    probeModel: models.includes(model) ? model : null,
    probeProtocol: errorCode ? null : 'openai_responses_v1' as const,
    probeEnvironment: 'us-west-v1',
    dailyBaseCostUpperBoundUsd: null,
    priceUnavailable: true,
    preflightToken,
  }
}

export async function getOwnerAPIProbeConnections() {
  if (shouldUseRealBackend()) return backendOwnerAPIProbeConnections()
  return [...mockConnections.values()].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt)).map(clone)
}

export async function getOwnerAPIProbeConnection(id: string) {
  if (shouldUseRealBackend()) return backendOwnerAPIProbeConnection(id)
  const connection = mockConnections.get(id)
  if (!connection) throw new Error('探针连接不存在。')
  return clone(connection)
}

export async function createOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput) {
  if (shouldUseRealBackend()) return backendCreateOwnerAPIProbeConnection(input)
  const name = input.name.trim()
  const credential = input.credential?.trim()
  if (!name) throw new Error('请填写连接名称。')
  if (!credential) throw new Error('首次创建必须填写探针专用 API Key。')
  const target = normalizeMockBaseURL(input.baseUrl)
  if (target.raw.startsWith('http://') && !input.acknowledgeInsecureHttp) throw new Error('请先确认 HTTP 未加密传输风险。')
  const timestamp = now()
  const verification = mockVerification(credential)
  const probeModel = input.probeModel || 'gpt-5.6-luna'
  consumeMockPreflight(input, target.canonical, credential, probeModel)
  const connection: OwnerAPIProbeConnection = {
    id: crypto.randomUUID(),
    name,
    baseUrl: target.raw,
    normalizedBaseUrl: target.canonical,
    credentialConfigured: true,
    enabled: input.enabled && verification.verificationStatus === 'verified',
    ...verification,
    probeModel,
    probeProtocol: 'openai_responses_v1',
    availableModels: ['gpt-5.6-luna', 'gpt-5.6-sol'],
    probeEnvironment: 'us-west-v1',
    probeModelChangedAt: null,
    dailyBaseCostUpperBoundUsd: null,
    priceUnavailable: true,
    measurementVersion: 1,
    version: 1,
    referencedServices: [],
    healthSummary: emptySummary(probeModel),
    createdAt: timestamp,
    updatedAt: timestamp,
  }
  mockConnections.set(connection.id, connection)
  mockCredentials.set(connection.id, credential)
  return clone(connection)
}

export async function updateOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput & { id: string, version: number }) {
  if (shouldUseRealBackend()) return backendUpdateOwnerAPIProbeConnection(input)
  const current = mockConnections.get(input.id)
  if (!current || current.version !== input.version) throw new Error('探针连接已更新，请刷新后重试。')
  const target = normalizeMockBaseURL(input.baseUrl)
  if (target.raw.startsWith('http://') && !input.acknowledgeInsecureHttp) throw new Error('请先确认 HTTP 未加密传输风险。')
  const credential = input.credential?.trim()
  const effectiveCredential = credential || mockCredentials.get(input.id) || ''
  const probeModel = input.probeModel || current.probeModel
  const mustVerify = target.canonical !== current.normalizedBaseUrl || Boolean(credential) || probeModel !== current.probeModel || (input.enabled && !current.enabled)
  if (mustVerify) consumeMockPreflight(input, target.canonical, effectiveCredential, probeModel ?? '')
  const verification = mustVerify ? mockVerification(effectiveCredential) : {
    verificationStatus: current.verificationStatus,
    verifiedAt: current.verifiedAt,
    lastVerificationErrorCode: current.lastVerificationErrorCode,
  }
  const updated: OwnerAPIProbeConnection = {
    ...current,
    name: input.name.trim(),
    baseUrl: target.raw,
    normalizedBaseUrl: target.canonical,
    credentialConfigured: current.credentialConfigured || Boolean(credential),
    enabled: input.enabled && verification.verificationStatus === 'verified',
    ...verification,
    probeModel,
    probeModelChangedAt: probeModel !== current.probeModel ? now() : current.probeModelChangedAt,
    healthSummary: {
      ...emptySummary(probeModel),
      probeModelChangedAt: probeModel !== current.probeModel ? now() : current.probeModelChangedAt,
    },
    measurementVersion: current.measurementVersion + (mustVerify ? 1 : 0),
    version: current.version + 1,
    updatedAt: now(),
  }
  mockConnections.set(input.id, updated)
  if (credential) mockCredentials.set(input.id, credential)
  return clone(updated)
}

export async function deleteOwnerAPIProbeConnection(input: { id: string, version: number }) {
  if (shouldUseRealBackend()) return backendDeleteOwnerAPIProbeConnection(input)
  const current = mockConnections.get(input.id)
  if (!current || current.version !== input.version) throw new Error('探针连接已更新，请刷新后重试。')
  if (current.referencedServices.length) throw new Error('该连接仍被 API 服务引用，请先改绑或解绑。')
  mockConnections.delete(input.id)
  mockCredentials.delete(input.id)
}

export async function verifyOwnerAPIProbeConnection(input: { id: string, version: number }) {
  if (shouldUseRealBackend()) return backendVerifyOwnerAPIProbeConnection(input)
  const current = mockConnections.get(input.id)
  if (!current || current.version !== input.version) throw new Error('探针连接已更新，请刷新后重试。')
  const updated: OwnerAPIProbeConnection = {
    ...current,
    verificationStatus: 'verified',
    verifiedAt: now(),
    lastVerificationErrorCode: null,
    version: current.version + 1,
    updatedAt: now(),
  }
  mockConnections.set(input.id, updated)
  return clone(updated)
}

export function resetMockAPIProbeConnections() {
  mockConnections.clear()
  mockCredentials.clear()
  mockPreflightTokens.clear()
}

export function updateMockAPIProbeConnectionReference(input: {
  previousConnectionId?: string
  connectionId?: string
  serviceId: string
  serviceTitle: string
}) {
  if (input.previousConnectionId) {
    const previous = mockConnections.get(input.previousConnectionId)
    if (previous) {
      previous.referencedServices = previous.referencedServices.filter(service => service.id !== input.serviceId)
    }
  }
  if (input.connectionId) {
    const next = mockConnections.get(input.connectionId)
    if (next && !next.referencedServices.some(service => service.id === input.serviceId)) {
      next.referencedServices = [...next.referencedServices, { id: input.serviceId, title: input.serviceTitle }]
    }
  }
}

export const getAdminAPIProbeCalibration = backendAdminAPIProbeCalibration
export const getAdminAPIProbeLatencyRules = backendAdminAPIProbeLatencyRules
export const previewAdminAPIProbeLatencyRule = backendPreviewAdminAPIProbeLatencyRule
export const publishAdminAPIProbeLatencyRule = (input: ApiProbeLatencyRuleInput) => backendPublishAdminAPIProbeLatencyRule(input)
