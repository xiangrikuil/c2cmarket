<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Clock3, Gavel, Handshake, MessageSquareText, Scale, Send, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  apiOrderDisputeIssueLabels,
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
} from '@/queries/useReportQueries'

const props = defineProps<{
  disputeId: string
  viewerUserId: string
}>()

const disputeId = computed(() => props.disputeId)
const disputeQuery = useMyDisputeQuery(disputeId)
const dispute = computed(() => disputeQuery.data.value ?? null)
const messageBody = ref('')
const proposalResolution = ref<ApiOrderDisputeResolution>('full_refund')
const proposalResolutionLabels = Object.fromEntries(
  Object.entries(apiOrderDisputeResolutionLabels).filter(([value]) => value !== 'continue_fulfillment'),
)
const proposalAmount = ref('')
const proposalTerms = ref('')
const rejectReason = ref('')
const escalationReason = ref('')
const remedyClaimNote = ref('')
const remedyConfirmationNote = ref('')
const remedyContestReason = ref('')

const appendMessageMutation = useAppendDisputeMessageMutation()
const createProposalMutation = useCreateDisputeSettlementProposalMutation()
const confirmProposalMutation = useConfirmDisputeSettlementProposalMutation()
const rejectProposalMutation = useRejectDisputeSettlementProposalMutation()
const escalateMutation = useEscalateDisputeMutation()
const claimRemedyMutation = useClaimDisputeRemedyMutation()
const confirmRemedyMutation = useConfirmDisputeRemedyMutation()
const contestRemedyMutation = useContestDisputeRemedyMutation()

const pendingProposal = computed(() => dispute.value?.settlementProposals?.find(item => item.status === 'pending') ?? null)
const proposalHistory = computed(() => dispute.value?.settlementProposals?.filter(item => item.status !== 'pending') ?? [])
const currentRemedy = computed(() => dispute.value?.remedies?.[0] ?? null)
const remedyHistory = computed(() => dispute.value?.remedies?.slice(1) ?? [])
const isRemedyResponsible = computed(() => currentRemedy.value?.responsibleUserId === props.viewerUserId)
const isRemedyBeneficiary = computed(() => currentRemedy.value?.beneficiaryUserId === props.viewerUserId)
const canClaimRemedy = computed(() => currentRemedy.value?.status === 'pending' && isRemedyResponsible.value)
const canRespondToRemedy = computed(() => currentRemedy.value?.status === 'claimed_fulfilled' && isRemedyBeneficiary.value)
const canNegotiate = computed(() => dispute.value?.status === 'negotiating')
const canMessage = computed(() => ['negotiating', 'open', 'waiting_info'].includes(dispute.value?.status ?? ''))
const pendingFromMe = computed(() => pendingProposal.value?.proposedByUserId === props.viewerUserId)
const mutationBusy = computed(() => appendMessageMutation.isPending.value
  || createProposalMutation.isPending.value
  || confirmProposalMutation.isPending.value
  || rejectProposalMutation.isPending.value
  || escalateMutation.isPending.value
  || claimRemedyMutation.isPending.value
  || confirmRemedyMutation.isPending.value
  || contestRemedyMutation.isPending.value)

function senderLabel(senderUserId: string) {
  return senderUserId === props.viewerUserId ? '我' : '对方'
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
  return userId === props.viewerUserId ? '我' : '对方'
}

function mutationError(error: unknown, fallback: string) {
  toast.error(error instanceof Error ? error.message : fallback)
}

async function appendMessage() {
  if (!messageBody.value.trim()) return
  try {
    await appendMessageMutation.mutateAsync({ disputeId: props.disputeId, body: messageBody.value.trim() })
    messageBody.value = ''
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
      },
    })
    proposalAmount.value = ''
    proposalTerms.value = ''
    toast.success('协商方案已提交。')
  } catch (error) {
    mutationError(error, '协商方案提交失败。')
  }
}

