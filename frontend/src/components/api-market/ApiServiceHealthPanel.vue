<script setup lang="ts">
import { computed } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
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
  <section class="api-service-health-panel" aria-label="平台近期健康探测">
    <div class="flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-1.5">
        <Activity class="h-3.5 w-3.5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span class="shrink-0 text-xs font-semibold">平台探测</span>
        <time
          class="truncate text-[10px] text-muted-foreground"
          :datetime="summary?.lastSampledAt ?? undefined"
          :title="formatSampleTime(summary?.lastSampledAt)"
        >
          {{ formatSampleTime(summary?.lastSampledAt) }}
        </time>
      </div>
      <Badge :variant="statusVariant" class="h-5 shrink-0 px-1.5 text-[10px]">{{ stateLabel }}</Badge>
    </div>

    <div class="api-service-health-panel__metrics">
      <div>
        <span>成功率</span>
        <strong>{{ summary?.successRatePercent === null || summary?.successRatePercent === undefined ? '—' : `${summary.successRatePercent}%` }}</strong>
      </div>
      <div>
        <span>首字中位</span>
        <strong>{{ summary?.medianTtftMs === null || summary?.medianTtftMs === undefined ? '—' : `${summary.medianTtftMs}ms` }}</strong>
      </div>
      <div>
        <span>近 1 小时</span>
        <strong>{{ summary ? `${summary.successfulSamples} / ${summary.totalSamples}` : '0 / 0' }}</strong>
      </div>
    </div>

    <div class="mt-1.5 grid grid-cols-12 gap-1" aria-label="最近一小时五分钟探测槽">
      <span
        v-for="(slot, index) in slots"
        :key="slot.slotStartedAt || index"
        class="h-1.5 min-w-0 rounded-[2px]"
        :class="slotClass(slot.state)"
        :title="slotTitle(slot.slotStartedAt, slot.state)"
        :aria-label="slotTitle(slot.slotStartedAt, slot.state)"
      />
    </div>

    <div class="mt-1.5 flex min-w-0 items-center gap-1 text-[9px] text-muted-foreground">
      <span class="min-w-0 truncate" :title="summary?.probeModel ?? '未配置探测模型'">
        {{ summary?.probeModel ? `模型 ${summary.probeModel}` : '未配置探测模型' }}
      </span>
      <span aria-hidden="true">·</span>
      <span class="min-w-0 truncate" :title="availabilityLabel ?? '仅代表当前模型与平台单节点'">
        {{ availabilityLabel ?? '仅代表当前模型与平台单节点' }}
      </span>
    </div>
  </section>
</template>

<style scoped>
.api-service-health-panel {
  border-top: 1px solid var(--border);
  padding: 7px 9px 7px;
  background: #fff;
}

.api-service-health-panel__metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-top: 5px;
}

.api-service-health-panel__metrics > div {
  min-width: 0;
}

.api-service-health-panel__metrics > div + div {
  border-left: 1px solid var(--border);
  padding-left: 7px;
}

.api-service-health-panel__metrics span,
.api-service-health-panel__metrics strong {
  display: block;
}

.api-service-health-panel__metrics span {
  color: var(--muted-foreground);
  font-size: 9px;
  line-height: 12px;
}

.api-service-health-panel__metrics strong {
  margin-top: 1px;
  color: var(--foreground);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 14px;
}
</style>
