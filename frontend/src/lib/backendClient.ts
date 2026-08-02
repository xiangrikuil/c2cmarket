import { requireApiMode, type ApiMode } from '@/lib/apiMode'
import { clearAnalyticsIdentity, identifyAnalyticsUser } from '@/lib/analytics'
import { captureRegistrationAttribution, getRegistrationAttribution } from '@/lib/registrationAttribution'

type ProblemDetails = {
  title?: string
  status?: number
  code?: string
  detail?: string
  errors?: Array<{ field?: string, code?: string, message?: string }>
  requestId?: string
}

export type BackendSessionUser = {
  id: string
  analyticsUserId: string
  username: string
  displayName: string
  isAdmin: boolean
  permissions: string[]
  linuxDoBinding: {
    bound: boolean
    linuxDoUserId?: string
    linuxDoUsername?: string
    trustLevel?: number
    avatarUrl?: string
  }
}

export type BackendSession = {
  user: BackendSessionUser
  csrfToken: string
  expiresAt: string
}

export type OAuthStartResponse = {
  authorizationUrl: string
}

export type PasswordLoginRequest = {
  username: string
  password: string
}

export class BackendProblemError extends Error {
  status: number
  code: string
  detail: string
  fieldErrors: NonNullable<ProblemDetails['errors']>

  constructor(problem: ProblemDetails, fallbackStatus: number) {
    super(problem.detail || problem.title || `HTTP ${fallbackStatus}`)
    this.name = 'BackendProblemError'
    this.status = problem.status ?? fallbackStatus
    this.code = problem.code ?? 'UNKNOWN_ERROR'
    this.detail = problem.detail ?? problem.title ?? ''
    this.fieldErrors = problem.errors ?? []
  }
}

let runtimeApiMode: ApiMode | null = null
let runtimeBaseURL = ''
const SESSION_REFRESH_GRACE_MS = 60_000
const SESSION_CACHE_INVALIDATION_CODES = new Set([
  'CSRF_TOKEN_INVALID',
  'SESSION_EXPIRED',
  'SESSION_REVOKED',
])
const SESSION_LOGIN_REQUIRED_CODES = new Set([
  'SESSION_EXPIRED',
  'SESSION_REVOKED',
])

let csrfToken: string | null = null
let cachedSession: BackendSession | null = null
let sessionRequest: Promise<BackendSession> | null = null
const pendingGetRequests = new Map<string, Promise<unknown>>()
const sessionInvalidationHandlers = new Set<(error: BackendProblemError) => void>()

export function shouldUseRealBackend() {
  if (runtimeApiMode === null) {
    if (import.meta.env.MODE === 'test') return false
    throw new Error('Backend runtime config has not initialized NUXT_PUBLIC_API_MODE.')
  }
  return runtimeApiMode === 'real'
}

export function backendBaseURL() {
  return runtimeBaseURL.replace(/\/$/, '')
}

export function setBackendRuntimeConfig(config: { apiMode?: string, apiBaseUrl?: string }) {
  runtimeApiMode = requireApiMode(config.apiMode)
  runtimeBaseURL = config.apiBaseUrl?.trim() ?? ''
}

export function setBackendCSRFToken(token: string | null) {
  csrfToken = token
}

export function getBackendCSRFToken() {
  return csrfToken
}

function cacheBackendSession(session: BackendSession) {
  cachedSession = session
  setBackendCSRFToken(session.csrfToken)
  identifyAnalyticsUser(session.user.analyticsUserId)
  return session
}

function clearBackendSessionCache() {
  cachedSession = null
  sessionRequest = null
  setBackendCSRFToken(null)
  clearAnalyticsIdentity()
}

function hasUsableCachedSession(now = Date.now()) {
  if (!cachedSession) return false
  const expiresAt = Date.parse(cachedSession.expiresAt)
  return Number.isFinite(expiresAt) && expiresAt > now + SESSION_REFRESH_GRACE_MS
}

