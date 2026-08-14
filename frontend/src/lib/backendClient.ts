import { requireApiMode, type ApiMode } from '@/lib/apiMode'
import { clearAnalyticsIdentity, identifyAnalyticsUser } from '@/lib/analytics'
import {
  captureRegistrationAttribution,
  clearRegistrationAttribution,
  getRegistrationAttribution,
  type RegistrationAttribution,
} from '@/lib/registrationAttribution'
import { getReferralCapture } from '@/lib/referralCapture'
import { CAPABILITY, hasCapability } from '@/lib/capabilities'
import {
  MockAuthProblem,
  confirmMockEmailRegistration,
  confirmMockPasswordReset,
  getMockIdentity,
  linkMockLinuxDo,
  loginMockWithPassword,
  mockEmailRegistrationConfig,
  reauthenticateMockPassword,
  requireMockIdentity,
  setMockPersona,
  startMockEmailRegistration,
  startMockPasswordReset,
  type MockIdentity,
  type MockPersona,
} from '@/lib/mockAuth'
import type {
	AccountAppealSessionResponse,
	AccountGovernanceBusinessCenter,
  AccountGovernanceAppeal as AccountGovernanceAppealResponse,
  EmailRegistrationConfirmRequest,
  EmailRegistrationStartResponse,
  OAuthStartResponse,
  PasswordLoginRequest,
  PasswordResetConfirmRequest,
  PasswordResetStartRequest,
  PasswordResetStartResponse,
  SessionResponse,
  StudentRegistrationPublicConfig,
  User,
  UsernameAvailability,
} from '@/api/generated/openapi'

type ProblemDetails = {
  title?: string
  status?: number
  code?: string
  detail?: string
  errors?: Array<{ field?: string, code?: string, message?: string }>
  requestId?: string
}

export type BackendSessionUser = User
export type BackendSession = SessionResponse

export type DevPersona = 'buyer' | 'seller' | 'admin'

export type DevPersonaSession = BackendSession & {
  persona: DevPersona
}

export type {
  OAuthStartResponse,
  PasswordLoginRequest,
  PasswordResetConfirmRequest,
  PasswordResetStartRequest,
  PasswordResetStartResponse,
}

export type AccountAppealSession = AccountAppealSessionResponse
export type AccountGovernanceAppeal = AccountGovernanceAppealResponse
export type AccountGovernanceBusinessCenterResponse = AccountGovernanceBusinessCenter

export type EmailRegistrationConfig = StudentRegistrationPublicConfig
export type EmailRegistrationChallenge = EmailRegistrationStartResponse
export type UsernameAvailabilityResult = UsernameAvailability

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
let accountAppealCSRFToken: string | null = null
let restrictedBusinessCSRFToken: string | null = null
let cachedSession: BackendSession | null = null
let cachedRestrictedBusinessSession: BackendSession | null = null
let sessionRequest: Promise<BackendSession> | null = null
let sessionGeneration = 0
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
	if (session.audience !== 'normal') {
		throw new Error('Normal session endpoint returned a non-normal audience.')
	}
  cachedSession = session
  setBackendCSRFToken(session.csrfToken)
  identifyAnalyticsUser(session.user.analyticsUserId)
  return session
}

function cacheRestrictedBusinessSession(session: BackendSession) {
	if (session.audience !== 'restricted_business') {
		throw new Error('Restricted-business endpoint returned an invalid audience.')
	}
	cachedRestrictedBusinessSession = session
	restrictedBusinessCSRFToken = session.csrfToken
	return session
}

function invalidateInFlightSessionRequests() {
  sessionGeneration += 1
  sessionRequest = null
}

function replaceBackendSession(session: BackendSession) {
  invalidateInFlightSessionRequests()
  return cacheBackendSession(session)
}

function clearBackendSessionCache() {
  sessionGeneration += 1
  cachedSession = null
  sessionRequest = null
  setBackendCSRFToken(null)
  clearAnalyticsIdentity()
}

function backendProblemFromMock(error: unknown): never {
  if (error instanceof MockAuthProblem) {
    throw new BackendProblemError({
      status: error.status,
      code: error.code,
      detail: error.detail,
    }, error.status)
  }
  throw error
}

function mockSession(identity: MockIdentity): BackendSession {
  return {
		audience: 'normal',
    csrfToken: `mock-csrf-${identity.persona}`,
    expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
    user: {
      id: identity.id,
      analyticsUserId: identity.analyticsUserId,
      username: identity.username,
      displayName: identity.displayName,
      isAdmin: hasCapability(identity, CAPABILITY.adminAccess),
      permissions: [...identity.permissions],
      capabilities: [...identity.capabilities],
      studentClaim: identity.studentClaim ? { ...identity.studentClaim } : null,
      linuxDoBinding: { ...identity.linuxDoBinding },
    },
  }
}

