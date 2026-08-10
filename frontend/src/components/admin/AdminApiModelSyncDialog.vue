<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CheckCheck, CloudDownload, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BackendProblemError, backendErrorMessage } from '@/lib/backendClient'
import { useApplyAPIModelsDevSync, usePreviewAPIModelsDevSync } from '@/queries/useApiModelCatalogQueries'
import type {
  AdminApiModelProvider,
  ApiModelBulkMutationResult,
  ApiModelSyncItem,
  ApiModelSyncPreview,
  ApiModelSyncSelection,
  ApiModelSyncStatus,
  ModelsDevProviderCode,
} from '@/types/apiModelCatalog'

const props = defineProps<{
  open: boolean
  providers: AdminApiModelProvider[]
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  applied: [result: ApiModelBulkMutationResult]
}>()

const supportedProviderCodes = new Set<ModelsDevProviderCode>(['openai', 'anthropic', 'google', 'perplexity'])
const statusTabs: Array<{ value: ApiModelSyncStatus, label: string }> = [
  { value: 'new', label: '新增' },
  { value: 'price_changed', label: '价格变化' },
  { value: 'unchanged', label: '无变化' },
  { value: 'source_missing', label: '来源未返回' },
  { value: 'unavailable', label: '无法导入' },
]

const selectedProviderIds = ref<string[]>([])
const selectedCandidateKeys = ref<string[]>([])
const activeCandidateKeys = ref<string[]>([])
const activeStatus = ref<ApiModelSyncStatus>('new')
const previewMutation = usePreviewAPIModelsDevSync()
const applyMutation = useApplyAPIModelsDevSync()

const dialogOpen = computed({
  get: () => props.open,
  set: value => emit('update:open', value),
})
const supportedProviders = computed(() => props.providers.filter(provider => supportedProviderCodes.has(provider.code as ModelsDevProviderCode)))
const preview = computed(() => previewMutation.data.value ?? null)
const visibleItems = computed(() => preview.value?.items.filter(item => item.status === activeStatus.value) ?? [])
const selectedItems = computed(() => preview.value?.items.filter(item => selectedCandidateKeys.value.includes(item.candidateKey)) ?? [])
const isBusy = computed(() => previewMutation.isPending.value || applyMutation.isPending.value)

watch(() => props.open, (open) => {
  if (!open) return
  previewMutation.reset()
  applyMutation.reset()
  selectedProviderIds.value = supportedProviders.value.filter(provider => provider.active).map(provider => provider.id)
  selectedCandidateKeys.value = []
  activeCandidateKeys.value = []
  activeStatus.value = 'new'
})

function toggleProvider(providerId: string, checked: boolean) {
  selectedProviderIds.value = checked
    ? Array.from(new Set([...selectedProviderIds.value, providerId]))
    : selectedProviderIds.value.filter(id => id !== providerId)
}

function toggleCandidate(candidateKey: string, checked: boolean) {
  selectedCandidateKeys.value = checked
    ? Array.from(new Set([...selectedCandidateKeys.value, candidateKey]))
    : selectedCandidateKeys.value.filter(key => key !== candidateKey)
  if (!checked) activeCandidateKeys.value = activeCandidateKeys.value.filter(key => key !== candidateKey)
}

function toggleCandidateActive(candidateKey: string, active: boolean) {
  activeCandidateKeys.value = active
    ? Array.from(new Set([...activeCandidateKeys.value, candidateKey]))
    : activeCandidateKeys.value.filter(key => key !== candidateKey)
}

async function fetchPreview() {
  if (selectedProviderIds.value.length === 0) {
    toast.warning('请至少选择一个官方提供商。')
    return
  }
  try {
    const result = await previewMutation.mutateAsync([...selectedProviderIds.value])
    applyPreviewResult(result)
  } catch (error) {
    toast.error(backendErrorMessage(error, 'models.dev 数据获取失败'))
  }
}

function applyPreviewResult(result: ApiModelSyncPreview) {
  selectedCandidateKeys.value = result.items.filter(item => item.status === 'new').map(item => item.candidateKey)
  activeCandidateKeys.value = []
  activeStatus.value = result.counts.new > 0 ? 'new' : result.counts.priceChanged > 0 ? 'price_changed' : 'unchanged'
}

