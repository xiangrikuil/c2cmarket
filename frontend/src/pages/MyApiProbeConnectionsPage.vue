<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, FilePenLine, Link2, Plus, RefreshCw, ShieldAlert, Trash2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiHealth24HourStrip from '@/components/api-market/ApiHealth24HourStrip.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useCreateOwnerAPIProbeConnectionMutation,
  useDeleteOwnerAPIProbeConnectionMutation,
  useOwnerAPIProbeConnections,
  usePreflightOwnerAPIProbeConnectionMutation,
  useUpdateOwnerAPIProbeConnectionMutation,
  useVerifyOwnerAPIProbeConnectionMutation,
} from '@/queries/useApiHealthQueries'
import type { ApiProbeConnectionPreflight, OwnerAPIProbeConnection } from '@/types/apiHealth'

const route = useRoute()
const router = useRouter()
const query = useOwnerAPIProbeConnections()
const preflightMutation = usePreflightOwnerAPIProbeConnectionMutation()
const createMutation = useCreateOwnerAPIProbeConnectionMutation()
const updateMutation = useUpdateOwnerAPIProbeConnectionMutation()
const deleteMutation = useDeleteOwnerAPIProbeConnectionMutation()
const verifyMutation = useVerifyOwnerAPIProbeConnectionMutation()
const formOpen = ref(false)
const editing = ref<OwnerAPIProbeConnection | null>(null)
const independentConnectionConfirmed = ref(false)
const preflight = ref<ApiProbeConnectionPreflight | null>(null)
const preflightFingerprint = ref('')

const form = reactive({
  name: '',
  baseUrl: '',
  credential: '',
  probeModel: '',
  enabled: true,
  acknowledgeInsecureHttp: false,
})

const connections = computed(() => query.data.value ?? [])
const busy = computed(() => preflightMutation.isPending.value || createMutation.isPending.value || updateMutation.isPending.value)
const formUsesHTTP = computed(() => form.baseUrl.trim().toLowerCase().startsWith('http://'))
const errorMessage = computed(() => backendErrorMessage(query.error.value, '探针连接暂时无法读取。'))

function comparableBaseURL(value: string) {
  try {
    const url = new URL(value.trim())
    url.hostname = url.hostname.toLowerCase()
    if ((url.protocol === 'https:' && url.port === '443') || (url.protocol === 'http:' && url.port === '80')) url.port = ''
    url.pathname = url.pathname.replace(/\/+$/, '') || '/'
    return url.toString().replace(/\/$/, '')
  } catch {
    return ''
  }
}

function currentFingerprint() {
  return [comparableBaseURL(form.baseUrl), form.credential.trim() || `stored:${editing.value?.id ?? ''}`, form.probeModel].join('|')
}

const duplicateConnections = computed(() => {
  if (editing.value) return []
  const target = comparableBaseURL(form.baseUrl)
  if (!target) return []
  return connections.value.filter(connection => connection.normalizedBaseUrl === target || comparableBaseURL(connection.baseUrl) === target)
})

const preflightValid = computed(() => Boolean(
  preflight.value
  && !preflight.value.errorCode
  && preflight.value.preflightToken
  && preflight.value.probeModel === form.probeModel
  && preflightFingerprint.value === currentFingerprint(),
))

const requiresPreflight = computed(() => !editing.value || Boolean(
  comparableBaseURL(form.baseUrl) !== editing.value.normalizedBaseUrl
  || form.credential.trim()
  || form.probeModel !== editing.value.probeModel
  || (form.enabled && !editing.value.enabled),
))

const canPreflight = computed(() => Boolean(
  comparableBaseURL(form.baseUrl)
  && (editing.value?.credentialConfigured || form.credential.trim())
  && (!formUsesHTTP.value || form.acknowledgeInsecureHttp),
))

const canSubmit = computed(() => Boolean(
  form.name.trim()
  && canPreflight.value
  && (!requiresPreflight.value || preflightValid.value)
  && (!duplicateConnections.value.length || independentConnectionConfirmed.value),
))

function verificationStatus(connection: OwnerAPIProbeConnection) {
  if (connection.verificationStatus === 'verified') return { label: '已验证', tone: 'success' as const }
  if (connection.verificationStatus === 'failed') return { label: '验证失败', tone: 'risk' as const }
  return { label: '待验证', tone: 'waiting' as const }
}

