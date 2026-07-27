<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, CalendarClock, CheckCircle2, Clock3, Package, PlayCircle, UserRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { usePagination } from '@/composables/usePagination'
import { getCarpoolApplicationNextAction, getCarpoolApplicationStatusLabel, type CarpoolApplication } from '@/lib/api'
import { getProductCategory } from '@/lib/productCategories'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import { useMyCarpoolApplications } from '@/queries/useMarketQueries'

const activeStatus = ref('全部')
const router = useRouter()
const { data: applications, isLoading } = useMyCarpoolApplications({ sort: 'default_buyer' })

const statusGroups: Record<string, CarpoolApplication['status'][]> = {
  待车主处理: ['pending_owner'],
  待联系: ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation'],
  服务中: ['active'],
  待完成: ['pending_completion'],
  已完成: ['completed'],
  已取消: ['rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'],
  纠纷: ['disputed'],
}

const rows = computed(() => {
  const all = applications.value ?? []
  if (activeStatus.value === '全部') return all
  return all.filter(item => statusGroups[activeStatus.value]?.includes(item.status))
})

const pagination = usePagination(rows)
const builtInProductIcons = new Map<string, string>()
const stats = computed(() => {
  const all = applications.value ?? []
  return [
    { label: '需要我处理', value: all.filter(item => ['waiting_contact', 'contacted', 'joined_pending_confirmation', 'pending_completion'].includes(item.status)).length },
    { label: '等待车主', value: all.filter(item => ['pending_owner', 'accepted_reserved'].includes(item.status)).length },
    { label: '服务中', value: all.filter(item => item.status === 'active').length },
    { label: '已完成', value: all.filter(item => item.status === 'completed').length },
  ]
})

function statusVariant(status: CarpoolApplication['status']) {
  if (['completed', 'active'].includes(status)) return 'default'
  if (['disputed', 'rejected', 'expired'].includes(status)) return 'secondary'
  return 'outline'
}

function productIconSrc(product: string) {
  return getProductCategoryIconSrc(getProductCategory(product), builtInProductIcons)
}

function productToneClass(product: string) {
  return `my-transaction-icon--${getProductCategory(product)}`
}

function seatLabel(item: CarpoolApplication) {
  if (item.status === 'pending_owner') return '尚未占用席位'
  if (item.reservedUntil) return '席位已预留'
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
    <div class="my-rides-heading rounded-xl border px-5 py-4"><PageTitle title="我的上车" description="查看上车申请、席位预留、联系沟通、服务中、待完成和评价状态。" action-text="继续找车源" action-to="/carpools" /></div>
    <div class="my-rides-reference-stats">
      <div><span><PlayCircle /></span><dl><dt>需要我处理</dt><dd>{{ stats[0]?.value ?? 0 }}</dd><small>继续确认当前步骤</small></dl></div>
      <div><span><Clock3 /></span><dl><dt>等待车主</dt><dd>{{ stats[1]?.value ?? 0 }}</dd><small>等待车主处理</small></dl></div>
      <div><span><UserRound /></span><dl><dt>服务中</dt><dd>{{ stats[2]?.value ?? 0 }}</dd><small>当前正在使用</small></dl></div>
      <div><span><CheckCircle2 /></span><dl><dt>已完成</dt><dd>{{ stats[3]?.value ?? 0 }}</dd><small>历史完成记录</small></dl></div>
    </div>
    <StatusTabs v-model="activeStatus" :items="['全部', '待车主处理', '待联系', '服务中', '待完成', '已完成', '已取消', '纠纷']" />
    <SkeletonTable v-if="isLoading" :rows="5" :columns="6" />
    <EmptyState v-else-if="rows.length === 0" title="当前筛选下暂无上车申请" description="可以继续浏览车源，或切换状态查看历史申请。" />
    <div v-else class="my-transaction-list">
      <Card
        v-for="item in pagination.paginatedRows.value"
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
        <div class="my-transaction-metric"><small>席位状态</small><strong>{{ seatLabel(item) }}</strong><em v-if="item.reservedUntil">预留至 <LocalTime :value="item.reservedUntil" /></em><em v-else><CalendarClock class="h-3.5 w-3.5" /><LocalTime :value="item.updatedAt" /></em></div>
        <div class="my-transaction-state"><Badge :variant="statusVariant(item.status)">{{ getCarpoolApplicationStatusLabel(item.status) }}</Badge><span>{{ getCarpoolApplicationNextAction(item, 'buyer') }}</span></div>
        <ArrowRight class="my-transaction-arrow" />
      </Card>
      <div class="my-transaction-pagination"><TablePagination v-model:page="pagination.page.value" :page-count="pagination.pageCount.value" :total="pagination.total.value" :start-item="pagination.startItem.value" :end-item="pagination.endItem.value" /></div>
    </div>
  </div>
</template>
