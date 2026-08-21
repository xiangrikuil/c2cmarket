<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  Eye,
  Info,
  PackageCheck,
  Send,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiProbeConnectionDialog from '@/components/api-probe/ApiProbeConnectionDialog.vue'
import AccountPaymentSummarySection from '@/components/api-service-publish/AccountPaymentSummarySection.vue'
import SellerCommerceStatusPanel from '@/components/api-order/SellerCommerceStatusPanel.vue'
import ApiPaymentSettingsDialog from '@/components/contact-payment/ApiPaymentSettingsDialog.vue'
import ApiAccessSourceSection from '@/components/api-service-publish/ApiAccessSourceSection.vue'
import ApiServicePublishPreview from '@/components/api-service-publish/ApiServicePublishPreview.vue'
import FixedPackageSection from '@/components/api-service-publish/FixedPackageSection.vue'
import MerchantIdentitySection from '@/components/api-service-publish/MerchantIdentitySection.vue'
import MerchantContactMethodsSection from '@/components/api-service-publish/MerchantContactMethodsSection.vue'
import MerchantNoteSection from '@/components/api-service-publish/MerchantNoteSection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import PriceInventorySection from '@/components/api-service-publish/PriceInventorySection.vue'
import ProbeConnectionSection from '@/components/api-service-publish/ProbeConnectionSection.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import SelectedModelsPricingTable from '@/components/api-service-publish/SelectedModelsPricingTable.vue'
import PublishStepSection from '@/components/api-service-publish/PublishStepSection.vue'
import PublishWorkflowStepper from '@/components/api-service-publish/PublishWorkflowStepper.vue'
import ResponsivePublishPreview from '@/components/api-service-publish/ResponsivePublishPreview.vue'
import SellingModeSelector from '@/components/api-service-publish/SellingModeSelector.vue'
import { sellingModeLabels, type ApiProviderCategory, type ApiServicePublishForm, type DistributionSystem, type SellingMode } from '@/components/api-service-publish/types'
import { toggleSelectedModel } from '@/components/api-service-publish/modelSelection'
import { createDefaultApiServicePackage } from '@/components/api-service-publish/packages'
import { apiPublishAssistantSummary, apiPublishModeFromQuery } from '@/components/api-service-publish/publishAssistant'
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
  providerCategoryLabel,
  selectedCatalogItems,
  sub2ApiPricingPolicy,
} from '@/components/api-service-publish/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { backendErrorMessage } from '@/lib/backendClient'
import { containsSensitiveContent, firstError, type FieldErrors } from '@/lib/formValidation'
import { submitApiService } from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { beijingDateTimeInputToISOString, defaultQuotaExpiresAtInput } from '@/lib/apiQuotaExpiration'
import { apiPaymentSettingsMissingReason, cloneApiPaymentAccountSettings, isApiPaymentAccountSettingsComplete, isApiPaymentOptionComplete, isApiPaymentWindowValid } from '@/lib/apiPaymentSettings'
import { apiQuotaUsagePolicyInputError, defaultApiQuotaUsagePolicyInput } from '@/lib/apiQuotaPolicy'
import { useApiPaymentAccountSettingsQuery, useModelCatalog, useMyProfileQuery, useSellerCommerceStatus } from '@/queries/useMarketQueries'
import { apiMarketAvailabilityQueryKey } from '@/queries/useApiMarketAvailability'
import { useOwnerAPIProbeConnections } from '@/queries/useApiHealthQueries'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import type { OwnerAPIProbeConnection } from '@/types/apiHealth'

type Field =
  | 'merchantIdentity'
  | 'merchantDisplayName'
	| 'ownerContactMethods'
  | 'distributionSystem'
  | 'probeConnection'
  | 'defaultMultiplier'
  | 'providerCategory'
  | 'cnyPerUsdCredit'
  | 'selectedModels'
  | 'availableCreditUsd'
  | 'quotaExpiresAt'
  | 'quotaUsagePolicy'
  | 'packages'
  | 'paymentWindowMinutes'
  | 'paymentOptions'
  | 'accountPool'
  | 'refundCommitment'
  | 'performance'
  | 'merchantNote'
  | 'sensitive'

type ApiServicePublishStep = 1 | 2 | 3

