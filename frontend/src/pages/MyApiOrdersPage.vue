<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { ArrowRight, CalendarClock, Code2, WalletCards } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { usePagination } from '@/composables/usePagination'
import {
  getApiMerchantVisibilityLabel,
  getApiOrderNextAction,
  getApiOrderStatusLabel,
} from '@/lib/api'
import { compareDecimal, formatDecimal } from '@/lib/decimal'
import { useMyApiOrders } from '@/queries/useMarketQueries'

const { data, isLoading, error, refetch } = useMyApiOrders({ sort: 'default_buyer' })
const router = useRouter()
const activeTab = ref('全部')
const keyword = ref('')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const sortMode = ref<'default' | 'updated' | 'created' | 'amount'>('default')

const activeStatuses = ['pending_payment', 'payment_issue', 'payment_submitted', 'paid_confirmed']
const deliveredStatuses = ['delivery_submitted', 'completed']

const rows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return [...(data.value ?? [])]
    .filter(item => {
      const createdAt = new Date(item.createdAt).getTime()
      const rangeMs = timeRange.value === 'today' ? 24 * 60 * 60 * 1000 : timeRange.value === '7d' ? 7 * 24 * 60 * 60 * 1000 : timeRange.value === '30d' ? 30 * 24 * 60 * 60 * 1000 : null
      const tabMatched = activeTab.value === '全部'
        || (activeTab.value === '待付款' && item.status === 'pending_payment')
        || (activeTab.value === '待补充' && item.status === 'payment_issue')
        || (activeTab.value === '已付款' && item.status === 'payment_submitted')
        || (activeTab.value === '待交付' && item.status === 'paid_confirmed')
        || (activeTab.value === '已交付' && deliveredStatuses.includes(item.status))
        || (activeTab.value === '已取消' && item.status === 'cancelled')
      return tabMatched
        && (!rangeMs || Date.now() - createdAt <= rangeMs)
        && (!q || [item.id, item.serviceTitle, item.seller].some(value => value.toLowerCase().includes(q)))
    })
    .sort((a, b) => {
      if (sortMode.value === 'amount') return compareDecimal(b.amountDecimal ?? String(b.amount), a.amountDecimal ?? String(a.amount))
      if (sortMode.value === 'created') return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      if (sortMode.value === 'updated') return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      const aAction = activeStatuses.includes(a.status)
      const bAction = activeStatuses.includes(b.status)
      return Number(bAction) - Number(aAction)
        || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    })
})

const pagination = usePagination(rows)
const totalAmount = computed(() => (data.value ?? []).reduce((sum, item) => sum + Number(item.amountDecimal ?? item.amount), 0))

function sellerInitial(value: string) {
  return value.trim().slice(0, 1).toUpperCase() || '商'
}

function openOrder(event: MouseEvent | KeyboardEvent, id: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button')) return
  router.push(`/my/api-orders/${id}`)
}
</script>

