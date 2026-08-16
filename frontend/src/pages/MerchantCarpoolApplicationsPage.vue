<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import {
  acceptCarpoolApplication,
  getCarpoolApplicationNextAction,
  getCarpoolApplicationStatusLabel,
  rejectCarpoolApplication,
  type CarpoolApplication,
} from '@/lib/api'
import { useMerchantCarpoolApplications, useMerchantCarpoolApplicationsPage } from '@/queries/useMarketQueries'

const activeStatus = ref('待处理')
const queryClient = useQueryClient()
const { data: applications } = useMerchantCarpoolApplications({ sort: 'default_owner' })
const actionId = ref('')

const statusGroups: Record<string, CarpoolApplication['status'][]> = {
  待处理: ['pending_owner'],
  有效成员: ['active'],
  退出或移除: ['cancelled_by_buyer', 'cancelled_by_owner'],
  已拒绝: ['rejected'],
  纠纷: ['disputed'],
}

const pageFilters = computed(() => ({
  status: statusGroups[activeStatus.value],
  sort: 'default_owner' as const,
}))
const pagination = useCursorPagination([activeStatus])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useMerchantCarpoolApplicationsPage(pageFilters, pageRequest)
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const pendingCount = computed(() => (applications.value ?? []).filter(item => item.status === 'pending_owner').length)
const activeCount = computed(() => (applications.value ?? []).filter(item => item.status === 'active').length)
const disputeCount = computed(() => (applications.value ?? []).filter(item => item.status === 'disputed').length)
const stats = computed(() => [
  { label: '待处理', value: pendingCount.value },
  { label: '有效成员', value: activeCount.value },
  { label: '纠纷中', value: disputeCount.value },
])

async function refreshApplications() {
  await queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['carpools'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-application'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-application-events'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
  await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
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
  runOwnerApplicationAction(item.id, () => acceptCarpoolApplication(item.id), '已确认上车，成员关系已生效。')
}

function rejectApplication(item: CarpoolApplication) {
  runOwnerApplicationAction(item.id, () => rejectCarpoolApplication(item.id, '车主暂不接受该申请'), '已拒绝申请。')
}
</script>

<template>
  <div>
    <PageTitle title="上车申请" description="车主确认上车后直接建立成员关系。" />
    <CompactStats class="mb-5" :items="stats" :loading="isLoading" />

    <StatusTabs v-model="activeStatus" :items="['待处理', '有效成员', '退出或移除', '已拒绝', '纠纷']" />
    <SkeletonTable v-if="isLoading" :rows="5" :columns="7" />
    <EmptyState v-else-if="rows.length === 0" title="当前筛选下暂无申请" description="新的上车申请到达后会显示在待处理队列。" />
    <SoftTable v-else animate-rows :columns="['申请人', '车源', '价格快照', '用户摘要', '状态', '申请时间', '操作']">
      <tr v-for="item in rows" :key="item.id">
        <td>
          <RouterLink :to="`/u/${item.applicantUsername}`" class="font-medium hover:underline">{{ item.applicantUsername }}</RouterLink>
          <div class="text-xs text-muted-foreground">{{ item.applicantStats.linuxdoBound === true ? '已绑定 linux.do' : item.applicantStats.linuxdoBound === false ? '未绑定 linux.do' : 'linux.do 绑定暂无数据' }} · {{ item.applicantStats.trustLevel === null ? '信任等级暂无数据' : `信任等级${item.applicantStats.trustLevel}` }}</div>
        </td>
        <td><div class="font-medium">{{ item.snapshot.productName }}</div><div class="text-xs text-muted-foreground"><ShortId :value="item.id" prefix="RIDE" /> · {{ item.snapshot.regionName }}</div></td>
        <td class="font-semibold">{{ item.snapshot.priceLabel }} ¥{{ item.snapshot.monthlyPriceCny }}</td>
        <td class="text-xs text-muted-foreground">近期完成 {{ item.applicantStats.completed30d ?? '暂无数据' }} · 买家责任取消 {{ item.applicantStats.buyerResponsibleCancellations ?? '暂无数据' }} · 纠纷 {{ item.applicantStats.unresolvedDisputes ?? '暂无数据' }}</td>
        <td><Badge :variant="item.status === 'pending_owner' ? 'default' : 'secondary'">{{ getCarpoolApplicationStatusLabel(item.status) }}</Badge></td>
        <td class="text-muted-foreground"><LocalTime :value="item.createdAt" /></td>
        <td>
          <div class="flex flex-wrap gap-2">
            <template v-if="item.status === 'pending_owner'">
              <Button size="sm" :disabled="actionId === item.id" @click="acceptApplication(item)">确认上车</Button>
              <Button size="sm" variant="outline" :disabled="actionId === item.id" @click="rejectApplication(item)">拒绝</Button>
            </template>
            <RouterLink :to="`/merchant/carpool-applications/${item.id}`">
              <Button size="sm" :variant="item.status === 'pending_owner' ? 'ghost' : 'outline'">{{ item.status === 'pending_owner' ? '详情' : getCarpoolApplicationNextAction(item, 'owner') }}</Button>
            </RouterLink>
          </div>
        </td>
      </tr>
      <template #footer>
        <CursorTablePagination
          :page="pagination.page.value"
          :item-count="rows.length"
          :has-next-page="Boolean(pageQuery.data.value?.nextCursor)"
          :loading="pageQuery.isFetching.value"
          @previous="pagination.previous"
          @next="pagination.next(pageQuery.data.value?.nextCursor)"
        />
      </template>
    </SoftTable>
  </div>
</template>
