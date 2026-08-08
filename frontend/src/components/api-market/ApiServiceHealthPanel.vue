<script setup lang="ts">
import { computed } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import Info from 'lucide-vue-next/dist/esm/icons/info.js'
import { Badge } from '@/components/ui/badge'
import ApiHealth24HourStrip from '@/components/api-market/ApiHealth24HourStrip.vue'
import type { ApiHealthAvailabilityReason, ApiHealthState, ApiServiceHealthSummary } from '@/types/apiHealth'

const props = defineProps<{ summary: ApiServiceHealthSummary | null | undefined }>()

const stateLabels: Record<ApiHealthState, string> = {
  normal: '正常',
  fluctuating: '波动',
  abnormal: '异常',
  no_sample: '暂无样本',
}

const reasonLabels: Record<Exclude<ApiHealthAvailabilityReason, null>, string> = {
  unconfigured: '尚未绑定探针连接',
  disabled: '探针连接已停用',
  unverified: '探针连接尚未完成真实模型验证',
  insufficient: '样本积累中',
  stale: '最近样本已过期',
  temporarily_unavailable: '探针统计暂时不可用',
  runner_disabled: '平台探针任务未运行',
}

const state = computed<ApiHealthState>(() => props.summary?.state ?? 'no_sample')
const availabilityLabel = computed(() => props.summary?.availabilityReason ? reasonLabels[props.summary.availabilityReason] : null)
const statusVariant = computed(() => state.value === 'normal'
  ? 'verified'
  : state.value === 'abnormal'
    ? 'destructive'
    : state.value === 'fluctuating'
      ? 'secondary'
      : 'outline')

function formatSampleTime(value: string | null | undefined) {
  if (!value) return '暂无更新时间'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '暂无更新时间'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}
</script>

<template>
  <section class="border-t border-border bg-card px-2.5 py-2" aria-label="近 24 小时真实模型探针">
    <div class="mb-2 flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-1.5">
        <Activity class="h-3.5 w-3.5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span class="shrink-0 text-xs font-semibold">真实模型探针</span>
        <Info class="h-3 w-3 shrink-0 text-muted-foreground" title="延迟从平台美西测量；稳定性按首次请求成功率计算，重试恢复仍记为波动。" />
        <time class="truncate text-[10px] text-muted-foreground" :datetime="summary?.lastSampledAt ?? undefined">{{ formatSampleTime(summary?.lastSampledAt) }}</time>
      </div>
      <Badge :variant="statusVariant" class="h-5 shrink-0 px-1.5 text-[10px] font-medium">{{ stateLabels[state] }}</Badge>
    </div>

    <ApiHealth24HourStrip :summary="summary" />

    <div class="mt-1.5 flex min-w-0 items-center gap-1 text-[10px] leading-4 text-muted-foreground">
      <span v-if="summary?.probeModelChangedAt" class="shrink-0 font-medium text-amber-700">探针模型已变更</span>
      <span class="min-w-0 truncate">{{ availabilityLabel ?? `探针：${summary?.probeModel ?? '未配置'}` }}</span>
      <template v-if="summary?.probeProtocol === 'openai_chat_completions_v1'"><span>·</span><span class="shrink-0">Chat 回退</span></template>
      <template v-if="summary?.transportSecurity === 'insecure_http'"><span>·</span><span class="shrink-0 font-medium text-amber-700">HTTP 未加密</span></template>
    </div>
  </section>
</template>
