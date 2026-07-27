<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Inbox, LockKeyhole, Send, Star, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import ReputationSummaryCard from '@/components/reputation/ReputationSummaryCard.vue'
import { usePagination } from '@/composables/usePagination'
import { snapshotToSummary } from '@/lib/reputationPresentation'
import type { ReviewCenterRow, SubmitReviewPayload } from '@/lib/api'
import { useReviewCenterRows, useSubmitReviewMutation } from '@/queries/useMarketQueries'
import { usePublicUserReputationQuery } from '@/queries/useReputationQueries'
import type { ReputationRole, ReputationScope } from '@/types/reputation'

const route = useRoute()
const activeStatus = ref('待评价')
const selectedRow = ref<ReviewCenterRow | null>(null)
const querySelectionApplied = ref(false)
const { data, isLoading, error, refetch } = useReviewCenterRows()
const submitReviewMutation = useSubmitReviewMutation()
const form = reactive({
  rating: '5',
  tags: [] as string[],
  note: '',
})

const center = computed(() => data.value ?? { items: [], presetTags: [] })
const counterpartyUsername = computed(() => selectedRow.value?.counterpartyUsername ?? '')
const counterpartyScope = computed<ReputationScope>(() =>
  selectedRow.value?.transactionType === 'api_order' ? 'api' : 'carpool',
)
const counterpartyRole = computed<ReputationRole>(() => {
  if (!selectedRow.value) return 'buyer'
  return selectedRow.value.direction === 'received'
    ? selectedRow.value.reviewerRole
    : selectedRow.value.revieweeRole
})
const counterpartyReputationQuery = usePublicUserReputationQuery(counterpartyUsername, counterpartyScope)
const counterpartyReputation = computed(() => {
  const snapshot = counterpartyReputationQuery.data.value?.reputations.find(item =>
    item.role === counterpartyRole.value && item.scope === counterpartyScope.value,
  )
  return snapshot ? snapshotToSummary(snapshot) : null
})
const rows = computed(() => center.value.items.filter((item) => {
  if (activeStatus.value === '待评价') return item.direction === 'pending'
  if (activeStatus.value === '我发出的') return item.direction === 'sent'
  if (activeStatus.value === '我收到的') return item.direction === 'received'
  return true
}))
const pagination = usePagination(rows)
const selectedCanSubmit = computed(() => Boolean(selectedRow.value?.canCreate || selectedRow.value?.canEdit))
const selectedContentVisible = computed(() => selectedRow.value?.rating !== null && selectedRow.value?.note !== null)
const selectedHiddenMessage = computed(() => {
  if (selectedRow.value?.visibility === 'removed') return '该评价已被管理员移除，内容不再公开。'
  if (selectedRow.value?.status === 'expired') return '评价窗口已截止，不能再提交。'
  return '对方已提交评价，内容将在你提交评价或截止时间到达后显示。'
})

watch(
  () => center.value.items,
  (items) => {
    if (querySelectionApplied.value || items.length === 0) return
    querySelectionApplied.value = true
    const transactionId = String(route.query.transactionId ?? '')
    const transactionType = String(route.query.transactionType ?? '')
    if (!transactionId) return
    const row = items.find(item => (
      item.transactionId === transactionId
      && (!transactionType || item.transactionType === transactionType)
    ))
    if (!row) return
    activeStatus.value = row.direction === 'pending' ? '待评价' : row.direction === 'received' ? '我收到的' : '我发出的'
    openReview(row)
  },
  { immediate: true },
)

function openReview(row: ReviewCenterRow) {
  selectedRow.value = row
  form.rating = String(row.rating ?? 5)
  form.tags = [...row.tags]
  form.note = row.note ?? ''
}

function toggleTag(tag: string, checked: boolean | 'indeterminate') {
  if (checked === true) {
    if (form.tags.includes(tag)) return
    if (form.tags.length >= 5) {
      toast.warning('最多选择 5 个标签。')
      return
    }
    form.tags.push(tag)
    return
  }
  form.tags = form.tags.filter(item => item !== tag)
}

function submitReview() {
  if (!selectedRow.value || !selectedCanSubmit.value) return
  if (!form.note.trim()) {
    toast.warning('请填写评价说明。')
    return
  }
  const payload: SubmitReviewPayload = {
    transactionType: selectedRow.value.transactionType,
    transactionId: selectedRow.value.transactionId,
    operation: selectedRow.value.canEdit ? 'edit' : 'create',
    rating: Number(form.rating),
    tags: [...form.tags],
    note: form.note.trim(),
  }
  submitReviewMutation.mutate(payload, {
    onSuccess: (saved) => {
      toast.success(saved.visibility === 'published' ? '双方评价已公开并冻结。' : '评价已保存，等待对方提交或评价截止。')
      selectedRow.value = null
    },
    onError: mutationError => toast.error(mutationError instanceof Error ? mutationError.message : '评价失败'),
  })
}

