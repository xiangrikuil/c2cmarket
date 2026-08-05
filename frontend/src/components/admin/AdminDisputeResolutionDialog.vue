<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, FileText, Gavel, RefreshCw, Scale, TriangleAlert, Users } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  defaultOutcomeReasonCode,
  disputeParticipantOptions,
  findDisputeOutcome,
  normalizeOutcomeSeverity,
  outcomeReasonOptions,
  parseDisputeEvidenceSnapshot,
  publicResultOptions,
  responsibilityLabel,
  responsibilityOptions,
  severityLabel,
  severityOptions,
  validateOutcomeForm,
  validateResolutionForm,
  type OutcomeForm,
  type ResolutionForm,
} from '@/lib/adminDisputeResolution'
import { BackendProblemError } from '@/lib/backendClient'
import {
  backendAdminDisputeDetail,
  backendAdminReportDetail,
  backendResolveAdminDispute,
  mapAdminDisputeRow,
  type AdminDisputeDetail,
} from '@/lib/reportBackend'
import {
  useAdminUserReputationQuery,
  useCreateDisputeReputationOutcomeMutation,
} from '@/queries/useReputationQueries'
import type { AdminRow } from '@/lib/api'
import type { DisputeReputationOutcome } from '@/types/reputation'

const props = defineProps<{
  open: boolean
  disputeId: string
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  updated: [row: AdminRow]
}>()

const queryClient = useQueryClient()
const createOutcomeMutation = useCreateDisputeReputationOutcomeMutation()
const createdOutcome = ref<DisputeReputationOutcome | null>(null)
const resolutionErrors = ref<Partial<Record<keyof ResolutionForm, string>>>({})
const outcomeErrors = ref<Partial<Record<keyof OutcomeForm, string>>>({})
const submitError = ref('')
const initializedCaseVersion = ref('')
const resolutionSubmitting = ref(false)

const dialogOpen = computed({
  get: () => props.open,
  set: value => emit('update:open', value),
})

const disputeQueryKey = computed(() => ['admin-dispute-resolution', props.disputeId] as const)
const disputeQuery = useQuery({
  queryKey: disputeQueryKey,
  queryFn: () => backendAdminDisputeDetail(props.disputeId),
  enabled: computed(() => props.open && Boolean(props.disputeId)),
  staleTime: 0,
  refetchOnWindowFocus: false,
})
const dispute = computed(() => disputeQuery.data.value ?? null)
const reportId = computed(() => dispute.value?.reportId ?? '')
const reportQuery = useQuery({
  queryKey: computed(() => ['admin-dispute-report-evidence', reportId.value] as const),
  queryFn: () => backendAdminReportDetail(reportId.value),
  enabled: computed(() => props.open && Boolean(reportId.value)),
  staleTime: 0,
  refetchOnWindowFocus: false,
})
const report = computed(() => reportQuery.data.value ?? null)
const snapshotResult = computed(() => parseDisputeEvidenceSnapshot(report.value?.targetSnapshotJson))
const snapshot = computed(() => snapshotResult.value.status === 'valid' ? snapshotResult.value.snapshot : null)
const participants = computed(() => dispute.value ? disputeParticipantOptions(dispute.value, snapshot.value) : [])
const firstParticipantId = computed(() => participants.value[0]?.userId ?? '')
const secondParticipantId = computed(() => participants.value[1]?.userId ?? '')
const firstAuditQuery = useAdminUserReputationQuery(firstParticipantId, {
  enabled: computed(() => props.open && Boolean(firstParticipantId.value)),
})
const secondAuditQuery = useAdminUserReputationQuery(secondParticipantId, {
  enabled: computed(() => props.open && Boolean(secondParticipantId.value)),
})

const resolutionForm = reactive<ResolutionForm>({
  publicResultCode: '',
  publicSummary: '',
  publicResult: '',
  internalReason: '',
  confirmed: false,
})

const outcomeForm = reactive<OutcomeForm>({
  subjectUserId: '',
  responsibility: 'undetermined',
  severity: 'none',
  roleScope: 'all',
  reasonCode: 'insufficient_evidence',
  publicReason: '',
  internalReason: '',
  confirmed: false,
})

