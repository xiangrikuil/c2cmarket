<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Box, Car, Check, Info, Megaphone } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import HomeTrendChart from '@/components/market/HomeTrendChart.vue'
import type { Carpool, ProductTrend, TransactionTrendPoint } from '@/lib/api'
import { useActiveHomeAnnouncement } from '@/queries/useAnnouncementQueries'
import { useHomeMarket } from '@/queries/useMarketQueries'

const { data } = useHomeMarket()
const { data: homeAnnouncement } = useActiveHomeAnnouncement()

const trendTabs = [
  { key: 'all', label: '全部', slug: null },
  { key: 'chatgpt-plus', label: 'ChatGPT Plus', slug: 'chatgpt-plus' },
  { key: 'chatgpt-pro', label: 'ChatGPT Pro', slug: 'chatgpt-pro-20x-web' },
  { key: 'chatgpt-business', label: 'ChatGPT Business', slug: 'chatgpt-business' },
  { key: 'claude-pro', label: 'Claude Pro', slug: 'claude-max-5x' },
  { key: 'gemini-advanced', label: 'Gemini Advanced', slug: 'more-products' },
] as const

const homeTooltipCopy = {
  trendReference: {
    aria: '近 30 日完成参考说明',
    title: '近 30 日完成参考',
    description: '基于近 30 日社区确认的价格样本生成，仅用于行情参考，不代表平台定价或交易承诺。',
  },
  currentReference: {
    aria: '当前可申请参考说明',
    title: '当前可申请参考',
    description: '优先取当前可申请车源中的最低月费；暂无可申请车源时，使用近期完成参考或已验证公开价作为参考。',
  },
  completedReference: {
    aria: '近 30 日完成参考价说明',
    title: '完成参考（近 30 日）',
    description: '取近 30 日社区确认样本的参考价中位数；样本不足时显示当前参考价。',
  },
} as const

const selectedTrendKey = ref<(typeof trendTabs)[number]['key']>('all')
const homeTrends = computed(() => data.value?.productTrends ?? [])
const selectedTrendTab = computed(() => trendTabs.find(tab => tab.key === selectedTrendKey.value) ?? trendTabs[0])
const selectedTrend = computed(() => {
  if (!selectedTrendTab.value.slug) return null
  return homeTrends.value.find(item => item.slug === selectedTrendTab.value.slug) ?? null
})

function trendPoints(trend: ProductTrend) {
  return trend.points['30d']
}

function averageTrendPoints(trends: ProductTrend[]): TransactionTrendPoint[] {
  const series = trends.map(trendPoints).filter(points => points.length > 0)
  const pointCount = Math.max(...series.map(points => points.length), 0)
  if (!pointCount) return []

  return Array.from({ length: pointCount }, (_, index) => {
    const points = series
      .map(item => item[index])
      .filter((point): point is TransactionTrendPoint => Boolean(point))
    const divisor = Math.max(points.length, 1)

    return {
      date: points[points.length - 1]?.date ?? '',
      medianPrice: Math.round(points.reduce((sum, point) => sum + point.medianPrice, 0) / divisor),
      p25Price: Math.round(points.reduce((sum, point) => sum + point.p25Price, 0) / divisor),
      p75Price: Math.round(points.reduce((sum, point) => sum + point.p75Price, 0) / divisor),
      transactionCount: points.reduce((sum, point) => sum + point.transactionCount, 0),
    }
  })
}

const trendChartData = computed(() => {
  if (selectedTrend.value) return trendPoints(selectedTrend.value)
  return averageTrendPoints(homeTrends.value)
})

const validTransactions = computed(() => (data.value?.transactionRecords ?? []).filter(item => {
  return item.status === 'completed'
    && !item.hasUnresolvedDispute
    && Number.isFinite(item.finalSettlementPrice)
}))

