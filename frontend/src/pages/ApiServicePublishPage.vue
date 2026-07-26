<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, ArrowRight, Bot, Eye, Info, PackageCheck, Send } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AccountPaymentSummarySection from '@/components/api-service-publish/AccountPaymentSummarySection.vue'
import ApiAccessSourceSection from '@/components/api-service-publish/ApiAccessSourceSection.vue'
import ApiServicePublishPreview from '@/components/api-service-publish/ApiServicePublishPreview.vue'
import MerchantIdentitySection from '@/components/api-service-publish/MerchantIdentitySection.vue'
import MerchantNoteSection from '@/components/api-service-publish/MerchantNoteSection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import PriceInventorySection from '@/components/api-service-publish/PriceInventorySection.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import PublishStepSection from '@/components/api-service-publish/PublishStepSection.vue'
import PublishWorkflowStepper from '@/components/api-service-publish/PublishWorkflowStepper.vue'
import ResponsivePublishPreview from '@/components/api-service-publish/ResponsivePublishPreview.vue'
import SellingModeSelector from '@/components/api-service-publish/SellingModeSelector.vue'
import type { ApiProviderCategory, ApiServicePublishForm, DistributionSystem, SellingMode } from '@/components/api-service-publish/types'
import { toggleSelectedModel } from '@/components/api-service-publish/modelSelection'
import { apiPublishAssistantSummary } from '@/components/api-service-publish/publishAssistant'
import { completePublishStep, firstErrorStep, publishStepStatus } from '@/components/api-service-publish/publishWorkflow'
import {
  applySimplifiedApiQuotaDefaults,
  createDefaultPaymentOptions,
  defaultPaymentWindowMinutes,
  enabledPaymentOptions,
  formatUsdQuotaForCny,
  generatedTitle,
  merchantNoteTemplate,
  modelProviderCategory,
  paymentMethodLabels,
  providerCategoryLabels,
  selectedCatalogItems,
  sub2ApiPricingPolicy,
} from '@/components/api-service-publish/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { containsSensitiveContent, firstError, type FieldErrors } from '@/lib/formValidation'
import { submitApiService } from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { beijingDateTimeInputToISOString, defaultQuotaExpiresAtInput, formatBeijingDateTimeInput } from '@/lib/apiQuotaExpiration'
import { apiPaymentSettingsMissingReason, cloneApiPaymentAccountSettings, isApiPaymentAccountSettingsComplete, isApiPaymentOptionComplete, isApiPaymentWindowValid } from '@/lib/apiPaymentSettings'
import { useApiPaymentAccountSettingsQuery, useModelCatalog, useMyProfileQuery } from '@/queries/useMarketQueries'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'

type Field =
  | 'merchantIdentity'
  | 'merchantDisplayName'
  | 'distributionSystem'
  | 'defaultMultiplier'
  | 'providerCategory'
  | 'cnyPerUsdCredit'
  | 'selectedModels'
  | 'availableCreditUsd'
  | 'quotaExpiresAt'
  | 'paymentWindowMinutes'
  | 'paymentOptions'
  | 'performance'
  | 'merchantNote'
  | 'sensitive'

type ApiServicePublishStep = 1 | 2 | 3 | 4

const { data: modelCatalog, isLoading: catalogLoading } = useModelCatalog()
const { data: accountPaymentSettings, isLoading: paymentSettingsLoading } = useApiPaymentAccountSettingsQuery()
const { data: myProfile, isLoading: profileLoading } = useMyProfileQuery()
const queryClient = useQueryClient()
const route = useRoute()
const router = useRouter()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const sellingMode = ref<SellingMode>(route.query.after === 'quota' ? 'limited' : 'free')
const isLimitedQuotaMode = computed(() => sellingMode.value === 'limited')
const currentStep = ref<ApiServicePublishStep>(route.query.after === 'quota' ? 2 : 1)
const completedSteps = ref<ApiServicePublishStep[]>(route.query.after === 'quota' ? [1] : [])
const publishSteps = computed(() => isLimitedQuotaMode.value
  ? [
      { title: '选择销售方式', description: '限时额度包' },
      { title: '配置基础服务', description: '接入、模型与体验' },
      { title: '设置额度包', description: '定价、倍率与放量' },
      { title: '确认发布', description: '核对库存与交付' },
    ]
  : [
      { title: '选择销售方式', description: '方式、售价与额度' },
      { title: '设置接入与模型', description: '接入、模型与倍率' },
      { title: '交易与服务', description: '收款、说明与身份' },
      { title: '确认发布', description: '公开服务并接单' },
    ])
