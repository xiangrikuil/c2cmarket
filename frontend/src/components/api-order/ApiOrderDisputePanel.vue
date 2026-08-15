<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Clock3, FileText, Gavel, Handshake, MessageSquareText, Scale, Send, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import type { DisputeEscalationRequest } from '@/api/generated/openapi'
import DisputeEvidenceGallery from '@/components/api-order/DisputeEvidenceGallery.vue'
import DisputeEvidencePicker from '@/components/api-order/DisputeEvidencePicker.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  apiOrderDisputeIssueLabels,
  apiOrderDisputeRemedyLatenessLabels,
  apiOrderDisputeRemedySourceLabels,
  apiOrderDisputeRemedyStatusLabels,
  apiOrderDisputeResolutionLabels,
  type ApiOrderDisputeResolution,
} from '@/lib/apiOrderDispute'
import { getDisputeCaseStatusLabel } from '@/lib/disputeCase'
import {
  useAppendDisputeMessageMutation,
  useClaimDisputeRemedyMutation,
  useConfirmDisputeSettlementProposalMutation,
  useConfirmDisputeRemedyMutation,
  useContestDisputeRemedyMutation,
  useCreateDisputeSettlementProposalMutation,
  useEscalateDisputeMutation,
  useMyDisputeQuery,
  useRejectDisputeSettlementProposalMutation,
  useSubmitInfoSupplementMutation,
} from '@/queries/useReportQueries'
import type { DisputeEvidenceAsset } from '@/lib/disputeEvidenceBackend'

const props = defineProps<{
  disputeId: string
}>()

const disputeId = computed(() => props.disputeId)
const disputeQuery = useMyDisputeQuery(disputeId)
const dispute = computed(() => disputeQuery.data.value ?? null)
const viewerUserId = computed(() => dispute.value?.viewerUserId ?? '')
const orderId = computed(() => dispute.value?.apiOrderId ?? dispute.value?.targetId ?? '')
const messageBody = ref('')
const messageEvidence = ref<DisputeEvidenceAsset[]>([])
const proposalResolution = ref<ApiOrderDisputeResolution>('full_refund')
const proposalResolutionLabels = Object.fromEntries(
  Object.entries(apiOrderDisputeResolutionLabels).filter(([value]) => value !== 'continue_fulfillment'),
)
const proposalAmount = ref('')
const proposalTerms = ref('')
const proposalResponsibleUserId = ref('')
const proposalDueAt = ref('')
const rejectReason = ref('')
type NegotiationChannel = DisputeEscalationRequest['negotiationChannels'][number]
const negotiationChannelOptions: Array<{ value: NegotiationChannel, label: string }> = [
  { value: 'wechat', label: '微信' },
  { value: 'email', label: '邮箱' },
  { value: 'linux_do', label: 'linux.do' },
  { value: 'in_site', label: '站内留痕' },
  { value: 'other', label: '其他方式' },
]
const negotiationChannels = ref<NegotiationChannel[]>([])
const negotiationEndedConfirmed = ref(false)
const negotiationSummary = ref('')
const requestedPlatformAction = ref('')
const escalationEvidence = ref<DisputeEvidenceAsset[]>([])
const remedyClaimNote = ref('')
const remedyClaimEvidence = ref<DisputeEvidenceAsset[]>([])
const remedyConfirmationNote = ref('')
const remedyContestReason = ref('')
const remedyContestEvidence = ref<DisputeEvidenceAsset[]>([])
const supplementBody = ref('')
const supplementEvidence = ref<DisputeEvidenceAsset[]>([])

const appendMessageMutation = useAppendDisputeMessageMutation()
const createProposalMutation = useCreateDisputeSettlementProposalMutation()
const confirmProposalMutation = useConfirmDisputeSettlementProposalMutation()
const rejectProposalMutation = useRejectDisputeSettlementProposalMutation()
const escalateMutation = useEscalateDisputeMutation()
const claimRemedyMutation = useClaimDisputeRemedyMutation()
const confirmRemedyMutation = useConfirmDisputeRemedyMutation()
const contestRemedyMutation = useContestDisputeRemedyMutation()
const submitSupplementMutation = useSubmitInfoSupplementMutation()

