<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  CalendarClock,
  Clock3,
  Eye,
  FileKey2,
  PackageCheck,
  Plus,
  Send,
  Server,
  Store,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AccountPaymentSummarySection from '@/components/api-service-publish/AccountPaymentSummarySection.vue'
import ApiPaymentSettingsDialog from '@/components/contact-payment/ApiPaymentSettingsDialog.vue'
import ApiAccessSourceSection from '@/components/api-service-publish/ApiAccessSourceSection.vue'
import MerchantNoteSection from '@/components/api-service-publish/MerchantNoteSection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import PublishStepSection from '@/components/api-service-publish/PublishStepSection.vue'
import PublishWorkflowStepper from '@/components/api-service-publish/PublishWorkflowStepper.vue'
import ResponsivePublishPreview from '@/components/api-service-publish/ResponsivePublishPreview.vue'
import { completePublishStep, publishStepStatus } from '@/components/api-service-publish/publishWorkflow'
import ApiQuotaRushPublishPreview from '@/components/api-quota/ApiQuotaRushPublishPreview.vue'
import ApiQuotaPolicyFields from '@/components/api-market/ApiQuotaPolicyFields.vue'
import type {
  ApiProviderCategory,
  ApiServicePublishForm,
  DistributionSystem,
} from '@/components/api-service-publish/types'
import { toggleSelectedModel } from '@/components/api-service-publish/modelSelection'
import {
  applySimplifiedApiQuotaDefaults,
  createDefaultPaymentOptions,
  defaultPaymentWindowMinutes,
  generatedTitle,
  merchantNoteTemplate,
  modelProviderCategory,
  selectedCatalogItems,
} from '@/components/api-service-publish/utils'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  getApiMerchantDisplayName,
  submitApiService,
  type ApiOrderDeliveryKind,
  type ApiQuotaDeliveryMode,
  type ApiQuotaSourceType,
  type ApiQuotaSystemSaleSlot,
} from '@/lib/api'
import {
  beijingDateTimeInputToISOString,
  defaultQuotaExpiresAtInput,
  formatBeijingDateTimeInput,
} from '@/lib/apiQuotaExpiration'
import {
  cloneApiPaymentAccountSettings,
  isApiPaymentAccountSettingsComplete,
} from '@/lib/apiPaymentSettings'
import { formatDecimal, normalizeDecimal } from '@/lib/decimal'
import { backendErrorMessage } from '@/lib/backendClient'
import { apiQuotaUsagePolicyInputError, defaultApiQuotaUsagePolicyInput } from '@/lib/apiQuotaPolicy'
import {
  useApiPaymentAccountSettingsQuery,
  useApiQuotaSaleSlots,
  useCreateApiQuotaRushOfferMutation,
  useModelCatalog,
  useMyApiServices,
  useMyProfileQuery,
} from '@/queries/useMarketQueries'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'

type ServiceMode = 'existing' | 'create'

const router = useRouter()
const route = useRoute()
const queryClient = useQueryClient()
const step = ref(1)
const completedSteps = ref<number[]>([])
const previewOpen = ref(false)
const paymentSettingsDialogOpen = ref(false)
const paymentPromptedForCreate = ref(false)
const formDirty = ref(false)
useUnsavedChangesGuard(formDirty, '限时额度包配置尚未发布，确认离开当前页面？')
const serviceMode = ref<ServiceMode>('existing')
const selectedServiceId = ref('')
const serviceError = ref('')
const rushError = ref('')
const slotError = ref('')
const selectedFile = ref<File | null>(null)
const selectedFileRows = ref(0)
const publishSteps = [
  { title: '选择服务', description: '选择或新建' },
  { title: '设置限时包', description: '额度与交付' },
  { title: '选择场次', description: '核对并发布' },
]

const myServicesQuery = useMyApiServices('all')
const slotQuery = useApiQuotaSaleSlots()
const { data: modelCatalog, isLoading: catalogLoading } = useModelCatalog()
const {
  data: myProfile,
  isLoading: profileLoading,
  isError: profileIsError,
  isSuccess: profileIsSuccess,
  error: profileError,
  refetch: refetchProfile,
} = useMyProfileQuery()
const {
  data: accountPaymentSettings,
  isLoading: paymentSettingsLoading,
  isSuccess: paymentSettingsSuccess,
} = useApiPaymentAccountSettingsQuery()
const createRushMutation = useCreateApiQuotaRushOfferMutation()
const isCopyDraft = route.query.copy === '1'

function copiedQueryValue(key: string) {
  const value = route.query[key]
  return typeof value === 'string' ? value : ''
}