function resetForm(connection?: OwnerAPIProbeConnection) {
  editing.value = connection ?? null
  form.name = connection?.name ?? ''
  form.baseUrl = connection?.baseUrl ?? ''
  form.credential = ''
  form.probeModel = connection?.probeModel ?? ''
  form.enabled = connection?.enabled ?? true
  form.acknowledgeInsecureHttp = connection?.baseUrl.toLowerCase().startsWith('http://') ?? false
  independentConnectionConfirmed.value = Boolean(connection)
  preflight.value = connection?.probeModel && connection.probeProtocol ? {
    errorCode: null,
    availableModels: connection.availableModels,
    probeModel: connection.probeModel,
    probeProtocol: connection.probeProtocol,
    probeEnvironment: connection.probeEnvironment,
    dailyBaseCostUpperBoundUsd: connection.dailyBaseCostUpperBoundUsd,
    priceUnavailable: connection.priceUnavailable,
    preflightToken: null,
  } : null
  preflightFingerprint.value = preflight.value ? currentFingerprint() : ''
}

function openCreate() { resetForm(); formOpen.value = true }
function openEdit(connection: OwnerAPIProbeConnection) { resetForm(connection); formOpen.value = true }

function reuseConnection(connection: OwnerAPIProbeConnection) {
  formOpen.value = false
  toast.info(`可以在发布服务时选择“${connection.name}”。`)
  void router.push('/api-market/new')
}

async function runPreflight() {
  if (!canPreflight.value || busy.value) return
  try {
    const result = await preflightMutation.mutateAsync({
      ...(editing.value ? { id: editing.value.id, version: editing.value.version } : {}),
      name: form.name,
      baseUrl: form.baseUrl,
      credential: form.credential || undefined,
      probeModel: form.probeModel,
      enabled: form.enabled,
      acknowledgeInsecureHttp: form.acknowledgeInsecureHttp,
    })
    preflight.value = result
    if (!form.probeModel) form.probeModel = result.probeModel ?? (result.availableModels.includes('gpt-5.6-luna') ? 'gpt-5.6-luna' : result.availableModels[0] ?? '')
    if (!result.errorCode && result.probeModel === form.probeModel) {
      preflightFingerprint.value = currentFingerprint()
      toast.success('真实模型验证通过。')
    } else if (result.errorCode === 'model_unavailable' && result.availableModels.length) {
      preflightFingerprint.value = ''
      toast.info('已读取模型，请选择一个模型后再次验证。')
    } else {
      preflightFingerprint.value = ''
      toast.error('当前连接未通过真实模型验证。')
    }
  } catch (error) {
    preflight.value = null
    preflightFingerprint.value = ''
    toast.error(backendErrorMessage(error, '无法读取并验证模型。'))
  }
}

async function submitForm() {
  if (!canSubmit.value || busy.value) return
  const payload = {
    name: form.name,
    baseUrl: form.baseUrl,
    credential: form.credential || undefined,
    probeModel: form.probeModel,
    enabled: form.enabled,
    acknowledgeInsecureHttp: form.acknowledgeInsecureHttp,
    ...(requiresPreflight.value && preflightValid.value && preflight.value?.preflightToken
      ? { preflightToken: preflight.value.preflightToken }
      : {}),
  }
  try {
    const connection = editing.value
      ? await updateMutation.mutateAsync({ ...payload, id: editing.value.id, version: editing.value.version })
      : await createMutation.mutateAsync(payload)
    formOpen.value = false
    toast.success(connection.enabled ? '探针连接已保存并开始周期探测。' : '探针连接已保存。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针连接保存失败。'))
  }
}

async function verify(connection: OwnerAPIProbeConnection) {
  try {
    const updated = await verifyMutation.mutateAsync({ id: connection.id, version: connection.version })
    toast.success(updated.verificationStatus === 'verified' ? '真实模型验证通过。' : '探针连接仍未通过验证。')
  } catch (error) { toast.error(backendErrorMessage(error, '探针连接验证失败。')) }
}

