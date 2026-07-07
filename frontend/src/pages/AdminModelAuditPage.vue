<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { AlertTriangle, ClipboardCopy, Download, FileText, Pencil, Play, Plus, RefreshCw, Save, ShieldAlert, TimerReset } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PageTitle from '@/components/market/PageTitle.vue'
import StatCard from '@/components/market/StatCard.vue'
import { Badge, type BadgeVariants } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  useCreateModelAuditBaseline,
  useCreateModelAuditMonitor,
  useCreateModelAuditRun,
  useCreateModelAuditTarget,
  useDeleteModelAuditTarget,
  useModelAuditBaselines,
  useModelAuditMonitors,
  useModelAuditReport,
  useModelAuditRuns,
  useModelAuditTargets,
  useUpdateModelAuditTarget,
} from '@/queries/useModelAuditQueries'
import type {
  ModelAuditBaseline,
  ModelAuditBaselineInput,
  ModelAuditMode,
  ModelAuditMonitorInput,
  ModelAuditProbeRiskLevel,
  ModelAuditRiskLevel,
  ModelAuditRun,
  ModelAuditRunInput,
  ModelAuditRunStatus,
  ModelAuditTarget,
  ModelAuditTargetInput,
} from '@/types/modelAudit'

type BadgeVariant = NonNullable<BadgeVariants['variant']>

const modeOptions: Array<{ value: ModelAuditMode, label: string }> = [
  { value: 'quick', label: 'Quick' },
  { value: 'standard', label: 'Standard' },
  { value: 'strict', label: 'Strict' },
  { value: 'scheduled', label: 'Scheduled' },
]

const sourceTypeOptions = [
  { value: 'official_api', label: 'Official API' },
  { value: 'trusted_api', label: 'Trusted API' },
  { value: 'local_model', label: 'Local Model' },
  { value: 'manual_import', label: 'Manual Import' },
]
const noSelectionValue = '__none__'

const riskLabels: Record<ModelAuditRiskLevel, string> = {
  consistent: '一致',
  suspicious: '可疑',
  high_risk: '高风险',
  insufficient_data: '数据不足',
}

const probeRiskLabels: Record<ModelAuditProbeRiskLevel, string> = {
  ...riskLabels,
  not_applicable: '不适用',
}