async function confirmProposal() {
  if (!pendingProposal.value) return
  try {
    await confirmProposalMutation.mutateAsync({ disputeId: props.disputeId, proposalId: pendingProposal.value.id })
    toast.success('双方已确认方案，纠纷已结案。')
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
  if (!escalationReason.value.trim()) return
  try {
    await escalateMutation.mutateAsync({ disputeId: props.disputeId, reason: escalationReason.value.trim() })
    escalationReason.value = ''
    toast.success('已申请平台介入。')
  } catch (error) {
    mutationError(error, '平台介入申请失败。')
  }
}

async function claimRemedy() {
  if (!remedyClaimNote.value.trim()) return
  try {
    await claimRemedyMutation.mutateAsync({ disputeId: props.disputeId, note: remedyClaimNote.value.trim() })
    remedyClaimNote.value = ''
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
    await contestRemedyMutation.mutateAsync({ disputeId: props.disputeId, reason: remedyContestReason.value.trim() })
    remedyContestReason.value = ''
    toast.success('已反馈未收到或未履行，纠纷重新进入平台审核。')
  } catch (error) {
    mutationError(error, '平台复核申请失败。')
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
        </dl>
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
          <Button :disabled="remedyClaimNote.trim().length < 2 || mutationBusy" @click="claimRemedy">
            <Check class="h-4 w-4" />声明已履行
          </Button>
        </div>
        <div v-else-if="canRespondToRemedy" class="mt-5 space-y-3">
          <Textarea v-model="remedyConfirmationNote" class="min-h-16" maxlength="500" placeholder="确认说明（选填）" />
          <Button :disabled="mutationBusy" @click="confirmRemedy"><Check class="h-4 w-4" />确认已收到或已完成</Button>
          <div class="border-t border-border pt-4">
            <Textarea v-model="remedyContestReason" class="min-h-20" maxlength="2000" placeholder="如未收到退款或履约未完成，请说明事实并申请平台复核。" />
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
              <div class="mt-2 text-xs text-muted-foreground"><LocalTime :value="remedy.updatedAt" /></div>
            </article>
          </div>
        </div>
      </section>

      <section class="border-b border-border py-5">
        <h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><MessageSquareText class="h-4 w-4" />沟通记录</h3>
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

        <div v-if="canMessage" class="mt-5 flex items-end gap-2">
          <Textarea v-model="messageBody" class="min-h-20 flex-1" maxlength="2000" placeholder="补充事实或处理进展，请勿填写 API Key、密码等敏感信息。" />
          <Button size="icon" :disabled="!messageBody.trim() || mutationBusy" title="发送留言" @click="appendMessage">
            <Send class="h-4 w-4" />
          </Button>
        </div>
      </section>

      <section class="border-b border-border py-5">
        <h3 class="mb-4 flex items-center gap-2 text-sm font-semibold"><Handshake class="h-4 w-4" />协商方案</h3>
        <div v-if="pendingProposal" class="border-l-2 border-warning pl-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[pendingProposal.resolution] }}<span v-if="pendingProposal.amountCny"> · ¥{{ pendingProposal.amountCny }}</span></span>
            <Badge variant="secondary">{{ proposalStatusLabel(pendingProposal.status) }}</Badge>
          </div>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ pendingProposal.terms }}</p>
          <p v-if="pendingFromMe" class="mt-3 text-xs text-muted-foreground">等待对方确认或拒绝。</p>
          <div v-else-if="canNegotiate" class="mt-4 space-y-3">
            <Textarea v-model="rejectReason" class="min-h-16" maxlength="500" placeholder="拒绝说明（选填）" />
            <div class="flex flex-wrap gap-2">
              <Button :disabled="mutationBusy" @click="confirmProposal"><Check class="h-4 w-4" />确认方案并结案</Button>
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
          <Textarea v-model="proposalTerms" class="min-h-20 sm:col-span-2" maxlength="2000" placeholder="填写需要双方共同确认的完整处理方案。" />
          <div class="sm:col-span-2">
            <Button variant="outline" :disabled="!proposalTerms.trim() || (proposalResolution === 'partial_refund' && !proposalAmount.trim()) || mutationBusy" @click="createProposal">
              <Handshake class="h-4 w-4" />提交协商方案
            </Button>
          </div>
        </div>
      </section>

      <section v-if="canNegotiate" class="pt-5">
        <h3 class="text-sm font-semibold">申请平台介入</h3>
        <div class="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
          <Textarea v-model="escalationReason" class="min-h-20 flex-1" maxlength="500" placeholder="说明双方未能达成一致的事项。" />
          <Button variant="destructive" :disabled="escalationReason.trim().length < 2 || mutationBusy" @click="escalate">
            <Scale class="h-4 w-4" />申请平台审核
          </Button>
        </div>
      </section>
    </template>
  </section>
</template>
