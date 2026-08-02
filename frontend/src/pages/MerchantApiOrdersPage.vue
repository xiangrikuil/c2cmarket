<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, KeyRound } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { usePagination } from '@/composables/usePagination'
import {
  confirmApiOrderPayment,
  getApiMerchantVisibilityLabel,
  getApiOrderDisplayStatus,
  getApiOrderNextAction,
  isApiOrderReceiptConfirmed,
  type ApiOrder,
} from '@/lib/api'
import { apiPaymentMethodLabels } from '@/lib/apiPaymentSettings'
import { addDecimal, compareDecimal, formatDecimal } from '@/lib/decimal'
import { useMerchantApiOrders } from '@/queries/useMarketQueries'

const queryClient = useQueryClient()
const { data, isLoading, error, refetch } = useMerchantApiOrders({ sort: 'default_merchant' })
const activeTab = ref('全部')
const keyword = ref('')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const serviceFilter = ref('all')
const sortMode = ref<'default' | 'updated' | 'amount'>('default')
const busyId = ref('')

const deliveredStatuses = ['delivery_submitted', 'completed']

const baseFilteredRows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  const rangeMs = timeRange.value === 'today' ? 24 * 60 * 60 * 1000 : timeRange.value === '7d' ? 7 * 24 * 60 * 60 * 1000 : timeRange.value === '30d' ? 30 * 24 * 60 * 60 * 1000 : null

  return [...(data.value ?? [])].filter(item => {
    const createdAt = new Date(item.createdAt).getTime()
    return (!rangeMs || Date.now() - createdAt <= rangeMs)
      && (serviceFilter.value === 'all' || item.apiServiceId === serviceFilter.value)
      && (!q || [item.id, item.buyer, item.serviceTitle].some(value => value.toLowerCase().includes(q)))
  })
})

const filteredRows = computed(() => baseFilteredRows.value.filter(item => activeTab.value === '全部'
  || (activeTab.value === '待买家付款' && item.status === 'pending_payment')
  || (activeTab.value === '待确认收款' && item.status === 'payment_submitted')
  || (activeTab.value === '等待买家补充' && item.status === 'payment_issue')
  || (activeTab.value === '待交付' && item.status === 'paid_confirmed')
  || (activeTab.value === '已完成交付' && deliveredStatuses.includes(item.status))
  || (activeTab.value === '已取消' && item.status === 'cancelled')))

const confirmedReceiptAmount = computed(() => baseFilteredRows.value
  .filter(item => isApiOrderReceiptConfirmed(item.status))
  .reduce(
    (total, item) => addDecimal(total, item.amountDecimal ?? String(item.amount), 2),
    '0.00',
  ))

const stats = computed(() => [
  { label: '待买家付款', value: baseFilteredRows.value.filter(item => item.status === 'pending_payment').length },
  { label: '待确认收款', value: baseFilteredRows.value.filter(item => item.status === 'payment_submitted').length },
  { label: '等待买家补充', value: baseFilteredRows.value.filter(item => item.status === 'payment_issue').length },
  { label: '待交付', value: baseFilteredRows.value.filter(item => item.status === 'paid_confirmed').length },
  { label: '已完成交付', value: baseFilteredRows.value.filter(item => deliveredStatuses.includes(item.status)).length },
  { label: '已取消订单', value: baseFilteredRows.value.filter(item => item.status === 'cancelled').length },
  { label: '已确认收款金额', value: `¥${formatDecimal(confirmedReceiptAmount.value, 2, 2)}` },
])

const rows = computed(() => [...filteredRows.value].sort((a, b) => {
  if (sortMode.value === 'amount') return compareDecimal(b.amountDecimal ?? String(b.amount), a.amountDecimal ?? String(a.amount))
  if (sortMode.value === 'updated') return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
  const aAction = a.status === 'payment_submitted' || a.status === 'paid_confirmed'
  const bAction = b.status === 'payment_submitted' || b.status === 'paid_confirmed'
  return Number(bAction) - Number(aAction)
    || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
}))

const pagination = usePagination(rows)
const serviceOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const item of data.value ?? []) seen.set(item.apiServiceId, item.serviceTitle)
  return [...seen.entries()].map(([id, title]) => ({ id, title }))
})