const statusLabels: Record<ModelAuditRunStatus, string> = {
  queued: '排队',
  running: '运行中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const targetsQuery = useModelAuditTargets()
const baselinesQuery = useModelAuditBaselines()
const runsQuery = useModelAuditRuns()
const monitorsQuery = useModelAuditMonitors()
const createTargetMutation = useCreateModelAuditTarget()
const updateTargetMutation = useUpdateModelAuditTarget()
const deleteTargetMutation = useDeleteModelAuditTarget()
const createBaselineMutation = useCreateModelAuditBaseline()
const createRunMutation = useCreateModelAuditRun()
const createMonitorMutation = useCreateModelAuditMonitor()

const selectedRunId = ref('')
const reportQuery = useModelAuditReport(selectedRunId)
const targetDialogOpen = ref(false)
const baselineDialogOpen = ref(false)
const editingTargetId = ref('')
const baselineParamsText = ref('{"temperature":0.7}')
const baselineFeaturesText = ref('{"random_distance_reference":0}')

const targetForm = reactive<ModelAuditTargetInput>(emptyTargetForm())
const baselineForm = reactive<ModelAuditBaselineInput>(emptyBaselineForm())
const runForm = reactive<ModelAuditRunInput>(emptyRunForm())
const monitorForm = reactive<ModelAuditMonitorInput>(emptyMonitorForm())

const targets = computed(() => targetsQuery.data.value ?? [])
const baselines = computed(() => baselinesQuery.data.value ?? [])
const runs = computed(() => runsQuery.data.value ?? [])
const monitors = computed(() => monitorsQuery.data.value ?? [])
const selectedRun = computed(() => runs.value.find(item => item.id === selectedRunId.value) ?? null)
const report = computed(() => reportQuery.data.value ?? null)
const activeTargets = computed(() => targets.value.filter(item => item.enabled))
const completedRuns = computed(() => runs.value.filter(item => item.status === 'completed'))
const highRiskCount = computed(() => runs.value.filter(item => item.riskLevel === 'high_risk').length)
const enabledMonitors = computed(() => monitors.value.filter(item => item.enabled).length)
const isLoading = computed(() => targetsQuery.isLoading.value || baselinesQuery.isLoading.value || runsQuery.isLoading.value || monitorsQuery.isLoading.value)
const hasError = computed(() => targetsQuery.isError.value || baselinesQuery.isError.value || runsQuery.isError.value || monitorsQuery.isError.value)
const errorMessage = computed(() => {
  const error = targetsQuery.error.value ?? baselinesQuery.error.value ?? runsQuery.error.value ?? monitorsQuery.error.value
  return error instanceof Error ? error.message : '模型审计数据读取失败。'
})
const savingTarget = computed(() => createTargetMutation.isPending.value || updateTargetMutation.isPending.value)
const savingBaseline = computed(() => createBaselineMutation.isPending.value)
const runningAudit = computed(() => createRunMutation.isPending.value)
const savingMonitor = computed(() => createMonitorMutation.isPending.value)

watch(activeTargets, rows => {
  if (!runForm.targetId && rows[0]) runForm.targetId = rows[0].id
  if (!monitorForm.targetId && rows[0]) monitorForm.targetId = rows[0].id
}, { immediate: true })

watch(baselines, rows => {
  if (!runForm.baselineId && rows[0]) runForm.baselineId = rows[0].id
  if (!monitorForm.baselineId && rows[0]) monitorForm.baselineId = rows[0].id
}, { immediate: true })

watch(runs, rows => {
  if (!selectedRunId.value && rows[0]) selectedRunId.value = rows[0].id
}, { immediate: true })

function emptyTargetForm(): ModelAuditTargetInput {
  return {
    name: '',
    baseUrl: '',
    providerType: 'openai_compatible',
    claimedModel: '',
    apiKey: '',
    enabled: true,
  }
}

function emptyBaselineForm(): ModelAuditBaselineInput {
  return {
    baselineName: '',
    sourceTargetId: noSelectionValue,
    model: '',
    sourceType: 'official_api',
    probeSetVersion: '2026-07-v1',
    paramsJson: { temperature: 0.7 },
    featureJson: { random_distance_reference: 0 },
    sampleCount: 0,
  }
}

function emptyRunForm(): ModelAuditRunInput {
  return {
    targetId: '',
    baselineId: noSelectionValue,
    claimedModel: '',
    mode: 'quick',
    enableModelEquality: false,
    enableLogprobs: 'auto',
    storePromptText: false,
    storeResponseText: false,
  }
}

function emptyMonitorForm(): ModelAuditMonitorInput {
  return {
    targetId: '',
    baselineId: noSelectionValue,
    mode: 'scheduled',
    enabled: true,
    cronSpec: '0 */6 * * *',
  }
}

function openCreateTarget() {
  editingTargetId.value = ''
  Object.assign(targetForm, emptyTargetForm())
  targetDialogOpen.value = true
}

function openEditTarget(target: ModelAuditTarget) {
  editingTargetId.value = target.id
  Object.assign(targetForm, {
    name: target.name,
    baseUrl: target.baseUrl,
    providerType: target.providerType,
    claimedModel: target.claimedModel,
    apiKey: '',
    enabled: target.enabled,
    apiServiceId: target.apiServiceId,
    apiServiceModelId: target.apiServiceModelId,
  })
  targetDialogOpen.value = true
}

function openBaselineDialog() {
  Object.assign(baselineForm, emptyBaselineForm())
  baselineParamsText.value = JSON.stringify(baselineForm.paramsJson, null, 2)
  baselineFeaturesText.value = JSON.stringify(baselineForm.featureJson, null, 2)
  baselineDialogOpen.value = true
}

async function saveTarget() {
  const validationError = validateTarget()
  if (validationError) {
    toast.warning(validationError)
    return
  }
  const input = { ...targetForm }
  try {
    if (editingTargetId.value) {
      await updateTargetMutation.mutateAsync({ id: editingTargetId.value, input })
      toast.success('审计目标已更新。')
    } else {
      await createTargetMutation.mutateAsync(input)
      toast.success('审计目标已创建。')
    }
    targetDialogOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '审计目标保存失败')
  }
}

