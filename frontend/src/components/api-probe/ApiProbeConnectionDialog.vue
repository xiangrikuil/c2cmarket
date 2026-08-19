<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Activity, Link2, ShieldAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useCreateOwnerAPIProbeConnectionMutation,
  usePreflightOwnerAPIProbeConnectionMutation,
  useUpdateOwnerAPIProbeConnectionMutation,
} from '@/queries/useApiHealthQueries'
import type { ApiProbeConnectionPreflight, OwnerAPIProbeConnection } from '@/types/apiHealth'

const props = defineProps<{
  open: boolean
  connections: OwnerAPIProbeConnection[]
  connection?: OwnerAPIProbeConnection | null
  requireEnabled?: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  saved: [connection: OwnerAPIProbeConnection]
  reuse: [connection: OwnerAPIProbeConnection]
}>()

const preflightMutation = usePreflightOwnerAPIProbeConnectionMutation()
const createMutation = useCreateOwnerAPIProbeConnectionMutation()
const updateMutation = useUpdateOwnerAPIProbeConnectionMutation()
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

const editing = computed(() => props.connection ?? null)
const busy = computed(() => (
  preflightMutation.isPending.value
  || createMutation.isPending.value
  || updateMutation.isPending.value
))
const formUsesHTTP = computed(() => form.baseUrl.trim().toLowerCase().startsWith('http://'))

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
  return [
    comparableBaseURL(form.baseUrl),
    form.credential.trim() || `stored:${editing.value?.id ?? ''}`,
    form.probeModel,
  ].join('|')
}

