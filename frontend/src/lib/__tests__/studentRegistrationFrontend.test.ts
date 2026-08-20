import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

const loginSource = readFileSync(new URL('../../pages/LoginPage.vue', import.meta.url), 'utf8')
const registrationSource = readFileSync(new URL('../../components/auth/StudentRegistrationPanel.vue', import.meta.url), 'utf8')
const myCenterSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')
const adminPageSource = readFileSync(new URL('../../pages/AdminStudentRegistrationPage.vue', import.meta.url), 'utf8')
const adminBackendSource = readFileSync(new URL('../studentRegistrationAdminBackend.ts', import.meta.url), 'utf8')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': status >= 400 ? 'application/problem+json' : 'application/json' },
  })
}

function studentSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-student',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'student-user',
      analyticsUserId: 'analytics-student',
      username: 'student_buyer',
      displayName: '学生买家',
      isAdmin: false,
      permissions: [],
      capabilities: ['api_order.create'],
      linuxDoBinding: { bound: false },
    },
  }
}

function adminSession() {
  return {
		audience: 'normal',
    csrfToken: 'csrf-admin',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'admin-user',
      analyticsUserId: 'analytics-admin',
      username: 'admin',
      displayName: '管理员',
      isAdmin: true,
      permissions: ['admin'],
      capabilities: ['admin.access'],
      linuxDoBinding: { bound: true },
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('registration page uses the two-step contract and resets one-time Turnstile after every start attempt', () => {
  assert.match(loginSource, /type AuthMode = 'login' \| 'student-register'/)
  assert.match(loginSource, /router\.push\(returnTo\.value\)/)
  assert.doesNotMatch(loginSource, /wechatOnboarding/)
  assert.match(registrationSource, /type RegistrationStep = 'email' \| 'account' \| 'completed'/)
  assert.match(registrationSource, /startEmailRegistration\(\{[\s\S]*email: submittedEmail,[\s\S]*turnstileToken: turnstileToken\.value/)
  assert.match(registrationSource, /finally \{[\s\S]*turnstileToken\.value = ''[\s\S]*turnstileWidget\.value\?\.reset\(\)/)
  assert.match(registrationSource, /action="student_signup"/)
  assert.match(registrationSource, /confirmEmailRegistration\(\{[\s\S]*email: canonicalEmail\.value,[\s\S]*code:[\s\S]*username: username\.value,[\s\S]*password:/)
  assert.match(registrationSource, /学生注册暂未开放/)
  assert.match(registrationSource, /requestGeneration/)
  assert.match(registrationSource, /pendingAction\.value === 'registration-start'\) pendingAction\.value = null/)
  assert.match(registrationSource, /if \(pendingAction\.value/)
  assert.match(registrationSource, /之前的验证码已失效/)
  assert.match(registrationSource, /@blur="checkUsernameOnBlur"/)
  assert.match(registrationSource, /type UsernameAvailabilityState = 'idle' \| 'checking' \| 'available' \| 'unavailable' \| 'error'/)
  assert.match(registrationSource, /generation !== usernameAvailabilityGeneration \|\| checkedUsername !== username\.value/)
  assert.match(registrationSource, /用户名可用/)
  assert.match(registrationSource, /该用户名已被占用/)
})

test('real username availability check encodes the username and leaves session state untouched', async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse({ username: 'student_name', available: true }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const result = await client.checkUsernameAvailability('student_name')

  assert.deepEqual(result, { username: 'student_name', available: true })
  assert.equal(fetchMock.mock.calls.length, 1)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/username-availability?username=student_name')
})

test('real registration calls use exact payloads, cache the returned session, and never fall back to mock', async () => {
  const responseSession = studentSession()
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({
      enabled: true,
      institutions: [{ domain: 'example.edu', institutionName: '示例大学' }],
    }))
    .mockResolvedValueOnce(jsonResponse({
      email: 'student@example.edu',
      expiresAt: '2026-08-12T12:15:00Z',
      devCode: '123456',
    }))
    .mockResolvedValueOnce(jsonResponse(responseSession))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const config = await client.getEmailRegistrationConfig()
  const challenge = await client.startEmailRegistration({ email: 'student@example.edu', turnstileToken: 'turnstile-once' })
  const session = await client.confirmEmailRegistration({
    email: challenge.email,
    code: '123456',
    username: 'student_buyer',
    password: 'Strong-password-1!',
  })

  assert.equal(config.enabled, true)
  assert.deepEqual(session, responseSession)
  assert.deepEqual(await client.getCurrentBackendSession(), responseSession)
  assert.equal(fetchMock.mock.calls.length, 3)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/email-registration/config')
  assert.deepEqual(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body)), {
    email: 'student@example.edu',
    turnstileToken: 'turnstile-once',
  })
  assert.deepEqual(JSON.parse(String((fetchMock.mock.calls[2]?.[1] as RequestInit).body)), {
    email: 'student@example.edu',
    code: '123456',
    username: 'student_buyer',
    password: 'Strong-password-1!',
    attribution: {},
  })
})

test('mock registration rechecks the persistent switch during confirmation', async () => {
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'mock' })
  await client.createMockPersonaSession('admin')
  const admin = await import('../studentRegistrationAdminBackend')
  const setting = await admin.getAdminStudentRegistrationSetting()
  if (!setting.enabled) {
    await admin.updateAdminStudentRegistrationSetting({ enabled: true, expectedVersion: setting.version, reason: '自动化注册验证' })
  }
  const enabledSetting = await admin.getAdminStudentRegistrationSetting()
  const challenge = await client.startEmailRegistration({ email: 'student@example.edu', turnstileToken: 'turnstile-once' })
  await admin.updateAdminStudentRegistrationSetting({ enabled: false, expectedVersion: enabledSetting.version, reason: '确认阶段关闭' })

  await assert.rejects(
    () => client.confirmEmailRegistration({
      email: challenge.email,
      code: challenge.devCode ?? '123456',
      username: 'student_buyer',
      password: 'Strong-password-1!',
    }),
    (error: unknown) => error instanceof client.BackendProblemError && error.code === 'EMAIL_REGISTRATION_DISABLED',
  )
})

test('real password reset uses generic start and no-session confirm contracts', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ accepted: true }, 202))
    .mockResolvedValueOnce(new Response(null, { status: 204 }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const accepted = await client.startPasswordReset({
    email: 'student@example.edu',
    turnstileToken: 'turnstile-reset',
  })
  await client.confirmPasswordReset({
    email: 'student@example.edu',
    code: '123456',
    newPassword: 'Changed-password-2!',
  })

  assert.deepEqual(accepted, { accepted: true })
  assert.equal(fetchMock.mock.calls.length, 2)
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/password-reset/start')
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/auth/password-reset/confirm')
  assert.deepEqual(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body)), {
    email: 'student@example.edu',
    turnstileToken: 'turnstile-reset',
  })
  assert.deepEqual(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body)), {
    email: 'student@example.edu',
    code: '123456',
    newPassword: 'Changed-password-2!',
  })
})