const eligibleServices = computed(() => (myServicesQuery.data.value ?? []).filter(service =>
  service.state === 'online' && service.online && service.publiclyOrderable,
))
const requestedServiceId = computed(() => typeof route.query.serviceId === 'string' ? route.query.serviceId : '')
const selectedService = computed(() => eligibleServices.value.find(service => service.id === selectedServiceId.value))
const requestedServiceUnavailable = computed(() =>
  Boolean(requestedServiceId.value)
  && myServicesQuery.isSuccess.value
  && !eligibleServices.value.some(service => service.id === requestedServiceId.value),
)
const openSlots = computed(() => (slotQuery.data.value?.items ?? []).filter(slot => slot.state === 'registration_open'))
const selectedSlot = computed(() => openSlots.value.find(slot => slot.key === rush.slotKey))

watch([eligibleServices, requestedServiceId, () => myServicesQuery.isSuccess.value], ([rows, requestedId, loaded]) => {
  if (!loaded) return
  if (requestedId) {
    selectedServiceId.value = rows.some(service => service.id === requestedId) ? requestedId : ''
    serviceMode.value = 'existing'
    return
  }
  if (!rows.some(service => service.id === selectedServiceId.value)) {
    selectedServiceId.value = rows[0]?.id ?? ''
  }
  if (!rows.length) serviceMode.value = 'create'
}, { immediate: true })

const baseForm = reactive<ApiServicePublishForm>({
  merchantIdentityMode: 'public_profile',
  merchantDisplayName: '',
  distributionSystem: 'sub2api',
  distributionSystemNote: '',
  providerCategory: 'gpt',
  billingMode: 'metered_credit',
  deliveryModes: ['api_key_endpoint'],
  shortDescription: '建议首次小额测试',
  cnyPerUsdCredit: 0.8,
  manualBillingNote: '',
  defaultMultiplier: 1,
  selectedModels: [],
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
  maximumPurchaseCny: 300,
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  declaredTtftBand: '1_to_3s',
  declaredMaxConcurrency: 1,
  performanceConfirmedAt: formatBeijingDateTimeInput(new Date()),
  packages: [],
  validity: { mode: 'days', days: 30, startsAt: 'delivered_at' },
  usageVisibility: 'merchant_confirmed',
  accountPoolType: '',
  accountPoolCustomName: '',
  warranty: {
    mode: '',
  },
  merchantNote: merchantNoteTemplate,
})
applySimplifiedApiQuotaDefaults(baseForm)
const serviceDefaultMultiplier = computed(() => {
  const multiplier = selectedService.value?.defaultMultiplier ?? baseForm.defaultMultiplier
  return Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 1
})
const serviceDefaultMultiplierDecimal = computed(() => normalizeDecimal(serviceDefaultMultiplier.value, 4))

const baseErrors = reactive<Record<string, string>>({})
const catalog = computed(() => modelCatalog.value ?? [])
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const filteredCatalog = computed(() => catalog.value.filter(item => modelProviderCategory(item.provider) === baseForm.providerCategory))
const selectedModels = computed(() => selectedCatalogItems(baseForm, catalogById.value))
const accountSettingsValue = computed(() => accountPaymentSettings.value
  ? cloneApiPaymentAccountSettings(accountPaymentSettings.value)
  : { paymentWindowMinutes: defaultPaymentWindowMinutes, paymentOptions: createDefaultPaymentOptions(), updatedAt: '' })
const accountSettingsComplete = computed(() => isApiPaymentAccountSettingsComplete(accountSettingsValue.value))
const profileErrorMessage = computed(() =>
  backendErrorMessage(profileError.value, '个人资料暂时无法加载，请稍后重试。'),
)

watch(() => myProfile.value, profile => {
  baseForm.merchantDisplayName = profile?.displayName.trim() || profile?.username.trim() || ''
}, { immediate: true })

watch(accountSettingsValue, settings => {
  baseForm.paymentWindowMinutes = settings.paymentWindowMinutes
  baseForm.paymentOptions = settings.paymentOptions.map(option => ({ ...option }))
}, { immediate: true })

watch(
  [serviceMode, () => paymentSettingsLoading.value, () => paymentSettingsSuccess.value, accountSettingsComplete],
  ([mode, loading, loaded, complete]) => {
    if (mode !== 'create' || loading || !loaded || complete || paymentPromptedForCreate.value) return
    paymentPromptedForCreate.value = true
    paymentSettingsDialogOpen.value = true
  },
  { immediate: true },
)

watch([catalog, () => baseForm.providerCategory], () => {
  const compatible = baseForm.selectedModels.filter(item => {
    const model = catalogById.value.get(item.modelId)
    return item.enabled && modelProviderCategory(model?.provider ?? 'other') === baseForm.providerCategory
  })
  if (compatible.length) return
  const first = filteredCatalog.value[0]
  baseForm.selectedModels = first ? [{ modelId: first.id, enabled: true }] : []
}, { immediate: true })

