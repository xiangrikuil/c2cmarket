<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { CircleHelp, ExternalLink, Filter, Info, Package, Percent, RefreshCcw, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { shouldUseRealBackend } from '@/lib/backendClient'
import { getProductCategory } from '@/lib/productCategories'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import { useOfficialPrices } from '@/queries/useMarketQueries'
import type { OfficialPrice } from '@/lib/api'

type StatusFilter = '全部' | OfficialPrice['status']
type SortMode = 'updated_desc' | 'cny_asc' | 'trust_desc' | 'verified_recent' | 'submitted_desc'

const route = useRoute()
const router = useRouter()
const { data } = useOfficialPrices()

const q = ref(typeof route.query.q === 'string' ? route.query.q : '')
const product = ref(typeof route.query.product === 'string' ? route.query.product : '全部')
const region = ref(typeof route.query.region === 'string' ? route.query.region : '全部')
const channel = ref(typeof route.query.channel === 'string' ? route.query.channel : '全部')
const officialPriceStatuses: OfficialPrice['status'][] = ['已验证', '待验证', '需复核', '有争议', '已过期']
const routeStatus = typeof route.query.status === 'string' && officialPriceStatuses.includes(route.query.status as OfficialPrice['status']) ? route.query.status as OfficialPrice['status'] : '全部'
const status = ref<StatusFilter>(shouldUseRealBackend() && routeStatus !== '已验证' ? '全部' : routeStatus)
const plan = ref(typeof route.query.plan === 'string' ? route.query.plan : '全部')
const openingMethod = ref(typeof route.query.openingMethod === 'string' ? route.query.openingMethod : '全部')
const source = ref(typeof route.query.source === 'string' ? route.query.source : '全部')
const trust = ref(typeof route.query.trust === 'string' ? route.query.trust : '不限')
const defaultSort: SortMode = shouldUseRealBackend() ? 'cny_asc' : 'updated_desc'
const sort = ref<SortMode>(route.query.sort === 'cny_asc' || route.query.sort === 'trust_desc' || route.query.sort === 'verified_recent' || route.query.sort === 'submitted_desc' ? route.query.sort : defaultSort)

const statusOptions = computed<StatusFilter[]>(() => shouldUseRealBackend() ? ['全部', '已验证'] : ['全部', '已验证', '待验证'])
const detailedStatuses = computed<StatusFilter[]>(() => shouldUseRealBackend() ? ['全部', '已验证'] : ['全部', ...officialPriceStatuses])
const products = ['全部', 'ChatGPT', 'Claude', 'Cursor', 'Gemini', '其他']
const regions = ['全部', '菲律宾', '土耳其', '香港', '日本', '美国', '其他']
const channels = ['全部', 'Web', 'iOS App Store', 'Google Play', '其他']
const plans = ['全部', 'Plus', 'Pro', 'Business', '其他']
const openingMethods = ['全部', '本地卡', '礼品卡', '虚拟卡', 'Apple Store', 'Google Play', '其他']
const sources = ['全部', 'linux.do 原帖', '官方页面', '页面截图', '管理员录入', '其他']
const trustLevels = ['不限', '信任等级1+', '信任等级2+', '信任等级3+', '信任等级4']
const sortOptions: Array<{ label: string, value: SortMode }> = [
  { label: '人民币价格最低', value: 'cny_asc' },
  { label: '最近更新', value: 'updated_desc' },
  { label: '验证人数最多', value: 'trust_desc' },
  { label: '最近验证', value: 'verified_recent' },
  { label: '提交时间最新', value: 'submitted_desc' },
]
const pageDescription = shouldUseRealBackend()
  ? '按产品、地区、渠道和开通方式维护官网公开价；公开表仅展示审核通过的已验证价格记录。'
  : '按产品、地区、渠道和开通方式维护官网公开价格记录。'

watch(
  [q, product, region, channel, status, plan, openingMethod, source, trust, sort],
  () => {
    const query: Record<string, string> = {}
    if (q.value.trim()) query.q = q.value.trim()
    if (product.value !== '全部') query.product = product.value
    if (region.value !== '全部') query.region = region.value
    if (channel.value !== '全部') query.channel = channel.value
    if (status.value !== '全部') query.status = status.value
    if (plan.value !== '全部') query.plan = plan.value
    if (openingMethod.value !== '全部') query.openingMethod = openingMethod.value
    if (source.value !== '全部') query.source = source.value
    if (trust.value !== '不限') query.trust = trust.value
    if (sort.value !== defaultSort) query.sort = sort.value
    router.replace({ query })
  },
  { deep: false },
)

function normalizeChannel(value: string) {
  return value.replace('iOS App Store', 'iOS')
}

function trustFloor(value: string) {
  const match = value.match(/\d/)
  return match ? Number(match[0]) : 0
}

const rows = computed(() => {
  const keyword = q.value.trim().toLowerCase()
  const filtered = (data.value ?? []).filter(row => {
    const keywordMatched = !keyword || [
      row.product,
      row.plan,
      row.region,
      row.channel,
      row.submitter,
      row.source,
    ].some(value => value.toLowerCase().includes(keyword))

    return keywordMatched
      && (product.value === '全部' || row.product.includes(product.value))
      && (region.value === '全部' || row.region.includes(region.value))
      && (channel.value === '全部' || row.channel.includes(normalizeChannel(channel.value)))
      && (status.value === '全部' || row.status === status.value)
      && (plan.value === '全部' || row.plan.includes(plan.value))
      && (openingMethod.value === '全部' || row.openingMethod.includes(openingMethod.value.replace('Apple Store', 'Apple')))
      && (source.value === '全部' || row.source.includes(source.value.replace('linux.do 原帖', 'linux.do').replace('管理员录入', '管理员')))
      && (trust.value === '不限' || row.submitterTrust >= trustFloor(trust.value))
  })

  return [...filtered].sort((a, b) => {
    if (sort.value === 'cny_asc') return (a.cny ?? Number.POSITIVE_INFINITY) - (b.cny ?? Number.POSITIVE_INFINITY)
    if (sort.value === 'trust_desc') return b.submitterTrust - a.submitterTrust
    if (sort.value === 'verified_recent' || sort.value === 'submitted_desc') return a.updatedAt.localeCompare(b.updatedAt)
    return Number(b.isLowest) - Number(a.isLowest) || a.updatedAt.localeCompare(b.updatedAt)
  })
})

const pagination = usePagination(rows)

const contributorCount = computed(() => new Set((data.value ?? []).map(item => item.submitter)).size)

const advancedCount = computed(() => [plan.value !== '全部', openingMethod.value !== '全部', source.value !== '全部', trust.value !== '不限'].filter(Boolean).length)

const chips = computed(() => [
  q.value.trim() ? { key: 'q', label: q.value.trim(), reset: () => { q.value = '' } } : null,
  product.value !== '全部' ? { key: 'product', label: product.value, reset: () => { product.value = '全部' } } : null,
  region.value !== '全部' ? { key: 'region', label: region.value, reset: () => { region.value = '全部' } } : null,
  channel.value !== '全部' ? { key: 'channel', label: channel.value, reset: () => { channel.value = '全部' } } : null,
  status.value !== '全部' ? { key: 'status', label: status.value, reset: () => { status.value = '全部' } } : null,
  plan.value !== '全部' ? { key: 'plan', label: plan.value, reset: () => { plan.value = '全部' } } : null,
  openingMethod.value !== '全部' ? { key: 'openingMethod', label: openingMethod.value, reset: () => { openingMethod.value = '全部' } } : null,
  source.value !== '全部' ? { key: 'source', label: source.value, reset: () => { source.value = '全部' } } : null,
  trust.value !== '不限' ? { key: 'trust', label: trust.value, reset: () => { trust.value = '不限' } } : null,
].filter((item): item is { key: string, label: string, reset: () => void } => item !== null))

function clearAll() {
  q.value = ''
  product.value = '全部'
  region.value = '全部'
  channel.value = '全部'
  status.value = '全部'
  plan.value = '全部'
  openingMethod.value = '全部'
  source.value = '全部'
  trust.value = '不限'
}

function setStatus(value: StatusFilter) {
  status.value = value
}

const builtInProductIcons = new Map<string, string>()

function productIconSrc(productName: string) {
  return getProductCategoryIconSrc(getProductCategory(productName), builtInProductIcons)
}

function productToneClass(productName: string) {
  return `official-price-product-icon--${getProductCategory(productName)}`
}

function sourceLabel(value: string) {
  try {
    return new URL(value).hostname
  }
  catch {
    return value
  }
}

function openPrice(event: MouseEvent | KeyboardEvent, id: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button,input')) return
  router.push(`/official-prices/${id}`)
}
</script>

<template>
  <div class="official-price-page">
    <div class="official-price-reference-heading mb-4"><div class="flex items-center gap-3"><h1>官网价格</h1><Badge class="bg-violet-600 text-white">官方</Badge></div><p>{{ pageDescription }}</p></div>

    <div class="official-price-layout">
      <main class="min-w-0">
        <div class="official-price-category-tabs mb-4 flex flex-wrap gap-2">
          <Button v-for="item in products" :key="item" :variant="product === item ? 'default' : 'outline'" @click="product = item">{{ item }}</Button>
        </div>

        <div class="official-price-filter c2c-filterbar mb-4 rounded-lg border border-border bg-card px-3 py-2">
      <div class="official-price-filter-grid">
        <Input v-model="q" name="official-price-search" class="official-price-search h-8 bg-background text-sm" placeholder="搜索产品、地区、渠道或来源" />
        <label class="grid gap-1">
          <span class="text-[11px] font-medium leading-none text-muted-foreground">产品</span>
          <Select v-model="product">
            <SelectTrigger class="h-8 w-full bg-background text-xs"><SelectValue placeholder="全部产品" /></SelectTrigger>
            <SelectContent><SelectItem v-for="item in products" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
          </Select>
        </label>
        <label class="grid gap-1">
          <span class="text-[11px] font-medium leading-none text-muted-foreground">地区</span>
          <Select v-model="region">
            <SelectTrigger class="h-8 w-full bg-background text-xs"><SelectValue placeholder="全部地区" /></SelectTrigger>
            <SelectContent><SelectItem v-for="item in regions" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
          </Select>
        </label>
        <label class="grid gap-1">
          <span class="text-[11px] font-medium leading-none text-muted-foreground">渠道</span>
          <Select v-model="channel">
            <SelectTrigger class="h-8 w-full bg-background text-xs"><SelectValue placeholder="全部渠道" /></SelectTrigger>
            <SelectContent><SelectItem v-for="item in channels" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
          </Select>
        </label>
        <div class="flex min-w-[236px] flex-wrap self-end rounded-md border border-border bg-background p-1">
          <Button
            v-for="item in statusOptions"
            :key="item"
            class="h-7 flex-1 basis-[72px] px-2 text-xs"
            size="sm"
            :variant="status === item ? 'default' : 'ghost'"
            @click="setStatus(item)"
          >
            {{ item }}
          </Button>
        </div>
        <Popover>
          <PopoverTrigger as-child>
            <Button class="h-8 self-end" size="sm" variant="outline">
              <Filter class="h-4 w-4" />更多筛选<span v-if="advancedCount">· {{ advancedCount }}</span>
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-[380px]">
            <div class="grid gap-3">
              <div class="text-sm font-medium">更多筛选</div>
              <div class="grid grid-cols-2 gap-2">
                <label class="grid gap-1 text-xs text-muted-foreground">套餐
                  <Select v-model="plan">
                    <SelectTrigger class="h-8 bg-background text-xs text-foreground"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="item in plans" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
                  </Select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">开通方式
                  <Select v-model="openingMethod">
                    <SelectTrigger class="h-8 bg-background text-xs text-foreground"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="item in openingMethods" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
                  </Select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">详细状态
                  <Select v-model="status">
                    <SelectTrigger class="h-8 bg-background text-xs text-foreground"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="item in detailedStatuses" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
                  </Select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">记录来源
                  <Select v-model="source">
                    <SelectTrigger class="h-8 bg-background text-xs text-foreground"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="item in sources" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
                  </Select>
                </label>
                <label class="grid gap-1 text-xs text-muted-foreground">记录等级
                  <Select v-model="trust">
                    <SelectTrigger class="h-8 bg-background text-xs text-foreground"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="item in trustLevels" :key="item" :value="item">{{ item }}</SelectItem></SelectContent>
                  </Select>
                </label>
              </div>
            </div>
          </PopoverContent>
        </Popover>
        <label class="grid gap-1">
          <span class="text-[11px] font-medium leading-none text-muted-foreground">排序</span>
          <Select v-model="sort">
            <SelectTrigger class="h-8 w-full bg-background text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in sortOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
            </SelectContent>
          </Select>
        </label>
      </div>
      <div v-if="chips.length" class="mt-2 flex items-center gap-2 border-t border-border pt-2">
        <span class="shrink-0 text-xs text-muted-foreground">已选</span>
        <div class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto">
          <Badge v-for="chip in chips" :key="chip.key" variant="trust" class="cursor-pointer gap-1" @click="chip.reset">
            {{ chip.label }} <X class="h-3 w-3" />
          </Badge>
        </div>
        <Button class="h-7 shrink-0 px-2 text-xs" variant="ghost" size="sm" @click="clearAll">清除全部</Button>
        <span class="shrink-0 text-xs text-muted-foreground">共 {{ rows.length }} 条价格记录</span>
      </div>
      <div v-else class="mt-2 flex justify-end border-t border-border pt-2 text-xs text-muted-foreground">
        共 {{ rows.length }} 条价格记录
      </div>
        </div>

        <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {{ shouldUseRealBackend() ? '当前筛选条件下暂无已验证官网公开价格记录。' : '当前筛选条件下暂无官网公开价格记录。' }}
        </div>
        <SoftTable v-else class="official-price-table" :columns="['产品', '地区 / 渠道', '官网公开价', '折合人民币', '状态', '来源 / 维护', '更新时间']">
      <tr v-for="row in pagination.paginatedRows.value" :key="row.id" class="cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" tabindex="0" @click="openPrice($event, row.id)" @keydown.enter="openPrice($event, row.id)">
        <td><div class="flex items-center gap-3"><span class="official-price-product-icon" :class="productToneClass(row.product)"><img v-if="productIconSrc(row.product)" :src="productIconSrc(row.product)!" alt="" /><Package v-else class="h-4 w-4" /></span><div class="min-w-0"><div class="font-medium">{{ row.product }} {{ row.plan }}</div><div class="text-xs text-muted-foreground">{{ row.openingMethod }}</div></div></div></td>
        <td><div>{{ row.region }}</div><div class="text-xs text-muted-foreground">{{ row.channel }}</div></td>
        <td>{{ row.originalPrice }}</td>
        <td class="font-semibold">{{ row.cny === null ? '待换算' : `¥${row.cny}` }}</td>
        <td><Badge :variant="row.status === '已验证' ? 'default' : 'secondary'">{{ row.status }}</Badge></td>
        <td><RouterLink :to="`/official-prices/${row.id}`" class="inline-flex max-w-full items-center gap-1 truncate text-primary hover:underline"><ExternalLink class="h-3.5 w-3.5 shrink-0" /><span class="truncate">{{ sourceLabel(row.source) }}</span></RouterLink><div class="mt-1 truncate text-xs text-muted-foreground">{{ row.submitter }} · 记录等级 {{ row.submitterTrust }}</div></td>
        <td class="text-muted-foreground">{{ row.updatedAt }}</td>
      </tr>
      <template #footer>
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </template>
        </SoftTable>
      </main>

      <aside class="official-price-aside space-y-3">
        <Card class="official-price-aside-primary p-4">
          <div class="flex items-center gap-2 font-semibold"><Info class="h-4 w-4 text-violet-600" />价格说明</div>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
            <li>展示产品官网公开价格，仅作为方案比较参考。</li>
            <li>实际支付可能受地区、汇率、税费和渠道政策影响。</li>
            <li>官网价不等于本站车源或 API 服务成交价。</li>
          </ul>
        </Card>
        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><Percent class="h-4 w-4 text-blue-600" />税费与汇率</div>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">折合人民币用于横向比较，结账时仍以官网页面显示的币种、税费与实时汇率为准。</p>
        </Card>
        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><RefreshCcw class="h-4 w-4 text-emerald-600" />维护状态</div>
          <dl class="mt-3 grid gap-2 text-sm"><div class="flex justify-between"><dt class="text-muted-foreground">公开记录</dt><dd class="font-semibold">{{ (data ?? []).length }}</dd></div><div class="flex justify-between"><dt class="text-muted-foreground">维护来源</dt><dd class="font-semibold">{{ contributorCount }}</dd></div><div class="flex justify-between"><dt class="text-muted-foreground">数据来源</dt><dd class="font-semibold text-emerald-700">产品官网</dd></div></dl>
        </Card>
        <Card class="official-price-help p-4">
          <div class="flex items-center gap-2 font-semibold"><CircleHelp class="h-4 w-4 text-primary" />需要帮助选择？</div>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">先比较官网公开价，再结合车源开通方式和风险说明判断是否适合。</p>
          <RouterLink to="/carpools" class="mt-3 block"><Button class="w-full" variant="outline">浏览订阅车源</Button></RouterLink>
        </Card>
      </aside>
    </div>
  </div>
</template>