const auditQueries = computed(() => [
  firstParticipantId.value ? firstAuditQuery : null,
  secondParticipantId.value ? secondAuditQuery : null,
].filter(Boolean))
const auditLoading = computed(() => auditQueries.value.some(query => query?.isPending.value || query?.isFetching.value))
const auditError = computed(() => auditQueries.value.map(query => query?.error.value).find(Boolean) ?? null)
const existingOutcome = computed(() => createdOutcome.value ?? findDisputeOutcome([
  firstAuditQuery.data.value,
  secondAuditQuery.data.value,
], props.disputeId))
const evidenceBlocked = computed(() => Boolean(reportId.value) && (
  reportQuery.isPending.value
  || Boolean(reportQuery.error.value)
  || snapshotResult.value.status !== 'valid'
))
const currentStep = computed(() => {
  if (existingOutcome.value) return 'complete'
  if (dispute.value?.status === 'open' || dispute.value?.status === 'waiting_info') return 'resolution'
  if (dispute.value?.status === 'resolved') return 'outcome'
  return 'closed'
})
const outcomeParticipantsUnavailable = computed(() => currentStep.value === 'outcome' && participants.value.length === 0)
const canSubmitOutcome = computed(() => currentStep.value === 'outcome'
  && !outcomeParticipantsUnavailable.value
  && !auditLoading.value
  && !auditError.value)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    createdOutcome.value = null
    submitError.value = ''
    resolutionErrors.value = {}
    outcomeErrors.value = {}
    initializedCaseVersion.value = ''
  },
)

watch(
  dispute,
  (value) => {
    if (!value || !props.open) return
    const versionKey = `${value.id}:${value.version}`
    if (initializedCaseVersion.value === versionKey) return
    initializedCaseVersion.value = versionKey
    resolutionForm.publicResultCode = ''
    resolutionForm.publicSummary = value.publicSummary
    resolutionForm.publicResult = ''
    resolutionForm.internalReason = ''
    resolutionForm.confirmed = false
    outcomeForm.subjectUserId = value.subjectUserId ?? value.counterpartyUserId ?? ''
    outcomeForm.responsibility = 'undetermined'
    outcomeForm.severity = 'none'
    outcomeForm.reasonCode = defaultOutcomeReasonCode(outcomeForm.responsibility)
    outcomeForm.publicReason = ''
    outcomeForm.internalReason = ''
    outcomeForm.confirmed = false
  },
  { immediate: true },
)

watch(
  () => outcomeForm.subjectUserId,
  (userId) => {
    outcomeForm.roleScope = participants.value.find(item => item.userId === userId)?.roleScope ?? 'all'
  },
)

watch(participants, (items) => {
  const selected = items.find(item => item.userId === outcomeForm.subjectUserId)
  if (!selected && items[0]) outcomeForm.subjectUserId = items[0].userId
  outcomeForm.roleScope = selected?.roleScope ?? items[0]?.roleScope ?? 'all'
})

watch(
  () => outcomeForm.responsibility,
  (responsibility) => {
    outcomeForm.severity = normalizeOutcomeSeverity(responsibility, outcomeForm.severity)
    outcomeForm.reasonCode = defaultOutcomeReasonCode(responsibility)
  },
)

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

function targetTypeLabel(value?: string) {
  const labels: Record<string, string> = {
    public_user: '公开主页',
    contact_snapshot: '联系快照',
    carpool_application: '拼车申请',
    carpool_membership: '拼车成员关系',
    api_purchase_intent: 'API 订单',
    api_order: 'API 订单',
  }
  return value ? labels[value] ?? value : '未知对象'
}

function statusLabel(value?: AdminDisputeDetail['status']) {
  if (value === 'open') return '待裁决'
  if (value === 'waiting_info') return '等待补充'
  if (value === 'resolved') return '已裁决'
  if (value === 'closed') return '已关闭'
  return '加载中'
}

function participantLabel(name: string | undefined, username: string | undefined, userId: string | undefined) {
  if (name?.trim()) return name.trim()
  if (username?.trim()) return `@${username.trim()}`
  return userId ? `参与方 ${userId.slice(0, 8)}` : '未记录'
}