<template>
  <div class="my-api-orders-reference space-y-4">
    <div class="my-api-orders-heading rounded-xl border px-5 py-4"><PageTitle title="我的 API 订单" description="查看收款资料、付款状态、商户交付信息和历史订单；付款由你与商户直接完成，平台不代收或托管资金。" action-text="继续找服务" action-to="/api-market" /></div>

    <div class="my-api-orders-layout">
      <main class="min-w-0 space-y-4">
        <StatusTabs v-model="activeTab" :items="['全部', '待付款', '待补充', '已付款', '待交付', '已交付', '已取消']" />

        <div class="grid gap-2 md:grid-cols-[1fr_160px_180px]">
          <Input v-model="keyword" placeholder="搜索订单编号、服务、商户" />
          <Select v-model="timeRange"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部时间</SelectItem><SelectItem value="today">今天</SelectItem><SelectItem value="7d">近 7 天</SelectItem><SelectItem value="30d">近 30 天</SelectItem></SelectContent></Select>
          <Select v-model="sortMode"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="default">默认排序</SelectItem><SelectItem value="updated">更新时间</SelectItem><SelectItem value="created">创建时间</SelectItem><SelectItem value="amount">订单金额</SelectItem></SelectContent></Select>
        </div>

        <ErrorState v-if="error" description="API 订单暂时无法加载。" @retry="refetch()" />
        <SkeletonTable v-else-if="isLoading" :columns="7" />
        <EmptyState v-else-if="rows.length === 0" title="暂无 API 订单" description="当前筛选条件下没有订单，可返回 API 市场浏览可购买服务。">
          <template #action><RouterLink to="/api-market"><Button>浏览 API 服务</Button></RouterLink></template>
        </EmptyState>
        <div v-else class="my-transaction-list">
          <Card v-for="item in pagination.paginatedRows.value" :key="item.id" class="my-transaction-row my-api-order-row" tabindex="0" @click="openOrder($event, item.id)" @keydown.enter="openOrder($event, item.id)">
            <div class="my-transaction-product">
              <span class="my-transaction-icon my-transaction-icon--api"><Code2 class="h-5 w-5" /></span>
              <div class="min-w-0"><div class="truncate font-semibold text-slate-950">{{ item.serviceTitle }}</div><div class="mt-1 flex items-center gap-1.5 truncate text-xs text-muted-foreground"><ShortId :value="item.id" prefix="API" copyable /> · {{ item.intentSnapshot.models.join(' / ') }}</div></div>
            </div>
            <div class="my-transaction-metric"><small>支付金额</small><strong>¥{{ formatDecimal(item.amountDecimal ?? String(item.amount), 2, 2) }}</strong><em>{{ formatDecimal(item.requestedUsdAllowanceDecimal ?? String(item.requestedUsdAllowance), 2, 6) }} 美元额度 · {{ item.intentSnapshot.multiplier }}</em></div>
            <div class="my-transaction-owner"><span>{{ sellerInitial(item.seller) }}</span><div><small>商户</small><strong>{{ item.seller }}</strong><em>信任等级 {{ item.intentSnapshot.trustLevel }} · {{ getApiMerchantVisibilityLabel(item.intentSnapshot) }}</em></div></div>
            <div class="my-transaction-metric"><small>创建时间</small><strong class="inline-flex items-center gap-1.5"><CalendarClock class="h-3.5 w-3.5 text-muted-foreground" /><LocalTime :value="item.createdAt" /></strong><em>付款和交付信息按参与方权限展示</em></div>
            <div class="my-transaction-state"><StatusBadge :status="item.status" :label="getApiOrderStatusLabel(item.status)" /><span>{{ getApiOrderNextAction(item, 'buyer') }}</span></div>
            <ArrowRight class="my-transaction-arrow" />
          </Card>
          <div class="my-transaction-pagination"><TablePagination v-model:page="pagination.page.value" :page-count="pagination.pageCount.value" :total="pagination.total.value" :start-item="pagination.startItem.value" :end-item="pagination.endItem.value" /></div>
        </div>
      </main>
      <aside class="my-api-orders-aside space-y-3">
        <Card class="my-api-order-overview p-4">
          <div class="flex items-center justify-between"><h2 class="font-semibold">订单概览</h2><WalletCards class="h-5 w-5 text-cyan-600" /></div>
          <div class="mt-4 grid grid-cols-2 gap-3"><div><small>订单总数</small><strong>{{ (data ?? []).length }}</strong></div><div><small>累计金额</small><strong>¥{{ totalAmount.toFixed(2) }}</strong></div><div><small>待我处理</small><strong>{{ (data ?? []).filter(item => activeStatuses.includes(item.status)).length }}</strong></div><div><small>已完成</small><strong>{{ (data ?? []).filter(item => item.status === 'completed').length }}</strong></div></div>
        </Card>
        <Card class="p-4">
          <h2 class="font-semibold">订单处理顺序</h2>
          <ol class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
            <li>1. 查看商户收款资料并完成付款</li>
            <li>2. 提交付款信息，异常时按提示补充</li>
            <li>3. 商户确认到账后提交交付</li>
            <li>4. 买家核对后完成或发起纠纷</li>
          </ol>
        </Card>
        <Card class="p-4">
          <h2 class="font-semibold">付款异常</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">未到账、金额不符或备注不符时，订单会进入“待补充”。补充付款说明后重新等待商户核对。</p>
        </Card>
        <Card class="p-4 text-sm leading-6 text-muted-foreground">
          付款由你与商户直接完成，平台不代收或托管资金；敏感交付资料仅对订单参与方按权限展示。
        </Card>
      </aside>
    </div>
  </div>
</template>
