const privatePaths = [
  '/search',
  '/login',
  '/auth/',
  '/my/',
  '/merchant/',
  '/admin/',
  '/api-intents/',
  '/announcements/',
  '/carpools/new',
  '/demands/new',
  '/api-market/new',
]

export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event)
  const requestUrl = getRequestURL(event)
  const configuredSiteUrl = String(config.public.siteUrl || requestUrl.origin).replace(/\/$/, '')
  const production = new URL(configuredSiteUrl).hostname === 'c2cmarket.shop'
    && requestUrl.hostname === 'c2cmarket.shop'

  setHeader(event, 'Content-Type', 'text/plain; charset=utf-8')
  setHeader(event, 'Cache-Control', 'public, max-age=300')

  if (!production) {
    return `User-agent: *\nDisallow: /\n`
  }

  return [
    'User-agent: *',
    'Allow: /',
    ...privatePaths.map(path => `Disallow: ${path}`),
    `Sitemap: ${configuredSiteUrl}/sitemap.xml`,
    '',
  ].join('\n')
})
