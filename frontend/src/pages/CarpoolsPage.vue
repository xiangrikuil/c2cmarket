<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowUpRight, Code2, PackageSearch, Search, ShieldCheck, Sparkles, Upload, UsersRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import FilterBar from '@/components/market/FilterBar.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import SourceBadges from '@/components/market/SourceBadges.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { useCarpools } from '@/queries/useMarketQueries'
import { compareByTradablePrice, getPricingDisplay } from '@/lib/pricing'
import { formatMonthlyQuota } from '@/lib/quota'
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
import { adminAccountLabel, distributionMethodLabel } from '@/components/carpool-publish/utils'

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
const { data } = useCarpools()
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
    return Number(b.linuxdoBound) - Number(a.linuxdoBound)
      || b.trustLevel - a.trustLevel
      || Number(a.ownerType !== '商户车源') - Number(b.ownerType !== '商户车源')
      || compareByTradablePrice(a, b)
  })
})

const pagination = usePagination(rows)

const availableCount = computed(() => rows.value.filter(row => listStatusForCarpool(row) === '可上车').length)
const linuxdoBoundCount = computed(() => rows.value.filter(row => row.linuxdoBound).length)
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
  return '筛选结果优先展示原帖已绑定、近期确认、无未解决纠纷的车源；加入前请查看车源详情与站外确认要求。'
})

function carpoolSourceBadges(row: { linuxdoBound: boolean, monthlyQuotaAmount?: number, quotaLabel?: string, quotaUnit?: string, quotaPeriod?: string }) {
  const badges: string[] = []
  if (row.linuxdoBound) badges.push('原帖已绑定')
  if (row.monthlyQuotaAmount) badges.push(formatMonthlyQuota(row))
  return badges
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
  if (!['可上车', '已满'].includes(row.status)) return row.status
  if (availableSeatsForList(row) > 0) return '可上车'
  if (reservedSeatsForList(row) > 0) return '预留中'
  return '已满'
}

function categoryIconSrc(category: ProductCategoryKey) {
  if (category === 'gpt') return '/chatgpt-mark.svg'
  if (category === 'claude') return '/claude-mark.svg'
  if (category === 'gemini') return '/gemini-mark.svg'
  return null
}

function categoryIconComponent(category: ProductCategoryKey) {
  if (category === 'cursor') return Code2
  if (category === 'perplexity') return Search
  if (category === 'other') return PackageSearch
  return Sparkles
}

function productIconSrc(product: string) {
  return categoryIconSrc(getProductCategory(product))
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
</script>

<template>
  <div class="carpool-page">
    <section class="carpool-hero mb-4">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <div class="carpool-kicker">
            <UsersRound class="h-4 w-4" />
            <span>订阅拼车</span>
          </div>
          <h1 class="mt-2 text-[32px] font-semibold leading-tight tracking-normal text-slate-950 md:text-[36px]">订阅拼车</h1>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
            默认月付、无押金。优先展示个人车主、原帖已绑定、近期确认、无未解决纠纷的车源。
          </p>
        </div>
        <RouterLink to="/carpools/new" class="w-full lg:w-auto">
          <Button class="carpool-primary-action w-full lg:w-auto">
            <Upload class="h-4 w-4" />
            导入 / 发布车源
          </Button>
        </RouterLink>
      </div>

      <div class="carpool-hero-metrics mt-4">
        <div>
          <span>可上车</span>
          <strong>{{ availableCount }}</strong>
        </div>
        <div>
          <span>原帖已绑定</span>
          <strong>{{ linuxdoBoundCount }}</strong>
        </div>
        <div>
          <span>边界确认</span>
          <strong>{{ boundaryConfirmationCount }}</strong>
        </div>
        <div>
          <span>当前筛选</span>
          <strong>{{ activeFilterCount }}</strong>
        </div>
      </div>
    </section>

    <section class="carpool-catalog-panel mb-4 rounded-lg border border-border bg-card px-4 py-4">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
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
        <div class="carpool-risk-note">
          <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-primary" />
          <span>{{ categoryNotice }}</span>
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
    </section>

    <FilterBar v-model="selected" :groups="filters" :result-count="rows.length" />
    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无可展示车源。</div>
    <SoftTable v-else :columns="['车源', '价格', '车位', '开通信息', '车主', '状态', '操作']">
      <tr v-for="row in pagination.paginatedRows.value" :key="row.id" class="carpool-table-row">
        <td class="carpool-source-cell">
          <div class="flex min-w-0 items-start gap-3">
            <div :class="['carpool-product-avatar', productToneClass(row.product)]">
              <img v-if="productIconSrc(row.product)" :src="productIconSrc(row.product)!" :alt="`${row.product} 图标`" />
              <component :is="productIconComponent(row.product)" v-else class="h-4 w-4" />
            </div>
            <div class="min-w-0">
              <div class="truncate font-semibold text-slate-900">{{ row.product }}</div>
              <div class="mt-1 text-xs text-muted-foreground">{{ row.region }}</div>
              <SourceBadges
                v-if="carpoolSourceBadges(row).length"
                class="mt-2"
                :badges="carpoolSourceBadges(row)"
              />
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
          <div class="font-medium text-slate-900">{{ row.openingMethod }}</div>
          <div class="mt-1 flex flex-wrap gap-1">
            <Badge variant="capability">{{ distributionMethodLabel(row.distributionMethod) }}</Badge>
            <Badge variant="capability">{{ adminAccountLabel(row.providesAdminAccount) }}</Badge>
          </div>
          <div class="mt-1 text-xs text-muted-foreground">{{ row.region }} · {{ row.warranty }}</div>
          <div v-if="row.monthlyQuotaAmount" class="mt-1 text-xs text-muted-foreground">
            {{ formatMonthlyQuota(row) }}
          </div>
        </td>
        <td>
          <div class="flex min-w-0 items-center gap-2">
            <span class="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">{{ ownerInitial(row.owner) }}</span>
            <div class="min-w-0">
              <div class="truncate font-medium text-slate-900">{{ row.owner }}</div>
              <SourceBadges class="mt-1" :trust="row.trustLevel" :owner-type="row.ownerType" />
            </div>
          </div>
        </td>
        <td>
          <Badge :class="['carpool-status-badge', statusToneClass(listStatusForCarpool(row))]">{{ listStatusForCarpool(row) }}</Badge>
        </td>
        <td>
          <RouterLink :to="`/carpools/${row.id}`">
            <Button class="carpool-view-button" size="sm" variant="outline">查看 <ArrowUpRight class="h-4 w-4" /></Button>
          </RouterLink>
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
  </div>
</template>