function isSessionCacheInvalidationError(error: unknown) {
  return error instanceof BackendProblemError && SESSION_CACHE_INVALIDATION_CODES.has(error.code)
}

function isCSRFTokenInvalidError(error: unknown) {
  return error instanceof BackendProblemError && error.code === 'CSRF_TOKEN_INVALID'
}

function clearBackendSessionCacheOnAuthError(error: unknown, notifySessionInvalidation = true) {
  if (!(error instanceof BackendProblemError)) return
  const hadCachedSession = cachedSession !== null
  if (isSessionCacheInvalidationError(error)) clearBackendSessionCache()
  if (notifySessionInvalidation && hadCachedSession && SESSION_LOGIN_REQUIRED_CODES.has(error.code)) {
    for (const handler of sessionInvalidationHandlers) {
      handler(error)
    }
  }
}

export function subscribeToBackendSessionInvalidation(handler: (error: BackendProblemError) => void) {
  sessionInvalidationHandlers.add(handler)
  return () => {
    sessionInvalidationHandlers.delete(handler)
  }
}

function requestMethod(init: RequestInit) {
  return (init.method ?? 'GET').toUpperCase()
}

function shouldCoalesceRequest(init: RequestInit) {
  return requestMethod(init) === 'GET' && init.body === undefined
}

function coalesceKey(path: string, init: RequestInit) {
  const headers = new Headers(init.headers)
  return `${backendBaseURL()}${path}|${headers.get('accept') ?? ''}`
}

export async function getCurrentBackendSession(options: {
  forceRefresh?: boolean
  notifySessionInvalidation?: boolean
} = {}) {
  if (!options.forceRefresh && hasUsableCachedSession()) {
    return cachedSession!
  }
  if (sessionRequest) {
    return sessionRequest
  }

  sessionRequest = backendRequest<BackendSession>('/api/v1/auth/session', {}, {
    notifySessionInvalidation: options.notifySessionInvalidation,
  })
    .then(cacheBackendSession)
    .catch(error => {
      clearBackendSessionCache()
      throw error
    })
    .finally(() => {
      sessionRequest = null
    })
  return sessionRequest
}

export async function startOAuthLogin(returnTo = '/') {
  const params = new URLSearchParams()
  if (returnTo) params.set('returnTo', returnTo)
  const attribution = getRegistrationAttribution() ?? captureRegistrationAttribution()
  if (attribution?.source) params.set('utmSource', attribution.source)
  if (attribution?.medium) params.set('utmMedium', attribution.medium)
  if (attribution?.campaign) params.set('utmCampaign', attribution.campaign)
  if (attribution?.referrerHost) params.set('referrerHost', attribution.referrerHost)
  if (attribution?.landingPath) params.set('landingPath', attribution.landingPath)
  return backendRequest<OAuthStartResponse>(`/api/v1/auth/oauth/start?${params.toString()}`)
}

export async function loginWithPassword(payload: PasswordLoginRequest) {
  const session = await backendJSON<BackendSession>('/api/v1/auth/password/login', payload)
  return cacheBackendSession(session)
}

export async function logoutBackendSession() {
  await backendMutation<void>('/api/v1/auth/logout', {}, { method: 'POST' })
  clearBackendSessionCache()
}

function jsonHeaders(headers: HeadersInit = {}) {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    ...headers,
  }
}