function currentMockSession() {
  try {
    return mockSession(requireMockIdentity())
  } catch (error) {
    return backendProblemFromMock(error)
  }
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

function coalesceKey(path: string, init: RequestInit, generation = sessionGeneration) {
  const headers = new Headers(init.headers)
  return `${generation}|${backendBaseURL()}${path}|${headers.get('accept') ?? ''}`
}

export async function getCurrentBackendSession(options: {
  forceRefresh?: boolean
  notifySessionInvalidation?: boolean
} = {}) {
  if (!shouldUseRealBackend()) {
    if (!options.forceRefresh && hasUsableCachedSession()) return cachedSession!
    return cacheBackendSession(currentMockSession())
  }
  if (!options.forceRefresh && hasUsableCachedSession()) {
    return cachedSession!
  }
  if (sessionRequest) {
    return sessionRequest
  }

  const requestGeneration = sessionGeneration
  const request = backendRequest<BackendSession>('/api/v1/auth/session', {}, {
    affectsSessionCache: false,
  })
    .then((session) => {
      if (requestGeneration !== sessionGeneration) {
        if (cachedSession) return cachedSession
        throw new Error('Backend session changed while the session request was in flight.')
      }
      return cacheBackendSession(session)
    })
    .catch(error => {
      if (requestGeneration !== sessionGeneration && cachedSession) return cachedSession
      clearBackendSessionCacheOnAuthError(error, options.notifySessionInvalidation !== false)
      clearBackendSessionCache()
      throw error
    })
    .finally(() => {
      if (sessionRequest === request) sessionRequest = null
    })
  sessionRequest = request
  return request
}

export async function startOAuthLogin(returnTo = '/', inviteCode = getReferralCapture()) {
  if (!shouldUseRealBackend()) {
    setMockPersona('linuxdo')
    replaceBackendSession(currentMockSession())
    return { authorizationUrl: returnTo || '/' }
  }
  const params = new URLSearchParams()
  if (returnTo) params.set('returnTo', returnTo)
  if (inviteCode) params.set('inviteCode', inviteCode)
  const attribution = getRegistrationAttribution() ?? captureRegistrationAttribution()
  if (attribution?.source) params.set('utmSource', attribution.source)
  if (attribution?.medium) params.set('utmMedium', attribution.medium)
  if (attribution?.campaign) params.set('utmCampaign', attribution.campaign)
  if (attribution?.referrerHost) params.set('referrerHost', attribution.referrerHost)
  if (attribution?.landingPath) params.set('landingPath', attribution.landingPath)
  return backendRequest<OAuthStartResponse>(`/api/v1/auth/oauth/start?${params.toString()}`)
}

export async function startAccountAppealVerification() {
  accountAppealCSRFToken = null
  return backendRequest<OAuthStartResponse>('/api/v1/auth/account-appeal/start', {}, {
    affectsSessionCache: false,
  })
}

export async function getAccountAppealSession() {
  try {
    const session = await backendRequest<AccountAppealSession>('/api/v1/account-appeal/session', {}, {
      affectsSessionCache: false,
    })
    accountAppealCSRFToken = session.csrfToken
    return session
  } catch (error) {
    accountAppealCSRFToken = null
    throw error
  }
}

export async function submitAccountGovernanceAppeal(statement: string) {
  if (!accountAppealCSRFToken) {
    throw new BackendProblemError({
      title: 'Account appeal verification required',
      status: 401,
      code: 'ACCOUNT_APPEAL_SESSION_REQUIRED',
      detail: '请先通过 linux.do 验证受限账号。',
    }, 401)
  }

  const requestKey = idempotencyKey('account-appeal-create')
  const request = () => backendRequest<AccountGovernanceAppeal>('/api/v1/account-appeal/appeals', {
    method: 'POST',
    headers: jsonHeaders({
      'X-Account-Appeal-CSRF': accountAppealCSRFToken ?? '',
      'Idempotency-Key': requestKey,
    }),
    body: JSON.stringify({ statement }),
  }, {
    affectsSessionCache: false,
  })

  try {
    const appeal = await request()
    accountAppealCSRFToken = null
    return appeal
  } catch (error) {
    if (!isCSRFTokenInvalidError(error)) {
      if (error instanceof BackendProblemError && error.status === 401) accountAppealCSRFToken = null
      throw error
    }
    await getAccountAppealSession()
    try {
      const appeal = await request()
      accountAppealCSRFToken = null
      return appeal
    } catch (retryError) {
      if (retryError instanceof BackendProblemError && retryError.status === 401) accountAppealCSRFToken = null
      throw retryError
    }
  }
}

export async function loginWithPassword(payload: PasswordLoginRequest) {
  if (!shouldUseRealBackend()) {
    try {
      return replaceBackendSession(mockSession(loginMockWithPassword(payload.username, payload.password)))
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
	const session = await backendJSON<BackendSession>('/api/v1/auth/password/login', payload)
	if (session.audience === 'restricted_business') return cacheRestrictedBusinessSession(session)
	return replaceBackendSession(session)
}

export async function getEmailRegistrationConfig(): Promise<EmailRegistrationConfig> {
  if (!shouldUseRealBackend()) return mockEmailRegistrationConfig()
  return backendRequest<EmailRegistrationConfig>('/api/v1/auth/email-registration/config', {}, {
    affectsSessionCache: false,
  })
}

export async function checkUsernameAvailability(username: string): Promise<UsernameAvailabilityResult> {
  if (!shouldUseRealBackend()) {
    const currentUsername = getMockIdentity()?.username
    const unavailable = new Set(['admin', 'orbit', 'student-buyer'])
    if (currentUsername) unavailable.add(currentUsername)
    return { username, available: !unavailable.has(username) }
  }
  const params = new URLSearchParams({ username })
  return backendRequest<UsernameAvailabilityResult>(`/api/v1/auth/username-availability?${params.toString()}`, {}, {
    affectsSessionCache: false,
  })
}

export async function startEmailRegistration(payload: {
  email: string
  turnstileToken: string
}): Promise<EmailRegistrationChallenge> {
  if (!shouldUseRealBackend()) {
    try {
      return startMockEmailRegistration(payload.email, payload.turnstileToken)
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
  return backendJSON<EmailRegistrationChallenge>('/api/v1/auth/email-registration/start', payload)
}

export async function confirmEmailRegistration(
  payload: Omit<EmailRegistrationConfirmRequest, 'attribution'>,
): Promise<BackendSession> {
  if (!shouldUseRealBackend()) {
    try {
      const session = replaceBackendSession(mockSession(confirmMockEmailRegistration(payload)))
      clearRegistrationAttribution()
      return session
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
  const attribution = getRegistrationAttribution() ?? captureRegistrationAttribution()
  const request: EmailRegistrationConfirmRequest = {
    ...payload,
    attribution: attribution ?? {},
  }
  const session = await backendJSON<BackendSession>('/api/v1/auth/email-registration/confirm', request)
  clearRegistrationAttribution()
  return replaceBackendSession(session)
}

export async function startPasswordReset(
  payload: PasswordResetStartRequest,
): Promise<PasswordResetStartResponse> {
  if (!shouldUseRealBackend()) {
    try {
      return startMockPasswordReset(payload.email, payload.turnstileToken)
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
  return backendRequest<PasswordResetStartResponse>('/api/v1/auth/password-reset/start', {
    method: 'POST',
    headers: jsonHeaders(),
    body: JSON.stringify(payload),
  }, { affectsSessionCache: false })
}

export async function confirmPasswordReset(payload: PasswordResetConfirmRequest): Promise<void> {
  if (!shouldUseRealBackend()) {
    try {
      confirmMockPasswordReset(payload)
      clearBackendSessionCache()
      return
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
  await backendRequest<void>('/api/v1/auth/password-reset/confirm', {
    method: 'POST',
    headers: jsonHeaders(),
    body: JSON.stringify(payload),
  }, { affectsSessionCache: false })
  clearBackendSessionCache()
}

export async function reauthenticatePassword(password: string, purpose?: 'grant_admin'): Promise<void> {
  if (!shouldUseRealBackend()) {
    try {
      reauthenticateMockPassword(password)
      return
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
	await backendMutation<void>('/api/v1/auth/password/reauthenticate', { password, ...(purpose ? { purpose } : {}) })
}

export async function startAdminGrantReauthentication(returnTo = '/admin/users'): Promise<OAuthStartResponse> {
  if (!shouldUseRealBackend()) {
    return { authorizationUrl: returnTo }
  }
  const params = new URLSearchParams({ purpose: 'grant_admin_reauth', returnTo })
  return backendRequest<OAuthStartResponse>(`/api/v1/auth/oauth/start?${params.toString()}`, {}, {
    affectsSessionCache: false,
  })
}

export async function getRestrictedBusinessSession(options: { forceRefresh?: boolean } = {}) {
	if (!shouldUseRealBackend()) throw new Error('Restricted business requires the real backend.')
	if (!options.forceRefresh && cachedRestrictedBusinessSession) return cachedRestrictedBusinessSession
	const session = await backendRequest<BackendSession>('/api/v1/auth/restricted-business/session', {}, { affectsSessionCache: false })
	return cacheRestrictedBusinessSession(session)
}

export async function getAccountGovernanceBusinessCenter(audience: BackendSession['audience'] = 'normal') {
	if (!shouldUseRealBackend()) {
		return {
			generatedAt: new Date().toISOString(),
			accountStatus: 'active',
			processingStatus: 'completed',
			currentAction: null,
			items: [],
		} satisfies AccountGovernanceBusinessCenterResponse
	}
	return backendRequest<AccountGovernanceBusinessCenterResponse>('/api/v1/me/account-governance/business-center', {
		headers: { 'X-Session-Audience': audience },
	}, { affectsSessionCache: audience === 'normal' })
}

export async function logoutRestrictedBusinessSession() {
	if (!shouldUseRealBackend()) return
	await backendJSON<void>('/api/v1/auth/restricted-business/logout', {}, {
		headers: restrictedBusinessCSRFToken ? { 'X-Restricted-Business-CSRF': restrictedBusinessCSRFToken } : {},
	})
	cachedRestrictedBusinessSession = null
	restrictedBusinessCSRFToken = null
}

export async function startLinuxDoLink(returnTo = '/my/account'): Promise<OAuthStartResponse> {
  if (!shouldUseRealBackend()) {
    try {
      linkMockLinuxDo()
      clearBackendSessionCache()
      return { authorizationUrl: returnTo }
    } catch (error) {
      return backendProblemFromMock(error)
    }
  }
  const params = new URLSearchParams({ purpose: 'link_linuxdo', returnTo })
  return backendRequest<OAuthStartResponse>(`/api/v1/auth/oauth/start?${params.toString()}`)
}

export async function createMockPersonaSession(persona: MockPersona) {
  if (shouldUseRealBackend()) throw new Error('Mock persona switching requires mock API mode.')
  setMockPersona(persona)
  clearBackendSessionCache()
  const identity = getMockIdentity()
  return identity ? replaceBackendSession(mockSession(identity)) : null
}

export async function createDevPersonaSession(persona: DevPersona) {
  if (!shouldUseRealBackend()) {
    throw new Error('Development persona switching requires real API mode.')
  }
  invalidateInFlightSessionRequests()
  const session = await backendJSON<DevPersonaSession>('/api/v1/auth/dev-persona-session', { persona })
  return replaceBackendSession(session)
}

export async function logoutBackendSession() {
  if (!shouldUseRealBackend()) {
    setMockPersona('anonymous')
    clearBackendSessionCache()
    return
  }
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
  options: { notifySessionInvalidation?: boolean, affectsSessionCache?: boolean } = {},
) {
  const requestGeneration = sessionGeneration
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
      const key = coalesceKey(path, requestInit, requestGeneration)
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
    if (options.affectsSessionCache !== false && requestGeneration === sessionGeneration) {
      clearBackendSessionCacheOnAuthError(error, options.notifySessionInvalidation !== false)
    }
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
  signal?: AbortSignal
} = {}) {
  const mutationGeneration = sessionGeneration
  try {
    return await backendJSON<T>(path, body ?? {}, {
      method: options.method ?? 'POST',
      headers: backendMutationHeaders(options),
      signal: options.signal,
    })
  } catch (error) {
    if (!isCSRFTokenInvalidError(error)) throw error
    if (mutationGeneration !== sessionGeneration && cachedSession) throw error
    await getCurrentBackendSession({ forceRefresh: true })
    return backendJSON<T>(path, body ?? {}, {
      method: options.method ?? 'POST',
      headers: backendMutationHeaders(options),
      signal: options.signal,
    })
  }
}

export async function backendFormDataMutation<T>(path: string, body: FormData, options: {
  idempotencyPrefix?: string
  ifMatch?: number | string
} = {}) {
  const mutationGeneration = sessionGeneration
  const request = () => backendRequest<T>(path, {
    method: 'POST',
    headers: backendMutationHeaders(options),
    body,
  })
  try {
    return await request()
  } catch (error) {
    if (!isCSRFTokenInvalidError(error)) throw error
    if (mutationGeneration !== sessionGeneration && cachedSession) throw error
    await getCurrentBackendSession({ forceRefresh: true })
    return request()
  }
}

export async function ensureBackendSession(
  username = 'orbit',
  admin = false,
  options: { notifySessionInvalidation?: boolean } = {},
) {
  void username
  const current = await getCurrentBackendSession({
    notifySessionInvalidation: options.notifySessionInvalidation,
  })
  if (!admin || hasCapability(current.user, CAPABILITY.adminAccess)) return current
  throw new BackendProblemError({
    title: 'Session role mismatch',
    status: 403,
    code: 'PERMISSION_DENIED',
    detail: '当前账号没有管理权限，请使用管理员账号登录。',
  }, 403)
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
  return getCurrentBackendSession()
}

export function backendErrorMessage(error: unknown, fallback: string) {
  if (error instanceof BackendProblemError) return error.detail || error.message || fallback
  if (error instanceof Error) return error.message
  return fallback
}
