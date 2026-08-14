import { nextTick } from 'vue'
import { BackendProblemError } from '@/lib/backendClient'
import { getBackupPasswordValidationMessage } from '@/lib/passwordPolicy'

export type AuthFieldErrors = Record<string, string>

export type BackendFieldErrorMapping = {
  fields: AuthFieldErrors
  hasUnmapped: boolean
}

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const usernamePattern = /^[a-z0-9_-]{3,24}$/
const verificationCodePattern = /^\d{6}$/

export const validateEmail = (value: string) => {
  const email = value.trim()
  if (!email) return '请输入学校邮箱。'
  if (!emailPattern.test(email)) return '请输入有效的邮箱地址。'
  return ''
}

export const validateLoginIdentifier = (value: string) => (
  value.trim() ? '' : '请输入用户名或学校邮箱。'
)

export const validateRequiredPassword = (value: string) => (
  value ? '' : '请输入密码。'
)

export const validateUsername = (value: string) => (
  usernamePattern.test(value)
    ? ''
    : '用户名需为 3-24 位小写字母、数字、下划线或短横线。'
)

export const validateVerificationCode = (value: string) => (
  verificationCodePattern.test(value.trim()) ? '' : '请输入 6 位验证码。'
)

export const validateNewPassword = (value: string) => (
  getBackupPasswordValidationMessage(value) ?? ''
)

export const validatePasswordConfirmation = (password: string, confirmation: string) => {
  if (!confirmation) return '请再次输入密码。'
  return password === confirmation ? '' : '两次输入的密码不一致。'
}

export const backendFieldErrors = (
  error: unknown,
  aliases: Record<string, string> = {},
  knownFields: readonly string[] = [],
): BackendFieldErrorMapping => {
  if (!(error instanceof BackendProblemError)) return { fields: {}, hasUnmapped: false }
  const mapped: AuthFieldErrors = {}
  const known = new Set(knownFields)
  let hasUnmapped = false
  for (const item of error.fieldErrors) {
    if (!item.field || !item.message) continue
    const field = aliases[item.field] ?? item.field
    if (known.size > 0 && !known.has(field)) {
      hasUnmapped = true
      continue
    }
    mapped[field] = item.message
  }
  return { fields: mapped, hasUnmapped }
}

export const focusFirstInvalidField = async (
  errors: AuthFieldErrors,
  orderedFieldIds: Array<[string, string]>,
) => {
  const target = orderedFieldIds.find(([field]) => Boolean(errors[field]))
  if (!target || typeof document === 'undefined') return
  await nextTick()
  document.getElementById(target[1])?.focus()
}
