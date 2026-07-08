export const backupPasswordPolicy = {
  minLength: 8,
  maxLength: 32,
} as const

export type PasswordCheckId = 'length' | 'letter' | 'number' | 'special'

export type PasswordCheck = {
  id: PasswordCheckId
  label: string
  completed: boolean
}

export type BackupPasswordStrength = {
  label: string
  tone: 'muted' | 'danger' | 'warning' | 'success'
  passedCount: number
}

const letterPattern = /[A-Za-z]/
const numberPattern = /\d/
const specialPattern = /[^A-Za-z0-9\s]/

export function getPasswordChecks(password: string): PasswordCheck[] {
  const length = password.length
  return [
    {
      id: 'length',
      label: `${backupPasswordPolicy.minLength}–${backupPasswordPolicy.maxLength} 位`,
      completed: length >= backupPasswordPolicy.minLength && length <= backupPasswordPolicy.maxLength,
    },
    {
      id: 'letter',
      label: '包含字母',
      completed: letterPattern.test(password),
    },
    {
      id: 'number',
      label: '包含数字',
      completed: numberPattern.test(password),
    },
    {
      id: 'special',
      label: '包含特殊字符',
      completed: specialPattern.test(password),
    },
  ]
}

export function getBackupPasswordRequirements(password: string): PasswordCheck[] {
  return getPasswordChecks(password)
}

export function getBackupPasswordValidationMessage(password: string): string | null {
  const checks = getPasswordChecks(password)
  if (checks.every(item => item.completed)) return null
  if (!checks.find(item => item.id === 'length')?.completed) {
    return `密码需为 ${backupPasswordPolicy.minLength}–${backupPasswordPolicy.maxLength} 位字符。`
  }
  return '密码需同时包含字母、数字和特殊字符。'
}

export function getBackupPasswordStrength(password: string): BackupPasswordStrength {
  const passedCount = getPasswordChecks(password).filter(item => item.completed).length
  if (!password) {
    return { label: '弱', tone: 'muted', passedCount }
  }

  if (passedCount === 4) {
    return { label: '强', tone: 'success', passedCount }
  }
  if (passedCount >= 2) {
    return { label: '中', tone: 'warning', passedCount }
  }
  return { label: '弱', tone: 'danger', passedCount }
}
