<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Clock3, FileText, Gavel, History, Scale, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import DisputeEvidenceGallery from '@/components/api-order/DisputeEvidenceGallery.vue'
import DisputeEvidencePicker from '@/components/api-order/DisputeEvidencePicker.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { apiOrderDisputeIssueLabels, apiOrderDisputeRemedyLatenessLabels, apiOrderDisputeRemedyStatusLabels, apiOrderDisputeResolutionLabels } from '@/lib/apiOrderDispute'
import { getDisputeCaseStatusLabel } from '@/lib/disputeCase'
import type { DisputeEvidenceAsset } from '@/lib/disputeEvidenceBackend'
import {
  useClaimDisputeRemedyMutation,
  useConfirmDisputeRemedyMutation,
  useContestDisputeRemedyMutation,
  useMyDisputeQuery,
  useRespondDisputeMutation,
  useSelfResolveDisputeMutation,
  useSubmitInfoSupplementMutation,
  useWithdrawDisputeMutation,
} from '@/queries/useReportQueries'

const props = defineProps<{ disputeId: string }>()
const disputeId = computed(() => props.disputeId)
const disputeQuery = useMyDisputeQuery(disputeId)
const dispute = computed(() => disputeQuery.data.value)
const viewerUserId = computed(() => dispute.value?.viewerUserId ?? '')
const orderId = computed(() => dispute.value?.apiOrderId ?? '')
const currentRemedy = computed(() => dispute.value?.remedies?.[0] ?? null)
const historicalMessages = computed(() => dispute.value?.messages ?? [])
const historicalProposals = computed(() => dispute.value?.settlementProposals ?? [])

const responseBody = ref('')
const responseEvidence = ref<DisputeEvidenceAsset[]>([])
const finishReason = ref('')
const supplementBody = ref('')
const supplementEvidence = ref<DisputeEvidenceAsset[]>([])
const remedyNote = ref('')
const remedyEvidence = ref<DisputeEvidenceAsset[]>([])
const remedyResponse = ref('')
const remedyContestEvidence = ref<DisputeEvidenceAsset[]>([])

const respondMutation = useRespondDisputeMutation()
const withdrawMutation = useWithdrawDisputeMutation()
const selfResolveMutation = useSelfResolveDisputeMutation()
const supplementMutation = useSubmitInfoSupplementMutation()
const claimRemedyMutation = useClaimDisputeRemedyMutation()
const confirmRemedyMutation = useConfirmDisputeRemedyMutation()
const contestRemedyMutation = useContestDisputeRemedyMutation()

const canRespond = computed(() => dispute.value?.active
  && dispute.value.status === 'open'
  && dispute.value.counterpartyUserId === viewerUserId.value
  && !dispute.value.respondedAt)
const canApplicantFinish = computed(() => dispute.value?.active
  && dispute.value.primaryUserId === viewerUserId.value
  && (dispute.value.status === 'open' || dispute.value.status === 'waiting_info'))
const canClaimRemedy = computed(() => currentRemedy.value?.status === 'pending'
  && currentRemedy.value.responsibleUserId === viewerUserId.value)
const canConfirmRemedy = computed(() => currentRemedy.value?.status === 'claimed_fulfilled'
  && currentRemedy.value.beneficiaryUserId === viewerUserId.value)
const mutationBusy = computed(() => [respondMutation, withdrawMutation, selfResolveMutation, supplementMutation, claimRemedyMutation, confirmRemedyMutation, contestRemedyMutation]
  .some(item => item.isPending.value))

const nextActorLabel = computed(() => ({
  applicant: '等待申请人补充材料',
  respondent: '等待被申请方正式答复',
  admin: '等待平台审核',
  responsible_party: '等待责任方履行整改',
  counterparty: '等待对方确认履行结果',
  none: '当前无需操作',
}[dispute.value?.nextActor ?? 'none']))

function mutationError(error: unknown, fallback: string) {
  toast.error(error instanceof Error ? error.message : fallback)
}