const duplicateConnections = computed(() => {
  if (editing.value) return []
  const target = comparableBaseURL(form.baseUrl)
  if (!target) return []
  return props.connections.filter(connection => (
    connection.normalizedBaseUrl === target || comparableBaseURL(connection.baseUrl) === target
  ))
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

function resetForm(connection?: OwnerAPIProbeConnection | null) {
  form.name = connection?.name ?? ''
  form.baseUrl = connection?.baseUrl ?? ''
  form.credential = ''
  form.probeModel = connection?.probeModel ?? ''
  form.enabled = props.requireEnabled ? true : (connection?.enabled ?? true)
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

function updateOpen(value: boolean) {
  if (!value && busy.value) return
  emit('update:open', value)
}

function canReuseConnection(connection: OwnerAPIProbeConnection) {
  return !props.requireEnabled || (connection.enabled && connection.verificationStatus === 'verified')
}

function reuseConnection(connection: OwnerAPIProbeConnection) {
  if (!canReuseConnection(connection)) return
  emit('reuse', connection)
  emit('update:open', false)
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
    if (!form.probeModel) {
      form.probeModel = result.probeModel
        ?? (result.availableModels.includes('gpt-5.6-luna') ? 'gpt-5.6-luna' : result.availableModels[0] ?? '')
    }
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
    emit('saved', connection)
    emit('update:open', false)
    toast.success(connection.enabled ? '探针连接已保存并开始周期探测。' : '探针连接已保存。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针连接保存失败。'))
  }
}

watch(() => props.open, (open) => {
  if (open) resetForm(props.connection)
})

watch(() => [form.baseUrl, form.credential, form.probeModel], () => {
  if (preflightFingerprint.value && preflightFingerprint.value !== currentFingerprint()) {
    preflightFingerprint.value = ''
  }
})
</script>

<template>
  <Dialog :open="open" @update:open="updateOpen">
    <DialogContent class="grid max-h-[calc(100dvh-1rem)] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-w-2xl">
      <DialogHeader class="border-b border-border px-4 py-4 pr-12 text-left sm:px-6">
        <DialogTitle>{{ editing ? '编辑探针连接' : '新建探针连接' }}</DialogTitle>
        <DialogDescription>读取当前 Key 可见的模型，再选择一个模型用于每五分钟的真实请求。</DialogDescription>
      </DialogHeader>

      <div class="min-w-0 space-y-4 overflow-y-auto px-4 py-4 sm:px-6">
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="min-w-0 space-y-2">
            <span class="text-sm font-medium">连接名称</span>
            <Input v-model="form.name" placeholder="主 Sub2API" maxlength="80" />
          </label>
          <label class="min-w-0 space-y-2">
            <span class="text-sm font-medium">Base URL</span>
            <Input v-model="form.baseUrl" placeholder="https://api.example.com/v1" inputmode="url" />
          </label>
        </div>

        <label class="block min-w-0 space-y-2">
          <span class="text-sm font-medium">探针专用 API Key</span>
          <Input
            v-model="form.credential"
            type="password"
            autocomplete="new-password"
            :placeholder="editing?.credentialConfigured ? '留空则使用当前 Key' : 'Bearer Key'"
          />
          <span class="block text-xs text-muted-foreground">只用于验证与周期探针，不会交付给买家。</span>
        </label>

        <div v-if="duplicateConnections.length" class="space-y-2 rounded-md border border-amber-200 bg-amber-50/70 p-3">
          <div class="flex items-start gap-2 text-sm text-amber-950">
            <Link2 class="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <div class="font-medium">已有相同 Base URL 的连接</div>
              <p class="mt-1 text-xs">不同权限或额度的 Key 才需要创建独立连接。</p>
            </div>
          </div>
          <div
            v-for="duplicate in duplicateConnections"
            :key="duplicate.id"
            class="flex items-center justify-between gap-3 border-t border-amber-200 pt-2"
          >
            <span class="min-w-0 truncate text-sm font-medium">{{ duplicate.name }}</span>
            <Button
              size="sm"
              variant="outline"
              type="button"
              :disabled="!canReuseConnection(duplicate)"
              @click="reuseConnection(duplicate)"
            >
              {{ canReuseConnection(duplicate) ? '复用' : '当前不可用' }}
            </Button>
          </div>
          <Label class="flex items-start gap-2 text-xs">
            <Checkbox v-model="independentConnectionConfirmed" />
            <span>仍使用独立 Key 创建连接</span>
          </Label>
        </div>

        <div v-if="formUsesHTTP" class="rounded-md border border-amber-200 bg-amber-50/70 p-3">
          <Label class="flex items-start gap-2 text-sm">
            <Checkbox v-model="form.acknowledgeInsecureHttp" class="mt-0.5" />
            <span>
              <span class="font-medium">确认使用未加密 HTTP</span>
              <span class="mt-1 block text-xs">Key 会通过未加密连接发送，请使用低权限、低额度凭据。</span>
            </span>
          </Label>
        </div>

        <div class="rounded-md border border-border p-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <div class="text-sm font-medium">真实模型验证</div>
              <div class="mt-1 text-xs text-muted-foreground">优先使用 Responses；仅在不支持时自动回退 Chat。</div>
            </div>
            <Button size="sm" variant="outline" type="button" :disabled="!canPreflight || busy" @click="runPreflight">
              <Activity class="h-4 w-4" />{{ preflight?.availableModels.length ? '验证所选模型' : '读取模型' }}
            </Button>
          </div>
          <div v-if="preflight?.availableModels.length" class="mt-3 grid gap-3 sm:grid-cols-2">
            <label class="min-w-0 space-y-1.5">
              <span class="text-xs font-medium">探针模型</span>
              <Select v-model="form.probeModel">
                <SelectTrigger class="w-full font-mono"><SelectValue placeholder="选择探针模型" /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="model in preflight.availableModels" :key="model" :value="model">
                    {{ model }}{{ model === 'gpt-5.6-luna' ? ' · 推荐' : '' }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
            <div class="space-y-1.5">
              <span class="text-xs font-medium">基础成本上限</span>
              <div class="flex h-9 items-center rounded-md bg-muted/50 px-3 text-sm tabular-nums">
                {{ preflight.priceUnavailable ? '价格未知' : `$${preflight.dailyBaseCostUpperBoundUsd} / 天` }}
              </div>
            </div>
          </div>
          <div v-if="preflightValid" class="mt-3 flex items-center gap-2 text-xs text-emerald-700">
            <span class="h-2 w-2 rounded-[2px] bg-emerald-500" />
            {{ form.probeModel }} 验证通过
            <span v-if="preflight?.probeProtocol === 'openai_chat_completions_v1'">· Chat 回退</span>
          </div>
          <div v-else-if="preflight?.errorCode && preflight.errorCode !== 'model_unavailable'" class="mt-3 flex items-start gap-2 text-xs text-destructive">
            <ShieldAlert class="h-4 w-4 shrink-0" />验证失败：{{ preflight.errorCode }}
          </div>
        </div>

        <Label class="flex items-center justify-between gap-3 rounded-md border border-border p-3">
          <span>
            <span class="block text-sm font-medium">启用周期探测</span>
            <span class="mt-1 block text-xs text-muted-foreground">
              {{ requireEnabled ? '发布服务需要启用探针；创建后会立即用于当前表单。' : '每五分钟一次；同一连接绑定多项服务也只请求一次。' }}
            </span>
          </span>
          <Switch v-model="form.enabled" :disabled="requireEnabled" />
        </Label>
        <div v-if="editing?.referencedServices.length" class="flex items-start gap-2 rounded-md border border-border px-3 py-2 text-xs">
          <ShieldAlert class="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
          <span>当前被 {{ editing.referencedServices.length }} 项服务引用。更换模型后，新旧统计会分开计算。</span>
        </div>
      </div>

      <DialogFooter class="border-t border-border px-4 py-4 sm:px-6">
        <Button variant="outline" type="button" :disabled="busy" @click="updateOpen(false)">取消</Button>
        <Button type="button" :disabled="!canSubmit || busy" @click="submitForm">
          {{ busy ? '处理中...' : '保存连接' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