const { data: modelCatalog, isLoading: catalogLoading } = useModelCatalog()
const { data: accountPaymentSettings, isLoading: paymentSettingsLoading } = useApiPaymentAccountSettingsQuery()
const { data: myProfile, isLoading: profileLoading } = useMyProfileQuery()
const commerceStatusQuery = useSellerCommerceStatus()
const probeConnectionsQuery = useOwnerAPIProbeConnections()
const queryClient = useQueryClient()
const route = useRoute()
const router = useRouter()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const requestedSellingMode = apiPublishModeFromQuery(route.query.mode, route.query.after)
const initialSellingMode: SellingMode | null = requestedSellingMode === 'limited' ? null : requestedSellingMode
const sellingMode = ref<SellingMode | null>(initialSellingMode)
const editorSellingMode = computed<SellingMode>(() => sellingMode.value ?? 'free')
const isLimitedQuotaMode = computed(() => sellingMode.value === 'limited')
const currentStep = ref<ApiServicePublishStep>(1)
const completedSteps = ref<ApiServicePublishStep[]>([])
const publishSteps = computed(() => isLimitedQuotaMode.value
  ? [
      { title: '销售模式', description: '限量额度包' },
      { title: '配置基础服务', description: '接入、模型与体验' },
      { title: '设置额度包', description: '定价、库存与放量' },
    ]
  : [
      {
        title: `配置${sellingModeLabels[sellingMode.value === 'package' ? 'package' : 'free']}`,
        description: sellingMode.value === 'package' ? '规格、售价与库存' : '售价、额度与有效期',
      },
      { title: '设置接入与模型', description: '接入、模型与统一倍率' },
      { title: '交易与服务', description: '收款、说明与身份' },
    ])
const previewOpen = ref(false)
const paymentSettingsDialogOpen = ref(false)
const probeConnectionDialogOpen = ref(false)
const errors = reactive<FieldErrors<Field>>({})
const pendingProviderCategory = ref<ApiProviderCategory | null>(null)
const formDirty = ref(false)
useUnsavedChangesGuard(formDirty, 'API 服务配置尚未发布，确认离开当前页面？')

const form = reactive<ApiServicePublishForm>({
	probeConnectionId: '',
	ownerContactMethodId: '',
  merchantIdentityMode: 'public_profile',
  merchantDisplayName: '',
  distributionSystem: 'sub2api',
  distributionSystemNote: '',
  providerCategory: 'gpt',
  billingMode: initialSellingMode === 'package' ? 'fixed_package' : 'metered_credit',
  deliveryModes: ['api_key_endpoint'],
  shortDescription: '建议首次小额测试',
  cnyPerUsdCredit: 0.15,
  manualBillingNote: '',
  defaultMultiplier: sub2ApiPricingPolicy.textModelMultiplier,
  selectedModels: [
    { modelId: 'gpt-5-mini', enabled: true },
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
  quotaUsagePolicy: defaultApiQuotaUsagePolicyInput(),
  minimumPurchaseCny: 10,
  maximumPurchaseCny: null,
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  declaredMaxConcurrency: 2,
  promptAuditEnabled: null,
  packages: initialSellingMode === 'package' ? [createDefaultApiServicePackage(['gpt-5-mini'])] : [],
  validity: {
    mode: 'days',
    days: 30,
    startsAt: 'delivered_at',
  },
  usageVisibility: 'merchant_confirmed',
  accountPoolType: '',
  accountPoolCustomName: '',
  warranty: {
    mode: '',
  },
  merchantNote: merchantNoteTemplate,
})
const isFixedPackageMode = computed(() => sellingMode.value === 'package')

const catalog = computed(() => modelCatalog.value ?? [])
const filteredCatalog = computed(() => catalog.value.filter(item => modelProviderCategory(item.provider) === form.providerCategory))
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const selectedModels = computed(() => selectedCatalogItems(form, catalogById.value))
const probeConnections = computed(() => probeConnectionsQuery.data.value ?? [])
const selectedProbeConnection = computed(() => probeConnections.value.find(connection => connection.id === form.probeConnectionId) ?? null)
const probeConnectionReady = computed(() => Boolean(
  selectedProbeConnection.value?.enabled && selectedProbeConnection.value.verificationStatus === 'verified',
))
const probeConnectionError = computed(() => probeConnectionsQuery.error.value
  ? backendErrorMessage(probeConnectionsQuery.error.value, '探针连接暂时无法读取。')
  : '')
const incompatibleSelectedModels = computed(() => selectedModels.value.filter(item => modelProviderCategory(item.provider) !== form.providerCategory))
const missingSelectedModels = computed(() => form.selectedModels.filter(item => item.enabled && !catalogById.value.has(item.modelId)))
const pendingProviderCategoryLabel = computed(() => pendingProviderCategory.value ? providerCategoryLabel(pendingProviderCategory.value) : '')
const quotaForMinimumPurchase = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, form.minimumPurchaseCny ?? 0))
const enabledPackages = computed(() => form.packages.filter(item => item.enabled))
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

function selectProbeConnection(connection: OwnerAPIProbeConnection) {
  form.probeConnectionId = connection.id
  delete errors.probeConnection
  formDirty.value = true
}