const pendingProposal = computed(() => dispute.value?.settlementProposals?.find(item => item.status === 'pending') ?? null)
const proposalHistory = computed(() => dispute.value?.settlementProposals?.filter(item => item.status !== 'pending') ?? [])
const currentRemedy = computed(() => dispute.value?.remedies?.[0] ?? null)
const remedyHistory = computed(() => dispute.value?.remedies?.slice(1) ?? [])
const isRemedyResponsible = computed(() => currentRemedy.value?.responsibleUserId === viewerUserId.value)
const isRemedyBeneficiary = computed(() => currentRemedy.value?.beneficiaryUserId === viewerUserId.value)
const canClaimRemedy = computed(() => currentRemedy.value?.status === 'pending' && isRemedyResponsible.value)
const canRespondToRemedy = computed(() => currentRemedy.value?.status === 'claimed_fulfilled' && isRemedyBeneficiary.value)
const canNegotiate = computed(() => dispute.value?.status === 'negotiating')
const canMessage = computed(() => dispute.value?.status === 'negotiating')
const pendingFromMe = computed(() => pendingProposal.value?.proposedByUserId === viewerUserId.value)
const proposalNeedsFulfillment = computed(() => proposalResolution.value !== 'other')
const participantOptions = computed(() => {
  if (!dispute.value) return []
  return [
    { userId: dispute.value.primaryUserId, label: dispute.value.primaryDisplayName || dispute.value.primaryUsername },
    { userId: dispute.value.counterpartyUserId, label: dispute.value.counterpartyName || dispute.value.counterpartyUsername },
  ].filter((item): item is { userId: string, label: string } => Boolean(item.userId))
})
const proposalBeneficiaryUserId = computed(() => participantOptions.value.find(item => item.userId !== proposalResponsibleUserId.value)?.userId ?? '')
const proposalDueAtValid = computed(() => {
  const value = new Date(proposalDueAt.value)
  return Boolean(proposalDueAt.value) && Number.isFinite(value.getTime()) && value.getTime() > Date.now()
})
const canCreateProposal = computed(() => Boolean(
  proposalTerms.value.trim()
  && (proposalResolution.value !== 'partial_refund' || proposalAmount.value.trim())
  && (!proposalNeedsFulfillment.value || (proposalResponsibleUserId.value && proposalBeneficiaryUserId.value && proposalDueAtValid.value)),
))
const canEscalate = computed(() => negotiationEndedConfirmed.value
  && negotiationChannels.value.length > 0
  && negotiationSummary.value.trim().length >= 2
  && requestedPlatformAction.value.trim().length >= 2)
const mutationBusy = computed(() => appendMessageMutation.isPending.value
  || createProposalMutation.isPending.value
  || confirmProposalMutation.isPending.value
  || rejectProposalMutation.isPending.value
  || escalateMutation.isPending.value
  || claimRemedyMutation.isPending.value
  || confirmRemedyMutation.isPending.value
  || contestRemedyMutation.isPending.value
  || submitSupplementMutation.isPending.value)

watch(participantOptions, (items) => {
  if (!items.some(item => item.userId === proposalResponsibleUserId.value)) {
    proposalResponsibleUserId.value = items.find(item => item.userId === viewerUserId.value)?.userId ?? items[0]?.userId ?? ''
  }
}, { immediate: true })

function senderLabel(senderUserId: string) {
  return senderUserId === viewerUserId.value ? '我' : '对方'
}

function toggleNegotiationChannel(channel: NegotiationChannel, selected: boolean) {
  negotiationChannels.value = selected
    ? [...new Set([...negotiationChannels.value, channel])]
    : negotiationChannels.value.filter(value => value !== channel)
}

