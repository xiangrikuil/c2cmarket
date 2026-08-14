type SentryRequest = {
  url?: string
  query_string?: unknown
  data?: unknown
  cookies?: unknown
  headers?: unknown
  env?: unknown
}

type SanitizableEvent = {
  request?: SentryRequest
  user?: unknown
}

type SanitizableBreadcrumb = {
  data?: Record<string, unknown>
}

const sensitiveBreadcrumbKey = /(authorization|cookie|csrf|token|password|secret|api.?key|email|phone|contact|body|payload)/i

export function sanitizeSentryURL(value: string): string {
  try {
    const parsed = new URL(value, 'https://sentry.invalid')
    if (parsed.origin === 'https://sentry.invalid') {
      return parsed.pathname
    }
    return `${parsed.origin}${parsed.pathname}`
  }
  catch {
    return value.split(/[?#]/, 1)[0] ?? ''
  }
}

export function sanitizeSentryEvent<T extends SanitizableEvent>(event: T): T {
  event.user = undefined
  if (event.request) {
    event.request.url = event.request.url ? sanitizeSentryURL(event.request.url) : undefined
    event.request.query_string = undefined
    event.request.data = undefined
    event.request.cookies = undefined
    event.request.headers = undefined
    event.request.env = undefined
  }
  return event
}

export function sanitizeSentryBreadcrumb<T extends SanitizableBreadcrumb>(breadcrumb: T): T {
  if (!breadcrumb.data) return breadcrumb

  for (const key of Object.keys(breadcrumb.data)) {
    if (sensitiveBreadcrumbKey.test(key)) {
      delete breadcrumb.data[key]
      continue
    }
    if ((key === 'url' || key === 'from' || key === 'to') && typeof breadcrumb.data[key] === 'string') {
      breadcrumb.data[key] = sanitizeSentryURL(breadcrumb.data[key])
    }
  }
  return breadcrumb
}

export function parseSentryEnabled(value: unknown): boolean {
  return value === true || value === 'true' || value === '1'
}

export function parseSentrySampleRate(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) && parsed >= 0 && parsed <= 1 ? parsed : fallback
}