const categoryDefinitions = [
  { key: 'chatgpt-plus', product: 'ChatGPT Plus', iconSrc: '/chatgpt-mark.svg', iconText: 'GPT', iconTone: 'home-product-gpt', official: '$20 / 月', trendSlug: 'chatgpt-plus', productKeywords: ['chatgpt plus', 'plus'], to: '/carpools?category=gpt' },
  { key: 'chatgpt-pro', product: 'ChatGPT Pro', iconSrc: '/chatgpt-mark.svg', iconText: 'GPT', iconTone: 'home-product-pro', official: '$200 / 月', trendSlug: 'chatgpt-pro-20x-web', productKeywords: ['chatgpt pro', 'pro 20x', 'pro 5x'], to: '/carpools?category=gpt' },
  { key: 'chatgpt-business', product: 'ChatGPT Business', iconSrc: '/chatgpt-mark.svg', iconText: 'GPT', iconTone: 'home-product-gpt', official: '$25 / 用户 / 月', trendSlug: 'chatgpt-business', productKeywords: ['chatgpt business', 'business'], to: '/carpools?category=gpt' },
  { key: 'claude-pro', product: 'Claude Pro', iconSrc: '/claude-mark.svg', iconText: 'AI', iconTone: 'home-product-claude', official: '$20 / 月', trendSlug: 'claude-max-5x', productKeywords: ['claude'], to: '/carpools?category=claude' },
  { key: 'gemini-advanced', product: 'Gemini Advanced', iconSrc: '/gemini-mark.svg', iconText: 'Gemini', iconTone: 'home-product-gemini', official: '$19.99 / 月', trendSlug: 'more-products', productKeywords: ['gemini'], to: '/carpools?category=gemini' },
]

function matchesProduct(value: string, keywords: string[]) {
  const normalized = value.toLowerCase()
  return keywords.some(keyword => normalized.includes(keyword))
}

function currentPriceFor(carpools: Carpool[], keywords: string[]) {
  const prices = carpools
    .filter(item => item.status === '可上车' && matchesProduct(item.product, keywords))
    .map(item => item.monthly)
    .filter(Number.isFinite)

  if (!prices.length) return null
  return Math.min(...prices)
}

const categoryRows = computed(() => categoryDefinitions.map((definition) => {
  const trend = homeTrends.value.find(item => item.slug === definition.trendSlug)
  const trendPoints30d = trend?.points['30d'] ?? []
  const latestTrendPoint = [...trendPoints30d].reverse().find(point => point.transactionCount > 0)
  const current = currentPriceFor(data.value?.carpools ?? [], definition.productKeywords)
    ?? latestTrendPoint?.medianPrice
    ?? trend?.officialVerifiedLow
    ?? 0
  const completed = latestTrendPoint?.medianPrice ?? current
  const carpoolCount = (data.value?.carpools ?? []).filter(item => matchesProduct(item.product, definition.productKeywords)).length
  const demandCount = (data.value?.demands ?? []).filter(item => matchesProduct(item.title, definition.productKeywords)).length

  return {
    ...definition,
    current,
    completed,
    available: carpoolCount > 0,
    supply: `车源 ${carpoolCount} · 需求 ${demandCount}`,
  }
}))

const recentDeals = computed(() => validTransactions.value.slice(0, 4).map((item, index) => ({
  id: item.id,
  title: index === 0 ? 'ChatGPT Plus 拼车（年付）' : index === 1 ? 'ChatGPT Business 车源' : index === 2 ? 'Claude Pro 拼车（年付）' : 'Gemini Advanced 拼车（年付）',
  time: index === 0 ? '06-05 14:32' : index === 1 ? '06-05 13:08' : index === 2 ? '06-05 12:21' : '06-05 11:47',
  price: index === 0 ? 158 : index === 1 ? 149 : index === 2 ? 142 : 116,
  people: index === 0 ? '4 人拼车' : index === 1 ? '1 人' : index === 2 ? '3 人拼车' : '2 人拼车',
})))

