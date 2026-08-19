<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FilePenLine, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiHealth24HourStrip from '@/components/api-market/ApiHealth24HourStrip.vue'
import ApiProbeConnectionDialog from '@/components/api-probe/ApiProbeConnectionDialog.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useDeleteOwnerAPIProbeConnectionMutation,
  useOwnerAPIProbeConnections,
  usePreflightOwnerAPIProbeConnectionMutation,
  useUpdateOwnerAPIProbeConnectionMutation,
  useVerifyOwnerAPIProbeConnectionMutation,
} from '@/queries/useApiHealthQueries'
import type { OwnerAPIProbeConnection } from '@/types/apiHealth'

const route = useRoute()
const router = useRouter()
const query = useOwnerAPIProbeConnections()
const preflightMutation = usePreflightOwnerAPIProbeConnectionMutation()
const updateMutation = useUpdateOwnerAPIProbeConnectionMutation()
const deleteMutation = useDeleteOwnerAPIProbeConnectionMutation()
const verifyMutation = useVerifyOwnerAPIProbeConnectionMutation()
const formOpen = ref(false)
const editing = ref<OwnerAPIProbeConnection | null>(null)

const connections = computed(() => query.data.value ?? [])
const busy = computed(() => preflightMutation.isPending.value || updateMutation.isPending.value)
const errorMessage = computed(() => backendErrorMessage(query.error.value, '探针连接暂时无法读取。'))

function verificationStatus(connection: OwnerAPIProbeConnection) {
  if (connection.verificationStatus === 'verified') return { label: '已验证', tone: 'success' as const }
  if (connection.verificationStatus === 'failed') return { label: '验证失败', tone: 'risk' as const }
  return { label: '待验证', tone: 'waiting' as const }
}

function openCreate() { editing.value = null; formOpen.value = true }
function openEdit(connection: OwnerAPIProbeConnection) { editing.value = connection; formOpen.value = true }

function reuseConnection(connection: OwnerAPIProbeConnection) {
  formOpen.value = false
  toast.info(`可以在发布服务时选择“${connection.name}”。`)
  void router.push('/api-market/new')
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

    <ApiProbeConnectionDialog
      v-model:open="formOpen"
      :connections="connections"
      :connection="editing"
      @reuse="reuseConnection"
    />
  </div>
</template>
