<script setup lang="ts">
import { computed, ref } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import Info from 'lucide-vue-next/dist/esm/icons/info.js'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import ApiHealth24HourStrip from '@/components/api-market/ApiHealth24HourStrip.vue'
import type { ApiHealthAvailabilityReason, ApiHealthState, ApiServiceHealthSummary } from '@/types/apiHealth'

const props = defineProps<{ summary: ApiServiceHealthSummary | null | undefined }>()
const detailsOpen = ref(false)
const detailsPinned = ref(false)

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

function formatDuration(value: number | null | undefined) {
  if (value === null || value === undefined) return '—'
  return value < 1000 ? `${value}ms` : `${(value / 1000).toFixed(2)}s`
}

const protocolLabel = computed(() => props.summary?.probeProtocol === 'openai_chat_completions_v1'
  ? 'Chat Completions（回退）'
  : props.summary?.probeProtocol === 'openai_responses_v1'
    ? 'Responses'
    : '未配置')

const projectedDailyCost = computed(() => {
  const cost = props.summary?.cost
  if (!cost?.projectedDailyCostUsd) return '未知'
  return `$${cost.projectedDailyCostUsd}${cost.hasUnknownUsage ? '（部分用量未知）' : ''}`
})

function updateDetailsOpen(open: boolean) {
  if (!open && detailsPinned.value) return
  detailsOpen.value = open
}

function toggleDetails() {
  detailsPinned.value = !detailsPinned.value
  detailsOpen.value = detailsPinned.value
}

function closeDetails() {
  detailsPinned.value = false
  detailsOpen.value = false
}

function closeDetailsFromOutside(event: Event) {
  if (event.defaultPrevented) return
  closeDetails()
}
</script>

<template>
  <section class="border-t border-border bg-card px-2.5 py-2" aria-label="近 24 小时真实模型探针">
    <div class="mb-2 flex min-w-0 items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-1.5">
        <Activity class="h-3.5 w-3.5 shrink-0 text-emerald-600" aria-hidden="true" />
        <span class="shrink-0 text-xs font-semibold">真实模型探针</span>
        <TooltipProvider :delay-duration="150">
          <Tooltip :open="detailsOpen" disable-closing-trigger @update:open="updateDetailsOpen">
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="size-5 rounded-sm text-muted-foreground"
                aria-label="查看探针统计详情"
                @click="toggleDetails"
              >
                <Info class="size-3" aria-hidden="true" />
              </Button>
            </TooltipTrigger>
            <TooltipContent
              side="bottom"
              align="center"
              class="w-[min(22rem,calc(100vw-2rem))] max-w-none overflow-visible p-0 text-left text-xs leading-5 text-pretty"
              @escape-key-down="closeDetails"
              @pointer-down-outside="closeDetailsFromOutside"
            >
              <div class="max-h-[min(28rem,var(--reka-tooltip-content-available-height),calc(100vh-2rem))] overflow-y-auto overscroll-contain p-3">
                <p class="font-semibold">近 24 小时真实模型探针</p>
                <p v-if="!summary" class="mt-1 text-background/75">暂无探针数据</p>
                <dl v-else class="mt-2 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-3 gap-y-1">
                  <dt class="text-background/70">探针模型</dt><dd class="min-w-0 break-words font-medium">{{ summary.probeModel ?? '未配置' }}</dd>
                  <dt class="text-background/70">协议</dt><dd>{{ protocolLabel }}</dd>
                  <dt class="text-background/70">测量位置</dt><dd>{{ summary.probeEnvironmentLabel ?? '平台美西' }}</dd>
                  <dt class="text-background/70">检测频率</dt><dd>每 5 分钟</dd>
                  <dt class="text-background/70">检测次数</dt><dd>{{ summary.completedCycles }} / {{ summary.theoreticalSlots }}</dd>
                  <dt class="text-background/70">采样覆盖</dt><dd>{{ summary.coveragePercent }}%</dd>
                  <dt class="text-background/70">首次成功</dt><dd>{{ summary.firstAttemptSuccesses }} 次</dd>
                  <dt class="text-background/70">异常恢复</dt><dd>{{ summary.retryRecoveries }} 次</dd>
                  <dt class="text-background/70">最终失败</dt><dd>{{ summary.finalFailures }} 次</dd>
                  <dt class="text-background/70">平均首字</dt><dd>{{ formatDuration(summary.averageTtftMs) }}</dd>
                  <dt class="text-background/70">P50 / P95</dt><dd>{{ formatDuration(summary.p50TtftMs) }} / {{ formatDuration(summary.p95TtftMs) }}</dd>
                  <dt class="text-background/70">预计日费用</dt><dd>{{ projectedDailyCost }}</dd>
                </dl>
                <p class="mt-2 border-t border-background/15 pt-2 text-background/70">
                  延迟从平台美西测量；稳定性按首次请求成功率计算，重试恢复仍记为波动。
                </p>
              </div>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
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
