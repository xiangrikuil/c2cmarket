import { projectCapabilities, type Capability } from '@/lib/capabilities'
import { getBackupPasswordValidationMessage } from '@/lib/passwordPolicy'

export class MockAuthProblem extends Error {
  status: number
  code: string
  detail: string

  constructor(status: number, code: string, detail: string) {
    super(detail)
    this.name = 'MockAuthProblem'
    this.status = status
    this.code = code
    this.detail = detail
  }
}

export type MockPersona = 'anonymous' | 'student' | 'linuxdo' | 'admin'

export type MockIdentity = {
  persona: Exclude<MockPersona, 'anonymous'>
  id: string
  analyticsUserId: string
  username: string
  displayName: string
  email: string | null
  capabilities: Capability[]
  permissions: string[]
  studentClaim: {
    institutionDomain: string
    institutionName: string
    claimedAt: string
  } | null
  linuxDoBinding: {
    bound: boolean
    linuxDoUserId?: string
    linuxDoUsername?: string
    trustLevel?: number
    avatarUrl?: string
  }
}

type StoredMockAuth = {
  persona: MockPersona
  studentUsername?: string
  studentEmail?: string
}

type MockRegistrationChallenge = {
  email: string
  code: string
  expiresAt: string
}

const storageKey = 'c2cmarket.mock-auth.v1'
let memoryState: StoredMockAuth = { persona: 'linuxdo' }
let registrationChallenge: MockRegistrationChallenge | null = null
let recentlyReauthenticated = false
let mockRegistrationSetting = { enabled: false, version: 1, updatedAt: undefined as string | undefined }
let mockInstitutionDomains = [{
  id: 'mock-institution-example',
  domain: 'example.edu',
  institutionName: '示例大学',
  enabled: true,
  version: 1,
  createdAt: '2026-08-12T00:00:00Z',
  updatedAt: '2026-08-12T00:00:00Z',
}]

function readState(): StoredMockAuth {
  if (typeof window === 'undefined') return memoryState
  const raw = window.sessionStorage.getItem(storageKey)
  if (!raw) return memoryState
  try {
    const parsed = JSON.parse(raw) as Partial<StoredMockAuth>
    if (parsed.persona === 'anonymous' || parsed.persona === 'student' || parsed.persona === 'linuxdo' || parsed.persona === 'admin') {
      memoryState = {
        persona: parsed.persona,
        studentUsername: typeof parsed.studentUsername === 'string' ? parsed.studentUsername : undefined,
        studentEmail: typeof parsed.studentEmail === 'string' ? parsed.studentEmail : undefined,
      }
    }
  } catch {
    window.sessionStorage.removeItem(storageKey)
  }
  return memoryState
}

function writeState(next: StoredMockAuth) {
  memoryState = next
  recentlyReauthenticated = false
  if (typeof window !== 'undefined') window.sessionStorage.setItem(storageKey, JSON.stringify(next))
}

export function getMockPersona(): MockPersona {
  return readState().persona
}

export function setMockPersona(persona: MockPersona) {
  const current = readState()
  writeState({
    persona,
    studentUsername: current.studentUsername,
    studentEmail: current.studentEmail,
  })
}

export function getMockIdentity(): MockIdentity | null {
  const state = readState()
  if (state.persona === 'anonymous') return null

  const linuxDoBound = state.persona === 'linuxdo' || state.persona === 'admin'
  const admin = state.persona === 'admin'
  const student = state.persona === 'student'
  const linkedStudent = state.persona === 'linuxdo' && Boolean(state.studentUsername)
  const username = student || linkedStudent ? (state.studentUsername ?? 'student-buyer') : 'orbit'
  const displayName = student ? '学生买家' : linkedStudent ? '已绑定学生用户' : admin ? '开发管理员' : 'linux.do 卖家'

  return {
    persona: state.persona,
    id: student || linkedStudent ? 'mock-student-user' : admin ? 'mock-admin-user' : 'mock-linuxdo-user',
    analyticsUserId: student || linkedStudent ? 'mock-student' : admin ? 'mock-admin' : 'mock-linuxdo',
    username,
    displayName,
    email: student || linkedStudent ? (state.studentEmail ?? 'student@example.edu') : null,
    capabilities: projectCapabilities({ linuxDoBound, studentClaim: student || linkedStudent, admin }),
    permissions: admin ? ['admin'] : [],
    studentClaim: student || linkedStudent ? {
      institutionDomain: (state.studentEmail ?? 'student@example.edu').split('@')[1] ?? 'example.edu',
      institutionName: '示例大学',
      claimedAt: '2026-08-12T00:00:00Z',
    } : null,
    linuxDoBinding: linuxDoBound ? {
      bound: true,
      linuxDoUserId: admin ? '9002' : '9001',
      linuxDoUsername: username,
      trustLevel: 3,
    } : { bound: false },
  }
}

export function requireMockIdentity(): MockIdentity {
  const identity = getMockIdentity()
  if (identity) return identity
  throw new MockAuthProblem(401, 'SESSION_EXPIRED', '请先登录。')
}

export function mockEmailRegistrationConfig() {
  return {
    enabled: mockRegistrationSetting.enabled,
    institutions: mockInstitutionDomains
      .filter(item => item.enabled)
      .map(({ domain, institutionName }) => ({ domain, institutionName })),
  }
}

export function getMockRegistrationAdminState() {
  return {
    setting: structuredClone(mockRegistrationSetting),
    domains: structuredClone(mockInstitutionDomains),
  }
}

