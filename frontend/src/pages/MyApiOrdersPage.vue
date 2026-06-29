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
  getApiDeliveryModeLabel,
  getApiIntentNextAction,
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  getApiStatusLabel,
  type ApiDeliveryMode,
} from '@/lib/api'
import { useMyApiPurchaseIntents } from '@/queries/useMarketQueries'

const { data } = useMyApiPurchaseIntents({ sort: 'default_buyer' })
const activeTab = ref('全部')
const keyword = ref('')
const deliveryMode = ref<'all' | ApiDeliveryMode>('all')
const timeRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const sortMode = ref<'default' | 'updated' | 'created' | 'amount'>('default')

const activeStatuses = ['open', 'contacted']

const stats = computed(() => {
  const rows = data.value ?? []
  return [
    { label: '进行中', value: rows.filter(item => activeStatuses.includes(item.status)).length },
    { label: '已取消', value: rows.filter(item => item.status === 'buyer_cancelled').length },
    { label: '商户关闭', value: rows.filter(item => item.status === 'owner_closed').length },
    { label: '历史记录', value: rows.filter(item => ['buyer_cancelled', 'owner_closed'].includes(item.status)).length },
  ]
})

const rows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return [...(data.value ?? [])]
    .filter(item => {
      const createdAt = new Date(item.createdAt).getTime()
      const rangeMs = timeRange.value === 'today' ? 24 * 60 * 60 * 1000 : timeRange.value === '7d' ? 7 * 24 * 60 * 60 * 1000 : timeRange.value === '30d' ? 30 * 24 * 60 * 60 * 1000 : null
      const tabMatched = activeTab.value === '全部'
        || (activeTab.value === '进行中' && activeStatuses.includes(item.status))
        || (activeTab.value === '已取消' && item.status === 'buyer_cancelled')
        || (activeTab.value === '商户关闭' && item.status === 'owner_closed')
      return tabMatched
        && (deliveryMode.value === 'all' || item.selectedDeliveryMode === deliveryMode.value)
        && (!rangeMs || Date.now() - createdAt <= rangeMs)
        && (!q || [item.id, item.snapshot.serviceTitle, getApiMerchantDisplayName(item)].some(value => value.toLowerCase().includes(q)))
    })
    .sort((a, b) => {
      if (sortMode.value === 'amount') return b.purchaseAmountCny - a.purchaseAmountCny
      if (sortMode.value === 'created') return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      if (sortMode.value === 'updated') return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      const aAction = activeStatuses.includes(a.status)
      const bAction = activeStatuses.includes(b.status)
      return Number(bAction) - Number(aAction)
        || new Date(a.merchantResponseDeadline ?? '2999-01-01').getTime() - new Date(b.merchantResponseDeadline ?? '2999-01-01').getTime()
        || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    })
})

const pagination = usePagination(rows)
</script>

<template>
  <div class="space-y-4">
    <PageTitle title="我的 API 意向" description="查看购买意向、商户联系方式、站外确认状态和历史意向记录；最终金额与付款由双方站外确认。" action-text="继续找服务" action-to="/api-market" />

    <div class="grid gap-3 md:grid-cols-4">
      <div v-for="item in stats" :key="item.label" class="rounded-lg border border-border bg-card p-4">
        <div class="text-xs text-muted-foreground">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeTab" :items="['全部', '进行中', '已取消', '商户关闭']" />

    <div class="grid gap-2 md:grid-cols-[1fr_180px_160px_180px]">
      <Input v-model="keyword" placeholder="搜索意向编号、服务、商户" />
      <select v-model="deliveryMode" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="all">全部接入方式</option>
        <option value="api_key_endpoint">API 请求地址接入说明</option>
        <option value="sub2api_panel_account">Sub2API 面板接入说明</option>
      </select>
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

    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无 API 意向记录。</div>
    <SoftTable v-else :columns="['意向记录', '服务快照', '商户', '意向金额 / 额度上限', '接入方式', '状态', '下一步', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium">{{ item.id }}</div><div class="text-xs text-muted-foreground">{{ item.createdAt }}</div></td>
        <td><div class="font-medium">{{ item.snapshot.serviceTitle }}</div><div class="text-xs text-muted-foreground">{{ item.snapshot.models.join(' / ') }}</div></td>
        <td>
          <div>{{ getApiMerchantDisplayName(item) }} · 信任等级{{ item.snapshot.trustLevel }}</div>
          <div class="text-xs text-muted-foreground">{{ getApiMerchantVisibilityLabel(item.snapshot) }}</div>
        </td>
        <td><div class="font-semibold">¥{{ item.purchaseAmountCny }}</div><div class="text-xs text-muted-foreground">上限 {{ formatUsdQuota(item.purchasedCredit) }} · {{ item.snapshot.multiplier }}</div></td>
        <td>{{ getApiDeliveryModeLabel(item.selectedDeliveryMode) }}</td>
        <td>
          <Badge :variant="activeStatuses.includes(item.status) ? 'default' : 'secondary'">{{ getApiStatusLabel(item.status) }}</Badge>
          <div class="mt-1 text-xs text-muted-foreground">联系方式已按参与方权限展示</div>
        </td>
        <td class="text-xs text-muted-foreground">{{ getApiIntentNextAction(item, 'buyer') }}</td>
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
