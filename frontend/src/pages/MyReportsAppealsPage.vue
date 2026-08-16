<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { FileWarning, Gavel, Scale, SendHorizontal, ShieldAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import CompactStats from '@/components/market/CompactStats.vue'
import DisputeEvidencePicker from '@/components/api-order/DisputeEvidencePicker.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import WorkspaceSectionTabs from '@/components/workspace/WorkspaceSectionTabs.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { backendErrorMessage } from '@/lib/backendClient'
import type { DisputeEvidenceAsset } from '@/lib/disputeEvidenceBackend'
import {
  buildModerationRecords,
  canCreateAppeal,
  createAppealPayload,
  filterModerationRecords,
  hasPendingAppeal,
  moderationRecordKindLabel,
  moderationTargetTypeLabel,
  reportReasonLabel,
  type ModerationRecord,
  type ModerationRecordFilter,
  type ModerationRecordKind,
} from '@/lib/reportCenter'
import {
  useCreateAppealMutation,
  useMyAppealsQuery,
  useMyDisputesQuery,
  useMyReportsQuery,
  useSubmitInfoSupplementMutation,
} from '@/queries/useReportQueries'

const route = useRoute()
const router = useRouter()
const reportsQuery = useMyReportsQuery()
const disputesQuery = useMyDisputesQuery()
const appealsQuery = useMyAppealsQuery()
const createAppealMutation = useCreateAppealMutation()
const submitSupplementMutation = useSubmitInfoSupplementMutation()

const filterLabels: Array<{ label: string, value: ModerationRecordFilter }> = [
  { label: '全部', value: 'all' },
  { label: '我的举报', value: 'report' },
  { label: '相关纠纷', value: 'dispute' },
  { label: '我的申诉', value: 'appeal' },
]
const activeFilter = ref<ModerationRecordFilter>('all')
const appealDialogOpen = ref(false)
const appealTarget = ref<ModerationRecord | null>(null)
const appealForm = reactive({ title: '', statement: '' })
const appealEvidence = ref<DisputeEvidenceAsset[]>([])
const supplementDialogOpen = ref(false)
const supplementTarget = ref<ModerationRecord | null>(null)
const supplementBody = ref('')
const supplementEvidence = ref<DisputeEvidenceAsset[]>([])

const records = computed(() => buildModerationRecords(
  reportsQuery.data.value ?? [],
  disputesQuery.data.value ?? [],
  appealsQuery.data.value ?? [],
))
const visibleRecords = computed(() => filterModerationRecords(records.value, activeFilter.value))
const requestedKind = computed(() => String(route.params.kind ?? ''))
const requestedId = computed(() => String(route.params.id ?? ''))
const hasDetailRequest = computed(() => Boolean(requestedKind.value && requestedId.value))
const requestedRecord = computed(() => records.value.find(item => item.kind === requestedKind.value && item.id === requestedId.value) ?? null)
const selectedRecord = computed(() => {
  if (hasDetailRequest.value) return requestedRecord.value
  return visibleRecords.value[0] ?? null
})
const detailUnavailable = computed(() => hasDetailRequest.value && requestedRecord.value === null)
const isLoading = computed(() => reportsQuery.isPending.value || disputesQuery.isPending.value || appealsQuery.isPending.value)
const hasError = computed(() => reportsQuery.isError.value || disputesQuery.isError.value || appealsQuery.isError.value)
const stats = computed(() => [
  { label: '我的举报', value: reportsQuery.data.value?.length ?? 0 },
  { label: '相关纠纷', value: disputesQuery.data.value?.length ?? 0 },
  { label: '处理中', value: records.value.filter(item => ['submitted', 'triaged', 'negotiating', 'pending_seller_response', 'pending_applicant_decision', 'voluntary_fulfillment', 'open', 'waiting_info'].includes(item.status)).length },
  { label: '我的申诉', value: appealsQuery.data.value?.length ?? 0 },
])

watch(requestedKind, value => {
  activeFilter.value = ['report', 'dispute', 'appeal'].includes(value)
    ? value as ModerationRecordKind
    : 'all'
}, { immediate: true })

function updateFilter(label: string) {
  const item = filterLabels.find(option => option.label === label)
  if (!item) return
  activeFilter.value = item.value
  router.replace('/my/reports')
}

function selectedFilterLabel() {
  return filterLabels.find(item => item.value === activeFilter.value)?.label ?? '全部'
}

function retryAll() {
  reportsQuery.refetch()
  disputesQuery.refetch()
  appealsQuery.refetch()
}

function recordRoute(record: ModerationRecord) {
  return `/my/reports/${record.kind}/${record.id}`
}

function statusVariant(record: ModerationRecord) {
  if (record.status === 'rejected') return 'destructive'
  if (['approved', 'resolved'].includes(record.status)) return 'verified'
  if (['needs_info', 'waiting_info'].includes(record.status)) return 'default'
  return 'secondary'
}

function recordSubtitle(record: ModerationRecord) {
  if (record.kind === 'report') return `${reportReasonLabel(record.source.reasonCode)} · ${record.targetLabel}`
  if (record.kind === 'dispute') return `${moderationTargetTypeLabel(record.source.targetType)} · ${record.targetLabel}`
  return `${record.source.disputeId ? '关联纠纷' : '关联举报'} · ${record.targetLabel}`
}

function openAppeal(record: ModerationRecord) {
  if (!canCreateAppeal(record, appealsQuery.data.value ?? [])) return
  appealTarget.value = record
  appealForm.title = `申诉：${record.title}`.slice(0, 80)
  appealForm.statement = ''
  appealEvidence.value = []
  appealDialogOpen.value = true
}

function submitAppeal() {
  const target = appealTarget.value
  const title = appealForm.title.trim()
  const statement = appealForm.statement.trim()
  if (!target) return
  if (title.length < 2) {
    toast.warning('请填写至少 2 个字符的申诉标题。')
    return
  }
  if (statement.length < 4) {
    toast.warning('请填写至少 4 个字符的脱敏申诉说明。')
    return
  }
  const payload = createAppealPayload(target, title, statement)
  if (!payload) return
  createAppealMutation.mutate({ ...payload, evidenceAssetIds: appealEvidence.value.map(item => item.id) }, {
    onSuccess: appeal => {
      appealDialogOpen.value = false
      appealTarget.value = null
      appealForm.title = ''
      appealForm.statement = ''
      appealEvidence.value = []
      toast.success('申诉已提交，处理状态已更新。')
      router.push(`/my/reports/appeal/${appeal.id}`)
    },
    onError: error => toast.error(backendErrorMessage(error, '申诉提交失败，请稍后重试。')),
  })
}

function canSupplement(record: ModerationRecord) {
  return record.kind !== 'appeal' && record.source.canSupplement === true && Boolean(record.source.openInfoRequestId)
}

function openSupplement(record: ModerationRecord) {
  if (!canSupplement(record)) return
  supplementTarget.value = record
  supplementBody.value = ''
  supplementEvidence.value = []
  supplementDialogOpen.value = true
}

function submitSupplement() {
  const target = supplementTarget.value
  const body = supplementBody.value.trim()
  if (!target || target.kind === 'appeal' || !target.source.openInfoRequestId) return
  if (body.length < 4) {
    toast.warning('请填写至少 4 个字符的脱敏补充说明。')
    return
  }
  submitSupplementMutation.mutate({
    entityType: target.kind,
    entityId: target.id,
    openInfoRequestId: target.source.openInfoRequestId,
    body,
    evidenceAssetIds: supplementEvidence.value.map(item => item.id),
  }, {
    onSuccess: () => {
      supplementDialogOpen.value = false
      supplementTarget.value = null
      supplementBody.value = ''
      supplementEvidence.value = []
      toast.success('补充材料已提交，等待平台继续处理。')
    },
    onError: error => toast.error(backendErrorMessage(error, '补充材料提交失败，请刷新后重试。')),
  })
}

function recordAPIOrderId(record: ModerationRecord | null) {
  if (!record) return ''
  if (record.kind === 'dispute' && record.source.targetType === 'api_order') {
    return record.source.apiOrderId ?? record.source.targetId
  }
  if (record.kind === 'appeal' && record.source.targetType === 'api_order') return record.source.targetId
  return ''
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="支持中心" description="查看举报、纠纷、申诉和问题反馈的处理进展。" />

    <WorkspaceSectionTabs section="support-center" />

    <CompactStats :items="stats" :loading="isLoading" />

    <SkeletonTable v-if="isLoading" :rows="6" :columns="4" />
    <ErrorState v-else-if="hasError" description="举报与申诉记录暂时无法加载。" @retry="retryAll" />
    <EmptyState
      v-else-if="records.length === 0"
      title="暂无举报或申诉记录"
      description="从交易详情或公开主页提交举报后，处理状态会显示在这里。"
    >
      <template #action>
        <Button as-child variant="outline"><RouterLink to="/my/feedback">前往问题反馈</RouterLink></Button>
      </template>
    </EmptyState>
    <div v-else class="grid gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(360px,0.75fr)]">
      <Card class="p-4">
        <div class="mb-4 flex items-start justify-between gap-3">
          <div>
            <h2 class="font-semibold">处理记录</h2>
            <p class="mt-1 text-sm text-muted-foreground">状态和公开处理结果来自平台记录。</p>
          </div>
          <Badge variant="secondary">{{ visibleRecords.length }}</Badge>
        </div>
        <StatusTabs :items="filterLabels.map(item => item.label)" :model-value="selectedFilterLabel()" @update:model-value="updateFilter" />
        <div class="grid gap-2">
          <RouterLink
            v-for="record in visibleRecords"
            :key="`${record.kind}-${record.id}`"
            :to="recordRoute(record)"
            class="rounded-md border border-border bg-background p-3 transition hover:border-primary/40 hover:bg-accent/40"
            :class="selectedRecord?.id === record.id && selectedRecord?.kind === record.kind ? 'border-primary/50 bg-accent/60' : ''"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex min-w-0 items-center gap-2">
                <FileWarning v-if="record.kind === 'report'" class="h-4 w-4 shrink-0 text-muted-foreground" />
                <Gavel v-else-if="record.kind === 'dispute'" class="h-4 w-4 shrink-0 text-muted-foreground" />
                <Scale v-else class="h-4 w-4 shrink-0 text-muted-foreground" />
                <span class="truncate font-medium">{{ record.title }}</span>
              </div>
              <Badge :variant="statusVariant(record)">{{ record.statusLabel }}</Badge>
            </div>
            <p class="mt-2 truncate text-sm text-muted-foreground">{{ recordSubtitle(record) }}</p>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ moderationRecordKindLabel(record.kind) }} · <ShortId :value="record.id" /> · <LocalTime :value="record.updatedAt" />
            </p>
          </RouterLink>
          <EmptyState v-if="visibleRecords.length === 0" title="当前分类暂无记录" description="切换到其他分类查看已有记录。" />
        </div>
      </Card>

      <Card class="p-4">
        <div v-if="selectedRecord" class="space-y-4">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-xs text-muted-foreground">{{ moderationRecordKindLabel(selectedRecord.kind) }}</div>
              <h2 class="mt-1 text-lg font-semibold">{{ selectedRecord.title }}</h2>
            </div>
            <Badge :variant="statusVariant(selectedRecord)">{{ selectedRecord.statusLabel }}</Badge>
          </div>

          <dl class="grid gap-3 rounded-md border border-border bg-muted/30 p-3 text-sm sm:grid-cols-2">
            <div><dt class="text-xs text-muted-foreground">记录编号</dt><dd class="mt-1 font-medium"><ShortId :value="selectedRecord.id" /></dd></div>
            <div><dt class="text-xs text-muted-foreground">最近更新</dt><dd class="mt-1 font-medium"><LocalTime :value="selectedRecord.updatedAt" /></dd></div>
            <div><dt class="text-xs text-muted-foreground">关联类型</dt><dd class="mt-1 font-medium">{{ moderationTargetTypeLabel(selectedRecord.source.targetType) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">关联内容</dt><dd class="mt-1 font-medium">{{ selectedRecord.targetLabel }}</dd></div>
          </dl>

          <template v-if="selectedRecord.kind === 'report'">
            <div>
              <div class="text-sm font-medium">举报原因</div>
              <p class="mt-2 rounded-md border border-border bg-background p-3 text-sm leading-6">{{ reportReasonLabel(selectedRecord.source.reasonCode) }}</p>
            </div>
            <Alert v-if="selectedRecord.source.status === 'needs_info'">
              <ShieldAlert />
              <AlertTitle>需要进一步信息</AlertTitle>
              <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
                <span>{{ canSupplement(selectedRecord) ? '请提交脱敏事实说明，不要包含联系方式或任何凭据。' : '该案件仍处于补充信息处理阶段，请留意后续通知。' }}</span>
                <Button v-if="canSupplement(selectedRecord)" size="sm" variant="outline" @click="openSupplement(selectedRecord)">补充材料</Button>
              </AlertDescription>
            </Alert>
          </template>

          <template v-else-if="selectedRecord.kind === 'dispute'">
            <div>
              <div class="text-sm font-medium">公开摘要</div>
              <p class="mt-2 rounded-md border border-border bg-background p-3 text-sm leading-6">{{ selectedRecord.source.publicSummary || '暂无公开摘要。' }}</p>
            </div>
            <div>
              <div class="text-sm font-medium">公开处理结果</div>
              <p class="mt-2 rounded-md border border-border bg-background p-3 text-sm leading-6">{{ selectedRecord.source.publicResult || '案件仍在处理中，暂无公开处理结果。' }}</p>
            </div>
            <Alert v-if="selectedRecord.source.status === 'waiting_info'">
              <ShieldAlert />
              <AlertTitle>需要进一步信息</AlertTitle>
              <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
                <span>{{ canSupplement(selectedRecord) ? '请提交脱敏事实说明，不要包含联系方式或任何凭据。' : '该案件仍处于补充信息处理阶段，请留意后续通知。' }}</span>
                <Button v-if="canSupplement(selectedRecord)" size="sm" variant="outline" @click="openSupplement(selectedRecord)">补充材料</Button>
              </AlertDescription>
            </Alert>
          </template>

          <template v-else>
            <div>
              <div class="text-sm font-medium">关联记录</div>
              <p class="mt-2 rounded-md border border-border bg-background p-3 text-sm leading-6">
                {{ selectedRecord.source.disputeId ? '关联纠纷' : '关联举报' }}
                <ShortId :value="selectedRecord.source.disputeId || selectedRecord.source.reportId || selectedRecord.source.targetId" />
              </p>
            </div>
          </template>

          <Alert v-if="hasPendingAppeal(selectedRecord, appealsQuery.data.value ?? [])">
            <Scale />
            <AlertTitle>已有申诉正在复核</AlertTitle>
            <AlertDescription>请在“我的申诉”中查看最新状态，避免重复提交。</AlertDescription>
          </Alert>

          <div v-if="canCreateAppeal(selectedRecord, appealsQuery.data.value ?? [])" class="flex justify-end border-t border-border pt-4">
            <Button variant="outline" @click="openAppeal(selectedRecord)"><Scale class="h-4 w-4" />对处理结果发起申诉</Button>
          </div>
        </div>
        <div v-else class="grid min-h-80 place-items-center text-center text-sm text-muted-foreground">
          {{ detailUnavailable ? '该记录不存在或你无权查看。' : '当前分类暂无可查看记录。' }}
        </div>
      </Card>
    </div>

    <Dialog v-model:open="appealDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>提交申诉</DialogTitle>
          <DialogDescription>申诉将关联当前举报或纠纷。请只填写脱敏事实，不要包含联系方式、密码、API Key、token、cookie 或恢复码。</DialogDescription>
        </DialogHeader>
        <div class="grid gap-4 py-2">
          <label class="grid gap-2 text-sm">
            <span class="font-medium">申诉标题</span>
            <Input v-model="appealForm.title" maxlength="80" />
          </label>
          <label class="grid gap-2 text-sm">
            <span class="font-medium">申诉说明</span>
            <Textarea v-model="appealForm.statement" class="min-h-32" maxlength="1000" placeholder="说明你认为需要复核的事实和理由。" />
          </label>
          <DisputeEvidencePicker v-if="recordAPIOrderId(appealTarget)" v-model="appealEvidence" :order-id="recordAPIOrderId(appealTarget)" visibility="appellant_admin" />
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="createAppealMutation.isPending.value" @click="appealDialogOpen = false">取消</Button>
          <Button :disabled="createAppealMutation.isPending.value" @click="submitAppeal">
            <SendHorizontal class="h-4 w-4" />{{ createAppealMutation.isPending.value ? '提交中' : '提交申诉' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="supplementDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>补充案件材料</DialogTitle>
          <DialogDescription>只填写与案件有关的脱敏事实。不要提交联系方式、密码、API Key、token、session、cookie 或恢复码。</DialogDescription>
        </DialogHeader>
        <label class="grid gap-2 py-2 text-sm">
          <span class="font-medium">补充说明</span>
          <Textarea v-model="supplementBody" class="min-h-36" maxlength="1200" placeholder="说明需要平台继续核对的事实。" />
        </label>
        <DisputeEvidencePicker v-if="recordAPIOrderId(supplementTarget)" v-model="supplementEvidence" :order-id="recordAPIOrderId(supplementTarget)" visibility="submitter_admin" />
        <DialogFooter>
          <Button variant="outline" :disabled="submitSupplementMutation.isPending.value" @click="supplementDialogOpen = false">取消</Button>
          <Button :disabled="submitSupplementMutation.isPending.value" @click="submitSupplement">
            <SendHorizontal class="h-4 w-4" />{{ submitSupplementMutation.isPending.value ? '提交中' : '提交材料' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
