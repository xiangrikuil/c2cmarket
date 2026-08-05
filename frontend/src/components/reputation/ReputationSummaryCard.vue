<script setup lang="ts">
import { computed } from 'vue'
import { Ban, CheckCircle2, ShieldQuestion, TriangleAlert } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  publicReputationBadges,
  reputationBadgeLabel,
  reputationConfidenceLabel,
  reputationRoleLabel,
  reputationScopeLabel,
  reputationStateLabel,
  reputationTierLabel,
} from '@/lib/reputationPresentation'
import type { ReputationSummary } from '@/types/reputation'

const props = withDefaults(defineProps<{
  summary: ReputationSummary | null | undefined
  compact?: boolean
  framed?: boolean
  showSourceAuthorVerification?: boolean
}>(), {
  compact: false,
  framed: true,
  showSourceAuthorVerification: true,
})

const badges = computed(() => {
  if (!props.summary) return []
  const values = publicReputationBadges(props.summary)
  return props.showSourceAuthorVerification ? values : values.filter(value => value !== 'source_verified')
})
const faultRate = computed(() => {
  const value = props.summary?.roleFaultCancelRate
  return value === null || value === undefined ? '暂无数据' : `${Math.round(value * 100)}%`
})
const rating = computed(() => {
  const value = props.summary?.weightedRating
  return value === null || value === undefined ? '暂无数据' : value.toFixed(2)
})
</script>

<template>
  <section
    class="space-y-3"
    :class="framed ? 'rounded-lg border border-border bg-card p-4' : ''"
    :aria-label="summary ? `${reputationRoleLabel(summary.role)}摘要` : '信誉摘要'"
  >
    <div v-if="!summary" class="flex items-center gap-2 text-sm text-muted-foreground">
      <ShieldQuestion class="h-4 w-4" />
      信誉暂无数据
    </div>

    <template v-else>
      <Alert v-if="summary.state === 'restricted'" variant="destructive">
        <Ban />
        <AlertTitle>{{ reputationStateLabel(summary.state) }}</AlertTitle>
        <AlertDescription>{{ summary.warnings[0] || '部分交易操作当前不可用，请以页面操作状态为准。' }}</AlertDescription>
      </Alert>
      <Alert v-else-if="summary.state === 'caution'">
        <TriangleAlert class="text-warning" />
        <AlertTitle>{{ reputationStateLabel(summary.state) }}</AlertTitle>
        <AlertDescription>{{ summary.warnings[0] || '存在待核对事实，交易前请查看完成与纠纷信息。' }}</AlertDescription>
      </Alert>

      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="text-xs text-muted-foreground">
            {{ reputationRoleLabel(summary.role) }} · {{ reputationScopeLabel(summary.scope) }}
          </div>
          <div class="mt-1 flex items-center gap-2">
            <CheckCircle2 v-if="summary.state === 'active'" class="h-4 w-4 text-emerald-600" />
            <strong class="text-base">{{ reputationTierLabel(summary.tier) }}</strong>
            <span class="text-xs text-muted-foreground">{{ reputationConfidenceLabel(summary.confidence) }}</span>
          </div>
        </div>
        <Badge
          :variant="summary.state === 'restricted' ? 'destructive' : summary.state === 'caution' ? 'secondary' : 'outline'"
        >
          {{ reputationStateLabel(summary.state) }}
        </Badge>
      </div>

      <div v-if="badges.length" class="flex flex-wrap gap-1.5">
        <Badge v-for="badge in badges" :key="badge" variant="trust">{{ reputationBadgeLabel(badge) }}</Badge>
      </div>

      <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm" :class="compact ? '' : 'sm:grid-cols-4'">
        <div>
          <dt class="text-xs text-muted-foreground">可验证完成</dt>
          <dd class="mt-0.5 font-medium">{{ summary.completedCount }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted-foreground">责任取消率</dt>
          <dd class="mt-0.5 font-medium">{{ faultRate }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted-foreground">未解决纠纷</dt>
          <dd class="mt-0.5 font-medium">{{ summary.unresolvedDisputes }}</dd>
        </div>
        <div>
          <dt class="text-xs text-muted-foreground">修正评分</dt>
          <dd class="mt-0.5 font-medium">{{ rating }}</dd>
        </div>
      </dl>
    </template>
  </section>
</template>