const previewOpen = ref(false)
const errors = reactive<FieldErrors<Field>>({})
const pendingProviderCategory = ref<ApiProviderCategory | null>(null)
const formDirty = ref(false)
useUnsavedChangesGuard(formDirty, 'API 服务配置尚未发布，确认离开当前页面？')

const form = reactive<ApiServicePublishForm>({
  merchantIdentityMode: 'store_alias',
  merchantDisplayName: '',
  distributionSystem: 'sub2api',
  distributionSystemNote: '',
  providerCategory: 'gpt',
  billingMode: 'metered_credit',
  deliveryModes: ['api_key_endpoint'],
  shortDescription: '建议首次小额测试',
  cnyPerUsdCredit: 0.8,
  manualBillingNote: '',
  defaultMultiplier: sub2ApiPricingPolicy.textModelMultiplier,
  selectedModels: [
    { modelId: 'gpt-5-mini', multiplierOverride: null, enabled: true },
  ],
  imageCapability: {
    enabled: false,
    supportsTextToImage: false,
    supportsImageToImage: false,
    pricingMode: 'same_multiplier',
    customMultiplier: null,
    note: '',
  },
  availableCreditUsd: 500,
  quotaExpiresAt: defaultQuotaExpiresAtInput(),
  minimumPurchaseCny: 10,
  maximumPurchaseCny: 300,
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  declaredTtftBand: '1_to_3s',
  recommendedConcurrency: 1,
  performanceConfirmedAt: formatBeijingDateTimeInput(new Date()),
  packages: [],
  validity: {
    mode: 'days',
    days: 30,
    startsAt: 'delivered_at',
  },
  usageVisibility: 'merchant_confirmed',
  warranty: {
    mode: 'no_warranty',
    warrantyDays: null,
    coverage: null,
    compensation: null,
    exclusions: null,
    refundNote: null,
  },
  merchantNote: merchantNoteTemplate,
})

const catalog = computed(() => modelCatalog.value ?? [])
const filteredCatalog = computed(() => catalog.value.filter(item => modelProviderCategory(item.provider) === form.providerCategory))
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const selectedModels = computed(() => selectedCatalogItems(form, catalogById.value))
const incompatibleSelectedModels = computed(() => selectedModels.value.filter(item => modelProviderCategory(item.provider) !== form.providerCategory))
const missingSelectedModels = computed(() => form.selectedModels.filter(item => item.enabled && !catalogById.value.has(item.modelId)))
const pendingProviderCategoryLabel = computed(() => pendingProviderCategory.value ? providerCategoryLabels[pendingProviderCategory.value] : '')
const quotaForMinimumPurchase = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, form.minimumPurchaseCny ?? 0))
const enabledPayments = computed(() => enabledPaymentOptions(form))
const paymentWindowValid = computed(() => isApiPaymentWindowValid(form.paymentWindowMinutes))
const paymentSettingsComplete = computed(() => isApiPaymentAccountSettingsComplete(form))
const accountPaymentSettingsValue = computed(() => accountPaymentSettings.value ? cloneApiPaymentAccountSettings(accountPaymentSettings.value) : {
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  updatedAt: '',
})
const accountPaymentSettingsComplete = computed(() => isApiPaymentAccountSettingsComplete(accountPaymentSettingsValue.value))
const profileDisplayName = computed(() => myProfile.value?.displayName.trim() ?? '')
const profileUsername = computed(() => myProfile.value?.username.trim() ?? '')
const profileMerchantDisplayName = computed(() => profileDisplayName.value || profileUsername.value)
const merchantDisplayNameStatus = computed(() => {
  if (profileLoading.value && !form.merchantDisplayName.trim()) return '正在读取个人资料显示名称...'
  if (form.merchantDisplayName.trim()) return '发布时会快照当前个人资料显示名称；单条 API 额度不单独改名。'
  return '请先到个人中心设置显示名称。'
})
function syncMerchantDisplayNameSnapshot() {
  form.merchantDisplayName = profileMerchantDisplayName.value
}

function setStoreAliasVisible(value: boolean) {
  form.merchantIdentityMode = value ? 'store_alias' : 'public_profile'
}

function syncHiddenPublishFields() {
  syncMerchantDisplayNameSnapshot()
  applySimplifiedApiQuotaDefaults(form)
}

syncHiddenPublishFields()

watch(profileMerchantDisplayName, () => syncMerchantDisplayNameSnapshot(), { immediate: true })