async function retryAll() {
  submitError.value = ''
  await disputeQuery.refetch()
  if (reportId.value) await reportQuery.refetch()
}

async function retryAudits() {
  submitError.value = ''
  await Promise.all(auditQueries.value.map(query => query?.refetch()))
}

async function recoverSubmissionConflict(error: unknown) {
  if (!(error instanceof BackendProblemError) || !['VERSION_CONFLICT', 'INVALID_STATE_TRANSITION'].includes(error.code)) {
    return false
  }
  const refreshed = await disputeQuery.refetch()
  if (refreshed.data) emit('updated', mapAdminDisputeRow(refreshed.data))
  await retryAudits()
  if (error.code === 'VERSION_CONFLICT') {
    submitError.value = '案件已被其他管理员更新，已重新读取最新版本，请核对后重试。'
  } else if (existingOutcome.value) {
    submitError.value = '案件状态已更新，已有责任认定已重新读取。'
  } else {
    submitError.value = '案件状态已改变，已重新读取最新状态，请按当前步骤继续。'
  }
  return true
}

async function submitResolution() {
  if (resolutionSubmitting.value) return
  resolutionErrors.value = validateResolutionForm(resolutionForm)
  submitError.value = ''
  const publicResultCode = resolutionForm.publicResultCode
  if (Object.keys(resolutionErrors.value).length > 0 || !dispute.value || !publicResultCode) return
  if (evidenceBlocked.value) {
    submitError.value = '关联举报证据尚未完整读取，当前不能提交基础裁决。'
    return
  }
  resolutionSubmitting.value = true
  try {
    const updated = await backendResolveAdminDispute({
      disputeId: dispute.value.id,
      expectedVersion: dispute.value.version,
      reason: resolutionForm.internalReason.trim(),
      publicSummary: resolutionForm.publicSummary.trim(),
      publicResultCode,
      publicResult: resolutionForm.publicResult.trim(),
    })
    initializedCaseVersion.value = `${updated.id}:${updated.version}`
    queryClient.setQueryData(disputeQueryKey.value, updated)
    outcomeForm.subjectUserId = updated.subjectUserId ?? updated.counterpartyUserId ?? participants.value[0]?.userId ?? ''
    outcomeForm.publicReason = resolutionForm.publicResult.trim()
    outcomeForm.internalReason = resolutionForm.internalReason.trim()
    outcomeForm.confirmed = false
    emit('updated', mapAdminDisputeRow(updated))
    await retryAudits()
    toast.success('基础裁决已保存，请继续责任认定。')
  } catch (error) {
    if (!await recoverSubmissionConflict(error)) {
      submitError.value = errorMessage(error, '基础裁决提交失败。')
    }
  } finally {
    resolutionSubmitting.value = false
  }
}

async function submitOutcome() {
  if (createOutcomeMutation.isPending.value) return
  outcomeErrors.value = validateOutcomeForm(outcomeForm, participants.value.map(item => item.userId))
  submitError.value = ''
  if (Object.keys(outcomeErrors.value).length > 0 || !dispute.value) return
  if (!canSubmitOutcome.value) {
    submitError.value = auditError.value ? '参与方信誉审计读取失败，当前不能创建责任认定。' : '参与方信誉审计仍在读取。'
    return
  }
  try {
    const outcome = await createOutcomeMutation.mutateAsync({
      disputeCaseId: dispute.value.id,
      subjectUserId: outcomeForm.subjectUserId,
      responsibility: outcomeForm.responsibility,
      severity: outcomeForm.severity,
      roleScope: outcomeForm.roleScope,
      reasonCode: outcomeForm.reasonCode.trim(),
      publicReason: outcomeForm.publicReason.trim(),
      internalReason: outcomeForm.internalReason.trim(),
      expectedVersion: dispute.value.version,
    })
    createdOutcome.value = outcome
    const updated = {
      ...dispute.value,
      subjectUserId: outcome.subjectUserId,
      version: outcome.disputeVersion,
    }
    initializedCaseVersion.value = `${updated.id}:${updated.version}`
    queryClient.setQueryData(disputeQueryKey.value, updated)
    emit('updated', mapAdminDisputeRow(updated))
    toast.success('责任与信誉结果已保存。')
  } catch (error) {
    if (!await recoverSubmissionConflict(error)) {
      submitError.value = errorMessage(error, '责任认定提交失败，基础裁决已保留。')
    }
  }
}
</script>

