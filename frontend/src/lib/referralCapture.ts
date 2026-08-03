type ReferralStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

type StoredReferral = {
  code: string
  expiresAt: number
}

const referralStorageKey = 'c2cmarket.referral.v1'
const referralLifetimeMs = 7 * 24 * 60 * 60 * 1000
const referralAlphabet = '23456789ABCDEFGHJKLMNPQRSTUVWXYZ'

const currentStorage = (): ReferralStorage | null => {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function canonicalReferralCode(value: unknown) {
  if (typeof value !== 'string') return ''
  const code = value.trim().toUpperCase()
  if (code.length !== 8) return ''
  return Array.from(code).every(character => referralAlphabet.includes(character)) ? code : ''
}

export function clearReferralCapture(storage: ReferralStorage | null = currentStorage()) {
  if (!storage) return
  try {
    storage.removeItem(referralStorageKey)
  } catch {
    // 邀请归因不能影响登录流程。
  }
}

export function getReferralCapture(
  storage: ReferralStorage | null = currentStorage(),
  now = Date.now(),
) {
  if (!storage) return ''
  try {
    const raw = storage.getItem(referralStorageKey)
    if (!raw) return ''
    const parsed = JSON.parse(raw) as Partial<StoredReferral>
    const code = canonicalReferralCode(parsed.code)
    if (!code || typeof parsed.expiresAt !== 'number' || parsed.expiresAt <= now) {
      clearReferralCapture(storage)
      return ''
    }
    return code
  } catch {
    clearReferralCapture(storage)
    return ''
  }
}

export function captureReferralCode(
  value: unknown,
  storage: ReferralStorage | null = currentStorage(),
  now = Date.now(),
) {
  const existing = getReferralCapture(storage, now)
  if (existing) return existing

  const code = canonicalReferralCode(value)
  if (!code || !storage) return code
  try {
    storage.setItem(referralStorageKey, JSON.stringify({
      code,
      expiresAt: now + referralLifetimeMs,
    } satisfies StoredReferral))
  } catch {
    // 浏览器存储不可用时，当前登录仍可携带有效邀请码。
  }
  return code
}
