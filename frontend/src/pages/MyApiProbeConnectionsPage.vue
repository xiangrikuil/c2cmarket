<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Activity,
  FilePenLine,
  Link2,
  Plus,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
  Trash2,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { Badge } from '@/components/ui/badge'
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
import { Switch } from '@/components/ui/switch'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useCreateOwnerAPIProbeConnectionMutation,
  useDeleteOwnerAPIProbeConnectionMutation,
  useOwnerAPIProbeConnections,
  useUpdateOwnerAPIProbeConnectionMutation,
  useVerifyOwnerAPIProbeConnectionMutation,
} from '@/queries/useApiHealthQueries'
import type { OwnerAPIProbeConnection } from '@/types/apiHealth'

const route = useRoute()
const router = useRouter()
const query = useOwnerAPIProbeConnections()
const createMutation = useCreateOwnerAPIProbeConnectionMutation()
const updateMutation = useUpdateOwnerAPIProbeConnectionMutation()
const deleteMutation = useDeleteOwnerAPIProbeConnectionMutation()
const verifyMutation = useVerifyOwnerAPIProbeConnectionMutation()
const formOpen = ref(false)
const editing = ref<OwnerAPIProbeConnection | null>(null)
const independentConnectionConfirmed = ref(false)

const form = reactive({
  name: '',
  baseUrl: '',
  credential: '',
  enabled: true,
  acknowledgeInsecureHttp: false,
})

const connections = computed(() => query.data.value ?? [])
const busy = computed(() => createMutation.isPending.value || updateMutation.isPending.value)
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

const duplicateConnections = computed(() => {
  if (editing.value) return []
  const target = comparableBaseURL(form.baseUrl)
  if (!target) return []
  return connections.value.filter(connection => connection.normalizedBaseUrl === target || comparableBaseURL(connection.baseUrl) === target)
})

const canSubmit = computed(() => Boolean(
  form.name.trim()
  && comparableBaseURL(form.baseUrl)
  && (editing.value?.credentialConfigured || form.credential.trim())
  && (!formUsesHTTP.value || form.acknowledgeInsecureHttp)
  && (!duplicateConnections.value.length || independentConnectionConfirmed.value),
))

function verificationStatus(connection: OwnerAPIProbeConnection) {
  if (connection.verificationStatus === 'verified') return { label: '验证通过', tone: 'success' as const }
  if (connection.verificationStatus === 'failed') return { label: '验证失败', tone: 'risk' as const }
  return { label: '待验证', tone: 'waiting' as const }
}

function connectionState(connection: OwnerAPIProbeConnection) {
  if (!connection.enabled) return { label: '已停用', tone: 'warning' as const }
  return { label: '运行中', tone: 'success' as const }
}

function resetForm(connection?: OwnerAPIProbeConnection) {
  editing.value = connection ?? null
  form.name = connection?.name ?? ''
  form.baseUrl = connection?.baseUrl ?? ''
  form.credential = ''
  form.enabled = connection?.enabled ?? true
  form.acknowledgeInsecureHttp = connection?.baseUrl.toLowerCase().startsWith('http://') ?? false
  independentConnectionConfirmed.value = Boolean(connection)
}

function openCreate() {
  resetForm()
  formOpen.value = true
}

function openEdit(connection: OwnerAPIProbeConnection) {
  resetForm(connection)
  formOpen.value = true
}

function reuseConnection(connection: OwnerAPIProbeConnection) {
  formOpen.value = false
  toast.info(`可以在发布服务时选择“${connection.name}”。`)
  void router.push('/api-market/new')
}

async function submitForm() {
  if (!canSubmit.value || busy.value) return
  const payload = {
    name: form.name,
    baseUrl: form.baseUrl,
    credential: form.credential || undefined,
    enabled: form.enabled,
    acknowledgeInsecureHttp: form.acknowledgeInsecureHttp,
  }
  try {
    const connection = editing.value
      ? await updateMutation.mutateAsync({ ...payload, id: editing.value.id, version: editing.value.version })
      : await createMutation.mutateAsync(payload)
    formOpen.value = false
    toast.success(connection.verificationStatus === 'verified'
      ? '探针连接已保存并通过鉴权验证。'
      : '探针连接已保存，但鉴权验证未通过。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针连接保存失败。'))
  }
}

async function verify(connection: OwnerAPIProbeConnection) {
  try {
    const updated = await verifyMutation.mutateAsync({ id: connection.id, version: connection.version })
    toast.success(updated.verificationStatus === 'verified' ? '探针连接验证通过。' : '探针连接仍未通过验证。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针连接验证失败。'))
  }
}

async function setEnabled(connection: OwnerAPIProbeConnection, enabled: boolean) {
  try {
    await updateMutation.mutateAsync({
      id: connection.id,
      version: connection.version,
      name: connection.name,
      baseUrl: connection.baseUrl,
      enabled,
      acknowledgeInsecureHttp: connection.baseUrl.toLowerCase().startsWith('http://'),
    })
    toast.success(enabled ? '探针连接已启用。' : '探针连接已停用。')
  } catch (error) {
    toast.error(backendErrorMessage(error, enabled ? '探针连接启用失败。' : '探针连接停用失败。'))
  }
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
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针连接删除失败。'))
  }
}

