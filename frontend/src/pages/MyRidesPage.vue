<script setup lang="ts">
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { getCarpoolApplicationNextAction, getCarpoolApplicationStatusLabel, type CarpoolApplication } from '@/lib/api'
import { useMyCarpoolApplications } from '@/queries/useMarketQueries'

const activeStatus = ref('全部')
const { data: applications } = useMyCarpoolApplications({ sort: 'default_buyer' })

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

function statusVariant(status: CarpoolApplication['status']) {
  if (['completed', 'active'].includes(status)) return 'default'
  if (['disputed', 'rejected', 'expired'].includes(status)) return 'secondary'
  return 'outline'
}
</script>

<template>
  <div>
    <PageTitle title="我的上车" description="查看上车申请、席位预留、站外联系、服务中、待完成和评价状态。" action-text="继续找车源" action-to="/carpools" />
    <StatusTabs v-model="activeStatus" :items="['全部', '待车主处理', '待联系', '服务中', '待完成', '已完成', '已取消', '纠纷']" />
    <SoftTable :columns="['车源', '车主', '月费快照', '名额状态', '当前状态', '操作截止 / 更新时间', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <div class="font-medium">{{ item.snapshot.productName }}</div>
          <div class="text-xs text-muted-foreground">{{ item.snapshot.regionName }} · {{ item.snapshot.warrantyText }}</div>
        </td>
        <td>{{ item.ownerUsername }} · 信任等级{{ item.snapshot.ownerTrustLevel }}</td>
        <td class="font-semibold">{{ item.snapshot.priceLabel }} ¥{{ item.snapshot.monthlyPriceCny }}</td>
        <td>{{ item.status === 'pending_owner' ? '未占用名额' : item.reservedUntil ? '席位已预留' : item.status === 'active' ? '已上车' : '状态记录' }}</td>
        <td><Badge :variant="statusVariant(item.status)">{{ getCarpoolApplicationStatusLabel(item.status) }}</Badge></td>
        <td class="text-muted-foreground">{{ item.reservedUntil ? `预留至 ${item.reservedUntil}` : item.updatedAt }}</td>
        <td>
          <RouterLink :to="`/my/rides/${item.id}`">
            <Button size="sm">{{ getCarpoolApplicationNextAction(item, 'buyer') }}</Button>
          </RouterLink>
        </td>
      </tr>
      <tr v-if="rows.length === 0">
        <td colspan="7" class="py-10 text-center text-sm text-muted-foreground">当前筛选下暂无上车申请。</td>
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
