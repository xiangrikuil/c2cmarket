<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Activity, CircleDollarSign, CircleHelp, Code2, Filter, Search, ShieldCheck, Sparkles, Upload } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import TablePagination from '@/components/market/TablePagination.vue'
import ApiPackageCard from '@/components/api-market/ApiPackageCard.vue'
import { usePagination } from '@/composables/usePagination'
import { rankApiPackages } from '@/lib/apiPackageRecommendation'
import {
  getApiMerchantAvatarText,
  getApiMerchantDisplayName,
  getApiMerchantProfileUrl,
  formatUsdQuota,
  type ApiBillingMode,
  type ApiService,
  type MinimumPurchaseFilter,
  type OtherApiMarketFilters,
  type OtherApiMarketSort,
  type Sub2ApiMarketFilters,
  type Sub2ApiMarketSort,
} from '@/lib/api'
import { useApiServices, useOtherApiMarketQuery, useSub2ApiMarketQuery } from '@/queries/useMarketQueries'

type MerchantFilter = 'all' | 'personal_first' | 'personal' | 'api'
type Panel = 'sub2api' | 'packages' | 'other'
type OnlineFilter = 'all' | 'online' | 'offline'
type ImageFilter = 'all' | 'supported' | 'none'
type BillingFilter = 'all' | ApiBillingMode
type DistributionFilter = OtherApiMarketFilters['distributionSystem']

const route = useRoute()
const router = useRouter()

const routePanel = (value: unknown): Panel => value === 'packages' ? 'packages' : value === 'other' ? 'other' : 'sub2api'
const activePanel = ref<Panel>(routePanel(route.query.panel))

const sub2Search = ref('')
const sub2Model = ref('全部')
const sub2CreditPriceMax = ref('all')
const sub2ImageCapability = ref<ImageFilter>('all')
const sub2MinimumPurchase = ref<MinimumPurchaseFilter>('all')
const sub2Online = ref<OnlineFilter>('all')
const sub2Merchant = ref<MerchantFilter>('all')
const sub2TrustLevel = ref('all')
const sub2Sort = ref<Sub2ApiMarketSort>('recommended')

const otherSearch = ref('')
const otherDistribution = ref<DistributionFilter>('all')
const otherBilling = ref<BillingFilter>('all')
const otherMinimumPurchase = ref<MinimumPurchaseFilter>('all')
const otherOnline = ref<OnlineFilter>('all')
const otherSort = ref<OtherApiMarketSort>('recommended')
const packageModel = ref('')
const packageDuration = ref('')

watch(
  () => route.query.panel,
  value => {
    const next = routePanel(value)
    activePanel.value = next
    if (value !== next) {
      router.replace({ query: { ...route.query, panel: next } })
    }
  },
  { immediate: true },
)

function setPanel(panel: Panel) {
  activePanel.value = panel
  router.push({ query: { ...route.query, panel } })
}

function onlineValue(value: OnlineFilter) {
  if (value === 'online') return true
  if (value === 'offline') return false
  return undefined
}

function creditPriceMax(value: string) {
  if (value === 'lte_030') return 0.3
  if (value === 'lte_050') return 0.5
  if (value === 'lte_080') return 0.8
  return undefined
}

const sub2Filters = computed<Sub2ApiMarketFilters>(() => ({
  search: sub2Search.value.trim() || undefined,
  model: sub2Model.value === '全部' ? undefined : sub2Model.value,
  creditPriceMax: creditPriceMax(sub2CreditPriceMax.value),
  imageCapability: sub2ImageCapability.value,
  minimumPurchase: sub2MinimumPurchase.value,
  online: onlineValue(sub2Online.value),
  merchantPreference: sub2Merchant.value === 'all' ? undefined : sub2Merchant.value,
  trustLevel: sub2TrustLevel.value === 'all' ? undefined : Number(sub2TrustLevel.value),
  sort: sub2Sort.value,
}))

const otherFilters = computed<OtherApiMarketFilters>(() => ({
  search: otherSearch.value.trim() || undefined,
  distributionSystem: otherDistribution.value,
  billingMode: otherBilling.value,
  minimumPurchase: otherMinimumPurchase.value,
  online: onlineValue(otherOnline.value),
  sort: otherSort.value,
}))

const { data: sub2Data } = useSub2ApiMarketQuery(sub2Filters)
const { data: otherData } = useOtherApiMarketQuery(otherFilters)
const { data: allServicesData } = useApiServices()

