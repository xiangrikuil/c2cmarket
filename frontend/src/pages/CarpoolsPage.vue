<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronDown, Code2, Globe2, MessageCircle, PackageSearch, Search, ShieldCheck, SlidersHorizontal, Sparkles, UsersRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import FilterBar from '@/components/market/FilterBar.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { getProductCategoryIconSrc, getProductIconSrc as getCatalogProductIconSrc } from '@/lib/productCategoryIcon'
import { useCarpools, useMyProfileQuery } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'
import { compareByTradablePrice, getPricingDisplay } from '@/lib/pricing'
import { formatWeeklyMonthlyQuota } from '@/lib/quota'
import {
  allProductPlanValue,
  getProductCategory,
  getProductCategoryLabel,
  getProductPlanOptions,
  isHighRiskGptCarpoolPlan,
  normalizeProductCategory,
  normalizeProductPlan,
  productCategoryOptions,
  productMatchesCategory,
  productMatchesPlan,
  type ProductCategoryKey,
} from '@/lib/productCategories'
import { adminAccountLabel, distributionMethodLabel, openingChannelLabels, paymentMethodLabels } from '@/components/carpool-publish/utils'
import type { Carpool } from '@/lib/api'

const filters = [
  { label: '开通区', items: ['全部', '菲律宾区', '日本区', '土耳其区', '香港区'], active: '全部' },
  { label: '车主类型', items: ['全部', '个人车主', '可信新车主', '商户车源'], active: '全部' },
  { label: '车主承诺', items: ['全部', '车主承诺', '售后协商'], active: '全部' },
  { label: '开通方式', items: ['全部', 'Apple Store', '虚拟卡', '本地卡', '信用卡'], active: '全部' },
  { label: '排序', items: ['推荐', '最近确认', '低于官方', '最低月费', '剩余名额'], active: '推荐' },
]

const route = useRoute()
const router = useRouter()
const selected = ref(Object.fromEntries(filters.map(group => [group.label, group.active ?? group.items[0]])))
const carpoolsQuery = useCarpools()
const productCategoriesQuery = useProductCategories()
const { data } = carpoolsQuery
const { data: myProfile } = useMyProfileQuery(import.meta.client)
const { data: catalogCategories } = productCategoriesQuery
prefetchQueriesOnServer(carpoolsQuery, productCategoriesQuery)
const canModerateCarpools = computed(() => myProfile.value?.permissions.includes('admin') ?? false)
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
const selectedCategory = ref<ProductCategoryKey>(normalizeProductCategory(route.query.category))
const selectedPlan = ref(normalizeProductPlan(selectedCategory.value, route.query.plan))

watch(
  () => route.query,
  query => {
    const category = normalizeProductCategory(query.category)
    selectedCategory.value = category
    selectedPlan.value = normalizeProductPlan(category, query.plan)
  },
)

watch([selectedCategory, selectedPlan], ([category, plan]) => {
  const normalizedPlan = normalizeProductPlan(category, plan)
  if (normalizedPlan !== plan) {
    selectedPlan.value = normalizedPlan
    return
  }
  if (route.query.category === category && (route.query.plan ?? allProductPlanValue) === normalizedPlan) return
  router.replace({
    query: {
      ...route.query,
      category,
      plan: normalizedPlan === allProductPlanValue ? undefined : normalizedPlan,
    },
  })
}, { immediate: true })

const planOptions = computed(() => getProductPlanOptions(selectedCategory.value))
const selectedPlanMeta = computed(() => selectedPlan.value === allProductPlanValue ? null : planOptions.value.find(item => item.slug === selectedPlan.value) ?? null)

function selectCategory(category: ProductCategoryKey) {
  selectedCategory.value = category
  selectedPlan.value = allProductPlanValue
}