async function toggleTarget(target: ModelAuditTarget) {
  try {
    await updateTargetMutation.mutateAsync({
      id: target.id,
      input: {
        name: target.name,
        baseUrl: target.baseUrl,
        providerType: target.providerType,
        claimedModel: target.claimedModel,
        apiKey: '',
        enabled: !target.enabled,
        apiServiceId: target.apiServiceId,
        apiServiceModelId: target.apiServiceModelId,
      },
    })
    toast.success(target.enabled ? '审计目标已停用。' : '审计目标已启用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '审计目标状态更新失败')
  }
}

async function disableTarget(target: ModelAuditTarget) {
  try {
    await deleteTargetMutation.mutateAsync(target.id)
    toast.success('审计目标已停用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '审计目标停用失败')
  }
}

async function saveBaseline() {
  const parsedParams = parseJSONRecord(baselineParamsText.value, '参数 JSON')
  if (!parsedParams.ok) return
  const parsedFeatures = parseJSONRecord(baselineFeaturesText.value, '特征 JSON')
  if (!parsedFeatures.ok) return
  if (!baselineForm.baselineName.trim()) {
    toast.warning('请填写基线名称。')
    return
  }
  if (!baselineForm.model.trim()) {
    toast.warning('请填写基线模型。')
    return
  }
  if (!baselineForm.probeSetVersion.trim()) {
    toast.warning('请填写探针版本。')
    return
  }
  try {
    await createBaselineMutation.mutateAsync({
      ...baselineForm,
      paramsJson: parsedParams.value,
      featureJson: parsedFeatures.value,
      sourceTargetId: baselineForm.sourceTargetId === noSelectionValue ? undefined : baselineForm.sourceTargetId,
    })
    toast.success('审计基线已创建。')
    baselineDialogOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '审计基线创建失败')
  }
}

async function createRun() {
  if (!runForm.targetId) {
    toast.warning('请选择审计目标。')
    return
  }
  try {
    const run = await createRunMutation.mutateAsync({
      ...runForm,
      baselineId: runForm.baselineId === noSelectionValue ? undefined : runForm.baselineId,
      claimedModel: (runForm.claimedModel ?? '').trim() || undefined,
    })
    selectedRunId.value = run.id
    toast.success('审计运行已完成。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '审计运行失败')
  }
}

async function createMonitor() {
  if (!monitorForm.targetId) {
    toast.warning('请选择巡检目标。')
    return
  }
  try {
    await createMonitorMutation.mutateAsync({
      ...monitorForm,
      baselineId: monitorForm.baselineId === noSelectionValue ? undefined : monitorForm.baselineId,
      cronSpec: monitorForm.cronSpec?.trim() || undefined,
    })
    toast.success('巡检配置已创建。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '巡检配置创建失败')
  }
}

async function copyReportMarkdown() {
  if (!report.value?.markdown) return
  await navigator.clipboard.writeText(report.value.markdown)
  toast.success('报告 Markdown 已复制。')
}

function downloadReportMarkdown() {
  if (!report.value?.markdown) return
  const blob = new Blob([report.value.markdown], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `model-audit-${report.value.runId}.md`
  link.click()
  URL.revokeObjectURL(url)
}

function selectRun(run: ModelAuditRun) {
  selectedRunId.value = run.id
}

function validateTarget() {
  if (!targetForm.name.trim()) return '请填写审计目标名称。'
  if (!targetForm.baseUrl.trim()) return '请填写 API Base URL。'
  if (!targetForm.claimedModel.trim()) return '请填写声称模型。'
  if (!editingTargetId.value && !targetForm.apiKey.trim()) return '新建目标必须填写 API Key。'
  return ''
}

function parseJSONRecord(text: string, label: string): { ok: true, value: Record<string, unknown> } | { ok: false } {
  try {
    const parsed = JSON.parse(text || '{}')
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      toast.warning(`${label} 必须是对象。`)
      return { ok: false }
    }
    return { ok: true, value: parsed as Record<string, unknown> }
  } catch {
    toast.warning(`${label} 格式不合法。`)
    return { ok: false }
  }
}

function riskLabel(risk?: ModelAuditRiskLevel) {
  return risk ? riskLabels[risk] : '未评估'
}

function probeRiskLabel(risk: ModelAuditProbeRiskLevel) {
  return probeRiskLabels[risk] ?? risk
}

function riskVariant(risk?: ModelAuditProbeRiskLevel): BadgeVariant {
  if (risk === 'consistent') return 'verified'
  if (risk === 'high_risk') return 'destructive'
  if (risk === 'suspicious') return 'secondary'
  if (risk === 'not_applicable') return 'outline'
  return 'status'
}

function statusVariant(status: ModelAuditRunStatus): BadgeVariant {
  if (status === 'completed') return 'verified'
  if (status === 'failed') return 'destructive'
  if (status === 'running') return 'secondary'
  return 'outline'
}

function modeLabel(mode: ModelAuditMode) {
  return modeOptions.find(item => item.value === mode)?.label ?? mode
}

function targetName(targetId: string) {
  return targets.value.find(item => item.id === targetId)?.name ?? targetId
}

function baselineName(baseline?: ModelAuditBaseline | string) {
  const baselineId = typeof baseline === 'string' ? baseline : baseline?.id
  if (!baselineId || baselineId === noSelectionValue) return '-'
  return baselines.value.find(item => item.id === baselineId)?.baselineName ?? baselineId
}

function formatPercent(value: number) {
  return `${Math.round(value * 100)}%`
}

function formatScore(value: number) {
  return value.toFixed(2)
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function evidenceText(evidence: Record<string, unknown>) {
  const text = JSON.stringify(evidence)
  return text.length > 96 ? `${text.slice(0, 96)}...` : text
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle
      title="模型一致性审计"
      description="对第三方 OpenAI-compatible API 渠道生成统计风险信号，不承载平台代理、凭据交付或自动处罚。"
    />

    <div class="grid gap-3 md:grid-cols-4">
      <StatCard label="审计目标" :value="targets.length" :hint="`启用 ${activeTargets.length}`" />
      <StatCard label="完成运行" :value="completedRuns.length" :hint="`高风险 ${highRiskCount}`" accent />
      <StatCard label="可信基线" :value="baselines.length" hint="按探针版本管理" />
      <StatCard label="巡检配置" :value="enabledMonitors" :hint="`全部 ${monitors.length}`" />
    </div>

    <Card v-if="hasError" class="border-destructive/30 p-5">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-destructive/10 text-destructive">
          <AlertTriangle class="h-5 w-5" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="font-semibold">模型审计数据读取失败</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ errorMessage }}</p>
          <Button class="mt-4" size="sm" variant="outline" @click="targetsQuery.refetch(); baselinesQuery.refetch(); runsQuery.refetch(); monitorsQuery.refetch()">
            <RefreshCw class="h-4 w-4" />重新读取
          </Button>
        </div>
      </div>
    </Card>

    <section class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold">审计目标</h2>
          <p class="text-sm text-muted-foreground">API Key 仅在创建或更新时提交，列表和报告不会返回明文。</p>
        </div>
        <Button size="sm" @click="openCreateTarget">
          <Plus class="h-4 w-4" />新建目标
        </Button>
      </div>

      <div v-if="isLoading" class="rounded-md border border-border p-8 text-center text-sm text-muted-foreground">
        模型审计数据加载中...
      </div>

      <Card v-else class="overflow-hidden p-0">
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[940px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">目标</th>
                <th class="px-3 py-2 font-medium">Base URL</th>
                <th class="px-3 py-2 font-medium">声称模型</th>
                <th class="px-3 py-2 font-medium">最近风险</th>
                <th class="px-3 py-2 font-medium">状态</th>
                <th class="px-3 py-2 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="target in targets" :key="target.id" class="border-b border-border/70 last:border-0">
                <td class="px-3 py-3">
                  <div class="font-medium">{{ target.name }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">{{ target.providerType }}</div>
                </td>
                <td class="max-w-[280px] px-3 py-3 text-xs text-muted-foreground">
                  <div class="truncate">{{ target.baseUrl }}</div>
                </td>
                <td class="px-3 py-3"><Badge variant="model">{{ target.claimedModel }}</Badge></td>
                <td class="px-3 py-3"><Badge :variant="riskVariant(target.lastRiskLevel)">{{ riskLabel(target.lastRiskLevel) }}</Badge></td>
                <td class="px-3 py-3"><Badge :variant="target.enabled ? 'verified' : 'secondary'">{{ target.enabled ? '启用' : '停用' }}</Badge></td>
                <td class="px-3 py-3">
                  <div class="flex flex-wrap justify-end gap-1.5">
                    <Button size="sm" variant="outline" @click="openEditTarget(target)">
                      <Pencil class="h-4 w-4" />编辑
                    </Button>
                    <Button size="sm" variant="outline" :disabled="updateTargetMutation.isPending.value" @click="toggleTarget(target)">
                      {{ target.enabled ? '停用' : '启用' }}
                    </Button>
                    <Button size="sm" variant="outline" :disabled="deleteTargetMutation.isPending.value || !target.enabled" @click="disableTarget(target)">
                      <ShieldAlert class="h-4 w-4" />停用
                    </Button>
                  </div>
                </td>
              </tr>
              <tr v-if="targets.length === 0">
                <td colspan="6" class="px-3 py-10 text-center text-sm text-muted-foreground">暂无审计目标。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>

    <section class="grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
      <Card class="p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">运行审计</h2>
            <p class="mt-1 text-sm text-muted-foreground">默认只保存系统生成样本的哈希和结构化证据。</p>
          </div>
          <Button size="sm" variant="outline" @click="openBaselineDialog">
            <FileText class="h-4 w-4" />新建基线
          </Button>
        </div>

        <div class="mt-5 grid gap-3 sm:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium">目标</span>
            <Select v-model="runForm.targetId">
              <SelectTrigger><SelectValue placeholder="选择目标" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="target in activeTargets" :key="target.id" :value="target.id">{{ target.name }} · {{ target.claimedModel }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">基线</span>
            <Select v-model="runForm.baselineId">
              <SelectTrigger><SelectValue placeholder="选择基线" /></SelectTrigger>
              <SelectContent>
                <SelectItem :value="noSelectionValue">不绑定基线</SelectItem>
                <SelectItem v-for="baseline in baselines" :key="baseline.id" :value="baseline.id">{{ baseline.baselineName }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">模式</span>
            <Select v-model="runForm.mode">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="mode in modeOptions" :key="mode.value" :value="mode.value">{{ mode.label }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">覆盖模型</span>
            <Input v-model="runForm.claimedModel" placeholder="留空使用目标声称模型" />
          </label>
        </div>

        <div class="mt-4 grid gap-2 sm:grid-cols-2">
          <label class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
            <input v-model="runForm.enableModelEquality" type="checkbox" class="h-4 w-4 accent-primary" />
            <span>启用等价性检验</span>
          </label>
          <label class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
            <input v-model="runForm.storePromptText" type="checkbox" class="h-4 w-4 accent-primary" />
            <span>保存 canary prompt 文本</span>
          </label>
          <label class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
            <input v-model="runForm.storeResponseText" type="checkbox" class="h-4 w-4 accent-primary" />
            <span>保存响应文本</span>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">Logprobs</span>
            <Select v-model="runForm.enableLogprobs">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="auto">Auto</SelectItem>
                <SelectItem value="enabled">Enabled</SelectItem>
                <SelectItem value="disabled">Disabled</SelectItem>
              </SelectContent>
            </Select>
          </label>
        </div>

        <div class="mt-5 flex flex-wrap justify-end gap-2">
          <Button :disabled="runningAudit || activeTargets.length === 0" @click="createRun">
            <Play class="h-4 w-4" />启动审计
          </Button>
        </div>
      </Card>

      <Card class="p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">审计报告</h2>
            <p class="mt-1 text-sm text-muted-foreground">结论以风险信号表达，需结合人工复核和连续监控。</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" :disabled="!report?.markdown" @click="copyReportMarkdown">
              <ClipboardCopy class="h-4 w-4" />复制
            </Button>
            <Button size="sm" variant="outline" :disabled="!report?.markdown" @click="downloadReportMarkdown">
              <Download class="h-4 w-4" />下载
            </Button>
          </div>
        </div>

        <div class="mt-4 space-y-4">
          <Select v-model="selectedRunId">
            <SelectTrigger><SelectValue placeholder="选择审计运行" /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="run in runs" :key="run.id" :value="run.id">
                {{ run.targetName }} · {{ modeLabel(run.mode) }} · {{ formatDate(run.createdAt) }}
              </SelectItem>
            </SelectContent>
          </Select>

          <div v-if="reportQuery.isFetching.value" class="rounded-md border border-border p-6 text-center text-sm text-muted-foreground">
            报告读取中...
          </div>
          <div v-else-if="report" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-3">
              <div class="rounded-md border border-border bg-muted/20 p-3">
                <div class="text-xs text-muted-foreground">风险等级</div>
                <Badge class="mt-2" :variant="riskVariant(report.riskLevel)">{{ riskLabel(report.riskLevel) }}</Badge>
              </div>
              <div class="rounded-md border border-border bg-muted/20 p-3">
                <div class="text-xs text-muted-foreground">置信度</div>
                <div class="mt-2 text-lg font-semibold">{{ formatPercent(report.confidence) }}</div>
              </div>
              <div class="rounded-md border border-border bg-muted/20 p-3">
                <div class="text-xs text-muted-foreground">风险分</div>
                <div class="mt-2 text-lg font-semibold">{{ formatScore(report.overallRiskScore) }}</div>
              </div>
            </div>
            <p class="rounded-md border border-border bg-muted/20 p-3 text-sm leading-6 text-muted-foreground">{{ report.summary }}</p>
            <Textarea :model-value="report.markdown" readonly class="min-h-64 font-mono text-xs" />
          </div>
          <div v-else class="rounded-md border border-border p-6 text-center text-sm text-muted-foreground">
            暂无可查看报告。
          </div>
        </div>
      </Card>
    </section>

    <section class="grid gap-4 xl:grid-cols-2">
      <Card class="overflow-hidden p-0">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border p-4">
          <div>
            <h2 class="font-semibold">运行记录</h2>
            <p class="text-sm text-muted-foreground">最近运行、风险等级和报告入口。</p>
          </div>
          <Badge variant="secondary">{{ runs.length }}</Badge>
        </div>
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[820px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">目标</th>
                <th class="px-3 py-2 font-medium">模式</th>
                <th class="px-3 py-2 font-medium">风险</th>
                <th class="px-3 py-2 font-medium">状态</th>
                <th class="px-3 py-2 font-medium">时间</th>
                <th class="px-3 py-2 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in runs" :key="run.id" class="border-b border-border/70 last:border-0">
                <td class="px-3 py-3">
                  <div class="font-medium">{{ run.targetName || targetName(run.targetId) }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">{{ run.claimedModel }}</div>
                </td>
                <td class="px-3 py-3">{{ modeLabel(run.mode) }}</td>
                <td class="px-3 py-3"><Badge :variant="riskVariant(run.riskLevel)">{{ riskLabel(run.riskLevel) }}</Badge></td>
                <td class="px-3 py-3"><Badge :variant="statusVariant(run.status)">{{ statusLabels[run.status] }}</Badge></td>
                <td class="px-3 py-3 text-xs text-muted-foreground">{{ formatDate(run.createdAt) }}</td>
                <td class="px-3 py-3 text-right">
                  <Button size="sm" variant="outline" @click="selectRun(run)">
                    <FileText class="h-4 w-4" />报告
                  </Button>
                </td>
              </tr>
              <tr v-if="runs.length === 0">
                <td colspan="6" class="px-3 py-10 text-center text-sm text-muted-foreground">暂无运行记录。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>

      <Card class="overflow-hidden p-0">
        <div class="border-b border-border p-4">
          <h2 class="font-semibold">分项证据</h2>
          <p class="text-sm text-muted-foreground">{{ selectedRun ? `${selectedRun.targetName} · ${baselineName(selectedRun.baselineId)}` : '未选择运行' }}</p>
        </div>
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[780px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">探针</th>
                <th class="px-3 py-2 font-medium">风险</th>
                <th class="px-3 py-2 font-medium">分数</th>
                <th class="px-3 py-2 font-medium">置信度</th>
                <th class="px-3 py-2 font-medium">证据</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="score in (selectedRun?.probeScores ?? report?.probeScores ?? [])" :key="score.probe" class="border-b border-border/70 last:border-0">
                <td class="px-3 py-3 font-medium">{{ score.probe }}</td>
                <td class="px-3 py-3"><Badge :variant="riskVariant(score.risk)">{{ probeRiskLabel(score.risk) }}</Badge></td>
                <td class="px-3 py-3">{{ formatScore(score.score) }}</td>
                <td class="px-3 py-3">{{ formatPercent(score.confidence) }}</td>
                <td class="max-w-[280px] px-3 py-3 text-xs text-muted-foreground">
                  <div class="truncate">{{ evidenceText(score.evidence) }}</div>
                </td>
              </tr>
              <tr v-if="(selectedRun?.probeScores ?? report?.probeScores ?? []).length === 0">
                <td colspan="5" class="px-3 py-10 text-center text-sm text-muted-foreground">暂无分项证据。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>

    <section class="grid gap-4 xl:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
      <Card class="p-5">
        <div class="flex items-center gap-2">
          <TimerReset class="h-5 w-5 text-muted-foreground" />
          <h2 class="font-semibold">定时巡检</h2>
        </div>
        <div class="mt-4 grid gap-3">
          <label class="space-y-2">
            <span class="text-sm font-medium">目标</span>
            <Select v-model="monitorForm.targetId">
              <SelectTrigger><SelectValue placeholder="选择目标" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="target in activeTargets" :key="target.id" :value="target.id">{{ target.name }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">基线</span>
            <Select v-model="monitorForm.baselineId">
              <SelectTrigger><SelectValue placeholder="选择基线" /></SelectTrigger>
              <SelectContent>
                <SelectItem :value="noSelectionValue">不绑定基线</SelectItem>
                <SelectItem v-for="baseline in baselines" :key="baseline.id" :value="baseline.id">{{ baseline.baselineName }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">模式</span>
              <Select v-model="monitorForm.mode">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="mode in modeOptions" :key="mode.value" :value="mode.value">{{ mode.label }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">Cron</span>
              <Input v-model="monitorForm.cronSpec" placeholder="0 */6 * * *" />
            </label>
          </div>
          <label class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
            <input v-model="monitorForm.enabled" type="checkbox" class="h-4 w-4 accent-primary" />
            <span>启用巡检</span>
          </label>
          <Button :disabled="savingMonitor || activeTargets.length === 0" @click="createMonitor">
            <Save class="h-4 w-4" />保存巡检
          </Button>
        </div>
      </Card>

      <Card class="overflow-hidden p-0">
        <div class="border-b border-border p-4">
          <h2 class="font-semibold">巡检列表</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[720px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">目标</th>
                <th class="px-3 py-2 font-medium">模式</th>
                <th class="px-3 py-2 font-medium">Cron</th>
                <th class="px-3 py-2 font-medium">最近风险</th>
                <th class="px-3 py-2 font-medium">状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="monitor in monitors" :key="monitor.id" class="border-b border-border/70 last:border-0">
                <td class="px-3 py-3">{{ targetName(monitor.targetId) }}</td>
                <td class="px-3 py-3">{{ modeLabel(monitor.mode) }}</td>
                <td class="px-3 py-3 text-xs text-muted-foreground">{{ monitor.cronSpec || '-' }}</td>
                <td class="px-3 py-3"><Badge :variant="riskVariant(monitor.lastRisk)">{{ riskLabel(monitor.lastRisk) }}</Badge></td>
                <td class="px-3 py-3"><Badge :variant="monitor.enabled ? 'verified' : 'secondary'">{{ monitor.enabled ? '启用' : '停用' }}</Badge></td>
              </tr>
              <tr v-if="monitors.length === 0">
                <td colspan="5" class="px-3 py-10 text-center text-sm text-muted-foreground">暂无巡检配置。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>

    <Dialog v-model:open="targetDialogOpen">
      <DialogContent class="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ editingTargetId ? '编辑审计目标' : '新建审计目标' }}</DialogTitle>
          <DialogDescription>API Key 只用于向目标渠道发起审计请求，保存后不会在接口中返回。</DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">名称</span>
              <Input v-model="targetForm.name" placeholder="OpenAI Relay 样例" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">声称模型</span>
              <Input v-model="targetForm.claimedModel" placeholder="gpt-4.1" />
            </label>
          </div>
          <label class="space-y-2">
            <span class="text-sm font-medium">API Base URL</span>
            <Input v-model="targetForm.baseUrl" placeholder="https://api.example.com/v1" />
          </label>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">Provider Type</span>
              <Input v-model="targetForm.providerType" placeholder="openai_compatible" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">API Key</span>
              <Input v-model="targetForm.apiKey" type="password" :placeholder="editingTargetId ? '留空保持不变' : 'sk-...'" />
            </label>
          </div>
          <label class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
            <input v-model="targetForm.enabled" type="checkbox" class="h-4 w-4 accent-primary" />
            <span>启用目标</span>
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="savingTarget" @click="targetDialogOpen = false">取消</Button>
          <Button :disabled="savingTarget" @click="saveTarget">
            <Save class="h-4 w-4" />保存目标
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="baselineDialogOpen">
      <DialogContent class="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>新建可信基线</DialogTitle>
          <DialogDescription>基线保存探针参数和结构化特征，用于后续风险比较。</DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">基线名称</span>
              <Input v-model="baselineForm.baselineName" placeholder="gpt-4.1 official 2026-07" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">模型</span>
              <Input v-model="baselineForm.model" placeholder="gpt-4.1" />
            </label>
          </div>
          <div class="grid gap-3 sm:grid-cols-3">
            <label class="space-y-2">
              <span class="text-sm font-medium">来源</span>
              <Select v-model="baselineForm.sourceType">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in sourceTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">探针版本</span>
              <Input v-model="baselineForm.probeSetVersion" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">样本数</span>
              <Input v-model.number="baselineForm.sampleCount" type="number" min="0" />
            </label>
          </div>
          <label class="space-y-2">
            <span class="text-sm font-medium">来源目标</span>
            <Select v-model="baselineForm.sourceTargetId">
              <SelectTrigger><SelectValue placeholder="可选" /></SelectTrigger>
              <SelectContent>
                <SelectItem :value="noSelectionValue">不绑定目标</SelectItem>
                <SelectItem v-for="target in targets" :key="target.id" :value="target.id">{{ target.name }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">参数 JSON</span>
              <Textarea v-model="baselineParamsText" class="min-h-36 font-mono text-xs" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">特征 JSON</span>
              <Textarea v-model="baselineFeaturesText" class="min-h-36 font-mono text-xs" />
            </label>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="savingBaseline" @click="baselineDialogOpen = false">取消</Button>
          <Button :disabled="savingBaseline" @click="saveBaseline">
            <Save class="h-4 w-4" />保存基线
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