async function respond() {
  if (responseBody.value.trim().length < 2) return
  try {
    await respondMutation.mutateAsync({ disputeId: props.disputeId, body: responseBody.value.trim(), evidenceAssetIds: responseEvidence.value.map(item => item.id) })
    responseBody.value = ''
    responseEvidence.value = []
    toast.success('正式答复已提交，之后不可修改。')
  } catch (error) {
    mutationError(error, '正式答复提交失败。')
  }
}

async function finish(kind: 'withdraw' | 'self_resolve') {
  try {
    const input = { disputeId: props.disputeId, reason: finishReason.value.trim() }
    if (kind === 'withdraw') await withdrawMutation.mutateAsync(input)
    else await selfResolveMutation.mutateAsync(input)
    finishReason.value = ''
    toast.success(kind === 'withdraw' ? '平台处理申请已撤回。' : '已确认双方线下解决。')
  } catch (error) {
    mutationError(error, '案件结束操作失败。')
  }
}

async function submitSupplement() {
  if (!dispute.value?.openInfoRequestId || supplementBody.value.trim().length < 4) return
  try {
    await supplementMutation.mutateAsync({ entityType: 'dispute', entityId: dispute.value.id, openInfoRequestId: dispute.value.openInfoRequestId, body: supplementBody.value.trim(), evidenceAssetIds: supplementEvidence.value.map(item => item.id) })
    supplementBody.value = ''
    supplementEvidence.value = []
    await disputeQuery.refetch()
    toast.success('补充材料已提交。')
  } catch (error) {
    mutationError(error, '补充材料提交失败。')
  }
}

async function claimRemedy() {
  if (remedyNote.value.trim().length < 2) return
  try {
    await claimRemedyMutation.mutateAsync({ disputeId: props.disputeId, note: remedyNote.value.trim(), evidenceAssetIds: remedyEvidence.value.map(item => item.id) })
    remedyNote.value = ''
    remedyEvidence.value = []
    toast.success('履行声明已提交。')
  } catch (error) {
    mutationError(error, '履行声明提交失败。')
  }
}

async function confirmRemedy() {
  try {
    await confirmRemedyMutation.mutateAsync({ disputeId: props.disputeId, reason: remedyResponse.value.trim() })
    remedyResponse.value = ''
    toast.success('已确认整改完成。')
  } catch (error) {
    mutationError(error, '整改确认失败。')
  }
}

async function contestRemedy() {
  if (remedyResponse.value.trim().length < 2) return
  try {
    await contestRemedyMutation.mutateAsync({ disputeId: props.disputeId, reason: remedyResponse.value.trim(), evidenceAssetIds: remedyContestEvidence.value.map(item => item.id) })
    remedyResponse.value = ''
    remedyContestEvidence.value = []
    toast.success('已提交平台复核。')
  } catch (error) {
    mutationError(error, '平台复核提交失败。')
  }
}
</script>