<template>
  <Dialog v-model:open="dialogOpen">
    <DialogContent class="flex max-h-[92dvh] w-[calc(100%-1rem)] max-w-4xl flex-col gap-0 overflow-hidden p-0 sm:w-full">
      <DialogHeader class="border-b border-border px-5 py-4 pr-12">
        <div class="flex flex-wrap items-center gap-2">
          <DialogTitle>纠纷裁决</DialogTitle>
          <Badge variant="secondary">{{ statusLabel(dispute?.status) }}</Badge>
        </div>
        <DialogDescription>{{ dispute?.targetLabel || '读取案件详情中' }}</DialogDescription>
      </DialogHeader>

      <div class="grid shrink-0 grid-cols-2 border-b border-border text-sm">
        <div class="flex items-center justify-center gap-2 px-3 py-3" :class="currentStep === 'resolution' ? 'bg-primary text-primary-foreground' : 'bg-muted/40'">
          <Gavel class="h-4 w-4" />基础裁决
        </div>
        <div class="flex items-center justify-center gap-2 border-l border-border px-3 py-3" :class="currentStep === 'outcome' || currentStep === 'complete' ? 'bg-primary text-primary-foreground' : 'bg-muted/40'">
          <Scale class="h-4 w-4" />责任认定
        </div>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <SkeletonBlock v-if="disputeQuery.isPending.value" :lines="8" />

        <Alert v-else-if="disputeQuery.error.value" variant="destructive">
          <TriangleAlert class="h-4 w-4" />
          <AlertTitle>案件读取失败</AlertTitle>
          <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
            <span>{{ errorMessage(disputeQuery.error.value, '无法读取最新纠纷详情。') }}</span>
            <Button size="sm" variant="outline" @click="retryAll"><RefreshCw class="h-4 w-4" />重试</Button>
          </AlertDescription>
        </Alert>

        <template v-else-if="dispute">
          <section class="grid gap-4 border-b border-border pb-5 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div class="text-xs text-muted-foreground">关联对象</div>
              <div class="mt-1 text-sm font-medium">{{ targetTypeLabel(dispute.targetType) }} · {{ dispute.targetLabel }}</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">案件状态</div>
              <div class="mt-1 text-sm font-medium">{{ statusLabel(dispute.status) }}</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">打开时间</div>
              <div class="mt-1 text-sm font-medium"><LocalTime :value="dispute.openedAt" /></div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">当前版本</div>
              <div class="mt-1 text-sm font-medium">v{{ dispute.version }}</div>
            </div>
          </section>

          <section v-if="dispute.supplements?.length || report?.supplements?.length" class="space-y-4 border-b border-border py-5">
            <div class="flex items-center gap-2">
              <FileText class="h-4 w-4" />
              <h2 class="text-sm font-semibold">用户补充材料</h2>
            </div>
            <div
              v-for="supplement in [...(report?.supplements ?? []), ...(dispute.supplements ?? [])]"
              :key="supplement.id"
              class="rounded-lg border border-border bg-muted/30 p-4"
            >
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                <span>{{ supplement.submittedByName || supplement.submittedByUsername || supplement.submittedByUserId }}</span>
                <LocalTime :value="supplement.createdAt" />
              </div>
              <p class="mt-3 whitespace-pre-wrap break-words text-sm leading-6">{{ supplement.body }}</p>
            </div>
          </section>

          <section class="space-y-4 border-b border-border py-5">
            <div class="flex items-center gap-2">
              <Users class="h-4 w-4" />
              <h2 class="text-sm font-semibold">参与方</h2>
            </div>
            <div class="grid gap-3 sm:grid-cols-2">
              <div class="border-l-2 border-primary pl-3">
                <div class="text-xs text-muted-foreground">发起 / 主要参与方</div>
                <div class="mt-1 text-sm font-medium">{{ participantLabel(dispute.primaryDisplayName, dispute.primaryUsername, dispute.primaryUserId) }}<span v-if="dispute.primaryUsername"> · @{{ dispute.primaryUsername }}</span></div>
              </div>
              <div class="border-l-2 border-border pl-3">
                <div class="text-xs text-muted-foreground">另一参与方</div>
                <div class="mt-1 text-sm font-medium">{{ participantLabel(dispute.counterpartyName, dispute.counterpartyUsername, dispute.counterpartyUserId) }}<span v-if="dispute.counterpartyUsername"> · @{{ dispute.counterpartyUsername }}</span></div>
              </div>
            </div>
          </section>

          <section class="space-y-4 border-b border-border py-5">
            <div class="flex items-center gap-2">
              <FileText class="h-4 w-4" />
              <h2 class="text-sm font-semibold">现有证据</h2>
            </div>
            <p v-if="!reportId" class="text-sm text-muted-foreground">参与方从关联业务对象直接申请平台介入，无关联举报记录。</p>
            <SkeletonBlock v-else-if="reportQuery.isPending.value" :lines="4" />
            <Alert v-else-if="reportQuery.error.value" variant="destructive">
              <TriangleAlert class="h-4 w-4" />
              <AlertTitle>关联举报读取失败</AlertTitle>
              <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
                <span>{{ errorMessage(reportQuery.error.value, '无法读取关联举报证据。') }}</span>
                <Button size="sm" variant="outline" @click="reportQuery.refetch()"><RefreshCw class="h-4 w-4" />重试</Button>
              </AlertDescription>
            </Alert>
            <template v-else-if="report">
              <div class="grid gap-4 sm:grid-cols-2">
                <div>
                  <div class="text-xs text-muted-foreground">举报说明</div>
                  <div class="mt-1 whitespace-pre-wrap text-sm">{{ report.description || '未填写' }}</div>
                </div>
                <div>
                  <div class="text-xs text-muted-foreground">举报信息</div>
                  <div class="mt-1 text-sm">{{ report.title }} · {{ report.reasonCode }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">{{ report.reporterName || report.reporterUsername }} · <LocalTime :value="report.createdAt" /></div>
                </div>
              </div>
              <Alert v-if="snapshotResult.status === 'invalid'" variant="destructive">
                <TriangleAlert class="h-4 w-4" />
                <AlertTitle>脱敏目标快照不可读取</AlertTitle>
                <AlertDescription>{{ snapshotResult.message }}</AlertDescription>
              </Alert>
              <Alert v-else-if="snapshotResult.status === 'empty'">
                <TriangleAlert class="h-4 w-4" />
                <AlertTitle>脱敏目标快照缺失</AlertTitle>
                <AlertDescription>关联举报未返回目标快照，基础裁决暂不可提交。</AlertDescription>
              </Alert>
              <div v-else-if="snapshotResult.status === 'valid'" class="grid gap-3 border-l-2 border-border pl-3 sm:grid-cols-2 lg:grid-cols-3">
                <div>
                  <div class="text-xs text-muted-foreground">归一业务对象</div>
                  <div class="mt-1 break-all text-sm">{{ snapshotResult.snapshot.canonicalTargetType || '未知' }} · {{ snapshotResult.snapshot.canonicalTargetId || '未知' }}</div>
                </div>
                <div>
                  <div class="text-xs text-muted-foreground">业务状态</div>
                  <div class="mt-1 text-sm">{{ snapshotResult.snapshot.businessStatus || '未记录' }}</div>
                </div>
                <div>
                  <div class="text-xs text-muted-foreground">参与方角色</div>
                  <div class="mt-1 text-sm">{{ snapshotResult.snapshot.participants.map(item => `${item.role}: @${item.username}`).join('；') || '未记录' }}</div>
                </div>
              </div>
            </template>
            <div v-if="dispute.adminReason" class="border-l-2 border-warning pl-3">
              <div class="text-xs text-muted-foreground">现有内部处理原因</div>
              <div class="mt-1 whitespace-pre-wrap text-sm">{{ dispute.adminReason }}</div>
            </div>
          </section>

          <Alert v-if="submitError" variant="destructive" class="mt-5">
            <TriangleAlert class="h-4 w-4" />
            <AlertTitle>提交未完成</AlertTitle>
            <AlertDescription>{{ submitError }}</AlertDescription>
          </Alert>

          <section v-if="currentStep === 'resolution'" class="space-y-4 pt-5">
            <div>
              <h2 class="text-sm font-semibold">基础裁决</h2>
              <p class="mt-1 text-sm text-muted-foreground">公开字段会进入用户可见纠纷记录，内部原因仅管理员可见。</p>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">公开裁决分类</span>
                <Select v-model="resolutionForm.publicResultCode">
                  <SelectTrigger class="w-full"><SelectValue placeholder="选择裁决分类" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in publicResultOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <span v-if="resolutionErrors.publicResultCode" class="text-xs text-destructive">{{ resolutionErrors.publicResultCode }}</span>
              </label>
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">公开摘要</span>
                <Textarea v-model="resolutionForm.publicSummary" rows="2" maxlength="120" />
                <span v-if="resolutionErrors.publicSummary" class="text-xs text-destructive">{{ resolutionErrors.publicSummary }}</span>
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span class="font-medium">公开结果</span>
                <Textarea v-model="resolutionForm.publicResult" rows="2" maxlength="120" />
                <span v-if="resolutionErrors.publicResult" class="text-xs text-destructive">{{ resolutionErrors.publicResult }}</span>
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span class="font-medium">内部裁决原因</span>
                <Textarea v-model="resolutionForm.internalReason" rows="4" maxlength="800" />
                <span v-if="resolutionErrors.internalReason" class="text-xs text-destructive">{{ resolutionErrors.internalReason }}</span>
              </label>
            </div>
            <label class="flex items-start gap-3 border border-border bg-muted/30 p-3 text-sm">
              <Checkbox v-model="resolutionForm.confirmed" class="mt-0.5" />
              <span>我已核对关联对象、参与方和现有证据，公开文案不包含联系方式、凭据或站外支付信息。</span>
            </label>
            <p v-if="resolutionErrors.confirmed" class="text-xs text-destructive">{{ resolutionErrors.confirmed }}</p>
          </section>

          <section v-else-if="currentStep === 'outcome'" class="space-y-4 pt-5">
            <div>
              <h2 class="text-sm font-semibold">责任与信誉结果</h2>
              <p class="mt-1 text-sm text-muted-foreground">该结果记录责任事实；账号限制仍在独立的用户信誉治理入口处理。</p>
            </div>
            <Alert v-if="auditError" variant="destructive">
              <TriangleAlert class="h-4 w-4" />
              <AlertTitle>参与方信誉审计读取失败</AlertTitle>
              <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
                <span>{{ errorMessage(auditError, '无法确认该纠纷是否已有责任认定。') }}</span>
                <Button size="sm" variant="outline" @click="retryAudits"><RefreshCw class="h-4 w-4" />重试</Button>
              </AlertDescription>
            </Alert>
            <Alert v-if="outcomeParticipantsUnavailable" variant="destructive">
              <TriangleAlert class="h-4 w-4" />
              <AlertTitle>没有可用责任主体</AlertTitle>
              <AlertDescription>当前案件缺少可验证的业务参与方，不能创建责任认定；基础裁决已保留。</AlertDescription>
            </Alert>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">责任主体</span>
                <Select v-model="outcomeForm.subjectUserId" :disabled="outcomeParticipantsUnavailable">
                  <SelectTrigger class="w-full"><SelectValue placeholder="选择参与方" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="participant in participants" :key="participant.userId" :value="participant.userId">
                      {{ participantLabel(participant.name, participant.username, participant.userId) }} · {{ participant.roleLabel }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <span v-if="outcomeErrors.subjectUserId" class="text-xs text-destructive">{{ outcomeErrors.subjectUserId }}</span>
              </label>
              <div class="space-y-1.5 text-sm">
                <span class="font-medium">角色范围</span>
                <div class="flex h-9 items-center border border-input bg-muted/30 px-3">
                  {{ participants.find(item => item.userId === outcomeForm.subjectUserId)?.roleLabel || '未确定' }}
                </div>
                <span class="text-xs text-muted-foreground">根据脱敏参与方快照派生；无法可靠判断时使用全部角色。</span>
              </div>
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">责任认定</span>
                <Select v-model="outcomeForm.responsibility">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in responsibilityOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">严重度</span>
                <Select v-model="outcomeForm.severity" :disabled="outcomeForm.responsibility === 'not_responsible' || outcomeForm.responsibility === 'undetermined'">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in severityOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <span v-if="outcomeErrors.severity" class="text-xs text-destructive">{{ outcomeErrors.severity }}</span>
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span class="font-medium">责任原因</span>
                <Select v-model="outcomeForm.reasonCode">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in outcomeReasonOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <span v-if="outcomeErrors.reasonCode" class="text-xs text-destructive">{{ outcomeErrors.reasonCode }}</span>
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span class="font-medium">公开责任说明</span>
                <Textarea v-model="outcomeForm.publicReason" rows="2" maxlength="120" />
                <span v-if="outcomeErrors.publicReason" class="text-xs text-destructive">{{ outcomeErrors.publicReason }}</span>
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span class="font-medium">内部责任说明</span>
                <Textarea v-model="outcomeForm.internalReason" rows="4" maxlength="800" />
                <span v-if="outcomeErrors.internalReason" class="text-xs text-destructive">{{ outcomeErrors.internalReason }}</span>
              </label>
            </div>
            <label class="flex items-start gap-3 border border-border bg-muted/30 p-3 text-sm">
              <Checkbox v-model="outcomeForm.confirmed" class="mt-0.5" />
              <span>我确认该主体是关联业务对象的实际参与方，责任结果将进入版本化信誉治理记录。</span>
            </label>
            <p v-if="outcomeErrors.confirmed" class="text-xs text-destructive">{{ outcomeErrors.confirmed }}</p>
          </section>

          <section v-else-if="currentStep === 'complete' && existingOutcome" class="space-y-4 pt-5">
            <div class="flex items-center gap-2 text-success">
              <CheckCircle2 class="h-5 w-5" />
              <h2 class="text-sm font-semibold">责任认定已记录</h2>
            </div>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div><div class="text-xs text-muted-foreground">状态</div><div class="mt-1 text-sm font-medium">{{ existingOutcome.status === 'active' ? '有效' : '已反转' }}</div></div>
              <div><div class="text-xs text-muted-foreground">责任</div><div class="mt-1 text-sm font-medium">{{ responsibilityLabel(existingOutcome.responsibility) }}</div></div>
              <div><div class="text-xs text-muted-foreground">严重度</div><div class="mt-1 text-sm font-medium">{{ severityLabel(existingOutcome.severity) }}</div></div>
              <div><div class="text-xs text-muted-foreground">决定时间</div><div class="mt-1 text-sm font-medium"><LocalTime :value="existingOutcome.decidedAt" /></div></div>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <div><div class="text-xs text-muted-foreground">公开说明</div><div class="mt-1 whitespace-pre-wrap text-sm">{{ existingOutcome.publicReason }}</div></div>
              <div><div class="text-xs text-muted-foreground">内部说明</div><div class="mt-1 whitespace-pre-wrap text-sm">{{ existingOutcome.internalReason }}</div></div>
            </div>
          </section>

          <Alert v-else class="mt-5">
            <TriangleAlert class="h-4 w-4" />
            <AlertTitle>案件已关闭</AlertTitle>
            <AlertDescription>关闭案件只能查看现有事实，不能补交基础裁决或责任认定。</AlertDescription>
          </Alert>
        </template>
      </div>

      <DialogFooter class="border-t border-border px-5 py-4">
        <Button variant="outline" @click="dialogOpen = false">关闭</Button>
        <Button
          v-if="currentStep === 'resolution'"
          :disabled="evidenceBlocked || disputeQuery.isFetching.value || resolutionSubmitting"
          @click="submitResolution"
        >
          <Gavel class="h-4 w-4" />保存基础裁决
        </Button>
        <Button
          v-else-if="currentStep === 'outcome'"
          :disabled="!canSubmitOutcome || createOutcomeMutation.isPending.value"
          @click="submitOutcome"
        >
          <Scale class="h-4 w-4" />保存责任认定
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
