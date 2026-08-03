import { normalizeAnalyticsPath } from '@/lib/analytics'

export type RegistrationAttribution = {
  source?: string
  medium?: string
  campaign?: string
  referrerHost?: string
  landingPath: string
}

type AttributionLocation = {
  origin: string
  pathname: string
  search: string
}

type AttributionStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

const registrationAttributionStorageKey = 'c2cmarket.registration-attribution.v1'

const sanitizeText = (value: unknown, maxLength: number) => {
  if (typeof value !== 'string') return ''
  const withoutControlCharacters = value.replace(/[\u0000-\u001f\u007f-\u009f]/g, '').trim()
  return Array.from(withoutControlCharacters).slice(0, maxLength).join('').trim()
}

const sanitizeReferrerHost = (value: unknown) => {
  const host = sanitizeText(value, 255).toLowerCase().replace(/\.$/, '')
  return host && /^[a-z0-9.-]+$/.test(host) ? host : ''
}

const currentStorage = (): AttributionStorage | null => {
  if (typeof window === 'undefined') return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

const normalizeStoredAttribution = (value: unknown): RegistrationAttribution | null => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const record = value as Record<string, unknown>
  const source = sanitizeText(record.source, 100)
  const medium = sanitizeText(record.medium, 100)
  const campaign = sanitizeText(record.campaign, 100)
  const referrerHost = sanitizeReferrerHost(record.referrerHost)
  const landingPath = normalizeAnalyticsPath(record.landingPath)
  return {
    ...(source ? { source } : {}),
    ...(medium ? { medium } : {}),
    ...(campaign ? { campaign } : {}),
    ...(referrerHost ? { referrerHost } : {}),
    landingPath,
  }
}

export const getRegistrationAttribution = (
  storage: AttributionStorage | null = currentStorage(),
): RegistrationAttribution | null => {
  if (!storage) return null
  try {
    const raw = storage.getItem(registrationAttributionStorageKey)
    if (!raw) return null
    return normalizeStoredAttribution(JSON.parse(raw))
  } catch {
    return null
  }
}

const externalReferrerHost = (referrer: string, location: AttributionLocation) => {
  const normalizedReferrer = sanitizeText(referrer, 2048)
  if (!normalizedReferrer) return ''
  try {
    const url = new URL(normalizedReferrer)
    if (url.origin === location.origin) return ''
    return sanitizeReferrerHost(url.hostname)
  } catch {
    return ''
  }
}

export const captureRegistrationAttribution = (
  location: AttributionLocation | null = typeof window === 'undefined' ? null : window.location,
  referrer = typeof document === 'undefined' ? '' : document.referrer,
  storage: AttributionStorage | null = currentStorage(),
): RegistrationAttribution | null => {
  if (!location) return null
  const existing = getRegistrationAttribution(storage)
  if (existing) return existing

  const query = new URLSearchParams(location.search)
  const source = sanitizeText(query.get('utm_source'), 100)
  const medium = sanitizeText(query.get('utm_medium'), 100)
  const campaign = sanitizeText(query.get('utm_campaign'), 100)
  const referrerHost = externalReferrerHost(referrer, location)
  const attribution: RegistrationAttribution = {
    ...(source ? { source } : {}),
    ...(medium ? { medium } : {}),
    ...(campaign ? { campaign } : {}),
    ...(referrerHost ? { referrerHost } : {}),
    landingPath: normalizeAnalyticsPath(location.pathname),
  }

  if (storage) {
    try {
      storage.setItem(registrationAttributionStorageKey, JSON.stringify(attribution))
    } catch {
      // 浏览器存储不可用时，注册流程仍需继续。
    }
  }
  return attribution
}

export const clearRegistrationAttribution = (
  storage: AttributionStorage | null = currentStorage(),
) => {
  if (!storage) return
  try {
    storage.removeItem(registrationAttributionStorageKey)
  } catch {
    // 归因清理仅用于分析，不应影响认证流程。
  }
}