test('mock password reset is generic for unknown email and revokes the eligible student session', async () => {
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'mock' })
  await client.createMockPersonaSession('admin')
  const admin = await import('../studentRegistrationAdminBackend')
  const setting = await admin.getAdminStudentRegistrationSetting()
  if (!setting.enabled) {
    await admin.updateAdminStudentRegistrationSetting({ enabled: true, expectedVersion: setting.version, reason: '密码重置测试' })
  }
  const challenge = await client.startEmailRegistration({ email: 'student@example.edu', turnstileToken: 'turnstile-register' })
  await client.confirmEmailRegistration({
    email: challenge.email,
    code: challenge.devCode ?? '123456',
    username: 'student_reset',
    password: 'Strong-password-1!',
  })

  assert.deepEqual(await client.startPasswordReset({ email: 'unknown@example.edu', turnstileToken: 'turnstile-unknown' }), { accepted: true })
  assert.deepEqual(await client.startPasswordReset({ email: 'student@example.edu', turnstileToken: 'turnstile-reset' }), { accepted: true })
  await client.confirmPasswordReset({ email: 'student@example.edu', code: '123456', newPassword: 'Changed-password-2!' })
  await assert.rejects(
    () => client.getCurrentBackendSession(),
    (error: unknown) => error instanceof client.BackendProblemError && error.code === 'SESSION_EXPIRED',
  )
})

