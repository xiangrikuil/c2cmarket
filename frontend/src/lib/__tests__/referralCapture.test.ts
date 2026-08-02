import { describe, expect, it } from 'vitest'
import {
  canonicalReferralCode,
  captureReferralCode,
  clearReferralCapture,
  getReferralCapture,
} from '@/lib/referralCapture'

function storage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
}

describe('邀请注册捕获', () => {
  it('按后端字母表规范化 8 位邀请码', () => {
    expect(canonicalReferralCode(' 2abcde89 ')).toBe('2ABCDE89')
    for (const value of ['', '2ABCDE8', '2ABCDE890', 'OABCDE89', '1ABCDE89', null]) {
      expect(canonicalReferralCode(value)).toBe('')
    }
  })

  it('七天内保留第一个有效邀请码', () => {
    const target = storage()
    const now = Date.UTC(2026, 7, 2)
    expect(captureReferralCode('2abcde89', target, now)).toBe('2ABCDE89')
    expect(captureReferralCode('3FGHJKMN', target, now + 1_000)).toBe('2ABCDE89')
    expect(getReferralCapture(target, now + 7 * 24 * 60 * 60 * 1000 - 1)).toBe('2ABCDE89')
  })

  it('过期、损坏或显式消费后清理存储', () => {
    const target = storage()
    const now = Date.UTC(2026, 7, 2)
    captureReferralCode('2ABCDE89', target, now)
    expect(getReferralCapture(target, now + 7 * 24 * 60 * 60 * 1000)).toBe('')

    target.setItem('c2cmarket.referral.v1', '{broken')
    expect(getReferralCapture(target, now)).toBe('')

    captureReferralCode('3FGHJKMN', target, now)
    clearReferralCapture(target)
    expect(getReferralCapture(target, now)).toBe('')
  })

  it('无效邀请码不会覆盖已有有效捕获', () => {
    const target = storage()
    const now = Date.UTC(2026, 7, 2)
    captureReferralCode('2ABCDE89', target, now)
    expect(captureReferralCode('INVALID!', target, now + 1_000)).toBe('2ABCDE89')
  })
})
