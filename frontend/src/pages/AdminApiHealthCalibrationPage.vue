<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Activity, RefreshCw, Save } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useAdminAPIProbeCalibration,
  useAdminAPIProbeLatencyRules,
  usePreviewAdminAPIProbeLatencyRuleMutation,
  usePublishAdminAPIProbeLatencyRuleMutation,
} from '@/queries/useAdminApiHealthQueries'
import type { ApiProbeLatencyRuleInput, ApiProbeProtocol } from '@/types/apiHealth'

const model = ref('gpt-5.6-luna')
const protocol = ref<ApiProbeProtocol>('openai_responses_v1')
const slowTtftMs = ref('')
const hardTimeoutMs = ref('30000')
const dimension = computed(() => ({ model: model.value.trim(), protocol: protocol.value, environment: 'us-west-v1' }))
const calibrationQuery = useAdminAPIProbeCalibration(dimension)
const rulesQuery = useAdminAPIProbeLatencyRules()
const previewMutation = usePreviewAdminAPIProbeLatencyRuleMutation()
const publishMutation = usePublishAdminAPIProbeLatencyRuleMutation()
const calibration = computed(() => calibrationQuery.data.value)
const rules = computed(() => rulesQuery.data.value ?? [])
const activeRule = computed(() => rules.value.find(rule => rule.model === dimension.value.model && rule.protocol === dimension.value.protocol && rule.environment === 'us-west-v1' && rule.status === 'active'))
const thresholdValid = computed(() => {
  const slow = Number(slowTtftMs.value)
  const hard = Number(hardTimeoutMs.value)
  return Number.isInteger(slow) && Number.isInteger(hard) && slow > 0 && hard > slow && hard <= 30000
})

function requestInput(): ApiProbeLatencyRuleInput {
  return { ...dimension.value, slowTtftMs: Number(slowTtftMs.value), hardTimeoutMs: Number(hardTimeoutMs.value) }
}

function duration(value: number | null | undefined) {
  if (value === null || value === undefined) return '—'
  return value < 1000 ? `${value}ms` : `${(value / 1000).toFixed(2)}s`
}

async function preview() {
  if (!thresholdValid.value) return
  try { await previewMutation.mutateAsync(requestInput()) }
  catch (error) { toast.error(backendErrorMessage(error, '阈值影响预览失败。')) }
}

async function publish() {
  if (!calibration.value?.ready || !thresholdValid.value) return
  if (!window.confirm(`发布 ${model.value} 的慢响应规则？新规则只影响后续样本。`)) return
  try {
    await publishMutation.mutateAsync(requestInput())
    toast.success('慢响应规则已发布。')
  } catch (error) { toast.error(backendErrorMessage(error, '慢响应规则发布失败。')) }
}

watch(dimension, () => {
  previewMutation.reset()
  slowTtftMs.value = ''
})
</script>