function proposalStatusLabel(status: string) {
  return ({
    pending: '待对方确认',
    accepted: '双方已确认',
    rejected: '已拒绝',
    superseded: '已被新方案替代',
  } as Record<string, string>)[status] ?? status
}

function participantRoleLabel(userId: string) {
  return userId === viewerUserId.value ? '我' : '对方'
}

function finalReasonLabel(value?: string) {
  if (!value) return '尚未形成终局'
  return ({
    mutual_agreement_no_remedy: '双方协商后直接结案',
    remedy_confirmed: '整改已由受益方确认',
    remedy_confirmation_expired: '整改确认期中性结束',
    admin_resolved_no_remedy: '平台裁决无需整改',
    admin_closed: '平台关闭案件',
  } as Record<string, string>)[value] ?? value
}

function mutationError(error: unknown, fallback: string) {
  toast.error(error instanceof Error ? error.message : fallback)
}

async function appendMessage() {
  if (!messageBody.value.trim()) return
  try {
    await appendMessageMutation.mutateAsync({ disputeId: props.disputeId, body: messageBody.value.trim(), evidenceAssetIds: messageEvidence.value.map(item => item.id) })
    messageBody.value = ''
    messageEvidence.value = []
  } catch (error) {
    mutationError(error, '留言提交失败。')
  }
}

async function createProposal() {
  if (!proposalTerms.value.trim()) return
  try {
    await createProposalMutation.mutateAsync({
      disputeId: props.disputeId,
      input: {
        resolution: proposalResolution.value,
        amountCny: proposalResolution.value === 'partial_refund' ? proposalAmount.value.trim() : null,
        terms: proposalTerms.value.trim(),
        fulfillmentRequired: proposalNeedsFulfillment.value,
        ...(proposalNeedsFulfillment.value ? {
          responsibleUserId: proposalResponsibleUserId.value,
          beneficiaryUserId: proposalBeneficiaryUserId.value,
          dueAt: new Date(proposalDueAt.value).toISOString(),
        } : {}),
      },
    })
    proposalAmount.value = ''
    proposalTerms.value = ''
    proposalDueAt.value = ''
    toast.success('协商方案已提交。')
  } catch (error) {
    mutationError(error, '协商方案提交失败。')
  }
}

async function confirmProposal() {
  if (!pendingProposal.value) return
  try {
    await confirmProposalMutation.mutateAsync({ disputeId: props.disputeId, proposalId: pendingProposal.value.id })
    toast.success(pendingProposal.value.fulfillmentRequired ? '双方已确认方案，等待责任方履行。' : '双方已确认方案，纠纷已结案。')
  } catch (error) {
    mutationError(error, '方案确认失败。')
  }
}

async function rejectProposal() {
  if (!pendingProposal.value) return
  try {
    await rejectProposalMutation.mutateAsync({
      disputeId: props.disputeId,
      proposalId: pendingProposal.value.id,
      reason: rejectReason.value.trim(),
    })
    rejectReason.value = ''
    toast.success('已拒绝当前方案，纠纷继续协商。')
  } catch (error) {
    mutationError(error, '方案拒绝失败。')
  }
}

async function escalate() {
  if (!canEscalate.value) return
  try {
    await escalateMutation.mutateAsync({
      disputeId: props.disputeId,
      input: {
        negotiationChannels: negotiationChannels.value,
        negotiationEndedConfirmed: true,
        negotiationSummary: negotiationSummary.value.trim(),
        requestedPlatformAction: requestedPlatformAction.value.trim(),
        evidenceAssetIds: escalationEvidence.value.map(item => item.id),
      },
    })
    negotiationChannels.value = []
    negotiationEndedConfirmed.value = false
    negotiationSummary.value = ''
    requestedPlatformAction.value = ''
    escalationEvidence.value = []
    toast.success('已申请平台介入。')
  } catch (error) {
    mutationError(error, '平台介入申请失败。')
  }
}

