<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { AlertTriangle, Building2, FilePenLine, Plus, RotateCcw, Save, ToggleLeft, ToggleRight } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PageTitle from '@/components/market/PageTitle.vue'
import StatCard from '@/components/market/StatCard.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import { Badge } from '@/components/ui/badge'
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
import {
  useAdminAPIModelProviders,
  useAdminAPIModels,
  useCreateAPIModel,
  useCreateAPIModelProvider,
  useSetAPIModelActive,
  useSetAPIModelProviderActive,
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
  type ApiModelProviderCategory,
  type ApiModelProviderInput,
} from '@/types/apiModelCatalog'

type StatusFilter = '全部' | '启用' | '停用'

const statusFilter = ref<StatusFilter>('全部')
const editingModelId = ref('')
const editingProviderId = ref('')
const isModelFormOpen = ref(false)
const isProviderFormOpen = ref(false)

const providersQuery = useAdminAPIModelProviders()
const modelsQuery = useAdminAPIModels()
const createProviderMutation = useCreateAPIModelProvider()
const updateProviderMutation = useUpdateAPIModelProvider()
const providerActiveMutation = useSetAPIModelProviderActive()
const createModelMutation = useCreateAPIModel()
const updateModelMutation = useUpdateAPIModel()
const modelActiveMutation = useSetAPIModelActive()

const providers = computed(() => providersQuery.data.value ?? [])
const activeProviders = computed(() => providers.value.filter(item => item.active))
const rows = computed(() => modelsQuery.data.value ?? [])
const providerForm = reactive<ApiModelProviderInput>(emptyProviderForm())
const modelForm = reactive<ApiModelInput>(emptyModelForm())
const visibleRows = computed(() => rows.value.filter(matchesStatusFilter))
const activeCount = computed(() => rows.value.filter(item => item.active && item.providerActive).length)
const inactiveCount = computed(() => rows.value.length - activeCount.value)
const providerCount = computed(() => providers.value.length)
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

const providerLabelMap = Object.fromEntries(apiModelProviderCategories.map(item => [item.value, item.label])) as Record<ApiModelProviderCategory, string>
const capabilityLabelMap = Object.fromEntries(apiModelCapabilities.map(item => [item.value, item.label])) as Record<ApiModelCapability, string>

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
    active: true,
    sortOrder: nextProviderSortOrder(),
  }
}

function emptyModelForm(): ApiModelInput {
  return {
    providerId: activeProviders.value[0]?.id ?? '',
    modelKey: '',
    displayName: '',
    capabilities: ['chat'],
    inputTokenPrice: '',
    cachedInputTokenPrice: '',
    outputTokenPrice: '',
    sourceUrl: '',
    sourceVersion: '',
    active: true,
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
    active: provider.active,
    sortOrder: provider.sortOrder,
  }
}

