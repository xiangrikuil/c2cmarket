<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  FilePenLine,
  FolderPlus,
  ImageIcon,
  Plus,
  RotateCcw,
  Save,
  ToggleLeft,
  ToggleRight,
  Trash2,
  Upload,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CompactStats from '@/components/market/CompactStats.vue'
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
import { Textarea } from '@/components/ui/textarea'
import { productCategoryIconAccept, readProductCategoryIcon, validateProductCategoryIconFile } from '@/lib/productCategoryIcon'
import {
  applyCatalogBusinessStatus,
  catalogBusinessStatusOptions,
  getCatalogBusinessStatus,
  getCatalogBusinessStatusMeta,
  type CatalogBusinessStatus,
} from '@/lib/productCatalogStatus'
import {
  useAdminProductCategories,
  useAdminProductPlans,
  useCreateProductCategory,
  useCreateProductPlan,
  useSetProductCategoryActive,
  useSetProductPlanActive,
  useUpdateProductCategory,
  useUpdateProductPlan,
} from '@/queries/useProductCatalogQueries'
import type { ProductAccessMode, ProductCategory, ProductCategoryInput, ProductPlan, ProductPlanInput } from '@/types/productCatalog'

type StatusFilter = '全部' | '启用' | '停用'
type CategoryFormMode = 'create' | 'edit'

type CategoryGroup = {
  category: ProductCategory
  plans: ProductPlan[]
  visiblePlans: ProductPlan[]
  activePlanCount: number
  inactivePlanCount: number
}

const statusFilter = ref<StatusFilter>('全部')
const expandedCategoryIds = ref<string[]>([])
const editingPlanId = ref('')
const editingCategoryId = ref('')
const isPlanFormOpen = ref(false)
const isCategoryFormOpen = ref(false)
const showAdvancedPlanFields = ref(false)
const selectedBusinessStatus = ref<CatalogBusinessStatus>('publishable')
const categoryFormMode = ref<CategoryFormMode>('create')
const categoryIconInput = ref<HTMLInputElement | null>(null)
const planForm = reactive<ProductPlanInput>({
  categoryId: '',
  providerCode: 'other',
  slug: '',
  displayName: '',
  description: '',
  publishPolicy: 'allowed',
  accessMode: 'owner_managed_access',
  providerPolicyStatus: 'unknown',
  riskLevel: 'normal',
  riskAckRequired: false,
  riskNoticeCode: '',
  policyNote: '',
  quotaLabel: '额度',
  quotaUnit: 'USD',
  quotaPeriod: 'monthly',
  active: true,
  allowCustomVariant: false,
  sortOrder: 100,
})
const categoryForm = reactive<ProductCategoryInput>({
  code: '',
  displayName: '',
  iconDataUrl: '',
  sortOrder: 10,
  active: true,
})

const categoriesQuery = useAdminProductCategories()
const plansQuery = useAdminProductPlans()
const createPlanMutation = useCreateProductPlan()
const updatePlanMutation = useUpdateProductPlan()
const activePlanMutation = useSetProductPlanActive()
const createCategoryMutation = useCreateProductCategory()
const updateCategoryMutation = useUpdateProductCategory()
const activeCategoryMutation = useSetProductCategoryActive()