<template>
  <section class="py-6" aria-labelledby="api-order-dispute-title">
    <div class="mb-5 flex flex-wrap items-start justify-between gap-3 border-b border-border pb-5">
      <div><h2 id="api-order-dispute-title" class="flex items-center gap-2 text-base font-semibold"><Scale class="h-4 w-4 text-warning" />平台纠纷处理</h2><p class="mt-1 text-sm text-muted-foreground">{{ dispute?.targetLabel }}</p></div>
      <Badge v-if="dispute" variant="status">{{ getDisputeCaseStatusLabel(dispute.status) }}</Badge>
    </div>

    <SkeletonBlock v-if="disputeQuery.isPending.value" :lines="6" />
    <Alert v-else-if="disputeQuery.error.value" variant="destructive"><AlertTitle>纠纷详情读取失败</AlertTitle><AlertDescription>{{ disputeQuery.error.value instanceof Error ? disputeQuery.error.value.message : '请稍后重试。' }}</AlertDescription></Alert>

    <template v-else-if="dispute">
      <Alert class="mb-5 border-warning/35 bg-warning/10"><Scale class="h-4 w-4 text-warning" /><AlertTitle>{{ nextActorLabel }}</AlertTitle><AlertDescription><span>{{ dispute.publicResult }}</span><span v-if="dispute.dueAt">，截止 <LocalTime :value="dispute.dueAt" /></span></AlertDescription></Alert>
      <dl class="grid gap-4 border-b border-border pb-5 sm:grid-cols-3">
        <div><dt class="text-xs text-muted-foreground">问题类型</dt><dd class="mt-1 text-sm font-medium">{{ dispute.issueCode ? apiOrderDisputeIssueLabels[dispute.issueCode] : '历史案件' }}</dd></div>
        <div><dt class="text-xs text-muted-foreground">申请诉求</dt><dd class="mt-1 text-sm font-medium">{{ dispute.requestedResolution ? apiOrderDisputeResolutionLabels[dispute.requestedResolution] : '未结构化记录' }}</dd></div>
        <div><dt class="text-xs text-muted-foreground">申请时间</dt><dd class="mt-1 text-sm font-medium"><LocalTime :value="dispute.openedAt" /></dd></div>
      </dl>

      <section class="border-b border-border py-5"><h3 class="text-sm font-semibold">申请人材料</h3><p class="mt-3 whitespace-pre-wrap break-words text-sm leading-6">{{ dispute.applicantStatement || dispute.publicSummary }}</p></section>

      <section class="border-b border-border py-5">
        <h3 class="text-sm font-semibold">被申请方正式答复</h3>
        <div v-if="dispute.respondedAt" class="mt-3 border-l-2 border-border pl-4"><p class="whitespace-pre-wrap break-words text-sm leading-6">{{ dispute.respondentResponse }}</p><p class="mt-2 text-xs text-muted-foreground">提交于 <LocalTime :value="dispute.respondedAt" />，答复不可修改</p></div>
        <div v-else-if="canRespond" class="mt-4 space-y-3"><Textarea v-model="responseBody" class="min-h-32" maxlength="2000" placeholder="针对申请事实逐项答复。请勿填写 API Key、密码等敏感信息。" /><DisputeEvidencePicker v-if="orderId" v-model="responseEvidence" :order-id="orderId" /><Button :disabled="responseBody.trim().length < 2 || mutationBusy" @click="respond"><Check class="h-4 w-4" />提交正式答复</Button><p class="text-xs text-muted-foreground">正式答复只能提交一次，提交后不可修改。</p></div>
        <p v-else class="mt-3 text-sm text-muted-foreground">尚未提交正式答复。</p>
      </section>

      <section v-if="dispute.evidence?.length" class="border-b border-border py-5"><h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><FileText class="h-4 w-4" />案件图片材料</h3><DisputeEvidenceGallery :items="dispute.evidence" /></section>

      <section v-if="dispute.canSupplement && dispute.openInfoRequestId" class="border-b border-border py-5"><h3 class="text-sm font-semibold">平台定向补件</h3><p class="mt-1 text-xs text-muted-foreground">只需回答平台当前要求，不会开启双方站内协商。</p><Textarea v-model="supplementBody" class="mt-3 min-h-28" maxlength="1200" placeholder="补充脱敏事实说明。" /><DisputeEvidencePicker v-if="orderId" v-model="supplementEvidence" :order-id="orderId" visibility="submitter_admin" /><Button class="mt-3" :disabled="supplementBody.trim().length < 4 || mutationBusy" @click="submitSupplement">提交补件</Button></section>

      <section v-if="currentRemedy" class="border-b border-border py-5">
        <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="flex items-center gap-2 text-sm font-semibold"><Gavel class="h-4 w-4" />平台裁决与整改</h3><Badge variant="status">{{ apiOrderDisputeRemedyStatusLabels[currentRemedy.status] }}</Badge></div>
        <dl class="mt-4 grid gap-4 sm:grid-cols-3"><div><dt class="text-xs text-muted-foreground">整改动作</dt><dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeResolutionLabels[currentRemedy.action] }}</dd></div><div><dt class="text-xs text-muted-foreground">履行截止</dt><dd class="mt-1 text-sm font-medium"><LocalTime :value="currentRemedy.dueAt" /></dd></div><div><dt class="text-xs text-muted-foreground">迟到记录</dt><dd class="mt-1 text-sm font-medium">{{ apiOrderDisputeRemedyLatenessLabels[currentRemedy.latenessStatus] }}</dd></div></dl>
        <p class="mt-4 whitespace-pre-wrap border-l-2 border-warning pl-4 text-sm leading-6">{{ currentRemedy.instructions }}</p>
        <div v-if="canClaimRemedy" class="mt-4 space-y-3"><Textarea v-model="remedyNote" class="min-h-24" maxlength="2000" placeholder="说明已经如何履行整改。" /><DisputeEvidencePicker v-if="orderId" v-model="remedyEvidence" :order-id="orderId" /><Button :disabled="remedyNote.trim().length < 2 || mutationBusy" @click="claimRemedy"><Check class="h-4 w-4" />声明已履行</Button></div>
        <div v-else-if="canConfirmRemedy" class="mt-4 space-y-3"><Textarea v-model="remedyResponse" class="min-h-20" maxlength="2000" placeholder="填写确认说明，或说明仍未收到/未完成。" /><DisputeEvidencePicker v-if="orderId" v-model="remedyContestEvidence" :order-id="orderId" /><div class="flex flex-wrap gap-2"><Button :disabled="mutationBusy" @click="confirmRemedy"><Check class="h-4 w-4" />确认完成</Button><Button variant="outline" :disabled="remedyResponse.trim().length < 2 || mutationBusy" @click="contestRemedy"><X class="h-4 w-4" />申请复核</Button></div></div>
        <p v-if="currentRemedy.claimNote" class="mt-4 text-sm leading-6">履行声明：{{ currentRemedy.claimNote }}</p>
      </section>

      <section v-if="historicalMessages.length || historicalProposals.length" class="border-b border-border py-5"><h3 class="flex items-center gap-2 text-sm font-semibold"><History class="h-4 w-4" />旧流程历史记录</h3><p class="mt-1 text-xs text-muted-foreground">这些记录来自旧版站内协商流程，仅供查看，不能继续留言或处理方案。</p><div class="mt-4 space-y-3"><article v-for="message in historicalMessages" :key="message.id" class="border-l-2 border-border pl-4"><p class="whitespace-pre-wrap text-sm leading-6">{{ message.body }}</p><p class="mt-1 text-xs text-muted-foreground"><LocalTime :value="message.createdAt" /></p></article><article v-for="proposal in historicalProposals" :key="proposal.id" class="border-l-2 border-border pl-4"><p class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[proposal.resolution] }} · {{ proposal.status }}</p><p class="mt-1 whitespace-pre-wrap text-sm leading-6">{{ proposal.terms }}</p></article></div></section>

      <section v-if="canApplicantFinish" class="pt-5"><h3 class="text-sm font-semibold">申请人结束案件</h3><p class="mt-1 text-xs text-muted-foreground">平台裁决前，可以撤回申请；如果双方已经通过微信、邮箱等方式解决，也可以直接确认线下解决。</p><Textarea v-model="finishReason" class="mt-3 min-h-20" maxlength="500" placeholder="说明原因（选填）" /><div class="mt-3 flex flex-wrap gap-2"><Button variant="outline" :disabled="mutationBusy" @click="finish('withdraw')">撤回申请</Button><Button :disabled="mutationBusy" @click="finish('self_resolve')"><Check class="h-4 w-4" />确认线下解决</Button></div></section>

      <Alert v-if="dispute.status === 'withdrawn' || dispute.status === 'self_resolved' || dispute.status === 'closed'" class="mt-5"><Clock3 class="h-4 w-4" /><AlertTitle>{{ getDisputeCaseStatusLabel(dispute.status) }}</AlertTitle><AlertDescription>{{ dispute.publicResult }}</AlertDescription></Alert>
    </template>
  </section>
</template>