const rush = reactive({
  sourceType: 'sub2api' as ApiQuotaSourceType,
  sourceLabel: '',
  name: '$50 限时开发额度',
  usdAllowance: '50',
  priceCny: '5.00',
  quotaUsagePolicy: defaultApiQuotaUsagePolicyInput(),
  copies: 10,
  deliveryMode: 'manual' as ApiQuotaDeliveryMode,
  deliveryEtaMinutes: 10,
  deliveryKind: 'api_key_endpoint' as ApiOrderDeliveryKind,
  slotKey: '',
  expiresAt: '',
  sourceConfirmedAt: new Date().toISOString(),
})

if (isCopyDraft) {
  rush.name = copiedQueryValue('name') || rush.name
  rush.usdAllowance = copiedQueryValue('usdAllowance') || rush.usdAllowance
  rush.priceCny = copiedQueryValue('priceCny') || rush.priceCny
  const copiedDeliveryMode = copiedQueryValue('deliveryMode')
  if (copiedDeliveryMode === 'manual' || copiedDeliveryMode === 'preimported') {
    rush.deliveryMode = copiedDeliveryMode
  }
  const copiedDeliveryEtaMinutes = Number(copiedQueryValue('deliveryEtaMinutes'))
  if (Number.isInteger(copiedDeliveryEtaMinutes) && copiedDeliveryEtaMinutes >= 1 && copiedDeliveryEtaMinutes <= 10) {
    rush.deliveryEtaMinutes = copiedDeliveryEtaMinutes
  }
}

watch(openSlots, slots => {
  if (!rush.slotKey && slots[0]) rush.slotKey = slots[0].key
}, { immediate: true })

watch(selectedSlot, slot => {
  if (!slot || rush.expiresAt) return
  setExpiryHours(24)
}, { immediate: true })

const groupedSlots = computed(() => {
  const groups = new Map<string, ApiQuotaSystemSaleSlot[]>()
  for (const slot of openSlots.value) {
    const key = slot.key.slice(0, 10)
    groups.set(key, [...(groups.get(key) ?? []), slot])
  }
  return [...groups.entries()].map(([date, slots]) => ({ date, slots }))
})

const expiryISO = computed(() => beijingDateTimeInputToISOString(rush.expiresAt))
const rushQuotaPolicyError = computed(() => apiQuotaUsagePolicyInputError(rush.quotaUsagePolicy) ?? '')
const minimumExpiry = computed(() => selectedSlot.value
  ? formatBeijingDateTimeInput(new Date(Date.parse(selectedSlot.value.endsAt) + 60 * 60 * 1000))
  : '')
const serviceStepSummary = computed(() => selectedService.value
  ? `${selectedService.value.title} · ${selectedService.value.models.slice(0, 3).join(' / ')}`
  : serviceMode.value === 'create' ? '正在新建 API 服务' : '待选择可接单服务')
const packageStepSummary = computed(() => `${rush.name || '限时额度包'} · $${formatDecimal(rush.usdAllowance || '0', 0, 6)} / ¥${formatDecimal(rush.priceCny || '0', 2, 2)} · ${rush.copies} 份 · ${rush.deliveryMode === 'manual' ? `手工 ≤ ${rush.deliveryEtaMinutes} 分钟` : '预导入凭据'}`)
const slotStepSummary = computed(() => selectedSlot.value
  ? `${formatSlotDate(selectedSlot.value.startsAt)} ${formatSlotTime(selectedSlot.value.startsAt)} · ${rush.expiresAt ? `失效于 ${rush.expiresAt.replace('T', ' ')}` : '待填写失效时间'}`
  : '待选择开放场次')
const selectedSlotLabel = computed(() => selectedSlot.value
  ? `${formatSlotDate(selectedSlot.value.startsAt)} ${formatSlotTime(selectedSlot.value.startsAt)}`
  : '')
const primaryActionLabel = computed(() => {
  if (createBaseServiceMutation.isPending.value) return '创建中...'
  if (createRushMutation.isPending.value) return '发布中...'
  if (step.value === 1 && serviceMode.value === 'create' && profileLoading.value) return '正在加载个人资料...'
  if (step.value === 1 && serviceMode.value === 'create' && profileIsError.value) return '请先重新加载个人资料'
  if (step.value === 1) return serviceMode.value === 'create' ? '新建服务并继续' : '使用当前服务继续'
  if (step.value === 2) return '继续：选择场次'
  return '确认发布'
})
const primaryActionDisabled = computed(() =>
  createBaseServiceMutation.isPending.value
  || createRushMutation.isPending.value
  || (step.value === 1 && serviceMode.value === 'create' && (profileLoading.value || profileIsError.value)),
)

function serviceShortId(serviceId: string) {
  const normalized = serviceId.trim()
  return normalized.length <= 10 ? normalized : normalized.slice(-8)
}

function openNewServiceForm() {
  serviceError.value = ''
  serviceMode.value = 'create'
  paymentPromptedForCreate.value = false
  if (paymentSettingsSuccess.value && !accountSettingsComplete.value) {
    paymentPromptedForCreate.value = true
    paymentSettingsDialogOpen.value = true
  }
}