function syncHiddenPublishFields() {
  syncMerchantDisplayNameSnapshot()
  applySimplifiedApiQuotaDefaults(form)
  if (form.billingMode !== 'fixed_package') return
  const prices = enabledPackages.value.map(item => item.priceCny).filter(value => Number.isFinite(value) && value > 0)
  form.minimumPurchaseCny = prices.length ? Math.min(...prices) : null
  form.maximumPurchaseCny = prices.length ? Math.max(...prices) : null
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
    ? [{ modelId: firstModel.id, enabled: true }]
    : []
}, { immediate: true })

watch(accountPaymentSettingsValue, settings => {
  form.paymentWindowMinutes = settings.paymentWindowMinutes
  form.paymentOptions = settings.paymentOptions.map(option => ({ ...option }))
}, { immediate: true })

watch(
  [
    () => form.billingMode,
    () => form.selectedModels.filter(item => item.enabled).map(item => item.modelId).sort().join('|'),
  ],
  () => {
    if (form.billingMode !== 'fixed_package') return
    const enabledModelIds = form.selectedModels.filter(item => item.enabled).map(item => item.modelId)
    for (const item of form.packages) {
      item.modelCatalogIds = [...enabledModelIds]
    }
  },
  { immediate: true },
)

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function applyRouteSellingMode(value: SellingMode | null) {
  if (value === 'limited') {
    void router.replace({ path: '/api-market/quota/new' })
    return
  }
  sellingMode.value = value
  currentStep.value = 1
  completedSteps.value = []
  setErrors({})
  if (!value) return
  form.billingMode = value === 'package' ? 'fixed_package' : 'metered_credit'
  if (value === 'package' && !form.packages.length) {
    const modelIds = form.selectedModels.filter(item => item.enabled).map(item => item.modelId)
    form.packages.push(createDefaultApiServicePackage(modelIds))
  }
}

watch(
  () => [route.query.mode, route.query.after],
  () => applyRouteSellingMode(apiPublishModeFromQuery(route.query.mode, route.query.after)),
  { immediate: true },
)

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
	ownerContactMethods: 3,
  distributionSystem: 2,
  probeConnection: 2,
  defaultMultiplier: 2,
  providerCategory: 2,
  cnyPerUsdCredit: 1,
  selectedModels: 2,
  availableCreditUsd: 1,
  quotaExpiresAt: 1,
  quotaUsagePolicy: 1,
  packages: 1,
  paymentWindowMinutes: 3,
  paymentOptions: 3,
  accountPool: 3,
  refundCommitment: 3,
  performance: 3,
  merchantNote: 3,
  sensitive: 3,
}