async function setEnabled(connection: OwnerAPIProbeConnection, enabled: boolean) {
  try {
    let preflightToken: string | undefined
    if (enabled) {
      const verification = await preflightMutation.mutateAsync({
        id: connection.id,
        version: connection.version,
        name: connection.name,
        baseUrl: connection.baseUrl,
        probeModel: connection.probeModel ?? undefined,
        enabled: true,
        acknowledgeInsecureHttp: connection.baseUrl.toLowerCase().startsWith('http://'),
      })
      if (verification.errorCode || !verification.preflightToken) {
        toast.error(`重新启用前验证失败：${verification.errorCode ?? '验证结果不可用'}`)
        return
      }
      preflightToken = verification.preflightToken
    }
    await updateMutation.mutateAsync({
      id: connection.id,
      version: connection.version,
      name: connection.name,
      baseUrl: connection.baseUrl,
      probeModel: connection.probeModel ?? undefined,
      preflightToken,
      enabled,
      acknowledgeInsecureHttp: connection.baseUrl.toLowerCase().startsWith('http://'),
    })
    toast.success(enabled ? '探针连接已启用。' : '探针连接已停用。')
  } catch (error) { toast.error(backendErrorMessage(error, enabled ? '探针连接启用失败。' : '探针连接停用失败。')) }
}

async function removeConnection(connection: OwnerAPIProbeConnection) {
  if (connection.referencedServices.length) {
    toast.error(`该连接仍被 ${connection.referencedServices.length} 项服务引用，请先完成改绑。`)
    return
  }
  if (!window.confirm(`确认删除探针连接“${connection.name}”？连接样本也会一并删除。`)) return
  try {
    await deleteMutation.mutateAsync({ id: connection.id, version: connection.version })
    toast.success('探针连接已删除。')
  } catch (error) { toast.error(backendErrorMessage(error, '探针连接删除失败。')) }
}

watch(() => [form.baseUrl, form.credential, form.probeModel], () => {
  if (preflightFingerprint.value && preflightFingerprint.value !== currentFingerprint()) preflightFingerprint.value = ''
})
watch(() => route.query.create, value => { if (value === '1') openCreate() }, { immediate: true })
</script>

