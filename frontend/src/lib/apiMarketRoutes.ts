import { LIMITED_API_QUOTA_OFFERS_ENABLED } from './featureFlags'

export type ApiMarketView = 'limited' | 'packages' | 'free'

const apiMarketQueryKeys: Record<ApiMarketView, ReadonlySet<string>> = {
  limited: new Set(['q', 'model', 'distribution', 'availability', 'multiplierMax', 'saleMode', 'sort']),
  packages: new Set(['q', 'model', 'duration', 'priceMax', 'multiplierMax', 'sort']),
  free: new Set(['q', 'model', 'distribution', 'priceMax', 'minimumMax', 'sort']),
}

export function apiMarketViewFromQuery(value: unknown): ApiMarketView {
  if (value === 'free' || value === 'packages') return value
  if (value === 'limited' && LIMITED_API_QUOTA_OFFERS_ENABLED) return value
  return 'free'
}

export function withApiMarketViewQuery<T extends Record<string, unknown>>(query: T, view: ApiMarketView) {
  return { ...query, view }
}

export function apiMarketPath(view: ApiMarketView) {
  return `/api-market/${view}`
}

export function apiMarketViewFromPath(path: string): ApiMarketView {
  const segment = path.split('/').filter(Boolean)[1]
  return apiMarketViewFromQuery(segment)
}

export function apiMarketQueryForView(query: Record<string, unknown>, view: ApiMarketView): Record<string, string | string[]> {
  const allowed = apiMarketQueryKeys[view]
  const filtered: Record<string, string | string[]> = {}
  for (const [key, value] of Object.entries(query)) {
    if (key === 'view' || !allowed.has(key)) continue
    if (typeof value === 'string' && value !== '') filtered[key] = value
    if (Array.isArray(value)) {
      const values = value.filter((item): item is string => typeof item === 'string' && item !== '')
      if (values.length) filtered[key] = values
    }
  }
  return filtered
}