const limitedFieldSteps: Record<Field, ApiServicePublishStep> = {
  merchantIdentity: 2,
  merchantDisplayName: 2,
	ownerContactMethods: 2,
  distributionSystem: 2,
  probeConnection: 2,
  defaultMultiplier: 2,
  providerCategory: 2,
  cnyPerUsdCredit: 2,
  selectedModels: 2,
  availableCreditUsd: 2,
  quotaExpiresAt: 2,
  quotaUsagePolicy: 2,
  packages: 2,
  paymentWindowMinutes: 2,
  paymentOptions: 2,
  accountPool: 2,
  refundCommitment: 2,
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
  if (!form.ownerContactMethodId) next.ownerContactMethods = '请选择一个有效的交易联系方式。'
  if (form.merchantIdentityMode === 'store_alias') {
    if (!merchantDisplayName) next.merchantDisplayName = profileLoading.value ? '正在读取个人资料显示名称。' : '请先到个人中心设置显示名称。'
    else if (displayNameLength(merchantDisplayName) > 32) next.merchantDisplayName = '商家展示名最多 32 个字符，请到个人中心调整。'
    else if (hasContactLikeText(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含联系方式、链接或 linux.do 用户名，请到个人中心调整。'
    else if (hasMisleadingMerchantName(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含官方、担保、兜底等误导词，请到个人中心调整。'
  }
  if (!['sub2api', 'other'].includes(form.distributionSystem)) next.distributionSystem = '请选择接入类型。'
  if (!form.probeConnectionId) next.probeConnection = '请选择已验证且启用的探针连接。'
  else if (!probeConnectionReady.value) next.probeConnection = '所选探针连接当前不可用，请重新验证、启用或选择其他连接。'
  if (!Number.isFinite(form.defaultMultiplier) || form.defaultMultiplier <= 0) {
    next.defaultMultiplier = '默认服务倍率必须大于 0。'
  }
  if (!form.providerCategory) next.providerCategory = '请选择模型大类。'
  if (form.billingMode === 'metered_credit') {
    if (!form.cnyPerUsdCredit || form.cnyPerUsdCredit < sub2ApiPricingPolicy.minimumCnyPerUsdCredit || form.cnyPerUsdCredit > sub2ApiPricingPolicy.maximumCnyPerUsdCredit) {
      next.cnyPerUsdCredit = '每 $1 美元额度售价必须大于 0。'
    }
    if (!form.availableCreditUsd || form.availableCreditUsd <= 0) next.availableCreditUsd = '可售美元额度必须大于 0。'
    const quotaExpiresAtISO = beijingDateTimeInputToISOString(form.quotaExpiresAt)
    if (!quotaExpiresAtISO) next.quotaExpiresAt = '请填写有效的额度有效至时间。'
    else if (new Date(quotaExpiresAtISO).getTime() <= Date.now()) next.quotaExpiresAt = '额度有效至时间必须晚于当前时间。'
    const quotaPolicyError = apiQuotaUsagePolicyInputError(form.quotaUsagePolicy)
    if (quotaPolicyError) next.quotaUsagePolicy = quotaPolicyError
  }
  if (!form.selectedModels.some(item => item.enabled)) next.selectedModels = '至少选择一个模型。'
  if (missingSelectedModels.value.length) next.selectedModels = '已选模型不在当前后端模型目录中，请重新选择。'
  if (incompatibleSelectedModels.value.length) next.selectedModels = '已选模型必须全部属于当前模型大类。'
  if (form.billingMode === 'fixed_package') {
    const selectedModelIds = new Set(form.selectedModels.filter(item => item.enabled).map(item => item.modelId))
    const packageIds = new Set<string>()
    if (!enabledPackages.value.length) next.packages = `至少启用一个${sellingModeLabels.package}。`
    for (const [index, item] of form.packages.entries()) {
      const label = `套餐 ${index + 1}`
      if (!item.id || packageIds.has(item.id)) next.packages = `${label} 的标识重复，请删除后重新添加。`
      packageIds.add(item.id)
      if (!item.enabled) continue
      if (!item.name.trim()) next.packages = `${label} 需要填写名称。`
      else if (!Number.isFinite(item.priceCny) || item.priceCny <= 0) next.packages = `${label} 的价格必须大于 0。`
      else if (!Number.isFinite(item.panelAllowance) || item.panelAllowance <= 0) next.packages = `${label} 的面板额度必须大于 0。`
      else if (![1, 3, 7, 30].includes(item.durationDays)) next.packages = `${label} 的有效期只能选 1、3、7 或 30 天。`
      else if (!Number.isInteger(item.stockTotal) || item.stockTotal < 0) next.packages = `${label} 的库存必须是大于等于 0 的整数。`
      else if (!item.modelCatalogIds.length) next.packages = `${label} 至少选择一个支持模型。`
      else if (item.modelCatalogIds.some(id => !selectedModelIds.has(id))) next.packages = `${label} 包含未在服务中启用的模型。`
      else {
        const quotaPolicyError = apiQuotaUsagePolicyInputError(item.quotaUsagePolicy)
        if (quotaPolicyError) next.packages = `${label}：${quotaPolicyError}`
      }
    }
  }
  if (!paymentWindowValid.value) next.paymentWindowMinutes = '买家确认付款窗口固定为 10 分钟。'
  if (!enabledPayments.value.length) {
    next.paymentOptions = '请至少启用一种收款方式。'
  } else {
    const missingOption = enabledPayments.value.find(option => !isApiPaymentOptionComplete(option))
    if (missingOption) next.paymentOptions = apiPaymentSettingsMissingReason(form)
  }
	if (!form.accountPoolType) next.accountPool = '请选择一个号池。'
	if (form.accountPoolType === 'custom') {
		const customNameLength = Array.from(form.accountPoolCustomName.trim()).length
		if (customNameLength < 2 || customNameLength > 40) next.accountPool = '其他号池名称需要填写 2-40 个字符。'
	}
	if (!form.warranty.mode) next.refundCommitment = '请选择无额外退款承诺或商户全额退款承诺。'
  if (!Number.isInteger(form.declaredMaxConcurrency) || form.declaredMaxConcurrency < 1 || form.declaredMaxConcurrency > 100000) {
		next.performance = '商户声明最大并发必须是 1-100000 的整数。'
  } else if (form.promptAuditEnabled === null) {
    next.performance = '请明确选择是否开启提示词审计。'
  }
  if (!form.merchantNote.trim()) next.merchantNote = '请填写备注信息。'
  if (form.merchantNote.length > 800) next.merchantNote = '备注信息最多 800 字。'
  if (containsSensitiveContent([
		form.merchantDisplayName,
		form.accountPoolCustomName,
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
    probeConnectionReady.value ? done('探针连接') : pending('探针连接'),
		form.ownerContactMethodId ? done('订单联系方式') : pending('订单联系方式'),
    form.distributionSystem === 'sub2api' || (Number.isFinite(form.defaultMultiplier) && form.defaultMultiplier > 0) ? done('服务倍率') : pending('服务倍率'),
  ]
  if (!isLimitedQuotaMode.value) {
    if (form.billingMode === 'fixed_package') {
      items.push(enabledPackages.value.length && enabledPackages.value.every(item => item.name.trim() && item.priceCny > 0 && item.panelAllowance > 0 && [1, 3, 7, 30].includes(item.durationDays) && Number.isInteger(item.stockTotal) && item.stockTotal >= 0 && item.modelCatalogIds.length && !apiQuotaUsagePolicyInputError(item.quotaUsagePolicy))
        ? done('套餐配置')
        : pending('套餐配置'))
    } else {
      items.push(
        form.cnyPerUsdCredit && form.cnyPerUsdCredit > 0 ? done('额度售价') : pending('额度售价'),
        form.availableCreditUsd && form.availableCreditUsd > 0 ? done('可售额度') : pending('可售额度'),
        beijingDateTimeInputToISOString(form.quotaExpiresAt) ? done('有效时间') : pending('有效时间'),
        !apiQuotaUsagePolicyInputError(form.quotaUsagePolicy) ? done('额度规则') : pending('额度规则'),
      )
    }
  }
  items.push(
		accountPaymentSettingsComplete.value && paymentSettingsComplete.value ? done('收款方式') : pending('收款方式'),
		form.accountPoolType && (form.accountPoolType !== 'custom' || Array.from(form.accountPoolCustomName.trim()).length >= 2) ? done('号池') : pending('号池'),
			form.warranty.mode ? done('退款承诺') : pending('退款承诺'),
    form.declaredMaxConcurrency > 0 && form.promptAuditEnabled !== null ? done('服务声明') : pending('服务声明'),
    form.providerCategory ? done('模型大类') : pending('模型大类'),
    incompatibleSelectedModels.value.length ? conflict('具体模型') : form.selectedModels.some(item => item.enabled) ? done('具体模型') : pending('具体模型'),
    form.merchantNote.trim() ? done('备注信息') : pending('备注信息'),
  )
  return items
})
const publishAssistant = computed(() => apiPublishAssistantSummary(completeness.value))
const activeDisputeCount = computed(() => commerceStatusQuery.data.value?.activeDisputeCount ?? 0)
const disputePublishBlocked = computed(() => (
	commerceStatusQuery.isLoading.value
	|| commerceStatusQuery.isError.value
	|| commerceStatusQuery.data.value?.level === 'account_limited'
))
const disputeRuleText = computed(() => {
	if (commerceStatusQuery.isLoading.value) return '正在检查当前账号的经营状态，检查完成前不能开启接单。'
	if (commerceStatusQuery.isError.value) return '暂时无法确认经营状态，请重试；编辑和保存草稿仍然可用。'
	if (commerceStatusQuery.data.value?.level === 'account_limited') return `当前有 ${commerceStatusQuery.data.value.activeBuyerCount} 位不同买家的活动纠纷或履行逾期，暂时不能开启新接单。`
	if (activeDisputeCount.value > 0) return `当前有 ${activeDisputeCount.value} 笔活动纠纷，但未达到账号限制阈值，不影响创建和提交新服务。`
	return '单笔普通纠纷只冻结对应订单；达到账号风险阈值后才暂停全部新接单。'
})

const risks = computed(() => {
  const rows: string[] = []
  rows.push(isLimitedQuotaMode.value
    ? '额度、总价、库存和失效时间将在下一步设置；倍率统一继承当前基础服务。'
    : 'API 细节和用量核对由双方站外确认，平台不保存凭据，也不提供实时校验。')
  if (incompatibleSelectedModels.value.length) rows.push('当前存在不属于所选模型大类的模型，需清空后才能提交。')
  return rows
})

const canSubmit = computed(() => completeness.value.every(item => item.status === 'done'))
const publishBlockReason = computed(() => {
	if (disputePublishBlocked.value) return disputeRuleText.value
	if (canSubmit.value) return ''
  const pendingItem = completeness.value.find(item => item.status !== 'done')
  if (pendingItem?.label === '收款方式') {
    if (!paymentWindowValid.value) return '买家确认付款窗口固定为 10 分钟。'
    if (!accountPaymentSettingsComplete.value || !enabledPayments.value.length) return '请先配置 API 收款设置，发布后才会进入公开服务列表。'
    return apiPaymentSettingsMissingReason(form) || '请补全已启用的收款方式。'
  }
  if (pendingItem?.label === '展示身份') {
    return profileLoading.value ? '正在读取个人资料显示名称。' : '请先到个人中心设置显示名称。'
  }
  if (pendingItem) return `请先补全：${pendingItem.label}。`
  return '请先补全发布配置。'
})
const paymentSummary = computed(() => {
  const labels = enabledPayments.value.map(option => paymentMethodLabels[option.paymentMethod])
  return labels.length ? `${labels.join(' / ')} · ${form.paymentWindowMinutes} 分钟确认` : '收款方式待配置'
})
const stepOneSummary = computed(() => {
  if (isLimitedQuotaMode.value) return '限量额度包 · 当前先配置可复用基础服务'
  if (isFixedPackageMode.value) {
    const totalStock = enabledPackages.value.reduce((sum, item) => sum + item.stockTotal, 0)
    return `${sellingModeLabels.package} · ${enabledPackages.value.length} 个套餐 · 总库存 ${totalStock}`
  }
  const expiry = form.quotaExpiresAt ? form.quotaExpiresAt.slice(0, 10).replaceAll('-', '/') : '待填写有效期'
  return `自选额度 · ¥${form.cnyPerUsdCredit ?? 0} / $1 · 可售 $${form.availableCreditUsd ?? 0} · 有效至 ${expiry}`
})
const stepTwoSummary = computed(() => {
  const multiplier = form.distributionSystem === 'sub2api' ? '1.00x' : `${form.defaultMultiplier.toFixed(2)}x`
  const modelCount = selectedModels.value.length
  const connection = selectedProbeConnection.value?.name ?? '待选择连接'
  return `${form.distributionSystem === 'sub2api' ? 'Sub2API' : '其他 API 接入'} · ${connection} · ${providerCategoryLabel(form.providerCategory)} · ${modelCount ? `${modelCount} 个模型` : '待选择模型'} · 统一倍率 ${multiplier}`
})
const stepThreeSummary = computed(() => `${paymentSummary.value} · ${form.merchantIdentityMode === 'store_alias' ? '商家展示名' : '公开个人身份'}`)
const primaryActionLabel = computed(() => {
  if (publishMutation.isPending.value) return '处理中'
  if (isLimitedQuotaMode.value) {
    return currentStep.value === 1 ? '继续：配置基础服务' : '保存基础服务，下一步设置额度包'
  }
  if (currentStep.value === 1) return '继续：设置接入与模型'
  if (currentStep.value === 2) return '继续：交易与服务'
  return isFixedPackageMode.value ? `发布${sellingModeLabels.package}` : `发布${sellingModeLabels.free}服务`
})
const actionHeading = computed(() => {
  if (isLimitedQuotaMode.value) return currentStep.value === 1 ? '下一步：配置基础服务' : '下一步：设置额度包'
  if (currentStep.value === 3) return canSubmit.value ? '可以发布' : '完成配置后发布'
  return publishSteps.value[currentStep.value - 1]?.title ?? '发布 API 额度'
})
const actionBlockReason = computed(() => {
  if (isLimitedQuotaMode.value && currentStep.value === 2) return publishBlockReason.value
  if (!isLimitedQuotaMode.value && currentStep.value === 3) return publishBlockReason.value
  return ''
})
const currentActionPublishes = computed(() => (
	(isLimitedQuotaMode.value && currentStep.value === 2)
	|| (!isLimitedQuotaMode.value && currentStep.value === 3)
))

const publishMutation = useMutation({
  mutationFn: () => {
    syncHiddenPublishFields()
    return submitApiService({
      ...form,
      generatedTitle: generatedTitle(form, catalogById.value),
      status: 'reviewing',
    })
  },
  async onSuccess() {
    formDirty.value = false
    trackAnalytics('api_service_publish_success', {
      source_route: analyticsSourceRoute(),
      provider_category: form.providerCategory,
      billing_mode: form.billingMode,
      delivery_mode: form.deliveryModes[0],
      minimum_purchase_cny: form.minimumPurchaseCny,
    })
    toast.success(isFixedPackageMode.value
      ? `${sellingModeLabels.package}已发布并开启接单。`
      : `${sellingModeLabels.free}服务已发布并开启接单。`)
    await invalidateApiServicePublishQueries()
    await router.replace('/my/api-services')
  },
  onError(error) {
    toast.error(error instanceof Error ? error.message : 'API 服务发布失败，请检查配置后重试。')
  },
})

async function invalidateApiServicePublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['api-services'] })
  await queryClient.invalidateQueries({ queryKey: apiMarketAvailabilityQueryKey })
  await queryClient.invalidateQueries({ queryKey: ['api-market'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['notifications'] })
}

async function chooseSellingMode(value: SellingMode) {
  if (value === 'limited') {
    await router.push('/api-market/quota/new')
    return
  }
  const { after: _after, ...query } = route.query
  await router.push({ query: { ...query, mode: value } })
}

async function returnToSellingModeSelector() {
  if (formDirty.value && !window.confirm('API 服务配置尚未发布，确认返回选择销售模式？')) return
  const { mode: _mode, after: _after, ...query } = route.query
  await router.push({ query })
}

function setDistribution(value: DistributionSystem) {
  form.distributionSystem = value
  form.deliveryModes = ['api_key_endpoint']
  if (value === 'sub2api') {
    form.defaultMultiplier = sub2ApiPricingPolicy.textModelMultiplier
    if (!form.distributionSystemNote.trim() || form.distributionSystemNote.includes('其他 API')) {
      form.distributionSystemNote = 'Sub2API 接入，额度和用量由双方站外确认。'
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

function removeModel(id: string) {
  const selection = form.selectedModels.find(item => item.modelId === id)
  if (selection) selection.enabled = false
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
  if (step < 1 || step > 3) return
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
  if (currentStep.value < 3) {
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
  if (!isLimitedQuotaMode.value && currentStep.value === 3) {
    publishService()
    return
  }
  continueWorkflow()
}

function publishService() {
	if (disputePublishBlocked.value) {
		toast.warning(disputeRuleText.value)
		return
	}
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
  form.accountPoolType = ''
  form.accountPoolCustomName = ''
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
    <template v-if="!sellingMode">
      <div class="py-3 sm:py-6">
        <div class="mb-6">
          <h1 class="text-xl font-semibold">发布 API 额度</h1>
          <p class="mt-1 text-sm text-muted-foreground">先选择销售模式，再配置对应的价格、额度和交付方式；存在未解决纠纷时不能发布或恢复额度。</p>
        </div>
        <SellingModeSelector @select="chooseSellingMode" />
      </div>
    </template>

    <template v-else>
      <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 class="text-xl font-semibold">发布 API 额度</h1>
          <p class="mt-1 max-w-3xl text-xs leading-5 text-muted-foreground sm:text-sm">
            {{ isLimitedQuotaMode
              ? '先配置可复用的 API 基础服务，下一步再设置额度包规格、价格、库存和放量时间。'
              : isFixedPackageMode
                ? `买家选择${sellingModeLabels.package}，订单保留套餐价格、额度、模型和有效期快照。`
                : '买家自定购买金额，系统按你的美元额度售价创建订单。' }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" @click="returnToSellingModeSelector"><ArrowLeft class="h-4 w-4" />更换销售模式</Button>
          <Button variant="outline" class="hidden min-[1241px]:inline-flex" @click="preview"><Eye class="h-4 w-4" />预览</Button>
        </div>
      </div>

      <SellerCommerceStatusPanel
        :status="commerceStatusQuery.data.value"
        :loading="commerceStatusQuery.isLoading.value"
        :error="commerceStatusQuery.isError.value"
        @retry="commerceStatusQuery.refetch()"
      />

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
          :title="isLimitedQuotaMode ? '销售模式' : `配置${sellingModeLabels[isFixedPackageMode ? 'package' : 'free']}`"
          :description="isLimitedQuotaMode ? '已选择限量额度包。' : isFixedPackageMode ? '设置买家可选择的固定规格、价格与库存。' : '设置美元额度售价、可售额度和有效期。'"
          :status="publishStepStatus(1, currentStep, completedSteps)"
          :summary="stepOneSummary"
          @edit="selectStep"
        >
          <div class="space-y-3">
            <div class="flex gap-2 rounded-md border border-primary/15 bg-primary/5 px-3 py-2 text-xs leading-5 text-foreground">
              <Info class="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />
              <div class="min-w-0">
                <span class="font-semibold">买家流程：</span>
                <span>{{ isLimitedQuotaMode || isFixedPackageMode ? '选择额度包 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证' : '填写购买金额 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证' }}</span>
                <span class="ml-2 text-muted-foreground">平台记录订单，不代收款。</span>
              </div>
            </div>
            <div v-if="isLimitedQuotaMode" class="flex gap-2 rounded-md border border-orange-200 bg-orange-50/60 px-3 py-2 text-xs leading-5 text-orange-950">
              <PackageCheck class="mt-0.5 h-4 w-4 shrink-0 text-orange-600" />
              <div><span class="font-semibold">下一步设置额度包：</span><span class="text-orange-900/75">美元额度、总价、库存、放量和失效时间；倍率沿用基础服务。</span></div>
            </div>
            <FixedPackageSection v-else-if="isFixedPackageMode" :form="form" :errors="errors" />
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
            <ApiAccessSourceSection :form="form" :errors="errors" :selling-mode="editorSellingMode" @set-distribution="setDistribution" @set-default-multiplier="setDefaultMultiplier" />
            <ProbeConnectionSection
              v-model="form.probeConnectionId"
              :connections="probeConnections"
              :loading="probeConnectionsQuery.isLoading.value"
              :error="probeConnectionError"
              :field-error="errors.probeConnection"
              @create="probeConnectionDialogOpen = true"
              @refresh="probeConnectionsQuery.refetch()"
            />
            <AccountPaymentSummarySection
              v-if="isLimitedQuotaMode"
              :form="form"
              :settings="accountPaymentSettingsValue"
              :loading="paymentSettingsLoading"
              @edit="paymentSettingsDialogOpen = true"
            />
            <ProviderCategorySelector :model-value="form.providerCategory" :selected-count="selectedModels.length" :catalog="catalog" @update:model-value="requestProviderCategory" />
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
                <SelectedModelsPricingTable
                  v-if="!isLimitedQuotaMode && !catalogLoading"
                  class="mt-4"
                  :form="form"
                  :catalog-by-id="catalogById"
                  @remove-model="removeModel"
                />
              </div>
            </Card>
            <template v-if="isLimitedQuotaMode">
              <MerchantContactMethodsSection
                :form="form"
                :error="errors.ownerContactMethods"
              />
              <MerchantNoteSection :form="form" :errors="errors" :provider-category="form.providerCategory" />
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
            <AccountPaymentSummarySection
              :form="form"
              :settings="accountPaymentSettingsValue"
              :loading="paymentSettingsLoading"
              @edit="paymentSettingsDialogOpen = true"
            />
            <MerchantContactMethodsSection
              :form="form"
              :error="errors.ownerContactMethods"
            />
            <MerchantNoteSection :form="form" :errors="errors" :provider-category="form.providerCategory" />
            <MerchantIdentitySection :form="form" :profile-loading="profileLoading" :display-name-status="merchantDisplayNameStatus" :error="errors.merchantDisplayName" @set-store-alias-visible="setStoreAliasVisible" />
          </div>
        </PublishStepSection>

        </section>

        <ResponsivePublishPreview v-model:open="previewOpen" :title="isLimitedQuotaMode ? `${sellingModeLabels.limited}基础服务预览` : `${sellingModeLabels[isFixedPackageMode ? 'package' : 'free']}预览`" :description="isLimitedQuotaMode ? '额度包价格、库存和时间将在下一步设置。' : '根据当前表单实时生成。'">
          <ApiServicePublishPreview :form="form" :catalog-by-id="catalogById" :risks="risks" :quota-for-minimum-purchase="quotaForMinimumPurchase" :selling-mode="editorSellingMode" preview-only />
        </ResponsivePublishPreview>
      </div>

      <div class="sticky bottom-0 z-30 -mx-4 border-t border-border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur md:mx-0 md:rounded-lg md:border md:bg-card/95">
        <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
          <div class="hidden md:block"><div class="font-semibold">{{ actionHeading }}</div><p class="text-xs text-muted-foreground">发布必填 {{ publishAssistant.doneCount }} / {{ publishAssistant.totalCount }} · {{ publishAssistant.topPendingText }}</p></div>
          <div class="grid grid-cols-2 gap-1.5 md:flex md:shrink-0 md:items-center md:gap-3">
            <Button v-if="currentStep > 1" variant="outline" :disabled="publishMutation.isPending.value" @click="goBack"><ArrowLeft class="h-4 w-4" />上一步</Button>
            <Button v-if="!isLimitedQuotaMode && currentStep === 3" type="button" variant="outline" class="min-[1241px]:hidden" @click="preview"><Eye class="h-4 w-4" />预览</Button>
            <Button class="col-span-2 md:col-span-1" :disabled="publishMutation.isPending.value || (currentActionPublishes && disputePublishBlocked)" @click="runPrimaryAction">
              <Send v-if="!isLimitedQuotaMode && currentStep === 3" class="h-4 w-4" />
              <ArrowRight v-else class="h-4 w-4" />
              {{ primaryActionLabel }}
            </Button>
            <p v-if="actionBlockReason" class="col-span-2 line-clamp-2 max-w-sm text-xs leading-5 text-warning md:text-right">{{ actionBlockReason }}</p>
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
    </template>

    <ApiPaymentSettingsDialog
      v-model:open="paymentSettingsDialogOpen"
      :settings="accountPaymentSettingsValue"
    />
    <ApiProbeConnectionDialog
      v-model:open="probeConnectionDialogOpen"
      :connections="probeConnections"
      require-enabled
      @saved="selectProbeConnection"
      @reuse="selectProbeConnection"
    />
  </div>
</template>
