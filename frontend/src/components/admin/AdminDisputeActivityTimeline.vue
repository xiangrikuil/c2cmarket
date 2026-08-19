<script setup lang="ts">
import { computed } from 'vue'
import { FileClock, History, MessageSquareText } from 'lucide-vue-next'
import DisputeEvidenceGallery from '@/components/api-order/DisputeEvidenceGallery.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import { Badge } from '@/components/ui/badge'
import {
  apiOrderDisputeIssueLabels,
  apiOrderDisputeRemedyStatusLabels,
  apiOrderDisputeResolutionLabels,
} from '@/lib/apiOrderDispute'
import type { DisputeEvidenceReference } from '@/lib/disputeEvidenceBackend'
import type { AdminDisputeDetail } from '@/lib/reportBackend'

const props = defineProps<{
  dispute: AdminDisputeDetail
}>()

type ActivityRecord = {
  id: string
  action: string
  actor: string
  body: string
  createdAt: string
  evidence: DisputeEvidenceReference[]
  status?: string
}

const evidence = computed(() => props.dispute.evidence ?? [])

function displayName(name: string | undefined, username: string | undefined, fallback: string) {
  return name?.trim() || (username ? `@${username}` : fallback)
}

function participantLabel(userId: string | undefined) {
  if (userId && userId === props.dispute.primaryUserId) {
    return `买家 · ${displayName(props.dispute.primaryDisplayName, props.dispute.primaryUsername, '申请人')}`
  }
  if (userId && userId === props.dispute.counterpartyUserId) {
    return `卖家 · ${displayName(props.dispute.counterpartyName, props.dispute.counterpartyUsername, '商家')}`
  }
  return userId || '系统'
}

function evidenceFor(usage: DisputeEvidenceReference['usage'], sourceId?: string) {
  return evidence.value.filter(item => item.usage === usage && (!sourceId || item.sourceId === sourceId))
}

function timestamp(value: string) {
  const parsed = new Date(value).getTime()
  return Number.isFinite(parsed) ? parsed : 0
}

const records = computed<ActivityRecord[]>(() => {
  const dispute = props.dispute
  const items: ActivityRecord[] = [{
    id: `${dispute.id}:application`,
    action: '买家提交售后申请',
    actor: participantLabel(dispute.primaryUserId),
    body: dispute.applicantStatement || dispute.publicSummary,
    createdAt: dispute.createdAt || dispute.openedAt,
    evidence: evidenceFor('dispute_initial', dispute.id),
  }]

  if (dispute.sellerDecidedAt) {
    items.push({
      id: `${dispute.id}:seller-decision`,
      action: dispute.sellerDecision === 'accepted' ? '卖家同意申请' : '卖家拒绝申请',
      actor: participantLabel(dispute.sellerDecidedByUserId || dispute.counterpartyUserId),
      body: dispute.sellerDecisionReason ?? '',
      createdAt: dispute.sellerDecidedAt,
      evidence: evidenceFor('formal_response', dispute.id),
      status: dispute.sellerResponseLate ? '逾期响应' : undefined,
    })
  } else if (dispute.respondedAt) {
    items.push({
      id: `${dispute.id}:legacy-response`,
      action: '被申请方提交正式答复',
      actor: participantLabel(dispute.respondedByUserId || dispute.counterpartyUserId),
      body: dispute.respondentResponse ?? '',
      createdAt: dispute.respondedAt,
      evidence: evidenceFor('formal_response', dispute.id),
      status: '旧流程',
    })
  }

  if (dispute.escalatedAt) {
    items.push({
      id: `${dispute.id}:platform-intervention`,
      action: '买家申请平台介入',
      actor: participantLabel(dispute.escalatedByUserId || dispute.primaryUserId),
      body: dispute.platformInterventionReason || '申请人已提交平台介入请求。',
      createdAt: dispute.escalatedAt,
      evidence: evidenceFor('platform_escalation', dispute.id),
    })
  }

  for (const supplement of dispute.supplements ?? []) {
    items.push({
      id: `${dispute.id}:supplement:${supplement.id}`,
      action: '提交平台要求的补充材料',
      actor: participantLabel(supplement.submittedByUserId),
      body: supplement.body,
      createdAt: supplement.createdAt,
      evidence: evidenceFor('info_supplement', supplement.id),
    })
  }

  for (const remedy of dispute.remedies ?? []) {
    if (remedy.source !== 'seller_acceptance') {
      items.push({
        id: `${dispute.id}:remedy:${remedy.id}:created`,
        action: remedy.source === 'admin_decision' ? '平台下达整改要求' : '双方方案进入履行',
        actor: remedy.source === 'admin_decision' ? '平台' : '双方参与方',
        body: remedy.instructions,
        createdAt: remedy.createdAt,
        evidence: [],
        status: apiOrderDisputeRemedyStatusLabels[remedy.status],
      })
    }
    if (remedy.claimedAt) {
      items.push({
        id: `${dispute.id}:remedy:${remedy.id}:claimed`,
        action: '责任方声明已履行',
        actor: participantLabel(remedy.responsibleUserId),
        body: remedy.claimNote ?? '',
        createdAt: remedy.claimedAt,
        evidence: evidenceFor('remedy_claim', remedy.id),
      })
    }
    if (remedy.contestedAt && !(remedy.source === 'seller_acceptance' && dispute.escalatedAt)) {
      items.push({
        id: `${dispute.id}:remedy:${remedy.id}:contested`,
        action: '受益方提出履行异议',
        actor: participantLabel(remedy.beneficiaryUserId),
        body: remedy.responseNote ?? '',
        createdAt: remedy.contestedAt,
        evidence: evidenceFor('remedy_contest', remedy.id),
      })
    }
    if (remedy.confirmedAt) {
      items.push({
        id: `${dispute.id}:remedy:${remedy.id}:confirmed`,
        action: '受益方确认履行完成',
        actor: participantLabel(remedy.beneficiaryUserId),
        body: remedy.responseNote ?? '',
        createdAt: remedy.confirmedAt,
        evidence: [],
      })
    }
    if (remedy.confirmationExpiredAt) {
      items.push({
        id: `${dispute.id}:remedy:${remedy.id}:expired`,
        action: '确认期届满，系统中性结案',
        actor: '系统',
        body: remedy.responseNote ?? '',
        createdAt: remedy.confirmationExpiredAt,
        evidence: [],
      })
    }
  }

  return items.sort((left, right) => timestamp(left.createdAt) - timestamp(right.createdAt))
})

