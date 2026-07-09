<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, KeyRound } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  confirmApiOrderPayment,
  formatUsdQuota,
  getApiMerchantVisibilityLabel,
  getApiOrderNextAction,
  getApiOrderStatusLabel,
  type ApiOrder,
} from '@/lib/api'
import { useMerchantApiOrders } from '@/queries/useMarketQueries'

const queryClient = useQueryClient()
const { data } = useMerchantApiOrders({ sort: 'default_merchant' })
const activeTab = ref('全部')
const keyword = ref('')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const serviceFilter = ref('all')
const sortMode = ref<'default' | 'updated' | 'amount'>('default')
const busyId = ref('')

const activeStatuses = ['pending_payment', 'payment_submitted', 'paid_confirmed']
const deliveredStatuses = ['delivery_submitted', 'completed']

const filteredRows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  const rangeMs = timeRange.value === 'today' ? 24 * 60 * 60 * 1000 : timeRange.value === '7d' ? 7 * 24 * 60 * 60 * 1000 : timeRange.value === '30d' ? 30 * 24 * 60 * 60 * 1000 : null

  return [...(data.value ?? [])].filter(item => {
    const createdAt = new Date(item.createdAt).getTime()
    const tabMatched = activeTab.value === '全部'
      || (activeTab.value === '待买家付款' && item.status === 'pending_payment')
      || (activeTab.value === '待确认收款' && item.status === 'payment_submitted')
      || (activeTab.value === '待交付' && item.status === 'paid_confirmed')
      || (activeTab.value === '已交付' && deliveredStatuses.includes(item.status))
      || (activeTab.value === '已取消' && item.status === 'cancelled')
    return tabMatched
      && (!rangeMs || Date.now() - createdAt <= rangeMs)
      && (serviceFilter.value === 'all' || item.apiServiceId === serviceFilter.value)
      && (!q || [item.id, item.buyer, item.serviceTitle].some(value => value.toLowerCase().includes(q)))
  })
})

const intentAmountTotal = computed(() => filteredRows.value.reduce((total, item) => total + item.amount, 0))

const stats = computed(() => [
  { label: '待买家付款', value: filteredRows.value.filter(item => item.status === 'pending_payment').length },
  { label: '待确认收款', value: filteredRows.value.filter(item => item.status === 'payment_submitted').length },
  { label: '待交付', value: filteredRows.value.filter(item => item.status === 'paid_confirmed').length },
  { label: '已交付', value: filteredRows.value.filter(item => deliveredStatuses.includes(item.status)).length },
  { label: '订单金额合计', value: `¥${intentAmountTotal.value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}` },
])

const rows = computed(() => [...filteredRows.value].sort((a, b) => {
  if (sortMode.value === 'amount') return b.amount - a.amount
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
    <PageTitle title="商户 API 订单" description="处理买家付款确认和一次性站内交付；交付信息提交后不可修改，后续问题通过联系方式站外沟通。" />

    <div class="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
      <div v-for="item in stats" :key="item.label" class="rounded-lg border border-border bg-card p-4">
        <div class="text-xs text-muted-foreground">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeTab" :items="['全部', '待买家付款', '待确认收款', '待交付', '已交付', '已取消']" />

    <div class="grid gap-2 md:grid-cols-2 xl:grid-cols-[1fr_160px_180px_180px]">
      <Input v-model="keyword" placeholder="搜索订单编号、买家、服务" />
      <select v-model="timeRange" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="all">全部时间</option>
        <option value="today">今天</option>
        <option value="7d">近 7 天</option>
        <option value="30d">近 30 天</option>
      </select>
      <select v-model="serviceFilter" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="all">全部服务</option>
        <option v-for="service in serviceOptions" :key="service.id" :value="service.id">{{ service.title }}</option>
      </select>
      <select v-model="sortMode" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="default">默认排序</option>
        <option value="updated">更新时间</option>
        <option value="amount">意向金额</option>
      </select>
    </div>

    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无商户 API 订单。</div>
    <SoftTable v-else :columns="['订单', '买家 / 服务', '金额 / 额度上限', '状态', '更新', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium">{{ item.id }}</div><div class="text-xs text-muted-foreground">{{ item.createdAt }}</div></td>
        <td>
          <div class="font-medium">{{ item.buyer }}</div>
          <div class="text-xs text-muted-foreground">{{ item.serviceTitle }} · {{ item.seller }} · {{ getApiMerchantVisibilityLabel(item.intentSnapshot) }}</div>
        </td>
        <td><div class="font-semibold">¥{{ item.amount }}</div><div class="text-xs text-muted-foreground">上限 {{ formatUsdQuota(item.requestedUsdAllowance) }}</div></td>
        <td><Badge :variant="activeStatuses.includes(item.status) ? 'default' : deliveredStatuses.includes(item.status) ? 'verified' : 'secondary'">{{ getApiOrderStatusLabel(item.status) }}</Badge></td>
        <td class="text-xs text-muted-foreground">{{ item.updatedAt }}</td>
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