const sub2Rows = computed(() => (sub2Data.value ?? []).filter(row => row.billingMode !== 'fixed_package'))
const otherRows = computed(() => (otherData.value ?? []).filter(row => row.billingMode !== 'fixed_package'))
const packageServices = computed(() => (allServicesData.value ?? []).filter(row => row.billingMode === 'fixed_package'))
const packageModelOptions = computed(() => {
  const options = new Map<string, string>()
  for (const service of packageServices.value) {
    for (const item of service.packages ?? []) {
      if (!item.enabled || item.stockAvailable <= 0) continue
      for (const model of item.models) options.set(model.modelCatalogId, model.modelName)
    }
  }
  return [...options.entries()].map(([id, name]) => ({ id, name })).sort((left, right) => left.name.localeCompare(right.name))
})
const packageRows = computed(() => rankApiPackages(packageServices.value, packageModel.value, Number(packageDuration.value)))
const packageReady = computed(() => Boolean(packageModel.value && packageDuration.value))
const totalAvailablePackages = computed(() => packageServices.value.reduce((total, service) => total + (service.packages ?? []).filter(item => item.enabled && item.stockAvailable > 0).length, 0))
const sub2Pagination = usePagination(sub2Rows)
const otherPagination = usePagination(otherRows)

const activePanelLabel = computed(() => activePanel.value === 'sub2api' ? 'Sub2API 美元额度' : activePanel.value === 'packages' ? '限时流量包' : '其他 API 接入')

const sub2Chips = computed(() => {
  const chips: { label: string, reset: () => void }[] = []
  if (sub2ImageCapability.value !== 'all') chips.push({ label: sub2ImageCapability.value === 'supported' ? '支持生图' : '不支持生图', reset: () => { sub2ImageCapability.value = 'all' } })
  if (sub2MinimumPurchase.value !== 'all') chips.push({ label: minimumPurchaseLabel(sub2MinimumPurchase.value), reset: () => { sub2MinimumPurchase.value = 'all' } })
  if (sub2TrustLevel.value !== 'all') chips.push({ label: `信任等级${sub2TrustLevel.value}+`, reset: () => { sub2TrustLevel.value = 'all' } })
  return chips
})

const sub2AdvancedCount = computed(() => sub2Chips.value.length)
const otherAdvancedCount = computed(() => otherChips.value.length)

const sub2SignalStats = computed(() => {
  const rows = sub2Rows.value
  return [
    { label: '可浏览服务', value: `${rows.length}`, detail: `${onlineCount(rows)} 个在线`, tone: 'primary' },
    { label: '支持生图', value: `${rows.filter(row => row.imagePricing.supported).length}`, detail: '按商户声明展示', tone: 'success' },
    { label: '最低订单', value: minimumIntentLabel(rows), detail: '创建订单起点', tone: 'warning' },
    { label: '响应中位', value: responseMedianLabel(rows), detail: '在线商户参考', tone: 'info' },
  ]
})

const otherSignalStats = computed(() => {
  const rows = otherRows.value
  return [
    { label: '可浏览服务', value: `${rows.length}`, detail: `${uniqueMerchantCount(rows)} 个商户`, tone: 'primary' },
    { label: '在线服务', value: `${onlineCount(rows)}`, detail: '可创建订单', tone: 'success' },
    { label: '最低订单', value: minimumIntentLabel(rows), detail: '不同系统单独比较', tone: 'warning' },
    { label: '响应中位', value: responseMedianLabel(rows), detail: '在线商户参考', tone: 'info' },
  ]
})

const packageSignalStats = computed(() => [
  { label: '在售套餐', value: `${totalAvailablePackages.value}`, detail: `${packageServices.value.length} 个服务`, tone: 'primary' },
  { label: '精确模型', value: `${packageModelOptions.value.length}`, detail: '先选择再推荐', tone: 'success' },
  { label: '有效期', value: '1 / 3 / 7 / 30', detail: '交付后开始计算', tone: 'warning' },
  { label: '当前结果', value: packageReady.value ? `${packageRows.value.length}` : '待选择', detail: '只保留综合推荐', tone: 'info' },
])

const activeSignalStats = computed(() => activePanel.value === 'sub2api' ? sub2SignalStats.value : activePanel.value === 'packages' ? packageSignalStats.value : otherSignalStats.value)

function onlineCount(rows: ApiService[]) {
  return rows.filter(row => row.publiclyOrderable).length
}

