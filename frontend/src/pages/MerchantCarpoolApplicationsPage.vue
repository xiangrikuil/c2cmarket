<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  acceptCarpoolApplication,
  getCarpoolApplicationNextAction,
  getCarpoolApplicationStatusLabel,
  rejectCarpoolApplication,
  type CarpoolApplication,
} from '@/lib/api'
import { useMerchantCarpoolApplications } from '@/queries/useMarketQueries'

const activeStatus = ref('待处理')
const queryClient = useQueryClient()
const { data: applications } = useMerchantCarpoolApplications({ sort: 'default_owner' })
const actionId = ref('')

const statusGroups: Record<string, CarpoolApplication['status'][]> = {
  待处理: ['pending_owner'],
  待联系: ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation'],
  服务中: ['active'],
  待完成: ['pending_completion'],
  已完成: ['completed'],
  已拒绝取消: ['rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'],
  纠纷: ['disputed'],
}

const rows = computed(() => {
  const all = applications.value ?? []
  return all.filter(item => statusGroups[activeStatus.value]?.includes(item.status))
})

const pagination = usePagination(rows)
const pendingCount = computed(() => (applications.value ?? []).filter(item => item.status === 'pending_owner').length)
const reservedCount = computed(() => (applications.value ?? []).filter(item => ['accepted_reserved', 'waiting_contact', 'contacted', 'joined_pending_confirmation'].includes(item.status)).length)
const disputeCount = computed(() => (applications.value ?? []).filter(item => item.status === 'disputed').length)

async function refreshApplications() {
  await queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['carpools'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-application'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-application-events'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
}

async function runOwnerApplicationAction(applicationId: string, action: () => Promise<unknown>, successMessage: string) {
  actionId.value = applicationId
  try {
    await action()
    await refreshApplications()
    toast.success(successMessage)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    actionId.value = ''
  }
}

function acceptApplication(item: CarpoolApplication) {
  runOwnerApplicationAction(item.id, () => acceptCarpoolApplication(item.id), '已接受申请，并预留 1 个席位 30 分钟。')
}

function rejectApplication(item: CarpoolApplication) {
  runOwnerApplicationAction(item.id, () => rejectCarpoolApplication(item.id, '车主暂不接受该申请'), '已拒绝申请。')
}
</script>

<template>
  <div>
    <PageTitle title="上车申请" description="车主处理申请、席位预留、站外联系、上车确认和完成状态。" />
    <div class="mb-5 grid gap-3 md:grid-cols-3">
      <div class="rounded-lg border border-border bg-card p-4">
        <div class="text-sm text-muted-foreground">待处理申请</div>
        <div class="mt-2 text-2xl font-semibold">{{ pendingCount }}</div>
      </div>
      <div class="rounded-lg border border-border bg-card p-4">
        <div class="text-sm text-muted-foreground">预留 / 联系中</div>
        <div class="mt-2 text-2xl font-semibold">{{ reservedCount }}</div>
      </div>
      <div class="rounded-lg border border-border bg-card p-4">
        <div class="text-sm text-muted-foreground">纠纷</div>
        <div class="mt-2 text-2xl font-semibold">{{ disputeCount }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeStatus" :items="['待处理', '待联系', '服务中', '待完成', '已完成', '已拒绝取消', '纠纷']" />
    <SoftTable :columns="['申请人', '车源', '价格快照', '用户摘要', '状态', '申请时间', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <RouterLink :to="`/u/${item.applicantUsername}`" class="font-medium hover:underline">{{ item.applicantUsername }}</RouterLink>
          <div class="text-xs text-muted-foreground">{{ item.applicantStats.linuxdoBound ? '已绑定 linux.do' : '未绑定 linux.do' }} · 信任等级{{ item.applicantStats.trustLevel }}</div>
        </td>
        <td><div class="font-medium">{{ item.snapshot.productName }}</div><div class="text-xs text-muted-foreground">{{ item.snapshot.regionName }}</div></td>
        <td class="font-semibold">{{ item.snapshot.priceLabel }} ¥{{ item.snapshot.monthlyPriceCny }}</td>
        <td class="text-xs text-muted-foreground">近30天完成 {{ item.applicantStats.completed30d }} · 买家责任取消 {{ item.applicantStats.buyerResponsibleCancellations }} · 纠纷 {{ item.applicantStats.unresolvedDisputes }}</td>
        <td><Badge :variant="item.status === 'pending_owner' ? 'default' : 'secondary'">{{ getCarpoolApplicationStatusLabel(item.status) }}</Badge></td>
        <td class="text-muted-foreground">{{ item.createdAt }}</td>
        <td>
          <div class="flex flex-wrap gap-2">
            <template v-if="item.status === 'pending_owner'">
              <Button size="sm" :disabled="actionId === item.id" @click="acceptApplication(item)">接受</Button>
              <Button size="sm" variant="outline" :disabled="actionId === item.id" @click="rejectApplication(item)">拒绝</Button>
            </template>
            <RouterLink :to="`/merchant/carpool-applications/${item.id}`">
              <Button size="sm" :variant="item.status === 'pending_owner' ? 'ghost' : 'outline'">{{ item.status === 'pending_owner' ? '详情' : getCarpoolApplicationNextAction(item, 'owner') }}</Button>
            </RouterLink>
          </div>
        </td>
      </tr>
      <tr v-if="rows.length === 0">
        <td colspan="7" class="py-10 text-center text-sm text-muted-foreground">当前筛选下暂无申请。</td>
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