watch([catalog, () => form.providerCategory], () => {
  if (!catalog.value.length) return
  const compatibleSelected = form.selectedModels.filter(item => {
    const model = catalogById.value.get(item.modelId)
    return item.enabled && model && modelProviderCategory(model.provider) === form.providerCategory
  })
  if (compatibleSelected.length) {
    form.selectedModels = compatibleSelected
    return
  }
  const firstModel = filteredCatalog.value[0]
  form.selectedModels = firstModel
    ? [{ modelId: firstModel.id, multiplierOverride: null, enabled: true }]
    : []
}, { immediate: true })

watch(accountPaymentSettingsValue, settings => {
  form.paymentWindowMinutes = settings.paymentWindowMinutes
  form.paymentOptions = settings.paymentOptions.map(option => ({ ...option }))
}, { immediate: true })

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function hasContactLikeText(value: string) {
  return /@|微信|VX|vx|telegram|tg|邮箱|email|https?:\/\/|linux\.do|\.com|\.cn|[0-9]{6,}/i.test(value)
}

function hasMisleadingMerchantName(value: string) {
  return /官方|担保|兜底|认证|跑路|实名/i.test(value)
}

function displayNameLength(value: string) {
  return Array.from(value.trim()).length
}

const freeFieldSteps: Record<Field, ApiServicePublishStep> = {
  merchantIdentity: 3,
  merchantDisplayName: 3,
  distributionSystem: 2,
  defaultMultiplier: 2,
  providerCategory: 2,
  cnyPerUsdCredit: 1,
  selectedModels: 2,
  availableCreditUsd: 1,
  quotaExpiresAt: 1,
  paymentWindowMinutes: 3,
  paymentOptions: 3,
  performance: 3,
  merchantNote: 3,
  sensitive: 3,
}

const limitedFieldSteps: Record<Field, ApiServicePublishStep> = {
  merchantIdentity: 2,
  merchantDisplayName: 2,
  distributionSystem: 2,
  defaultMultiplier: 2,
  providerCategory: 2,
  cnyPerUsdCredit: 2,
  selectedModels: 2,
  availableCreditUsd: 2,
  quotaExpiresAt: 2,
  paymentWindowMinutes: 2,
  paymentOptions: 2,
  performance: 2,
  merchantNote: 2,
  sensitive: 2,
}

function currentFieldSteps() {
  return isLimitedQuotaMode.value ? limitedFieldSteps : freeFieldSteps
}

