import {
  collectPublicPages,
  recordValue,
  safeSitemapSegment,
  sitemapLastmod,
  type PublicListResponse,
  type PublicRecord,
} from '../../utils/sitemap'

const staticRoutes = [
  '/',
  '/official-prices',
  '/carpools',
  '/demands',
  '/api-market',
]

async function fetchPublicItems(apiBaseUrl: string, path: string) {
  if (!apiBaseUrl) throw createError({ statusCode: 500, statusMessage: 'Public API base URL is not configured.' })
  return collectPublicPages(async (cursor) => {
    const url = new URL(`${apiBaseUrl}${path}`)
    url.searchParams.set('limit', '100')
    if (cursor) url.searchParams.set('cursor', cursor)
    return $fetch<PublicListResponse>(url.toString())
  })
}

type SitemapEntry = { loc: string, lastmod?: string }

function addPublicDetail(entries: Map<string, SitemapEntry>, prefix: string, item: PublicRecord) {
  const id = safeSitemapSegment(recordValue(item, 'id'))
  if (!id) return
  const loc = `${prefix}/${id}`
  const lastmod = sitemapLastmod(item)
  entries.set(loc, { loc, ...(lastmod ? { lastmod } : {}) })
}

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event)
  const apiBaseUrl = String(config.apiBaseUrl ?? '').replace(/\/$/, '')
  const [officialPrices, carpools, demands, apiServices] = await Promise.all([
    fetchPublicItems(apiBaseUrl, '/api/v1/official-prices'),
    fetchPublicItems(apiBaseUrl, '/api/v1/carpools'),
    fetchPublicItems(apiBaseUrl, '/api/v1/demands'),
    fetchPublicItems(apiBaseUrl, '/api/v1/api-services'),
  ])

  const entries = new Map<string, SitemapEntry>(staticRoutes.map(loc => [loc, { loc }]))
  for (const item of officialPrices) {
    addPublicDetail(entries, '/official-prices', item)
  }
  for (const item of carpools) {
    addPublicDetail(entries, '/carpools', item)
  }
  for (const item of demands) {
    addPublicDetail(entries, '/demands', item)
  }
  for (const item of apiServices) {
    addPublicDetail(entries, '/api-market', item)
  }

  setHeader(event, 'Cache-Control', 'public, max-age=60, s-maxage=300')
  return [...entries.values()].sort((left, right) => left.loc.localeCompare(right.loc))
})