function inputFromModel(model: AdminApiModel): ApiModelInput {
  return {
    providerId: model.providerId,
    modelKey: model.modelKey,
    displayName: model.displayName,
    capabilities: [...model.capabilities],
    inputTokenPrice: model.inputPricePerMillion ?? '',
    cachedInputTokenPrice: model.cachedInputPricePerMillion ?? '',
    outputTokenPrice: model.outputPricePerMillion ?? '',
    sourceUrl: model.currentPriceSourceUrl ?? '',
    sourceVersion: model.currentPriceSourceVersion ?? '',
    active: model.active,
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
  if (!modelForm.displayName.trim()) return '请填写展示名。'
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

async function setProviderActive(provider: AdminApiModelProvider, active: boolean) {
  try {
    await providerActiveMutation.mutateAsync({ id: provider.id, active })
    toast.success(active ? 'API 提供商已启用。' : 'API 提供商已停用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提供商状态更新失败')
  }
}

async function setModelActive(model: AdminApiModel, active: boolean) {
  try {
    await modelActiveMutation.mutateAsync({ id: model.id, active })
    toast.success(active ? 'API 模型已启用。' : 'API 模型已停用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '模型状态更新失败')
  }
}

function toggleCapability(capability: ApiModelCapability) {
  modelForm.capabilities = modelForm.capabilities.includes(capability)
    ? modelForm.capabilities.filter(item => item !== capability)
    : [...modelForm.capabilities, capability]
}

function matchesStatusFilter(item: AdminApiModel) {
  if (statusFilter.value === '全部') return true
  const effectiveActive = item.active && item.providerActive
  return statusFilter.value === '启用' ? effectiveActive : !effectiveActive
}

function nextProviderSortOrder() {
  const max = Math.max(0, ...providers.value.map(item => item.sortOrder))
  return max + 10
}

function nextModelSortOrder() {
  const max = Math.max(0, ...rows.value.map(item => item.sortOrder))
  return max + 10
}

function priceText(model: AdminApiModel) {
  const input = model.inputPricePerMillion || '-'
  const cached = model.cachedInputPricePerMillion || '-'
  const output = model.outputPricePerMillion || '-'
  return `输入 ${input} · 缓存 ${cached} · 输出 ${output}`
}

function capabilityText(model: AdminApiModel) {
  return model.capabilities.map(item => capabilityLabelMap[item] ?? item).join(' / ')
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle
      title="API 模型目录"
      description="维护 API 提供商、具体模型、能力标签和官网公开价格版本。"
    />

    <div class="grid gap-3 md:grid-cols-4">
      <StatCard label="全部模型" :value="rows.length" hint="含停用模型" />
      <StatCard label="可发布模型" :value="activeCount" :hint="`不可用 ${inactiveCount}`" accent />
      <StatCard label="提供商" :value="providerCount" :hint="`启用 ${activeProviders.length}`" />
      <StatCard label="当前筛选" :value="visibleRows.length" :hint="statusFilter" />
    </div>

    <section class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold">API 提供商</h2>
          <p class="text-sm text-muted-foreground">模型从这里选择提供商，分类和展示名由提供商目录统一维护。</p>
        </div>
        <Button size="sm" @click="openCreateProvider">
          <Building2 class="h-4 w-4" />新建提供商
        </Button>
      </div>

      <Card class="overflow-hidden p-0">
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[760px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">提供商</th>
                <th class="px-3 py-2 font-medium">分类</th>
                <th class="px-3 py-2 font-medium">状态</th>
                <th class="px-3 py-2 font-medium">排序</th>
                <th class="px-3 py-2 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="provider in providers" :key="provider.id" class="border-b border-border/70 last:border-0">
                <td class="px-3 py-3">
                  <div class="font-medium">{{ provider.displayName }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">{{ provider.code }}</div>
                </td>
                <td class="px-3 py-3"><Badge variant="model">{{ providerLabelMap[provider.providerCategory] }}</Badge></td>
                <td class="px-3 py-3"><Badge :variant="provider.active ? 'verified' : 'secondary'">{{ provider.active ? '启用' : '停用' }}</Badge></td>
                <td class="px-3 py-3 text-sm text-muted-foreground">{{ provider.sortOrder }}</td>
                <td class="px-3 py-3">
                  <div class="flex flex-wrap justify-end gap-1.5">
                    <Button size="sm" variant="outline" @click="openEditProvider(provider)">
                      <FilePenLine class="h-4 w-4" />编辑
                    </Button>
                    <Button size="sm" variant="outline" :disabled="providerActiveMutation.isPending.value" @click="setProviderActive(provider, !provider.active)">
                      <component :is="provider.active ? ToggleLeft : ToggleRight" class="h-4 w-4" />
                      {{ provider.active ? '停用' : '启用' }}
                    </Button>
                  </div>
                </td>
              </tr>
              <tr v-if="providers.length === 0">
                <td colspan="5" class="px-3 py-8 text-center text-sm text-muted-foreground">暂无 API 提供商。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>

    <section class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <StatusTabs v-model="statusFilter" :items="['全部', '启用', '停用']" class="mb-0" />
        <Button size="sm" :disabled="activeProviders.length === 0" @click="openCreateModel">
          <Plus class="h-4 w-4" />新建模型
        </Button>
      </div>

      <div v-if="isLoading" class="rounded-md border border-border p-8 text-center text-sm text-muted-foreground">
        API 模型目录加载中...
      </div>

      <Card v-else-if="hasError" class="border-destructive/30 p-5">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-destructive/10 text-destructive">
            <AlertTriangle class="h-5 w-5" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="font-semibold">API 模型目录读取失败</h2>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ errorMessage }}</p>
            <Button class="mt-4" size="sm" :disabled="modelsQuery.isFetching.value || providersQuery.isFetching.value" @click="providersQuery.refetch(); modelsQuery.refetch()">重新读取</Button>
          </div>
        </div>
      </Card>

      <Card v-else class="overflow-hidden p-0">
        <div class="overflow-x-auto">
          <table class="c2c-table w-full min-w-[980px] text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-3 py-2 font-medium">模型</th>
                <th class="px-3 py-2 font-medium">提供商</th>
                <th class="px-3 py-2 font-medium">能力</th>
                <th class="px-3 py-2 font-medium">官网公开价格（每百万 tokens）</th>
                <th class="px-3 py-2 font-medium">来源版本</th>
                <th class="px-3 py-2 font-medium">状态</th>
                <th class="px-3 py-2 font-medium">排序</th>
                <th class="px-3 py-2 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="model in visibleRows" :key="model.id" class="border-b border-border/70 last:border-0">
                <td class="max-w-[260px] px-3 py-3">
                  <div class="font-medium">{{ model.displayName }}</div>
                  <div class="mt-1 truncate text-xs text-muted-foreground">{{ model.modelKey }}</div>
                </td>
                <td class="px-3 py-3">
                  <div class="font-medium">{{ model.provider }}</div>
                  <div class="mt-1 flex flex-wrap gap-1">
                    <Badge variant="model">{{ providerLabelMap[model.providerCategory] }}</Badge>
                    <Badge v-if="!model.providerActive" variant="secondary">提供商停用</Badge>
                  </div>
                </td>
                <td class="max-w-[180px] px-3 py-3 text-xs text-muted-foreground">{{ capabilityText(model) }}</td>
                <td class="px-3 py-3 text-xs text-muted-foreground">{{ priceText(model) }}</td>
                <td class="max-w-[180px] px-3 py-3 text-xs text-muted-foreground">
                  <div class="truncate">{{ model.currentPriceSourceVersion || '-' }}</div>
                  <div class="truncate">{{ model.currentPriceSourceUrl || '' }}</div>
                </td>
                <td class="px-3 py-3">
                  <Badge :variant="model.active && model.providerActive ? 'verified' : 'secondary'">{{ model.active && model.providerActive ? '启用' : '停用' }}</Badge>
                </td>
                <td class="px-3 py-3 text-sm text-muted-foreground">{{ model.sortOrder }}</td>
                <td class="px-3 py-3">
                  <div class="flex flex-wrap justify-end gap-1.5">
                    <Button size="sm" variant="outline" @click="openEditModel(model)">
                      <FilePenLine class="h-4 w-4" />编辑
                    </Button>
                    <Button size="sm" variant="outline" :disabled="modelActiveMutation.isPending.value" @click="setModelActive(model, !model.active)">
                      <component :is="model.active ? ToggleLeft : ToggleRight" class="h-4 w-4" />
                      {{ model.active ? '停用' : '启用' }}
                    </Button>
                  </div>
                </td>
              </tr>
              <tr v-if="visibleRows.length === 0">
                <td colspan="8" class="px-3 py-10 text-center text-sm text-muted-foreground">
                  当前筛选下暂无 API 模型。
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>

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
            <Select v-model="providerForm.providerCategory">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in apiModelProviderCategories" :key="item.value" :value="item.value">{{ item.label }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">Code</span>
              <Input v-model="providerForm.code" placeholder="openai" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">展示名</span>
              <Input v-model="providerForm.displayName" placeholder="OpenAI" />
            </label>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">排序</span>
              <Input v-model.number="providerForm.sortOrder" type="number" />
            </label>
            <label class="flex items-end gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
              <input v-model="providerForm.active" type="checkbox" class="mb-1 h-4 w-4 accent-primary" />
              <span>启用提供商</span>
            </label>
          </div>
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
            <Select v-model="modelForm.providerId">
              <SelectTrigger><SelectValue placeholder="选择提供商" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="provider in activeProviders" :key="provider.id" :value="provider.id">
                  {{ provider.displayName }} · {{ providerLabelMap[provider.providerCategory] }}
                </SelectItem>
              </SelectContent>
            </Select>
          </label>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">模型标识</span>
              <Input v-model="modelForm.modelKey" placeholder="gpt-4o-mini" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">展示名</span>
              <Input v-model="modelForm.displayName" placeholder="GPT-4o mini" />
            </label>
          </div>

          <div class="space-y-2">
            <span class="text-sm font-medium">能力</span>
            <div class="grid gap-2 sm:grid-cols-3">
              <label v-for="item in apiModelCapabilities" :key="item.value" class="flex items-center gap-2 rounded-md border border-border bg-muted/30 p-2 text-sm">
                <input :checked="modelForm.capabilities.includes(item.value)" type="checkbox" class="h-4 w-4 accent-primary" @change="toggleCapability(item.value)" />
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

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">排序</span>
              <Input v-model.number="modelForm.sortOrder" type="number" />
            </label>
            <label class="flex items-end gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
              <input v-model="modelForm.active" type="checkbox" class="mb-1 h-4 w-4 accent-primary" />
              <span>启用模型</span>
            </label>
          </div>

          <DialogFooter>
            <Button variant="outline" :disabled="isSavingModel" @click="isModelFormOpen = false">取消</Button>
            <Button :disabled="isSavingModel" @click="saveModel">
              <Save class="h-4 w-4" />保存模型
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