const legacyEvidenceIDs = computed(() => new Set(
  (props.dispute.messages ?? []).flatMap(message => evidenceFor('message', message.id).map(item => item.id)),
))
const assignedEvidenceIDs = computed(() => new Set([
  ...records.value.flatMap(record => record.evidence.map(item => item.id)),
  ...legacyEvidenceIDs.value,
]))
const otherEvidence = computed(() => evidence.value.filter(item => !assignedEvidenceIDs.value.has(item.id)))
const hasLegacyHistory = computed(() => Boolean(props.dispute.messages?.length || props.dispute.settlementProposals?.length))
</script>

<template>
  <section class="space-y-5 border-b border-border py-5">
    <div class="flex items-center gap-2">
      <FileClock class="h-4 w-4" />
      <h2 class="text-sm font-semibold">售后处理记录</h2>
    </div>

    <dl class="grid gap-3 sm:grid-cols-3">
      <div>
        <dt class="text-xs text-muted-foreground">问题类型</dt>
        <dd class="mt-1 text-sm font-medium">{{ dispute.issueCode ? apiOrderDisputeIssueLabels[dispute.issueCode] : '历史案件' }}</dd>
      </div>
      <div>
        <dt class="text-xs text-muted-foreground">申请诉求</dt>
        <dd class="mt-1 text-sm font-medium">{{ dispute.requestedResolution ? apiOrderDisputeResolutionLabels[dispute.requestedResolution] : '未结构化记录' }}</dd>
      </div>
      <div>
        <dt class="text-xs text-muted-foreground">申请金额</dt>
        <dd class="mt-1 text-sm font-medium">{{ dispute.requestedAmountCny ? `¥${dispute.requestedAmountCny}` : '不涉及' }}</dd>
      </div>
    </dl>

    <ol class="relative ml-3 border-l border-border">
      <li v-for="record in records" :key="record.id" class="relative pb-6 pl-6 last:pb-0">
        <span class="absolute -left-[5px] top-1.5 h-2.5 w-2.5 rounded-full border-2 border-background bg-primary" aria-hidden="true" />
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="text-sm font-semibold">{{ record.action }}</h3>
              <Badge v-if="record.status" variant="secondary">{{ record.status }}</Badge>
            </div>
            <p class="mt-0.5 text-xs text-muted-foreground">{{ record.actor }}</p>
          </div>
          <LocalTime :value="record.createdAt" class="text-xs text-muted-foreground" />
        </div>
        <p v-if="record.body" class="mt-3 whitespace-pre-wrap break-words text-sm leading-6">{{ record.body }}</p>
        <DisputeEvidenceGallery v-if="record.evidence.length" :items="record.evidence" class="mt-4" />
      </li>
    </ol>

    <div v-if="otherEvidence.length" class="space-y-3 border-t border-border pt-4">
      <h3 class="text-sm font-semibold">其他案件图片材料</h3>
      <DisputeEvidenceGallery :items="otherEvidence" />
    </div>

    <div v-if="hasLegacyHistory" class="space-y-4 border-t border-border pt-5">
      <div class="flex items-center gap-2">
        <History class="h-4 w-4" />
        <h3 class="text-sm font-semibold">旧流程历史记录</h3>
      </div>
      <div v-if="dispute.messages?.length" class="space-y-4">
        <article v-for="message in dispute.messages" :key="message.id" class="border-l-2 border-border pl-4">
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>{{ participantLabel(message.senderUserId) }}</span>
            <LocalTime :value="message.createdAt" />
          </div>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ message.body }}</p>
          <DisputeEvidenceGallery v-if="evidenceFor('message', message.id).length" :items="evidenceFor('message', message.id)" class="mt-3" />
        </article>
      </div>
      <div v-if="dispute.settlementProposals?.length" class="space-y-3">
        <article v-for="proposal in dispute.settlementProposals" :key="proposal.id" class="border-l-2 border-warning pl-4">
          <div class="flex flex-wrap items-center gap-2">
            <MessageSquareText class="h-4 w-4 text-muted-foreground" />
            <span class="text-sm font-medium">{{ apiOrderDisputeResolutionLabels[proposal.resolution] }}</span>
            <Badge variant="secondary">{{ proposal.status }}</Badge>
          </div>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{{ proposal.terms }}</p>
        </article>
      </div>
    </div>
  </section>
</template>