<template>
  <div class="mx-auto w-full max-w-[1440px] space-y-5">
    <PageTitle title="探针连接" description="一条连接可供多项 API 服务复用，平台每五分钟执行一次真实模型请求。">
      <template #action><Button @click="openCreate"><Plus class="h-4 w-4" />新建连接</Button></template>
    </PageTitle>

    <div class="grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-3">
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">连接</div><div class="mt-1 text-xl font-semibold">{{ connections.length }}</div></div>
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">运行中</div><div class="mt-1 text-xl font-semibold">{{ connections.filter(item => item.enabled && item.verificationStatus === 'verified').length }}</div></div>
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">引用服务</div><div class="mt-1 text-xl font-semibold">{{ connections.reduce((sum, item) => sum + item.referencedServices.length, 0) }}</div></div>
    </div>

    <SkeletonTable v-if="query.isLoading.value" :rows="4" :columns="6" />
    <ErrorState v-else-if="query.error.value" title="无法读取探针连接" :description="errorMessage" @retry="query.refetch()" />
    <EmptyState v-else-if="connections.length === 0" title="还没有探针连接" description="创建一次连接后，可让多项 API 服务直接复用。">
      <template #action><Button @click="openCreate"><Plus class="h-4 w-4" />新建连接</Button></template>
    </EmptyState>

    <div v-else class="overflow-hidden rounded-md border border-border bg-card">
      <div class="hidden overflow-x-auto md:block">
        <table class="c2c-table w-full min-w-[1120px] text-sm">
          <thead><tr class="border-b border-border text-left text-xs text-muted-foreground"><th class="px-3 py-2 font-medium">连接</th><th class="px-3 py-2 font-medium">探针模型</th><th class="w-[290px] px-3 py-2 font-medium">近 24 小时</th><th class="px-3 py-2 font-medium">运行</th><th class="px-3 py-2 font-medium">引用服务</th><th class="px-3 py-2 text-right font-medium">操作</th></tr></thead>
          <tbody>
            <tr v-for="connection in connections" :key="connection.id" class="border-b border-border/70 last:border-0">
              <td class="max-w-[260px] px-3 py-3"><div class="flex items-center gap-2"><span class="font-medium">{{ connection.name }}</span><StatusBadge :status="connection.verificationStatus" :label="verificationStatus(connection).label" :tone="verificationStatus(connection).tone" /></div><div class="mt-1 truncate font-mono text-xs text-muted-foreground" :title="connection.baseUrl">{{ connection.baseUrl }}</div></td>
              <td class="px-3 py-3"><div class="font-mono text-xs font-medium">{{ connection.probeModel ?? '未配置' }}</div><div v-if="connection.probeProtocol === 'openai_chat_completions_v1'" class="mt-1 text-[10px] text-amber-700">Chat 回退</div></td>
              <td class="px-3 py-3"><ApiHealth24HourStrip :summary="connection.healthSummary" compact /></td>
              <td class="px-3 py-3"><div class="flex items-center gap-2"><Switch :model-value="connection.enabled" :disabled="busy" @update:model-value="value => setEnabled(connection, Boolean(value))" /><span class="text-xs" :class="connection.enabled ? 'text-emerald-700' : 'text-muted-foreground'">{{ connection.enabled ? '运行中' : '已停用' }}</span></div></td>
              <td class="px-3 py-3"><div class="flex flex-wrap gap-1"><Badge v-if="connection.referencedServices.length === 0" variant="outline">未引用</Badge><template v-else><Badge v-for="service in connection.referencedServices.slice(0, 2)" :key="service.id" variant="secondary">{{ service.title }}</Badge><Badge v-if="connection.referencedServices.length > 2" variant="outline">+{{ connection.referencedServices.length - 2 }}</Badge></template></div></td>
              <td class="px-3 py-3"><div class="flex justify-end gap-1"><Button size="icon" variant="ghost" title="重新验证" :disabled="verifyMutation.isPending.value" @click="verify(connection)"><RefreshCw class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="编辑" @click="openEdit(connection)"><FilePenLine class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="删除" :disabled="deleteMutation.isPending.value || connection.referencedServices.length > 0" @click="removeConnection(connection)"><Trash2 class="h-4 w-4" /></Button></div></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="divide-y divide-border md:hidden">
        <article v-for="connection in connections" :key="connection.id" class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3"><div class="min-w-0"><div class="font-medium">{{ connection.name }}</div><div class="mt-1 break-all font-mono text-xs text-muted-foreground">{{ connection.baseUrl }}</div></div><Switch :model-value="connection.enabled" :disabled="busy" @update:model-value="value => setEnabled(connection, Boolean(value))" /></div>
          <div class="font-mono text-xs">{{ connection.probeModel ?? '未配置探针模型' }}</div>
          <ApiHealth24HourStrip :summary="connection.healthSummary" compact />
          <div class="flex items-center justify-between"><span class="text-xs text-muted-foreground">引用 {{ connection.referencedServices.length }} 项服务</span><div class="flex gap-1"><Button size="icon" variant="ghost" title="重新验证" @click="verify(connection)"><RefreshCw class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="编辑" @click="openEdit(connection)"><FilePenLine class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="删除" :disabled="connection.referencedServices.length > 0" @click="removeConnection(connection)"><Trash2 class="h-4 w-4" /></Button></div></div>
        </article>
      </div>
    </div>

    <Dialog v-model:open="formOpen">
      <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader><DialogTitle>{{ editing ? '编辑探针连接' : '新建探针连接' }}</DialogTitle><DialogDescription>读取当前 Key 可见的模型，再选择一个模型用于每五分钟的真实请求。</DialogDescription></DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2"><label class="space-y-2"><span class="text-sm font-medium">连接名称</span><Input v-model="form.name" placeholder="主 Sub2API" maxlength="80" /></label><label class="space-y-2"><span class="text-sm font-medium">Base URL</span><Input v-model="form.baseUrl" placeholder="https://api.example.com/v1" inputmode="url" /></label></div>
          <label class="block space-y-2"><span class="text-sm font-medium">探针专用 API Key</span><Input v-model="form.credential" type="password" autocomplete="new-password" :placeholder="editing?.credentialConfigured ? '留空则使用当前 Key' : 'Bearer Key'" /><span class="block text-xs text-muted-foreground">只用于验证与周期探针，不会交付给买家。</span></label>

          <div v-if="duplicateConnections.length" class="space-y-2 rounded-md border border-amber-200 bg-amber-50/70 p-3"><div class="flex items-start gap-2 text-sm text-amber-950"><Link2 class="mt-0.5 h-4 w-4 shrink-0" /><div><div class="font-medium">已有相同 Base URL 的连接</div><p class="mt-1 text-xs">不同权限或额度的 Key 才需要创建独立连接。</p></div></div><div v-for="connection in duplicateConnections" :key="connection.id" class="flex items-center justify-between border-t border-amber-200 pt-2"><span class="text-sm font-medium">{{ connection.name }}</span><Button size="sm" variant="outline" @click="reuseConnection(connection)">复用</Button></div><Label class="flex items-start gap-2 text-xs"><Checkbox v-model="independentConnectionConfirmed" /><span>仍使用独立 Key 创建连接</span></Label></div>

          <div v-if="formUsesHTTP" class="rounded-md border border-amber-200 bg-amber-50/70 p-3"><Label class="flex items-start gap-2 text-sm"><Checkbox v-model="form.acknowledgeInsecureHttp" class="mt-0.5" /><span><span class="font-medium">确认使用未加密 HTTP</span><span class="mt-1 block text-xs">Key 会通过未加密连接发送，请使用低权限、低额度凭据。</span></span></Label></div>

          <div class="rounded-md border border-border p-3">
            <div class="flex flex-wrap items-center justify-between gap-2"><div><div class="text-sm font-medium">真实模型验证</div><div class="mt-1 text-xs text-muted-foreground">优先使用 Responses；仅在不支持时自动回退 Chat。</div></div><Button size="sm" variant="outline" :disabled="!canPreflight || busy" @click="runPreflight"><Activity class="h-4 w-4" />{{ preflight?.availableModels.length ? '验证所选模型' : '读取模型' }}</Button></div>
            <div v-if="preflight?.availableModels.length" class="mt-3 grid gap-3 sm:grid-cols-2">
               <label class="space-y-1.5"><span class="text-xs font-medium">探针模型</span><Select v-model="form.probeModel"><SelectTrigger class="w-full font-mono"><SelectValue placeholder="选择探针模型" /></SelectTrigger><SelectContent><SelectItem v-for="model in preflight.availableModels" :key="model" :value="model">{{ model }}{{ model === 'gpt-5.6-luna' ? ' · 推荐' : '' }}</SelectItem></SelectContent></Select></label>
              <div class="space-y-1.5"><span class="text-xs font-medium">基础成本上限</span><div class="flex h-9 items-center rounded-md bg-muted/50 px-3 text-sm tabular-nums">{{ preflight.priceUnavailable ? '价格未知' : `$${preflight.dailyBaseCostUpperBoundUsd} / 天` }}</div></div>
            </div>
            <div v-if="preflightValid" class="mt-3 flex items-center gap-2 text-xs text-emerald-700"><span class="h-2 w-2 rounded-[2px] bg-emerald-500" />{{ form.probeModel }} 验证通过<span v-if="preflight?.probeProtocol === 'openai_chat_completions_v1'">· Chat 回退</span></div>
            <div v-else-if="preflight?.errorCode && preflight.errorCode !== 'model_unavailable'" class="mt-3 flex items-start gap-2 text-xs text-destructive"><ShieldAlert class="h-4 w-4 shrink-0" />验证失败：{{ preflight.errorCode }}</div>
          </div>

          <Label class="flex items-center justify-between rounded-md border border-border p-3"><span><span class="block text-sm font-medium">启用周期探测</span><span class="mt-1 block text-xs text-muted-foreground">每五分钟一次；同一连接绑定多项服务也只请求一次。</span></span><Switch v-model="form.enabled" /></Label>
          <div v-if="editing?.referencedServices.length" class="flex items-start gap-2 rounded-md border border-border px-3 py-2 text-xs"><ShieldAlert class="mt-0.5 h-4 w-4 shrink-0 text-amber-600" /><span>当前被 {{ editing.referencedServices.length }} 项服务引用。更换模型后，新旧统计会分开计算。</span></div>
        </div>
        <DialogFooter><Button variant="outline" :disabled="busy" @click="formOpen = false">取消</Button><Button :disabled="!canSubmit || busy" @click="submitForm">保存连接</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
