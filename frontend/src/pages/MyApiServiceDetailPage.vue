<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import ApiQuotaOwnerManager from '@/components/api-quota/ApiQuotaOwnerManager.vue'
import ApiServiceOwnerHeader from '@/components/api-service-owner/ApiServiceOwnerHeader.vue'
import ApiServiceOwnerMetrics from '@/components/api-service-owner/ApiServiceOwnerMetrics.vue'
import ApiServiceOwnerOverview from '@/components/api-service-owner/ApiServiceOwnerOverview.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import RefreshCw from 'lucide-vue-next/dist/esm/icons/refresh-cw.js'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { backendErrorMessage } from '@/lib/backendClient'
import { getApiServiceProductIconSrc } from '@/lib/productCategoryIcon'
import {
  useMyApiService,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
  useUpdateApiServiceProbeConnectionMutation,
} from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { useOwnerAPIProbeConnections } from '@/queries/useApiHealthQueries'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: service, isLoading, error, refetch } = useMyApiService(id)
const { data: catalogCategories } = useProductCategories()
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const updateProbeConnectionMutation = useUpdateApiServiceProbeConnectionMutation()
const probeConnectionsQuery = useOwnerAPIProbeConnections()
const selectedProbeConnectionId = ref('')
const probeConnectionError = ref('')
const actionPending = computed(() => publishMutation.isPending.value || pauseMutation.isPending.value || resumeMutation.isPending.value)
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '无法读取这条 API 服务，请确认当前账号是发布者。')
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
const serviceIconSrc = computed(() => service.value ? getApiServiceProductIconSrc(service.value, categoryIconByCode.value) : null)
const probeConnections = computed(() => probeConnectionsQuery.data.value ?? [])
const readyProbeConnections = computed(() => probeConnections.value.filter(connection => connection.enabled && connection.verificationStatus === 'verified'))
const currentProbeConnection = computed(() => probeConnections.value.find(connection => connection.id === service.value?.probeConnectionId) ?? null)
const selectedProbeConnectionReady = computed(() => readyProbeConnections.value.some(connection => connection.id === selectedProbeConnectionId.value))
const probeSelectionChanged = computed(() => selectedProbeConnectionId.value !== (service.value?.probeConnectionId ?? ''))
const ownerSectionHashes = new Set(['#quota-offers'])

async function scrollToOwnerSection() {
  if (!import.meta.client || !ownerSectionHashes.has(route.hash) || !service.value) return
  await nextTick()
  document.getElementById(route.hash.slice(1))?.scrollIntoView({ block: 'start' })
}

watch([() => route.hash, () => service.value?.id], scrollToOwnerSection, { flush: 'post' })
watch(() => service.value?.probeConnectionId, value => {
  selectedProbeConnectionId.value = value ?? ''
  probeConnectionError.value = ''
}, { immediate: true })
onMounted(scrollToOwnerSection)

function publishService() {
  if (!service.value || actionPending.value) return
  publishMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '上线失败。'),
  })
}

function pauseService() {
  if (!service.value || actionPending.value) return
  if (!window.confirm('确认暂停这项 API 服务的接单？暂停后买家将无法创建新订单，已有订单不受影响。')) return
  pauseMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已暂停。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '暂停失败。'),
  })
}

function resumeService() {
  if (!service.value || actionPending.value) return
  resumeMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已恢复上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '恢复失败。'),
  })
}

async function saveProbeConnection() {
  if (!service.value || !selectedProbeConnectionReady.value || !probeSelectionChanged.value) return
  probeConnectionError.value = ''
  try {
    await updateProbeConnectionMutation.mutateAsync({
      id: service.value.id,
      probeConnectionId: selectedProbeConnectionId.value,
      version: service.value.version ?? 1,
    })
    toast.success('探针连接已改绑。')
  } catch (updateError) {
    probeConnectionError.value = backendErrorMessage(updateError, '探针连接改绑失败。')
  }
}

async function unbindProbeConnection() {
  if (!service.value?.probeConnectionId) return
  if (!window.confirm('确认解除当前探针连接？服务会立即暂停接收新订单，已有订单不受影响。')) return
  probeConnectionError.value = ''
  try {
    await updateProbeConnectionMutation.mutateAsync({
      id: service.value.id,
      probeConnectionId: '',
      version: service.value.version ?? 1,
    })
    toast.success('已解除探针连接，服务已暂停接收新订单。')
  } catch (updateError) {
    probeConnectionError.value = backendErrorMessage(updateError, '解除探针连接失败。')
  }
}
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="8" />

  <ErrorState v-else-if="!service" title="无法打开服务管理页" :description="errorMessage" @retry="refetch()" />

  <main v-else class="mx-auto w-full max-w-[1440px] space-y-5">
    <ApiServiceOwnerHeader
      :service="service"
      :icon-src="serviceIconSrc"
      :action-pending="actionPending"
      @publish="publishService"
      @pause="pauseService"
      @resume="resumeService"
    />

    <ApiServiceOwnerMetrics :service="service" />

    <ApiServiceOwnerOverview :service="service" />

    <section class="space-y-3 border-y border-border py-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <h2 class="flex items-center gap-2 text-sm font-semibold"><Activity class="h-4 w-4 text-primary" />探针连接</h2>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">
            {{ currentProbeConnection ? `当前绑定：${currentProbeConnection.name}` : service.probeConnectionId ? '当前连接不可用，请重新绑定。' : '当前服务尚未绑定探针连接，不能上线或接收新订单。' }}
          </p>
        </div>
        <Button as-child size="sm" variant="outline"><RouterLink to="/my/api-probe-connections">管理探针连接</RouterLink></Button>
      </div>

      <div v-if="probeConnectionsQuery.isLoading.value" class="flex items-center gap-2 text-xs text-muted-foreground"><RefreshCw class="h-4 w-4 animate-spin" />正在读取可用连接...</div>
      <div v-else-if="probeConnectionsQuery.error.value" class="flex flex-wrap items-center gap-2 text-xs text-destructive">
        <span>{{ backendErrorMessage(probeConnectionsQuery.error.value, '探针连接暂时无法读取。') }}</span>
        <Button size="sm" variant="outline" @click="probeConnectionsQuery.refetch()"><RefreshCw class="h-4 w-4" />重试</Button>
      </div>
      <div v-else class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-center">
        <Select v-model="selectedProbeConnectionId">
          <SelectTrigger class="w-full"><SelectValue placeholder="选择已验证且启用的连接" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="connection in readyProbeConnections" :key="connection.id" :value="connection.id">{{ connection.name }} · {{ connection.baseUrl }}</SelectItem>
          </SelectContent>
        </Select>
        <Button :disabled="!probeSelectionChanged || !selectedProbeConnectionReady || updateProbeConnectionMutation.isPending.value" @click="saveProbeConnection">保存改绑</Button>
        <Button v-if="service.probeConnectionId" variant="outline" :disabled="updateProbeConnectionMutation.isPending.value" @click="unbindProbeConnection">解除绑定</Button>
      </div>
      <p v-if="probeConnectionError" class="text-xs text-destructive" role="alert">{{ probeConnectionError }}</p>
      <p v-else-if="!probeConnectionsQuery.isLoading.value && !probeConnectionsQuery.error.value && readyProbeConnections.length === 0" class="text-xs text-warning">暂无已验证且启用的连接，请先前往探针连接页面创建或重新验证。</p>
    </section>

    <ApiQuotaOwnerManager :api-service-id="service.id" :distribution-system="service.delivery" :default-multiplier="service.defaultMultiplier" />
  </main>
</template>
