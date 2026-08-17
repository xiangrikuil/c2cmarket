<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, CalendarClock, Clock3, Package, PlayCircle, UserRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getCarpoolApplicationNextAction, getCarpoolApplicationStatusLabel, type CarpoolApplication } from '@/lib/api'
import { getProductCategory } from '@/lib/productCategories'
import { functionalMotion } from '@/lib/motion'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import { useMyCarpoolApplications, useMyCarpoolApplicationsPage } from '@/queries/useMarketQueries'

const activeStatus = ref('全部')
const router = useRouter()
const { data: applications } = useMyCarpoolApplications({ sort: 'default_buyer' })

const statusGroups: Record<string, CarpoolApplication['status'][]> = {
  待车主处理: ['pending_owner'],
  有效成员: ['active'],
  退出或移除: ['cancelled_by_buyer', 'cancelled_by_owner'],
  已拒绝: ['rejected'],
  纠纷: ['disputed'],
}

const pageFilters = computed(() => ({
  status: activeStatus.value === '全部' ? undefined : statusGroups[activeStatus.value],
  sort: 'default_buyer' as const,
}))
const pagination = useCursorPagination([activeStatus])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useMyCarpoolApplicationsPage(pageFilters, pageRequest)
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const builtInProductIcons = new Map<string, string>()
const stats = computed(() => {
  const all = applications.value ?? []
  return [
    { label: '待车主处理', value: all.filter(item => item.status === 'pending_owner').length },
    { label: '有效成员', value: all.filter(item => item.status === 'active').length },
    { label: '已结束', value: all.filter(item => ['cancelled_by_buyer', 'cancelled_by_owner'].includes(item.status)).length },
  ]
})

function statusVariant(status: CarpoolApplication['status']) {
  if (status === 'active') return 'default'
  if (['disputed', 'rejected'].includes(status)) return 'secondary'
  return 'outline'
}

function productIconSrc(product: string) {
  return getProductCategoryIconSrc(getProductCategory(product), builtInProductIcons)
}

function productToneClass(product: string) {
  return `my-transaction-icon--${getProductCategory(product)}`
}

function seatLabel(item: CarpoolApplication) {
  if (item.status === 'pending_owner') return '等待车主确认'
  if (item.status === 'active') return '正在使用'
  return '查看状态记录'
}

function openApplication(event: MouseEvent | KeyboardEvent, id: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button')) return
  router.push(`/my/rides/${id}`)
}
</script>

<template>
  <div class="my-rides-reference space-y-4">
    <div class="my-rides-heading rounded-xl border px-5 py-4"><PageTitle title="我的上车" description="查看申请、有效成员关系和退出记录。" action-text="继续找车源" action-to="/carpools" /></div>
    <div class="my-rides-reference-stats">
      <div><span><PlayCircle /></span><dl><dt>待车主处理</dt><dd>{{ stats[0]?.value ?? 0 }}</dd><small>可确认最新版条件</small></dl></div>
      <div><span><UserRound /></span><dl><dt>有效成员</dt><dd>{{ stats[1]?.value ?? 0 }}</dd><small>成员关系存续中</small></dl></div>
      <div><span><Clock3 /></span><dl><dt>已结束</dt><dd>{{ stats[2]?.value ?? 0 }}</dd><small>退出或被移除</small></dl></div>
    </div>
    <StatusTabs v-model="activeStatus" :items="['全部', '待车主处理', '有效成员', '退出或移除', '已拒绝', '纠纷']" />
    <SkeletonTable v-if="isLoading" :rows="5" :columns="6" />
    <EmptyState v-else-if="rows.length === 0" title="当前筛选下暂无上车申请" description="可以继续浏览车源，或切换状态查看历史申请。" />
    <div v-else v-auto-animate="functionalMotion" class="my-transaction-list">
      <Card
        v-for="item in rows"
        :key="item.id"
        class="my-transaction-row my-ride-row"
        tabindex="0"
        @click="openApplication($event, item.id)"
        @keydown.enter="openApplication($event, item.id)"
      >
        <div class="my-transaction-product">
          <span class="my-transaction-icon" :class="productToneClass(item.snapshot.productName)">
            <img v-if="productIconSrc(item.snapshot.productName)" :src="productIconSrc(item.snapshot.productName)!" alt="" />
            <Package v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0">
            <div class="truncate font-semibold text-slate-950">{{ item.snapshot.productName }}</div>
            <div class="mt-1 truncate text-xs text-muted-foreground"><ShortId :value="item.id" prefix="RIDE" /> · {{ item.snapshot.regionName }} · {{ item.snapshot.warrantyText }}</div>
          </div>
        </div>
        <div class="my-transaction-metric"><small>月费快照</small><strong>{{ item.snapshot.priceLabel }} ¥{{ item.snapshot.monthlyPriceCny }}</strong></div>
        <div class="my-transaction-owner"><span><UserRound class="h-4 w-4" /></span><div><small>车主</small><strong>{{ item.ownerUsername }}</strong><em>信任等级 {{ item.snapshot.ownerTrustLevel }}</em></div></div>
        <div class="my-transaction-metric"><small>成员状态</small><strong>{{ seatLabel(item) }}</strong><em><CalendarClock class="h-3.5 w-3.5" /><LocalTime :value="item.updatedAt" /></em></div>
        <div class="my-transaction-state"><Badge :variant="statusVariant(item.status)">{{ getCarpoolApplicationStatusLabel(item.status) }}</Badge><span>{{ getCarpoolApplicationNextAction(item, 'buyer') }}</span></div>
        <ArrowRight class="my-transaction-arrow" />
      </Card>
      <div class="my-transaction-pagination"><CursorTablePagination :page="pagination.page.value" :item-count="rows.length" :has-next-page="Boolean(pageQuery.data.value?.nextCursor)" :loading="pageQuery.isFetching.value" @previous="pagination.previous" @next="pagination.next(pageQuery.data.value?.nextCursor)" /></div>
    </div>
  </div>
</template>