function idempotencyKey(prefix: string) {
  const random = typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}-${random}`
}

async function decodeResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) return undefined as T
  const text = await response.text()
  const contentType = response.headers.get('content-type') ?? ''
  const data = text ? JSON.parse(text) : null
  if (!response.ok) {
    if (contentType.includes('application/problem+json')) {
      throw new BackendProblemError(data as ProblemDetails, response.status)
    }
    throw new BackendProblemError({ title: response.statusText, status: response.status, detail: text }, response.status)
  }
  return data as T
}

export async function backendRequest<T>(
  path: string,
  init: RequestInit = {},
  options: { notifySessionInvalidation?: boolean } = {},
) {
  const requestInit = {
    ...init,
    credentials: 'include' as const,
    headers: {
      Accept: 'application/json',
      ...(init.headers ?? {}),
    },
  }
  try {
    if (shouldCoalesceRequest(requestInit)) {
      const key = coalesceKey(path, requestInit)
      const pending = pendingGetRequests.get(key)
      if (pending) return await pending as T
      const request = fetch(`${backendBaseURL()}${path}`, requestInit)
        .then(response => decodeResponse<T>(response))
        .finally(() => {
          pendingGetRequests.delete(key)
        })
      pendingGetRequests.set(key, request)
      return await request
    }

    const response = await fetch(`${backendBaseURL()}${path}`, requestInit)
    return await decodeResponse<T>(response)
  } catch (error) {
    clearBackendSessionCacheOnAuthError(error, options.notifySessionInvalidation !== false)
    throw error
  }
}

export async function backendJSON<T>(path: string, body: unknown, init: RequestInit = {}) {
  return backendRequest<T>(path, {
    ...init,
    method: init.method ?? 'POST',
    headers: jsonHeaders(init.headers),
    body: JSON.stringify(body ?? {}),
  })
}

export async function backendMutation<T>(path: string, body: unknown, options: {
  method?: 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  idempotencyPrefix?: string
  ifMatch?: number | string
} = {}) {
  try {
    return await backendJSON<T>(path, body ?? {}, {
      method: options.method ?? 'POST',
      headers: backendMutationHeaders(options),
    })
  } catch (error) {
    if (!isCSRFTokenInvalidError(error)) throw error
    await getCurrentBackendSession({ forceRefresh: true })
    return backendJSON<T>(path, body ?? {}, {
      method: options.method ?? 'POST',
      headers: backendMutationHeaders(options),
    })
  }
}

export async function backendFormDataMutation<T>(path: string, body: FormData, options: {
  idempotencyPrefix?: string
  ifMatch?: number | string
} = {}) {
  const request = () => backendRequest<T>(path, {
    method: 'POST',
    headers: backendMutationHeaders(options),
    body,
  })
  try {
    return await request()
  } catch (error) {
    if (!isCSRFTokenInvalidError(error)) throw error
    await getCurrentBackendSession({ forceRefresh: true })
    return request()
  }
}

export async function ensureBackendSession(
  username = 'orbit',
  admin = false,
  options: { notifySessionInvalidation?: boolean } = {},
) {
  try {
    const current = await getCurrentBackendSession({
      notifySessionInvalidation: options.notifySessionInvalidation,
    })
    if (shouldUseRealBackend()) {
      if (!admin || current.user.isAdmin) return current
      throw new BackendProblemError({
        title: 'Session role mismatch',
        status: 403,
        code: 'PERMISSION_DENIED',
        detail: admin ? '当前账号没有管理权限，请使用管理员账号登录。' : '当前登录账号与操作要求不匹配。',
      }, 403)
    }
    if (current.user.username === username && current.user.isAdmin === admin) {
      return current
    }
  } catch (error) {
    if (shouldUseRealBackend()) throw error
  }
  const created = await backendJSON<BackendSession>('/api/v1/auth/dev-session', { username, admin })
  return cacheBackendSession(created)
}

function backendMutationHeaders(options: {
  idempotencyPrefix?: string
  ifMatch?: number | string
}) {
  const headers: Record<string, string> = {}
  if (csrfToken) headers['X-CSRF-Token'] = csrfToken
  if (options.idempotencyPrefix) headers['Idempotency-Key'] = idempotencyKey(options.idempotencyPrefix)
  if (options.ifMatch !== undefined) headers['If-Match'] = `"${options.ifMatch}"`
  return headers
}

export async function requireBackendSession() {
  try {
    return await getCurrentBackendSession()
  } catch (error) {
    if (shouldUseRealBackend()) throw error
    return ensureBackendSession()
  }
}

export function backendErrorMessage(error: unknown, fallback: string) {
  if (error instanceof BackendProblemError) return error.detail || error.message || fallback
  if (error instanceof Error) return error.message
  return fallback
}
