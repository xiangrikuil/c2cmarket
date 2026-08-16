<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { useNow } from '@vueuse/core'
import { CheckCircle2, Clock3, FileText, Gavel, RefreshCw, Scale, ShieldAlert, TriangleAlert, Users } from 'lucide-vue-next'
import AdminDisputeActivityTimeline from '@/components/admin/AdminDisputeActivityTimeline.vue'
import { toast } from 'vue-sonner'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
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
  backendConfirmAdminDisputeRemedyLateness,
  backendExcuseAdminDisputeRemedyLateness,
  backendResolveAdminDispute,
  mapAdminDisputeRow,
  type AdminDisputeDetail,
} from '@/lib/reportBackend'
import {
  useAdminUserReputationQuery,
  useAPIOrderSanctionRecommendationQuery,
  useApplyAPIOrderSanctionMutation,
  useCreateDisputeReputationOutcomeMutation,
} from '@/queries/useReputationQueries'
import type { AdminRow } from '@/lib/api'
import type { DisputeRemedyRequest } from '@/api/generated/openapi'
import type { DisputeReputationOutcome } from '@/types/reputation'
import {
  apiOrderDisputeRemedyLatenessLabels,
  apiOrderDisputeRemedySourceLabels,
  apiOrderDisputeRemedyStatusLabels,
  apiOrderDisputeResolutionLabels,
  type ApiOrderDisputeResolution,
} from '@/lib/apiOrderDispute'

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
const applySanctionMutation = useApplyAPIOrderSanctionMutation()

function negotiationParticipantLabel(userId: string) {
  if (!dispute.value) return userId
  if (userId === dispute.value.primaryUserId) return participantLabel(dispute.value.primaryDisplayName, dispute.value.primaryUsername, userId)
  return participantLabel(dispute.value.counterpartyName, dispute.value.counterpartyUsername, userId)
}
const createdOutcome = ref<DisputeReputationOutcome | null>(null)
const resolutionErrors = ref<Partial<Record<keyof ResolutionForm, string>>>({})
const outcomeErrors = ref<Partial<Record<keyof OutcomeForm, string>>>({})
const submitError = ref('')
const initializedCaseVersion = ref('')
const resolutionSubmitting = ref(false)
const latenessSubmitting = ref(false)
const remedyErrors = ref<Record<string, string>>({})
const latenessDecision = ref<'confirm' | 'excuse'>('confirm')
const latenessReason = ref('')
const latenessConfirmed = ref(false)
const sanctionInternalReason = ref('')
const sanctionConfirmed = ref(false)
const sanctionSubmitError = ref('')

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

