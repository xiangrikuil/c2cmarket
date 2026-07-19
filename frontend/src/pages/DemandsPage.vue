<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { ArrowRight, CircleHelp, MessagesSquare, Plus, Search, ShieldCheck, Sparkles } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import FilterBar from '@/components/market/FilterBar.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { usePagination } from '@/composables/usePagination'
import { useCarpoolProductCatalog, useCarpools, useDemands } from '@/queries/useMarketQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

const router = useRouter()
const productCatalogQuery = useCarpoolProductCatalog()
const carpoolsQuery = useCarpools()
const demandsQuery = useDemands()
const { data: productCatalog } = productCatalogQuery
const { data: carpools } = carpoolsQuery
const { data, isLoading } = demandsQuery
prefetchQueriesOnServer(productCatalogQuery, carpoolsQuery, demandsQuery)
const query = ref('')
const selected = ref({ 产品: '全部', 状态: '全部' })

const productLabels = computed(() => (productCatalog.value ?? [])
  .filter(item => item.active && item.publishPolicy === 'allowed')
  .map(item => item.displayName))
const filters = computed(() => [
  { label: '产品', items: ['全部', ...productLabels.value], active: '全部' },
  { label: '状态', items: ['全部', '匹配中', '已匹配', '已关闭'], active: '全部' },
])

function productMatches(title: string, filter: string) {
  const normalizedTitle = title.replace(/^求\s*/, '').toLowerCase()
  const normalizedFilter = filter.toLowerCase()
  return normalizedTitle.includes(normalizedFilter) || normalizedFilter.includes(normalizedTitle)
}

const rows = computed(() => (data.value ?? []).filter(row => {
  const keyword = query.value.trim().toLowerCase()
  const productFilter = selected.value['产品']
  return (!keyword || [row.title, row.region, row.poster, row.require].join(' ').toLowerCase().includes(keyword))
    && (productFilter === '全部' || productMatches(row.title, productFilter))
    && (selected.value['状态'] === '全部' || row.status === selected.value['状态'])
}))

const recommendedCarpools = computed(() => (carpools.value ?? [])
  .filter(item => item.status === '可上车')
  .slice(0, 3))
const pagination = usePagination(rows)

function ownerPreferenceLabel(value: string) {
  if (value === 'only-personal' || value === 'only_personal') return '只看个人车主'
  if (value === 'any') return '不限车主类型'
  return '个人车主优先'
}

function openDemand(event: MouseEvent | KeyboardEvent, id: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button,input,select')) return
  router.push(`/demands/${id}`)
}
</script>