async function refreshPreviewAfterConflict() {
  applyMutation.reset()
  try {
    const result = await previewMutation.mutateAsync([...selectedProviderIds.value])
    applyPreviewResult(result)
    toast.warning('模型目录已更新，旧选择已失效。请重新确认预览后再应用。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '模型目录已变化，重新获取同步预览失败。'))
  }
}

function selectAllPriceChanges() {
  if (!preview.value) return
  selectedCandidateKeys.value = Array.from(new Set([
    ...selectedCandidateKeys.value,
    ...preview.value.items.filter(item => item.status === 'price_changed').map(item => item.candidateKey),
  ]))
}

async function applySelection() {
  if (selectedItems.value.length === 0) {
    toast.warning('请至少选择一个新增模型或价格变化。')
    return
  }
  const selections = selectedItems.value.map(toSyncSelection)
  try {
    const result = await applyMutation.mutateAsync(selections)
    toast.success(`已导入 ${result.created} 个模型，更新 ${result.updated} 个价格。`)
    emit('applied', result)
    dialogOpen.value = false
  } catch (error) {
    if (error instanceof BackendProblemError && error.code === 'VERSION_CONFLICT') {
      await refreshPreviewAfterConflict()
      return
    }
    toast.error(backendErrorMessage(error, '应用 models.dev 变化失败'))
  }
}

function toSyncSelection(item: ApiModelSyncItem): ApiModelSyncSelection {
  return {
    fingerprint: item.fingerprint,
    status: item.status as 'new' | 'price_changed',
    providerId: item.providerId,
    providerCode: item.providerCode as ModelsDevProviderCode,
    modelKey: item.modelKey,
    capabilities: item.capabilities.filter((capability): capability is 'text' | 'vision' | 'reasoning' => capability === 'text' || capability === 'vision' || capability === 'reasoning'),
    sourceUrl: 'https://models.dev/api.json',
    sourceVersion: item.sourceVersion ?? '',
    inputPricePerMillion: item.inputPricePerMillion ?? '',
    cachedInputPricePerMillion: item.cachedInputPricePerMillion ?? '',
    outputPricePerMillion: item.outputPricePerMillion ?? '',
    localModelId: item.localModelId ?? '',
    localPriceVersionId: item.localPriceVersionId ?? '',
    active: item.status === 'new' && activeCandidateKeys.value.includes(item.candidateKey),
  }
}

function statusCount(status: ApiModelSyncStatus) {
  const counts = preview.value?.counts
  if (!counts) return 0
  if (status === 'new') return counts.new
  if (status === 'price_changed') return counts.priceChanged
  if (status === 'unchanged') return counts.unchanged
  if (status === 'source_missing') return counts.sourceMissing
  return counts.unavailable
}

function canSelect(item: ApiModelSyncItem) {
  return item.status === 'new' || item.status === 'price_changed'
}

function priceLine(item: ApiModelSyncItem, local: boolean) {
  const input = local ? item.localInputPricePerMillion : item.inputPricePerMillion
  const cached = local ? item.localCachedInputPricePerMillion : item.cachedInputPricePerMillion
  const output = local ? item.localOutputPricePerMillion : item.outputPricePerMillion
  return `输入 ${input || '-'} · 缓存 ${cached || '-'} · 输出 ${output || '-'}`
}
</script>

<template>
  <Dialog v-model:open="dialogOpen">
    <DialogContent class="max-h-[calc(100dvh-1.5rem)] min-w-0 overflow-x-hidden p-0 sm:max-w-[min(1120px,calc(100vw-2rem))]">
      <DialogHeader class="border-b border-border px-5 pb-4 pt-5 pr-12">
        <DialogTitle>从 models.dev 更新</DialogTitle>
        <DialogDescription>选择官方提供商，审核模型和价格差异后再应用。</DialogDescription>
      </DialogHeader>

      <div class="min-w-0 space-y-5 px-5">
        <section class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="text-sm font-semibold">官方提供商</h3>
            <Button size="sm" variant="outline" :disabled="isBusy || selectedProviderIds.length === 0" @click="fetchPreview">
              <component :is="previewMutation.isPending.value ? LoaderCircle : RefreshCw" :class="['h-4 w-4', previewMutation.isPending.value && 'animate-spin']" />
              {{ preview ? '重新获取' : '获取预览' }}
            </Button>
          </div>
          <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            <label v-for="provider in supportedProviders" :key="provider.id" class="flex items-center gap-3 rounded-md border border-border px-3 py-2.5 text-sm">
              <Checkbox
                :model-value="selectedProviderIds.includes(provider.id)"
                :disabled="isBusy"
                @update:model-value="value => toggleProvider(provider.id, Boolean(value))"
              />
              <span class="min-w-0">
                <span class="block truncate font-medium">{{ provider.displayName }}</span>
                <span class="block truncate text-xs text-muted-foreground">{{ provider.code }}</span>
              </span>
            </label>
          </div>
        </section>

        <Alert v-if="previewMutation.isError.value" variant="destructive">
          <TriangleAlert class="h-4 w-4" />
          <AlertTitle>预览获取失败</AlertTitle>
          <AlertDescription>{{ previewMutation.error.value instanceof Error ? previewMutation.error.value.message : '请稍后重试。' }}</AlertDescription>
        </Alert>

        <section v-if="preview" class="min-w-0 space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <Tabs :model-value="activeStatus" @update:model-value="value => activeStatus = value as ApiModelSyncStatus">
              <TabsList class="h-auto flex-wrap justify-start">
                <TabsTrigger v-for="tab in statusTabs" :key="tab.value" :value="tab.value" class="gap-1.5">
                  {{ tab.label }} <span class="text-xs opacity-70">{{ statusCount(tab.value) }}</span>
                </TabsTrigger>
              </TabsList>
            </Tabs>
            <Button v-if="preview.counts.priceChanged > 0" size="sm" variant="outline" :disabled="isBusy" @click="selectAllPriceChanges">
              <CheckCheck class="h-4 w-4" />选择全部价格变化
            </Button>
          </div>

          <div class="w-full max-w-full overflow-x-auto rounded-md border border-border">
            <table class="c2c-table w-full min-w-[920px] text-sm">
              <thead>
                <tr class="border-b border-border text-left text-xs text-muted-foreground">
                  <th class="w-12 px-3 py-2 font-medium">选择</th>
                  <th class="px-3 py-2 font-medium">模型</th>
                  <th class="px-3 py-2 font-medium">本地价格</th>
                  <th class="px-3 py-2 font-medium">models.dev 价格</th>
                  <th class="px-3 py-2 font-medium">来源版本</th>
                  <th class="w-28 px-3 py-2 font-medium">导入后启用</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in visibleItems" :key="item.candidateKey" class="border-b border-border/70 last:border-0">
                  <td class="px-3 py-3">
                    <Checkbox
                      v-if="canSelect(item)"
                      :model-value="selectedCandidateKeys.includes(item.candidateKey)"
                      :disabled="isBusy"
                      @update:model-value="value => toggleCandidate(item.candidateKey, Boolean(value))"
                    />
                    <span v-else class="text-muted-foreground">-</span>
                  </td>
                  <td class="max-w-[260px] px-3 py-3">
                    <div class="break-all font-mono text-xs font-medium">{{ item.modelKey || item.provider }}</div>
                    <div class="mt-1 flex flex-wrap gap-1">
                      <Badge variant="model">{{ item.provider }}</Badge>
                      <Badge v-if="item.reasonCode" variant="secondary">{{ item.reasonCode }}</Badge>
                    </div>
                    <p v-if="item.reason" class="mt-1.5 text-xs leading-5 text-muted-foreground">{{ item.reason }}</p>
                  </td>
                  <td class="px-3 py-3 text-xs text-muted-foreground">{{ priceLine(item, true) }}</td>
                  <td class="px-3 py-3 text-xs">{{ priceLine(item, false) }}</td>
                  <td class="max-w-[180px] px-3 py-3 text-xs text-muted-foreground">
                    <div class="break-all">{{ item.sourceVersion || '-' }}</div>
                  </td>
                  <td class="px-3 py-3">
                    <div v-if="item.status === 'new'" class="flex items-center gap-2">
                      <Switch
                        :model-value="activeCandidateKeys.includes(item.candidateKey)"
                        :disabled="isBusy || !selectedCandidateKeys.includes(item.candidateKey)"
                        :aria-label="`导入后启用 ${item.modelKey}`"
                        @update:model-value="value => toggleCandidateActive(item.candidateKey, Boolean(value))"
                      />
                      <span class="text-xs text-muted-foreground">{{ activeCandidateKeys.includes(item.candidateKey) ? '启用' : '停用' }}</span>
                    </div>
                    <span v-else class="text-xs text-muted-foreground">保持不变</span>
                  </td>
                </tr>
                <tr v-if="visibleItems.length === 0">
                  <td colspan="6" class="px-3 py-10 text-center text-sm text-muted-foreground">此分类暂无记录。</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <DialogFooter class="border-t border-border px-5 pb-5 pt-4">
        <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <span class="text-xs text-muted-foreground">已选择 {{ selectedItems.length }} 项</span>
          <div class="flex justify-end gap-2">
            <Button variant="outline" :disabled="isBusy" @click="dialogOpen = false">取消</Button>
            <Button :disabled="!preview || selectedItems.length === 0 || isBusy" @click="applySelection">
              <component :is="applyMutation.isPending.value ? LoaderCircle : CloudDownload" :class="['h-4 w-4', applyMutation.isPending.value && 'animate-spin']" />
              确认应用
            </Button>
          </div>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