async function claimRemedy() {
  if (!remedyClaimNote.value.trim()) return
  try {
    await claimRemedyMutation.mutateAsync({ disputeId: props.disputeId, note: remedyClaimNote.value.trim(), evidenceAssetIds: remedyClaimEvidence.value.map(item => item.id) })
    remedyClaimNote.value = ''
    remedyClaimEvidence.value = []
    toast.success('履行声明已提交，等待对方确认。')
  } catch (error) {
    mutationError(error, '履行声明提交失败。')
  }
}

async function confirmRemedy() {
  try {
    await confirmRemedyMutation.mutateAsync({ disputeId: props.disputeId, reason: remedyConfirmationNote.value.trim() })
    remedyConfirmationNote.value = ''
    toast.success('已确认整改履行完成，纠纷已结案。')
  } catch (error) {
    mutationError(error, '整改确认失败。')
  }
}

async function contestRemedy() {
  if (remedyContestReason.value.trim().length < 2) return
  try {
    await contestRemedyMutation.mutateAsync({ disputeId: props.disputeId, reason: remedyContestReason.value.trim(), evidenceAssetIds: remedyContestEvidence.value.map(item => item.id) })
    remedyContestReason.value = ''
    remedyContestEvidence.value = []
    toast.success('已反馈未收到或未履行，纠纷重新进入平台审核。')
  } catch (error) {
    mutationError(error, '平台复核申请失败。')
  }
}

async function submitSupplement() {
  if (!dispute.value?.openInfoRequestId || supplementBody.value.trim().length < 4) return
  try {
    await submitSupplementMutation.mutateAsync({
      entityType: 'dispute',
      entityId: dispute.value.id,
      openInfoRequestId: dispute.value.openInfoRequestId,
      body: supplementBody.value.trim(),
      evidenceAssetIds: supplementEvidence.value.map(item => item.id),
    })
    supplementBody.value = ''
    supplementEvidence.value = []
    await disputeQuery.refetch()
    toast.success('平台要求的补充材料已提交。')
  } catch (error) {
    mutationError(error, '补充材料提交失败。')
  }
}
</script>

