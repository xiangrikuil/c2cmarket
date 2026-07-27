export type PublicRecord = Record<string, unknown>

export type PublicListResponse = {
  items?: PublicRecord[]
  nextCursor?: string | null
}

export async function collectPublicPages(
  loadPage: (cursor: string) => Promise<PublicListResponse>,
  maxPages = 100,
) {
  const items: PublicRecord[] = []
  const seenCursors = new Set<string>()
  let cursor = ''

  for (let page = 0; page < maxPages; page += 1) {
    const response = await loadPage(cursor)
    if (!Array.isArray(response.items)) {
      throw new Error('Public sitemap source returned an invalid list response.')
    }
    items.push(...response.items)

    const nextCursor = typeof response.nextCursor === 'string' ? response.nextCursor.trim() : ''
    if (!nextCursor) return items
    if (seenCursors.has(nextCursor)) {
      throw new Error('Public sitemap source returned a repeated cursor.')
    }
    seenCursors.add(nextCursor)
    cursor = nextCursor
  }

  throw new Error(`Public sitemap source exceeded ${maxPages} pages.`)
}

export function recordValue(record: PublicRecord, ...keys: string[]) {
  for (const key of keys) {
    const candidate = record[key]
    if (typeof candidate === 'string' && candidate.trim()) return candidate.trim()
  }
  return ''
}

export function safeSitemapSegment(input: string) {
  return /^[A-Za-z0-9_-]+$/.test(input) ? encodeURIComponent(input) : ''
}

export function sitemapLastmod(record: PublicRecord) {
  const candidate = recordValue(record, 'updatedAt', 'observedAt', 'publishedAt', 'validFrom', 'createdAt')
  if (!candidate) return undefined
  const timestamp = Date.parse(candidate)
  return Number.isFinite(timestamp) ? new Date(timestamp).toISOString() : undefined
}
