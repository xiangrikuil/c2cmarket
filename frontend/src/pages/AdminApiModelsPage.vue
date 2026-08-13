<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Ban, CloudDownload, FilePenLine, MoreHorizontal, Plus, RefreshCcw, RotateCcw, Save, Search, TriangleAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AdminApiModelSyncDialog from '@/components/admin/AdminApiModelSyncDialog.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  useAdminAPIModelProviders,
  useAdminAPIModels,
  useCreateAPIModel,
  useCreateAPIModelProvider,
  useApplyAPIModelLifecycle,
  useApplyAPIModelProviderLifecycle,
  useUpdateAPIModel,
  useUpdateAPIModelProvider,
} from '@/queries/useApiModelCatalogQueries'
import {
  apiModelCapabilities,
  apiModelProviderCategories,
  type AdminApiModel,
  type AdminApiModelProvider,
  type ApiModelCapability,
  type ApiModelInput,
  type ApiModelProviderInput,
  type CatalogLifecycleAction,
  type CatalogStatus,
} from '@/types/apiModelCatalog'

type CatalogTab = 'models' | 'providers'
type StatusFilter = 'all' | CatalogStatus

const activeCatalogTab = ref<CatalogTab>('models')
const statusFilter = ref<StatusFilter>('all')
const modelSearch = ref('')
const providerFilter = ref('all')
const editingModelId = ref('')
const editingProviderId = ref('')
const isModelFormOpen = ref(false)
const isProviderFormOpen = ref(false)
const isSyncDialogOpen = ref(false)
const lifecycleTarget = ref<{ kind: 'provider' | 'model', item: AdminApiModelProvider | AdminApiModel, action: CatalogLifecycleAction } | null>(null)
const lifecycleReason = ref('')
const unblockTargetStatus = ref<'active' | 'deprecated'>('active')

const providersQuery = useAdminAPIModelProviders()
const modelsQuery = useAdminAPIModels()
const createProviderMutation = useCreateAPIModelProvider()
const updateProviderMutation = useUpdateAPIModelProvider()
const providerLifecycleMutation = useApplyAPIModelProviderLifecycle()
const createModelMutation = useCreateAPIModel()
const updateModelMutation = useUpdateAPIModel()
const modelLifecycleMutation = useApplyAPIModelLifecycle()

const providers = computed(() => providersQuery.data.value ?? [])
const activeProviders = computed(() => providers.value.filter(item => item.effectiveStatus === 'active'))
const rows = computed(() => modelsQuery.data.value ?? [])
const providerForm = reactive<ApiModelProviderInput>(emptyProviderForm())
const modelForm = reactive<ApiModelInput>(emptyModelForm())
const visibleRows = computed(() => {
  const search = modelSearch.value.trim().toLowerCase()
  return rows.value.filter((item) => {
    if (!matchesStatusFilter(item)) return false
    if (providerFilter.value !== 'all' && item.providerId !== providerFilter.value) return false
    if (!search) return true
    return [item.modelKey, item.provider, item.providerCode]
      .some(value => value.toLowerCase().includes(search))
  })
})
const isSavingProvider = computed(() => createProviderMutation.isPending.value || updateProviderMutation.isPending.value)
const isSavingModel = computed(() => createModelMutation.isPending.value || updateModelMutation.isPending.value)
const isLoading = computed(() => providersQuery.isLoading.value || modelsQuery.isLoading.value)
const hasError = computed(() => providersQuery.isError.value || modelsQuery.isError.value)
const errorMessage = computed(() => {
  const error = providersQuery.error.value ?? modelsQuery.error.value
  return error instanceof Error ? error.message : 'API 模型目录读取失败，请确认管理权限后重试。'
})
const providerFormTitle = computed(() => editingProviderId.value ? '编辑 API 提供商' : '新建 API 提供商')
const modelFormTitle = computed(() => editingModelId.value ? '编辑 API 模型' : '新建 API 模型')
const editingProvider = computed(() => providers.value.find(item => item.id === editingProviderId.value) ?? null)
const editingModel = computed(() => rows.value.find(item => item.id === editingModelId.value) ?? null)

