<script setup lang="ts">
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { useMerchantCarpoolApplications, usePagedMyCarpools } from '@/queries/useMarketQueries'
import { getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { formatDailyWeeklyQuota } from '@/lib/quota'
import type { OwnerCarpoolView } from '@/lib/api'

const tabItems = ['组队进行中', '服务中', '历史车队', '草稿与待修改']
const activeTab = ref(tabItems[0]!)
const ownerView = computed<OwnerCarpoolView>(() => {
	if (activeTab.value === '服务中') return 'serving'
	if (activeTab.value === '历史车队') return 'history'
	if (activeTab.value === '草稿与待修改') return 'needs_edit'
	return 'recruiting'
})
const pagination = useCursorPagination([activeTab])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = usePagedMyCarpools(ownerView, pageRequest)
const { data: applications } = useMerchantCarpoolApplications({ sort: 'default_owner' })
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const emptyCopy = computed(() => {
	if (ownerView.value === 'serving') return { title: '暂无服务中的车队', description: '成员确认上车后，车队会显示在这里。' }
	if (ownerView.value === 'history') return { title: '暂无历史车队', description: '已拒绝或移除的车源会显示在这里。' }
	if (ownerView.value === 'needs_edit') return { title: '暂无待修改车源', description: '草稿、待审核、需修改和暂停车源会显示在这里。' }
	return { title: '暂无组队中的车源', description: '已发布且尚无服务中成员的车源会显示在这里。' }
})

function editable(item: { backendStatus?: string }) {
	return item.backendStatus === 'draft' || item.backendStatus === 'changes_requested'
}

function ownerStatusLabel(item: { backendStatus?: string, status: string }) {
	const labels: Record<string, string> = {
		draft: '草稿',
		pending_review: '待审核',
		changes_requested: '待修改',
		paused: '已暂停',
		rejected: '已拒绝',
		removed: '已下架',
	}
	return item.backendStatus ? labels[item.backendStatus] ?? item.status : item.status
}

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
    <PageTitle title="我的车源" description="管理组队进行中、服务中、历史车队和待修改车源。" action-text="发布车源" action-to="/carpools/new" />
    <StatusTabs v-model="activeTab" :items="tabItems" />
    <SkeletonTable v-if="isLoading" :rows="5" :columns="7" />
    <EmptyState v-else-if="rows.length === 0" :title="emptyCopy.title" :description="emptyCopy.description"><template #action><RouterLink to="/carpools/new"><Button>发布车源</Button></RouterLink></template></EmptyState>
    <SoftTable v-else :columns="['车源', '价格', '车位', '申请', '状态', '最后确认', '操作']">
      <tr v-for="item in rows" :key="item.id">
        <td>
          <div class="font-medium">{{ item.product }}</div>
          <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <ShortId :value="item.id" prefix="CAR" />
            <span>· {{ item.region }}</span>
          </div>
        </td>
        <td>
          <div class="font-semibold">{{ getPricingDisplay(item).primaryLabel }} ¥{{ getPricingDisplay(item).primaryPrice }}</div>
          <div class="mt-1 text-xs text-muted-foreground">
            {{ formatDailyWeeklyQuota(item) }}
          </div>
        </td>
        <td>已上车 {{ item.seatSummary?.activeMemberCount ?? item.currentConfirmedMembers }}/{{ item.maxMembers }} · 预留 {{ item.seatSummary?.reservedSeatCount ?? 0 }} · 可申请 {{ item.seatSummary?.availableSeats ?? getRemainingSeats(item) }}</td>
        <td class="text-xs text-muted-foreground">
          待处理 {{ applicationCounts(item.id).pending }} · 预留 {{ applicationCounts(item.id).reserved }} · 服务中 {{ applicationCounts(item.id).active }}
        </td>
		<td><Badge :variant="item.status === '可上车' ? 'default' : 'secondary'">{{ ownerStatusLabel(item) }}</Badge></td>
        <td class="text-muted-foreground"><LocalTime :value="item.confirmedAt" /></td>
        <td>
          <div class="flex flex-wrap gap-2">
			<RouterLink v-if="ownerView === 'recruiting' || ownerView === 'serving'" to="/merchant/carpool-applications"><Button size="sm">处理申请</Button></RouterLink>
			<RouterLink v-if="editable(item)" :to="`/my/carpools/${item.id}/edit`"><Button size="sm" variant="outline">编辑</Button></RouterLink>
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