function uniqueMerchantCount(rows: ApiService[]) {
  return new Set(rows.map(row => row.merchantId)).size
}

function minimumIntentLabel(rows: ApiService[]) {
  if (!rows.length) return '暂无'
  return `¥${Math.min(...rows.map(row => row.minimumPurchaseCny))} 起`
}

function responseMedianLabel(rows: ApiService[]) {
  const onlineRows = rows.filter(row => row.publiclyOrderable)
  if (!onlineRows.length) return '暂无'
  const total = onlineRows.reduce((sum, row) => sum + row.responseMedianMinutes, 0)
  return `约 ${Math.round(total / onlineRows.length)} 分钟`
}

function signalStatClass(tone: string) {
  if (tone === 'success') return 'api-market-stat--success'
  if (tone === 'warning') return 'api-market-stat--warning'
  if (tone === 'info') return 'api-market-stat--info'
  return 'api-market-stat--primary'
}

const otherChips = computed(() => {
  const chips: { label: string, reset: () => void }[] = []
  if (otherDistribution.value && otherDistribution.value !== 'all') chips.push({ label: otherDistribution.value, reset: () => { otherDistribution.value = 'all' } })
  if (otherBilling.value !== 'all') chips.push({ label: billingModeLabel(otherBilling.value), reset: () => { otherBilling.value = 'all' } })
  if (otherMinimumPurchase.value !== 'all') chips.push({ label: minimumPurchaseLabel(otherMinimumPurchase.value), reset: () => { otherMinimumPurchase.value = 'all' } })
  return chips
})

function merchantIdentity(row: { merchantType: string }) {
  if (row.merchantType === '商户') return 'API 商户'
  if (row.merchantType === '可信新车主') return '可信新商户'
  return '个人商户'
}

function billingModeLabel(value: ApiBillingMode) {
  if (value === 'metered_credit') return '精确额度计费'
  if (value === 'manual_credit') return '商户手工核对'
  return '固定套餐'
}

function minimumPurchaseLabel(value: MinimumPurchaseFilter) {
  if (value === 'lte_20') return '≤ 20 元'
  if (value === 'between_21_50') return '21-50 元'
  if (value === 'gt_50') return '> 50 元'
  return '不限'
}

function accessConfirmationLabel(row: ApiService) {
  return row.delivery
}

function usageVerificationLabel() {
  return '用量与余额由商户说明，买家自行核对'
}

function creditPriceLabel(row: ApiService) {
  const cnyPerUsdCredit = row.creditPerCny > 0 ? 1 / row.creditPerCny : 0
  return `¥${cnyPerUsdCredit.toFixed(2).replace(/\.?0+$/, '')} / $1`
}

function imagePricingLabel(row: ApiService) {
  if (!row.imagePricing.supported) return '不支持'
  const prices = [
    row.imagePricing.oneKPriceUsd ? `1K $${row.imagePricing.oneKPriceUsd}` : '',
    row.imagePricing.twoKPriceUsd ? `2K $${row.imagePricing.twoKPriceUsd}` : '',
    row.imagePricing.fourKPriceUsd ? `4K $${row.imagePricing.fourKPriceUsd}` : '',
  ].filter(Boolean)
  return prices.length ? prices.join(' / ') : '支持'
}

function warrantyDisplay(value: string) {
  return value
    .replace(/买家专属、可撤销的/g, '买家专属的')
    .replace(/支持撤销/g, '支持站外协商更换')
}

function visibleBadges(items: string[]) {
  return { shown: items.slice(0, 3), hidden: Math.max(0, items.length - 3) }
}

function modelMultiplierBadges(row: ApiService) {
  return row.modelMultipliers.map(item => `${item.model} · ${item.multiplier}`)
}

function statusLabel(row: Pick<ApiService, 'state' | 'online' | 'publiclyOrderable'>) {
  if (row.state === 'reviewing') return { text: '审核中', dot: 'bg-amber-500', textClass: 'text-amber-700' }
  if (row.state === 'paused') return { text: '暂停接单', dot: 'bg-red-500', textClass: 'text-red-700' }
  if (row.publiclyOrderable) return { text: '可创建订单', dot: 'bg-emerald-500', textClass: 'text-emerald-700' }
  if (row.online) return { text: '待配置接单', dot: 'bg-amber-500', textClass: 'text-amber-700' }
  return { text: '离线', dot: 'bg-muted-foreground', textClass: 'text-muted-foreground' }
}

