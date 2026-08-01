<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Ban, CheckCircle2, ShieldQuestion } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import {
  publicReputationBadges,
  reputationBadgeLabel,
  reputationConfidenceLabel,
  reputationRoleLabel,
  reputationStateLabel,
  reputationTierLabel,
} from '@/lib/reputationPresentation'
import type { ReputationSummary } from '@/types/reputation'

const props = defineProps<{
  summary: ReputationSummary | null | undefined
  compact?: boolean
}>()

const badges = computed(() => props.summary ? publicReputationBadges(props.summary) : [])
</script>

<template>
  <div
    class="text-xs"
    :class="props.compact ? 'min-w-0' : 'space-y-1.5'"
    :aria-label="summary ? `${reputationRoleLabel(summary.role)}摘要` : '信誉摘要'"
  >
    <div v-if="!summary" class="flex items-center gap-1.5 text-muted-foreground">
      <ShieldQuestion class="h-3.5 w-3.5" />
      信誉暂无数据
    </div>
    <template v-else>
      <div class="flex items-center gap-1.5" :class="props.compact ? 'min-w-0' : 'flex-wrap'">
        <Ban v-if="summary.state === 'restricted'" class="h-3.5 w-3.5 text-destructive" />
        <AlertTriangle v-else-if="summary.state === 'caution'" class="h-3.5 w-3.5 text-amber-600" />
        <CheckCircle2 v-else class="h-3.5 w-3.5 text-emerald-600" />
        <Badge :variant="summary.state === 'restricted' ? 'destructive' : summary.state === 'caution' ? 'secondary' : 'outline'">
          {{ reputationStateLabel(summary.state) }}
        </Badge>
        <strong class="shrink-0">{{ reputationTierLabel(summary.tier) }}</strong>
        <span class="truncate text-muted-foreground" :title="reputationConfidenceLabel(summary.confidence)">{{ reputationConfidenceLabel(summary.confidence) }}</span>
      </div>
      <p v-if="summary.state !== 'active' && !props.compact" class="leading-5 text-muted-foreground">
        {{ summary.warnings[0] || (summary.state === 'restricted' ? '部分交易操作当前不可用。' : '交易前请核对履约和纠纷事实。') }}
      </p>
      <div v-if="badges.length && !props.compact" class="flex flex-wrap gap-1">
        <Badge v-for="badge in badges" :key="badge" variant="trust">{{ reputationBadgeLabel(badge) }}</Badge>
      </div>
    </template>
  </div>
</template>
