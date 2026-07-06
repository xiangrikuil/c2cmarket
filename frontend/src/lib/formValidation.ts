export type FieldErrors<T extends string> = Partial<Record<T, string>>

const credentialPatterns = [
  /sk-[A-Za-z0-9_-]{12,}/i,
  /api[_-]?key\s*[:=]\s*[\w-]{8,}/i,
  /access[_-]?token\s*[:=]\s*[\w.-]{8,}/i,
  /secret[_-]?key\s*[:=]\s*[\w.-]{8,}/i,
  /sub2api\s*[:=]?\s*[\w-]{8,}/i,
  /token\s*[:=]\s*[\w.-]{8,}/i,
  /session[_-]?token\s*[:=]\s*[\w.-]{8,}/i,
  /refresh[_-]?token\s*[:=]\s*[\w.-]{8,}/i,
  /authorization\s*:\s*(bearer|basic)\s+[\w.+/=-]{8,}/i,
  /cookie\s*[:=]\s*[^;\n]{8,}/i,
  /set-cookie\s*[:=]\s*[^;\n]{8,}/i,
  /-----BEGIN (?:RSA |EC |OPENSSH |DSA |)?PRIVATE KEY-----/i,
  /(mongodb|postgres|postgresql|mysql|redis):\/\/[^\s]+/i,
  /password\s*[:=]\s*.{4,}/i,
  /passwd\s*[:=]\s*.{4,}/i,
  /pwd\s*[:=]\s*.{4,}/i,
  /密码\s*[:：=]\s*.{4,}/i,
  /付款码|二维码内容|完整二维码|支付二维码/i,
  /银行卡号|银行卡|卡号\s*[:：=]?\s*\d{12,}/i,
]

export function isBlank(value: string) {
  return value.trim().length === 0
}

export function isPositiveNumber(value: string) {
  const number = Number(value)
  return Number.isFinite(number) && number > 0
}

export function isHttpUrl(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function isLinuxDoTopicUrl(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' && parsed.hostname === 'linux.do' && parsed.pathname.startsWith('/t/')
  } catch {
    return false
  }
}

export function containsSensitiveContent(values: string[]) {
  const content = values.join('\n')
  return credentialPatterns.some(pattern => pattern.test(content))
}

export function firstError<T extends string>(errors: FieldErrors<T>) {
  return Object.values(errors).find(Boolean)
}