const providerLabelMap = Object.fromEntries(apiModelProviderCategories.map(item => [item.value, item.label])) as Record<string, string>

function openCreateProvider() {
  editingProviderId.value = ''
  fillProviderForm(emptyProviderForm())
  isProviderFormOpen.value = true
}

function openEditProvider(provider: AdminApiModelProvider) {
  editingProviderId.value = provider.id
  fillProviderForm(inputFromProvider(provider))
  isProviderFormOpen.value = true
}

function openCreateModel() {
  editingModelId.value = ''
  fillModelForm(emptyModelForm())
  isModelFormOpen.value = true
}

function openEditModel(model: AdminApiModel) {
  editingModelId.value = model.id
  fillModelForm(inputFromModel(model))
  isModelFormOpen.value = true
}

function resetProviderForm() {
  fillProviderForm(editingProvider.value ? inputFromProvider(editingProvider.value) : emptyProviderForm())
}

function resetModelForm() {
  fillModelForm(editingModel.value ? inputFromModel(editingModel.value) : emptyModelForm())
}

function emptyProviderForm(): ApiModelProviderInput {
  return {
    providerCategory: 'gpt',
    code: '',
    displayName: '',
    sortOrder: nextProviderSortOrder(),
  }
}

function emptyModelForm(): ApiModelInput {
  return {
    providerId: activeProviders.value[0]?.id ?? '',
    modelKey: '',
    capabilities: ['chat'],
    inputTokenPrice: '',
    cachedInputTokenPrice: '',
    outputTokenPrice: '',
    sourceUrl: '',
    sourceVersion: '',
    sortOrder: nextModelSortOrder(),
  }
}

function fillProviderForm(input: ApiModelProviderInput) {
  Object.assign(providerForm, input)
}

function fillModelForm(input: ApiModelInput) {
  Object.assign(modelForm, input)
}

function inputFromProvider(provider: AdminApiModelProvider): ApiModelProviderInput {
  return {
    providerCategory: provider.providerCategory,
    code: provider.code,
    displayName: provider.displayName,
    sortOrder: provider.sortOrder,
  }
}

function inputFromModel(model: AdminApiModel): ApiModelInput {
  return {
    providerId: model.providerId,
    modelKey: model.modelKey,
    capabilities: [...model.capabilities],
    inputTokenPrice: model.inputPricePerMillion ?? '',
    cachedInputTokenPrice: model.cachedInputPricePerMillion ?? '',
    outputTokenPrice: model.outputPricePerMillion ?? '',
    sourceUrl: model.currentPriceSourceUrl ?? '',
    sourceVersion: model.currentPriceSourceVersion ?? '',
    sortOrder: model.sortOrder,
  }
}

function validateProviderForm() {
  if (!providerForm.code.trim()) return '请填写提供商 code。'
  if (!providerForm.displayName.trim()) return '请填写提供商展示名。'
  return ''
}

function validateModelForm() {
  if (!modelForm.providerId.trim()) return '请选择 API 提供商。'
  if (!activeProviders.value.some(item => item.id === modelForm.providerId)) return '请选择启用中的 API 提供商。'
  if (!modelForm.modelKey.trim()) return '请填写模型标识。'
  if (modelForm.capabilities.length === 0) return '至少选择一种能力。'
  for (const field of [modelForm.inputTokenPrice, modelForm.cachedInputTokenPrice, modelForm.outputTokenPrice]) {
    if (!field.trim()) continue
    const numeric = Number(field)
    if (!Number.isFinite(numeric) || numeric < 0) return '价格必须是非负数字。'
  }
  return ''
}