<template>
  <section class="border-y border-border py-6" aria-labelledby="api-order-dispute-title">
    <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="api-order-dispute-title" class="flex items-center gap-2 text-base font-semibold">
          <Scale class="h-4 w-4 text-warning" />订单纠纷
        </h2>
        <p class="mt-1 text-sm text-muted-foreground">{{ dispute?.targetLabel }}</p>
      </div>
      <Badge v-if="dispute" variant="status">{{ getDisputeCaseStatusLabel(dispute.status) }}</Badge>
    </div>

    <SkeletonBlock v-if="disputeQuery.isPending.value" :lines="6" />
    <Alert v-else-if="disputeQuery.error.value" variant="destructive">
      <AlertTitle>纠纷详情读取失败</AlertTitle>
      <AlertDescription class="flex items-center justify-between gap-3">
        <span>{{ disputeQuery.error.value instanceof Error ? disputeQuery.error.value.message : '请稍后重试。' }}</span>
        <Button size="sm" variant="outline" @click="disputeQuery.refetch()">重试</Button>
      </AlertDescription>
    </Alert>

    <template v-else-if="dispute">
      <dl class="grid gap-x-6 gap-y-4 border-b border-border pb-5 sm:grid-cols-3">
        <div>
          <dt class="text-xs text-muted-foreground">问题类型</dt>
          <dd class="mt-1 text-sm font-medium">{{ dispute.issueCode ? apiOrderDisputeIssueLabels[dispute.issueCode] : '历史纠纷' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted-foreground">当前诉求</dt>
          <dd class="mt-1 text-sm font-medium">{{ dispute.requestedResolution ? apiOrderDisputeResolutionLabels[dispute.requestedResolution] : '未结构化记录' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted-foreground">诉求金额</dt>
          <dd class="mt-1 text-sm font-medium">{{ dispute.requestedAmountCny ? `¥${dispute.requestedAmountCny}` : '不涉及' }}</dd>
        </div>
      </dl>

      <Alert v-if="dispute.status === 'closed'" class="my-5">
        <FileText class="h-4 w-4" />
        <AlertTitle>{{ finalReasonLabel(dispute.finalReason) }}</AlertTitle>
        <AlertDescription>
          <span v-if="dispute.canAppeal && dispute.appealExpiresAt">可在 <LocalTime :value="dispute.appealExpiresAt" /> 前从“举报与申诉”提交申诉。</span>
          <span v-else-if="dispute.appealExpiresAt">申诉期限：<LocalTime :value="dispute.appealExpiresAt" />。</span>
          <span v-else>该历史案件仅保留终局事实。</span>
        </AlertDescription>
      </Alert>

      <Alert v-else-if="dispute.status === 'open' || dispute.status === 'waiting_info'" class="my-5 border-warning/35 bg-warning/10">
        <Scale class="h-4 w-4 text-warning" />
        <AlertTitle>双方协商已结束，平台处理中</AlertTitle>
        <AlertDescription>
          <div v-if="dispute.negotiationSummary">最终分歧：{{ dispute.negotiationSummary }}</div>
          <div v-if="dispute.requestedPlatformAction" class="mt-1">申请事项：{{ dispute.requestedPlatformAction }}</div>
          <div v-if="dispute.negotiationChannels?.length" class="mt-1 text-xs">已使用渠道：{{ dispute.negotiationChannels.map(value => negotiationChannelOptions.find(item => item.value === value)?.label ?? value).join('、') }}</div>
        </AlertDescription>
      </Alert>

      <section v-if="dispute.evidence?.length" class="border-b border-border py-5">
        <h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><FileText class="h-4 w-4" />图片材料</h3>
        <DisputeEvidenceGallery :items="dispute.evidence" />
      </section>

      <section v-if="currentRemedy" class="border-b border-border py-5">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
          <h3 class="flex items-center gap-2 text-sm font-semibold"><Gavel class="h-4 w-4" />平台裁决与整改</h3>
          <Badge variant="status">{{ apiOrderDisputeRemedyStatusLabels[currentRemedy.status] }}</Badge>
        </div>
        <dl class="grid gap-x-6 gap-y-4 sm:grid-cols-3">
          <div>
            <dt class="text-xs text-muted-foreground">整改动作</dt>
            <dd class="mt-1 text-sm font-medium">
              {{ apiOrderDisputeResolutionLabels[currentRemedy.action] }}<span v-if="currentRemedy.amountCny"> · ¥{{ currentRemedy.amountCny }}</span>
            </dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">责任方</dt>
            <dd class="mt-1 text-sm font-medium">{{ participantRoleLabel(currentRemedy.responsibleUserId) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">履行期限</dt>
            <dd class="mt-1 flex items-center gap-1.5 text-sm font-medium"><Clock3 class="h-3.5 w-3.5" /><LocalTime :value="currentRemedy.dueAt" /></dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">整改来源</dt>
            <dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeRemedySourceLabels[currentRemedy.source] }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">迟到事实</dt>
            <dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeRemedyLatenessLabels[currentRemedy.latenessStatus] }}</dd>
          </div>
          <div v-if="currentRemedy.lateAt">
            <dt class="text-xs text-muted-foreground">迟到起点</dt>
            <dd class="mt-1 text-sm font-medium"><LocalTime :value="currentRemedy.lateAt" /></dd>
          </div>
        </dl>
        <Alert v-if="currentRemedy.claimedLate" class="mt-4">
          <Clock3 class="h-4 w-4" />
          <AlertTitle>责任方在期限后声明履行</AlertTitle>
          <AlertDescription>迟到事实与当前履行进度分别记录；是否产生责任影响以平台裁定为准。</AlertDescription>
        </Alert>
        <p v-if="currentRemedy.latenessReason" class="mt-3 text-xs text-muted-foreground">平台迟到裁定说明：{{ currentRemedy.latenessReason }}</p>
        <div class="mt-4 border-l-2 border-warning pl-4">
          <p class="text-xs text-muted-foreground">整改要求</p>
          <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.instructions }}</p>
        </div>
        <div v-if="currentRemedy.claimNote" class="mt-4 border-l-2 border-border pl-4">
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>责任方履行声明</span>
            <LocalTime v-if="currentRemedy.claimedAt" :value="currentRemedy.claimedAt" />
          </div>
          <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.claimNote }}</p>
          <p v-if="currentRemedy.confirmationDueAt && currentRemedy.status === 'claimed_fulfilled'" class="mt-2 text-xs text-muted-foreground">
            对方反馈截止：<LocalTime :value="currentRemedy.confirmationDueAt" />
          </p>
        </div>
        <div v-if="currentRemedy.responseNote" class="mt-4 border-l-2 border-border pl-4">
          <p class="text-xs text-muted-foreground">结果记录</p>
          <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ currentRemedy.responseNote }}</p>
        </div>
        <Alert v-if="currentRemedy.status === 'confirmation_expired'" class="mt-4">
          <AlertTitle>确认期已结束</AlertTitle>
          <AlertDescription>对方未在期限内反馈，流程已中性结案；平台未核验退款到账或履约事实。</AlertDescription>
        </Alert>

        <div v-if="canClaimRemedy" class="mt-5 space-y-3">
          <Textarea v-model="remedyClaimNote" class="min-h-20" maxlength="2000" placeholder="说明已如何完成退款或继续履约，请勿填写 API Key、密码等敏感信息。" />
          <DisputeEvidencePicker v-if="orderId" v-model="remedyClaimEvidence" :order-id="orderId" />
          <Button :disabled="remedyClaimNote.trim().length < 2 || mutationBusy" @click="claimRemedy">
            <Check class="h-4 w-4" />声明已履行
          </Button>
        </div>
        <div v-else-if="canRespondToRemedy" class="mt-5 space-y-3">
          <Textarea v-model="remedyConfirmationNote" class="min-h-16" maxlength="500" placeholder="确认说明（选填）" />
          <Button :disabled="mutationBusy" @click="confirmRemedy"><Check class="h-4 w-4" />确认已收到或已完成</Button>
          <div class="border-t border-border pt-4">
            <Textarea v-model="remedyContestReason" class="min-h-20" maxlength="2000" placeholder="如未收到退款或履约未完成，请说明事实并申请平台复核。" />
            <DisputeEvidencePicker v-if="orderId" v-model="remedyContestEvidence" :order-id="orderId" />
            <Button class="mt-3" variant="outline" :disabled="remedyContestReason.trim().length < 2 || mutationBusy" @click="contestRemedy">
              <X class="h-4 w-4" />未收到或未完成
            </Button>
          </div>
        </div>

        <div v-if="remedyHistory.length" class="mt-5 border-t border-border pt-4">
          <h4 class="text-xs font-medium text-muted-foreground">历史整改</h4>
          <div class="mt-3 space-y-4">
            <article v-for="remedy in remedyHistory" :key="remedy.id" class="border-l-2 border-border pl-4">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[remedy.action] }}<span v-if="remedy.amountCny"> · ¥{{ remedy.amountCny }}</span></span>
                <Badge variant="secondary">{{ apiOrderDisputeRemedyStatusLabels[remedy.status] }}</Badge>
              </div>
              <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ remedy.instructions }}</p>
              <p class="mt-2 text-xs text-muted-foreground">{{ apiOrderDisputeRemedySourceLabels[remedy.source] }} · {{ apiOrderDisputeRemedyLatenessLabels[remedy.latenessStatus] }}</p>
              <div class="mt-2 text-xs text-muted-foreground"><LocalTime :value="remedy.updatedAt" /></div>
            </article>
          </div>
        </div>
      </section>

      <section class="border-b border-border py-5">
        <h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><MessageSquareText class="h-4 w-4" />{{ canNegotiate ? '站内协商留痕' : '协商记录' }}</h3>
        <div v-if="dispute.messages?.length" class="space-y-4">
          <article v-for="message in dispute.messages" :key="message.id" class="border-l-2 border-border pl-4">
            <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
              <span>{{ senderLabel(message.senderUserId) }}</span>
              <LocalTime :value="message.createdAt" />
            </div>
            <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{{ message.body }}</p>
          </article>
        </div>
        <p v-else class="text-sm text-muted-foreground">暂无沟通记录。</p>

        <div v-if="canMessage" class="mt-5 space-y-3">
          <Textarea v-model="messageBody" class="min-h-20" maxlength="2000" placeholder="记录协商事实或进展，请勿填写 API Key、密码等敏感信息。" />
          <DisputeEvidencePicker v-if="orderId" v-model="messageEvidence" :order-id="orderId" />
          <Button :disabled="!messageBody.trim() || mutationBusy" @click="appendMessage">
            <Send class="h-4 w-4" />发送留痕
          </Button>
        </div>
      </section>

      <section v-if="dispute.canSupplement && dispute.openInfoRequestId" class="border-b border-border py-5">
        <h3 class="text-sm font-semibold">平台定向补件</h3>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">平台正在等待你的答复。此处提交的是补件，不会重新开启双方协商。</p>
        <Textarea v-model="supplementBody" class="mt-3 min-h-28" maxlength="1200" placeholder="针对平台要求补充脱敏事实说明。" />
        <DisputeEvidencePicker v-if="orderId" v-model="supplementEvidence" :order-id="orderId" visibility="submitter_admin" />
        <Button class="mt-3" :disabled="supplementBody.trim().length < 4 || mutationBusy" @click="submitSupplement">提交补件</Button>
      </section>

      <section class="border-b border-border py-5">
        <h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><Handshake class="h-4 w-4" />协商方案</h3>
        <div v-if="pendingProposal" class="border-l-2 border-warning pl-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[pendingProposal.resolution] }}<span v-if="pendingProposal.amountCny"> · ¥{{ pendingProposal.amountCny }}</span></span>
            <Badge variant="secondary">{{ proposalStatusLabel(pendingProposal.status) }}</Badge>
          </div>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ pendingProposal.terms }}</p>
          <p v-if="pendingProposal.fulfillmentRequired" class="mt-2 text-xs text-muted-foreground">
            确认后由{{ participantRoleLabel(pendingProposal.responsibleUserId || '') }}在 <LocalTime v-if="pendingProposal.dueAt" :value="pendingProposal.dueAt" /> 前履行，不会立即结案。
          </p>
          <p v-if="pendingFromMe" class="mt-3 text-xs text-muted-foreground">等待对方确认或拒绝。</p>
          <div v-else-if="canNegotiate" class="mt-4 space-y-3">
            <Textarea v-model="rejectReason" class="min-h-16" maxlength="500" placeholder="拒绝说明（选填）" />
            <div class="flex flex-wrap gap-2">
              <Button :disabled="mutationBusy" @click="confirmProposal"><Check class="h-4 w-4" />{{ pendingProposal.fulfillmentRequired ? '确认方案并等待履行' : '确认方案并结案' }}</Button>
              <Button variant="outline" :disabled="mutationBusy" @click="rejectProposal"><X class="h-4 w-4" />拒绝方案</Button>
            </div>
          </div>
        </div>
        <p v-else class="text-sm text-muted-foreground">暂无待确认方案。</p>

        <div v-if="proposalHistory.length" class="mt-5 border-t border-border pt-4">
          <h4 class="text-xs font-medium text-muted-foreground">历史方案</h4>
          <div class="mt-3 space-y-4">
            <article v-for="proposal in proposalHistory" :key="proposal.id" class="border-l-2 border-border pl-4">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[proposal.resolution] }}<span v-if="proposal.amountCny"> · ¥{{ proposal.amountCny }}</span></span>
                <Badge variant="secondary">{{ proposalStatusLabel(proposal.status) }}</Badge>
              </div>
              <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ proposal.terms }}</p>
              <p v-if="proposal.fulfillmentRequired" class="mt-2 text-xs text-muted-foreground">需由{{ participantRoleLabel(proposal.responsibleUserId || '') }}履行<span v-if="proposal.dueAt"> · 截止 <LocalTime :value="proposal.dueAt" /></span></p>
              <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                <span>{{ senderLabel(proposal.proposedByUserId) }}提出</span>
                <LocalTime :value="proposal.updatedAt" />
              </div>
            </article>
          </div>
        </div>

        <div v-if="canNegotiate" class="mt-5 grid gap-3 sm:grid-cols-[180px_1fr]">
          <Select v-model="proposalResolution">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="(label, value) in proposalResolutionLabels" :key="value" :value="value">{{ label }}</SelectItem>
            </SelectContent>
          </Select>
          <Input v-if="proposalResolution === 'partial_refund'" v-model="proposalAmount" inputmode="decimal" placeholder="退款金额（元）" />
          <template v-if="proposalNeedsFulfillment">
            <Select v-model="proposalResponsibleUserId">
              <SelectTrigger><SelectValue placeholder="选择履行责任方" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="participant in participantOptions" :key="participant.userId" :value="participant.userId">
                  {{ participantRoleLabel(participant.userId) }} · {{ participant.label }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Input v-model="proposalDueAt" type="datetime-local" aria-label="履行截止时间" />
          </template>
          <Textarea v-model="proposalTerms" class="min-h-20 sm:col-span-2" maxlength="2000" placeholder="填写需要双方共同确认的完整处理方案。" />
          <div class="sm:col-span-2">
            <Button variant="outline" :disabled="!canCreateProposal || mutationBusy" @click="createProposal">
              <Handshake class="h-4 w-4" />提交协商方案
            </Button>
          </div>
        </div>
      </section>

      <section v-if="canNegotiate" class="pt-5">
        <h3 class="text-sm font-semibold">申请平台介入</h3>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">申请后双方协商结束，站内留言和协商方案都会停止，只保留平台补件、裁决和履行反馈。</p>
        <div class="mt-4 space-y-4">
          <fieldset>
            <legend class="text-sm font-medium">已经使用过的沟通渠道</legend>
            <div class="mt-2 flex flex-wrap gap-x-5 gap-y-2">
              <label v-for="option in negotiationChannelOptions" :key="option.value" class="flex items-center gap-2 text-sm">
                <Checkbox :model-value="negotiationChannels.includes(option.value)" @update:model-value="value => toggleNegotiationChannel(option.value, Boolean(value))" />
                <span>{{ option.label }}</span>
              </label>
            </div>
          </fieldset>
          <label class="block space-y-2">
            <span class="text-sm font-medium">最终分歧</span>
            <Textarea v-model="negotiationSummary" class="min-h-24" maxlength="2000" placeholder="说明已经沟通但仍无法一致的事实和方案。" />
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">希望平台处理的事项</span>
            <Textarea v-model="requestedPlatformAction" class="min-h-20" maxlength="1000" placeholder="说明需要平台判断、要求补件或下达整改的事项。" />
          </label>
          <DisputeEvidencePicker v-if="orderId" v-model="escalationEvidence" :order-id="orderId" />
          <label class="flex items-start gap-2 text-sm leading-6">
            <Checkbox v-model="negotiationEndedConfirmed" class="mt-0.5" />
            <span>我确认双方协商已经结束，申请后不再通过本系统继续协商。</span>
          </label>
          <Button variant="destructive" :disabled="!canEscalate || mutationBusy" @click="escalate">
            <Scale class="h-4 w-4" />结束协商并申请平台介入
          </Button>
        </div>
      </section>
    </template>
  </section>
</template>
