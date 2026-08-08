<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  Ban,
  CheckSquare2,
  FlaskConical,
  KeyRound,
  ListChecks,
  Play,
  RefreshCw,
  Search,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiModelTestResultCell from '@/components/api-model-tester/ApiModelTestResultCell.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useAPIModelTesterOrderSources,
  useDiscoverAPIModelsMutation,
  useTestAPIModelMutation,
} from '@/queries/useApiModelTesterQueries'
import type {
  ApiModelTesterCredentialSource,
  ApiModelTesterDiscovery,
  ApiModelTesterRowState,
} from '@/types/apiModelTester'

type SourceMode = 'manual' | 'order'

const route = useRoute()
const sourceMode = ref<SourceMode>('manual')
const manual = reactive({ baseUrl: '', apiKey: '' })
const selectedOrderId = ref('')
const acknowledgeInsecureHttp = ref(false)
const modelSearch = ref('')
const discovery = ref<ApiModelTesterDiscovery | null>(null)
const selectedModels = ref<string[]>([])
const rows = reactive(new Map<string, ApiModelTesterRowState>())
const running = ref(false)
const runCompleted = ref(0)
const runTotal = ref(0)
const runToken = ref(0)
const activeControllers = new Set<AbortController>()
let discoveryController: AbortController | null = null

