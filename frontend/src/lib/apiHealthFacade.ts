import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  backendCreateOwnerAPIProbeConnection,
  backendDeleteOwnerAPIProbeConnection,
  backendOwnerAPIProbeConnection,
  backendOwnerAPIProbeConnections,
  backendUpdateOwnerAPIProbeConnection,
  backendVerifyOwnerAPIProbeConnection,
} from '@/lib/apiHealthBackend'
import type { OwnerAPIProbeConnection, SaveOwnerAPIProbeConnectionInput } from '@/types/apiHealth'

const mockConnections = new Map<string, OwnerAPIProbeConnection>()

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
  const connection: OwnerAPIProbeConnection = {
    id: crypto.randomUUID(),
    name,
    baseUrl: target.raw,
    normalizedBaseUrl: target.canonical,
    credentialConfigured: true,
    enabled: input.enabled && verification.verificationStatus === 'verified',
    ...verification,
    measurementVersion: 1,
    version: 1,
    referencedServices: [],
    createdAt: timestamp,
    updatedAt: timestamp,
  }
  mockConnections.set(connection.id, connection)
  return clone(connection)
}

export async function updateOwnerAPIProbeConnection(input: SaveOwnerAPIProbeConnectionInput & { id: string, version: number }) {
  if (shouldUseRealBackend()) return backendUpdateOwnerAPIProbeConnection(input)
  const current = mockConnections.get(input.id)
  if (!current || current.version !== input.version) throw new Error('探针连接已更新，请刷新后重试。')
  const target = normalizeMockBaseURL(input.baseUrl)
  if (target.raw.startsWith('http://') && !input.acknowledgeInsecureHttp) throw new Error('请先确认 HTTP 未加密传输风险。')
  const credential = input.credential?.trim()
  const mustVerify = target.canonical !== current.normalizedBaseUrl || Boolean(credential) || (input.enabled && !current.enabled)
  const verification = mustVerify ? mockVerification(credential) : {
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
    measurementVersion: current.measurementVersion + (mustVerify ? 1 : 0),
    version: current.version + 1,
    updatedAt: now(),
  }
  mockConnections.set(input.id, updated)
  return clone(updated)
}

export async function deleteOwnerAPIProbeConnection(input: { id: string, version: number }) {
  if (shouldUseRealBackend()) return backendDeleteOwnerAPIProbeConnection(input)
  const current = mockConnections.get(input.id)
  if (!current || current.version !== input.version) throw new Error('探针连接已更新，请刷新后重试。')
  if (current.referencedServices.length) throw new Error('该连接仍被 API 服务引用，请先改绑或解绑。')
  mockConnections.delete(input.id)
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