function merchantProfileUrl(row: ApiService) {
  return getApiMerchantProfileUrl(row)
}

function openService(event: MouseEvent | KeyboardEvent, row: ApiService) {
  if (!row.publiclyOrderable) return
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button,input,select')) return
  router.push(`/api-market/${row.id}`)
}
</script>

<template>
  <div class="api-market-page">
    <div class="api-market-layout">
      <main class="min-w-0 space-y-4">
      <section class="api-market-hero">
      <div class="api-market-hero-main">
        <div class="api-market-kicker">
          <Code2 class="h-4 w-4" />
          <span>API 服务目录</span>
          <Badge variant="secondary">{{ activePanelLabel }}</Badge>
        </div>
        <div class="mt-3 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <h1 class="text-[32px] font-semibold leading-tight tracking-normal md:text-[38px]">连接优质 API 服务，按需灵活接入</h1>
            <p class="mt-2 max-w-3xl text-sm leading-6">
              按服务来源分面板查看 API 服务；创建订单后按参与方权限查看交易资料，平台不在公开页、列表或管理摘要展示 API Key、token、面板账号或密码。
            </p>
          </div>
        </div>
      </div>

      <div class="api-market-stats">
        <div
          v-for="item in activeSignalStats"
          :key="item.label"
          class="api-market-stat"
          :class="signalStatClass(item.tone)"
        >
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.detail }}</small>
        </div>
      </div>
      <div class="api-market-hero-art" aria-hidden="true"><span>API</span><i /><i /><i /></div>
    </section>

      <section class="api-market-panel-switch">
      <button
        type="button"
        class="api-market-panel-card"
        :class="activePanel === 'packages' ? 'api-market-panel-card--active' : ''"
        @click="setPanel('packages')"
      >
        <span class="api-market-panel-icon"><CircleDollarSign class="h-4 w-4" /></span>
        <span class="min-w-0">
          <span class="api-market-panel-title">限时流量包</span>
          <span class="api-market-panel-desc">按精确模型和有效期比较综合性价比</span>
        </span>
        <Badge variant="trust">{{ totalAvailablePackages }} 个</Badge>
      </button>
      <button
        type="button"
        class="api-market-panel-card"
        :class="activePanel === 'sub2api' ? 'api-market-panel-card--active' : ''"
        @click="setPanel('sub2api')"
      >
        <span class="api-market-panel-icon"><Sparkles class="h-4 w-4" /></span>
          <span class="min-w-0">
            <span class="api-market-panel-title">Sub2API 标准额度</span>
          <span class="api-market-panel-desc">统一倍率，优先比较美元额度售价、生图价格和履约记录</span>
        </span>
        <Badge variant="trust">{{ sub2Rows.length }} 条</Badge>
      </button>
      <button
        type="button"
        class="api-market-panel-card"
        :class="activePanel === 'other' ? 'api-market-panel-card--active' : ''"
        @click="setPanel('other')"
      >
        <span class="api-market-panel-icon"><Activity class="h-4 w-4" /></span>
        <span class="min-w-0">
          <span class="api-market-panel-title">其他 API 接入</span>
          <span class="api-market-panel-desc">NewAPI Proxy、自建代理、固定套餐和商户手工核对单独展示</span>
        </span>
        <Badge variant="trust">{{ otherRows.length }} 条</Badge>
      </button>
    </section>

      <section v-if="activePanel === 'sub2api'" class="space-y-4">
      <div class="api-market-notice">
        <div class="api-market-notice-icon"><ShieldCheck class="h-4 w-4" /></div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <div class="font-semibold text-slate-950">Sub2API 默认优先</div>
            <Badge variant="verified">默认 1.00x</Badge>
            <Badge variant="secondary">可售美元额度</Badge>
          </div>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">
            默认倍率为 1，商家可按实际上游规则填写 0.01 等倍率，并可为具体模型单独设置。买家主要比较美元额度售价、模型倍率、商户承诺和履约记录；接入细节和用量核对由双方站外确认。
          </p>
        </div>
      </div>

      <div class="api-market-filterbar c2c-filterbar rounded-lg border border-border bg-card px-3 py-3">
        <div class="api-market-source-tabs mb-3 flex flex-wrap gap-2 border-b border-border pb-3">
          <Button size="sm" variant="default" @click="setPanel('sub2api')">Sub2API 美元额度</Button>
          <Button size="sm" variant="outline" @click="setPanel('packages')">限时流量包</Button>
          <Button size="sm" variant="outline" @click="setPanel('other')">其他 API 接入</Button>
        </div>
        <div class="grid gap-2 xl:grid-cols-[minmax(220px,1fr)_120px_130px_110px_150px_auto_150px]">
          <label class="api-market-search-field">
            <Search class="h-4 w-4" />
            <Input v-model="sub2Search" name="sub2api-service-search" class="h-8 border-0 bg-transparent pl-8 text-sm shadow-none focus-visible:ring-0" placeholder="搜索服务或商户" />
          </label>
          <select v-model="sub2Model" class="api-market-select">
            <option>全部</option>
            <option>GPT</option>
            <option>Claude</option>
            <option>Gemini</option>
          </select>
          <select v-model="sub2CreditPriceMax" class="api-market-select">
            <option value="all">额度售价不限</option>
            <option value="lte_030">≤ ¥0.30 / $1</option>
            <option value="lte_050">≤ ¥0.50 / $1</option>
            <option value="lte_080">≤ ¥0.80 / $1</option>
          </select>
          <select v-model="sub2Online" class="api-market-select">
            <option value="all">全部状态</option>
            <option value="online">仅在线</option>
            <option value="offline">离线/暂停</option>
          </select>
          <select v-model="sub2Merchant" class="api-market-select">
            <option value="all">全部商户</option>
            <option value="personal_first">个人优先</option>
            <option value="personal">个人商户</option>
            <option value="api">API 商户</option>
          </select>
          <Popover>
            <PopoverTrigger as-child>
              <Button class="h-8" size="sm" variant="outline">
                <Filter class="h-4 w-4" />
                更多筛选<span v-if="sub2AdvancedCount">· {{ sub2AdvancedCount }}</span>
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-[360px]">
              <div class="grid gap-3">
                <div class="text-sm font-medium">Sub2API 筛选</div>
                <label class="grid gap-1 text-xs text-muted-foreground">生图能力
                  <select v-model="sub2ImageCapability" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">全部</option>
                    <option value="supported">支持生图</option>
                    <option value="none">不支持生图</option>
                  </select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">最低订单金额
                  <select v-model="sub2MinimumPurchase" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">不限</option>
                    <option value="lte_20">≤ 20 元</option>
                    <option value="between_21_50">21-50 元</option>
                    <option value="gt_50">&gt; 50 元</option>
                  </select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">信任等级
                  <select v-model="sub2TrustLevel" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">全部</option>
                    <option value="2">信任等级2+</option>
                    <option value="3">信任等级3+</option>
                    <option value="4">信任等级4</option>
                  </select>
                </label>
              </div>
            </PopoverContent>
          </Popover>
          <select v-model="sub2Sort" class="api-market-select">
            <option value="recommended">综合推荐</option>
            <option value="credit_price_asc">额度售价最低</option>
            <option value="minimum_purchase_asc">最低订单金额</option>
            <option value="response_fast">响应最快</option>
            <option value="recent">最近上架</option>
          </select>
        </div>
        <div v-if="sub2Chips.length" class="mt-2 flex items-center gap-2 border-t border-border pt-2">
          <span class="shrink-0 text-xs text-muted-foreground">已选</span>
          <div class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto">
            <Badge v-for="chip in sub2Chips" :key="chip.label" variant="trust" class="cursor-pointer" @click="chip.reset">{{ chip.label }} ×</Badge>
          </div>
          <span class="shrink-0 text-xs text-muted-foreground">共 {{ sub2Rows.length }} 条服务</span>
        </div>
        <div v-else class="mt-2 flex justify-end border-t border-border pt-2 text-xs text-muted-foreground">
          共 {{ sub2Rows.length }} 条服务
        </div>
      </div>

      <div v-if="sub2Rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无 Sub2API 标准额度服务。</div>
      <template v-else>
        <div class="api-service-card-grid">
          <Card
            v-for="row in sub2Pagination.paginatedRows.value"
            :key="row.id"
            class="api-service-market-card"
            :class="row.publiclyOrderable ? 'cursor-pointer' : 'cursor-not-allowed opacity-70'"
            :tabindex="row.publiclyOrderable ? 0 : -1"
            @click="openService($event, row)"
            @keydown.enter="openService($event, row)"
          >
            <div class="api-service-card-head">
              <span class="api-service-card-logo"><Code2 class="h-5 w-5" /></span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2"><h2 class="truncate font-semibold text-slate-950">{{ row.title }}</h2><Badge variant="verified">标准额度</Badge></div>
                <div class="mt-1 flex flex-wrap gap-1"><Badge v-for="m in visibleBadges(modelMultiplierBadges(row)).shown" :key="m" variant="model">{{ m }}</Badge><Badge v-if="visibleBadges(modelMultiplierBadges(row)).hidden" variant="model">+{{ visibleBadges(modelMultiplierBadges(row)).hidden }}</Badge></div>
              </div>
              <div class="shrink-0 text-right"><div class="api-service-card-price">{{ creditPriceLabel(row) }}</div><div class="mt-1 text-xs text-muted-foreground">可售 {{ formatUsdQuota(row.balance) }}</div></div>
            </div>
            <dl class="api-service-card-metrics">
              <div><dt>接入方式</dt><dd>{{ accessConfirmationLabel(row) }}</dd></div>
              <div><dt>生图价格</dt><dd>{{ imagePricingLabel(row) }}</dd></div>
              <div><dt>最低订单</dt><dd>¥{{ row.minimumPurchaseCny }} 起</dd></div>
              <div><dt>响应参考</dt><dd>{{ row.publiclyOrderable ? `约 ${row.responseMedianMinutes} 分钟` : '暂不可接单' }}</dd></div>
            </dl>
            <p class="api-service-card-note">{{ usageVerificationLabel() }} · {{ warrantyDisplay(row.warranty) }}</p>
            <div class="api-service-card-footer">
              <component :is="merchantProfileUrl(row) ? RouterLink : 'div'" :to="merchantProfileUrl(row) || undefined" class="api-market-merchant">
                <span class="api-market-avatar">{{ getApiMerchantAvatarText(row) }}</span>
                <span class="min-w-0"><span class="block truncate text-sm font-medium">{{ getApiMerchantDisplayName(row) }}</span><span class="mt-0.5 flex flex-wrap gap-1"><Badge variant="identity">{{ merchantIdentity(row) }}</Badge><Badge variant="trust">信任等级{{ row.trustLevel }}</Badge></span></span>
              </component>
              <div class="text-right"><div class="flex items-center justify-end gap-1 text-sm" :class="statusLabel(row).textClass"><span class="h-2 w-2 rounded-full" :class="statusLabel(row).dot"></span>{{ statusLabel(row).text }}</div><div class="mt-1 text-xs font-medium text-primary">查看服务 →</div></div>
            </div>
          </Card>
        </div>
        <div class="mt-4 rounded-xl border border-border bg-card px-4 py-3">
          <TablePagination v-model:page="sub2Pagination.page.value" :page-count="sub2Pagination.pageCount.value" :total="sub2Pagination.total.value" :start-item="sub2Pagination.startItem.value" :end-item="sub2Pagination.endItem.value" />
        </div>
      </template>
    </section>

      <section v-else-if="activePanel === 'packages'" class="space-y-4">
        <div class="api-market-filterbar rounded-lg border border-border bg-card px-3 py-3">
          <div class="api-market-source-tabs mb-3 flex flex-wrap gap-2 border-b border-border pb-3">
            <Button size="sm" variant="outline" @click="setPanel('sub2api')">Sub2API 美元额度</Button>
            <Button size="sm" variant="default" @click="setPanel('packages')">限时流量包</Button>
            <Button size="sm" variant="outline" @click="setPanel('other')">其他 API 接入</Button>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
              精确模型
              <select v-model="packageModel" class="api-market-select h-10">
                <option value="">请选择模型</option>
                <option v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</option>
              </select>
            </label>
            <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
              套餐有效期
              <select v-model="packageDuration" class="api-market-select h-10">
                <option value="">请选择有效期</option>
                <option value="1">1 天</option>
                <option value="3">3 天</option>
                <option value="7">7 天</option>
                <option value="30">30 天</option>
              </select>
            </label>
          </div>
          <p class="mt-3 border-t border-border pt-3 text-xs leading-5 text-muted-foreground">选择后按价值 60%、履约 25%、响应 10%、新鲜度 5% 计算综合推荐；倍率和价值成本按商家声明估算。</p>
        </div>

        <div v-if="!packageReady" class="rounded-lg border border-dashed border-border bg-card p-8 text-center">
          <div class="text-sm font-semibold">先选择精确模型和有效期</div>
          <p class="mt-2 text-xs text-muted-foreground">选择完成后才会展示可购买套餐和综合推荐顺序。</p>
        </div>
        <div v-else-if="packageRows.length === 0" class="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前模型和有效期下暂无有库存的套餐。</div>
        <div v-else class="api-service-card-grid">
          <ApiPackageCard v-for="(row, index) in packageRows" :key="row.package.id" :row="row" :rank="index + 1" />
        </div>
      </section>

      <section v-else class="space-y-4">
      <div class="api-market-notice">
        <div class="api-market-notice-icon"><CircleDollarSign class="h-4 w-4" /></div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <div class="font-semibold text-slate-950">其他 API 接入不与 Sub2API 混合排名。</div>
            <Badge variant="secondary">单独比较</Badge>
            <Badge variant="capability">多分发系统</Badge>
          </div>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">用户需手动切换到本面板查看。这里展示 NewAPI Proxy、自建代理、固定套餐、商户手工核对和其他系统。</p>
        </div>
      </div>

      <div class="api-market-filterbar c2c-filterbar rounded-lg border border-border bg-card px-3 py-3">
        <div class="api-market-source-tabs mb-3 flex flex-wrap gap-2 border-b border-border pb-3">
          <Button size="sm" variant="outline" @click="setPanel('sub2api')">Sub2API 美元额度</Button>
          <Button size="sm" variant="outline" @click="setPanel('packages')">限时流量包</Button>
          <Button size="sm" variant="default" @click="setPanel('other')">其他 API 接入</Button>
        </div>
        <div class="grid gap-2 xl:grid-cols-[minmax(220px,1fr)_150px_150px_120px_auto_150px]">
          <label class="api-market-search-field">
            <Search class="h-4 w-4" />
            <Input v-model="otherSearch" name="other-api-service-search" class="h-8 border-0 bg-transparent pl-8 text-sm shadow-none focus-visible:ring-0" placeholder="搜索套餐或商户" />
          </label>
          <select v-model="otherDistribution" class="api-market-select">
            <option value="all">全部分发系统</option>
            <option value="NewAPI Proxy">NewAPI Proxy</option>
            <option value="自建中转">自建代理</option>
            <option value="固定套餐">固定套餐</option>
            <option value="商户手工核对">商户手工核对</option>
            <option value="其他">其他系统</option>
          </select>
          <select v-model="otherBilling" class="api-market-select">
            <option value="all">全部计费方式</option>
            <option value="metered_credit">精确额度计费</option>
            <option value="manual_credit">商户手工核对</option>
            <option value="fixed_package">固定套餐</option>
          </select>
          <select v-model="otherOnline" class="api-market-select">
            <option value="all">全部状态</option>
            <option value="online">仅在线</option>
            <option value="offline">离线/暂停</option>
          </select>
          <Popover>
            <PopoverTrigger as-child>
              <Button class="h-8" size="sm" variant="outline">
                <Filter class="h-4 w-4" />
                更多筛选<span v-if="otherAdvancedCount">· {{ otherAdvancedCount }}</span>
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-[320px]">
              <div class="grid gap-3">
                <div class="text-sm font-medium">其他 API 筛选</div>
                <label class="grid gap-1 text-xs text-muted-foreground">最低订单金额
                  <select v-model="otherMinimumPurchase" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">不限</option>
                    <option value="lte_20">≤ 20 元</option>
                    <option value="between_21_50">21-50 元</option>
                    <option value="gt_50">&gt; 50 元</option>
                  </select>
                </label>
              </div>
            </PopoverContent>
          </Popover>
          <select v-model="otherSort" class="api-market-select">
            <option value="recommended">综合推荐</option>
            <option value="minimum_purchase_asc">最低订单金额</option>
            <option value="response_fast">响应最快</option>
            <option value="recent">最近上架</option>
          </select>
        </div>
        <div v-if="otherChips.length" class="mt-2 flex items-center gap-2 border-t border-border pt-2">
          <span class="shrink-0 text-xs text-muted-foreground">已选</span>
          <div class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto">
            <Badge v-for="chip in otherChips" :key="chip.label" variant="trust" class="cursor-pointer" @click="chip.reset">{{ chip.label }} ×</Badge>
          </div>
          <span class="shrink-0 text-xs text-muted-foreground">共 {{ otherRows.length }} 条服务</span>
        </div>
        <div v-else class="mt-2 flex justify-end border-t border-border pt-2 text-xs text-muted-foreground">
          共 {{ otherRows.length }} 条服务
        </div>
      </div>

      <div v-if="otherRows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无其他 API 接入服务。</div>
      <template v-else>
        <div class="api-service-card-grid">
          <Card
            v-for="row in otherPagination.paginatedRows.value"
            :key="row.id"
            class="api-service-market-card"
            :class="row.publiclyOrderable ? 'cursor-pointer' : 'cursor-not-allowed opacity-70'"
            :tabindex="row.publiclyOrderable ? 0 : -1"
            @click="openService($event, row)"
            @keydown.enter="openService($event, row)"
          >
            <div class="api-service-card-head">
              <span class="api-service-card-logo api-service-card-logo--other"><Activity class="h-5 w-5" /></span>
              <div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h2 class="truncate font-semibold text-slate-950">{{ row.title }}</h2><Badge variant="secondary">{{ row.delivery }}</Badge></div><p class="mt-1 truncate text-xs text-muted-foreground">{{ billingModeLabel(row.billingMode) }}</p></div>
              <div class="shrink-0 text-right"><div class="api-service-card-price">¥{{ row.minimumPurchaseCny }} 起</div><div class="mt-1 text-xs text-muted-foreground">最低订单</div></div>
            </div>
            <dl class="api-service-card-metrics">
              <div><dt>分发系统</dt><dd>{{ row.delivery }}</dd></div>
              <div><dt>计费方式</dt><dd>{{ billingModeLabel(row.billingMode) }}</dd></div>
              <div><dt>接入核对</dt><dd>{{ accessConfirmationLabel(row) }}</dd></div>
              <div><dt>响应参考</dt><dd>{{ row.publiclyOrderable ? `约 ${row.responseMedianMinutes} 分钟` : '暂不可接单' }}</dd></div>
            </dl>
            <p class="api-service-card-note">{{ usageVerificationLabel() }} · {{ warrantyDisplay(row.warranty) }}</p>
            <div class="api-service-card-footer">
              <component :is="merchantProfileUrl(row) ? RouterLink : 'div'" :to="merchantProfileUrl(row) || undefined" class="api-market-merchant"><span class="api-market-avatar">{{ getApiMerchantAvatarText(row) }}</span><span class="min-w-0"><span class="block truncate text-sm font-medium">{{ getApiMerchantDisplayName(row) }}</span><span class="mt-0.5 flex flex-wrap gap-1"><Badge variant="identity">{{ merchantIdentity(row) }}</Badge><Badge variant="trust">信任等级{{ row.trustLevel }}</Badge></span></span></component>
              <div class="text-right"><div class="flex items-center justify-end gap-1 text-sm" :class="statusLabel(row).textClass"><span class="h-2 w-2 rounded-full" :class="statusLabel(row).dot"></span>{{ statusLabel(row).text }}</div><div class="mt-1 text-xs font-medium text-primary">查看服务 →</div></div>
            </div>
          </Card>
        </div>
        <div class="mt-4 rounded-xl border border-border bg-card px-4 py-3"><TablePagination v-model:page="otherPagination.page.value" :page-count="otherPagination.pageCount.value" :total="otherPagination.total.value" :start-item="otherPagination.startItem.value" :end-item="otherPagination.endItem.value" /></div>
      </template>
      </section>
      </main>

      <aside class="api-market-aside space-y-3">
        <Card class="api-market-check-card p-4">
          <div class="flex items-center gap-2 font-semibold"><ShieldCheck class="h-4 w-4 text-cyan-700" />下单前确认</div>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
            <li>• 比较额度售价、最低订单与商户履约记录</li>
            <li>• 确认接入方式、用量核对和售后承诺</li>
            <li>• 付款、Key 与账号资料仅按订单参与方权限展示</li>
          </ul>
        </Card>
        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><CircleHelp class="h-4 w-4 text-primary" />新手帮助</div>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground"><li>如何选择合适的 API 服务</li><li>计费方式与倍率说明</li><li>常见问题解答</li></ul>
        </Card>
        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><CircleHelp class="h-4 w-4 text-primary" />交易说明</div>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">平台记录订单状态，不代收款、不托管额度，也不公开展示 API Key、token、面板账号或密码。</p>
        </Card>
        <RouterLink to="/api-market/new"><Button class="w-full" variant="outline"><Upload class="h-4 w-4" />发布 API 服务</Button></RouterLink>
      </aside>
    </div>
  </div>
</template>
