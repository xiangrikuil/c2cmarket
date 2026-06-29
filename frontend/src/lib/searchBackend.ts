import type { SearchResult } from '@/lib/api'
import { backendRequest } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendSearchResult = {
  id: string
  type: SearchResult['type']
  title: string
  subtitle: string
  badge: string
  to: string
}

function mapSearchResult(item: BackendSearchResult): SearchResult {
  return {
    id: item.id,
    type: item.type,
    title: item.title,
    subtitle: item.subtitle,
    badge: item.badge,
    to: item.to,
  }
}

export async function backendSearchMarket(keyword: string): Promise<SearchResult[]> {
  const q = keyword.trim()
  if (!q) return []
  const response = await backendRequest<ListResponse<BackendSearchResult>>(`/api/v1/search?q=${encodeURIComponent(q)}`)
  return response.items.map(mapSearchResult)
}