watch(
  () => route.query.create,
  value => {
    if (value === '1') openCreate()
  },
  { immediate: true },
)
</script>

<template>
  <div class="mx-auto w-full max-w-[1440px] space-y-5">
    <PageTitle title="探针连接" description="集中管理可供多项 API 服务复用的 Base URL 与专用探针 Key。">
      <template #action><Button @click="openCreate"><Plus class="h-4 w-4" />新建连接</Button></template>
    </PageTitle>

    <div class="grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-3">
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">连接</div><div class="mt-1 text-xl font-semibold">{{ connections.length }}</div></div>
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">已验证并启用</div><div class="mt-1 text-xl font-semibold">{{ connections.filter(item => item.enabled && item.verificationStatus === 'verified').length }}</div></div>
      <div class="bg-card px-4 py-3"><div class="text-xs text-muted-foreground">引用服务</div><div class="mt-1 text-xl font-semibold">{{ connections.reduce((sum, item) => sum + item.referencedServices.length, 0) }}</div></div>
    </div>

    <SkeletonTable v-if="query.isLoading.value" :rows="4" :columns="6" />
    <ErrorState v-else-if="query.error.value" title="无法读取探针连接" :description="errorMessage" @retry="query.refetch()" />
    <EmptyState v-else-if="connections.length === 0" title="还没有探针连接" description="创建一次连接后，可让多项 API 服务直接复用。">
      <template #action><Button @click="openCreate"><Plus class="h-4 w-4" />新建连接</Button></template>
    </EmptyState>

    <div v-else class="overflow-hidden rounded-md border border-border bg-card">
      <div class="hidden overflow-x-auto md:block">
        <table class="c2c-table w-full min-w-[980px] text-sm">
          <thead><tr class="border-b border-border text-left text-xs text-muted-foreground"><th class="px-3 py-2 font-medium">连接</th><th class="px-3 py-2 font-medium">验证</th><th class="px-3 py-2 font-medium">运行</th><th class="px-3 py-2 font-medium">引用服务</th><th class="px-3 py-2 font-medium">最近更新</th><th class="px-3 py-2 text-right font-medium">操作</th></tr></thead>
          <tbody>
            <tr v-for="connection in connections" :key="connection.id" class="border-b border-border/70 last:border-0">
              <td class="max-w-[340px] px-3 py-3"><div class="font-medium">{{ connection.name }}</div><div class="mt-1 truncate font-mono text-xs text-muted-foreground" :title="connection.baseUrl">{{ connection.baseUrl }}</div></td>
              <td class="px-3 py-3"><StatusBadge :status="connection.verificationStatus" :label="verificationStatus(connection).label" :tone="verificationStatus(connection).tone" /><div v-if="connection.verifiedAt" class="mt-1 text-xs text-muted-foreground"><LocalTime :value="connection.verifiedAt" /></div></td>
              <td class="px-3 py-3"><div class="flex items-center gap-2"><Switch :model-value="connection.enabled" :disabled="updateMutation.isPending.value" @update:model-value="value => setEnabled(connection, Boolean(value))" /><span class="text-xs text-muted-foreground">{{ connectionState(connection).label }}</span></div></td>
              <td class="px-3 py-3">
                <div class="flex flex-wrap gap-1">
                  <Badge v-if="connection.referencedServices.length === 0" variant="outline">未引用</Badge>
                  <template v-else>
                    <Badge v-for="service in connection.referencedServices.slice(0, 3)" :key="service.id" variant="secondary">{{ service.title }}</Badge>
                    <Badge v-if="connection.referencedServices.length > 3" variant="outline">+{{ connection.referencedServices.length - 3 }}</Badge>
                  </template>
                </div>
              </td>
              <td class="px-3 py-3 text-xs text-muted-foreground"><LocalTime :value="connection.updatedAt" /></td>
              <td class="px-3 py-3"><div class="flex justify-end gap-1.5"><Button size="icon" variant="ghost" title="重新验证" :disabled="verifyMutation.isPending.value" @click="verify(connection)"><RefreshCw class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="编辑" @click="openEdit(connection)"><FilePenLine class="h-4 w-4" /></Button><Button size="icon" variant="ghost" title="删除" :disabled="deleteMutation.isPending.value || connection.referencedServices.length > 0" @click="removeConnection(connection)"><Trash2 class="h-4 w-4" /></Button></div></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="divide-y divide-border md:hidden">
        <article v-for="connection in connections" :key="connection.id" class="space-y-3 p-4">
          <div class="flex items-start justify-between gap-3"><div class="min-w-0"><div class="font-medium">{{ connection.name }}</div><div class="mt-1 break-all font-mono text-xs text-muted-foreground">{{ connection.baseUrl }}</div></div><StatusBadge :status="connection.verificationStatus" :label="verificationStatus(connection).label" :tone="verificationStatus(connection).tone" /></div>
          <div class="flex flex-wrap items-center justify-between gap-2"><span class="text-xs text-muted-foreground">引用 {{ connection.referencedServices.length }} 项服务</span><div class="flex items-center gap-2"><Switch :model-value="connection.enabled" @update:model-value="value => setEnabled(connection, Boolean(value))" /><span class="text-xs">{{ connectionState(connection).label }}</span></div></div>
          <div class="flex flex-wrap gap-2"><Button size="sm" variant="outline" @click="verify(connection)"><RefreshCw class="h-4 w-4" />验证</Button><Button size="sm" variant="outline" @click="openEdit(connection)"><FilePenLine class="h-4 w-4" />编辑</Button><Button size="sm" variant="outline" :disabled="connection.referencedServices.length > 0" @click="removeConnection(connection)"><Trash2 class="h-4 w-4" />删除</Button></div>
        </article>
      </div>
    </div>

    <Dialog v-model:open="formOpen">
      <DialogContent class="sm:max-w-2xl">
        <DialogHeader><DialogTitle>{{ editing ? '编辑探针连接' : '新建探针连接' }}</DialogTitle><DialogDescription>平台会使用专用 Key 请求 Base URL 下的 /models；不会验证服务器所有权或具体模型。</DialogDescription></DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2"><span class="text-sm font-medium">连接名称</span><Input v-model="form.name" placeholder="主 Sub2API" maxlength="80" /></label>
            <label class="space-y-2"><span class="text-sm font-medium">Base URL</span><Input v-model="form.baseUrl" placeholder="https://api.example.com/v1" inputmode="url" /></label>
          </div>
          <label class="block space-y-2"><span class="text-sm font-medium">探针专用 API Key</span><Input v-model="form.credential" type="password" autocomplete="new-password" :placeholder="editing?.credentialConfigured ? '留空则保持当前 Key' : 'Bearer Key'" /><span class="block text-xs text-muted-foreground">仅用于连接验证和周期鉴权探测，不会交付给买家。</span></label>

          <div v-if="duplicateConnections.length" class="space-y-2 rounded-md border border-amber-200 bg-amber-50/70 p-3">
            <div class="flex items-start gap-2 text-sm text-amber-950"><Link2 class="mt-0.5 h-4 w-4 shrink-0" /><div><div class="font-medium">已有相同 Base URL 的连接</div><p class="mt-1 text-xs leading-5 text-amber-900/80">优先复用可减少重复探测；不同权限或额度的 Key 才需要创建独立连接。</p></div></div>
            <div v-for="connection in duplicateConnections" :key="connection.id" class="flex flex-wrap items-center justify-between gap-2 border-t border-amber-200 pt-2"><div><div class="text-sm font-medium">{{ connection.name }}</div><div class="text-xs text-amber-900/70">{{ connection.verificationStatus === 'verified' ? '已验证' : '待验证' }} · 引用 {{ connection.referencedServices.length }} 项服务</div></div><Button size="sm" variant="outline" @click="reuseConnection(connection)">复用此连接</Button></div>
            <Label class="flex items-start gap-2 text-xs leading-5"><Checkbox v-model="independentConnectionConfirmed" class="mt-0.5" /><span>我需要使用不同权限或额度的 Key，仍创建独立连接。</span></Label>
          </div>

          <div v-if="formUsesHTTP" class="rounded-md border border-amber-200 bg-amber-50/70 p-3">
            <Label class="flex items-start gap-2 text-sm leading-6 text-amber-950"><Checkbox v-model="form.acknowledgeInsecureHttp" class="mt-1" /><span><span class="font-medium">确认使用未加密 HTTP</span><span class="block text-xs text-amber-900/75">专用 Key 会通过未加密连接发送，请使用低权限、低额度凭据。</span></span></Label>
          </div>

          <Label class="flex items-center justify-between rounded-md border border-border p-3"><span><span class="block text-sm font-medium">启用周期探测</span><span class="mt-1 block text-xs text-muted-foreground">验证通过后每五分钟最多探测一次。</span></span><Switch v-model="form.enabled" /></Label>

          <div class="flex items-start gap-2 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground"><ShieldCheck class="mt-0.5 h-4 w-4 shrink-0" /><span>同一连接可绑定多项 API 服务。连接停用或配置变更后，相关服务会暂停接收新订单，重新验证后恢复。</span></div>
          <div v-if="editing?.referencedServices.length" class="flex items-start gap-2 rounded-md border border-border px-3 py-2 text-xs leading-5"><ShieldAlert class="mt-0.5 h-4 w-4 shrink-0 text-amber-600" /><span>当前被 {{ editing.referencedServices.length }} 项服务引用。修改 Base URL 或 Key 会让这些服务暂时停止接单。</span></div>
        </div>
        <DialogFooter><Button variant="outline" :disabled="busy" @click="formOpen = false">取消</Button><Button :disabled="!canSubmit || busy" @click="submitForm"><Activity class="h-4 w-4" />保存并验证</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