const apiRows = computed(() => (data.value?.apiServices ?? []).slice(0, 3).map((service, index) => ({
  id: service.id,
  title: index === 0 ? 'OpenAI GPT-4o' : index === 1 ? 'Anthropic Claude 3.5 Sonnet' : 'Google Gemini 1.5 Pro',
  quota: index === 0 ? '2.5M Tokens' : index === 1 ? '500K Tokens' : '1M Tokens',
  price: index === 0 ? '¥ 0.95' : index === 1 ? '¥ 1.30' : '¥ 0.80',
  color: 'bg-white ring-1 ring-slate-200',
  label: index === 0 ? '◎' : index === 1 ? 'AI' : '✦',
  iconSrc: index === 0 ? '/openai-mark.svg' : index === 1 ? '/claude-mark.svg' : '/gemini-mark.svg',
})))

const stats = computed(() => {
  const verifiedPrices = (data.value?.officialPrices ?? [])
    .filter(item => item.status === '已验证' && item.cny !== null)
    .map(item => item.cny as number)
  const availableCarpools = (data.value?.carpools ?? []).filter(item => item.status === '可上车').length
  const openDemands = (data.value?.demands ?? []).filter(item => !['已关闭', '需处理'].includes(item.status)).length
  const onlineApiServices = (data.value?.apiServices ?? []).filter(item => item.publiclyOrderable).length

  return [
    { label: '官网低价', value: verifiedPrices.length ? `¥${Math.min(...verifiedPrices)}` : '暂无', hint: '实时汇总', delta: '已验证', tone: 'text-emerald-600' },
    { label: '可申请车源', value: String(availableCarpools), hint: '实时汇总', delta: '可上车', tone: 'text-emerald-600' },
    { label: '求车需求', value: String(openDemands), hint: '实时汇总', delta: '开放中', tone: 'text-blue-600' },
    { label: 'API 额度', value: String(onlineApiServices), hint: '实时汇总', delta: '可提交意向', tone: 'text-emerald-600' },
  ]
})

const quickActions = [
  { title: '发布车源', desc: '有可用账号？发布车源接收申请', to: '/carpools/new', icon: Car, tone: 'from-teal-500 to-cyan-500' },
  { title: '发布 API 服务', desc: '提供 API 额度？发布服务', to: '/api-market/new', icon: Box, tone: 'from-violet-500 to-purple-500' },
]

const announcementCenterTo = '/my/notifications?tab=announcements'
const homeAnnouncementTo = computed(() => homeAnnouncement.value ? `/announcements/${homeAnnouncement.value.slug}` : announcementCenterTo)
const homeAnnouncementTitle = computed(() => homeAnnouncement.value?.title ?? '平台公告')
const homeAnnouncementSummary = computed(() => homeAnnouncement.value?.summary ?? '查看平台公告与业务更新。')
const homeAnnouncementCtaLabel = computed(() => homeAnnouncement.value ? '查看详情' : '查看公告')
</script>