const rows = computed(() => {
  const filtered = (data.value ?? []).filter(row => {
    return productMatchesCategory(row.product, selectedCategory.value)
      && productMatchesPlan(row.product, selectedPlan.value)
      && (selected.value['开通区'] === '全部' || row.region === selected.value['开通区'])
      && (selected.value['车主类型'] === '全部' || row.ownerType === selected.value['车主类型'])
      && (selected.value['车主承诺'] === '全部' || row.warranty === selected.value['车主承诺'])
      && (selected.value['开通方式'] === '全部' || row.openingMethod === selected.value['开通方式'])
  })

  return [...filtered].sort((a, b) => {
    if (selected.value['排序'] === '最低月费') return compareByTradablePrice(a, b)
    if (selected.value['排序'] === '最近确认') return a.confirmedAt.localeCompare(b.confirmedAt)
    if (selected.value['排序'] === '剩余名额') return availableSeatsForList(b) - availableSeatsForList(a)
    return Number(b.confirmedWithin48h) - Number(a.confirmedWithin48h)
      || Number(a.ownerType !== '商户车源') - Number(b.ownerType !== '商户车源')
      || compareByTradablePrice(a, b)
  })
})

const pagination = usePagination(rows)

const availableCount = computed(() => rows.value.filter(row => listStatusForCarpool(row) === '可上车').length)
const recentlyConfirmedCount = computed(() => rows.value.filter(row => row.confirmedWithin48h).length)
const boundaryConfirmationCount = computed(() => rows.value.filter(row => isHighRiskGptCarpoolPlan(row.product)).length)
const selectedCategoryLabel = computed(() => getProductCategoryLabel(selectedCategory.value))
const activeFilterCount = computed(() => {
  const selectedFilterCount = filters.filter(group => selected.value[group.label] !== group.active).length
  return selectedFilterCount
    + Number(selectedCategory.value !== 'all')
    + Number(selectedPlan.value !== allProductPlanValue)
})
const categoryNotice = computed(() => {
  if (selectedCategory.value === 'gpt') {
    return 'GPT 分类会包含 Business、Plus、Pro 5x Web、Pro 20x Web；部分套餐申请前需要确认发布和使用边界。'
  }
  return '筛选结果优先展示近期确认、无未解决纠纷的车源；加入前请查看车源详情与站外确认要求。'
})

function openingChannelLabel(row: Pick<Carpool, 'openingChannelCode' | 'customOpeningChannel'>) {
  if (row.openingChannelCode === 'other') return row.customOpeningChannel?.trim() || '未声明'
  return row.openingChannelCode ? openingChannelLabels[row.openingChannelCode] : '未声明'
}

function paymentMethodLabel(row: Pick<Carpool, 'paymentMethodCode' | 'customPaymentMethod'>) {
  if (row.paymentMethodCode === 'other') return row.customPaymentMethod?.trim() || '未声明'
  return row.paymentMethodCode ? paymentMethodLabels[row.paymentMethodCode] : '未声明'
}

function quotaResetLabel(value: boolean | null | undefined) {
  if (value === true) return '跟随官方重置'
  if (value === false) return '不跟随官方重置'
  return '未声明'
}

function mainlandDirectLabel(value: boolean | null | undefined) {
  if (value === true) return '支持国内直连'
  if (value === false) return '不支持国内直连'
  return '未声明'
}

function accessMethodCount(row: Pick<Carpool, 'openingChannelCode' | 'distributionMethod'>) {
  return Number(Boolean(row.openingChannelCode)) + Number(Boolean(row.distributionMethod))
}

type CarpoolListSeatRow = {
  status: string
  currentConfirmedMembers: number
  maxMembers: number
  seatSummary?: {
    totalSeats: number
    activeMemberCount: number
    reservedSeatCount: number
    availableSeats: number
  }
  applicationEligibility?: { code: string, canApply: boolean, reason: string }
}

function activeSeatsForList(row: CarpoolListSeatRow) {
  return row.seatSummary?.activeMemberCount ?? row.currentConfirmedMembers
}

function reservedSeatsForList(row: CarpoolListSeatRow) {
  return row.seatSummary?.reservedSeatCount ?? 0
}

