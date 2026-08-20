import { describe, expect, it } from 'vitest'
import type { UserContactMethod } from '@/lib/api'
import {
  isTransactionContactEligible,
  transactionContactById,
  transactionContactLabel,
} from '@/lib/transactionContacts'

function contact(overrides: Partial<UserContactMethod> = {}): UserContactMethod {
  return {
    id: 'contact-wechat',
    userId: 'user-1',
    type: 'wechat',
    label: '微信',
    maskedValue: 'wx***23',
    displayValue: 'wx_user_123',
    isDefault: false,
    enabled: true,
    verified: false,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
    ...overrides,
  }
}

describe('交易联系方式资格', () => {
  it('接受启用的微信和已验证邮箱', () => {
    expect(isTransactionContactEligible(contact())).toBe(true)
    expect(isTransactionContactEligible(contact({ type: 'email', verified: true }))).toBe(true)
  })

  it('拒绝停用项、未验证邮箱和身份联系方式', () => {
    expect(isTransactionContactEligible(contact({ enabled: false }))).toBe(false)
    expect(isTransactionContactEligible(contact({ type: 'email', verified: false }))).toBe(false)
    expect(isTransactionContactEligible(contact({ type: 'linuxdo', verified: true }))).toBe(false)
  })

  it('只按显式 ID 返回符合资格的联系方式', () => {
    const email = contact({ id: 'contact-email', type: 'email', label: '', verified: true })
    const disabledWechat = contact({ id: 'contact-disabled', enabled: false })

    expect(transactionContactById([email, disabledWechat], email.id)).toBe(email)
    expect(transactionContactById([email, disabledWechat], disabledWechat.id)).toBeNull()
    expect(transactionContactById([email], 'missing')).toBeNull()
    expect(transactionContactLabel(email)).toBe('邮箱')
  })
})
