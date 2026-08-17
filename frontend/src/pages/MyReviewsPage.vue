<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Clock3, Inbox, Send, Star } from 'lucide-vue-next'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import ReviewDialog from '@/components/review/ReviewDialog.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import type { ReviewCenterRow } from '@/lib/api'
import { apiOrderCommercialOutcomeLabels } from '@/lib/apiOrderDispute'
import { useReviewCenterPage, useReviewCenterRows } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const activeStatus = ref('待评价')
const querySelectionApplied = ref(false)
const { data } = useReviewCenterRows()

const center = computed(() => data.value ?? { items: [], presetTags: [] })
const selectedRow = computed(() => {
  const transactionId = String(route.query.transactionId ?? '')
  const transactionType = String(route.query.transactionType ?? '')
  const reviewDirection = String(route.query.reviewDirection ?? '')
  if (!transactionId) return null
  const matches = center.value.items.filter(item => (
    item.transactionId === transactionId
    && (!transactionType || item.transactionType === transactionType)
  ))
  if (reviewDirection) return matches.find(item => item.direction === reviewDirection) ?? null
  return matches.find(item => item.direction === 'pending') ?? matches.find(item => item.direction === 'sent') ?? matches[0] ?? null
})
const direction = computed<ReviewCenterRow['direction'] | undefined>(() => {
  if (activeStatus.value === '待评价') return 'pending'
  if (activeStatus.value === '我发出的') return 'sent'
  if (activeStatus.value === '我收到的') return 'received'
  return undefined
})
const pagination = useCursorPagination([activeStatus])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useReviewCenterPage(direction, pageRequest)
const rows = computed(() => pageQuery.data.value?.items ?? [])
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const error = pageQuery.error
const refetch = pageQuery.refetch

watch(
  () => center.value.items,
  (items) => {
    if (querySelectionApplied.value || items.length === 0) return
    querySelectionApplied.value = true
    const transactionId = String(route.query.transactionId ?? '')
    const transactionType = String(route.query.transactionType ?? '')
    if (!transactionId) return
    const row = selectedRow.value
    if (!row) return
    activeStatus.value = row.direction === 'pending' ? '待评价' : row.direction === 'received' ? '我收到的' : '我发出的'
  },
  { immediate: true },
)

function openReview(row: ReviewCenterRow) {
  router.push({ query: { ...route.query, transactionType: row.transactionType, transactionId: row.transactionId, reviewDirection: row.direction } })
}

function setReviewOpen(open: boolean) {
  if (open) return
  const query = { ...route.query }
  delete query.transactionType
  delete query.transactionId
  delete query.reviewDirection
  router.replace({ query })
}

function statusLabel(row: ReviewCenterRow) {
  if (row.status === 'reviewable') return '可评价'
  if (row.status === 'paused') return '纠纷处理中'
  if (row.status === 'expired') return '已截止'
  if (row.status === 'sealed') return '待公开'
  if (row.status === 'published') return '已公开'
  return '已移除'
}

function directionLabel(direction: ReviewCenterRow['direction']) {
  if (direction === 'pending') return '待评价'
  if (direction === 'sent') return '我发出的'
  return '我收到的'
}

function transactionLabel(transactionType: ReviewCenterRow['transactionType']) {
  return transactionType === 'api_order' ? 'API 订单' : '拼车'
}

function actionLabel(row: ReviewCenterRow) {
  if (row.canCreate) return '去评价'
  if (row.canEdit) return '修改'
  if (row.direction === 'received' && row.visibility === 'sealed') return '查看状态'
  return '查看'
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="评价中心" description="管理已完成 API 订单的双向评价。" />
    <StatusTabs v-model="activeStatus" :items="['待评价', '我发出的', '我收到的', '全部']" />

    <Alert>
      <Clock3 class="h-4 w-4" />
      <AlertTitle>评价公开规则</AlertTitle>
      <AlertDescription>
        评价公开前对对方不可见。双方都提交后立即公开；如只有一方提交，将在评价截止后公开。公开后不可修改。
      </AlertDescription>
    </Alert>

    <ReviewDialog :open="Boolean(selectedRow)" :row="selectedRow" @update:open="setReviewOpen" @saved="setReviewOpen(false)" />

    <SkeletonTable v-if="isLoading" :rows="5" :columns="6" />
    <ErrorState v-else-if="error" description="评价中心暂时无法加载。" @retry="refetch()" />
    <EmptyState
      v-else-if="rows.length === 0"
      :title="activeStatus === '我收到的' ? '暂无收到的评价' : '当前没有符合条件的评价记录'"
      description="已完成且仍在评价窗口内的交易会显示在这里。"
    />
    <SoftTable v-else class="[&_table]:min-w-[760px]" :columns="['交易', '对方', '方向', '状态', '截止时间', '操作']">
      <tr v-for="item in rows" :key="item.id">
        <td>
          <div class="font-medium">{{ item.target }}</div>
          <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <Badge variant="outline">{{ transactionLabel(item.transactionType) }}</Badge>
            <Badge v-if="item.transactionType === 'api_order' && item.commercialOutcome" variant="secondary">
              {{ apiOrderCommercialOutcomeLabels[item.commercialOutcome === 'legacy_fulfillment' ? 'normal_fulfillment' : item.commercialOutcome] }}
            </Badge>
            <span>关联交易</span>
            <ShortId :value="item.transactionId" :prefix="item.transactionType === 'api_order' ? 'API' : 'RIDE'" />
          </div>
        </td>
        <td>
          <div>{{ item.counterparty }}</div>
          <div class="text-xs text-muted-foreground">@{{ item.counterpartyUsername }}</div>
        </td>
        <td>
          <span class="inline-flex items-center gap-1.5 text-sm">
            <Inbox v-if="item.direction === 'received'" class="h-4 w-4 text-muted-foreground" />
            <Send v-else-if="item.direction === 'sent'" class="h-4 w-4 text-muted-foreground" />
            <Star v-else class="h-4 w-4 text-muted-foreground" />
            {{ directionLabel(item.direction) }}
          </span>
        </td>
        <td>
          <Badge :variant="item.status === 'reviewable' ? 'default' : item.status === 'removed' ? 'destructive' : 'secondary'">
            {{ statusLabel(item) }}
          </Badge>
        </td>
        <td>
          <LocalTime :value="item.reviewDeadlineAt" />
        </td>
        <td>
          <Button size="sm" :variant="item.canCreate || item.canEdit ? 'default' : 'outline'" @click="openReview(item)">
            {{ actionLabel(item) }}
          </Button>
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