function availableSeatsForList(row: CarpoolListSeatRow) {
  return row.seatSummary?.availableSeats ?? Math.max(row.maxMembers - activeSeatsForList(row) - reservedSeatsForList(row), 0)
}

function totalSeatsForList(row: CarpoolListSeatRow) {
  return row.seatSummary?.totalSeats ?? row.maxMembers
}

function listStatusForCarpool(row: CarpoolListSeatRow) {
  if (row.applicationEligibility && !row.applicationEligibility.canApply) {
    if (row.applicationEligibility.code === 'credential_risk' || row.applicationEligibility.code === 'owner_action_required') return '需车主修正'
    if (row.applicationEligibility.code === 'sold_out') return reservedSeatsForList(row) > 0 ? '预留中' : '已满'
    return row.applicationEligibility.reason
  }
  if (!['可上车', '已满'].includes(row.status)) return row.status
  if (availableSeatsForList(row) > 0) return '可上车'
  if (reservedSeatsForList(row) > 0) return '预留中'
  return '已满'
}

function categoryIconSrc(category: ProductCategoryKey) {
  return getProductCategoryIconSrc(category, categoryIconByCode.value)
}

function categoryIconComponent(category: ProductCategoryKey) {
  if (category === 'cursor') return Code2
  if (category === 'perplexity') return Search
  if (category === 'other') return PackageSearch
  return Sparkles
}

function productIconSrc(product: string) {
  return getCatalogProductIconSrc(product, categoryIconByCode.value)
}

function productIconComponent(product: string) {
  return categoryIconComponent(getProductCategory(product))
}

function categoryIconAlt(category: ProductCategoryKey) {
  return `${getProductCategoryLabel(category)} 图标`
}

function productToneClass(product: string) {
  return `carpool-product-avatar--${getProductCategory(product)}`
}

function statusToneClass(status: string) {
  if (status === '可上车') return 'carpool-status-badge--available'
  if (status === '预留中') return 'carpool-status-badge--reserved'
  if (status === '候补') return 'carpool-status-badge--waitlist'
  if (status === '审核中') return 'carpool-status-badge--reviewing'
  if (status === '已满') return 'carpool-status-badge--full'
  return 'carpool-status-badge--paused'
}

function seatProgress(row: CarpoolListSeatRow) {
  const occupiedSeats = activeSeatsForList(row) + reservedSeatsForList(row)
  return `${Math.min(Math.round((occupiedSeats / Math.max(totalSeatsForList(row), 1)) * 100), 100)}%`
}

function ownerInitial(owner: string) {
  const normalized = owner.replace(/^用户\s*/, '')
  if (/^[0-9a-f]/i.test(normalized)) return '车'
  return normalized.slice(0, 1).toUpperCase()
}

function openCarpool(event: MouseEvent | KeyboardEvent, id: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button,input,select')) return
  router.push(`/carpools/${id}`)
}
</script>