const categories = categoriesQuery.data
const plans = plansQuery.data
const categoryRows = computed(() => categories.value ?? [])
const rows = computed(() => plans.value ?? [])
const isLoading = computed(() => categoriesQuery.isLoading.value || plansQuery.isLoading.value)
const hasCatalogError = computed(() => categoriesQuery.isError.value || plansQuery.isError.value)
const catalogErrorMessage = computed(() => {
  const error = categoriesQuery.error.value ?? plansQuery.error.value
  return error instanceof Error ? error.message : '套餐目录读取失败，请确认当前账号有管理权限并重试。'
})
const activeCategoryCount = computed(() => categoryRows.value.filter(item => item.active).length)
const activePlanCount = computed(() => rows.value.filter(item => item.active).length)
const inactivePlanCount = computed(() => rows.value.filter(item => !item.active).length)
const restrictedPlanCount = computed(() => rows.value.filter(item => item.publishPolicy !== 'allowed').length)
const uncategorizedPlans = computed(() => rows.value.filter(plan => !categoryRows.value.some(category => category.id === plan.categoryId)))
const categoryGroups = computed<CategoryGroup[]>(() => categoryRows.value.map(category => {
  const plansInCategory = rows.value
    .filter(plan => plan.categoryId === category.id)
    .sort((left, right) => left.sortOrder - right.sortOrder || left.displayName.localeCompare(right.displayName))
  return {
    category,
    plans: plansInCategory,
    visiblePlans: plansInCategory.filter(matchesStatusFilter),
    activePlanCount: plansInCategory.filter(item => item.active).length,
    inactivePlanCount: plansInCategory.filter(item => !item.active).length,
  }
}))
const visibleCategoryGroups = computed(() => categoryGroups.value.filter(group => group.visiblePlans.length > 0 || matchesStatusFilter(group.category)))
const planSaving = computed(() => createPlanMutation.isPending.value || updatePlanMutation.isPending.value)
const categorySaving = computed(() => createCategoryMutation.isPending.value || updateCategoryMutation.isPending.value)
const planFormTitle = computed(() => editingPlanId.value ? '编辑套餐' : '新建套餐')
const planFormDescription = computed(() => editingPlanId.value
  ? '调整套餐基础信息和业务状态，保存后会刷新用户端可选目录。'
  : '在选定分类下创建套餐，slug 会按展示名自动生成。')
const categoryFormTitle = computed(() => categoryFormMode.value === 'edit' ? '编辑分类' : '新建分类')
const editingPlan = computed(() => rows.value.find(item => item.id === editingPlanId.value) ?? null)
const editingCategory = computed(() => categoryRows.value.find(item => item.id === editingCategoryId.value) ?? null)
const selectedPlanCategory = computed(() => categoryRows.value.find(item => item.id === planForm.categoryId) ?? categoryRows.value[0])
const visibleCategoriesForPlanForm = computed(() => {
  if (editingPlan.value) return categoryRows.value
  return categoryRows.value.filter(item => item.active)
})

const accessModeLabels: Record<ProductAccessMode, string> = {
  personal_account_cost_share: '个人订阅费用分摊',
  provider_member_invitation: '成员邀请',
  owner_managed_access: '车主管理访问',
  other_off_platform: '其他站外方式',
  unsupported: '不支持',
}

watch(categoryRows, value => {
  if (value.length === 0) return
  if (expandedCategoryIds.value.length === 0) {
    expandedCategoryIds.value = value.map(item => item.id)
  }
  if (!planForm.categoryId || !value.some(item => item.id === planForm.categoryId)) {
    planForm.categoryId = value.find(item => item.active)?.id ?? value[0].id
  }
}, { immediate: true })

watch(() => planForm.displayName, value => {
  if (editingPlanId.value || showAdvancedPlanFields.value) return
  planForm.slug = slugify(value)
})

watch(() => planForm.categoryId, value => {
  if (showAdvancedPlanFields.value) return
  const category = categoryRows.value.find(item => item.id === value)
  if (category) planForm.providerCode = category.code
})

watch(() => categoryForm.displayName, value => {
  if (categoryFormMode.value === 'edit') return
  categoryForm.code = slugify(value)
})

function emptyCategoryForm(): ProductCategoryInput {
  return {
    code: '',
    displayName: '',
    iconDataUrl: '',
    sortOrder: nextCategorySortOrder(),
    active: true,
  }
}

function emptyPlanForm(categoryId?: string): ProductPlanInput {
  const category = categoryRows.value.find(item => item.id === categoryId)
    ?? categoryRows.value.find(item => item.active)
    ?? categoryRows.value[0]
  return {
    categoryId: category?.id ?? '',
    providerCode: category?.code ?? 'other',
    slug: '',
    displayName: '',
    description: '',
    publishPolicy: 'allowed',
    accessMode: 'owner_managed_access',
    providerPolicyStatus: 'unknown',
    riskLevel: 'normal',
    riskAckRequired: false,
    riskNoticeCode: '',
    policyNote: '',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    active: true,
    allowCustomVariant: false,
    sortOrder: nextPlanSortOrder(category?.id),
  }
}

function fillPlanForm(input: ProductPlanInput) {
  Object.assign(planForm, input)
  selectedBusinessStatus.value = getCatalogBusinessStatus(input)
}

function fillCategoryForm(input: ProductCategoryInput) {
  Object.assign(categoryForm, input)
}

