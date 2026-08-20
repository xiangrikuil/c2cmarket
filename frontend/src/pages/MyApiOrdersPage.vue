<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { ArrowRight, CalendarClock, Code2, KeyRound, WalletCards } from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import {
  getApiMerchantVisibilityLabel,
  getApiOrderDisplayStatus,
  getApiOrderNextAction,
  type ApiOrderStatus,
} from '@/lib/api'
import { apiPaymentMethodLabels } from '@/lib/apiPaymentSettings'
import { formatDecimal } from '@/lib/decimal'
import { functionalMotion } from '@/lib/motion'
import { getApiServiceProductIconSrc } from '@/lib/productCategoryIcon'
import { useMyApiOrders, useMyApiOrdersPage } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'

const { data } = useMyApiOrders({ sort: 'default_buyer' })
const { data: catalogCategories } = useProductCategories()
const router = useRouter()
const activeTab = ref('全部')
const keyword = ref('')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const sortMode = ref<'default' | 'updated' | 'created' | 'amount'>('default')

const activeStatuses = ['pending_payment', 'payment_issue']

const tabStatusByLabel: Partial<Record<string, ApiOrderStatus>> = {
  待付款: 'pending_payment',
  待补充: 'payment_issue',
  已付款: 'payment_submitted',
  待交付: 'paid_confirmed',
  已完成: 'completed',
  已取消: 'cancelled',
}
const tabStatus = computed(() => tabStatusByLabel[activeTab.value])
const pageFilters = computed(() => ({
  status: tabStatus.value,
  dispute: activeTab.value === '纠纷中' ? 'active' as const : undefined,
  search: keyword.value.trim() || undefined,
  dateRange: timeRange.value,
  sort: sortMode.value === 'amount' ? 'amount_desc' as const
    : sortMode.value === 'created' ? 'created_desc' as const
      : sortMode.value === 'updated' ? 'updated_desc' as const
        : 'default_buyer' as const,
}))
const pagination = useCursorPagination([activeTab, keyword, timeRange, sortMode])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useMyApiOrdersPage(pageFilters, pageRequest)
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const error = pageQuery.error
const refetch = pageQuery.refetch
const totalAmount = computed(() => (data.value ?? []).reduce((sum, item) => sum + Number(item.amountDecimal ?? item.amount), 0))
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

function orderProductIconSrc(item: (typeof rows.value)[number]) {
  return getApiServiceProductIconSrc({
    title: item.serviceTitle,
    models: item.intentSnapshot.models,
    modelPriceRows: item.intentSnapshot.modelPrices,
  }, categoryIconByCode.value)
}

function sellerInitial(value: string) {
  return value.trim().slice(0, 1).toUpperCase() || '商'
}

function openOrder(event: MouseEvent | KeyboardEvent, id: string) {
  if (event.target instanceof Element && event.target.closest('a,button')) return
  router.push(`/my/api-orders/${id}`)
}

function disputeActionLabel(item: (typeof rows.value)[number]) {
  if (item.disputeAvailableActions?.includes('request_platform_intervention')) return '可申请平台介入'
  if (item.disputeAvailableActions?.includes('confirm_remedy')) return '确认卖家处理结果'
  if (item.disputeAvailableActions?.includes('withdraw')) return '等待卖家处理，可撤回'
  if (item.disputeNeedsAction) return '售后待你处理'
  if (item.disputeNextActor === 'admin') return '等待平台处理'
  if (item.disputeNextActor === 'respondent') return '等待卖家处理'
  if (item.disputeNextActor === 'responsible_party') return '等待卖家或责任方履行'
  if (item.disputeNextActor === 'counterparty') return '等待对方确认'
  return '纠纷处理中'
}

function disputeStatusLabel(item: (typeof rows.value)[number]) {
  if (item.disputeNeedsAction) return '待你处理'
  if (item.disputeNextActor === 'admin') return '平台处理中'
  if (item.disputeNextActor === 'respondent') return '等待卖家处理'
  if (item.disputeNextActor === 'responsible_party') return '等待责任方履行'
  return '纠纷处理中'
}
</script>