async function refresh() {
  await queryClient.invalidateQueries({ queryKey: ['merchant-api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['my-api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
  await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
}

async function runAction(item: ApiOrder, action: () => Promise<unknown>, message: string) {
  busyId.value = item.id
  try {
    await action()
    await refresh()
    toast.success(message)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="space-y-4">
    <PageTitle title="API 销售订单" description="处理买家付款确认和一次性交付；提交凭证后你的履约任务即完成，后续问题可通过订单联系方式沟通。" />

    <CompactStats :items="stats" :loading="isLoading" />

    <StatusTabs v-model="activeTab" :items="['全部', '待买家付款', '待确认收款', '等待买家补充', '待交付', '已完成交付', '已取消']" />

    <div class="grid gap-2 md:grid-cols-2 xl:grid-cols-[1fr_160px_180px_180px]">
      <Input v-model="keyword" placeholder="搜索订单编号、买家、服务" />
      <Select v-model="timeRange"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部时间</SelectItem><SelectItem value="today">今天</SelectItem><SelectItem value="7d">近 7 天</SelectItem><SelectItem value="30d">近 30 天</SelectItem></SelectContent></Select>
      <Select v-model="serviceFilter"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部服务</SelectItem><SelectItem v-for="service in serviceOptions" :key="service.id" :value="service.id">{{ service.title }}</SelectItem></SelectContent></Select>
      <Select v-model="sortMode"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="default">默认排序</SelectItem><SelectItem value="updated">更新时间</SelectItem><SelectItem value="amount">订单金额</SelectItem></SelectContent></Select>
    </div>

    <ErrorState v-if="error" description="商户 API 订单暂时无法加载。" @retry="refetch()" />
    <SkeletonTable v-else-if="isLoading" :columns="6" />
    <EmptyState v-else-if="rows.length === 0" title="当前筛选下暂无订单" description="调整筛选条件后再试；新订单到达后会在这里显示。" />
    <SoftTable v-else animate-rows class="[&_table]:min-w-[760px]" :columns="['订单', '买家 / 服务', '订单金额 / 购买额度', '状态', '更新', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium"><ShortId :value="item.id" prefix="API" copyable /></div><div class="text-xs text-muted-foreground"><LocalTime :value="item.createdAt" /></div></td>
        <td>
          <div class="font-medium">{{ item.buyer }}</div>
          <div class="text-xs text-muted-foreground">{{ item.serviceTitle }} · {{ item.seller }} · {{ getApiMerchantVisibilityLabel(item.intentSnapshot) }}</div>
        </td>
        <td>
          <div class="font-semibold">¥{{ formatDecimal(item.amountDecimal ?? String(item.amount), 2, 2) }}</div>
          <div class="mt-1 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <ApiPaymentMethodIcon :method="item.selectedPaymentMethod" size="sm" />
            {{ apiPaymentMethodLabels[item.selectedPaymentMethod] }} · {{ formatDecimal(item.requestedUsdAllowanceDecimal ?? String(item.requestedUsdAllowance), 2, 6) }} 美元额度
          </div>
        </td>
        <td><StatusBadge :status="item.status" :label="getApiOrderDisplayStatus(item, 'merchant')" /></td>
        <td class="text-xs text-muted-foreground"><LocalTime :value="item.updatedAt" /></td>
        <td>
          <div class="flex flex-wrap gap-1">
            <Button v-if="item.status === 'payment_submitted'" size="sm" :disabled="busyId === item.id" @click="runAction(item, () => confirmApiOrderPayment(item.id, item.version), '已确认收款。')">
              <CheckCircle2 class="h-4 w-4" />确认已收款
            </Button>
            <RouterLink v-if="item.status === 'paid_confirmed'" :to="`/merchant/api-orders/${item.id}`"><Button size="sm" variant="outline"><KeyRound class="h-4 w-4" />填写交付</Button></RouterLink>
            <RouterLink v-if="item.status !== 'payment_submitted' && item.status !== 'paid_confirmed'" :to="`/merchant/api-orders/${item.id}`"><Button size="sm" variant="outline">查看</Button></RouterLink>
            <span v-if="item.status === 'delivery_submitted' || item.status === 'completed'" class="text-xs text-muted-foreground">{{ getApiOrderNextAction(item, 'merchant') }}</span>
          </div>
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
