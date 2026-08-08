<script setup lang="ts">
import { computed } from 'vue'
import type { ApiHealthSlotState, ApiServiceHealthSummary } from '@/types/apiHealth'

const props = withDefaults(defineProps<{
  summary: ApiServiceHealthSummary | null | undefined
  compact?: boolean
}>(), { compact: false })

const buckets = computed(() => Array.from({ length: 24 }, (_, index) => props.summary?.hourlyBuckets?.[index] ?? {
  hourStartedAt: '',
  state: 'no_sample' as const,
  completedCycles: 0,
  firstAttemptSuccesses: 0,
  retryRecoveries: 0,
  finalFailures: 0,
  slowSuccesses: 0,
  finalSuccessPercent: null,
  averageTtftMs: null,
}))

function stateClass(state: ApiHealthSlotState) {
  if (state === 'smooth') return 'bg-success'
  if (state === 'fluctuating') return 'bg-signal'
  if (state === 'abnormal') return 'bg-destructive'
  return 'bg-muted-foreground/20'
}

function stateLabel(state: ApiHealthSlotState) {
  if (state === 'smooth') return '正常'
  if (state === 'fluctuating') return '轻微波动'
  if (state === 'abnormal') return '不可用'
  return '无样本'
}

function formatHour(value: string) {
  if (!value) return '时间未知'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间未知'
  const end = new Date(date.getTime() + 3_600_000)
  const formatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
  return `${formatter.format(date)} - ${formatter.format(end)}`
}

function duration(value: number | null | undefined) {
  if (value === null || value === undefined) return '—'
  return value < 1000 ? `${value}ms` : `${(value / 1000).toFixed(2)}s`
}

function bucketTitle(bucket: typeof buckets.value[number]) {
  return [
    formatHour(bucket.hourStartedAt),
    `状态：${stateLabel(bucket.state)}`,
    `检测 ${bucket.completedCycles} 次`,
    `首次成功 ${bucket.firstAttemptSuccesses} 次`,
    `异常恢复 ${bucket.retryRecoveries} 次`,
    `最终失败 ${bucket.finalFailures} 次`,
    `平均首字 ${duration(bucket.averageTtftMs)}`,
  ].join('\n')
}

const detailTitle = computed(() => {
  const summary = props.summary
  if (!summary) return '近 24 小时暂无探针数据'
  const protocol = summary.probeProtocol === 'openai_chat_completions_v1' ? 'Chat Completions（回退）' : 'Responses'
  const cost = summary.cost.projectedDailyCostUsd ? `$${summary.cost.projectedDailyCostUsd}` : '未知'
  return [
    '近 24 小时真实模型探针',
    `探针模型：${summary.probeModel ?? '未配置'}`,
    `协议：${protocol}`,
    `测量位置：${summary.probeEnvironmentLabel ?? '平台美西'}`,
    '频率：每 5 分钟',
    `检测：${summary.completedCycles} / ${summary.theoreticalSlots}`,
    `采样覆盖：${summary.coveragePercent}%`,
    `首次成功：${summary.firstAttemptSuccesses} 次`,
    `异常恢复：${summary.retryRecoveries} 次`,
    `最终失败：${summary.finalFailures} 次`,
    `平均首字：${duration(summary.averageTtftMs)}`,
    `P50 / P95：${duration(summary.p50TtftMs)} / ${duration(summary.p95TtftMs)}`,
    `滚动日预计费用：${cost}${summary.cost.hasUnknownUsage ? '（部分用量未知）' : ''}`,
  ].join('\n')
})
</script>

<template>
  <div class="min-w-0" :title="detailTitle">
    <div class="flex items-end justify-between gap-3 tabular-nums">
      <div>
        <span class="block text-[10px] leading-4 text-muted-foreground">稳定性</span>
        <strong class="block text-sm font-semibold leading-4">{{ summary?.stabilityPercent ? `${summary.stabilityPercent}%` : '—' }}</strong>
      </div>
      <div class="text-right">
        <span class="block text-[10px] leading-4 text-muted-foreground">平均首字</span>
        <strong class="block text-sm font-semibold leading-4">{{ duration(summary?.averageTtftMs) }}</strong>
      </div>
    </div>
    <div class="mt-2 grid grid-cols-[repeat(24,minmax(0,1fr))] gap-[3px]" aria-label="近 24 小时真实模型探针状态">
      <span
        v-for="(bucket, index) in buckets"
        :key="bucket.hourStartedAt || index"
        class="h-3 min-w-0 rounded-[2px]"
        :class="stateClass(bucket.state)"
        :title="bucketTitle(bucket)"
        :aria-label="bucketTitle(bucket)"
      />
    </div>
    <div v-if="!compact" class="mt-1.5 flex items-center justify-between gap-3 text-[10px] leading-4 text-muted-foreground">
      <span>近 24 小时</span>
      <span>覆盖 {{ summary?.completedCycles ?? 0 }} / {{ summary?.theoreticalSlots ?? 288 }}</span>
    </div>
  </div>
</template>
