<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { useMerchantCarpoolApplications, useMyCarpools } from '@/queries/useMarketQueries'
import { getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { formatMonthlyQuota } from '@/lib/quota'
import { toast } from 'vue-sonner'

const { data: carpools } = useMyCarpools()
const { data: applications } = useMerchantCarpoolApplications({ sort: 'default_owner' })
const rows = computed(() => carpools.value ?? [])
const pagination = usePagination(rows)

function applicationCounts(carpoolId: string) {
  const related = (applications.value ?? []).filter(item => item.carpoolId === carpoolId)
  return {
    pending: related.filter(item => item.status === 'pending_owner').length,
    reserved: related.filter(item => ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation'].includes(item.status)).length,
    active: related.filter(item => ['active', 'pending_completion'].includes(item.status)).length,
  }
}
</script>

<template>
  <div>
    <PageTitle title="我的开车" description="管理组队进行中、服务中、历史车队和编辑记录。" action-text="导入 / 发布车源" action-to="/carpools/new" />
    <StatusTabs :items="['组队进行中', '服务中', '历史车队', '编辑记录']" />
    <SoftTable :columns="['车源', '价格', '车位', '申请', '状态', '最后确认', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td><div class="font-medium">{{ item.product }}</div><div class="text-xs text-muted-foreground">{{ item.region }} · linux.do 原帖已绑定</div></td>
        <td>
          <div class="font-semibold">{{ getPricingDisplay(item).primaryLabel }} ¥{{ getPricingDisplay(item).primaryPrice }}</div>
          <div class="mt-1 text-xs text-muted-foreground">
            {{ item.serviceMultiplier ? `${item.serviceMultiplier}x` : '倍率待补充' }} · {{ formatMonthlyQuota(item) }}
          </div>
        </td>
        <td>已上车 {{ item.seatSummary?.activeMemberCount ?? item.currentConfirmedMembers }}/{{ item.maxMembers }} · 预留 {{ item.seatSummary?.reservedSeatCount ?? 0 }} · 可申请 {{ item.seatSummary?.availableSeats ?? getRemainingSeats(item) }}</td>
        <td class="text-xs text-muted-foreground">
          待处理 {{ applicationCounts(item.id).pending }} · 预留 {{ applicationCounts(item.id).reserved }} · 服务中 {{ applicationCounts(item.id).active }}
        </td>
        <td><Badge :variant="item.status === '可上车' ? 'default' : 'secondary'">{{ item.status }}</Badge></td>
        <td class="text-muted-foreground">{{ item.confirmedAt }}</td>
        <td>
          <div class="flex flex-wrap gap-2">
            <RouterLink to="/merchant/carpool-applications"><Button size="sm">处理申请</Button></RouterLink>
            <Button size="sm" variant="outline" @click="toast(`正在编辑 ${item.product} 车源。`)">编辑</Button>
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
