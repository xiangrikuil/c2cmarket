import type { UserContactMethod } from '@/lib/api'

export function isTransactionContactEligible(contact: UserContactMethod) {
  if (!contact.enabled) return false
  if (contact.type === 'wechat') return true
  return contact.type === 'email' && contact.verified
}

export function transactionContactById(methods: UserContactMethod[], contactId: string) {
  return methods.find(contact => contact.id === contactId && isTransactionContactEligible(contact)) ?? null
}

export function transactionContactLabel(contact: UserContactMethod) {
  if (contact.type === 'email') return contact.label || '邮箱'
  return contact.label || '微信'
}