function returnToServiceList() {
  serviceError.value = ''
  serviceMode.value = 'existing'
}

function handlePaymentSettingsSaved() {
  paymentPromptedForCreate.value = true
  delete baseErrors.paymentOptions
  if (serviceError.value === '请先设置收款方式。') serviceError.value = ''
}

const createBaseServiceMutation = useMutation({
  mutationFn: () => submitApiService({
    ...baseForm,
    generatedTitle: generatedTitle(baseForm, catalogById.value),
    status: 'reviewing',
  }),
  async onSuccess(service) {
    await queryClient.invalidateQueries({ queryKey: ['my-api-services'] })
    selectedServiceId.value = service.id
    serviceMode.value = 'existing'
    completedSteps.value = completePublishStep(completedSteps.value, 1)
    step.value = 2
    toast.success('API 服务已创建，继续设置限时包。')
    void focusStep(2)
  },
  onError(error) {
    serviceError.value = error instanceof Error ? error.message : 'API 服务创建失败，请稍后重试。'
  },
})

function setDistribution(value: DistributionSystem) {
  baseForm.distributionSystem = value
  baseForm.defaultMultiplier = value === 'sub2api' ? 1 : baseForm.defaultMultiplier
  applySimplifiedApiQuotaDefaults(baseForm)
}

function setDefaultMultiplier(value: string) {
  baseForm.defaultMultiplier = Number(value)
}

function setProviderCategory(value: ApiProviderCategory) {
  baseForm.providerCategory = value
  baseForm.selectedModels = []
}

function toggleModel(id: string) {
  baseForm.selectedModels = toggleSelectedModel(baseForm.selectedModels, id)
}

function validateBaseService() {
  for (const key of Object.keys(baseErrors)) delete baseErrors[key]
  if (profileLoading.value) {
    serviceError.value = '个人资料正在加载，请稍后再继续。'
    return false
  }
  if (profileIsError.value || !profileIsSuccess.value) {
    serviceError.value = profileErrorMessage.value
    return false
  }
  if (!baseForm.merchantDisplayName.trim()) baseErrors.merchantDisplayName = '请先设置个人资料显示名称。'
  if (!baseForm.selectedModels.some(item => item.enabled)) baseErrors.selectedModels = '至少选择一个模型。'
	if (!baseForm.accountPoolType) baseErrors.accountPool = '请选择一个号池。'
	if (baseForm.accountPoolType === 'custom') {
		const customNameLength = Array.from(baseForm.accountPoolCustomName.trim()).length
		if (customNameLength < 2 || customNameLength > 40) baseErrors.accountPool = '其他号池名称需要填写 2-40 个字符。'
	}
	if (!baseForm.warranty.mode) baseErrors.refundCommitment = '请选择是否提供商户全额退款承诺。'
  if (!paymentSettingsSuccess.value) {
    baseErrors.paymentOptions = '收款设置暂时无法加载，请稍后重试。'
  } else if (!accountSettingsComplete.value) {
    baseErrors.paymentOptions = '请先设置收款方式。'
    paymentPromptedForCreate.value = true
    paymentSettingsDialogOpen.value = true
  }
  if (!baseForm.merchantNote.trim()) baseErrors.merchantNote = '请填写服务备注。'
  serviceError.value = Object.values(baseErrors)[0] ?? ''
  return !serviceError.value
}

function continueFromService() {
  serviceError.value = ''
  if (serviceMode.value === 'create') {
    if (validateBaseService()) createBaseServiceMutation.mutate()
    return
  }
  if (!selectedService.value) {
    serviceError.value = '请选择一个已上线且可接单的 API 服务。'
    return
  }
  completedSteps.value = completePublishStep(completedSteps.value, 1)
  step.value = 2
  void focusStep(2)
}

function validateRush() {
  if (!rush.name.trim()) return '请填写限时包名称。'
  if (Number(rush.usdAllowance) <= 0) return '单份美元额度必须大于 0。'
  if (Number(rush.priceCny) <= 0) return '人民币总价必须大于 0。'
  if (rushQuotaPolicyError.value) return rushQuotaPolicyError.value
  if (!Number.isInteger(rush.copies) || rush.copies < 1 || rush.copies > 5000) return '份数必须是 1-5000 的整数。'
  if (rush.deliveryEtaMinutes < 1 || rush.deliveryEtaMinutes > 10) return '交付时限必须在 1-10 分钟之间。'
  if (rush.sourceType === 'other' && !rush.sourceLabel.trim()) return '其他来源需要填写来源说明。'
  if (rush.deliveryMode === 'preimported') {
    if (!selectedFile.value) return '请上传凭据 CSV。'
    if (selectedFileRows.value < rush.copies) return `凭据数量至少需要 ${rush.copies} 条。`
  }
  return ''
}

