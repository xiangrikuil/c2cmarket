<script setup lang="ts">
import { computed } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import Clock3 from 'lucide-vue-next/dist/esm/icons/clock-3.js'
import Gauge from 'lucide-vue-next/dist/esm/icons/gauge.js'
import Server from 'lucide-vue-next/dist/esm/icons/server.js'
import { Badge } from '@/components/ui/badge'
import type { ApiHealthAvailabilityReason, ApiHealthSlotState, ApiHealthState, ApiServiceHealthSummary } from '@/types/apiHealth'

const props = defineProps<{
  summary: ApiServiceHealthSummary | null | undefined
}>()

const stateLabels: Record<ApiHealthState, string> = {
  normal: '正常',
  fluctuating: '波动',
  abnormal: '异常',
  no_sample: '暂无数据',
}

const reasonLabels: Record<Exclude<ApiHealthAvailabilityReason, null>, string> = {
  unconfigured: '尚未配置平台探针',
  disabled: '平台探针已停用',
  unauthorized: '探针目标尚未完成授权',
  insufficient: '最近一小时样本不足',
  stale: '最近样本已过期',
  temporarily_unavailable: '探针系统暂时不可用',
}

const slotLabels: Record<ApiHealthSlotState, string> = {
  smooth: '顺畅',
  fluctuating: '波动',
  abnormal: '异常',
  no_sample: '无样本',
}

const state = computed<ApiHealthState>(() => props.summary?.state ?? 'no_sample')
const stateLabel = computed(() => stateLabels[state.value])
const availabilityLabel = computed(() => {
  const reason = props.summary?.availabilityReason
  return reason ? reasonLabels[reason] : null
})
const slots = computed(() => Array.from({ length: 12 }, (_, index) => props.summary?.samples[index] ?? {
  slotStartedAt: '',
  state: 'no_sample' as const,
}))
const statusVariant = computed(() => state.value === 'normal'
  ? 'verified'
  : state.value === 'abnormal'
    ? 'destructive'
    : state.value === 'fluctuating'
      ? 'secondary'
      : 'outline')

function slotClass(slotState: ApiHealthSlotState) {
  if (slotState === 'smooth') return 'bg-success'
  if (slotState === 'fluctuating') return 'bg-signal'
  if (slotState === 'abnormal') return 'bg-destructive'
  return 'bg-muted-foreground/20'
}

function formatSampleTime(value: string | null | undefined) {
  if (!value) return '暂无更新时间'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '暂无更新时间'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function slotTitle(slotStartedAt: string, slotState: ApiHealthSlotState) {
  const time = slotStartedAt ? formatSampleTime(slotStartedAt) : '时间槽'
  return `${time}：${slotLabels[slotState]}`
}
</script>

<template>
  <section class="min-h-[168px] border-t border-border pt-4" aria-label="平台近期健康探测">
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <Activity class="h-4 w-4 text-primary" aria-hidden="true" />
          <span>平台近期测量</span>
        </div>
        <p class="mt-1 truncate text-xs text-muted-foreground" :title="summary?.probeModel ?? '未配置探测模型'">
          {{ summary?.probeModel ? `模型 ${summary.probeModel}` : '未配置探测模型' }}
        </p>
      </div>
      <Badge :variant="statusVariant">{{ stateLabel }}</Badge>
    </div>

    <div class="mt-4 grid grid-cols-3 gap-2 text-sm">
      <div class="min-w-0">
        <div class="flex items-center gap-1 text-xs text-muted-foreground"><Gauge class="h-3.5 w-3.5" />成功率</div>
        <div class="mt-1 font-semibold tabular-nums">{{ summary?.successRatePercent ? `${summary.successRatePercent}%` : '—' }}</div>
      </div>
      <div class="min-w-0">
        <div class="flex items-center gap-1 text-xs text-muted-foreground"><Clock3 class="h-3.5 w-3.5" />TTFT 中位</div>
        <div class="mt-1 font-semibold tabular-nums">{{ summary?.medianTtftMs === null || summary?.medianTtftMs === undefined ? '—' : `${summary.medianTtftMs}ms` }}</div>
      </div>
      <div class="min-w-0">
        <div class="flex items-center gap-1 text-xs text-muted-foreground"><Server class="h-3.5 w-3.5" />成功样本</div>
        <div class="mt-1 font-semibold tabular-nums">{{ summary ? `${summary.successfulSamples}/${summary.totalSamples}` : '0/0' }}</div>
      </div>
    </div>

    <div class="mt-4 grid grid-cols-12 gap-1" aria-label="最近一小时五分钟探测槽">
      <span
        v-for="(slot, index) in slots"
        :key="slot.slotStartedAt || index"
        class="h-2.5 min-w-0 rounded-[2px]"
        :class="slotClass(slot.state)"
        :title="slotTitle(slot.slotStartedAt, slot.state)"
        :aria-label="slotTitle(slot.slotStartedAt, slot.state)"
      />
    </div>

    <div class="mt-3 flex min-h-5 items-center justify-between gap-3 text-xs text-muted-foreground">
      <span class="min-w-0 truncate" :title="availabilityLabel ?? '只代表当前探测模型与平台单节点'">
        {{ availabilityLabel ?? '仅代表当前模型与平台单节点' }}
      </span>
      <time class="shrink-0 tabular-nums" :datetime="summary?.lastSampledAt ?? undefined">
        {{ formatSampleTime(summary?.lastSampledAt) }}
      </time>
    </div>
  </section>
</template>
