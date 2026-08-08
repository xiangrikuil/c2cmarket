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
  no_sample: '暂无样本',
}

const reasonLabels: Record<Exclude<ApiHealthAvailabilityReason, null>, string> = {
  unconfigured: '尚未绑定探针连接',
  disabled: '探针连接已停用',
  unverified: '探针连接尚未通过鉴权验证',
  insufficient: '最近一小时样本不足',
  stale: '最近样本已过期',
  temporarily_unavailable: '探针连接暂时不可用',
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
  <section class="api-service-health-panel" aria-label="探针连接近期可用性">
    <div class="flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-1.5">
        <Activity class="h-3.5 w-3.5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span class="shrink-0 text-xs font-semibold">探针连接可用性</span>
        <time
          class="truncate text-[11px] text-muted-foreground"
          :datetime="summary?.lastSampledAt ?? undefined"
          :title="formatSampleTime(summary?.lastSampledAt)"
        >
          {{ formatSampleTime(summary?.lastSampledAt) }}
        </time>
      </div>
      <Badge :variant="statusVariant" class="h-5 shrink-0 px-1.5 text-[11px] font-medium">{{ stateLabel }}</Badge>
    </div>

    <div class="api-service-health-panel__metrics">
      <div>
        <span>成功率</span>
        <strong>{{ summary?.successRatePercent === null || summary?.successRatePercent === undefined ? '—' : `${summary.successRatePercent}%` }}</strong>
      </div>
      <div>
        <span>近 1 小时</span>
        <strong>{{ summary ? `${summary.successfulSamples} / ${summary.totalSamples}` : '0 / 0' }}</strong>
      </div>
      <div>
        <span>连接方式</span>
        <strong>{{ summary?.transportSecurity === 'insecure_http' ? 'HTTP' : summary?.transportSecurity === 'secure_https' ? 'HTTPS' : '—' }}</strong>
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

    <div class="mt-1.5 flex min-w-0 items-center gap-1 text-[10px] leading-4 text-muted-foreground">
      <span class="min-w-0 truncate" :title="availabilityLabel ?? '仅代表 Base URL 与专用探针 Key 的鉴权可用性，不代表任一具体模型可调用'">
        {{ availabilityLabel ?? '仅代表连接鉴权可用性，不代表任一具体模型可调用' }}
      </span>
      <template v-if="summary?.transportSecurity === 'insecure_http'">
        <span aria-hidden="true">·</span>
        <span
          class="shrink-0 font-medium text-amber-700"
          title="本次探测使用未加密 HTTP，API Key 和请求响应可能在传输途中被读取或篡改，结果可信度低于 HTTPS。"
        >HTTP 未加密</span>
      </template>
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
  font-size: 10px;
  line-height: 13px;
}

.api-service-health-panel__metrics strong {
  margin-top: 1px;
  color: var(--foreground);
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 14px;
}
</style>
