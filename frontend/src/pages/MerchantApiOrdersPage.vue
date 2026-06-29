<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { MessageCircle, RotateCcw } from 'lucide-vue-next'
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
  cancelApiPurchaseIntent,
  closeApiPurchaseIntent,
  formatUsdQuota,
  getApiDeliveryModeLabel,
  getApiIntentNextAction,
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  getApiStatusLabel,
  markApiPurchaseIntentContacted,
  type ApiDeliveryMode,
  type ApiPurchaseIntent,
} from '@/lib/api'
import { useMerchantApiPurchaseIntents } from '@/queries/useMarketQueries'

const queryClient = useQueryClient()
const { data } = useMerchantApiPurchaseIntents({ sort: 'default_merchant' })
const activeTab = ref('全部')
const keyword = ref('')
const deliveryMode = ref<'all' | ApiDeliveryMode>('all')
const serviceFilter = ref('all')
const sortMode = ref<'default' | 'updated' | 'amount'>('default')
const busyId = ref('')

const activeStatuses = ['open', 'contacted']
const closedStatuses = ['buyer_cancelled', 'owner_closed']

const stats = computed(() => {
  const rows = data.value ?? []
  return [
    { label: '新意向', value: rows.filter(item => item.status === 'open').length },
    { label: '已记录联系', value: rows.filter(item => item.status === 'contacted').length },
    { label: '买家取消', value: rows.filter(item => item.status === 'buyer_cancelled').length },
    { label: '商户关闭', value: rows.filter(item => item.status === 'owner_closed').length },
  ]
})

const rows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return [...(data.value ?? [])]
    .filter(item => {
      const tabMatched = activeTab.value === '全部'
        || (activeTab.value === '新意向' && item.status === 'open')
        || (activeTab.value === '已记录联系' && item.status === 'contacted')
        || (activeTab.value === '已关闭' && closedStatuses.includes(item.status))
      return tabMatched
        && (deliveryMode.value === 'all' || item.selectedDeliveryMode === deliveryMode.value)
        && (serviceFilter.value === 'all' || item.serviceId === serviceFilter.value)
        && (!q || [item.id, item.buyer, item.snapshot.serviceTitle].some(value => value.toLowerCase().includes(q)))
    })
    .sort((a, b) => {
      if (sortMode.value === 'amount') return b.purchaseAmountCny - a.purchaseAmountCny
      if (sortMode.value === 'updated') return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      const aAction = a.status === 'open'
      const bAction = b.status === 'open'
      return Number(bAction) - Number(aAction)
        || new Date(a.merchantResponseDeadline ?? '2999-01-01').getTime() - new Date(b.merchantResponseDeadline ?? '2999-01-01').getTime()
        || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    })
})

const pagination = usePagination(rows)
const serviceOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const item of data.value ?? []) seen.set(item.serviceId, item.snapshot.serviceTitle)
  return [...seen.entries()].map(([id, title]) => ({ id, title }))
})

async function refresh() {
  await queryClient.invalidateQueries({ queryKey: ['merchant-api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['my-api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
}

async function runAction(item: ApiPurchaseIntent, action: () => Promise<unknown>, message: string) {
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
    <PageTitle title="商户意向记录" description="处理 API 意向、站外联系状态和历史意向记录；平台不保存、不托管、不展示账号、Key、token、密码。" />

    <div class="grid gap-3 md:grid-cols-4">
      <div v-for="item in stats" :key="item.label" class="rounded-lg border border-border bg-card p-4">
        <div class="text-xs text-muted-foreground">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeTab" :items="['全部', '新意向', '已记录联系', '已关闭']" />

    <div class="grid gap-2 md:grid-cols-[1fr_180px_180px_180px]">
      <Input v-model="keyword" placeholder="搜索意向编号、买家、服务" />
      <select v-model="deliveryMode" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="all">全部接入方式</option>
        <option value="api_key_endpoint">API 请求地址接入说明</option>
        <option value="sub2api_panel_account">Sub2API 面板接入说明</option>
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

    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无商户意向记录。</div>
    <SoftTable v-else :columns="['意向记录', '买家 / 服务', '意向金额 / 额度上限', '接入方式', '状态', '更新', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium">{{ item.id }}</div><div class="text-xs text-muted-foreground">{{ item.createdAt }}</div></td>
        <td>
          <div class="font-medium">{{ item.buyer }}</div>
          <div class="text-xs text-muted-foreground">{{ item.snapshot.serviceTitle }} · {{ getApiMerchantDisplayName(item) }} · {{ getApiMerchantVisibilityLabel(item.snapshot) }}</div>
        </td>
        <td><div class="font-semibold">¥{{ item.purchaseAmountCny }}</div><div class="text-xs text-muted-foreground">上限 {{ formatUsdQuota(item.purchasedCredit) }}</div></td>
        <td>{{ getApiDeliveryModeLabel(item.selectedDeliveryMode) }}</td>
        <td><Badge :variant="activeStatuses.includes(item.status) ? 'default' : 'secondary'">{{ getApiStatusLabel(item.status) }}</Badge></td>
        <td class="text-xs text-muted-foreground">{{ item.updatedAt }}</td>
        <td>
          <div class="flex flex-wrap gap-1">
            <Button v-if="item.status === 'open'" size="sm" :disabled="busyId === item.id" @click="runAction(item, () => markApiPurchaseIntentContacted(item.id), '已记录站外联系。')">
              <MessageCircle class="h-4 w-4" />记录已联系
            </Button>
            <Button v-if="activeStatuses.includes(item.status)" size="sm" variant="outline" :disabled="busyId === item.id" @click="runAction(item, () => closeApiPurchaseIntent(item.id, '商户不再继续处理该购买意向。'), '已关闭该购买意向。')">
              <RotateCcw class="h-4 w-4" />关闭
            </Button>
            <span v-if="closedStatuses.includes(item.status)" class="text-xs text-muted-foreground">{{ getApiIntentNextAction(item, 'merchant') }}</span>
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
