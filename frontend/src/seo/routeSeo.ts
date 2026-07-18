import type { RouteLocationNormalizedLoaded } from 'vue-router'

export type RouteSeo = {
  title: string
  description: string
  indexable: boolean
}

const defaultDescription = '浏览订阅拼车、API 服务、求车需求与官网价格；C2CMarket 提供信息撮合和风险治理，不代收、不托管资金。'

const staticSeo: Record<string, Omit<RouteSeo, 'indexable'>> = {
  '/': {
    title: 'C2CMarket｜AI 服务撮合与风险治理',
    description: defaultDescription,
  },
  '/official-prices': {
    title: 'AI 产品官网价格｜C2CMarket',
    description: '比较 ChatGPT、Claude、Cursor、Gemini 等产品在不同地区与渠道的公开官网价格。',
  },
  '/carpools': {
    title: '订阅拼车市场｜C2CMarket',
    description: '浏览公开订阅拼车车源，比较月费、地区、访问安排、车主信誉与剩余席位。',
  },
  '/demands': {
    title: '求车需求大厅｜C2CMarket',
    description: '浏览公开求车需求，按套餐、预算、地区与车主偏好匹配已有车源。',
  },
  '/api-market': {
    title: 'API 服务市场｜C2CMarket',
    description: '比较公开 API 服务的额度售价、模型支持、最低订单、交付方式与商户承诺。',
  },
}

const privatePrefixes = [
  '/search',
  '/login',
  '/auth',
  '/my',
  '/merchant',
  '/admin',
  '/api-intents',
  '/announcements',
  '/u',
]

const privateExactPaths = new Set([
  '/carpools/new',
  '/demands/new',
  '/api-market/new',
])

export function resolveRouteSeo(route: RouteLocationNormalizedLoaded): RouteSeo {
  const path = route.path
  const isNotFound = route.name === 'not-found'
  const indexable = !isNotFound
    && !privateExactPaths.has(path)
    && !privatePrefixes.some(prefix => path === prefix || path.startsWith(`${prefix}/`))

  if (isNotFound) {
    return {
      title: '页面不存在｜C2CMarket',
      description: '你访问的页面不存在或已移动。',
      indexable: false,
    }
  }

  const exact = staticSeo[path]
  if (exact) return { ...exact, indexable }

  if (path.startsWith('/official-prices/')) {
    return { title: '官网价格详情｜C2CMarket', description: '查看公开官网价格、地区、渠道、更新时间与原始来源。', indexable }
  }
  if (path.startsWith('/carpools/')) {
    return { title: '订阅拼车详情｜C2CMarket', description: '查看车源月费、席位、访问安排、车主信誉与风险提示。', indexable }
  }
  if (path.startsWith('/demands/')) {
    return { title: '求车需求详情｜C2CMarket', description: '查看公开求车需求的预算、地区、车主偏好与回应方式。', indexable }
  }
  if (path.startsWith('/api-market/')) {
    return { title: 'API 服务详情｜C2CMarket', description: '查看 API 服务模型、售价、最低订单、交付方式与商户说明。', indexable }
  }
  if (path.startsWith('/u/')) {
    return { title: '用户公开主页｜C2CMarket', description: '查看用户公开资料、脱敏信誉统计与公开业务记录。', indexable }
  }

  return {
    title: String(route.meta.title ?? 'C2CMarket'),
    description: String(route.meta.description ?? defaultDescription),
    indexable,
  }
}

export function breadcrumbItems(route: RouteLocationNormalizedLoaded, siteUrl: string) {
  const parts = route.path.split('/').filter(Boolean)
  const labels: Record<string, string> = {
    'official-prices': '官网价格',
    carpools: '订阅拼车',
    demands: '求车需求',
    'api-market': 'API 市场',
    u: '用户主页',
  }

  return [
    { '@type': 'ListItem', position: 1, name: '首页', item: new URL('/', siteUrl).toString() },
    ...parts.map((part, index) => ({
      '@type': 'ListItem',
      position: index + 2,
      name: labels[part] ?? (index === parts.length - 1 ? '详情' : part),
      item: new URL(`/${parts.slice(0, index + 1).join('/')}`, siteUrl).toString(),
    })),
  ]
}