function inputFromPlan(plan: ProductPlan): ProductPlanInput {
  return {
    categoryId: plan.categoryId,
    providerCode: plan.providerCode,
    slug: plan.slug,
    displayName: plan.displayName,
    description: plan.description,
    publishPolicy: plan.publishPolicy,
    accessMode: plan.accessMode,
    providerPolicyStatus: plan.providerPolicyStatus,
    riskLevel: plan.riskLevel,
    riskAckRequired: plan.riskAckRequired,
    riskNoticeCode: plan.riskNoticeCode ?? '',
    policyNote: plan.policyNote,
    quotaLabel: plan.quotaLabel,
    quotaUnit: plan.quotaUnit,
    quotaPeriod: plan.quotaPeriod,
    active: plan.active,
    allowCustomVariant: plan.allowCustomVariant,
    sortOrder: plan.sortOrder,
  }
}

function inputFromCategory(category: ProductCategory): ProductCategoryInput {
  return {
    code: category.code,
    displayName: category.displayName,
    iconDataUrl: category.iconDataUrl,
    sortOrder: category.sortOrder,
    active: category.active,
  }
}

function openNewCategory() {
  categoryFormMode.value = 'create'
  editingCategoryId.value = ''
  fillCategoryForm(emptyCategoryForm())
  isCategoryFormOpen.value = true
}

function openEditCategory(category: ProductCategory) {
  categoryFormMode.value = 'edit'
  editingCategoryId.value = category.id
  fillCategoryForm(inputFromCategory(category))
  isCategoryFormOpen.value = true
}

function openNewPlan(category?: ProductCategory) {
  editingPlanId.value = ''
  showAdvancedPlanFields.value = false
  fillPlanForm(emptyPlanForm(category?.id))
  if (category) expandedCategoryIds.value = uniqueIds([...expandedCategoryIds.value, category.id])
  isPlanFormOpen.value = true
}

function openEditPlan(plan: ProductPlan) {
  editingPlanId.value = plan.id
  showAdvancedPlanFields.value = false
  fillPlanForm(inputFromPlan(plan))
  expandedCategoryIds.value = uniqueIds([...expandedCategoryIds.value, plan.categoryId])
  isPlanFormOpen.value = true
}

function resetPlanForm() {
  if (editingPlan.value) {
    fillPlanForm(inputFromPlan(editingPlan.value))
    return
  }
  fillPlanForm(emptyPlanForm(selectedPlanCategory.value?.id))
}

function resetCategoryForm() {
  if (editingCategory.value) {
    fillCategoryForm(inputFromCategory(editingCategory.value))
    return
  }
  fillCategoryForm(emptyCategoryForm())
}

async function selectCategoryIcon(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const validationError = validateProductCategoryIconFile(file)
  if (validationError) {
    toast.error(validationError)
    return
  }
  try {
    categoryForm.iconDataUrl = await readProductCategoryIcon(file)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '分类图标读取失败。')
  }
}

function removeCategoryIcon() {
  categoryForm.iconDataUrl = ''
}

async function refetchCatalog() {
  await Promise.all([
    categoriesQuery.refetch(),
    plansQuery.refetch(),
  ])
}

function validateCategoryForm() {
  if (!categoryForm.displayName.trim()) return '请填写分类名称。'
  if (!categoryForm.code.trim()) return '请填写分类 code。'
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(categoryForm.code.trim())) return '分类 code 只允许小写字母、数字和短横线。'
  return ''
}

function validatePlanForm() {
  if (!planForm.categoryId) return '请选择分类。'
  if (!planForm.displayName.trim()) return '请填写套餐名称。'
  if (!planForm.slug.trim()) return '请填写 slug。'
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(planForm.slug.trim())) return 'slug 只允许小写字母、数字和短横线。'
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(planForm.providerCode.trim())) return 'Provider code 只允许小写字母、数字和短横线。'
  if (!planForm.quotaLabel.trim()) return '请填写额度名称。'
  if (!planForm.quotaUnit.trim()) return '请填写额度单位。'
  return ''
}

async function saveCategory() {
  const error = validateCategoryForm()
  if (error) {
    toast.warning(error)
    return
  }
  try {
    const input = { ...categoryForm }
    if (categoryFormMode.value === 'edit' && editingCategoryId.value) {
      await updateCategoryMutation.mutateAsync({ id: editingCategoryId.value, input })
      toast.success('分类已更新。')
    } else {
      const created = await createCategoryMutation.mutateAsync(input)
      expandedCategoryIds.value = uniqueIds([...expandedCategoryIds.value, created.id])
      toast.success('分类已创建。')
    }
    isCategoryFormOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '保存分类失败')
  }
}