<template>
  <div class="home-exact grid w-full">
    <section class="home-top-grid">
      <div class="home-exact-hero">
        <div>
          <h1>社区行情总览</h1>
          <p>汇聚官网价格记录、拼车车源、求车需求与 API 额度信息，为你提供更透明的 AI 服务交易参考。</p>
        </div>
        <div class="home-hero-art" aria-hidden="true">
          <div class="home-hero-cube"></div>
          <div class="home-hero-card">
            <div class="home-hero-pie"></div>
          </div>
          <div class="home-hero-bars"><span></span><span></span><span></span></div>
        </div>
      </div>

      <div class="home-exact-stats">
        <div v-for="item in stats" :key="item.label" class="home-exact-stat">
          <div class="text-[13px] font-medium text-slate-500">{{ item.label }}</div>
          <div class="mt-3 text-[26px] font-semibold leading-none text-teal-700">{{ item.value }}</div>
          <div class="mt-3 flex items-center gap-2 text-xs text-slate-500">
            <span>{{ item.hint }}</span>
            <span :class="item.tone">{{ item.delta }}</span>
          </div>
        </div>
      </div>
    </section>

    <section class="home-content-grid">
      <div class="home-card home-chart-card">
        <div class="flex items-center justify-between gap-4">
          <div class="flex items-center gap-1.5">
            <h2>近 30 日完成参考</h2>
            <Popover>
              <PopoverTrigger as-child>
                <button type="button" class="home-info-trigger" :aria-label="homeTooltipCopy.trendReference.aria">
                  <Info class="h-3.5 w-3.5" />
                </button>
              </PopoverTrigger>
              <PopoverContent class="w-64 text-xs leading-5" side="top" align="center">
                <div class="font-semibold text-slate-900">{{ homeTooltipCopy.trendReference.title }}</div>
                <p class="mt-1 text-slate-500">{{ homeTooltipCopy.trendReference.description }}</p>
              </PopoverContent>
            </Popover>
          </div>
          <Button variant="outline" size="sm" class="h-8 text-xs">近 30 天</Button>
        </div>
        <div class="mt-4 flex flex-wrap gap-1.5">
          <button
            v-for="tab in trendTabs"
            :key="tab.key"
            type="button"
            class="home-chip"
            :class="selectedTrendKey === tab.key ? 'home-chip-active' : ''"
            @click="selectedTrendKey = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>
        <div class="home-line-chart">
          <HomeTrendChart :data="trendChartData" />
        </div>
      </div>

      <div class="home-action-stack">
        <RouterLink v-for="action in quickActions" :key="action.to" :to="action.to" class="home-action-card">
          <span class="home-action-icon bg-gradient-to-br" :class="action.tone">
            <component :is="action.icon" class="h-5 w-5" />
          </span>
          <span>
            <span class="block text-[14px] font-semibold text-slate-900">{{ action.title }}</span>
            <span class="mt-1 block text-xs leading-5 text-slate-500">{{ action.desc }}</span>
          </span>
        </RouterLink>
      </div>

      <aside class="home-right-stack">
        <div class="home-side-top-stack">
          <RouterLink :to="homeAnnouncementTo" class="home-announcement group" aria-label="查看平台公告">
            <Megaphone class="h-11 w-11 text-teal-700" />
            <div class="min-w-0 flex-1">
              <div class="truncate font-semibold text-teal-900">{{ homeAnnouncementTitle }}</div>
              <p class="mt-1 line-clamp-1 text-xs leading-5 text-teal-800/80">{{ homeAnnouncementSummary }}</p>
              <span class="mt-2 inline-flex items-center gap-1 text-xs font-medium text-teal-700 group-hover:text-teal-900">
                {{ homeAnnouncementCtaLabel }} <ArrowRight class="h-3.5 w-3.5" />
              </span>
            </div>
            <ArrowRight class="h-3.5 w-3.5 text-slate-400 transition group-hover:translate-x-0.5 group-hover:text-teal-700" />
          </RouterLink>

          <div class="home-card home-list-card">
            <div class="mb-2 flex items-center justify-between">
              <h2>近期已验证成交</h2>
              <RouterLink to="/carpools" class="text-xs text-slate-500">查看全部</RouterLink>
            </div>
            <div class="divide-y divide-slate-100">
              <div v-for="deal in recentDeals" :key="deal.id" class="home-deal-row">
                <Check class="mt-0.5 h-4 w-4 rounded-full bg-teal-600 p-0.5 text-white" />
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[13px] font-medium text-slate-700">{{ deal.title }}</div>
                  <div class="mt-1 text-[11px] text-slate-400">成交时间&nbsp;&nbsp; {{ deal.time }}</div>
                </div>
                <div class="text-right">
                  <div class="text-[13px] font-semibold text-teal-700">¥{{ deal.price }}</div>
                  <div class="mt-1 text-[11px] text-slate-400">{{ deal.people }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="home-card home-api-card">
          <div class="mb-2 flex items-center justify-between">
            <h2>API 集市（精选）</h2>
            <RouterLink to="/api-market" class="text-xs text-slate-500">查看全部</RouterLink>
          </div>
          <div class="divide-y divide-slate-100">
            <RouterLink v-for="row in apiRows" :key="row.id" :to="`/api-market/${row.id}`" class="home-api-row">
              <span class="home-api-icon" :class="row.color">
                <img v-if="row.iconSrc" :src="row.iconSrc" :alt="row.title" class="home-brand-img" />
                <span v-else>{{ row.label }}</span>
              </span>
              <span class="min-w-0 flex-1">
                <span class="block truncate text-[13px] font-medium text-slate-700">{{ row.title }}</span>
                <span class="mt-1 block text-[11px] text-slate-400">额度&nbsp;&nbsp;{{ row.quota }}</span>
              </span>
              <span class="text-[13px] font-semibold text-slate-900">{{ row.price }} <span class="font-normal text-slate-400">/ 1K Tokens</span></span>
            </RouterLink>
          </div>
          <RouterLink to="/api-market" class="home-api-more-link">
            查看全部 API 服务 <ArrowRight class="h-3.5 w-3.5" />
          </RouterLink>
        </div>
      </aside>

      <div class="home-card home-table-card min-w-0 max-w-full">
        <h2>热门套餐行情</h2>
        <div class="home-table-scroll overflow-x-auto">
          <table class="home-exact-table">
            <thead>
              <tr>
                <th>产品</th>
                <th>官网公开价</th>
                <th>
                  <span class="inline-flex items-center gap-1">
                    当前可申请参考
                    <Popover>
                      <PopoverTrigger as-child>
                        <button type="button" class="home-info-trigger" :aria-label="homeTooltipCopy.currentReference.aria">
                          <Info class="h-3 w-3" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent class="w-64 text-xs leading-5" side="top" align="center">
                        <div class="font-semibold text-slate-900">{{ homeTooltipCopy.currentReference.title }}</div>
                        <p class="mt-1 text-slate-500">{{ homeTooltipCopy.currentReference.description }}</p>
                      </PopoverContent>
                    </Popover>
                  </span>
                </th>
                <th>
                  <span class="inline-flex items-center gap-1">
                    完成参考（近 30 日）
                    <Popover>
                      <PopoverTrigger as-child>
                        <button type="button" class="home-info-trigger" :aria-label="homeTooltipCopy.completedReference.aria">
                          <Info class="h-3 w-3" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent class="w-64 text-xs leading-5" side="top" align="center">
                        <div class="font-semibold text-slate-900">{{ homeTooltipCopy.completedReference.title }}</div>
                        <p class="mt-1 text-slate-500">{{ homeTooltipCopy.completedReference.description }}</p>
                      </PopoverContent>
                    </Popover>
                  </span>
                </th>
                <th>供需</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in categoryRows" :key="row.key">
                <td>
                  <div class="flex items-center gap-3">
                    <span class="home-product-icon" :class="row.iconTone">
                      <img v-if="row.iconSrc" :src="row.iconSrc" :alt="row.product" class="home-brand-img" />
                      <span v-else>{{ row.iconText }}</span>
                    </span>
                    <span>{{ row.product }}</span>
                  </div>
                </td>
                <td>{{ row.official }}</td>
                <td>
                  <span class="font-semibold text-slate-900">¥{{ row.current }}</span>
                  <Badge
                    class="ml-2 text-[10px]"
                    :class="row.available ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-100' : 'bg-slate-100 text-slate-500 hover:bg-slate-100'"
                  >
                    {{ row.available ? '可申请' : '参考' }}
                  </Badge>
                </td>
                <td><span class="font-semibold text-slate-900">¥{{ row.completed }}</span></td>
                <td>{{ row.supply }}</td>
                <td>
                  <RouterLink :to="row.to" class="home-table-action-link">
                    查看全部 <ArrowRight class="h-3 w-3" />
                  </RouterLink>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>