<template>
  <div class="mx-auto w-full max-w-[1280px] space-y-5">
    <PageTitle title="探针延迟校准" description="管理固定美西探针环境的绝对慢响应规则。">
      <template #action><Button variant="outline" :disabled="calibrationQuery.isFetching.value" @click="calibrationQuery.refetch()"><RefreshCw class="h-4 w-4" />刷新观测</Button></template>
    </PageTitle>

    <div class="grid gap-3 rounded-md border border-border bg-card p-4 md:grid-cols-[minmax(220px,1fr)_240px_180px]">
      <label class="space-y-1.5"><span class="text-xs font-medium">模型 ID</span><Input v-model="model" class="font-mono" /></label>
      <label class="space-y-1.5"><span class="text-xs font-medium">探针协议</span><Select v-model="protocol"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="openai_responses_v1">Responses</SelectItem><SelectItem value="openai_chat_completions_v1">Chat Completions</SelectItem></SelectContent></Select></label>
      <div class="space-y-1.5"><span class="text-xs font-medium">测量环境</span><div class="flex h-9 items-center rounded-md bg-muted/50 px-3 text-sm">平台美西</div></div>
    </div>

    <ErrorState v-if="calibrationQuery.error.value" title="无法读取校准数据" :description="backendErrorMessage(calibrationQuery.error.value, '校准数据暂时不可用。')" @retry="calibrationQuery.refetch()" />
    <template v-else>
      <div class="grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-3 lg:grid-cols-6">
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">完整自然日</span><strong class="mt-1 block text-xl">{{ calibration?.completeCalendarDays ?? 0 }}</strong></div>
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">独立连接</span><strong class="mt-1 block text-xl">{{ calibration?.connectionCount ?? 0 }}</strong></div>
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">首次成功样本</span><strong class="mt-1 block text-xl">{{ calibration?.sampleCount ?? 0 }}</strong></div>
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">P50</span><strong class="mt-1 block text-xl">{{ duration(calibration?.p50TtftMs) }}</strong></div>
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">P95</span><strong class="mt-1 block text-xl">{{ duration(calibration?.p95TtftMs) }}</strong></div>
        <div class="bg-card px-4 py-3"><span class="text-xs text-muted-foreground">P99</span><strong class="mt-1 block text-xl">{{ duration(calibration?.p99TtftMs) }}</strong></div>
      </div>

      <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
        <section class="rounded-md border border-border bg-card p-4">
          <div class="flex items-center justify-between gap-3"><div><h2 class="text-sm font-semibold">固定阈值</h2><p class="mt-1 text-xs text-muted-foreground">P90 {{ duration(calibration?.p90TtftMs) }} · 只分类发布后的新样本</p></div><Badge :variant="calibration?.ready ? 'verified' : 'outline'">{{ calibration?.ready ? '可发布' : '继续观测' }}</Badge></div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2"><label class="space-y-1.5"><span class="text-xs font-medium">慢响应 X（ms）</span><Input v-model="slowTtftMs" inputmode="numeric" placeholder="例如 5000" /></label><label class="space-y-1.5"><span class="text-xs font-medium">硬超时 Y（ms）</span><Input v-model="hardTimeoutMs" inputmode="numeric" /></label></div>
          <div class="mt-4 flex flex-wrap gap-2"><Button variant="outline" :disabled="!thresholdValid || previewMutation.isPending.value" @click="preview"><Activity class="h-4 w-4" />预览影响</Button><Button :disabled="!calibration?.ready || !thresholdValid || publishMutation.isPending.value" @click="publish"><Save class="h-4 w-4" />发布规则</Button></div>
          <div v-if="previewMutation.data.value" class="mt-4 grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-2"><div class="bg-background p-3"><span class="text-xs text-muted-foreground">预计慢响应黄色</span><strong class="mt-1 block">{{ previewMutation.data.value.slowSampleCount }} · {{ previewMutation.data.value.slowPercent }}%</strong></div><div class="bg-background p-3"><span class="text-xs text-muted-foreground">超过硬超时</span><strong class="mt-1 block">{{ previewMutation.data.value.overTimeoutCount }} · {{ previewMutation.data.value.overTimeoutPercent }}%</strong></div></div>
        </section>

        <section class="rounded-md border border-border bg-card p-4"><div class="flex items-center justify-between"><h2 class="text-sm font-semibold">当前规则</h2><Badge v-if="activeRule" variant="verified">v{{ activeRule.version }}</Badge></div><template v-if="activeRule"><dl class="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-sm"><dt class="text-muted-foreground">慢响应</dt><dd class="text-right tabular-nums">{{ duration(activeRule.slowTtftMs) }}</dd><dt class="text-muted-foreground">硬超时</dt><dd class="text-right tabular-nums">{{ duration(activeRule.hardTimeoutMs) }}</dd><dt class="text-muted-foreground">发布于</dt><dd class="text-right"><LocalTime :value="activeRule.publishedAt" /></dd></dl></template><p v-else class="mt-4 text-sm text-muted-foreground">当前仍为纯观测模式。</p></section>
      </div>

      <section class="overflow-hidden rounded-md border border-border bg-card"><div class="border-b border-border px-4 py-3"><h2 class="text-sm font-semibold">规则历史</h2></div><div class="overflow-x-auto"><table class="c2c-table w-full min-w-[760px] text-sm"><thead><tr class="text-left text-xs text-muted-foreground"><th class="px-3 py-2">模型</th><th class="px-3 py-2">协议 / 环境</th><th class="px-3 py-2">版本</th><th class="px-3 py-2">X / Y</th><th class="px-3 py-2">样本</th><th class="px-3 py-2">状态</th></tr></thead><tbody><tr v-for="rule in rules" :key="rule.id" class="border-t border-border"><td class="px-3 py-2 font-mono text-xs">{{ rule.model }}</td><td class="px-3 py-2 text-xs">{{ rule.protocol === 'openai_responses_v1' ? 'Responses' : 'Chat' }} · {{ rule.environmentLabel }}</td><td class="px-3 py-2">v{{ rule.version }}</td><td class="px-3 py-2 tabular-nums">{{ duration(rule.slowTtftMs) }} / {{ duration(rule.hardTimeoutMs) }}</td><td class="px-3 py-2 tabular-nums">{{ rule.sampleCount }}</td><td class="px-3 py-2"><Badge :variant="rule.status === 'active' ? 'verified' : 'outline'">{{ rule.status === 'active' ? '生效中' : '已停止' }}</Badge></td></tr><tr v-if="rules.length === 0"><td colspan="6" class="px-4 py-8 text-center text-muted-foreground">暂无已发布规则</td></tr></tbody></table></div></section>
    </template>
  </div>
</template>