export function updateMockRegistrationSetting(input: { enabled: boolean, expectedVersion: number }) {
  if (input.expectedVersion !== mockRegistrationSetting.version) throw new Error('配置已被其他管理员更新，请刷新后重试。')
  mockRegistrationSetting = {
    enabled: input.enabled,
    version: mockRegistrationSetting.version + 1,
    updatedAt: new Date().toISOString(),
  }
  return structuredClone(mockRegistrationSetting)
}

export function createMockInstitutionDomain(input: { domain: string, institutionName: string, enabled: boolean }) {
  const domain = input.domain.trim().toLowerCase()
  if (mockInstitutionDomains.some(item => item.domain === domain)) throw new Error('该精确域名已存在。')
  const now = new Date().toISOString()
  const created = {
    id: `mock-domain-${mockInstitutionDomains.length + 1}`,
    domain,
    institutionName: input.institutionName.trim(),
    enabled: input.enabled,
    version: 1,
    createdAt: now,
    updatedAt: now,
  }
  mockInstitutionDomains = [created, ...mockInstitutionDomains]
  return structuredClone(created)
}

export function updateMockInstitutionDomain(input: { id: string, institutionName: string, enabled: boolean, expectedVersion: number }) {
  const current = mockInstitutionDomains.find(item => item.id === input.id)
  if (!current) throw new Error('院校域名不存在。')
  if (current.version !== input.expectedVersion) throw new Error('院校域名已被其他管理员更新，请刷新后重试。')
  const updated = {
    ...current,
    institutionName: input.institutionName.trim(),
    enabled: input.enabled,
    version: current.version + 1,
    updatedAt: new Date().toISOString(),
  }
  mockInstitutionDomains = mockInstitutionDomains.map(item => item.id === input.id ? updated : item)
  return structuredClone(updated)
}

export function startMockEmailRegistration(email: string, turnstileToken: string) {
  const normalizedEmail = email.trim().toLowerCase()
  if (!turnstileToken.trim()) throw new Error('请先完成人机验证。')
  if (!mockRegistrationSetting.enabled) throw new MockAuthProblem(403, 'EMAIL_REGISTRATION_DISABLED', '学校邮箱注册当前未开放。')
  const domain = normalizedEmail.split('@')[1] ?? ''
  if (!mockInstitutionDomains.some(item => item.enabled && item.domain === domain)) {
    throw new MockAuthProblem(422, 'STUDENT_EMAIL_NOT_ELIGIBLE', '该邮箱不属于当前开放注册的学校域名。')
  }
  registrationChallenge = {
    email: normalizedEmail,
    code: '123456',
    expiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
  }
  return { ...registrationChallenge, devCode: registrationChallenge.code }
}

export function confirmMockEmailRegistration(input: {
  email: string
  code: string
  username: string
  password: string
}) {
  const email = input.email.trim().toLowerCase()
  if (!mockRegistrationSetting.enabled) {
    throw new MockAuthProblem(403, 'EMAIL_REGISTRATION_DISABLED', '学校邮箱注册当前未开放。')
  }
  const domain = email.split('@')[1] ?? ''
  if (!mockInstitutionDomains.some(item => item.enabled && item.domain === domain)) {
    throw new MockAuthProblem(422, 'STUDENT_EMAIL_NOT_ELIGIBLE', '该邮箱不属于当前开放注册的学校域名。')
  }
  const validChallenge = registrationChallenge
    && registrationChallenge.email === email
    && registrationChallenge.code === input.code
    && Date.parse(registrationChallenge.expiresAt) > Date.now()
  if (!validChallenge) {
    throw new MockAuthProblem(422, 'VERIFICATION_CODE_INVALID', '验证码无效或已过期。')
  }
  if (!/^[a-z0-9_-]{3,24}$/.test(input.username)) {
    throw new MockAuthProblem(422, 'USERNAME_INVALID', '用户名格式不正确。')
  }
  const passwordError = getBackupPasswordValidationMessage(input.password)
  if (passwordError) throw new MockAuthProblem(422, 'PASSWORD_INVALID', passwordError)

  registrationChallenge = null
  writeState({ persona: 'student', studentUsername: input.username, studentEmail: email })
  return requireMockIdentity()
}

export function loginMockWithPassword(identifier: string, password: string) {
  if (!password) throw new Error('请输入密码。')
  const state = readState()
  const normalized = identifier.trim().toLowerCase()
  const persona: MockPersona = normalized === 'dev-admin' || normalized === 'admin'
    ? 'admin'
    : normalized === 'orbit' || normalized === 'linuxdo'
      ? 'linuxdo'
      : normalized === (state.studentUsername ?? 'student-buyer') || normalized === (state.studentEmail ?? 'student@example.edu')
        ? 'student'
        : 'anonymous'
  if (persona === 'anonymous') {
    throw new MockAuthProblem(401, 'INVALID_CREDENTIALS', '用户名或密码不正确。')
  }
  setMockPersona(persona)
  return requireMockIdentity()
}

export function reauthenticateMockPassword(password: string) {
  requireMockIdentity()
  if (!password) throw new Error('请输入当前密码。')
  recentlyReauthenticated = true
}

export function linkMockLinuxDo() {
  const identity = requireMockIdentity()
  if (identity.linuxDoBinding.bound) return
  if (!recentlyReauthenticated) {
    throw new MockAuthProblem(403, 'RECENT_REAUTHENTICATION_REQUIRED', '请先使用当前密码完成近期身份验证。')
  }
  setMockPersona('linuxdo')
}