function statusLabel(row: ReviewCenterRow) {
  if (row.status === 'reviewable') return '可评价'
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
    <PageTitle title="评价中心" description="管理已完成拼车与 API 订单的双向评价。" />
    <StatusTabs v-model="activeStatus" :items="['待评价', '我发出的', '我收到的', '全部']" />

    <Alert>
      <LockKeyhole class="h-4 w-4" />
      <AlertTitle>双盲评价</AlertTitle>
      <AlertDescription>
        一方提交后内容仅自己可见；双方都提交，或交易完成满 14 天时公开。公开后的评分、标签和说明将冻结。
      </AlertDescription>
    </Alert>

    <Card v-if="selectedRow" class="p-5">
      <div class="relative">
        <div class="min-w-0 pr-10">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-lg font-semibold">{{ selectedRow.target }}</h2>
            <Badge variant="secondary">{{ directionLabel(selectedRow.direction) }}</Badge>
            <Badge variant="outline">{{ statusLabel(selectedRow) }}</Badge>
          </div>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ transactionLabel(selectedRow.transactionType) }} · 对方 {{ selectedRow.counterparty }}
          </p>
        </div>
        <Button class="absolute right-0 top-0" size="icon" variant="ghost" title="关闭评价面板" aria-label="关闭评价面板" @click="selectedRow = null">
          <X class="h-4 w-4" />
        </Button>
      </div>

      <div class="mt-5 border-y border-border py-4">
        <h3 class="text-sm font-semibold">交易对手信誉</h3>
        <p class="mt-1 text-xs text-muted-foreground">
          @{{ selectedRow.counterpartyUsername }} · {{ counterpartyRole === 'buyer' ? '买家信誉' : '卖家信誉' }}
        </p>
        <SkeletonBlock v-if="counterpartyReputationQuery.isLoading.value" class="mt-3" :lines="3" />
        <div v-else-if="counterpartyReputationQuery.error.value" class="mt-3 flex items-center justify-between gap-3 text-sm text-destructive">
          <span>真实信誉摘要加载失败。</span>
          <Button size="sm" variant="outline" @click="counterpartyReputationQuery.refetch()">重试</Button>
        </div>
        <ReputationSummaryCard v-else class="mt-3" :summary="counterpartyReputation" compact :framed="false" />
      </div>

      <div v-if="selectedCanSubmit" class="mt-5 grid gap-5 md:grid-cols-[160px_1fr]">
        <label class="space-y-2">
          <span class="text-sm font-medium">评分</span>
          <Select v-model="form.rating">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择评分" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="rating in [5, 4, 3, 2, 1]" :key="rating" :value="String(rating)">{{ rating }} 分</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <fieldset class="space-y-2">
          <legend class="text-sm font-medium">体验标签（最多 5 个）</legend>
          <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            <label
              v-for="tag in center.presetTags"
              :key="tag"
              class="flex min-h-9 items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
            >
              <Checkbox :model-value="form.tags.includes(tag)" @update:model-value="checked => toggleTag(tag, checked)" />
              <span>{{ tag }}</span>
            </label>
          </div>
        </fieldset>

        <label class="space-y-2 md:col-span-2">
          <span class="text-sm font-medium">评价说明</span>
          <Textarea v-model="form.note" rows="4" placeholder="描述沟通、规则、付款或交付体验；不要填写联系方式或敏感凭据。" />
        </label>
        <div class="flex flex-col gap-2 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between md:col-span-2">
          <span>评价截止：<LocalTime :value="selectedRow.reviewDeadlineAt" /></span>
          <Button :disabled="submitReviewMutation.isPending.value" @click="submitReview">
            <Send class="h-4 w-4" />
            {{ submitReviewMutation.isPending.value ? '提交中' : selectedRow.canEdit ? '保存修改' : '提交评价' }}
          </Button>
        </div>
      </div>

      <div v-else class="mt-5 space-y-4">
        <div v-if="selectedContentVisible" class="space-y-3">
          <div class="flex items-center gap-2">
            <Star v-for="rating in 5" :key="rating" class="h-5 w-5" :class="rating <= (selectedRow.rating ?? 0) ? 'fill-amber-400 text-amber-500' : 'text-muted-foreground/30'" />
            <span class="text-sm font-medium">{{ selectedRow.rating }} 分</span>
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Badge v-for="tag in selectedRow.tags" :key="tag" variant="secondary">{{ tag }}</Badge>
          </div>
          <p class="text-sm leading-6">{{ selectedRow.note }}</p>
        </div>
        <div v-else class="flex items-start gap-3 rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
          <LockKeyhole class="mt-0.5 h-4 w-4 shrink-0" />
          <p>{{ selectedHiddenMessage }}</p>
        </div>
        <div class="flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
          <span>交易完成：<LocalTime :value="selectedRow.completedAt" /></span>
          <span>评价截止：<LocalTime :value="selectedRow.reviewDeadlineAt" /></span>
          <span v-if="selectedRow.visibleAt">公开时间：<LocalTime :value="selectedRow.visibleAt" /></span>
        </div>
      </div>
    </Card>

    <SkeletonTable v-if="isLoading" :rows="5" :columns="6" />
    <ErrorState v-else-if="error" description="评价中心暂时无法加载。" @retry="refetch()" />
    <EmptyState
      v-else-if="rows.length === 0"
      :title="activeStatus === '我收到的' ? '暂无收到的评价' : '当前没有符合条件的评价记录'"
      description="已完成且仍在评价窗口内的交易会显示在这里。"
    />
    <SoftTable v-else class="[&_table]:min-w-[760px]" :columns="['交易', '对方', '方向', '状态', '截止时间', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <div class="font-medium">{{ item.target }}</div>
          <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <Badge variant="outline">{{ transactionLabel(item.transactionType) }}</Badge>
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
