<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Activity, CircleDollarSign, Clock, Code2, Filter, Search, ShieldCheck, ShoppingBag, Sparkles, Upload, UsersRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  canOpenApiMerchantProfile,
  getApiDeliveryModeLabel,
  getApiMerchantAvatarText,
  getApiMerchantDisplayName,
  getApiMerchantProfileUrl,
  getApiMerchantVisibilityLabel,
  getApiUsageVisibilityLabel,
  formatUsdQuota,
  type ApiBillingMode,
  type ApiDeliveryMode,
  type ApiService,
  type ApiUsageVisibility,
  type MinimumPurchaseFilter,
  type OtherApiMarketFilters,
  type OtherApiMarketSort,
  type Sub2ApiMarketFilters,
  type Sub2ApiMarketSort,
} from '@/lib/api'
import { useOtherApiMarketQuery, useSub2ApiMarketQuery } from '@/queries/useMarketQueries'

type MerchantFilter = 'all' | 'personal_first' | 'personal' | 'api'
type Panel = 'sub2api' | 'other'
type DeliveryFilter = 'all' | ApiDeliveryMode
type OnlineFilter = 'all' | 'online' | 'offline'
type ImageFilter = 'all' | 'supported' | 'none'
type BillingFilter = 'all' | ApiBillingMode
type DistributionFilter = OtherApiMarketFilters['distributionSystem']

const route = useRoute()
const router = useRouter()

const activePanel = ref<Panel>(route.query.panel === 'other' ? 'other' : 'sub2api')

const sub2Search = ref('')
const sub2Model = ref('全部')
const sub2CreditPriceMax = ref('all')
const sub2DeliveryMode = ref<DeliveryFilter>('all')
const sub2ImageCapability = ref<ImageFilter>('all')
const sub2MinimumPurchase = ref<MinimumPurchaseFilter>('all')
const sub2Online = ref<OnlineFilter>('all')
const sub2Merchant = ref<MerchantFilter>('all')
const sub2TrustLevel = ref('all')
const sub2Sort = ref<Sub2ApiMarketSort>('recommended')

const otherSearch = ref('')
const otherDistribution = ref<DistributionFilter>('all')
const otherBilling = ref<BillingFilter>('all')
const otherDeliveryMode = ref<DeliveryFilter>('all')
const otherMinimumPurchase = ref<MinimumPurchaseFilter>('all')
const otherOnline = ref<OnlineFilter>('all')
const otherSort = ref<OtherApiMarketSort>('recommended')

watch(
  () => route.query.panel,
  value => {
    const next = value === 'other' ? 'other' : 'sub2api'
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
  deliveryMode: sub2DeliveryMode.value === 'all' ? undefined : sub2DeliveryMode.value,
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
  deliveryMode: otherDeliveryMode.value === 'all' ? undefined : otherDeliveryMode.value,
  minimumPurchase: otherMinimumPurchase.value,
  online: onlineValue(otherOnline.value),
  sort: otherSort.value,
}))

const { data: sub2Data } = useSub2ApiMarketQuery(sub2Filters)
const { data: otherData } = useOtherApiMarketQuery(otherFilters)

const sub2Rows = computed(() => sub2Data.value ?? [])
const otherRows = computed(() => otherData.value ?? [])
const sub2Pagination = usePagination(sub2Rows)
const otherPagination = usePagination(otherRows)

const activeRows = computed(() => activePanel.value === 'sub2api' ? sub2Rows.value : otherRows.value)
const activePanelLabel = computed(() => activePanel.value === 'sub2api' ? 'Sub2API 标准额度' : '其他 API 接入')

const sub2Chips = computed(() => {
  const chips: { label: string, reset: () => void }[] = []
  if (sub2DeliveryMode.value !== 'all') chips.push({ label: getApiDeliveryModeLabel(sub2DeliveryMode.value), reset: () => { sub2DeliveryMode.value = 'all' } })
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
    { label: '最低意向', value: minimumIntentLabel(rows), detail: '提交意向起点', tone: 'warning' },
    { label: '响应中位', value: responseMedianLabel(rows), detail: '在线商户参考', tone: 'info' },
  ]
})

const otherSignalStats = computed(() => {
  const rows = otherRows.value
  return [
    { label: '可浏览服务', value: `${rows.length}`, detail: `${uniqueMerchantCount(rows)} 个商户`, tone: 'primary' },
    { label: '在线服务', value: `${onlineCount(rows)}`, detail: '可提交购买意向', tone: 'success' },
    { label: '最低意向', value: minimumIntentLabel(rows), detail: '不同系统单独比较', tone: 'warning' },
    { label: '响应中位', value: responseMedianLabel(rows), detail: '在线商户参考', tone: 'info' },
  ]
})