async function saveProvider() {
  const error = validateProviderForm()
  if (error) {
    toast.warning(error)
    return
  }
  const input = { ...providerForm }
  try {
    if (editingProviderId.value) {
      await updateProviderMutation.mutateAsync({ id: editingProviderId.value, input })
      toast.success('API 提供商已更新。')
    } else {
      await createProviderMutation.mutateAsync(input)
      toast.success('API 提供商已创建。')
    }
    isProviderFormOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '保存 API 提供商失败')
  }
}

async function saveModel() {
  const error = validateModelForm()
  if (error) {
    toast.warning(error)
    return
  }
  const input = { ...modelForm, capabilities: [...modelForm.capabilities] }
  try {
    if (editingModelId.value) {
      await updateModelMutation.mutateAsync({ id: editingModelId.value, input })
      toast.success('API 模型已更新。')
    } else {
      await createModelMutation.mutateAsync(input)
      toast.success('API 模型已创建。')
    }
    isModelFormOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '保存 API 模型失败')
  }
}

function openLifecycle(kind: 'provider' | 'model', item: AdminApiModelProvider | AdminApiModel, action: CatalogLifecycleAction) {
  lifecycleTarget.value = { kind, item, action }
  lifecycleReason.value = ''
  unblockTargetStatus.value = 'active'
}

async function applyLifecycle() {
  const target = lifecycleTarget.value
  if (!target) return
  if (lifecycleReason.value.trim().length < 2) {
    toast.warning('请填写至少 2 个字符的状态变更原因。')
    return
  }
  const input = {
    id: target.item.id,
    version: target.item.version,
    action: target.action,
    reason: lifecycleReason.value,
    targetStatus: target.action === 'unblock' ? unblockTargetStatus.value : undefined,
  }
  try {
    if (target.kind === 'provider') await providerLifecycleMutation.mutateAsync(input)
    else await modelLifecycleMutation.mutateAsync(input)
    toast.success('目录状态已更新。')
    lifecycleTarget.value = null
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '目录状态更新失败')
  }
}

function lifecycleActions(status: CatalogStatus): CatalogLifecycleAction[] {
  if (status === 'active') return ['deprecate', 'block']
  if (status === 'deprecated') return ['reactivate', 'block']
  return ['unblock']
}

function lifecycleActionLabel(action: CatalogLifecycleAction) {
  return { deprecate: '退役目录', block: '紧急阻断', reactivate: '重新启用', unblock: '解除阻断' }[action]
}

function statusLabel(status: CatalogStatus) {
  return { active: '可用', deprecated: '已退役', blocked: '已阻断' }[status]
}

function statusClass(status: CatalogStatus) {
  if (status === 'active') return 'border-success/30 bg-success/10 text-success'
  if (status === 'blocked') return 'border-destructive/30 bg-destructive/10 text-destructive'
  return 'border-border bg-muted text-muted-foreground'
}

function setCapability(capability: ApiModelCapability, checked: boolean) {
  modelForm.capabilities = checked
    ? Array.from(new Set([...modelForm.capabilities, capability]))
    : modelForm.capabilities.filter(item => item !== capability)
}

function matchesStatusFilter(item: AdminApiModel) {
  if (statusFilter.value === 'all') return true
  return item.effectiveStatus === statusFilter.value
}

function nextProviderSortOrder() {
  const max = Math.max(0, ...providers.value.map(item => item.sortOrder))
  return max + 10
}

function nextModelSortOrder() {
  const max = Math.max(0, ...rows.value.map(item => item.sortOrder))
  return max + 10
}

</script>

