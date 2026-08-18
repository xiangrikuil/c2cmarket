<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
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
import MerchantCarpoolApplicationsPage from '@/pages/MerchantCarpoolApplicationsPage.vue'
import { getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { formatDailyWeeklyQuota } from '@/lib/quota'
import { updateCarpoolRecruitment, type OwnerCarpoolView } from '@/lib/api'

const tabItems = ['招募中', '已停止', '草稿', '治理下架']
const activeTab = ref(tabItems[0]!)
const route = useRoute()
const queryClient = useQueryClient()
const actionId = ref('')
const ownerView = computed<OwnerCarpoolView>(() => {
	if (activeTab.value === '已停止') return 'serving'
	if (activeTab.value === '治理下架') return 'history'
	if (activeTab.value === '草稿') return 'needs_edit'
	return 'recruiting'
})
const pagination = useCursorPagination([activeTab])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = usePagedMyCarpools(ownerView, pageRequest)
const showApplications = computed(() => route.query.view === 'applications')
const { data: applications } = useMerchantCarpoolApplications({ sort: 'default_owner' }, computed(() => !showApplications.value))
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const pendingApplicationCount = computed(() => (applications.value ?? []).filter(item => item.status === 'pending_owner').length)
const emptyCopy = computed(() => {
	if (ownerView.value === 'serving') return { title: '暂无已停止车源', description: '主动停止或满员停止的车源会显示在这里。' }
	if (ownerView.value === 'history') return { title: '暂无治理下架车源', description: '因目录或治理原因下架的车源会显示在这里。' }
	if (ownerView.value === 'needs_edit') return { title: '暂无草稿', description: '尚未发布的车源会显示在这里。' }
	return { title: '暂无招募中车源', description: '发布并正在接受申请的车源会显示在这里。' }
})

function editable(item: { backendStatus?: string }) {
	return item.backendStatus === 'draft' || item.backendStatus === 'changes_requested'
}

function ownerStatusLabel(item: { backendStatus?: string, status: string }) {
	const labels: Record<string, string> = {
		draft: '草稿',
		pending_review: '待审核',
		changes_requested: '待修改',
		stopped: '已停止',
		rejected: '已拒绝',
		removed: '已下架',
	}
	return item.backendStatus ? labels[item.backendStatus] ?? item.status : item.status
}

function applicationCounts(carpoolId: string) {
  const related = (applications.value ?? []).filter(item => item.carpoolId === carpoolId)
  return {
    pending: related.filter(item => item.status === 'pending_owner').length,
    active: related.filter(item => item.status === 'active').length,
  }
}

async function changeRecruitment(id: string, action: 'stop' | 'resume') {
  actionId.value = id
  try {
    await updateCarpoolRecruitment(id, action)
    await queryClient.invalidateQueries({ queryKey: ['my-carpools'] })
    toast.success(action === 'stop' ? '已停止招募。' : '已恢复招募。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    actionId.value = ''
  }
}

</script>

<template>
  <MerchantCarpoolApplicationsPage v-if="showApplications" />
  <div v-else>
    <PageTitle title="拼车管理" description="管理我的车源、待处理申请和每条车源的成员关系。">
      <template #action>
        <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
          <RouterLink to="/my/carpools?view=applications" class="w-full sm:w-auto"><Button class="w-full sm:w-auto" variant="outline">待处理申请<span v-if="pendingApplicationCount" class="ml-1.5">{{ pendingApplicationCount }}</span></Button></RouterLink>
          <RouterLink to="/carpools/new" class="w-full sm:w-auto"><Button class="w-full sm:w-auto">发布车源</Button></RouterLink>
        </div>
      </template>
    </PageTitle>
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
        <td>平台成员 {{ item.seatSummary?.activeMemberCount ?? item.currentConfirmedMembers }} · 线下已占 {{ item.offlineOccupiedSeats ?? 0 }} · 总名额 {{ item.maxMembers }} · 可申请 {{ item.seatSummary?.availableSeats ?? getRemainingSeats(item) }}</td>
        <td class="text-xs text-muted-foreground">
          待处理 {{ applicationCounts(item.id).pending }} · 有效成员 {{ applicationCounts(item.id).active }}
        </td>
		<td><Badge :variant="item.status === '可上车' ? 'default' : 'secondary'">{{ ownerStatusLabel(item) }}</Badge></td>
        <td class="text-muted-foreground"><LocalTime :value="item.confirmedAt" /></td>
        <td>
          <div class="flex flex-wrap gap-2">
			<RouterLink :to="`/my/carpools/${item.id}/manage`"><Button size="sm">管理车队</Button></RouterLink>
			<RouterLink v-if="ownerView === 'recruiting' || ownerView === 'serving'" to="/my/carpools?view=applications"><Button size="sm" variant="outline">处理申请</Button></RouterLink>
			<Button v-if="item.backendStatus === 'active'" size="sm" variant="outline" :disabled="actionId === item.id" @click="changeRecruitment(item.id, 'stop')">停止招募</Button>
			<Button v-if="item.backendStatus === 'stopped'" size="sm" variant="outline" :disabled="actionId === item.id" @click="changeRecruitment(item.id, 'resume')">恢复招募</Button>
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