function continueFromPackage() {
  rushError.value = validateRush()
  if (!rushError.value) {
    completedSteps.value = completePublishStep(completedSteps.value, 2)
    step.value = 3
    void focusStep(3)
  }
}

async function selectCredentialFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  selectedFile.value = file
  selectedFileRows.value = 0
  if (!file) return
  const lines = (await file.text()).split(/\r?\n/).map(row => row.trim()).filter(Boolean)
  selectedFileRows.value = Math.max(lines.length - 1, 0)
}

function setDeliveryMode(value: unknown) {
  rush.deliveryMode = value === 'preimported' ? 'preimported' : 'manual'
  rush.deliveryEtaMinutes = rush.deliveryMode === 'manual' ? 10 : 2
  if (rush.deliveryMode === 'manual') {
    selectedFile.value = null
    selectedFileRows.value = 0
  }
}

function setExpiryHours(hours: number) {
  if (!selectedSlot.value) return
  rush.expiresAt = formatBeijingDateTimeInput(new Date(Date.parse(selectedSlot.value.endsAt) + hours * 60 * 60 * 1000))
}

function formatSlotDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: 'numeric',
    day: 'numeric',
    weekday: 'short',
  }).format(new Date(value))
}

function formatSlotTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
}

async function publishRushOffer() {
  if (!selectedService.value) {
    serviceError.value = '原服务已不可用，请重新选择一个已上线且可接单的 API 服务。'
    step.value = 1
    void focusStep(1)
    return
  }
  slotError.value = ''
  rushError.value = validateRush()
  if (rushError.value) {
    step.value = 2
    void focusStep(2)
    return
  }
  if (!selectedSlot.value) {
    slotError.value = '请选择仍可报名的平台场次。'
    return
  }
  if (!expiryISO.value || Date.parse(expiryISO.value) < Date.parse(selectedSlot.value.endsAt) + 60 * 60 * 1000) {
    slotError.value = '额度失效时间必须至少晚于场次结束 1 小时。'
    return
  }
  try {
    const publication = await createRushMutation.mutateAsync({
      apiServiceId: selectedService.value.id,
      sourceType: rush.sourceType,
      sourceLabel: rush.sourceType === 'other' ? rush.sourceLabel.trim() : undefined,
      name: rush.name.trim(),
      usdAllowance: rush.usdAllowance,
      priceCny: rush.priceCny,
      modelMultiplier: serviceDefaultMultiplierDecimal.value,
      quotaUsagePolicy: rush.quotaUsagePolicy,
      copies: rush.copies,
      deliveryMode: rush.deliveryMode,
      deliveryEtaMinutes: rush.deliveryEtaMinutes,
      slotKey: rush.slotKey,
      expiresAt: expiryISO.value,
      sourceConfirmedAt: rush.sourceConfirmedAt,
      deliveryKind: rush.deliveryMode === 'preimported' ? rush.deliveryKind : undefined,
      file: rush.deliveryMode === 'preimported' ? selectedFile.value ?? undefined : undefined,
    })
    formDirty.value = false
    toast.success('限时额度包已发布。')
    await router.replace(`/my/api-services/${selectedService.value.id}#quota-offers`)
    void publication
  } catch (error) {
    slotError.value = error instanceof Error ? error.message : '发布失败，请检查场次和表单后重试。'
  }
}

async function focusStep(targetStep: number) {
  await nextTick()
  const section = document.getElementById(`publish-step-${targetStep}`)
  section?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  const target = section?.querySelector<HTMLElement>('[aria-invalid="true"]')
    ?? section?.querySelector<HTMLElement>('input, textarea, button, [tabindex="0"]')
    ?? section?.querySelector<HTMLElement>('[data-publish-step-heading]')
  target?.focus({ preventScroll: true })
}

function selectStep(targetStep: number) {
  if (targetStep !== step.value && !completedSteps.value.includes(targetStep)) return
  step.value = targetStep
  serviceError.value = ''
  rushError.value = ''
  slotError.value = ''
  void focusStep(targetStep)
}

function goBack() {
  if (step.value <= 1) return
  step.value -= 1
  void focusStep(step.value)
}

function runPrimaryAction() {
  if (step.value === 1) {
    continueFromService()
    return
  }
  if (step.value === 2) {
    continueFromPackage()
    return
  }
  void publishRushOffer()
}