<template>
  <div class="min-w-0 space-y-4">
    <PageTitle title="API 模型目录">
      <template #action>
        <div class="flex w-full flex-wrap items-center gap-2 md:w-auto md:justify-end">
          <template v-if="activeCatalogTab === 'models'">
            <Button size="sm" variant="outline" title="从 models.dev 同步价格" :disabled="providers.length === 0" @click="isSyncDialogOpen = true">
              <CloudDownload class="h-4 w-4" />同步价格
            </Button>
            <Button size="sm" :disabled="activeProviders.length === 0" @click="openCreateModel">
              <Plus class="h-4 w-4" />新建模型
            </Button>
          </template>
          <Button v-else size="sm" @click="openCreateProvider">
            <Plus class="h-4 w-4" />新建提供商
          </Button>
        </div>
      </template>
    </PageTitle>

    <Tabs v-model="activeCatalogTab" class="min-w-0 gap-0">
      <div class="overflow-x-auto border-b border-border">
        <TabsList class="h-auto min-w-max rounded-none bg-transparent p-0">
          <TabsTrigger value="models" class="rounded-none border-0 border-b-2 border-transparent px-4 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            模型 <span class="font-mono text-xs">{{ rows.length }}</span>
          </TabsTrigger>
          <TabsTrigger value="providers" class="rounded-none border-0 border-b-2 border-transparent px-4 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            提供商 <span class="font-mono text-xs">{{ providers.length }}</span>
          </TabsTrigger>
        </TabsList>
      </div>

      <SkeletonTable v-if="isLoading" class="mt-4" :rows="8" :columns="activeCatalogTab === 'models' ? 6 : 4" />

      <Card v-else-if="hasError" class="mt-4 border-destructive/30 p-5">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-destructive/10 text-destructive">
            <TriangleAlert class="h-5 w-5" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="font-semibold">API 模型目录读取失败</h2>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ errorMessage }}</p>
            <Button class="mt-4" size="sm" :disabled="modelsQuery.isFetching.value || providersQuery.isFetching.value" @click="providersQuery.refetch(); modelsQuery.refetch()">重新读取</Button>
          </div>
        </div>
      </Card>

      <template v-else>
        <TabsContent value="models" class="mt-4 min-w-0 space-y-3">
          <div class="grid min-w-0 gap-2 lg:grid-cols-[minmax(240px,1fr)_auto_minmax(180px,220px)]">
            <div class="relative min-w-0">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input v-model="modelSearch" class="pl-9" aria-label="搜索模型" placeholder="搜索模型" />
            </div>

            <div class="flex w-fit max-w-full items-center gap-1 overflow-x-auto rounded-lg border border-border bg-muted/40 p-1" aria-label="模型状态筛选">
              <Button size="sm" class="shrink-0" :variant="statusFilter === 'all' ? 'default' : 'ghost'" @click="statusFilter = 'all'">全部</Button>
              <Button size="sm" class="shrink-0" :variant="statusFilter === 'active' ? 'default' : 'ghost'" @click="statusFilter = 'active'">已启用</Button>
              <Button size="sm" class="shrink-0" :variant="statusFilter === 'deprecated' ? 'default' : 'ghost'" @click="statusFilter = 'deprecated'">已退役</Button>
              <Button size="sm" class="shrink-0" :variant="statusFilter === 'blocked' ? 'default' : 'ghost'" @click="statusFilter = 'blocked'">已阻断</Button>
            </div>

            <Select v-model="providerFilter">
              <SelectTrigger class="w-full" aria-label="按提供商筛选">
                <SelectValue placeholder="全部提供商" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部提供商</SelectItem>
                <SelectItem v-for="provider in providers" :key="provider.id" :value="provider.id">{{ provider.displayName }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="min-w-0 overflow-hidden rounded-lg border border-border bg-card">
            <div class="max-w-full overflow-x-auto">
              <table class="c2c-table w-full min-w-[920px] text-sm">
                <thead class="sticky top-0 z-10 bg-muted/95">
                  <tr class="border-b border-border text-left text-xs text-muted-foreground">
                    <th class="px-3 py-2.5 font-medium">模型</th>
                    <th class="px-3 py-2.5 font-medium">提供商</th>
                    <th class="px-3 py-2.5 font-medium">官网价格</th>
                    <th class="px-3 py-2.5 font-medium">状态</th>
                    <th class="w-36 px-3 py-2.5 text-right font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="model in visibleRows" :key="model.id" class="border-b border-border/70 last:border-0">
                    <td class="max-w-[280px] px-3 py-2.5">
                      <div class="truncate font-mono font-medium" :title="model.modelKey">{{ model.modelKey }}</div>
                      <div v-if="model.identityLocked" class="mt-1 text-xs text-muted-foreground" :title="model.identityLockReason">身份已锁定</div>
                    </td>
                    <td class="px-3 py-2.5">
                      <div class="font-medium">{{ model.provider }}</div>
                      <div class="mt-0.5 text-xs text-muted-foreground">{{ model.providerCode }}</div>
                    </td>
                    <td class="min-w-[340px] px-3 py-2.5">
                      <div class="grid grid-cols-3 gap-3 text-xs">
                        <span><span class="text-muted-foreground">输入</span> <span class="font-mono">{{ model.inputPricePerMillion || '-' }}</span></span>
                        <span><span class="text-muted-foreground">缓存</span> <span class="font-mono">{{ model.cachedInputPricePerMillion || '-' }}</span></span>
                        <span><span class="text-muted-foreground">输出</span> <span class="font-mono">{{ model.outputPricePerMillion || '-' }}</span></span>
                      </div>
                    </td>
                    <td class="px-3 py-2.5">
                      <div class="space-y-1.5">
                        <Badge variant="outline" :class="statusClass(model.status)">{{ statusLabel(model.status) }}</Badge>
                        <div v-if="model.effectiveStatusSource === 'parent'" class="text-xs text-warning">因提供商{{ statusLabel(model.effectiveStatus) }}</div>
                      </div>
                    </td>
                    <td class="px-3 py-2.5 text-right">
                      <div class="flex justify-end gap-1">
                        <Button size="icon" variant="ghost" title="编辑模型" aria-label="编辑模型" @click="openEditModel(model)"><FilePenLine class="h-4 w-4" /></Button>
                        <DropdownMenu>
                          <DropdownMenuTrigger as-child><Button size="icon" variant="ghost" title="目录状态操作" aria-label="目录状态操作"><MoreHorizontal class="h-4 w-4" /></Button></DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem v-for="action in lifecycleActions(model.status)" :key="action" :class="action === 'block' ? 'text-destructive' : ''" @select="openLifecycle('model', model, action)">
                              <Ban v-if="action === 'block'" class="h-4 w-4" /><RefreshCcw v-else class="h-4 w-4" />{{ lifecycleActionLabel(action) }}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="visibleRows.length === 0">
                    <td colspan="5" class="px-3 py-10 text-center text-sm text-muted-foreground">当前筛选下暂无 API 模型。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="providers" class="mt-4 min-w-0">
          <div class="min-w-0 overflow-hidden rounded-lg border border-border bg-card">
            <div class="max-w-full overflow-x-auto">
              <table class="c2c-table w-full min-w-[680px] text-sm">
                <thead class="sticky top-0 z-10 bg-muted/95">
                  <tr class="border-b border-border text-left text-xs text-muted-foreground">
                    <th class="px-3 py-2.5 font-medium">提供商</th>
                    <th class="px-3 py-2.5 font-medium">分类</th>
                    <th class="px-3 py-2.5 font-medium">状态</th>
                    <th class="w-24 px-3 py-2.5 text-right font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="provider in providers" :key="provider.id" class="border-b border-border/70 last:border-0">
                    <td class="px-3 py-2.5">
                      <div class="font-medium">{{ provider.displayName }}</div>
                      <div class="mt-0.5 font-mono text-xs text-muted-foreground">{{ provider.code }}</div>
                    </td>
                    <td class="px-3 py-2.5 text-muted-foreground">{{ providerLabelMap[provider.providerCategory] ?? provider.providerCategory }}</td>
                    <td class="px-3 py-2.5">
                      <Badge variant="outline" :class="statusClass(provider.status)">{{ statusLabel(provider.status) }}</Badge>
                    </td>
                    <td class="px-3 py-2.5 text-right">
                      <div class="flex justify-end gap-1">
                        <Button size="icon" variant="ghost" title="编辑提供商" aria-label="编辑提供商" @click="openEditProvider(provider)"><FilePenLine class="h-4 w-4" /></Button>
                        <DropdownMenu>
                          <DropdownMenuTrigger as-child><Button size="icon" variant="ghost" title="目录状态操作" aria-label="目录状态操作"><MoreHorizontal class="h-4 w-4" /></Button></DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem v-for="action in lifecycleActions(provider.status)" :key="action" :class="action === 'block' ? 'text-destructive' : ''" @select="openLifecycle('provider', provider, action)">
                              <Ban v-if="action === 'block'" class="h-4 w-4" /><RefreshCcw v-else class="h-4 w-4" />{{ lifecycleActionLabel(action) }}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="providers.length === 0">
                    <td colspan="4" class="px-3 py-10 text-center text-sm text-muted-foreground">暂无 API 提供商。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </TabsContent>
      </template>
    </Tabs>

    <Dialog v-model:open="isProviderFormOpen">
      <DialogContent class="sm:max-w-xl">
        <DialogHeader>
          <div class="flex flex-wrap items-start justify-between gap-3 pr-8">
            <div>
              <DialogTitle>{{ providerFormTitle }}</DialogTitle>
              <DialogDescription class="mt-2">提供商目录用于模型归类和发布页展示。</DialogDescription>
            </div>
            <Button size="sm" variant="outline" @click="resetProviderForm"><RotateCcw class="h-4 w-4" />重置</Button>
          </div>
        </DialogHeader>

        <div class="space-y-4">
          <label class="space-y-2">
            <span class="text-sm font-medium">分类</span>
            <Input v-model="providerForm.providerCategory" :disabled="Boolean(editingProvider?.identityLocked)" placeholder="例如 deepseek" />
            <span v-if="editingProvider?.identityLocked" class="block text-xs text-muted-foreground">{{ editingProvider.identityLockReason }}</span>
          </label>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">Code</span>
              <Input v-model="providerForm.code" :disabled="Boolean(editingProvider?.identityLocked)" placeholder="openai" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">展示名</span>
              <Input v-model="providerForm.displayName" placeholder="OpenAI" />
            </label>
          </div>
          <label class="block max-w-xs space-y-2"><span class="text-sm font-medium">排序</span><Input v-model.number="providerForm.sortOrder" type="number" /></label>
          <DialogFooter>
            <Button variant="outline" :disabled="isSavingProvider" @click="isProviderFormOpen = false">取消</Button>
            <Button :disabled="isSavingProvider" @click="saveProvider">
              <Save class="h-4 w-4" />保存提供商
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="isModelFormOpen">
      <DialogContent class="sm:max-w-3xl">
        <DialogHeader>
          <div class="flex flex-wrap items-start justify-between gap-3 pr-8">
            <div>
              <DialogTitle>{{ modelFormTitle }}</DialogTitle>
              <DialogDescription class="mt-2">模型保存后会刷新发布 API 服务时可选的模型目录。</DialogDescription>
            </div>
            <Button size="sm" variant="outline" @click="resetModelForm"><RotateCcw class="h-4 w-4" />重置</Button>
          </div>
        </DialogHeader>

        <div class="space-y-4">
          <label class="space-y-2">
            <span class="text-sm font-medium">API 提供商</span>
            <Select v-model="modelForm.providerId" :disabled="Boolean(editingModel?.identityLocked)">
              <SelectTrigger><SelectValue placeholder="选择提供商" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="provider in activeProviders" :key="provider.id" :value="provider.id">
                  {{ provider.displayName }} · {{ providerLabelMap[provider.providerCategory] ?? provider.providerCategory }}
                </SelectItem>
              </SelectContent>
            </Select>
          </label>

          <label class="block max-w-md space-y-2">
            <span class="text-sm font-medium">模型标识</span>
            <Input v-model="modelForm.modelKey" :disabled="Boolean(editingModel?.identityLocked)" placeholder="gpt-4.1-mini" />
            <span v-if="editingModel?.identityLocked" class="block text-xs text-muted-foreground">{{ editingModel.identityLockReason }}</span>
          </label>

          <div class="space-y-2">
            <span class="text-sm font-medium">能力</span>
            <div class="grid gap-2 sm:grid-cols-3">
              <label v-for="item in apiModelCapabilities" :key="item.value" class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-2 text-sm">
                <Checkbox
                  :model-value="modelForm.capabilities.includes(item.value)"
                  @update:model-value="value => setCapability(item.value, Boolean(value))"
                />
                <span>{{ item.label }}</span>
              </label>
            </div>
          </div>

          <div class="rounded-md border border-border bg-muted/20 p-3">
            <div class="mb-3 text-sm font-medium">官网公开价格（每百万 tokens）</div>
            <div class="grid gap-3 sm:grid-cols-3">
              <label class="space-y-2">
                <span class="text-sm text-muted-foreground">输入价</span>
                <Input v-model="modelForm.inputTokenPrice" inputmode="decimal" placeholder="0.150000" />
              </label>
              <label class="space-y-2">
                <span class="text-sm text-muted-foreground">缓存输入价</span>
                <Input v-model="modelForm.cachedInputTokenPrice" inputmode="decimal" placeholder="0.075000" />
              </label>
              <label class="space-y-2">
                <span class="text-sm text-muted-foreground">输出价</span>
                <Input v-model="modelForm.outputTokenPrice" inputmode="decimal" placeholder="0.600000" />
              </label>
            </div>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <label class="space-y-2">
                <span class="text-sm text-muted-foreground">来源 URL / 说明</span>
                <Input v-model="modelForm.sourceUrl" placeholder="https://example.com/pricing" />
              </label>
              <label class="space-y-2">
                <span class="text-sm text-muted-foreground">来源版本</span>
                <Input v-model="modelForm.sourceVersion" placeholder="2026-06-29" />
              </label>
            </div>
          </div>

          <label class="block max-w-xs space-y-2"><span class="text-sm font-medium">排序</span><Input v-model.number="modelForm.sortOrder" type="number" /></label>

          <DialogFooter>
            <Button variant="outline" :disabled="isSavingModel" @click="isModelFormOpen = false">取消</Button>
            <Button :disabled="isSavingModel" @click="saveModel">
              <Save class="h-4 w-4" />保存模型
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>

    <AdminApiModelSyncDialog
      v-model:open="isSyncDialogOpen"
      :providers="providers"
    />

    <Dialog :open="Boolean(lifecycleTarget)" @update:open="open => { if (!open) lifecycleTarget = null }">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ lifecycleTarget ? lifecycleActionLabel(lifecycleTarget.action) : '目录状态操作' }}</DialogTitle>
          <DialogDescription>该操作会立即影响新发布和新订单资格，历史正式订单仍按快照继续处理。</DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <label v-if="lifecycleTarget?.action === 'unblock'" class="block space-y-2">
            <span class="text-sm font-medium">解除后状态</span>
            <Select v-model="unblockTargetStatus">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="active">恢复可用</SelectItem><SelectItem value="deprecated">保持退役</SelectItem></SelectContent>
            </Select>
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">操作原因</span>
            <Textarea v-model="lifecycleReason" maxlength="500" class="min-h-24" placeholder="说明本次目录治理依据，至少 2 个字符。" />
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="lifecycleTarget = null">取消</Button>
          <Button :variant="lifecycleTarget?.action === 'block' ? 'destructive' : 'default'" :disabled="providerLifecycleMutation.isPending.value || modelLifecycleMutation.isPending.value" @click="applyLifecycle">确认执行</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