const remedyForm = reactive<{
  decision: '' | 'none' | ApiOrderDisputeResolution
  amountCny: string
  responsibleUserId: string
  instructions: string
  dueAt: string
}>({
  decision: '',
  amountCny: '',
  responsibleUserId: '',
  instructions: '',
  dueAt: '',
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
const currentRemedy = computed(() => dispute.value?.remedies?.[0] ?? null)
const hasActiveRemedy = computed(() => currentRemedy.value?.status === 'pending' || currentRemedy.value?.status === 'claimed_fulfilled')
const now = useNow({ interval: 1000 })
const remedyDeadlineReached = computed(() => Boolean(currentRemedy.value?.dueAt) && now.value.getTime() >= new Date(currentRemedy.value!.dueAt).getTime())
const canDecideRemedyLateness = computed(() => dispute.value?.targetType === 'api_order' && Boolean(currentRemedy.value) && (
  currentRemedy.value?.latenessStatus === 'late_unreviewed'
  || (currentRemedy.value?.latenessStatus === 'not_due' && remedyDeadlineReached.value)
))
const hasConfirmedAPIOrderRemedyLateness = computed(() => dispute.value?.targetType === 'api_order' && currentRemedy.value?.latenessStatus === 'late_confirmed')
const sanctionRecommendationEnabled = computed(() => Boolean(
  props.open
  && hasConfirmedAPIOrderRemedyLateness.value
  && existingOutcome.value?.status === 'active'
  && ['responsible', 'shared'].includes(existingOutcome.value.responsibility),
))
const sanctionDisputeID = computed(() => props.disputeId)
const sanctionQuery = useAPIOrderSanctionRecommendationQuery(sanctionDisputeID, sanctionRecommendationEnabled)
const sanctionRecommendation = computed(() => sanctionQuery.data.value ?? null)
const canApplySanction = computed(() => Boolean(
  sanctionRecommendation.value?.eligible
  && !sanctionRecommendation.value.alreadyApplied
  && sanctionRecommendation.value.subjectUserVersion
  && sanctionInternalReason.value.trim().length >= 2
  && sanctionInternalReason.value.trim().length <= 2000
  && sanctionConfirmed.value,
))
const currentStep = computed(() => {
  if (existingOutcome.value) return 'complete'
  if (dispute.value?.status === 'open' || dispute.value?.status === 'waiting_info') return 'resolution'
  if (canDecideRemedyLateness.value) return 'remedy'
  if (dispute.value?.status === 'resolved' && hasActiveRemedy.value) return 'remedy'
  if (dispute.value?.status === 'resolved' || hasConfirmedAPIOrderRemedyLateness.value) return 'outcome'
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
    remedyErrors.value = {}
    latenessDecision.value = 'confirm'
    latenessReason.value = ''
    latenessConfirmed.value = false
    sanctionInternalReason.value = ''
    sanctionConfirmed.value = false
    sanctionSubmitError.value = ''
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
    remedyForm.decision = ''
    remedyForm.amountCny = ''
    remedyForm.responsibleUserId = ''
    remedyForm.instructions = ''
    remedyForm.dueAt = ''
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
  if (value === 'withdrawn') return '已撤回'
  if (value === 'self_resolved') return '线下已解决'
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
  remedyErrors.value = validateRemedyDecision()
  submitError.value = ''
  const publicResultCode = resolutionForm.publicResultCode
  if (Object.keys(resolutionErrors.value).length > 0 || Object.keys(remedyErrors.value).length > 0 || !dispute.value || !publicResultCode) return
  if (evidenceBlocked.value) {
    submitError.value = '关联举报证据尚未完整读取，当前不能提交基础裁决。'
    return
  }
  resolutionSubmitting.value = true
  try {
    const remedy = buildRemedyRequest()
    const updated = await backendResolveAdminDispute({
      disputeId: dispute.value.id,
      expectedVersion: dispute.value.version,
      reason: resolutionForm.internalReason.trim(),
      publicSummary: resolutionForm.publicSummary.trim(),
      publicResultCode,
      publicResult: resolutionForm.publicResult.trim(),
      remedy,
    })
    initializedCaseVersion.value = `${updated.id}:${updated.version}`
    queryClient.setQueryData(disputeQueryKey.value, updated)
    outcomeForm.subjectUserId = updated.subjectUserId ?? updated.counterpartyUserId ?? participants.value[0]?.userId ?? ''
    outcomeForm.publicReason = resolutionForm.publicResult.trim()
    outcomeForm.internalReason = resolutionForm.internalReason.trim()
    outcomeForm.confirmed = false
    emit('updated', mapAdminDisputeRow(updated))
    await retryAudits()
    toast.success(remedy ? '基础裁决已保存，等待责任方履行。' : '基础裁决已保存，纠纷已结案。')
  } catch (error) {
    if (!await recoverSubmissionConflict(error)) {
      submitError.value = errorMessage(error, '基础裁决提交失败。')
    }
  } finally {
    resolutionSubmitting.value = false
  }
}

function validateRemedyDecision() {
  const errors: Record<string, string> = {}
  if (dispute.value?.targetType !== 'api_order') return errors
  if (!remedyForm.decision) {
    errors.decision = '请选择无需履行或具体整改动作。'
    return errors
  }
  if (remedyForm.decision === 'none') return errors
  if (!participants.value.some(item => item.userId === remedyForm.responsibleUserId)) errors.responsibleUserId = '请选择当前纠纷参与方。'
  const instructionsLength = remedyForm.instructions.trim().length
  if (instructionsLength < 2 || instructionsLength > 2000) errors.instructions = '整改说明需为 2 至 2000 个字符。'
  const dueAt = new Date(remedyForm.dueAt)
  if (!remedyForm.dueAt || !Number.isFinite(dueAt.getTime()) || dueAt.getTime() <= Date.now()) errors.dueAt = '整改期限必须晚于当前时间。'
  if (remedyForm.decision === 'partial_refund' && !/^(?:0|[1-9]\d*)(?:\.\d{1,2})?$/.test(remedyForm.amountCny.trim())) {
    errors.amountCny = '请输入最多两位小数的退款金额。'
  } else if (remedyForm.decision === 'partial_refund' && Number(remedyForm.amountCny) <= 0) {
    errors.amountCny = '退款金额必须大于 0。'
  }
  return errors
}

function buildRemedyRequest(): DisputeRemedyRequest | null {
  if (dispute.value?.targetType !== 'api_order' || remedyForm.decision === 'none' || remedyForm.decision === '') return null
  return {
    action: remedyForm.decision,
    ...(remedyForm.decision === 'partial_refund' ? { amountCny: remedyForm.amountCny.trim() } : {}),
    responsibleUserId: remedyForm.responsibleUserId,
    instructions: remedyForm.instructions.trim(),
    dueAt: new Date(remedyForm.dueAt).toISOString(),
  }
}

async function decideRemedyLateness() {
  if (!dispute.value || !currentRemedy.value || latenessSubmitting.value || latenessReason.value.trim().length < 2 || !latenessConfirmed.value) return
  latenessSubmitting.value = true
  submitError.value = ''
  try {
    const action = latenessDecision.value === 'confirm'
      ? backendConfirmAdminDisputeRemedyLateness
      : backendExcuseAdminDisputeRemedyLateness
    const updated = await action({
      disputeId: dispute.value.id,
      expectedVersion: dispute.value.version,
      reason: latenessReason.value.trim(),
    })
    queryClient.setQueryData(disputeQueryKey.value, updated)
    emit('updated', mapAdminDisputeRow(updated))
    latenessConfirmed.value = false
    toast.success(latenessDecision.value === 'confirm' ? '已确认整改迟到，履行进度保持不变。' : '已豁免整改迟到，履行进度保持不变。')
  } catch (error) {
    if (!await recoverSubmissionConflict(error)) submitError.value = errorMessage(error, '整改迟到裁定失败。')
  } finally {
    latenessSubmitting.value = false
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

function sanctionReasonLabel(reasonCode: string) {
  const labels: Record<string, string> = {
    api_order_required: '该纠纷不是 API 订单纠纷。',
    overdue_remedy_required: '最新整改尚未形成管理员确认的逾期事实。',
    active_outcome_required: '当前没有有效的责任认定。',
    responsible_outcome_required: '责任认定不是责任方或共同责任。',
    responsible_seller_required: '整改责任方、责任主体与订单卖家不一致。',
  }
  return labels[reasonCode] ?? '当前事实不满足处罚条件。'
}

async function applySanction() {
  const recommendation = sanctionRecommendation.value
  if (!dispute.value || !recommendation?.subjectUserVersion || !canApplySanction.value) return
  sanctionSubmitError.value = ''
  try {
    await applySanctionMutation.mutateAsync({
      disputeCaseId: dispute.value.id,
      subjectUserId: recommendation.subjectUserId,
      internalReason: sanctionInternalReason.value.trim(),
      expectedUserVersion: recommendation.subjectUserVersion,
    })
    sanctionConfirmed.value = false
    await sanctionQuery.refetch()
    toast.success('API 服务限制已生效。')
  } catch (error) {
    if (error instanceof BackendProblemError && ['VERSION_CONFLICT', 'INVALID_STATE_TRANSITION'].includes(error.code)) {
      sanctionConfirmed.value = false
      await sanctionQuery.refetch()
      sanctionSubmitError.value = error.code === 'VERSION_CONFLICT'
        ? '卖家账号版本已变化，已重新计算处罚建议，请核对后重试。'
        : '处罚依据已变化，已重新读取当前建议。'
      return
    }
    sanctionSubmitError.value = errorMessage(error, 'API 服务限制创建失败。')
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

      <div class="grid shrink-0 grid-cols-3 border-b border-border text-sm">
        <div class="flex items-center justify-center gap-2 px-3 py-3" :class="currentStep === 'resolution' ? 'bg-primary text-primary-foreground' : 'bg-muted/40'">
          <Gavel class="h-4 w-4" />基础裁决
        </div>
        <div class="flex items-center justify-center gap-2 border-l border-border px-3 py-3" :class="currentStep === 'remedy' ? 'bg-primary text-primary-foreground' : 'bg-muted/40'">
          <Clock3 class="h-4 w-4" />整改履行
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

          <AdminDisputeActivityTimeline v-if="dispute.targetType === 'api_order'" :dispute="dispute" />

          <section v-if="dispute.targetType === 'api_order' && currentRemedy" class="space-y-5 border-b border-border py-5">
            <div class="space-y-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <h3 class="text-sm font-semibold">当前整改要求</h3>
                <Badge variant="status">{{ apiOrderDisputeRemedyStatusLabels[currentRemedy.status] }}</Badge>
              </div>
              <dl class="grid gap-3 sm:grid-cols-3">
                <div>
                  <dt class="text-xs text-muted-foreground">动作</dt>
                  <dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeResolutionLabels[currentRemedy.action] }}<span v-if="currentRemedy.amountCny"> · ¥{{ currentRemedy.amountCny }}</span></dd>
                </div>
                <div>
                  <dt class="text-xs text-muted-foreground">责任方</dt>
                  <dd class="mt-1 text-sm font-medium">{{ negotiationParticipantLabel(currentRemedy.responsibleUserId) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-muted-foreground">履行期限</dt>
                  <dd class="mt-1 text-sm font-medium"><LocalTime :value="currentRemedy.dueAt" /></dd>
                </div>
                <div>
                  <dt class="text-xs text-muted-foreground">整改来源</dt>
                  <dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeRemedySourceLabels[currentRemedy.source] }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-muted-foreground">迟到事实</dt>
                  <dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeRemedyLatenessLabels[currentRemedy.latenessStatus] }}</dd>
                </div>
              </dl>
              <div class="border-l-2 border-warning pl-3">
                <p class="text-xs text-muted-foreground">公开整改说明</p>
                <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.instructions }}</p>
              </div>
              <div v-if="currentRemedy.claimNote" class="border-l-2 border-border pl-3">
                <p class="text-xs text-muted-foreground">责任方履行声明</p>
                <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.claimNote }}</p>
              </div>
              <div v-if="currentRemedy.responseNote" class="border-l-2 border-border pl-3">
                <p class="text-xs text-muted-foreground">结果记录</p>
                <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.responseNote }}</p>
              </div>
            </div>
          </section>

          <section class="space-y-4 border-b border-border py-5">
            <div class="flex items-center gap-2">
              <FileText class="h-4 w-4" />
              <h2 class="text-sm font-semibold">关联举报与目标快照</h2>
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
            <div v-if="dispute.targetType === 'api_order'" class="space-y-4 border-t border-border pt-4">
              <div>
                <h3 class="text-sm font-semibold">整改决策</h3>
                <p class="mt-1 text-sm text-muted-foreground">必须明确选择无需履行或创建一条整改要求。责任方声明履行后不会自动结案。</p>
              </div>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="space-y-1.5 text-sm">
                  <span class="font-medium">裁决后的履行动作</span>
                  <Select v-model="remedyForm.decision">
                    <SelectTrigger class="w-full"><SelectValue placeholder="选择履行动作" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">无需履行，直接结案</SelectItem>
                      <SelectItem v-for="(label, value) in apiOrderDisputeResolutionLabels" :key="value" :value="value">{{ label }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <span v-if="remedyErrors.decision" class="text-xs text-destructive">{{ remedyErrors.decision }}</span>
                </label>
                <label v-if="remedyForm.decision && remedyForm.decision !== 'none'" class="space-y-1.5 text-sm">
                  <span class="font-medium">整改责任方</span>
                  <Select v-model="remedyForm.responsibleUserId">
                    <SelectTrigger class="w-full"><SelectValue placeholder="选择责任方" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="participant in participants" :key="participant.userId" :value="participant.userId">
                        {{ participantLabel(participant.name, participant.username, participant.userId) }} · {{ participant.roleLabel }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <span v-if="remedyErrors.responsibleUserId" class="text-xs text-destructive">{{ remedyErrors.responsibleUserId }}</span>
                </label>
                <label v-if="remedyForm.decision === 'partial_refund'" class="space-y-1.5 text-sm">
                  <span class="font-medium">部分退款金额（CNY）</span>
                  <Input v-model="remedyForm.amountCny" inputmode="decimal" placeholder="例如 25.50" />
                  <span v-if="remedyErrors.amountCny" class="text-xs text-destructive">{{ remedyErrors.amountCny }}</span>
                </label>
                <label v-if="remedyForm.decision && remedyForm.decision !== 'none'" class="space-y-1.5 text-sm">
                  <span class="font-medium">履行期限</span>
                  <Input v-model="remedyForm.dueAt" type="datetime-local" />
                  <span v-if="remedyErrors.dueAt" class="text-xs text-destructive">{{ remedyErrors.dueAt }}</span>
                </label>
                <label v-if="remedyForm.decision && remedyForm.decision !== 'none'" class="space-y-1.5 text-sm sm:col-span-2">
                  <span class="font-medium">公开整改说明</span>
                  <Textarea v-model="remedyForm.instructions" rows="3" maxlength="2000" placeholder="说明责任方要完成的退款或继续履约事项，请勿填写凭据。" />
                  <span v-if="remedyErrors.instructions" class="text-xs text-destructive">{{ remedyErrors.instructions }}</span>
                </label>
              </div>
            </div>
            <label class="flex items-start gap-3 border border-border bg-muted/30 p-3 text-sm">
              <Checkbox v-model="resolutionForm.confirmed" class="mt-0.5" />
              <span>我已核对关联对象、参与方和现有证据，公开文案不包含联系方式、凭据或站外支付信息。</span>
            </label>
            <p v-if="resolutionErrors.confirmed" class="text-xs text-destructive">{{ resolutionErrors.confirmed }}</p>
          </section>

          <section v-else-if="currentStep === 'remedy' && currentRemedy" class="space-y-4 pt-5">
            <div>
              <h2 class="text-sm font-semibold">整改履行进度</h2>
              <p class="mt-1 text-sm text-muted-foreground">责任方声明履行不会结案；等待受益方确认、反馈未收到，或确认期中性结束。</p>
            </div>
            <Alert v-if="currentRemedy.status === 'claimed_fulfilled'">
              <Clock3 class="h-4 w-4" />
              <AlertTitle>等待受益方反馈</AlertTitle>
              <AlertDescription>
                责任方已提交履行声明。确认截止时间：
                <LocalTime v-if="currentRemedy.confirmationDueAt" :value="currentRemedy.confirmationDueAt" />
              </AlertDescription>
            </Alert>
            <Alert>
              <Clock3 class="h-4 w-4" />
              <AlertTitle>{{ apiOrderDisputeRemedyLatenessLabels[currentRemedy.latenessStatus] }}</AlertTitle>
              <AlertDescription>
                迟到裁定与履行进度分别记录。确认或豁免迟到都不会取消整改、关闭案件或表示退款已经完成。
              </AlertDescription>
            </Alert>
            <Alert v-if="currentRemedy.status === 'pending' && !remedyDeadlineReached">
              <Clock3 class="h-4 w-4" />
              <AlertTitle>整改期限尚未到达</AlertTitle>
              <AlertDescription>责任方仍可声明履行。平台不能提前裁定迟到。</AlertDescription>
            </Alert>
            <div v-if="canDecideRemedyLateness" class="space-y-3 border-t border-border pt-4">
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">迟到裁定</span>
                <Select v-model="latenessDecision">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="confirm">确认迟到</SelectItem>
                    <SelectItem value="excuse">豁免迟到</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label class="space-y-1.5 text-sm">
                <span class="font-medium">裁定原因</span>
                <Textarea v-model="latenessReason" rows="3" maxlength="800" :placeholder="latenessDecision === 'confirm' ? '记录平台核对到的迟到事实。' : '记录可核实的客观豁免依据。'" />
              </label>
              <label class="flex items-start gap-3 border border-border bg-muted/30 p-3 text-sm">
                <Checkbox v-model="latenessConfirmed" class="mt-0.5" />
                <span>我确认该裁定只记录迟到事实，不改变当前履行进度；只有“确认迟到”可作为后续责任或限制依据。</span>
              </label>
            </div>
          </section>

          <section v-else-if="currentStep === 'outcome'" class="space-y-4 pt-5">
            <div>
              <h2 class="text-sm font-semibold">责任与信誉结果</h2>
              <p class="mt-1 text-sm text-muted-foreground">该结果只记录责任事实；保存后系统会根据当前逾期事实重新计算处罚建议，不会自动创建限制。</p>
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
            <div v-if="sanctionRecommendationEnabled" class="space-y-4 border-t border-border pt-5">
              <div class="flex items-center gap-2">
                <ShieldAlert class="h-5 w-5 text-destructive" />
                <h3 class="text-sm font-semibold">API 订单逾期处罚</h3>
              </div>
              <SkeletonBlock v-if="sanctionQuery.isPending.value" :lines="3" />
              <Alert v-else-if="sanctionQuery.error.value" variant="destructive">
                <TriangleAlert class="h-4 w-4" />
                <AlertTitle>处罚建议读取失败</AlertTitle>
                <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
                  <span>{{ errorMessage(sanctionQuery.error.value, '无法读取当前处罚建议。') }}</span>
                  <Button size="sm" variant="outline" @click="sanctionQuery.refetch()"><RefreshCw class="h-4 w-4" />重试</Button>
                </AlertDescription>
              </Alert>
              <template v-else-if="sanctionRecommendation?.alreadyApplied">
                <Alert>
                  <CheckCircle2 class="h-4 w-4" />
                  <AlertTitle>处罚已应用</AlertTitle>
                  <AlertDescription>
                    当前处罚限制已创建<template v-if="sanctionRecommendation.existingRestriction?.endsAt">，截止 <LocalTime :value="sanctionRecommendation.existingRestriction.endsAt" /></template>。
                  </AlertDescription>
                </Alert>
                <p class="text-sm font-medium">暂停 API 服务新接单、发布和恢复；已成立订单继续付款、交付、完成、售后和纠纷处理。</p>
              </template>
              <template v-else-if="sanctionRecommendation?.eligible">
                <dl class="grid gap-4 sm:grid-cols-3">
                  <div><dt class="text-xs text-muted-foreground">近 180 天确认逾期</dt><dd class="mt-1 text-lg font-semibold">{{ sanctionRecommendation.confirmedBreaches180Days }} 次</dd></div>
                  <div><dt class="text-xs text-muted-foreground">本次限制期限</dt><dd class="mt-1 text-lg font-semibold">{{ sanctionRecommendation.recommendedDays }} 天</dd></div>
                  <div><dt class="text-xs text-muted-foreground">固定范围</dt><dd class="mt-1 text-sm font-semibold">卖家 · API 服务</dd></div>
                </dl>
                <Alert>
                  <ShieldAlert class="h-4 w-4" />
                  <AlertTitle>限制影响</AlertTitle>
                  <AlertDescription>暂停新接单、发布和恢复。已成立订单仍可继续付款、交付、完成、售后和纠纷处理。</AlertDescription>
                </Alert>
                <label class="space-y-1.5 text-sm">
                  <span class="font-medium">内部处罚说明</span>
                  <Textarea v-model="sanctionInternalReason" rows="3" maxlength="2000" placeholder="记录本次处罚判断，不会展示给卖家。" />
                  <span v-if="sanctionInternalReason.trim().length > 0 && sanctionInternalReason.trim().length < 2" class="text-xs text-destructive">内部说明至少 2 个字符。</span>
                </label>
                <label class="flex items-start gap-3 border border-border bg-muted/30 p-3 text-sm">
                  <Checkbox v-model="sanctionConfirmed" class="mt-0.5" />
                  <span>我已核对逾期整改、责任主体和订单卖家一致，并确认按当前 {{ sanctionRecommendation.recommendedDays }} 天档位创建限制。</span>
                </label>
                <Alert v-if="sanctionSubmitError" variant="destructive">
                  <TriangleAlert class="h-4 w-4" />
                  <AlertTitle>处罚未应用</AlertTitle>
                  <AlertDescription>{{ sanctionSubmitError }}</AlertDescription>
                </Alert>
              </template>
              <Alert v-else-if="sanctionRecommendation">
                <TriangleAlert class="h-4 w-4" />
                <AlertTitle>当前不满足处罚条件</AlertTitle>
                <AlertDescription>{{ sanctionReasonLabel(sanctionRecommendation.reasonCode) }}</AlertDescription>
              </Alert>
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
          v-else-if="currentStep === 'remedy' && canDecideRemedyLateness"
          :variant="latenessDecision === 'confirm' ? 'destructive' : 'default'"
          :disabled="latenessReason.trim().length < 2 || !latenessConfirmed || latenessSubmitting"
          @click="decideRemedyLateness"
        >
          <Clock3 class="h-4 w-4" />{{ latenessDecision === 'confirm' ? '确认迟到' : '豁免迟到' }}
        </Button>
        <Button
          v-else-if="currentStep === 'outcome'"
          :disabled="!canSubmitOutcome || createOutcomeMutation.isPending.value"
          @click="submitOutcome"
        >
          <Scale class="h-4 w-4" />保存责任认定
        </Button>
        <Button
          v-else-if="currentStep === 'complete' && sanctionRecommendation?.eligible && !sanctionRecommendation.alreadyApplied"
          variant="destructive"
          :disabled="!canApplySanction || applySanctionMutation.isPending.value"
          @click="applySanction"
        >
          <ShieldAlert class="h-4 w-4" />应用 API 服务限制
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
