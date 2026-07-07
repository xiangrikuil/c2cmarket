import type { UserProfile } from '@/lib/api'

export const ACCOUNT_RECOVERY_PATH = '/my/account'

type AccountRecoveryProfile = Pick<UserProfile, 'emailVerified' | 'passwordConfigured'>

export type AccountRecoveryRequirement = {
  id: 'email' | 'password'
  label: string
  description: string
  completed: boolean
}

const accountRecoveryAllowedPaths = new Set([
  '/',
  '/auth/mock',
  '/login',
  ACCOUNT_RECOVERY_PATH,
])

const accountRecoveryAllowedPrefixes = [
  '/announcements/',
  '/u/',
]

export function accountRecoveryRequirements(profile: AccountRecoveryProfile): AccountRecoveryRequirement[] {
  return [
    {
      id: 'email',
      label: '绑定验证邮箱',
      description: '用于站内账号通知和后续恢复访问，不作为公开注册入口。',
      completed: profile.emailVerified,
    },
    {
      id: 'password',
      label: '设置密码',
      description: 'linux.do 暂不可用时，可用站内用户名和密码登录。',
      completed: profile.passwordConfigured,
    },
  ]
}

export function outstandingAccountRecoveryRequirements(profile: AccountRecoveryProfile) {
  return accountRecoveryRequirements(profile).filter(item => !item.completed)
}

export function isAccountRecoveryComplete(profile: AccountRecoveryProfile) {
  return outstandingAccountRecoveryRequirements(profile).length === 0
}

export function isAccountRecoveryAllowedPath(path: string) {
  if (accountRecoveryAllowedPaths.has(path)) return true
  return accountRecoveryAllowedPrefixes.some(prefix => path.startsWith(prefix))
}

export function sanitizeAccountRecoveryReturnTo(value: unknown) {
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  if (!trimmed || !trimmed.startsWith('/') || trimmed.startsWith('//')) return null
  const [path = ''] = trimmed.split(/[?#]/, 1)
  if (!path || isAccountRecoveryAllowedPath(path)) return null
  return trimmed
}
