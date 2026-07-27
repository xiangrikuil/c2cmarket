<script setup lang="ts">
import type { ReputationMetrics } from '@/types/reputation'

defineProps<{ metrics: ReputationMetrics }>()

function percentage(value: number | null) {
  return value === null ? '暂无数据' : `${Math.round(value * 100)}%`
}

function rating(value: number | null) {
  return value === null ? '暂无数据' : value.toFixed(2)
}
</script>

<template>
  <dl class="grid gap-x-6 gap-y-4 sm:grid-cols-2 lg:grid-cols-4">
    <div>
      <dt class="text-xs text-muted-foreground">可验证完成</dt>
      <dd class="mt-1 text-lg font-semibold">{{ metrics.completedCount }}</dd>
      <p class="text-xs text-muted-foreground">近 90 天 {{ metrics.completedCountLast90Days }}</p>
    </div>
    <div>
      <dt class="text-xs text-muted-foreground">角色完成率</dt>
      <dd class="mt-1 text-lg font-semibold">{{ percentage(metrics.roleCompletionRate) }}</dd>
      <p class="text-xs text-muted-foreground">责任取消率 {{ percentage(metrics.roleFaultCancelRate) }}</p>
    </div>
    <div>
      <dt class="text-xs text-muted-foreground">已验证评价</dt>
      <dd class="mt-1 text-lg font-semibold">{{ metrics.verifiedReviewCount }}</dd>
      <p class="text-xs text-muted-foreground">修正评分 {{ rating(metrics.weightedRating) }}</p>
    </div>
    <div>
      <dt class="text-xs text-muted-foreground">纠纷与限制</dt>
      <dd class="mt-1 text-lg font-semibold">{{ metrics.unresolvedDisputeCount }} / {{ metrics.activeRestrictionCount }}</dd>
      <p class="text-xs text-muted-foreground">未解决纠纷 / 有效限制</p>
    </div>
  </dl>
</template>
