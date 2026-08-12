import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

const loginSource = readFileSync(new URL('../../pages/LoginPage.vue', import.meta.url), 'utf8')
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
  assert.match(loginSource, /registrationStep = ref<'start' \| 'confirm'>\('start'\)/)
  assert.match(loginSource, /startEmailRegistration\(\{[\s\S]*email,[\s\S]*turnstileToken: registrationTurnstileToken\.value/)
  assert.match(loginSource, /finally \{[\s\S]*registrationTurnstileToken\.value = ''[\s\S]*registrationTurnstileWidget\.value\?\.reset\(\)/)
  assert.match(loginSource, /action="student_signup"/)
  assert.match(loginSource, /const usernameValue = registrationUsername\.value\n/)
  assert.doesNotMatch(loginSource, /const usernameValue = registrationUsername\.value\.trim\(\)/)
  assert.match(loginSource, /\^\[a-z0-9_-\]\{3,24\}\$/)
  assert.match(loginSource, /\^\\d\{6\}\$/)
  assert.match(loginSource, /confirmEmailRegistration\(\{[\s\S]*email: registrationEmail\.value,[\s\S]*code:[\s\S]*username: usernameValue,[\s\S]*password:/)
  assert.match(loginSource, /学校邮箱注册暂未开放/)
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