<template>
  <div class="my-api-orders-reference space-y-4">
    <div class="my-api-orders-heading rounded-xl border px-5 py-4"><PageTitle title="API 购买订单" description="查看自己作为买家创建的订单、付款状态和商户交付记录；付款由你与商户直接完成，平台不代收或托管资金。" action-text="继续找服务" action-to="/api-market" /></div>

    <div class="my-api-orders-layout">
      <main class="min-w-0 space-y-4">
        <StatusTabs v-model="activeTab" :items="['全部', '纠纷中', '待付款', '待补充', '已付款', '待交付', '已完成', '已取消']" />

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
        <div v-else v-auto-animate="functionalMotion" class="my-transaction-list">
          <Card v-for="item in rows" :key="item.id" class="my-transaction-row my-api-order-row" tabindex="0" @click="openOrder($event, item.id)" @keydown.enter="openOrder($event, item.id)">
            <div class="my-transaction-product">
              <span class="my-transaction-icon my-transaction-icon--api">
                <img v-if="orderProductIconSrc(item)" :src="orderProductIconSrc(item) ?? undefined" alt="" />
                <Code2 v-else class="h-5 w-5" />
              </span>
              <div class="min-w-0"><div class="truncate font-semibold text-slate-950">{{ item.serviceTitle }}</div><div class="mt-1 flex items-center gap-1.5 truncate text-xs text-muted-foreground"><ShortId :value="item.orderNo" full copyable /> · {{ item.intentSnapshot.models.join(' / ') }}</div></div>
            </div>
            <div class="my-transaction-metric">
              <small>支付金额</small>
              <strong>¥{{ formatDecimal(item.amountDecimal ?? String(item.amount), 2, 2) }}</strong>
              <em><ApiPaymentMethodIcon :method="item.selectedPaymentMethod" size="sm" />{{ apiPaymentMethodLabels[item.selectedPaymentMethod] }} · {{ formatDecimal(item.requestedUsdAllowanceDecimal ?? String(item.requestedUsdAllowance), 2, 6) }} 美元额度 · {{ item.intentSnapshot.multiplier }}</em>
            </div>
            <div class="my-transaction-owner"><span>{{ sellerInitial(item.seller) }}</span><div><small>商户</small><strong>{{ item.seller }}</strong><em>{{ item.intentSnapshot.trustLevel === null ? '信任等级暂无数据' : `信任等级 ${item.intentSnapshot.trustLevel}` }} · {{ getApiMerchantVisibilityLabel(item.intentSnapshot) }}</em></div></div>
            <div class="my-transaction-metric"><small>创建时间</small><strong class="inline-flex items-center gap-1.5"><CalendarClock class="h-3.5 w-3.5 text-muted-foreground" /><LocalTime :value="item.createdAt" /></strong><em>付款和交付信息按参与方权限展示</em></div>
            <div class="my-transaction-state">
              <StatusBadge :status="item.status" :label="getApiOrderDisplayStatus(item, 'buyer')" />
              <StatusBadge v-if="item.disputeCaseId" status="open" :label="disputeStatusLabel(item)" />
              <span>{{ item.disputeCaseId ? disputeActionLabel(item) : getApiOrderNextAction(item, 'buyer') }}</span>
              <span v-if="item.disputeCaseId && item.disputeDueAt" class="text-xs text-muted-foreground">截止 <LocalTime :value="item.disputeDueAt" /></span>
              <RouterLink v-if="item.disputeCaseId" :to="`/my/disputes/${item.disputeCaseId}?orderId=${item.id}`" class="text-xs font-medium text-primary">查看案件</RouterLink>
              <Button v-if="item.deliverySubmittedAt" as-child size="sm" class="mt-1">
                <RouterLink :to="`/my/api-orders/${item.id}/delivery`"><KeyRound class="h-4 w-4" />查看交付</RouterLink>
              </Button>
            </div>
            <ArrowRight class="my-transaction-arrow" />
          </Card>
          <div class="my-transaction-pagination"><CursorTablePagination :page="pagination.page.value" :item-count="rows.length" :has-next-page="Boolean(pageQuery.data.value?.nextCursor)" :loading="pageQuery.isFetching.value" @previous="pagination.previous" @next="pagination.next(pageQuery.data.value?.nextCursor)" /></div>
        </div>
      </main>
      <aside class="my-api-orders-aside space-y-3">
        <Card class="my-api-order-overview p-4">
          <div class="flex items-center justify-between"><h2 class="font-semibold">订单概览</h2><WalletCards class="h-5 w-5 text-cyan-600" /></div>
          <div class="mt-4 grid grid-cols-2 gap-3"><div><small>订单总数</small><strong>{{ (data ?? []).length }}</strong></div><div><small>纠纷中</small><strong>{{ (data ?? []).filter(item => item.disputeCaseId).length }}</strong></div><div><small>待我处理</small><strong>{{ (data ?? []).filter(item => activeStatuses.includes(item.status) || item.disputeNeedsAction).length }}</strong></div><div><small>已完成</small><strong>{{ (data ?? []).filter(item => item.status === 'completed' || item.status === 'delivery_submitted').length }}</strong></div></div>
        </Card>
        <Card class="p-4">
          <h2 class="font-semibold">订单处理顺序</h2>
          <ol class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
            <li>1. 查看商户收款资料并完成付款</li>
            <li>2. 提交付款信息，异常时按提示补充</li>
            <li>3. 商户确认到账后提交交付</li>
						<li>4. 商家提交凭证后订单完成；有问题可联系商家或发起纠纷</li>
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