<template>
  <div class="carpool-page">
    <section class="carpool-reference-top mb-4">
      <div class="carpool-reference-main">
        <div class="carpool-reference-heading">
          <div class="text-xs text-muted-foreground">发现市场　/　订阅拼车</div>
          <h1>订阅拼车</h1>
          <p>月付订阅的共享席位，默认无押金。请仔细确认账号类型、额度接入信息与一次申请的联系和确认流程。</p>
        </div>
        <div class="carpool-reference-stats">
          <div><span><UsersRound /></span><dl><dt>可上车</dt><dd>{{ availableCount }}</dd><small>可立即加入</small></dl></div>
          <div><span><ShieldCheck /></span><dl><dt>近期确认</dt><dd>{{ recentlyConfirmedCount }}</dd><small>48 小时内确认</small></dl></div>
          <div><span><MessageCircle /></span><dl><dt>边界确认</dt><dd>{{ boundaryConfirmationCount }}</dd><small>已明确规则</small></dl></div>
          <div><span><SlidersHorizontal /></span><dl><dt>当前筛选</dt><dd>{{ activeFilterCount }}</dd><small>已应用筛选</small></dl></div>
        </div>
      </div>
      <aside class="carpool-reference-note">
        <div class="flex items-center gap-2 font-semibold text-primary"><ShieldCheck class="h-5 w-5" />关于当前筛选</div>
        <p>{{ categoryNotice }}</p>
        <div class="mt-3 text-xs font-semibold text-primary">推荐优先选择近期确认且使用条件完整的车源。</div>
      </aside>
    </section>

    <div>
      <main class="min-w-0">
      <section class="carpool-catalog-panel mb-4 rounded-lg border border-border bg-card px-4 py-4">
      <div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
            <Sparkles class="h-4 w-4 text-primary" />
            产品分类
            <Badge variant="status" class="ml-1">当前：{{ selectedCategoryLabel }}</Badge>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <Button
              v-for="category in productCategoryOptions"
              :key="category.key"
              class="carpool-category-button h-8 shrink-0 px-3 text-xs"
              size="sm"
              :variant="selectedCategory === category.key ? 'default' : 'outline'"
              @click="selectCategory(category.key)"
            >
              <span class="carpool-category-icon" aria-hidden="true">
                <img v-if="categoryIconSrc(category.key)" :src="categoryIconSrc(category.key)!" :alt="categoryIconAlt(category.key)" />
                <component :is="categoryIconComponent(category.key)" v-else class="h-3.5 w-3.5" />
              </span>
              {{ category.label }}
            </Button>
          </div>
        </div>
      </div>

      <div v-if="planOptions.length" class="mt-4 border-t border-border pt-4">
        <div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
          具体套餐
          <span v-if="selectedPlanMeta" class="font-normal">· {{ selectedPlanMeta.note }}</span>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <Button
            class="carpool-plan-button h-8 shrink-0 px-3 text-xs"
            size="sm"
            :variant="selectedPlan === allProductPlanValue ? 'secondary' : 'ghost'"
            @click="selectedPlan = allProductPlanValue"
          >
            全部{{ productCategoryOptions.find(item => item.key === selectedCategory)?.label }}
          </Button>
          <Button
            v-for="plan in planOptions"
            :key="plan.slug"
            class="carpool-plan-button h-8 shrink-0 px-3 text-xs"
            size="sm"
            :variant="selectedPlan === plan.slug ? 'secondary' : 'ghost'"
            @click="selectedPlan = plan.slug"
          >
            {{ plan.label }}
          </Button>
        </div>
      </div>
      <div class="mt-4 border-t border-border pt-4">
        <FilterBar v-model="selected" :groups="filters" :result-count="rows.length" />
      </div>
    </section>
    <Alert v-if="canModerateCarpools" class="mb-4">
      <ShieldCheck />
      <AlertTitle>管理员巡查模式</AlertTitle>
      <AlertDescription>当前列表就是公开车源巡查入口。打开任意车源详情可执行下架或要求复核；暂停和遗留审核记录请前往车源异常处理。</AlertDescription>
    </Alert>
    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无可展示车源。</div>
    <SoftTable v-else :columns="['车源', '价格', '车位', '额度 / 接入', '车主', '状态']">
      <tr
        v-for="row in pagination.paginatedRows.value"
        :key="row.id"
        class="carpool-table-row cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        tabindex="0"
        @click="openCarpool($event, row.id)"
        @keydown.enter="openCarpool($event, row.id)"
      >
        <td class="carpool-source-cell">
          <div class="flex min-w-0 items-start gap-3">
            <div :class="['carpool-product-avatar', productToneClass(row.product)]">
              <img v-if="productIconSrc(row.product)" :src="productIconSrc(row.product)!" :alt="`${row.product} 图标`" />
              <component :is="productIconComponent(row.product)" v-else class="h-4 w-4" />
            </div>
            <div class="min-w-0">
              <div class="truncate font-semibold text-slate-900">{{ row.product }}</div>
              <div class="mt-1 text-xs text-muted-foreground">{{ row.region }}</div>
            </div>
          </div>
        </td>
        <td>
          <div class="text-[15px] font-semibold text-slate-950">¥{{ getPricingDisplay(row).primaryPrice }}/月</div>
        </td>
        <td>
          <div class="flex items-center justify-between gap-2 text-sm">
            <span class="font-medium">已上车 {{ activeSeatsForList(row) }}/{{ totalSeatsForList(row) }} 人</span>
            <span class="text-xs text-muted-foreground">可申请 {{ availableSeatsForList(row) }} 位</span>
          </div>
          <div v-if="reservedSeatsForList(row)" class="mt-1 text-xs text-muted-foreground">预留 {{ reservedSeatsForList(row) }} 位</div>
          <div class="carpool-seat-meter mt-2" aria-hidden="true">
            <span :style="{ width: seatProgress(row) }"></span>
          </div>
        </td>
        <td>
          <div class="whitespace-nowrap text-sm font-semibold text-slate-900">{{ formatWeeklyMonthlyQuota(row) }}</div>
          <div class="mt-2 flex flex-wrap items-center gap-1">
            <Badge variant="capability">{{ quotaResetLabel(row.followsOfficialQuotaReset) }}</Badge>
            <Badge variant="capability">{{ adminAccountLabel(row.providesAdminAccount) }}</Badge>
            <Popover>
              <PopoverTrigger as-child>
                <Button
                  variant="ghost"
                  size="sm"
                  class="h-5 gap-1 px-1.5 text-xs text-muted-foreground"
                  :aria-label="`查看 ${row.product} 的接入详情`"
                  @click.stop
                  @keydown.stop
                >
                  <Globe2 class="h-3.5 w-3.5" />
                  {{ accessMethodCount(row) || '未声明' }}{{ accessMethodCount(row) ? ' 种接入' : '' }}
                  <ChevronDown class="h-3 w-3" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-80" align="start" @click.stop>
                <div class="space-y-3 text-sm">
                  <div class="font-semibold">额度与接入详情</div>
                  <dl class="grid grid-cols-[88px_minmax(0,1fr)] gap-x-3 gap-y-2">
                    <dt class="text-muted-foreground">开通渠道</dt><dd>{{ openingChannelLabel(row) }}</dd>
                    <dt class="text-muted-foreground">付款方式</dt><dd>{{ paymentMethodLabel(row) }}</dd>
                    <dt class="text-muted-foreground">分发方式</dt><dd>{{ distributionMethodLabel(row.distributionMethod) }}</dd>
                    <dt class="text-muted-foreground">VPS 区域</dt><dd>{{ row.vpsRegion?.trim() || '未声明' }}</dd>
                    <dt class="text-muted-foreground">国内直连</dt><dd>{{ mainlandDirectLabel(row.supportsMainlandChinaDirectConnection) }}</dd>
                  </dl>
                  <p class="border-t border-border pt-3 text-xs leading-5 text-muted-foreground">具体权限与使用细节请站外确认，平台不保存管理员凭据。</p>
                </div>
              </PopoverContent>
            </Popover>
          </div>
        </td>
        <td>
          <div class="flex min-w-0 items-center gap-2">
            <span class="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">{{ ownerInitial(row.owner) }}</span>
            <div class="min-w-0">
              <div class="flex min-w-0 items-center gap-1.5">
                <span class="truncate font-medium text-slate-900">{{ row.owner }}</span>
                <Badge class="shrink-0" variant="secondary">{{ row.ownerType }}</Badge>
              </div>
              <ReputationInlineSummary class="mt-1" :summary="row.sellerReputation" :compact="true" />
            </div>
          </div>
        </td>
        <td>
          <Badge :class="['carpool-status-badge', statusToneClass(listStatusForCarpool(row))]">{{ listStatusForCarpool(row) }}</Badge>
        </td>
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
    </div>
  </div>
</template>