async function savePlan() {
  const error = validatePlanForm()
  if (error) {
    toast.warning(error)
    return
  }
  const input = applyCatalogBusinessStatus({ ...planForm }, selectedBusinessStatus.value)
  try {
    if (editingPlanId.value) {
      await updatePlanMutation.mutateAsync({ id: editingPlanId.value, input })
      toast.success('套餐已更新。')
    } else {
      const created = await createPlanMutation.mutateAsync(input)
      editingPlanId.value = created.id
      expandedCategoryIds.value = uniqueIds([...expandedCategoryIds.value, created.categoryId])
      toast.success('套餐已创建。')
    }
    isPlanFormOpen.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '保存套餐失败')
  }
}

async function setCategoryActive(category: ProductCategory, active: boolean) {
  if (!active && !window.confirm(`停用分类“${category.displayName}”会隐藏其公开套餐并阻止新发布，已有交易记录不受影响。确认继续？`)) return
  try {
    await activeCategoryMutation.mutateAsync({ id: category.id, active })
    toast.success(active ? '分类已启用。' : '分类已停用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '分类状态更新失败')
  }
}

async function setPlanActive(plan: ProductPlan, active: boolean) {
  if (!active && !window.confirm(`停用套餐“${plan.displayName}”会阻止新的车源发布并隐藏公开目录入口，已有申请继续使用快照。确认继续？`)) return
  try {
    await activePlanMutation.mutateAsync({ id: plan.id, active })
    toast.success(active ? '套餐已启用。' : '套餐已停用。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '套餐状态更新失败')
  }
}

function toggleCategory(categoryId: string) {
  expandedCategoryIds.value = isCategoryExpanded(categoryId)
    ? expandedCategoryIds.value.filter(id => id !== categoryId)
    : [...expandedCategoryIds.value, categoryId]
}

function isCategoryExpanded(categoryId: string) {
  return expandedCategoryIds.value.includes(categoryId)
}

function matchesStatusFilter(item: { active: boolean }) {
  if (statusFilter.value === '全部') return true
  return statusFilter.value === '启用' ? item.active : !item.active
}

function businessStatusLabel(plan: ProductPlan) {
  return getCatalogBusinessStatusMeta(getCatalogBusinessStatus(plan)).label
}

function businessStatusVariant(plan: ProductPlan) {
  return getCatalogBusinessStatusMeta(getCatalogBusinessStatus(plan)).badgeVariant
}

function categoryStatusText(category: ProductCategory) {
  return category.active ? '启用' : '停用'
}

function quotaText(plan: ProductPlan) {
  return `每月${plan.quotaLabel} · ${plan.quotaUnit}`
}

function nextCategorySortOrder() {
  const max = Math.max(0, ...categoryRows.value.map(item => item.sortOrder))
  return max + 10
}

function nextPlanSortOrder(categoryId?: string) {
  const siblings = categoryId ? rows.value.filter(item => item.categoryId === categoryId) : rows.value
  const max = Math.max(0, ...siblings.map(item => item.sortOrder))
  return max + 10
}

function slugify(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .replace(/-{2,}/g, '-')
}

function uniqueIds(ids: string[]) {
  return Array.from(new Set(ids))
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle
      title="套餐目录"
      description="按分类维护可发布套餐，主界面只保留业务状态和发布所需基础字段。"
    />

    <CompactStats :items="[{ label: '全部分类', value: categoryRows.length, hint: `启用 ${activeCategoryCount}` }, { label: '全部套餐', value: rows.length, hint: '含停用套餐' }, { label: '启用套餐', value: activePlanCount, hint: `停用 ${inactivePlanCount}` }, { label: '限制展示', value: restrictedPlanCount, hint: '仅信息或禁止发布' }]" :loading="isLoading" />

    <section class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <StatusTabs v-model="statusFilter" :items="['全部', '启用', '停用']" class="mb-0" />
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" @click="openNewCategory">
            <FolderPlus class="h-4 w-4" />新建分类
          </Button>
          <Button size="sm" @click="openNewPlan()">
            <Plus class="h-4 w-4" />新建套餐
          </Button>
        </div>
      </div>

      <div v-if="isLoading" class="rounded-md border border-border p-8 text-center text-sm text-muted-foreground">
        套餐目录加载中...
      </div>

      <Card v-else-if="hasCatalogError" class="border-destructive/30 p-5">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-destructive/10 text-destructive">
            <AlertTriangle class="h-5 w-5" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="font-semibold">套餐目录读取失败</h2>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ catalogErrorMessage }}</p>
            <div class="mt-4 flex flex-wrap gap-2">
              <Button size="sm" :disabled="plansQuery.isFetching.value || categoriesQuery.isFetching.value" @click="refetchCatalog">
                重新读取
              </Button>
              <Button size="sm" variant="outline" @click="openNewCategory">
                <FolderPlus class="h-4 w-4" />新建分类
              </Button>
            </div>
          </div>
        </div>
      </Card>

      <div v-else class="space-y-3">
        <Card v-for="group in visibleCategoryGroups" :key="group.category.id" class="overflow-hidden p-0">
          <div
            class="grid gap-3 border-b border-border bg-muted/25 p-3 md:grid-cols-[minmax(0,1.6fr)_auto_auto] md:items-center"
            :class="group.category.active ? '' : 'opacity-75'"
          >
            <button class="flex min-w-0 items-center gap-3 text-left" @click="toggleCategory(group.category.id)">
              <span class="grid h-8 w-8 shrink-0 place-items-center rounded-md border bg-background text-muted-foreground">
                <component :is="isCategoryExpanded(group.category.id) ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </span>
              <span class="grid h-9 w-9 shrink-0 place-items-center overflow-hidden rounded-lg border bg-background text-muted-foreground">
                <img v-if="group.category.iconDataUrl" :src="group.category.iconDataUrl" :alt="`${group.category.displayName} 图标`" class="h-full w-full object-contain" />
                <ImageIcon v-else class="h-4 w-4" />
              </span>
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-2">
                  <span class="truncate font-semibold">{{ group.category.displayName }}</span>
                  <Badge :variant="group.category.active ? 'verified' : 'secondary'">{{ categoryStatusText(group.category) }}</Badge>
                </span>
                <span class="mt-1 block truncate text-xs text-muted-foreground">{{ group.category.code }}</span>
              </span>
            </button>
            <div class="flex flex-wrap gap-2 text-xs text-muted-foreground md:justify-end">
              <span class="rounded-md border border-border bg-background px-2 py-1">启用 {{ group.activePlanCount }}</span>
              <span class="rounded-md border border-border bg-background px-2 py-1">停用 {{ group.inactivePlanCount }}</span>
              <span class="rounded-md border border-border bg-background px-2 py-1">排序 {{ group.category.sortOrder }}</span>
            </div>
            <div class="flex flex-wrap gap-1.5 md:justify-end">
              <Button size="sm" variant="outline" @click="openNewPlan(group.category)">
                <Plus class="h-4 w-4" />套餐
              </Button>
              <Button size="sm" variant="outline" @click="openEditCategory(group.category)">
                <FilePenLine class="h-4 w-4" />分类
              </Button>
              <Button
                size="sm"
                variant="outline"
                :disabled="activeCategoryMutation.isPending.value"
                @click="setCategoryActive(group.category, !group.category.active)"
              >
                <component :is="group.category.active ? ToggleLeft : ToggleRight" class="h-4 w-4" />
                {{ group.category.active ? '停用' : '启用' }}
              </Button>
            </div>
          </div>

          <div v-if="isCategoryExpanded(group.category.id)" class="overflow-x-auto">
            <table class="c2c-table w-full min-w-[760px] text-sm">
              <thead>
                <tr class="border-b border-border text-left text-xs text-muted-foreground">
                  <th class="px-3 py-2 font-medium">套餐</th>
                  <th class="px-3 py-2 font-medium">业务状态</th>
                  <th class="px-3 py-2 font-medium">额度配置</th>
                  <th class="px-3 py-2 font-medium">接入方式</th>
                  <th class="px-3 py-2 font-medium">状态</th>
                  <th class="px-3 py-2 font-medium">排序</th>
                  <th class="px-3 py-2 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="plan in group.visiblePlans"
                  :key="plan.id"
                  class="border-b border-border/70 last:border-0"
                  :class="plan.id === editingPlanId ? 'bg-accent/60' : ''"
                >
                  <td class="max-w-[260px] px-3 py-3">
                    <div class="font-medium">{{ plan.displayName }}</div>
                    <div class="mt-1 truncate text-xs text-muted-foreground">{{ plan.description || plan.slug }}</div>
                  </td>
                  <td class="px-3 py-3">
                    <Badge :variant="businessStatusVariant(plan)">{{ businessStatusLabel(plan) }}</Badge>
                  </td>
                  <td class="px-3 py-3 text-sm text-muted-foreground">{{ quotaText(plan) }}</td>
                  <td class="px-3 py-3 text-sm text-muted-foreground">{{ accessModeLabels[plan.accessMode] }}</td>
                  <td class="px-3 py-3">
                    <Badge :variant="plan.active ? 'verified' : 'secondary'">{{ plan.active ? '启用' : '停用' }}</Badge>
                  </td>
                  <td class="px-3 py-3 text-sm text-muted-foreground">{{ plan.sortOrder }}</td>
                  <td class="px-3 py-3">
                    <div class="flex flex-wrap justify-end gap-1.5">
                      <Button size="sm" variant="outline" @click="openEditPlan(plan)">
                        <FilePenLine class="h-4 w-4" />编辑
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        :disabled="activePlanMutation.isPending.value"
                        @click="setPlanActive(plan, !plan.active)"
                      >
                        <component :is="plan.active ? ToggleLeft : ToggleRight" class="h-4 w-4" />
                        {{ plan.active ? '停用' : '启用' }}
                      </Button>
                    </div>
                  </td>
                </tr>
                <tr v-if="group.visiblePlans.length === 0">
                  <td colspan="7" class="px-3 py-8 text-center text-sm text-muted-foreground">
                    当前筛选下没有套餐。
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </Card>

        <Card v-if="uncategorizedPlans.length > 0" class="border-destructive/30 p-4">
          <div class="font-medium text-destructive">存在未归类套餐</div>
          <p class="mt-2 text-sm text-muted-foreground">这些套餐引用的分类已不存在，请编辑套餐重新选择分类。</p>
        </Card>

        <div v-if="categoryGroups.length === 0 || visibleCategoryGroups.length === 0" class="rounded-md border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
          {{ categoryGroups.length === 0 ? '当前目录暂无分类，先创建分类后再新增套餐。' : '当前筛选下暂无分类或套餐。' }}
        </div>
      </div>
    </section>

    <Dialog v-model:open="isCategoryFormOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ categoryFormTitle }}</DialogTitle>
          <DialogDescription>分类是套餐目录的一级节点，停用后公开目录会隐藏该分类下套餐。</DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <label class="space-y-2">
            <span class="text-sm font-medium">分类名称</span>
            <Input v-model="categoryForm.displayName" placeholder="GPT" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">分类 code</span>
            <Input v-model="categoryForm.code" placeholder="gpt" />
          </label>
          <div class="space-y-2">
            <span class="text-sm font-medium">分类图标</span>
            <div class="flex items-center gap-3 rounded-md border border-border bg-muted/20 p-3">
              <span class="grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-lg border bg-background text-muted-foreground">
                <img v-if="categoryForm.iconDataUrl" :src="categoryForm.iconDataUrl" alt="分类图标预览" class="h-full w-full object-contain" />
                <ImageIcon v-else class="h-5 w-5" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap gap-2">
                  <Button type="button" size="sm" variant="outline" @click="categoryIconInput?.click()">
                    <Upload class="h-4 w-4" />{{ categoryForm.iconDataUrl ? '替换图标' : '上传图标' }}
                  </Button>
                  <Button v-if="categoryForm.iconDataUrl" type="button" size="sm" variant="outline" @click="removeCategoryIcon">
                    <Trash2 class="h-4 w-4" />移除
                  </Button>
                </div>
                <p class="mt-2 text-xs text-muted-foreground">PNG / WebP，建议方形图片，最大 256 KB。</p>
              </div>
              <input ref="categoryIconInput" class="hidden" type="file" :accept="productCategoryIconAccept" @change="selectCategoryIcon" />
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">排序</span>
              <Input v-model.number="categoryForm.sortOrder" type="number" />
            </label>
            <label class="flex items-end gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
              <input v-model="categoryForm.active" type="checkbox" class="mb-1 h-4 w-4 accent-primary" />
              <span>启用分类</span>
            </label>
          </div>

          <DialogFooter>
            <Button variant="outline" :disabled="categorySaving" @click="isCategoryFormOpen = false">取消</Button>
            <Button variant="outline" :disabled="categorySaving" @click="resetCategoryForm">
              <RotateCcw class="h-4 w-4" />重置
            </Button>
            <Button :disabled="categorySaving" @click="saveCategory">
              <Save class="h-4 w-4" />保存分类
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="isPlanFormOpen">
      <DialogContent class="sm:max-w-2xl">
        <DialogHeader>
          <div class="flex flex-wrap items-start justify-between gap-3 pr-8">
            <div>
              <DialogTitle>{{ planFormTitle }}</DialogTitle>
              <DialogDescription class="mt-2">{{ planFormDescription }}</DialogDescription>
            </div>
            <Button size="sm" variant="outline" @click="resetPlanForm"><RotateCcw class="h-4 w-4" />重置</Button>
          </div>
        </DialogHeader>

        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">分类</span>
              <Select v-model="planForm.categoryId">
                <SelectTrigger><SelectValue placeholder="选择分类" /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="category in visibleCategoriesForPlanForm" :key="category.id" :value="category.id">
                    {{ category.displayName }}{{ category.active ? '' : '（停用）' }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">业务状态</span>
              <Select v-model="selectedBusinessStatus">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="item in catalogBusinessStatusOptions" :key="item.value" :value="item.value">
                    {{ item.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
          </div>

          <label class="space-y-2">
            <span class="text-sm font-medium">套餐名称</span>
            <Input v-model="planForm.displayName" placeholder="ChatGPT Pro 20x Web" />
          </label>

          <label class="space-y-2">
            <span class="text-sm font-medium">描述</span>
            <Textarea v-model="planForm.description" class="min-h-20" placeholder="用于后台识别和用户端下拉说明。" />
          </label>

          <div class="grid gap-3 sm:grid-cols-3">
            <label class="space-y-2">
              <span class="text-sm font-medium">额度名称</span>
              <Input v-model="planForm.quotaLabel" placeholder="额度" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">额度单位</span>
              <Input v-model="planForm.quotaUnit" placeholder="USD / GB" />
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">额度周期</span>
              <Input :model-value="planForm.quotaPeriod === 'monthly' ? '每月' : planForm.quotaPeriod" readonly />
            </label>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">接入方式</span>
              <Select v-model="planForm.accessMode">
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="(label, value) in accessModeLabels" :key="value" :value="value">{{ label }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-2">
              <span class="text-sm font-medium">排序</span>
              <Input v-model.number="planForm.sortOrder" type="number" />
            </label>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="flex items-start gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
              <input v-model="planForm.allowCustomVariant" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
              <span>允许用户补充自定义变体</span>
            </label>
            <label class="flex items-start gap-2 rounded-md border border-border bg-muted/30 p-3 text-sm">
              <input v-model="planForm.active" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
              <span>启用套餐</span>
            </label>
          </div>

          <div class="rounded-md border border-border bg-muted/20 p-3">
            <button class="flex w-full items-center justify-between gap-3 text-left text-sm font-medium" @click="showAdvancedPlanFields = !showAdvancedPlanFields">
              <span>高级设置</span>
              <component :is="showAdvancedPlanFields ? ChevronDown : ChevronRight" class="h-4 w-4 text-muted-foreground" />
            </button>
            <div v-if="showAdvancedPlanFields" class="mt-3 space-y-3">
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="space-y-2">
                  <span class="text-sm font-medium">Slug</span>
                  <Input v-model="planForm.slug" placeholder="chatgpt-pro-20x-web" />
                </label>
                <label class="space-y-2">
                  <span class="text-sm font-medium">Provider code</span>
                  <Input v-model="planForm.providerCode" placeholder="openai" />
                </label>
              </div>
              <label class="space-y-2">
                <span class="text-sm font-medium">策略说明</span>
                <Textarea v-model="planForm.policyNote" class="min-h-20" placeholder="说明当前平台开放或限制该套餐的原因。" />
              </label>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" :disabled="planSaving" @click="isPlanFormOpen = false">取消</Button>
            <Button :disabled="planSaving" @click="savePlan"><Save class="h-4 w-4" />保存套餐</Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