function collectValidationErrors() {
  syncHiddenPublishFields()
  const next: FieldErrors<Field> = {}
  const merchantDisplayName = form.merchantDisplayName.trim()
  if (!['public_profile', 'store_alias'].includes(form.merchantIdentityMode)) next.merchantIdentity = '请选择对外展示身份。'
  if (form.merchantIdentityMode === 'store_alias') {
    if (!merchantDisplayName) next.merchantDisplayName = profileLoading.value ? '正在读取个人资料显示名称。' : '请先到个人中心设置显示名称。'
    else if (displayNameLength(merchantDisplayName) > 32) next.merchantDisplayName = '商家展示名最多 32 个字符，请到个人中心调整。'
    else if (hasContactLikeText(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含联系方式、链接或 linux.do 用户名，请到个人中心调整。'
    else if (hasMisleadingMerchantName(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含官方、担保、兜底等误导词，请到个人中心调整。'
  }
  if (!['sub2api', 'other'].includes(form.distributionSystem)) next.distributionSystem = '请选择接入类型。'
  if (form.distributionSystem === 'other' && (!Number.isFinite(form.defaultMultiplier) || form.defaultMultiplier <= 0)) {
    next.defaultMultiplier = '默认服务倍率必须大于 0。'
  }
  if (!form.providerCategory) next.providerCategory = '请选择模型大类。'
  if (!form.cnyPerUsdCredit || form.cnyPerUsdCredit < sub2ApiPricingPolicy.minimumCnyPerUsdCredit || form.cnyPerUsdCredit > sub2ApiPricingPolicy.maximumCnyPerUsdCredit) {
    next.cnyPerUsdCredit = '每 $1 美元额度售价必须大于 0。'
  }
  if (!form.availableCreditUsd || form.availableCreditUsd <= 0) next.availableCreditUsd = '可售美元额度必须大于 0。'
  const quotaExpiresAtISO = beijingDateTimeInputToISOString(form.quotaExpiresAt)
  if (!quotaExpiresAtISO) next.quotaExpiresAt = '请填写有效的额度有效至时间。'
  else if (new Date(quotaExpiresAtISO).getTime() <= Date.now()) next.quotaExpiresAt = '额度有效至时间必须晚于当前时间。'
  if (!form.selectedModels.some(item => item.enabled)) next.selectedModels = '至少选择一个模型。'
  if (missingSelectedModels.value.length) next.selectedModels = '已选模型不在当前后端模型目录中，请重新选择。'
  if (incompatibleSelectedModels.value.length) next.selectedModels = '已选模型必须全部属于当前模型大类。'
  if (!paymentWindowValid.value) next.paymentWindowMinutes = '买家确认付款窗口固定为 10 分钟。'
  if (!enabledPayments.value.length) {
    next.paymentOptions = '请至少启用一种收款方式。'
  } else {
    const missingOption = enabledPayments.value.find(option => !isApiPaymentOptionComplete(option))
    if (missingOption) next.paymentOptions = apiPaymentSettingsMissingReason(form)
  }
  if (!form.declaredTtftBand || form.recommendedConcurrency < 1 || !beijingDateTimeInputToISOString(form.performanceConfirmedAt)) {
    next.performance = '请完整填写首字响应区间、建议并发和最近确认时间。'
  }
  if (!form.merchantNote.trim()) next.merchantNote = '请填写备注信息。'
  if (form.merchantNote.length > 800) next.merchantNote = '备注信息最多 800 字。'
  if (containsSensitiveContent([
    form.merchantDisplayName,
    form.merchantNote,
    ...form.paymentOptions.map(option => option.paymentInstructions),
  ])) next.sensitive = '请移除 API Key、Sub2API key、endpoint 密钥、token、Session、Cookie、密码、付款码或面板凭据。'

  return next
}

function validateStep(step: ApiServicePublishStep) {
  const fieldSteps = currentFieldSteps()
  const next = Object.fromEntries(
    Object.entries(collectValidationErrors()).filter(([field]) => fieldSteps[field as Field] === step),
  ) as FieldErrors<Field>
  setErrors(next)
  return Object.keys(next).length === 0
}

function validateAll() {
  const next = collectValidationErrors()
  setErrors(next)
  return Object.keys(next).length === 0
}

const completeness = computed(() => {
  const conflict = (label: string) => ({ label, status: 'conflict' as const })
  const done = (label: string) => ({ label, status: 'done' as const })
  const pending = (label: string) => ({ label, status: 'pending' as const })
  const items: Array<{ label: string, status: 'done' | 'pending' | 'conflict' }> = [
    form.merchantIdentityMode === 'public_profile' || form.merchantDisplayName.trim() ? done('展示身份') : pending('展示身份'),
    form.distributionSystem ? done('接入类型') : pending('接入类型'),
    form.distributionSystem === 'sub2api' || (Number.isFinite(form.defaultMultiplier) && form.defaultMultiplier > 0) ? done('服务倍率') : pending('服务倍率'),
  ]
  if (!isLimitedQuotaMode.value) {
    items.push(
      form.cnyPerUsdCredit && form.cnyPerUsdCredit > 0 ? done('额度售价') : pending('额度售价'),
      form.availableCreditUsd && form.availableCreditUsd > 0 ? done('可售额度') : pending('可售额度'),
      beijingDateTimeInputToISOString(form.quotaExpiresAt) ? done('有效时间') : pending('有效时间'),
    )
  }
  items.push(
    accountPaymentSettingsComplete.value && paymentSettingsComplete.value ? done('收款方式') : pending('收款方式'),
    form.declaredTtftBand && form.recommendedConcurrency > 0 && beijingDateTimeInputToISOString(form.performanceConfirmedAt) ? done('服务体验声明') : pending('服务体验声明'),
    form.providerCategory ? done('模型大类') : pending('模型大类'),
    incompatibleSelectedModels.value.length ? conflict('具体模型') : form.selectedModels.some(item => item.enabled) ? done('具体模型') : pending('具体模型'),
    form.merchantNote.trim() ? done('备注信息') : pending('备注信息'),
  )
  return items
})
const publishAssistant = computed(() => apiPublishAssistantSummary(completeness.value))

const risks = computed(() => {
  const rows: string[] = []
  rows.push(isLimitedQuotaMode.value
    ? '额度、总价、模型倍率、库存和失效时间将在下一步设置；当前页面只保存关联的基础服务。'
    : 'API 细节和用量核对由双方站外确认，平台不保存凭据，也不提供实时校验。')
  if (incompatibleSelectedModels.value.length) rows.push('当前存在不属于所选模型大类的模型，需清空后才能提交。')
  return rows
})

const canSubmit = computed(() => completeness.value.every(item => item.status === 'done'))
const publishBlockReason = computed(() => {
  if (canSubmit.value) return ''
  const pendingItem = completeness.value.find(item => item.status !== 'done')
  if (pendingItem?.label === '收款方式') {
    if (!paymentWindowValid.value) return '买家确认付款窗口固定为 10 分钟。'
    if (!accountPaymentSettingsComplete.value || !enabledPayments.value.length) return '先到个人中心配置 API 收款设置，发布后才会进入公开服务列表。'
    return apiPaymentSettingsMissingReason(form) || '请到个人中心补全已启用收款方式。'
  }
  if (pendingItem?.label === '展示身份') {
    return profileLoading.value ? '正在读取个人资料显示名称。' : '请先到个人中心设置显示名称。'
  }
  if (pendingItem) return `请先补全：${pendingItem.label}。`
  return '请先补全发布配置。'
})
const selectedModelSummary = computed(() => selectedModels.value.map(item => item.displayName).join(' / ') || '待选择模型')
const paymentSummary = computed(() => {
  const labels = enabledPayments.value.map(option => paymentMethodLabels[option.paymentMethod])
  return labels.length ? `${labels.join(' / ')} · ${form.paymentWindowMinutes} 分钟确认` : '收款方式待配置'
})
const stepOneSummary = computed(() => {
  if (isLimitedQuotaMode.value) return '限时额度包 · 当前先配置可复用基础服务'
  const expiry = form.quotaExpiresAt ? form.quotaExpiresAt.slice(0, 10).replaceAll('-', '/') : '待填写有效期'
  return `自由额度 · ¥${form.cnyPerUsdCredit ?? 0} / $1 · 可售 $${form.availableCreditUsd ?? 0} · 有效至 ${expiry}`
})
const stepTwoSummary = computed(() => {
  const multiplier = form.distributionSystem === 'sub2api' ? '1.00x' : `${form.defaultMultiplier.toFixed(2)}x`
  return `${form.distributionSystem === 'sub2api' ? 'Sub2API' : '其他 API 接入'} · ${providerCategoryLabels[form.providerCategory]} · ${selectedModelSummary.value} · ${multiplier}`
})
const stepThreeSummary = computed(() => `${paymentSummary.value} · ${form.merchantIdentityMode === 'store_alias' ? '商家展示名' : '公开个人身份'}`)
const primaryActionLabel = computed(() => {
  if (publishMutation.isPending.value) return '处理中'
  if (isLimitedQuotaMode.value) {
    return currentStep.value === 1 ? '继续：配置基础服务' : '保存基础服务，下一步设置额度包'
  }
  if (currentStep.value === 1) return '继续：设置接入与模型'
  if (currentStep.value === 2) return '继续：交易与服务'
  if (currentStep.value === 3) return '检查并预览'
  return '发布自由额度服务'
})
const actionHeading = computed(() => {
  if (isLimitedQuotaMode.value) return currentStep.value === 1 ? '下一步：配置基础服务' : '下一步：设置额度包'
  return publishSteps.value[currentStep.value - 1]?.title ?? '发布 API 额度'
})
const actionBlockReason = computed(() => {
  if (isLimitedQuotaMode.value && currentStep.value === 2) return publishBlockReason.value
  if (!isLimitedQuotaMode.value && currentStep.value === 4) return publishBlockReason.value
  return ''
})

const publishMutation = useMutation({
  mutationFn: () => {
    syncHiddenPublishFields()
    return submitApiService({
      ...form,
      generatedTitle: generatedTitle(form, catalogById.value),
      status: 'reviewing',
    })
  },
  async onSuccess(service) {
    formDirty.value = false
    await invalidateApiServicePublishQueries()
    trackAnalytics('api_service_publish_success', {
      source_route: analyticsSourceRoute(),
      provider_category: form.providerCategory,
      billing_mode: form.billingMode,
      delivery_mode: form.deliveryModes[0],
      minimum_purchase_cny: form.minimumPurchaseCny,
    })
    toast.success(isLimitedQuotaMode.value ? '基础服务已保存，继续设置额度包。' : '自由额度服务已发布并开启接单。')
    const destination = isLimitedQuotaMode.value
      ? `/api-market/quota/new?serviceId=${service.id}`
      : '/my/api-services'
    await router.replace(destination)
  },
  onError(error) {
    toast.error(error instanceof Error ? error.message : 'API 服务发布失败，请检查配置后重试。')
  },
})

async function invalidateApiServicePublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['api-services'] })
  await queryClient.invalidateQueries({ queryKey: ['api-market'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['notifications'] })
}

function setSellingMode(value: SellingMode) {
  sellingMode.value = value
  form.billingMode = 'metered_credit'
  currentStep.value = 1
  completedSteps.value = []
  setErrors({})
}

function setDistribution(value: DistributionSystem) {
  form.distributionSystem = value
  form.billingMode = 'metered_credit'
  form.usageVisibility = 'merchant_confirmed'
  form.deliveryModes = ['api_key_endpoint']
  if (value === 'sub2api') {
    form.defaultMultiplier = sub2ApiPricingPolicy.textModelMultiplier
    if (!form.distributionSystemNote.trim() || form.distributionSystemNote.includes('其他 API')) {
      form.distributionSystemNote = 'Sub2API 标准美元额度，接入细节由双方站外确认。'
    }
    return
  }
  form.distributionSystemNote = form.distributionSystemNote.trim() || '其他 API 接入，额度与用量由商户站外说明。'
  if (!Number.isFinite(form.defaultMultiplier) || form.defaultMultiplier <= 0) form.defaultMultiplier = 1
}

function setDefaultMultiplier(value: string) {
  form.defaultMultiplier = Number(value)
}

function toggleModel(id: string) {
  const model = catalogById.value.get(id)
  if (!model || modelProviderCategory(model.provider) !== form.providerCategory) return
  form.selectedModels = toggleSelectedModel(form.selectedModels, id)
}

async function focusStep(step: ApiServicePublishStep) {
  await nextTick()
  const section = document.getElementById(`publish-step-${step}`)
  section?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  const target = section?.querySelector<HTMLElement>('[aria-invalid="true"]')
    ?? section?.querySelector<HTMLElement>('input, textarea, button, [tabindex="0"]')
    ?? section?.querySelector<HTMLElement>('[data-publish-step-heading]')
  target?.focus({ preventScroll: true })
}

function selectStep(step: number) {
  if (step < 1 || step > 4) return
  const target = step as ApiServicePublishStep
  if (target !== currentStep.value && !completedSteps.value.includes(target)) return
  currentStep.value = target
  setErrors({})
  void focusStep(target)
}

function completeCurrentStep() {
  if (!validateStep(currentStep.value)) {
    toast.warning(firstError(errors) ?? '请先完成当前步骤。')
    void focusStep(currentStep.value)
    return false
  }
  completedSteps.value = completePublishStep(completedSteps.value, currentStep.value) as ApiServicePublishStep[]
  return true
}

function continueWorkflow() {
  if (!completeCurrentStep()) return
  if (isLimitedQuotaMode.value && currentStep.value === 2) {
    publishService()
    return
  }
  if (currentStep.value < 4) {
    currentStep.value = (currentStep.value + 1) as ApiServicePublishStep
    void focusStep(currentStep.value)
  }
}

function goBack() {
  if (currentStep.value === 1) return
  currentStep.value = (currentStep.value - 1) as ApiServicePublishStep
  setErrors({})
  void focusStep(currentStep.value)
}

function runPrimaryAction() {
  if (!isLimitedQuotaMode.value && currentStep.value === 4) {
    publishService()
    return
  }
  continueWorkflow()
}

function publishService() {
  syncHiddenPublishFields()
  if (!validateAll()) {
    const errorStep = firstErrorStep(errors, currentFieldSteps()) as ApiServicePublishStep | undefined
    if (errorStep) {
      currentStep.value = errorStep
      void focusStep(errorStep)
    }
    toast.warning(firstError(errors) ?? '请先补全发布配置。')
    return
  }
  publishMutation.mutate()
}

function preview() {
  syncHiddenPublishFields()
  if (window.matchMedia('(min-width: 1241px)').matches) {
    document.querySelector<HTMLElement>('.api-publish-responsive-preview')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  previewOpen.value = true
}

function selectedModelsCompatibleWith(category: ApiProviderCategory) {
  return selectedModels.value.filter(item => modelProviderCategory(item.provider) !== category)
}

function requestProviderCategory(value: ApiProviderCategory) {
  if (value === form.providerCategory) return
  if (selectedModelsCompatibleWith(value).length) {
    pendingProviderCategory.value = value
    return
  }
  applyProviderCategory(value)
}

function applyProviderCategory(value: ApiProviderCategory) {
  form.providerCategory = value
  form.selectedModels = form.selectedModels.filter(item => {
    const model = catalogById.value.get(item.modelId)
    return model ? modelProviderCategory(model.provider) === value : false
  })
  pendingProviderCategory.value = null
}

function cancelProviderCategoryChange() {
  pendingProviderCategory.value = null
}

function confirmProviderCategoryChange() {
  if (!pendingProviderCategory.value) return
  applyProviderCategory(pendingProviderCategory.value)
}
</script>

<template>
  <div class="api-publish-page space-y-4 pb-20" @input="formDirty = true" @change="formDirty = true">
    <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
      <div>
        <h1 class="text-xl font-semibold">发布 API 额度</h1>
        <p class="mt-1 max-w-3xl text-xs leading-5 text-muted-foreground sm:text-sm">
          {{ isLimitedQuotaMode ? '先配置可复用的 API 基础服务，下一步再设置额度包规格、价格、库存和放量时间。' : '买家自定购买金额，系统按你的美元额度售价创建订单。' }}
        </p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" @click="preview"><Eye class="h-4 w-4" />预览</Button>
      </div>
    </div>

    <PublishWorkflowStepper
      :steps="publishSteps"
      :current-step="currentStep"
      :completed-steps="completedSteps"
      @select="selectStep"
    />

    <div class="api-publish-layout grid min-w-0 gap-3 lg:items-start">
      <section class="api-publish-editor min-w-0 space-y-3">
        <PublishStepSection
          :step="1"
          title="选择销售方式"
          description="选择自由额度或限时额度包，并填写当前方式所需的价格与额度。"
          :status="publishStepStatus(1, currentStep, completedSteps)"
          :summary="stepOneSummary"
          @edit="selectStep"
        >
          <div class="space-y-3">
            <SellingModeSelector :model-value="sellingMode" @update:model-value="setSellingMode" />
            <div class="flex gap-2 rounded-md border border-primary/15 bg-primary/5 px-3 py-2 text-xs leading-5 text-foreground">
              <Info class="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />
              <div class="min-w-0">
                <span class="font-semibold">买家流程：</span>
                <span>{{ isLimitedQuotaMode ? '选择额度包 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证' : '填写购买金额 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证' }}</span>
                <span class="ml-2 text-muted-foreground">平台记录订单，不代收款。</span>
              </div>
            </div>
            <div v-if="isLimitedQuotaMode" class="flex gap-2 rounded-md border border-orange-200 bg-orange-50/60 px-3 py-2 text-xs leading-5 text-orange-950">
              <PackageCheck class="mt-0.5 h-4 w-4 shrink-0 text-orange-600" />
              <div><span class="font-semibold">下一步设置额度包：</span><span class="text-orange-900/75">美元额度、总价、倍率、库存、放量和失效时间。</span></div>
            </div>
            <PriceInventorySection v-else :form="form" :errors="errors" />
          </div>
        </PublishStepSection>

        <PublishStepSection
          :step="2"
          :title="isLimitedQuotaMode ? '配置基础服务' : '设置接入与模型'"
          :description="isLimitedQuotaMode ? '配置额度包需要复用的接入、模型、收款和展示信息。' : '告诉买家支持哪些接入方式和模型。'"
          :status="publishStepStatus(2, currentStep, completedSteps)"
          :summary="stepTwoSummary"
          @edit="selectStep"
        >
          <div class="space-y-3">
            <ApiAccessSourceSection :form="form" :errors="errors" :selling-mode="sellingMode" @set-distribution="setDistribution" @set-default-multiplier="setDefaultMultiplier" />
            <AccountPaymentSummarySection v-if="isLimitedQuotaMode" :form="form" :settings="accountPaymentSettingsValue" :loading="paymentSettingsLoading" />
            <ProviderCategorySelector :model-value="form.providerCategory" :selected-count="selectedModels.length" @update:model-value="requestProviderCategory" />
            <Card class="api-publish-card">
              <div class="api-publish-card-header">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="flex items-start gap-2">
                    <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-600"><Bot class="h-4 w-4" /></span>
                    <div><h2>具体模型</h2><p>搜索并勾选要出售的模型。</p></div>
                  </div>
                  <Badge variant="model">{{ selectedModels.length }} 个模型</Badge>
                </div>
              </div>
              <div class="api-publish-card-body">
                <div v-if="incompatibleSelectedModels.length" class="mb-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">当前存在不属于所选模型大类的模型，请切换模型大类并确认清空，或手动移除冲突模型。</div>
                <div v-if="catalogLoading" class="rounded-md border border-border bg-background p-4 text-sm text-muted-foreground">正在加载平台模型目录...</div>
                <ModelMultiSelect v-else :form="form" :provider-category="form.providerCategory" :catalog="filteredCatalog" :errors="errors" @toggle-model="toggleModel" />
              </div>
            </Card>
            <template v-if="isLimitedQuotaMode">
              <MerchantNoteSection :form="form" :errors="errors" />
              <MerchantIdentitySection :form="form" :profile-loading="profileLoading" :display-name-status="merchantDisplayNameStatus" :error="errors.merchantDisplayName" @set-store-alias-visible="setStoreAliasVisible" />
            </template>
          </div>
        </PublishStepSection>

        <PublishStepSection
          :step="3"
          :title="isLimitedQuotaMode ? '设置额度包' : '交易与服务'"
          :description="isLimitedQuotaMode ? '基础服务保存后，在下一页设置价格、库存与场次。' : '核对收款、服务说明和买家看到的卖家名称。'"
          :status="publishStepStatus(3, currentStep, completedSteps)"
          :summary="isLimitedQuotaMode ? '待保存基础服务后继续' : stepThreeSummary"
          @edit="selectStep"
        >
          <div v-if="!isLimitedQuotaMode" class="space-y-3">
            <div v-if="errors.sensitive" class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive" role="alert">{{ errors.sensitive }}</div>
            <AccountPaymentSummarySection :form="form" :settings="accountPaymentSettingsValue" :loading="paymentSettingsLoading" />
            <MerchantNoteSection :form="form" :errors="errors" />
            <MerchantIdentitySection :form="form" :profile-loading="profileLoading" :display-name-status="merchantDisplayNameStatus" :error="errors.merchantDisplayName" @set-store-alias-visible="setStoreAliasVisible" />
          </div>
        </PublishStepSection>

        <PublishStepSection
          :step="4"
          title="确认发布"
          :description="isLimitedQuotaMode ? '在额度包页面核对库存、场次与交付后发布。' : '核对所有公开信息并发布自由额度服务。'"
          :status="publishStepStatus(4, currentStep, completedSteps)"
          :summary="isLimitedQuotaMode ? '待设置额度包' : '发布前完整核对'"
          @edit="selectStep"
        >
          <div v-if="!isLimitedQuotaMode" class="api-publish-confirm-list">
            <div><span>销售方式与定价</span><strong>{{ stepOneSummary }}</strong></div>
            <div><span>接入与模型</span><strong>{{ stepTwoSummary }}</strong></div>
            <div><span>交易与服务</span><strong>{{ stepThreeSummary }}</strong></div>
            <p class="text-xs leading-5 text-muted-foreground">发布后买家可按当前快照创建订单；平台记录订单，不代收款，也不保存 API Key。</p>
          </div>
        </PublishStepSection>
      </section>

      <ResponsivePublishPreview v-model:open="previewOpen" :title="isLimitedQuotaMode ? '限时额度包基础服务预览' : '自由额度预览'" :description="isLimitedQuotaMode ? '额度包价格、库存和时间将在下一步设置。' : '根据当前表单实时生成。'">
        <ApiServicePublishPreview :form="form" :catalog-by-id="catalogById" :completeness="completeness" :risks="risks" :quota-for-minimum-purchase="quotaForMinimumPurchase" :selling-mode="sellingMode" :preview-only="currentStep === 4" />
      </ResponsivePublishPreview>
    </div>

    <div class="sticky bottom-0 z-30 -mx-4 border-t border-border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur md:mx-0 md:rounded-lg md:border md:bg-card/95">
      <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
        <div class="hidden md:block"><div class="font-semibold">{{ actionHeading }}</div><p class="text-xs text-muted-foreground">发布必填 {{ publishAssistant.doneCount }} / {{ publishAssistant.totalCount }} · {{ publishAssistant.topPendingText }}</p></div>
        <div class="grid gap-1.5 md:flex md:shrink-0 md:items-center md:gap-3">
          <Button v-if="currentStep > 1" variant="outline" :disabled="publishMutation.isPending.value" @click="goBack"><ArrowLeft class="h-4 w-4" />上一步</Button>
          <Button :disabled="publishMutation.isPending.value" @click="runPrimaryAction">
            <Send v-if="!isLimitedQuotaMode && currentStep === 4" class="h-4 w-4" />
            <ArrowRight v-else class="h-4 w-4" />
            {{ primaryActionLabel }}
          </Button>
          <p v-if="actionBlockReason" class="line-clamp-2 max-w-sm text-xs leading-5 text-warning md:text-right">{{ actionBlockReason }}</p>
        </div>
      </div>
    </div>

    <div
      v-if="pendingProviderCategory"
      class="fixed inset-0 z-40 grid place-items-center bg-background/80 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="provider-category-confirm-title"
      @click.self="cancelProviderCategoryChange"
    >
      <Card class="w-full max-w-md p-5 shadow-lg">
        <h2 id="provider-category-confirm-title" class="text-base font-semibold">切换模型大类</h2>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">
          切换到 {{ pendingProviderCategoryLabel }} 会清空当前不兼容的模型选择。GPT 与 Claude 必须分开发布，不能同时存在于同一服务中。
        </p>
        <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" @click="cancelProviderCategoryChange">取消</Button>
          <Button @click="confirmProviderCategoryChange">确认切换并清空</Button>
        </div>
      </Card>
    </div>
  </div>
</template>