<template>
  <div class="demand-market-page space-y-4">
    <div class="demand-reference-heading">
      <PageTitle title="求车大厅" description="浏览真实求车需求，按预算、地区和车主偏好判断是否适合用现有车源回应。">
        <template #action>
          <RouterLink to="/demands/new"><Button><Plus class="h-4 w-4" />发布求车</Button></RouterLink>
        </template>
      </PageTitle>
    </div>

    <section class="demand-process-strip" aria-label="求车匹配流程">
      <div class="demand-process-copy">
        <span class="demand-process-kicker">发布需求 · 快速匹配</span>
        <p>写清套餐、预算和地区，车主会使用已发布车源回应。</p>
      </div>
      <div class="demand-process-step"><span><Plus class="h-4 w-4" /></span><div><strong>发布需求</strong><small>描述真实条件</small></div></div>
      <ArrowRight class="demand-process-arrow" />
      <div class="demand-process-step"><span><Sparkles class="h-4 w-4" /></span><div><strong>获得回应</strong><small>查看匹配车源</small></div></div>
      <ArrowRight class="demand-process-arrow" />
      <div class="demand-process-step"><span><MessagesSquare class="h-4 w-4" /></span><div><strong>继续申请</strong><small>按车源流程确认</small></div></div>
    </section>

    <div class="demand-market-layout">
      <main class="min-w-0 space-y-4">
        <div class="demand-market-filter sticky top-14 z-20 space-y-3 rounded-xl border border-border bg-background/95 p-3 backdrop-blur">
          <label class="relative block max-w-xl">
            <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="query" class="pl-9" placeholder="搜索套餐、地区、发布者或需求说明" />
          </label>
          <FilterBar v-model="selected" :groups="filters" :result-count="rows.length" />
        </div>

        <SkeletonTable v-if="isLoading" :rows="5" />
        <EmptyState v-else-if="rows.length === 0" title="暂无匹配的求车需求" description="可以调整筛选，或发布你的求车需求等待车主回应。">
          <template #action><RouterLink to="/demands/new"><Button>发布求车</Button></RouterLink></template>
        </EmptyState>
        <SoftTable v-else class="demand-market-table" :columns="['需求', '预算', '地区', '车主偏好', '发布者信誉', '状态', '更新时间']">
          <tr
            v-for="row in pagination.paginatedRows.value"
            :key="row.id"
            class="cursor-pointer transition hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            tabindex="0"
            @click="openDemand($event, row.id)"
            @keydown.enter="openDemand($event, row.id)"
          >
            <td><div class="flex items-start gap-3"><span class="demand-row-icon"><Search class="h-4 w-4" /></span><div class="min-w-0"><div class="font-medium">{{ row.title }}</div><div class="mt-1 max-w-md truncate text-xs text-muted-foreground">{{ row.require }}</div></div></div></td>
            <td class="font-semibold">¥{{ row.maxPrice }}/月</td>
            <td>{{ row.region }}</td>
            <td>{{ ownerPreferenceLabel(row.ownerPreference) }}</td>
            <td>{{ row.poster }}<div class="mt-1 text-xs text-muted-foreground">信任等级 {{ row.trustLevel }}</div></td>
            <td><Badge variant="secondary">{{ row.status }}</Badge></td>
            <td class="text-muted-foreground">{{ row.updatedAt }}</td>
          </tr>
          <template #footer>
            <TablePagination v-model:page="pagination.page.value" :page-count="pagination.pageCount.value" :total="pagination.total.value" :start-item="pagination.startItem.value" :end-item="pagination.endItem.value" />
          </template>
        </SoftTable>
      </main>

      <aside class="demand-market-aside space-y-3">
        <Card class="demand-market-cta p-4">
          <span class="demand-market-aside-icon"><Plus class="h-5 w-5" /></span>
          <h2 class="mt-3 font-semibold text-slate-950">没有合适的车源？</h2>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">写清套餐、地区、预算与车主偏好，让车主带着现有车源回应。</p>
          <RouterLink to="/demands/new" class="mt-4 block"><Button class="w-full">发布求车需求<ArrowRight class="h-4 w-4" /></Button></RouterLink>
        </Card>
        <Card class="p-4">
          <div class="flex items-center justify-between gap-2 font-semibold"><span class="flex items-center gap-2"><Sparkles class="h-4 w-4 text-primary" />匹配建议</span><RouterLink to="/carpools" class="text-xs text-primary">查看更多</RouterLink></div>
          <div v-if="recommendedCarpools.length" class="mt-3 divide-y divide-border">
            <RouterLink v-for="item in recommendedCarpools" :key="item.id" :to="`/carpools/${item.id}`" class="block py-2 first:pt-0 last:pb-0"><div class="truncate text-sm font-medium">{{ item.product }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.region }} · {{ item.ownerType }}</div></RouterLink>
          </div>
          <p v-else class="mt-3 text-sm text-muted-foreground">当前暂无可推荐车源。</p>
        </Card>
        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><CircleHelp class="h-4 w-4 text-primary" />平台边界</div>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">平台提供信息展示、申请记录与纠纷入口，不托管订阅款，也不保证站外履约结果。</p>
        </Card>
      </aside>
    </div>
  </div>
</template>