test('password reset page guards re-entry, isolates stale responses, and completes before returning to login', () => {
  const resetSource = readFileSync(new URL('../../pages/PasswordResetPage.vue', import.meta.url), 'utf8')

  assert.match(resetSource, /type ResetStep = 'request' \| 'confirm' \| 'completed'/)
  assert.match(resetSource, /if \(pendingAction\.value \|\| step\.value !== 'request'\) return/)
  assert.match(resetSource, /if \(pendingAction\.value \|\| step\.value !== 'confirm'\) return/)
  assert.match(resetSource, /requestGeneration \+= 1/)
  assert.match(resetSource, /pendingAction\.value === 'reset-start'\) pendingAction\.value = null/)
  assert.match(resetSource, /generation !== requestGeneration \|\| submittedEmail !== canonicalEmail\.value/)
  assert.match(resetSource, /step\.value = 'completed'/)
  assert.match(resetSource, /<RouterLink :to="loginDestination">返回登录<\/RouterLink>/)
  assert.doesNotMatch(resetSource, /getCurrentBackendSession|cacheBackendSession/)
})

test('student registration administration is reasoned, optimistic, immutable-domain UI', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(adminSession()))
    .mockResolvedValueOnce(jsonResponse({ enabled: true, version: 5 }))
    .mockResolvedValueOnce(jsonResponse({
      id: 'domain-created',
      domain: 'new.example.edu',
      institutionName: '新示例大学',
      enabled: true,
      version: 1,
      createdAt: '2026-08-12T00:30:00Z',
      updatedAt: '2026-08-12T00:30:00Z',
    }), 201)
    .mockResolvedValueOnce(jsonResponse({
      id: 'domain-1',
      domain: 'example.edu',
      institutionName: '示例大学',
      enabled: false,
      version: 3,
      createdAt: '2026-08-12T00:00:00Z',
      updatedAt: '2026-08-12T01:00:00Z',
    }))
  vi.stubGlobal('fetch', fetchMock)

  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const admin = await import('../studentRegistrationAdminBackend')
  await admin.updateAdminStudentRegistrationSetting({ enabled: true, expectedVersion: 4, reason: '批准开放' })
  await admin.createAdminStudentInstitutionDomain({
    domain: 'new.example.edu',
    institutionName: '新示例大学',
    enabled: true,
    reason: '新增院校',
  })
  await admin.updateAdminStudentInstitutionDomain({
    id: 'domain-1',
    institutionName: '示例大学',
    enabled: false,
    expectedVersion: 2,
    reason: '暂停入口',
  })

  const settingRequest = fetchMock.mock.calls[1]?.[1] as RequestInit
  const createDomainRequest = fetchMock.mock.calls[2]?.[1] as RequestInit
  const domainRequest = fetchMock.mock.calls[3]?.[1] as RequestInit
  assert.equal(settingRequest.method, 'PATCH')
  assert.equal(new Headers(settingRequest.headers).get('If-Match'), '"4"')
  assert.equal(new Headers(settingRequest.headers).get('X-CSRF-Token'), 'csrf-admin')
  assert.deepEqual(JSON.parse(String(settingRequest.body)), { enabled: true, expectedVersion: 4, reason: '批准开放' })
  assert.equal(createDomainRequest.method, 'POST')
  assert.equal(new Headers(createDomainRequest.headers).get('If-Match'), '"0"')
  assert.deepEqual(JSON.parse(String(createDomainRequest.body)), {
    domain: 'new.example.edu',
    institutionName: '新示例大学',
    enabled: true,
    reason: '新增院校',
  })
  assert.equal(domainRequest.method, 'PATCH')
  assert.equal(new Headers(domainRequest.headers).get('If-Match'), '"2"')
  assert.deepEqual(JSON.parse(String(domainRequest.body)), {
    institutionName: '示例大学',
    enabled: false,
    expectedVersion: 2,
    reason: '暂停入口',
  })
  assert.match(adminPageSource, /settingReason/)
  assert.match(adminPageSource, /domainForm = reactive\(\{ domain: '', institutionName: '', enabled: true, reason: '' \}\)/)
  assert.match(adminBackendSource, /AdminStudentInstitutionDomainUpdateRequest & \{ id: string \}/)
  assert.doesNotMatch(String(domainRequest.body), /"domain"/)
})

test('account page requires recent password authentication before in-place linux.do link', () => {
  assert.match(myCenterSource, /reauthenticatePassword\(linuxDoLinkPassword\.value\)/)
  assert.match(myCenterSource, /linuxDoRecentlyReauthenticated\.value = true/)
  assert.match(myCenterSource, /startLinuxDoLink\('\/my\/account\?linuxdoLinked=1'\)/)
  assert.match(myCenterSource, /currentProfile\?\.linuxDoBinding\.bound[\s\S]*当前会话已更新/)
  assert.match(myCenterSource, /绑定会保留当前账号、邮箱、密码和交易历史/)
  assert.doesNotMatch(myCenterSource, /解绑 linux\.do|合并账号/)
})