const orderSourcesQuery = useAPIModelTesterOrderSources()
const discoverMutation = useDiscoverAPIModelsMutation()
const testMutation = useTestAPIModelMutation()
const orderSources = computed(() => orderSourcesQuery.data.value ?? [])
const selectedOrder = computed(() => orderSources.value.find(order => order.orderId === selectedOrderId.value) ?? null)
const orderSourcesError = computed(() => backendErrorMessage(orderSourcesQuery.error.value, '可导入订单暂时无法读取。'))
const sourceBaseUrl = computed(() => sourceMode.value === 'manual' ? manual.baseUrl.trim() : selectedOrder.value?.baseUrl.trim() ?? '')
const sourceUsesHTTP = computed(() => /^http:\/\//i.test(sourceBaseUrl.value))
const source = computed<ApiModelTesterCredentialSource | null>(() => {
  if (sourceMode.value === 'order') {
    return selectedOrderId.value
      ? { kind: 'order', orderId: selectedOrderId.value, acknowledgeInsecureHttp: acknowledgeInsecureHttp.value }
      : null
  }
  const baseUrl = manual.baseUrl.trim()
  const apiKey = manual.apiKey.trim()
  return baseUrl && apiKey
    ? { kind: 'manual', baseUrl, apiKey, acknowledgeInsecureHttp: acknowledgeInsecureHttp.value }
    : null
})
const sourceFingerprint = computed(() => sourceMode.value === 'manual'
  ? `manual:${manual.baseUrl.trim()}:${manual.apiKey.trim()}:${acknowledgeInsecureHttp.value}`
  : `order:${selectedOrderId.value}:${acknowledgeInsecureHttp.value}`)
const filteredModels = computed(() => {
  const query = modelSearch.value.trim().toLowerCase()
  const models = discovery.value?.models ?? []
  return query ? models.filter(model => model.toLowerCase().includes(query)) : models
})
const selectedCount = computed(() => selectedModels.value.length)
const allSelected = computed(() => Boolean(discovery.value?.models.length) && selectedModels.value.length === discovery.value?.models.length)
const progressPercent = computed(() => runTotal.value ? Math.round((runCompleted.value / runTotal.value) * 100) : 0)

function clearTransientResults() {
  discovery.value = null
  selectedModels.value = []
  modelSearch.value = ''
  rows.clear()
  runCompleted.value = 0
  runTotal.value = 0
}

function cancelRun() {
  runToken.value += 1
  for (const controller of activeControllers) controller.abort()
  activeControllers.clear()
  for (const [model, row] of rows) {
    if (row.state === 'pending') rows.set(model, { state: 'cancelled', message: '未完成的请求已停止等待。' })
  }
  running.value = false
}

function resetForSourceChange() {
  discoveryController?.abort()
  discoveryController = null
  cancelRun()
  clearTransientResults()
}

watch(sourceMode, (next, previous) => {
  if (next === previous) return
  manual.apiKey = ''
  selectedOrderId.value = ''
  acknowledgeInsecureHttp.value = false
  resetForSourceChange()
})

watch(
  () => sourceMode.value === 'manual' ? manual.baseUrl.trim() : selectedOrderId.value,
  (next, previous) => {
    if (next !== previous) acknowledgeInsecureHttp.value = false
  },
)

watch(sourceFingerprint, (next, previous) => {
  if (next === previous || (!discovery.value && rows.size === 0)) return
  resetForSourceChange()
})

watch(orderSources, sources => {
  const orderId = String(route.query.orderId ?? '')
  if (!orderId || !sources.some(order => order.orderId === orderId)) return
  sourceMode.value = 'order'
  void nextTick(() => {
    selectedOrderId.value = orderId
  })
}, { immediate: true })

onBeforeUnmount(() => {
  discoveryController?.abort()
  cancelRun()
  manual.apiKey = ''
  acknowledgeInsecureHttp.value = false
  clearTransientResults()
})

function toggleModel(model: string, checked: boolean) {
  const next = new Set(selectedModels.value)
  if (checked) next.add(model)
  else next.delete(model)
  selectedModels.value = [...next]
}

function toggleAll(checked: boolean) {
  selectedModels.value = checked ? [...(discovery.value?.models ?? [])] : []
}

async function discoverModels() {
  if (sourceUsesHTTP.value && !acknowledgeInsecureHttp.value) {
    toast.error('请先确认 HTTP 未加密传输风险。')
    return
  }
  if (!source.value || discoverMutation.isPending.value) {
    toast.error(sourceMode.value === 'manual' ? '请填写 Base URL 和 API Key。' : '请选择可导入订单。')
    return
  }
  resetForSourceChange()
  const controller = new AbortController()
  discoveryController = controller
  try {
    discovery.value = await discoverMutation.mutateAsync({ source: structuredClone(source.value), signal: controller.signal })
    if (!discovery.value.models.length) toast.info('当前 /models 未返回可测试模型。')
  } catch (error) {
    if ((error as { name?: string }).name !== 'AbortError') toast.error(backendErrorMessage(error, '模型列表获取失败。'))
  } finally {
    if (discoveryController === controller) discoveryController = null
  }
}

async function runModels(models: string[]) {
  const currentSource = source.value
  const uniqueModels = [...new Set(models)].filter(model => discovery.value?.models.includes(model))
  if (!currentSource || !uniqueModels.length) return

  cancelRun()
  const token = ++runToken.value
  const queue = [...uniqueModels]
  runCompleted.value = 0
  runTotal.value = queue.length
  running.value = true
  for (const model of uniqueModels) rows.set(model, { state: 'pending' })

  const worker = async () => {
    while (queue.length && token === runToken.value) {
      const model = queue.shift()
      if (!model) return
      const controller = new AbortController()
      activeControllers.add(controller)
      try {
        const result = await testMutation.mutateAsync({
          source: structuredClone(currentSource),
          model,
          signal: controller.signal,
        })
        if (token === runToken.value) rows.set(model, { state: 'completed', result })
      } catch (error) {
        if (token !== runToken.value || (error as { name?: string }).name === 'AbortError') {
          rows.set(model, { state: 'cancelled', message: '请求已取消。' })
        } else {
          rows.set(model, { state: 'failed', message: backendErrorMessage(error, '模型测试请求失败。') })
        }
      } finally {
        activeControllers.delete(controller)
        if (token === runToken.value) runCompleted.value += 1
      }
    }
  }

  await Promise.all(Array.from({ length: Math.min(3, uniqueModels.length) }, worker))
  if (token === runToken.value) running.value = false
}

function testSelected() {
  void runModels(selectedModels.value)
}

function testAll() {
  void runModels(discovery.value?.models ?? [])
}

function testOne(model: string) {
  void runModels([model])
}
</script>

<template>
  <div class="mx-auto w-full max-w-[1440px] space-y-5">
    <PageTitle title="API 模型测试" description="使用自己的凭据发现模型，并分别测试 Responses 与 Chat Completions。" />

    <section class="space-y-4 border-y border-border bg-card px-4 py-5 sm:px-5">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="space-y-3">
          <Tabs v-model="sourceMode">
            <TabsList>
              <TabsTrigger value="manual"><KeyRound class="h-4 w-4" />手动填写</TabsTrigger>
              <TabsTrigger value="order"><ListChecks class="h-4 w-4" />从订单导入</TabsTrigger>
            </TabsList>
          </Tabs>
          <p class="text-xs text-muted-foreground">凭据只用于本次请求；页面刷新或离开后不会保留测试结果。</p>
        </div>
        <Button :disabled="!source || (sourceUsesHTTP && !acknowledgeInsecureHttp) || discoverMutation.isPending.value || running" @click="discoverModels">
          <RefreshCw :class="['h-4 w-4', discoverMutation.isPending.value && 'animate-spin']" />
          {{ discovery ? '重新获取模型' : '获取模型' }}
        </Button>
      </div>

      <div v-if="sourceMode === 'manual'" class="grid gap-3 lg:grid-cols-2">
        <Label class="flex-col items-stretch gap-2">
          <span class="block text-sm font-medium">Base URL</span>
          <Input v-model="manual.baseUrl" inputmode="url" placeholder="https://api.example.com/v1" />
        </Label>
        <Label class="flex-col items-stretch gap-2">
          <span class="block text-sm font-medium">Bearer API Key</span>
          <Input v-model="manual.apiKey" type="password" autocomplete="new-password" placeholder="sk-..." />
        </Label>
      </div>

      <div v-else class="max-w-3xl space-y-3">
        <ErrorState v-if="orderSourcesQuery.error.value" title="无法读取可导入订单" :description="orderSourcesError" @retry="orderSourcesQuery.refetch()" />
        <div v-else class="space-y-2">
          <Label for="api-model-tester-order">API 订单</Label>
          <Select v-model="selectedOrderId" :disabled="orderSourcesQuery.isLoading.value">
            <SelectTrigger id="api-model-tester-order" class="w-full">
              <SelectValue :placeholder="orderSourcesQuery.isLoading.value ? '正在读取订单...' : '选择已交付且凭据仍可用的订单'" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="order in orderSources" :key="order.orderId" :value="order.orderId">
                {{ order.orderNo }} · {{ order.serviceTitle }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="selectedOrder" class="break-all text-xs text-muted-foreground">{{ selectedOrder.baseUrl }} · 交付于 <LocalTime :value="selectedOrder.deliveredAt" /></p>
          <Alert v-if="!orderSourcesQuery.isLoading.value && orderSources.length === 0">
            <AlertTitle>没有可导入订单</AlertTitle>
            <AlertDescription>仍可切换到手动填写，使用自己创建的 API Key。</AlertDescription>
          </Alert>
        </div>
      </div>

      <div v-if="sourceUsesHTTP" class="border-l-2 border-amber-500 bg-amber-50 px-3 py-2">
        <Label class="flex items-start gap-2 text-sm leading-6 text-amber-950">
          <Checkbox v-model="acknowledgeInsecureHttp" class="mt-1" />
          <span><span class="font-medium">确认使用未加密 HTTP</span><span class="block text-xs text-amber-900/75">Bearer API Key 会通过未加密连接发送，可能在传输途中被读取或篡改。</span></span>
        </Label>
      </div>
    </section>

    <Alert>
      <FlaskConical class="h-4 w-4" />
      <AlertTitle>结果表示当次实际调用情况</AlertTitle>
      <AlertDescription>列表来自当前凭据的 /models；通过不代表模型官方来源或平台担保。</AlertDescription>
    </Alert>

    <section v-if="discovery" class="space-y-3">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-base font-semibold">发现 {{ discovery.models.length }} 个模型</h2>
          <p class="mt-1 break-all text-xs text-muted-foreground">{{ discovery.baseUrl }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" :disabled="running || selectedCount === 0" @click="testSelected"><CheckSquare2 class="h-4 w-4" />测试选中（{{ selectedCount * 2 }} 次调用）</Button>
          <Button :disabled="running || discovery.models.length === 0" @click="testAll"><FlaskConical class="h-4 w-4" />测试全部（{{ discovery.models.length * 2 }} 次调用）</Button>
          <Button v-if="running" variant="destructive" @click="cancelRun"><Ban class="h-4 w-4" />取消</Button>
        </div>
      </div>

      <div v-if="running || runTotal" class="space-y-1.5" aria-live="polite">
        <div class="flex items-center justify-between text-xs text-muted-foreground"><span>进度 {{ runCompleted }} / {{ runTotal }}</span><span>{{ progressPercent }}%</span></div>
        <div class="h-2 overflow-hidden rounded-full bg-muted"><div class="h-full bg-primary transition-[width]" :style="{ width: `${progressPercent}%` }" /></div>
      </div>

      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <Label class="flex items-center gap-2 text-sm"><Checkbox :model-value="allSelected" @update:model-value="value => toggleAll(Boolean(value))" />全选模型</Label>
        <div class="relative w-full sm:max-w-xs"><Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input v-model="modelSearch" class="pl-9" placeholder="筛选模型 ID" /></div>
      </div>

      <div class="overflow-x-auto rounded-md border border-border bg-card">
        <table class="c2c-table w-full min-w-[780px] table-fixed text-sm">
          <colgroup><col class="w-[42%]"><col class="w-[22%]"><col class="w-[22%]"><col class="w-[14%]"></colgroup>
          <thead><tr class="border-b border-border text-left text-xs text-muted-foreground"><th class="px-3 py-2 font-medium">模型</th><th class="px-3 py-2 font-medium">Responses</th><th class="px-3 py-2 font-medium">Chat Completions</th><th class="px-3 py-2 text-right font-medium">操作</th></tr></thead>
          <tbody>
            <tr v-for="model in filteredModels" :key="model" class="border-b border-border/70 last:border-0">
              <td class="px-3 py-3"><Label class="flex min-w-0 items-center gap-2"><Checkbox :model-value="selectedModels.includes(model)" @update:model-value="value => toggleModel(model, Boolean(value))" /><span class="break-all font-mono text-xs">{{ model }}</span></Label></td>
              <td class="px-3 py-3"><ApiModelTestResultCell :row="rows.get(model)" :result="rows.get(model)?.result?.responsesResult" /></td>
              <td class="px-3 py-3"><ApiModelTestResultCell :row="rows.get(model)" :result="rows.get(model)?.result?.chatCompletionsResult" /></td>
              <td class="px-3 py-3 text-right"><Button size="sm" variant="outline" :disabled="running" @click="testOne(model)"><Play class="h-4 w-4" />测试</Button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="filteredModels.length === 0" class="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">没有匹配的模型 ID。</div>
    </section>

    <section v-else class="rounded-md border border-dashed border-border bg-card p-8 text-center">
      <FlaskConical class="mx-auto h-6 w-6 text-muted-foreground" />
      <h2 class="mt-3 text-sm font-semibold">先获取模型列表</h2>
      <p class="mt-1 text-xs text-muted-foreground">获取成功后可单独测试，也可一次测试全部返回模型。</p>
    </section>
  </div>
</template>
