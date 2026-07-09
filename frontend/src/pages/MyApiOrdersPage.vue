<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  formatUsdQuota,
  getApiMerchantVisibilityLabel,
  getApiOrderNextAction,
  getApiOrderStatusLabel,
} from '@/lib/api'
import { useMyApiOrders } from '@/queries/useMarketQueries'

const { data } = useMyApiOrders({ sort: 'default_buyer' })
const activeTab = ref('全部')
const keyword = ref('')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const sortMode = ref<'default' | 'updated' | 'created' | 'amount'>('default')

const activeStatuses = ['pending_payment', 'payment_submitted', 'paid_confirmed']
const deliveredStatuses = ['delivery_submitted', 'completed']

const stats = computed(() => {
  const rows = data.value ?? []
  return [
    { label: '待付款', value: rows.filter(item => item.status === 'pending_payment').length },
    { label: '待商户确认', value: rows.filter(item => item.status === 'payment_submitted').length },
    { label: '待交付', value: rows.filter(item => item.status === 'paid_confirmed').length },
    { label: '已交付', value: rows.filter(item => deliveredStatuses.includes(item.status)).length },
  ]
})

const rows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return [...(data.value ?? [])]
    .filter(item => {
      const createdAt = new Date(item.createdAt).getTime()
      const rangeMs = timeRange.value === 'today' ? 24 * 60 * 60 * 1000 : timeRange.value === '7d' ? 7 * 24 * 60 * 60 * 1000 : timeRange.value === '30d' ? 30 * 24 * 60 * 60 * 1000 : null
      const tabMatched = activeTab.value === '全部'
        || (activeTab.value === '待付款' && item.status === 'pending_payment')
        || (activeTab.value === '已付款' && item.status === 'payment_submitted')
        || (activeTab.value === '待交付' && item.status === 'paid_confirmed')
        || (activeTab.value === '已交付' && deliveredStatuses.includes(item.status))
        || (activeTab.value === '已取消' && item.status === 'cancelled')
      return tabMatched
        && (!rangeMs || Date.now() - createdAt <= rangeMs)
        && (!q || [item.id, item.serviceTitle, item.seller].some(value => value.toLowerCase().includes(q)))
    })
    .sort((a, b) => {
      if (sortMode.value === 'amount') return b.amount - a.amount
      if (sortMode.value === 'created') return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      if (sortMode.value === 'updated') return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      const aAction = activeStatuses.includes(a.status)
      const bAction = activeStatuses.includes(b.status)
      return Number(bAction) - Number(aAction)
        || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    })
})

const pagination = usePagination(rows)
</script>

<template>
  <div class="space-y-4">
    <PageTitle title="我的 API 订单" description="查看收款资料、付款状态、商户交付信息和历史订单；支付仍由双方站外完成。" action-text="继续找服务" action-to="/api-market" />

    <div class="grid gap-3 md:grid-cols-4">
      <div v-for="item in stats" :key="item.label" class="rounded-lg border border-border bg-card p-4">
        <div class="text-xs text-muted-foreground">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeTab" :items="['全部', '待付款', '已付款', '待交付', '已交付', '已取消']" />

    <div class="grid gap-2 md:grid-cols-[1fr_160px_180px]">
      <Input v-model="keyword" placeholder="搜索订单编号、服务、商户" />
      <select v-model="timeRange" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="all">全部时间</option>
        <option value="today">今天</option>
        <option value="7d">近 7 天</option>
        <option value="30d">近 30 天</option>
      </select>
      <select v-model="sortMode" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="default">默认排序</option>
        <option value="updated">更新时间</option>
        <option value="created">创建时间</option>
        <option value="amount">意向金额</option>
      </select>
    </div>

    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无 API 订单。</div>
    <SoftTable v-else :columns="['订单', '服务快照', '商户', '金额 / 额度上限', '状态', '下一步', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium">{{ item.id }}</div><div class="text-xs text-muted-foreground">{{ item.createdAt }}</div></td>
        <td><div class="font-medium">{{ item.serviceTitle }}</div><div class="text-xs text-muted-foreground">{{ item.intentSnapshot.models.join(' / ') }}</div></td>
        <td>
          <div>{{ item.seller }} · 信任等级{{ item.intentSnapshot.trustLevel }}</div>
          <div class="text-xs text-muted-foreground">{{ getApiMerchantVisibilityLabel(item.intentSnapshot) }}</div>
        </td>
        <td><div class="font-semibold">¥{{ item.amount }}</div><div class="text-xs text-muted-foreground">上限 {{ formatUsdQuota(item.requestedUsdAllowance) }} · {{ item.intentSnapshot.multiplier }}</div></td>
        <td>
          <Badge :variant="activeStatuses.includes(item.status) ? 'default' : deliveredStatuses.includes(item.status) ? 'verified' : 'secondary'">{{ getApiOrderStatusLabel(item.status) }}</Badge>
          <div class="mt-1 text-xs text-muted-foreground">付款和交付信息按参与方权限展示</div>
        </td>
        <td class="text-xs text-muted-foreground">{{ getApiOrderNextAction(item, 'buyer') }}</td>
        <td>
          <RouterLink :to="`/my/api-orders/${item.id}`"><Button size="sm">查看</Button></RouterLink>
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