const activeSignalStats = computed(() => activePanel.value === 'sub2api' ? sub2SignalStats.value : otherSignalStats.value)

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
  if (otherDeliveryMode.value !== 'all') chips.push({ label: getApiDeliveryModeLabel(otherDeliveryMode.value), reset: () => { otherDeliveryMode.value = 'all' } })
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

function deliveryModesLabel(modes: ApiDeliveryMode[]) {
  return modes.map(getApiDeliveryModeLabel).join(' / ')
}

function deliveryModeHint(mode: ApiDeliveryMode) {
  return mode === 'api_key_endpoint' ? '提交意向后查看站外确认说明' : '提交意向后联系商户确认接入方式'
}

function deliveryModeColumnLabel(mode: ApiDeliveryMode) {
  return mode === 'api_key_endpoint' ? '请求地址说明' : '面板接入说明'
}

function deliveryModesHint(modes: ApiDeliveryMode[]) {
  return modes.includes('api_key_endpoint') ? deliveryModeHint('api_key_endpoint') : deliveryModeHint('sub2api_panel_account')
}

function deliveryModePillClass(index: number) {
  return index === 0 ? 'c2c-api-delivery-pill-primary' : 'c2c-api-delivery-pill-secondary'
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

function serviceSummary(row: ApiService) {
  return `${deliveryModesLabel(row.deliveryModes)} · ${getApiUsageVisibilityLabel(row.usageVisibility)} · ${row.warranty}`
}

function capabilityBadges(row: ApiService) {
  return [
    getApiUsageVisibilityLabel(row.usageVisibility),
    row.imagePricing.supported ? '支持生图' : '不支持生图',
    row.warranty.includes('补') || row.warranty.includes('承诺') || row.warranty.includes('24') ? '商户承诺' : '售后协商',
  ]
}

function visibleBadges(items: string[]) {
  return { shown: items.slice(0, 3), hidden: Math.max(0, items.length - 3) }
}

function statusLabel(row: Pick<ApiService, 'state' | 'online' | 'publiclyOrderable'>) {
  if (row.state === 'reviewing') return { text: '审核中', dot: 'bg-amber-500', textClass: 'text-amber-700' }
  if (row.state === 'paused') return { text: '暂停接单', dot: 'bg-red-500', textClass: 'text-red-700' }
  if (row.publiclyOrderable) return { text: '可提交意向', dot: 'bg-emerald-500', textClass: 'text-emerald-700' }
  if (row.online) return { text: '待配置接单', dot: 'bg-amber-500', textClass: 'text-amber-700' }
  return { text: '离线', dot: 'bg-muted-foreground', textClass: 'text-muted-foreground' }
}

function merchantProfileUrl(row: ApiService) {
  return getApiMerchantProfileUrl(row)
}
</script>

<template>
  <div class="api-market-page">
    <section class="api-market-hero">
      <div class="api-market-hero-main">
        <div class="api-market-kicker">
          <Code2 class="h-4 w-4" />
          <span>API 服务目录</span>
          <Badge variant="secondary">{{ activePanelLabel }}</Badge>
        </div>
        <div class="mt-3 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <h1 class="text-[32px] font-semibold leading-tight tracking-normal text-slate-950 md:text-[38px]">API 集市</h1>
            <p class="mt-2 max-w-3xl text-sm leading-6 text-slate-600">
              按服务来源分面板查看 API 服务；提交购买意向后直接查看商户联系方式，平台不保存 API Key、token、面板账号或密码。
            </p>
          </div>
          <RouterLink to="/api-market/new" class="w-full shrink-0 sm:w-auto">
            <Button class="api-market-primary-action w-full sm:w-auto">
              <Upload class="h-4 w-4" />
              发布 API 服务
            </Button>
          </RouterLink>
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
    </section>

    <section class="api-market-panel-switch">
      <button
        type="button"
        class="api-market-panel-card"
        :class="activePanel === 'sub2api' ? 'api-market-panel-card--active' : ''"
        @click="setPanel('sub2api')"
      >
        <span class="api-market-panel-icon"><Sparkles class="h-4 w-4" /></span>
        <span class="min-w-0">
          <span class="api-market-panel-title">Sub2API 标准额度</span>
          <span class="api-market-panel-desc">统一倍率，优先比较美元额度售价、接入方式和履约记录</span>
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
            <Badge variant="verified">固定 1.00x</Badge>
            <Badge variant="secondary">可售美元额度</Badge>
          </div>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">
            文本与生图倍率固定 1.00x；买家主要比较美元额度售价、接入方式、用量可见性、生图价格、商户承诺和履约记录。可售美元额度是商户声明的最大可购买额度参考，不由平台发放或托管。
          </p>
        </div>
      </div>

      <div class="api-market-filterbar c2c-filterbar rounded-lg border border-border bg-card px-3 py-3">
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
                <label class="grid gap-1 text-xs text-muted-foreground">接入方式
                  <select v-model="sub2DeliveryMode" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">全部</option>
                    <option value="api_key_endpoint">API 请求地址接入说明</option>
                    <option value="sub2api_panel_account">Sub2API 面板接入说明</option>
                  </select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">生图能力
                  <select v-model="sub2ImageCapability" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">全部</option>
                    <option value="supported">支持生图</option>
                    <option value="none">不支持生图</option>
                  </select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">最低意向金额
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
            <option value="minimum_purchase_asc">最低意向金额</option>
            <option value="panel_supported">支持面板登录</option>
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
      <SoftTable v-else :columns="['服务', '额度售价', '接入方式', '用量可见', '生图价格', '商户承诺', '最低意向', '商户', '状态 / 响应', '操作']">
        <tr v-for="row in sub2Pagination.paginatedRows.value" :key="row.id" class="api-market-table-row">
          <td class="api-market-service-cell">
            <div class="font-semibold text-slate-900">{{ row.title }}</div>
            <div class="mt-2 flex flex-wrap gap-1">
              <Badge v-for="m in visibleBadges(row.models).shown" :key="m" variant="model">{{ m }}</Badge>
              <Badge v-if="visibleBadges(row.models).hidden" variant="model">+{{ visibleBadges(row.models).hidden }}</Badge>
            </div>
          </td>
          <td>
            <div class="api-market-price">{{ creditPriceLabel(row) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">可售 {{ formatUsdQuota(row.balance) }}</div>
          </td>
          <td>
            <div class="grid gap-2">
              <div class="flex flex-wrap gap-1.5">
                <span
                  v-for="(mode, index) in row.deliveryModes"
                  :key="mode"
                  class="c2c-api-delivery-pill inline-flex w-fit items-center whitespace-nowrap rounded-full border px-2 py-1 text-xs font-medium leading-none"
                  :class="deliveryModePillClass(index)"
                >
                  {{ deliveryModeColumnLabel(mode) }}
                </span>
              </div>
              <div class="flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
                <span class="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary"></span>
                <span>{{ deliveryModesHint(row.deliveryModes) }}</span>
              </div>
            </div>
          </td>
          <td>{{ getApiUsageVisibilityLabel(row.usageVisibility) }}</td>
          <td>{{ imagePricingLabel(row) }}</td>
          <td>{{ row.warranty }}</td>
          <td><span class="api-market-minimum">¥{{ row.minimumPurchaseCny }} 起</span></td>
          <td>
            <component :is="merchantProfileUrl(row) ? RouterLink : 'div'" :to="merchantProfileUrl(row) || undefined" class="api-market-merchant">
              <span class="api-market-avatar">{{ getApiMerchantAvatarText(row) }}</span>
              <span class="min-w-0">
                <span class="block font-medium">{{ getApiMerchantDisplayName(row) }}</span>
                <span class="mt-0.5 flex flex-wrap gap-1">
                  <Badge variant="identity">{{ merchantIdentity(row) }}</Badge>
                  <Badge variant="trust">信任等级{{ row.trustLevel }}</Badge>
                  <Badge v-if="!canOpenApiMerchantProfile(row)" variant="secondary">{{ getApiMerchantVisibilityLabel(row) }}</Badge>
                </span>
              </span>
            </component>
          </td>
          <td>
            <div class="flex items-center gap-1 text-sm" :class="statusLabel(row).textClass">
              <span class="h-2 w-2 rounded-full" :class="statusLabel(row).dot"></span>{{ statusLabel(row).text }}
            </div>
            <div class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><Clock class="h-3 w-3" />{{ row.publiclyOrderable ? `响应约 ${row.responseMedianMinutes} 分钟` : row.warning ?? '暂不可接单' }}</div>
          </td>
          <td>
            <RouterLink v-if="row.publiclyOrderable" :to="`/api-market/${row.id}`"><Button class="api-market-intent-button" size="sm"><ShoppingBag class="h-4 w-4" />提交意向</Button></RouterLink>
            <Button v-else class="api-market-intent-button" size="sm" variant="outline" disabled><ShoppingBag class="h-4 w-4" />暂不可接单</Button>
          </td>
        </tr>
        <template #footer>
          <TablePagination
            v-model:page="sub2Pagination.page.value"
            :page-count="sub2Pagination.pageCount.value"
            :total="sub2Pagination.total.value"
            :start-item="sub2Pagination.startItem.value"
            :end-item="sub2Pagination.endItem.value"
          />
        </template>
      </SoftTable>
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
                <label class="grid gap-1 text-xs text-muted-foreground">接入方式
                  <select v-model="otherDeliveryMode" class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground">
                    <option value="all">全部</option>
                    <option value="api_key_endpoint">API 请求地址接入说明</option>
                    <option value="sub2api_panel_account">Sub2API 面板接入说明</option>
                  </select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">最低意向金额
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
            <option value="minimum_purchase_asc">最低意向金额</option>
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
      <SoftTable v-else :columns="['服务', '分发系统', '计费方式', '接入方式', '用量可见', '商户承诺', '最低意向', '商户', '状态', '操作']">
        <tr v-for="row in otherPagination.paginatedRows.value" :key="row.id" class="api-market-table-row">
          <td class="api-market-service-cell">
            <div class="font-semibold text-slate-900">{{ row.title }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ serviceSummary(row) }}</div>
          </td>
          <td>{{ row.delivery }}</td>
          <td>{{ billingModeLabel(row.billingMode) }}</td>
          <td>
            <div class="grid gap-2">
              <div class="flex flex-wrap gap-1.5">
                <span
                  v-for="(mode, index) in row.deliveryModes"
                  :key="mode"
                  class="c2c-api-delivery-pill inline-flex w-fit items-center whitespace-nowrap rounded-full border px-2 py-1 text-xs font-medium leading-none"
                  :class="deliveryModePillClass(index)"
                >
                  {{ deliveryModeColumnLabel(mode) }}
                </span>
              </div>
              <div class="flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
                <span class="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary"></span>
                <span>{{ deliveryModesHint(row.deliveryModes) }}</span>
              </div>
            </div>
          </td>
          <td>{{ getApiUsageVisibilityLabel(row.usageVisibility) }}</td>
          <td>{{ row.warranty }}</td>
          <td><span class="api-market-minimum">¥{{ row.minimumPurchaseCny }} 起</span></td>
          <td>
            <component :is="merchantProfileUrl(row) ? RouterLink : 'div'" :to="merchantProfileUrl(row) || undefined" class="api-market-merchant">
              <span class="api-market-avatar">{{ getApiMerchantAvatarText(row) }}</span>
              <span class="min-w-0">
                <span class="block font-medium">{{ getApiMerchantDisplayName(row) }}</span>
                <span class="mt-0.5 flex flex-wrap gap-1">
                  <Badge variant="identity">{{ merchantIdentity(row) }}</Badge>
                  <Badge variant="trust">信任等级{{ row.trustLevel }}</Badge>
                  <Badge v-if="!canOpenApiMerchantProfile(row)" variant="secondary">{{ getApiMerchantVisibilityLabel(row) }}</Badge>
                </span>
              </span>
            </component>
          </td>
          <td>
            <div class="flex items-center gap-1 text-sm" :class="statusLabel(row).textClass">
              <span class="h-2 w-2 rounded-full" :class="statusLabel(row).dot"></span>{{ statusLabel(row).text }}
            </div>
            <div class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><Clock class="h-3 w-3" />{{ row.publiclyOrderable ? `响应约 ${row.responseMedianMinutes} 分钟` : row.warning ?? '暂不可接单' }}</div>
          </td>
          <td>
            <RouterLink v-if="row.publiclyOrderable" :to="`/api-market/${row.id}`"><Button class="api-market-intent-button" size="sm"><ShoppingBag class="h-4 w-4" />提交意向</Button></RouterLink>
            <Button v-else class="api-market-intent-button" size="sm" variant="outline" disabled><ShoppingBag class="h-4 w-4" />暂不可接单</Button>
          </td>
        </tr>
        <template #footer>
          <TablePagination
            v-model:page="otherPagination.page.value"
            :page-count="otherPagination.pageCount.value"
            :total="otherPagination.total.value"
            :start-item="otherPagination.startItem.value"
            :end-item="otherPagination.endItem.value"
          />
        </template>
      </SoftTable>
    </section>
  </div>
</template>