function preview() {
  if (window.matchMedia('(min-width: 1241px)').matches) {
    document.querySelector<HTMLElement>('.api-publish-responsive-preview')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  previewOpen.value = true
}
</script>

<template>
  <div class="api-publish-page space-y-5 pb-20" @input="formDirty = true" @change="formDirty = true">
    <header class="flex flex-col gap-2 border-b border-border pb-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div class="flex items-center gap-2 text-sm font-medium text-primary"><CalendarClock class="h-4 w-4" />限时额度包</div>
        <h1 class="mt-1 text-2xl font-semibold">发布到固定抢购场次</h1>
        <p class="mt-1 text-sm text-muted-foreground">选择服务、设置单份额度与库存，再选择北京时间 09:00、13:00 或 20:00 场次。</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" @click="preview"><Eye class="h-4 w-4" />预览</Button>
        <RouterLink to="/api-market"><Button variant="outline"><ArrowLeft class="h-4 w-4" />返回市场</Button></RouterLink>
      </div>
    </header>

    <PublishWorkflowStepper :steps="publishSteps" :current-step="step" :completed-steps="completedSteps" @select="selectStep" />

    <div class="api-publish-layout grid min-w-0 gap-3 lg:items-start">
      <section class="api-publish-editor min-w-0 space-y-3">
        <PublishStepSection :step="1" title="选择要发布额度的 API 服务" description="限时额度包会归属到当前选中的服务。" :status="publishStepStatus(1, step, completedSteps)" :summary="serviceStepSummary" @edit="selectStep">
          <div class="space-y-4">
            <template v-if="serviceMode === 'existing'">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h2 class="text-sm font-semibold">我的 API 服务</h2>
                  <p class="mt-0.5 text-xs text-muted-foreground">请选择这次限时额度包所属的服务。</p>
                </div>
                <Button type="button" size="sm" variant="outline" @click="openNewServiceForm">
                  <Plus class="h-4 w-4" />新建 API 服务
                </Button>
              </div>
              <ErrorState v-if="myServicesQuery.error.value" description="API 服务暂时无法加载。" @retry="myServicesQuery.refetch()" />
              <SkeletonBlock v-else-if="myServicesQuery.isLoading.value" :lines="5" />
              <Alert v-else-if="requestedServiceUnavailable" variant="destructive">
                <Server />
                <AlertTitle>指定的 API 服务不可用</AlertTitle>
                <AlertDescription>
                  该服务不存在、未上线或当前不可接单。请返回服务列表重新选择，不会自动改用其他服务。
                  <div class="mt-3">
                    <Button type="button" size="sm" variant="outline" @click="router.push({ path: '/my/api-services', query: { intent: 'quota' } })">重新选择 API 服务</Button>
                  </div>
                </AlertDescription>
              </Alert>
              <EmptyState v-else-if="eligibleServices.length === 0" title="暂无可用的 API 服务" description="新建一个 API 服务后，再为它发布限时额度包。" />
              <RadioGroup v-else v-model="selectedServiceId" class="grid max-h-72 gap-2 overflow-y-auto pr-1 md:grid-cols-2">
                <label v-for="service in eligibleServices" :key="service.id" class="flex min-h-24 cursor-pointer gap-3 rounded-md border border-border p-3 hover:border-primary/50 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                  <RadioGroupItem :value="service.id" class="mt-1" />
                  <span class="min-w-0 flex-1">
                    <span class="flex flex-wrap items-center gap-2"><strong class="break-words text-sm">{{ service.title }}</strong><Badge variant="verified">可接单</Badge></span>
                    <span class="mt-1 block text-xs text-muted-foreground">服务编号 {{ serviceShortId(service.id) }} · {{ getApiMerchantDisplayName(service) }} · {{ service.delivery }}</span>
                    <span class="mt-1.5 line-clamp-1 block text-xs text-muted-foreground">{{ service.models.slice(0, 3).join(' / ') }}</span>
                  </span>
                </label>
              </RadioGroup>
              <div v-if="selectedService" class="flex flex-col gap-2 rounded-md border border-primary/25 bg-primary/5 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between">
                <div class="min-w-0">
                  <div class="text-xs font-medium text-primary">当前服务</div>
                  <div class="mt-0.5 truncate text-sm font-semibold">{{ selectedService.title }}</div>
                </div>
                <div class="shrink-0 text-xs text-muted-foreground">服务编号 {{ serviceShortId(selectedService.id) }} · {{ selectedService.models.length }} 个模型</div>
              </div>
            </template>
            <div v-else class="space-y-3">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h2 class="text-sm font-semibold">新建 API 服务</h2>
                  <p class="mt-0.5 text-xs text-muted-foreground">完成服务接入和模型信息后，会自动选中新服务。</p>
                </div>
                <Button v-if="eligibleServices.length" type="button" size="sm" variant="outline" @click="returnToServiceList">
                  <ArrowLeft class="h-4 w-4" />返回选择服务
                </Button>
              </div>
              <ErrorState v-if="profileIsError" title="个人资料加载失败" :description="profileErrorMessage" @retry="refetchProfile()" />
              <SkeletonBlock v-else-if="profileLoading" :lines="6" />
              <template v-else>
                <Alert><Server /><AlertTitle>先完善 API 服务</AlertTitle><AlertDescription>这里设置接入方式、模型与账户收款资料；额度价格、份数和场次在后续步骤设置。</AlertDescription></Alert>
                <ApiAccessSourceSection :form="baseForm" :errors="baseErrors" selling-mode="limited" @set-distribution="setDistribution" @set-default-multiplier="setDefaultMultiplier" />
                <AccountPaymentSummarySection
                  :form="baseForm"
                  :settings="accountSettingsValue"
                  :loading="paymentSettingsLoading"
                  @edit="paymentSettingsDialogOpen = true"
                />
                <ProviderCategorySelector :model-value="baseForm.providerCategory" :selected-count="selectedModels.length" @update:model-value="setProviderCategory" />
                <Card class="api-publish-card"><div class="api-publish-card-header"><div class="flex items-start gap-2"><Bot class="mt-0.5 h-4 w-4 text-primary" /><div><h2>具体模型</h2><p>选择这个 API 服务支持的模型。</p></div></div></div><div class="api-publish-card-body"><div v-if="catalogLoading" class="text-sm text-muted-foreground">正在加载模型目录...</div><ModelMultiSelect v-else :form="baseForm" :provider-category="baseForm.providerCategory" :catalog="filteredCatalog" :errors="baseErrors" @toggle-model="toggleModel" /></div></Card>
                <MerchantNoteSection :form="baseForm" :errors="baseErrors" />
              </template>
            </div>
            <p v-if="serviceError" class="text-sm text-destructive" role="alert">{{ serviceError }}</p>
          </div>
        </PublishStepSection>

        <PublishStepSection :step="2" title="设置限时包" description="设置单份额度、总价、库存、来源和交付方式。" :status="publishStepStatus(2, step, completedSteps)" :summary="packageStepSummary" @edit="selectStep">
          <div class="space-y-4">
            <Card class="p-5">
          <div class="flex items-start gap-2"><PackageCheck class="mt-0.5 h-5 w-5 text-primary" /><div><h2 class="font-semibold">额度与价格</h2><p class="text-sm text-muted-foreground">一个限时包对应一个场次和一种固定规格。</p></div></div>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <label class="space-y-1.5 text-sm sm:col-span-2"><span class="font-medium">限时包名称</span><Input v-model="rush.name" maxlength="80" /></label>
            <label class="space-y-1.5 text-sm"><span class="font-medium">单份美元额度</span><Input v-model="rush.usdAllowance" inputmode="decimal" /></label>
            <label class="space-y-1.5 text-sm"><span class="font-medium">单份人民币总价</span><Input v-model="rush.priceCny" inputmode="decimal" /></label>
            <label class="space-y-1.5 text-sm"><span class="font-medium">计划份数</span><Input v-model.number="rush.copies" type="number" min="1" max="5000" /></label>
          </div>
          <div class="mt-4 border-t border-border pt-4">
            <ApiQuotaPolicyFields v-model="rush.quotaUsagePolicy" :error="rushQuotaPolicyError || undefined" />
          </div>
            </Card>
            <Card class="p-5">
          <div class="flex items-start gap-2"><Store class="mt-0.5 h-5 w-5 text-primary" /><div><h2 class="font-semibold">来源与交付</h2><p class="text-sm text-muted-foreground">默认由卖家在确认收款后手工交付。</p></div></div>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <label class="space-y-1.5 text-sm"><span class="font-medium">额度来源</span>
              <Select v-model="rush.sourceType"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>
                <SelectItem value="sub2api">Sub2API</SelectItem><SelectItem value="new_api_proxy">NewAPI</SelectItem><SelectItem value="self_hosted">自建中转</SelectItem><SelectItem value="other">其他</SelectItem>
              </SelectContent></Select>
            </label>
            <label v-if="rush.sourceType === 'other'" class="space-y-1.5 text-sm"><span class="font-medium">来源说明</span><Input v-model="rush.sourceLabel" /></label>
            <div class="space-y-1.5 text-sm sm:col-span-2"><span class="font-medium">交付方式</span>
              <RadioGroup :model-value="rush.deliveryMode" class="grid gap-3 sm:grid-cols-2" @update:model-value="setDeliveryMode">
                <label class="flex cursor-pointer gap-3 rounded-md border border-border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5"><RadioGroupItem value="manual" class="mt-1" /><span><strong>卖家手工交付</strong><span class="mt-1 block text-xs text-muted-foreground">确认收款后最长 10 分钟交付。</span></span></label>
                <label class="flex cursor-pointer gap-3 rounded-md border border-border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5"><RadioGroupItem value="preimported" class="mt-1" /><span><strong>预导入凭据</strong><span class="mt-1 block text-xs text-muted-foreground">确认收款后发放预导入凭据。</span></span></label>
              </RadioGroup>
            </div>
            <label class="space-y-1.5 text-sm"><span class="font-medium">最长交付分钟数</span><Input v-model.number="rush.deliveryEtaMinutes" type="number" min="1" max="10" /></label>
          </div>
          <div v-if="rush.deliveryMode === 'preimported'" class="mt-4 space-y-3 border-t border-border pt-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <label class="space-y-1.5 text-sm"><span class="font-medium">凭据类型</span>
                <Select v-model="rush.deliveryKind"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="api_key_endpoint">API Key 与请求地址</SelectItem><SelectItem value="login_account">面板登录账号</SelectItem></SelectContent></Select>
              </label>
              <label class="space-y-1.5 text-sm"><span class="font-medium">凭据 CSV</span><Input type="file" accept=".csv,text/csv" @change="selectCredentialFile" /></label>
            </div>
            <p class="text-xs text-muted-foreground"><FileKey2 class="mr-1 inline h-3.5 w-3.5" />已读取 {{ selectedFileRows }} 条，必须不少于 {{ rush.copies }} 条。页面不会保存或展示原始凭据。</p>
          </div>
            </Card>
            <p v-if="rushError" class="text-sm text-destructive" role="alert">{{ rushError }}</p>
          </div>
        </PublishStepSection>

        <PublishStepSection :step="3" title="选择场次" description="选择仍可报名的固定场次，核对绝对失效时间并发布。" :status="publishStepStatus(3, step, completedSteps)" :summary="slotStepSummary" @edit="selectStep">
          <div class="space-y-4">
            <ErrorState v-if="slotQuery.error.value" description="平台场次暂时无法加载。" @retry="slotQuery.refetch()" />
            <SkeletonBlock v-else-if="slotQuery.isLoading.value" :lines="7" />
            <EmptyState v-else-if="openSlots.length === 0" title="未来七天暂无开放场次" description="当前场次都已截止报名，请稍后刷新。" />
            <RadioGroup v-else v-model="rush.slotKey" class="space-y-4">
              <div v-for="group in groupedSlots" :key="group.date"><div class="mb-2 text-sm font-semibold">{{ formatSlotDate(group.slots[0].startsAt) }}</div><div class="grid gap-2 sm:grid-cols-3"><label v-for="slot in group.slots" :key="slot.key" class="flex cursor-pointer items-center gap-3 rounded-md border border-border px-3 py-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5"><RadioGroupItem :value="slot.key" /><span><strong class="font-mono text-base tabular-nums">{{ formatSlotTime(slot.startsAt) }}</strong><span class="block text-xs text-muted-foreground">持续 30 分钟</span></span></label></div></div>
            </RadioGroup>
            <Card class="p-5"><div class="flex items-start gap-2"><Clock3 class="mt-0.5 h-5 w-5 text-primary" /><div><h2 class="font-semibold">额度绝对失效时间</h2><p class="text-sm text-muted-foreground">必须至少晚于所选场次结束 1 小时。</p></div></div><div class="mt-4 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]"><Input v-model="rush.expiresAt" type="datetime-local" :min="minimumExpiry" /><div class="flex flex-wrap gap-2"><Button type="button" size="sm" variant="outline" @click="setExpiryHours(24)">24 小时后</Button><Button type="button" size="sm" variant="outline" @click="setExpiryHours(72)">3 天后</Button><Button type="button" size="sm" variant="outline" @click="setExpiryHours(168)">7 天后</Button></div></div></Card>
            <p v-if="slotError" class="text-sm text-destructive" role="alert">{{ slotError }}</p>
          </div>
        </PublishStepSection>
      </section>

      <ResponsivePublishPreview v-model:open="previewOpen" title="限时额度包预览" description="根据当前服务、额度包和场次实时生成。">
        <ApiQuotaRushPublishPreview :step="step" :service-title="selectedService?.title" :slot-label="selectedSlotLabel" :default-multiplier="serviceDefaultMultiplier" :draft="rush" />
      </ResponsivePublishPreview>
    </div>

    <div class="sticky bottom-0 z-30 -mx-4 border-t border-border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur md:mx-0 md:rounded-lg md:border md:bg-card/95">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div class="hidden sm:block"><div class="font-semibold">{{ publishSteps[step - 1]?.title }}</div><p class="text-xs text-muted-foreground">第 {{ step }} / 3 步 · 已填写内容会在返回修改时保留</p></div>
        <div class="grid gap-2 sm:flex sm:items-center">
          <Button v-if="step > 1" variant="outline" :disabled="createBaseServiceMutation.isPending.value || createRushMutation.isPending.value" @click="goBack"><ArrowLeft class="h-4 w-4" />上一步</Button>
          <Button :disabled="primaryActionDisabled" @click="runPrimaryAction"><Send v-if="step === 3" class="h-4 w-4" /><ArrowRight v-else class="h-4 w-4" />{{ primaryActionLabel }}</Button>
        </div>
      </div>
    </div>

    <ApiPaymentSettingsDialog
      v-model:open="paymentSettingsDialogOpen"
      :settings="accountSettingsValue"
      @saved="handlePaymentSettingsSaved"
    />
  </div>
</template>
