<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Eye, Loader2, LogIn, RefreshCw, Save, Send, ShieldCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import CarpoolBasicInfoSection from '@/components/carpool-publish/CarpoolBasicInfoSection.vue'
import CarpoolPublishAssistant from '@/components/carpool-publish/CarpoolPublishAssistant.vue'
import CarpoolPublishPreview from '@/components/carpool-publish/CarpoolPublishPreview.vue'
import CarpoolRulesEditor from '@/components/carpool-publish/CarpoolRulesEditor.vue'
import CarpoolWarrantySelector from '@/components/carpool-publish/CarpoolWarrantySelector.vue'
import ChannelPaymentSection from '@/components/carpool-publish/ChannelPaymentSection.vue'
import SeatCapacityEditor from '@/components/carpool-publish/SeatCapacityEditor.vue'
import type {
  CarpoolProductCatalogItem,
  CarpoolPublishForm,
  CompletenessItem,
  PublishDefaultItem,
  PublishFieldState,
  PublishTask,
  PublishTaskKey,
  TrustItem,
} from '@/components/carpool-publish/types'
import {
  accessArrangementComplete,
  availableSeats,
  buildCarpoolShareText,
  canBuildCarpoolShareText,
  canPublishProduct,
  distributionFieldsComplete,
  hasForbiddenCredentialSharingText,
  regionDisplayName,
  requiresSubscriptionRiskAck,
  warrantyComplete,
} from '@/components/carpool-publish/utils'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { BackendProblemError, shouldUseRealBackend, startOAuthLogin } from '@/lib/backendClient'
import { containsSensitiveContent, firstError, type FieldErrors } from '@/lib/formValidation'
import { getMyCarpoolForEdit, submitCarpool, updateMyCarpool } from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import {
  useCarpoolOpeningChannels,
  useCarpoolPaymentMethods,
  useCarpoolProductCatalog,
  useCarpoolRegions,
  useMyProfileQuery,
} from '@/queries/useMarketQueries'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'

type Field =
  | 'product'
  | 'region'
  | 'monthlyPriceCny'
  | 'dailyQuota'
  | 'weeklyQuota'
  | 'quotaReset'
  | 'connection'
  | 'seats'
  | 'openingChannelCode'
  | 'paymentMethodCode'
  | 'distribution'
  | 'accessArrangement'
  | 'warranty'
  | 'rulesNote'
  | 'sensitive'

const formDirty = ref(false)
useUnsavedChangesGuard(formDirty, '车源内容尚未保存，确认离开当前页面？')

const { data: productCatalog } = useCarpoolProductCatalog()
const { data: regions } = useCarpoolRegions()
const { data: openingChannels } = useCarpoolOpeningChannels()
const { data: paymentMethods } = useCarpoolPaymentMethods()
const profileQuery = useMyProfileQuery()
const profile = profileQuery.data
const queryClient = useQueryClient()
const route = useRoute()
const router = useRouter()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const isEditMode = computed(() => route.name === 'my-carpool-edit')
const editingId = computed(() => isEditMode.value ? String(route.params.id ?? '') : '')
const editQuery = useQuery({
	queryKey: computed(() => ['my-carpools', 'detail', editingId.value]),
	queryFn: () => getMyCarpoolForEdit(editingId.value),
	enabled: computed(() => isEditMode.value && Boolean(editingId.value)),
	retry: false,
})

const submittedId = ref('')
const oauthPending = ref(false)
const hasTriedPublish = ref(false)
const mobileCheckOpen = ref(false)
const highlightedTaskKey = ref<string | null>(null)
const editInitialized = ref(false)
const hydratingEdit = ref(false)
const editVersion = ref(0)
const editOwnerContactMethodId = ref('')
const errors = reactive<FieldErrors<Field>>({})
const publishReturnTo = isEditMode.value ? route.fullPath : '/carpools/new'
const publishLoginRoute = { path: '/login', query: { returnTo: publishReturnTo } }
let highlightTimer: ReturnType<typeof setTimeout> | null = null

const form = reactive<CarpoolPublishForm>({
  productId: '',
  customProductName: null,
  regionCode: '',
  customRegionName: null,
  monthlyPriceCny: null,
  serviceMultiplier: 1,
  dailyQuotaAmount: null,
  weeklyQuotaAmount: null,
  followsOfficialQuotaReset: null,
  vpsRegion: '',
  supportsMainlandChinaDirectConnection: null,
  totalSeats: 5,
  occupiedSeats: 3,
  openingChannelCode: '',
  customOpeningChannel: '',
  paymentMethodCode: '',
  customPaymentMethod: '',
  distributionMethod: '',
  distributionMethodNote: '',
  providesAdminAccount: null,
  accessArrangementMode: 'provider_member_invitation',
  accessArrangementNote: '通过官方团队或 Business workspace 邀请成员席位，买家使用自己的账号接受邀请。',
  riskAcknowledged: false,
  policyVersion: null,
  riskNoticeCode: null,
  warranty: {
    mode: 'remaining_days_compensation',
    fixedWarrantyDays: null,
    compensationMethod: '按未使用天数补时或退还对应周期费用',
    exclusions: '',
  },
  rulesNote: '',
})

const catalog = computed(() => productCatalog.value ?? [])
const regionOptions = computed(() => regions.value ?? [])
const channelOptions = computed(() => openingChannels.value ?? [])
const paymentOptions = computed(() => paymentMethods.value ?? [])
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const regionsByCode = computed(() => new Map(regionOptions.value.map(item => [item.code, item])))
const openingChannelsByCode = computed(() => new Map(channelOptions.value.map(item => [item.code, item])))
const paymentMethodsByCode = computed(() => new Map(paymentOptions.value.map(item => [item.code, item])))
const selectedProductForValidation = computed(() => catalogById.value.get(form.productId) ?? null)
const finalRegionName = computed(() => regionDisplayName(form, regionsByCode.value))
const editableBackendStatus = computed(() => {
	const status = editQuery.data.value?.backendStatus
	return status === 'draft' || status === 'changes_requested'
})
const canAccessPublishForm = computed(() => Boolean(profile.value?.linuxDoBinding.bound) && (!isEditMode.value || editableBackendStatus.value))
const profileErrorMessage = computed(() => {
  const error = profileQuery.error.value
  return error instanceof Error ? error.message : '请先登录并完成 linux.do 身份绑定。'
})

watch(selectedProductForValidation, product => {
	if (!product) return
	if (hydratingEdit.value) return
  form.policyVersion = product.policyVersion
  form.riskNoticeCode = product.riskNoticeCode ?? null
  form.accessArrangementMode = product.accessMode
  form.accessArrangementNote = defaultAccessArrangementNote(product)
  form.riskAcknowledged = false
})

watch(() => editQuery.data.value, async detail => {
	if (!detail || editInitialized.value) return
	hydratingEdit.value = true
	editVersion.value = detail.version
	editOwnerContactMethodId.value = detail.ownerContactMethodId
	Object.assign(form, detail.payload, { warranty: { ...detail.payload.warranty } })
	submittedId.value = detail.id
	formDirty.value = false
	editInitialized.value = true
	await nextTick()
	hydratingEdit.value = false
}, { immediate: true })

const saveDraftMutation = useMutation({
	mutationFn: () => isEditMode.value
		? updateMyCarpool(editingId.value, toPayload('draft'), editVersion.value, editOwnerContactMethodId.value, false)
		: submitCarpool(toPayload('draft')),
	async onSuccess(result) {
		formDirty.value = false
		submittedId.value = String(result.id)
		toast.success(isEditMode.value ? '车源修改已保存。' : '车源草稿已保存。')
		await invalidateCarpoolPublishQueries()
		if (isEditMode.value) await router.replace('/my/carpools')
	},
	onError: handleCarpoolMutationError,
})

const submitReviewMutation = useMutation({
	mutationFn: () => isEditMode.value
		? updateMyCarpool(editingId.value, toPayload('reviewing'), editVersion.value, editOwnerContactMethodId.value, true)
		: submitCarpool(toPayload('reviewing')),
  async onSuccess(result) {
    formDirty.value = false
    submittedId.value = String(result.id)
    trackAnalytics('carpool_publish_success', {
      source_route: analyticsSourceRoute(),
      product: selectedProductForValidation.value?.categoryCode ?? form.productId,
      monthly_price_cny: form.monthlyPriceCny,
      seats: form.totalSeats,
      access_mode: form.accessArrangementMode,
      risk_ack_required: Boolean(form.riskNoticeCode),
      risk_notice: form.riskNoticeCode ?? 'none',
    })
		toast.success(isEditMode.value ? '车源修改已提交审核。' : '车源已提交。')
		await invalidateCarpoolPublishQueries()
		await router.replace('/my/carpools')
	},
	onError: handleCarpoolMutationError,
})

async function invalidateCarpoolPublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['carpools'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
	await queryClient.invalidateQueries({ queryKey: ['notifications'] })
	await queryClient.invalidateQueries({ queryKey: ['my-carpools'] })
}

async function handleCarpoolMutationError(error: unknown) {
	if (isEditMode.value && error instanceof BackendProblemError && (error.status === 412 || error.code === 'VERSION_CONFLICT')) {
		toast.warning('车源已被更新，已重新读取最新版本，请确认后再次保存。')
		formDirty.value = false
		editInitialized.value = false
		await editQuery.refetch()
		return
	}
	if (error instanceof BackendProblemError && error.fieldErrors.length) {
		const fieldMap: Record<string, Field> = {
			productPlanId: 'product', regionCode: 'region', regionName: 'region', priceMonthlyCny: 'monthlyPriceCny',
			dailyQuotaAmount: 'dailyQuota', weeklyQuotaAmount: 'weeklyQuota', followsOfficialQuotaReset: 'quotaReset',
			vpsRegion: 'connection', supportsMainlandChinaDirectConnection: 'connection', buyerSeatCapacity: 'seats',
			activeBuyerMembers: 'seats', openingChannelCode: 'openingChannelCode', paymentMethodCode: 'paymentMethodCode',
			distributionMethod: 'distribution', distributionMethodNote: 'distribution', providesAdminAccount: 'distribution',
			accessArrangement: 'accessArrangement', summary: 'rulesNote',
		}
		const next: FieldErrors<Field> = {}
		for (const issue of error.fieldErrors) {
			const field = issue.field ? fieldMap[issue.field] : undefined
			if (field && issue.message) next[field] = issue.message
		}
		if (Object.keys(next).length) setErrors(next)
	}
	toast.error(error instanceof Error ? error.message : '车源保存失败。')
}

async function startLinuxDoPublishAuth() {
  if (oauthPending.value) return
  oauthPending.value = true
  try {
    const { authorizationUrl } = await startOAuthLogin(publishReturnTo)
    window.location.assign(authorizationUrl)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '启动 linux.do 登录失败')
  } finally {
    oauthPending.value = false
  }
}

function ensurePublishAccess() {
	if (canAccessPublishForm.value) return true
	if (isEditMode.value && editQuery.data.value && !editableBackendStatus.value) {
		toast.warning('当前车源状态不可编辑。')
		return false
	}
  toast.warning('完成 linux.do 身份绑定后才能发布车源。')
  return false
}

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function clearError(key: Field) {
  if (errors[key]) delete errors[key]
}

const publishTaskFieldIds: Record<PublishTaskKey, string> = {
  product: 'carpool-task-product',
  region: 'carpool-task-region',
  monthlyPrice: 'carpool-task-monthlyPrice',
  dailyQuota: 'carpool-task-dailyQuota',
  weeklyQuota: 'carpool-task-weeklyQuota',
  quotaReset: 'carpool-task-quotaReset',
  connection: 'carpool-task-vpsRegion',
  openingChannel: 'carpool-task-openingChannel',
  paymentMethods: 'carpool-task-paymentMethods',
  distribution: 'carpool-task-distribution',
  riskAcknowledgement: 'carpool-task-riskAcknowledgement',
  rulesNote: 'carpool-task-rulesNote',
}

function fieldErrorForTask(key: PublishTaskKey) {
  if (!hasTriedPublish.value) return ''
  if (key === 'product') return errors.product ?? ''
  if (key === 'region') return errors.region ?? ''
  if (key === 'monthlyPrice') return errors.monthlyPriceCny ?? ''
  if (key === 'dailyQuota') return errors.dailyQuota ?? ''
  if (key === 'weeklyQuota') return errors.weeklyQuota ?? ''
  if (key === 'quotaReset') return errors.quotaReset ?? ''
  if (key === 'connection') return errors.connection ?? ''
  if (key === 'openingChannel') return errors.openingChannelCode ?? ''
  if (key === 'paymentMethods') return errors.paymentMethodCode ?? ''
  if (key === 'distribution') return errors.distribution ?? ''
  if (key === 'riskAcknowledgement') return errors.accessArrangement ?? ''
  if (key === 'rulesNote') return errors.rulesNote ?? ''
  return ''
}

function taskComplete(key: PublishTaskKey) {
  if (key === 'product') return Boolean(form.productId && (form.productId !== 'other-custom' || form.customProductName?.trim()))
  if (key === 'region') return Boolean(form.regionCode && finalRegionName.value)
  if (key === 'monthlyPrice') return Boolean(form.monthlyPriceCny && form.monthlyPriceCny > 0)
  if (key === 'dailyQuota') return Boolean(form.dailyQuotaAmount && form.dailyQuotaAmount > 0)
  if (key === 'weeklyQuota') return Boolean(form.weeklyQuotaAmount && form.weeklyQuotaAmount > 0)
  if (key === 'quotaReset') return form.followsOfficialQuotaReset !== null
  if (key === 'connection') return Boolean(form.vpsRegion.trim() && form.supportsMainlandChinaDirectConnection !== null)
  if (key === 'openingChannel') return Boolean(form.openingChannelCode && (form.openingChannelCode !== 'other' || form.customOpeningChannel.trim()))
  if (key === 'paymentMethods') return Boolean(form.paymentMethodCode && (form.paymentMethodCode !== 'other' || form.customPaymentMethod.trim()))
  if (key === 'distribution') return distributionFieldsComplete(form)
  if (key === 'riskAcknowledgement') return form.riskAcknowledged
  if (key === 'rulesNote') return Boolean(form.rulesNote.trim())
  return false
}

const publishTasks = computed<PublishTask[]>(() => [
  {
    key: 'product',
    label: '选择产品',
    shortLabel: '产品',
    section: 'basic',
    fieldId: publishTaskFieldIds.product,
    description: '车源基础信息',
    complete: taskComplete('product'),
    error: fieldErrorForTask('product'),
  },
  {
    key: 'region',
    label: '选择开通区',
    shortLabel: '开通区',
    section: 'basic',
    fieldId: publishTaskFieldIds.region,
    description: '车源基础信息',
    complete: taskComplete('region'),
    error: fieldErrorForTask('region'),
  },
  {
    key: 'monthlyPrice',
    label: '填写月费',
    shortLabel: '月费',
    section: 'basic',
    fieldId: publishTaskFieldIds.monthlyPrice,
    description: '车源基础信息',
    complete: taskComplete('monthlyPrice'),
    error: fieldErrorForTask('monthlyPrice'),
  },
  {
    key: 'dailyQuota',
    label: `填写每天${selectedProductForValidation.value?.quotaLabel || '额度'}`,
    shortLabel: `每天${selectedProductForValidation.value?.quotaLabel || '额度'}`,
    section: 'basic',
    fieldId: publishTaskFieldIds.dailyQuota,
    description: '额度与重置',
    complete: taskComplete('dailyQuota'),
    error: fieldErrorForTask('dailyQuota'),
  },
  {
    key: 'weeklyQuota',
    label: `填写每周${selectedProductForValidation.value?.quotaLabel || '额度'}`,
    shortLabel: `每周${selectedProductForValidation.value?.quotaLabel || '额度'}`,
    section: 'basic',
    fieldId: publishTaskFieldIds.weeklyQuota,
    description: '车源基础信息',
    complete: taskComplete('weeklyQuota'),
    error: fieldErrorForTask('weeklyQuota'),
  },
  {
    key: 'quotaReset',
    label: '确认额度重置方式',
    shortLabel: '额度重置',
    section: 'basic',
    fieldId: publishTaskFieldIds.quotaReset,
    description: '额度与重置',
    complete: taskComplete('quotaReset'),
    error: fieldErrorForTask('quotaReset'),
  },
  {
    key: 'connection',
    label: '填写 VPS 与直连信息',
    shortLabel: '网络接入',
    section: 'basic',
    fieldId: publishTaskFieldIds.connection,
    description: 'VPS 区域与国内直连',
    complete: taskComplete('connection'),
    error: fieldErrorForTask('connection'),
  },
  {
    key: 'openingChannel',
    label: '选择开通渠道',
    shortLabel: '开通渠道',
    section: 'activationPayment',
    fieldId: publishTaskFieldIds.openingChannel,
    description: '开通与付款方式',
    complete: taskComplete('openingChannel'),
    error: fieldErrorForTask('openingChannel'),
  },
  {
    key: 'paymentMethods',
    label: '选择一种付款方式',
    shortLabel: '付款方式',
    section: 'activationPayment',
    fieldId: publishTaskFieldIds.paymentMethods,
    description: '开通与付款方式',
    complete: taskComplete('paymentMethods'),
    error: fieldErrorForTask('paymentMethods'),
  },
  {
    key: 'distribution',
    label: '确认分发方式',
    shortLabel: '分发方式',
    section: 'basic',
    fieldId: publishTaskFieldIds.distribution,
    description: '分发方式与管理员账号',
    complete: taskComplete('distribution'),
    error: fieldErrorForTask('distribution'),
  },
  ...(requiresSubscriptionRiskAck(selectedProductForValidation.value, form) ? [{
    key: 'riskAcknowledgement' as const,
    label: '确认发布边界',
    shortLabel: '发布边界确认',
    section: 'basic' as const,
    fieldId: publishTaskFieldIds.riskAcknowledgement,
    description: '确认平台不处理或交付账号凭据',
    complete: taskComplete('riskAcknowledgement'),
    error: fieldErrorForTask('riskAcknowledgement'),
  }] : []),
  {
    key: 'rulesNote',
    label: '补充买家须知',
    shortLabel: '买家须知',
    section: 'rules',
    fieldId: publishTaskFieldIds.rulesNote,
    description: '使用规则',
    complete: taskComplete('rulesNote'),
    error: fieldErrorForTask('rulesNote'),
  },
])

const completedPublishTasks = computed(() => publishTasks.value.filter(item => item.complete))
const pendingPublishTasks = computed(() => publishTasks.value.filter(item => !item.complete))
const publishProgressPercent = computed(() => Math.round((completedPublishTasks.value.length / publishTasks.value.length) * 100))
const errorSummaryText = computed(() => {
  if (!hasTriedPublish.value) return ''
  if (pendingPublishTasks.value.length) return `请补充：${pendingPublishTasks.value.map(item => item.shortLabel).join('、')}。`
  const blockingErrors = (Object.entries(errors) as Array<[Field, string]>)
    .filter(([key]) => key !== 'sensitive')
    .map(([, message]) => message)
    .filter(Boolean)
  if (blockingErrors.length) return blockingErrors.join(' ')
  return errors.sensitive ?? ''
})
const mobileStatusText = computed(() => {
  if (!pendingPublishTasks.value.length) return '发布必填项已完成'
  return `还差：${pendingPublishTasks.value.map(item => item.shortLabel).slice(0, 2).join('、')}`
})

function stateForTask(key: PublishTaskKey): PublishFieldState {
  const complete = taskComplete(key)
  if (complete) return 'complete'
  if (hasTriedPublish.value && fieldErrorForTask(key)) return 'error'
  return 'pendingRequired'
}

function stateForFullValidation(field: Field): PublishFieldState {
  if (field === 'seats') {
    if (hasTriedPublish.value && errors.seats) return 'error'
    return form.totalSeats >= 1 && form.totalSeats <= 20 && form.occupiedSeats >= 0 && form.occupiedSeats <= form.totalSeats ? 'defaulted' : 'pendingRequired'
  }
  if (field === 'accessArrangement') {
    if (hasTriedPublish.value && errors.accessArrangement) return 'error'
    return accessArrangementComplete(form, selectedProductForValidation.value) ? 'defaulted' : 'pendingRequired'
  }
  if (field === 'warranty') {
    if (hasTriedPublish.value && errors.warranty) return 'error'
    return warrantyComplete(form.warranty) ? 'defaulted' : 'pendingRequired'
  }
  return 'idle'
}

const basicFieldStates = computed<Partial<Record<string, PublishFieldState>>>(() => ({
  product: stateForTask('product'),
  region: stateForTask('region'),
  monthlyPrice: stateForTask('monthlyPrice'),
  dailyQuota: stateForTask('dailyQuota'),
  weeklyQuota: stateForTask('weeklyQuota'),
  quotaReset: stateForTask('quotaReset'),
  connection: stateForTask('connection'),
  distribution: stateForTask('distribution'),
}))

const channelPaymentFieldStates = computed<Partial<Record<string, PublishFieldState>>>(() => ({
  openingChannel: stateForTask('openingChannel'),
  paymentMethods: stateForTask('paymentMethods'),
}))

const defaultItems = computed<PublishDefaultItem[]>(() => [
  {
    key: 'seats',
    label: '名额设置已默认',
    description: `总 ${form.totalSeats} 人，已上车 ${form.occupiedSeats} 人。`,
    status: stateForFullValidation('seats'),
  },
  {
    key: 'accessArrangement',
    label: '访问安排自动生成',
    description: accessArrangementComplete(form, selectedProductForValidation.value) ? '将按所选套餐策略生成，不需要单独填写。' : '需要先完成风险边界确认。',
    status: stateForFullValidation('accessArrangement'),
  },
  {
    key: 'warranty',
    label: '车主承诺已默认',
    description: warrantyComplete(form.warranty) ? '可继续修改售后承诺。' : '需要补全售后承诺。',
    status: stateForFullValidation('warranty'),
  },
])

function taskFromErrorKey(key: Field): PublishTaskKey | null {
  if (key === 'product') return 'product'
  if (key === 'region') return 'region'
  if (key === 'monthlyPriceCny') return 'monthlyPrice'
  if (key === 'dailyQuota') return 'dailyQuota'
  if (key === 'weeklyQuota') return 'weeklyQuota'
  if (key === 'quotaReset') return 'quotaReset'
  if (key === 'connection') return 'connection'
  if (key === 'openingChannelCode') return 'openingChannel'
  if (key === 'paymentMethodCode') return 'paymentMethods'
  if (key === 'distribution') return 'distribution'
  if (key === 'rulesNote') return 'rulesNote'
  if (key === 'seats') return 'monthlyPrice'
  if (key === 'accessArrangement') {
    return requiresSubscriptionRiskAck(selectedProductForValidation.value, form)
      ? 'riskAcknowledgement'
      : 'product'
  }
  if (key === 'warranty') return 'rulesNote'
  if (key === 'sensitive') return 'rulesNote'
  return null
}

async function jumpToTask(key: PublishTaskKey | string) {
  await nextTick()
  const targetId = publishTaskFieldIds[key as PublishTaskKey]
  const target = targetId ? document.getElementById(targetId) : null
  if (!target) return
  target.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'center' })
  const focusable = target.querySelector<HTMLElement>('input, textarea, button, [tabindex]:not([tabindex="-1"])')
  focusable?.focus({ preventScroll: true })
  highlightedTaskKey.value = key
  if (highlightTimer) clearTimeout(highlightTimer)
  highlightTimer = setTimeout(() => {
    highlightedTaskKey.value = null
  }, 1200)
}

async function focusFirstInvalidTask() {
  const firstMissing = pendingPublishTasks.value[0]?.key
  const firstErrorKey = (Object.keys(errors) as Field[]).map(taskFromErrorKey).find(Boolean)
  await jumpToTask(firstMissing ?? firstErrorKey ?? 'product')
}

function isMobilePublishCheckViewport() {
  return window.matchMedia('(max-width: 639px)').matches
}

function defaultAccessArrangementNote(product: CarpoolProductCatalogItem) {
  if (product.accessMode === 'personal_account_cost_share') {
    return '个人订阅费用分摊，平台不保存、不交付任何密码、Session、Cookie 或 token。'
  }
  if (product.accessMode === 'provider_member_invitation') {
    return '通过成员邀请、团队席位或独立座位加入，买家使用自己的账号接受邀请。'
  }
  if (product.accessMode === 'owner_managed_access') {
    return '站外托管或中转安排由双方确认，平台不保存、不交付任何密码、管理员凭据、Session、Cookie 或 token。'
  }
  return '站外访问安排需由双方确认，平台不保存、不交付任何密码、Session、Cookie 或 token。'
}

watch(() => [form.productId, form.customProductName], () => {
  if (taskComplete('product')) clearError('product')
})

watch(() => form.regionCode, value => {
  if (value !== 'other') form.customRegionName = null
  if (taskComplete('region')) clearError('region')
})

watch(() => form.customRegionName, () => {
  if (taskComplete('region')) clearError('region')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => form.monthlyPriceCny, () => {
  if (taskComplete('monthlyPrice')) clearError('monthlyPriceCny')
})

watch(() => form.weeklyQuotaAmount, () => {
  if (taskComplete('weeklyQuota')) clearError('weeklyQuota')
})

watch(() => form.dailyQuotaAmount, () => {
  if (taskComplete('dailyQuota')) clearError('dailyQuota')
})

watch(() => form.followsOfficialQuotaReset, () => {
  if (taskComplete('quotaReset')) clearError('quotaReset')
})

watch(() => [form.vpsRegion, form.supportsMainlandChinaDirectConnection], () => {
  if (taskComplete('connection')) clearError('connection')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => [form.totalSeats, form.occupiedSeats], () => {
  if (form.totalSeats >= 1 && form.totalSeats <= 20 && form.occupiedSeats >= 0 && form.occupiedSeats <= form.totalSeats) clearError('seats')
})

watch(() => [form.openingChannelCode, form.customOpeningChannel], ([code]) => {
  if (code !== 'other') form.customOpeningChannel = ''
  if (taskComplete('openingChannel')) clearError('openingChannelCode')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => [form.paymentMethodCode, form.customPaymentMethod], ([code]) => {
  if (code !== 'other') form.customPaymentMethod = ''
  if (taskComplete('paymentMethods')) clearError('paymentMethodCode')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => [form.distributionMethod, form.distributionMethodNote, form.providesAdminAccount], () => {
  if (taskComplete('distribution')) clearError('distribution')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => [form.accessArrangementMode, form.accessArrangementNote, form.riskAcknowledged, form.productId], () => {
  if (accessArrangementComplete(form, selectedProductForValidation.value)) clearError('accessArrangement')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => [form.warranty.mode, form.warranty.fixedWarrantyDays, form.warranty.compensationMethod, form.warranty.exclusions], () => {
  if (warrantyComplete(form.warranty)) clearError('warranty')
  if (!hasSensitiveText.value) clearError('sensitive')
})

watch(() => form.rulesNote, () => {
  if (taskComplete('rulesNote')) clearError('rulesNote')
  if (!hasSensitiveText.value) clearError('sensitive')
})

function validate(requireComplete: boolean) {
  const next: FieldErrors<Field> = {}
  if (!form.productId) next.product = '请选择产品目录。'
  if (form.productId === 'other-custom' && !form.customProductName?.trim()) next.product = '请填写自定义产品名称。'
  if (selectedProductForValidation.value && !canPublishProduct(selectedProductForValidation.value)) {
    next.product = selectedProductForValidation.value.publishPolicy === 'info_only'
      ? '该产品当前仅允许行情和线索展示，不能发布车源。'
      : '该产品当前不允许发布车源。'
  }
  if (!form.regionCode) next.region = '请选择开通区。'
  else if (!finalRegionName.value) next.region = '请填写自定义开通区。'
  if (!Number.isFinite(form.monthlyPriceCny) || !form.monthlyPriceCny || form.monthlyPriceCny <= 0) next.monthlyPriceCny = '月费必须大于 0。'
  if (!Number.isFinite(form.dailyQuotaAmount) || !form.dailyQuotaAmount || form.dailyQuotaAmount <= 0) next.dailyQuota = `每天${selectedProductForValidation.value?.quotaLabel || '额度'}必须大于 0。`
  if (!Number.isFinite(form.weeklyQuotaAmount) || !form.weeklyQuotaAmount || form.weeklyQuotaAmount <= 0) {
    next.weeklyQuota = `每周${selectedProductForValidation.value?.quotaLabel || '额度'}必须大于 0。`
  }
  if (form.followsOfficialQuotaReset === null) next.quotaReset = '请选择额度是否跟随官方重置。'
  if (!form.vpsRegion.trim()) next.connection = '请填写 VPS 区域。'
  else if (form.supportsMainlandChinaDirectConnection === null) next.connection = '请选择是否支持国内直连。'
  if (form.totalSeats < 1 || form.totalSeats > 20 || form.occupiedSeats < 0 || form.occupiedSeats > form.totalSeats) next.seats = '名额必须满足总名额 1-20，且已上车人数不超过总名额。'
  if (!form.openingChannelCode) next.openingChannelCode = '请选择开通渠道。'
  else if (form.openingChannelCode === 'other' && !form.customOpeningChannel.trim()) next.openingChannelCode = '请填写其他开通渠道。'
  if (!form.paymentMethodCode) next.paymentMethodCode = '请选择一种付款方式。'
  else if (form.paymentMethodCode === 'other' && !form.customPaymentMethod.trim()) next.paymentMethodCode = '请填写其他付款方式。'
  if (!form.distributionMethod) {
    next.distribution = '请选择分发方式。'
  } else if (form.distributionMethod === 'other' && !form.distributionMethodNote.trim()) {
    next.distribution = '选择其他分发方式时必须填写说明。'
  } else if (form.providesAdminAccount === null) {
    next.distribution = '请选择是否提供管理员账号。'
  } else if (hasForbiddenCredentialSharingText(form.distributionMethodNote)) {
    next.distribution = '分发方式说明不能包含共享主账号、密码、API Key、Session、Cookie、token 或登录态。'
  }
  if (form.accessArrangementMode === 'not_allowed') {
    next.accessArrangement = '共用账号、密码或登录态方案不能发布。'
  } else if (hasForbiddenCredentialSharingText(form.accessArrangementNote)) {
    next.accessArrangement = '安排说明不能包含共享主账号、密码、API Key、Session、Cookie、token 或登录态。'
  } else if (requiresSubscriptionRiskAck(selectedProductForValidation.value, form) && !form.riskAcknowledged) {
    next.accessArrangement = '请先确认该套餐的发布边界。'
  } else if (form.productId && form.accessArrangementNote.trim().length < 8) {
    next.accessArrangement = '系统未能生成访问安排，请重新选择产品。'
  }
  if (!warrantyComplete(form.warranty)) next.warranty = '请补全车主承诺规则。'
  if (!form.rulesNote.trim()) next.rulesNote = '请填写规则说明。'
  if (containsSensitiveContent([
    form.customProductName ?? '',
    form.customRegionName ?? '',
    form.customOpeningChannel,
    form.customPaymentMethod,
    form.vpsRegion,
    form.warranty.compensationMethod ?? '',
    form.warranty.exclusions ?? '',
    form.distributionMethodNote,
    form.accessArrangementNote,
    form.rulesNote,
  ])) next.sensitive = '请移除账号密码、session token、refresh token、API Key、付款二维码、银行卡号或其他敏感凭据。'

  if (!requireComplete) {
    delete next.product
    delete next.region
    delete next.monthlyPriceCny
    delete next.dailyQuota
    delete next.weeklyQuota
    delete next.quotaReset
    delete next.connection
    delete next.openingChannelCode
    delete next.paymentMethodCode
    delete next.distribution
    delete next.accessArrangement
    delete next.warranty
    delete next.rulesNote
  }

  if (requireComplete && selectedProductForValidation.value && !canPublishProduct(selectedProductForValidation.value)) {
    next.product = selectedProductForValidation.value.publishPolicy === 'info_only'
      ? '该产品当前仅允许行情和线索展示，不能发布车源。'
      : '该产品当前不允许发布车源。'
  }

  setErrors(next)
  return Object.keys(next).length === 0
}

function toPayload(status: 'draft' | 'reviewing') {
  return {
    productId: form.productId,
    customProductName: form.customProductName,
    regionCode: form.regionCode,
    customRegionName: form.regionCode === 'other' ? form.customRegionName?.trim() || null : null,
    monthlyPriceCny: form.monthlyPriceCny,
    serviceMultiplier: 1,
    dailyQuotaAmount: form.dailyQuotaAmount,
    weeklyQuotaAmount: form.weeklyQuotaAmount,
    followsOfficialQuotaReset: form.followsOfficialQuotaReset,
    vpsRegion: form.vpsRegion.trim(),
    supportsMainlandChinaDirectConnection: form.supportsMainlandChinaDirectConnection,
    totalSeats: form.totalSeats,
    occupiedSeats: form.occupiedSeats,
    openingChannelCode: form.openingChannelCode,
    customOpeningChannel: form.openingChannelCode === 'other' ? form.customOpeningChannel.trim() : '',
    paymentMethodCode: form.paymentMethodCode,
    customPaymentMethod: form.paymentMethodCode === 'other' ? form.customPaymentMethod.trim() : '',
    distributionMethod: form.distributionMethod,
    distributionMethodNote: form.distributionMethodNote,
    providesAdminAccount: form.providesAdminAccount,
    accessArrangementMode: form.accessArrangementMode,
    accessArrangementNote: form.accessArrangementNote,
    riskAcknowledged: form.riskAcknowledged,
    policyVersion: form.policyVersion,
    riskNoticeCode: form.riskNoticeCode,
    warranty: { ...form.warranty },
    rulesNote: form.rulesNote,
    status,
  }
}

function saveDraft() {
  if (!ensurePublishAccess()) return
  if (!validate(false)) {
    toast.warning(firstError(errors) ?? '请先修正草稿字段。')
    return
  }
  saveDraftMutation.mutate()
}

async function submitReview() {
  if (!ensurePublishAccess()) return
  hasTriedPublish.value = true
  if (!validate(true)) {
    mobileCheckOpen.value = isMobilePublishCheckViewport()
    toast.warning(firstError(errors) ?? '请先补全车源配置。')
    await focusFirstInvalidTask()
    return
  }
  submitReviewMutation.mutate()
}

const completeness = computed<CompletenessItem[]>(() => [
  form.productId && (form.productId !== 'other-custom' || form.customProductName?.trim()) ? { label: '产品', status: 'done' } : { label: '产品', status: 'pending' },
  taskComplete('region') ? { label: '地区', status: 'done' } : { label: '地区', status: 'pending' },
  form.monthlyPriceCny && form.monthlyPriceCny > 0 ? { label: '月费', status: 'done' } : { label: '月费', status: 'pending' },
  taskComplete('dailyQuota') && taskComplete('weeklyQuota') ? { label: '每天 / 每周额度', status: 'done' } : { label: '每天 / 每周额度', status: 'pending' },
  taskComplete('quotaReset') ? { label: '额度重置', status: 'done' } : { label: '额度重置', status: 'pending' },
  taskComplete('connection') ? { label: 'VPS 与国内直连', status: 'done' } : { label: 'VPS 与国内直连', status: 'pending' },
  form.totalSeats >= 1 && form.totalSeats <= 20 && form.occupiedSeats >= 0 && form.occupiedSeats < form.totalSeats ? { label: '名额', status: 'done' } : { label: '名额', status: 'conflict' },
  form.openingChannelCode ? { label: '开通渠道', status: 'done' } : { label: '开通渠道', status: 'pending' },
  taskComplete('paymentMethods') ? { label: '付款方式', status: 'done' } : { label: '付款方式', status: 'pending' },
  distributionFieldsComplete(form) ? { label: '分发方式', status: 'done' } : { label: '分发方式', status: 'pending' },
  accessArrangementComplete(form, selectedProductForValidation.value) ? { label: '发布边界确认', status: 'done' } : { label: '发布边界确认', status: 'conflict' },
  warrantyComplete(form.warranty) ? { label: '车主承诺', status: 'done' } : { label: '车主承诺', status: 'pending' },
  form.rulesNote.trim() ? { label: '买家须知', status: 'done' } : { label: '买家须知', status: 'pending' },
])

const trustItems = computed<TrustItem[]>(() => [
  {
    label: !profile.value
      ? 'linux.do 身份状态待确认'
      : profile.value.linuxDoBinding.bound
        ? `已绑定 linux.do @${profile.value.linuxDoBinding.linuxDoUsername}`
        : '未登录或未绑定 linux.do 身份',
    status: profile.value?.linuxDoBinding.bound ? 'done' : 'pending',
    description: profile.value?.linuxDoBinding.bound ? '当前账号已具备发布车源资格。' : '发布车源需要账号完成 linux.do 身份绑定。',
  },
])

const reminders = computed(() => {
  const rows: string[] = []
  if (form.productId === 'other-custom') rows.push('自定义产品提交后需要先完成目录确认。')
  if (selectedProductForValidation.value && selectedProductForValidation.value.publishPolicy !== 'allowed') {
    rows.push(selectedProductForValidation.value.publishPolicy === 'info_only' ? '该产品当前仅用于行情和线索展示。' : '该产品当前不允许发布。')
  }
  if (requiresSubscriptionRiskAck(selectedProductForValidation.value, form) && !form.riskAcknowledged) rows.push('该套餐需要先确认发布边界后才能发布。')
  if (availableSeats(form) === 0) rows.push('当前剩余名额为 0，发布后前台会显示已满。')
  return rows
})

const submittedMessage = computed(() => {
  if (!submittedId.value) return ''
  if (shouldUseRealBackend()) return `车源记录已提交：${submittedId.value}。`
  return `已生成本地演示车源记录：${submittedId.value}。`
})

const hasSensitiveText = computed(() => containsSensitiveContent([
  form.customProductName ?? '',
  form.customRegionName ?? '',
  form.customOpeningChannel,
  form.customPaymentMethod,
  form.vpsRegion,
  form.warranty.compensationMethod ?? '',
  form.warranty.exclusions ?? '',
  form.distributionMethodNote,
  form.accessArrangementNote,
  form.rulesNote,
]))
const canCopyShareText = computed(() => (
  canBuildCarpoolShareText(form, regionsByCode.value, openingChannelsByCode.value, paymentMethodsByCode.value)
  && !hasSensitiveText.value
))
const shareText = computed(() => buildCarpoolShareText(
  form,
  catalogById.value,
  regionsByCode.value,
  openingChannelsByCode.value,
  paymentMethodsByCode.value,
  submittedId.value ? `${window.location.origin}/carpools/${submittedId.value}` : undefined,
))
const copyDisabledReason = computed(() => {
  if (hasSensitiveText.value) return '请先移除账号密码、token、API Key、付款二维码、银行卡号等敏感凭据。'
  if (!canBuildCarpoolShareText(form, regionsByCode.value, openingChannelsByCode.value, paymentMethodsByCode.value)) {
    return '填写产品、地区、价格、名额、渠道、付款方式、售后和买家须知后可生成分享文案。'
  }
  return ''
})

async function copyShareText() {
  if (!canCopyShareText.value) {
    toast.warning(copyDisabledReason.value || '请先补全分享文案所需字段。')
    return
  }
  try {
    await navigator.clipboard.writeText(shareText.value)
    toast.success('已复制车源分享文案')
  } catch {
    toast.warning('复制失败，请手动选择文案复制')
  }
}
</script>

<template>
  <div class="space-y-5" :class="canAccessPublishForm ? 'pb-[calc(96px+env(safe-area-inset-bottom))] sm:pb-0' : 'pb-0'" @input="formDirty = true" @change="formDirty = true">
    <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div>
		<h1 class="text-2xl font-semibold md:text-3xl">{{ isEditMode ? '编辑车源' : '发布车源' }}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{{ isEditMode ? '更新车源的套餐、额度、接入和使用规则。' : '填写车源的套餐、额度、接入和使用规则。' }}</p>
      </div>
      <div v-if="canAccessPublishForm" class="hidden gap-2 sm:flex">
		<Button variant="outline" :disabled="saveDraftMutation.isPending.value" @click="saveDraft"><Save class="h-4 w-4" />{{ isEditMode ? '保存修改' : '保存草稿' }}</Button>
		<Button :disabled="submitReviewMutation.isPending.value" @click="submitReview"><Send class="h-4 w-4" />{{ isEditMode ? '保存并提交审核' : '检查并发布' }}</Button>
      </div>
    </div>

    <Card v-if="profileQuery.isPending.value || (isEditMode && editQuery.isPending.value)" class="mx-auto max-w-2xl p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <Loader2 class="h-5 w-5 animate-spin" />
        </div>
        <div class="min-w-0 flex-1">
		  <h2 class="text-lg font-semibold tracking-tight">{{ isEditMode ? '正在读取车源' : '正在确认发布资格' }}</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">
			{{ isEditMode ? '正在读取最新车源内容和版本。' : '车源发布需要当前账号已绑定 linux.do，确认完成后会进入发布表单。' }}
          </p>
        </div>
      </div>
    </Card>

	<Card v-else-if="isEditMode && editQuery.isError.value" class="mx-auto max-w-2xl p-6">
	  <h2 class="text-lg font-semibold">无法读取车源</h2>
	  <p class="mt-2 text-sm text-muted-foreground">{{ editQuery.error.value instanceof Error ? editQuery.error.value.message : '车源不存在或不属于当前账号。' }}</p>
	  <div class="mt-5 flex gap-2">
		<Button variant="outline" :disabled="editQuery.isFetching.value" @click="editQuery.refetch()"><RefreshCw class="h-4 w-4" />重新读取</Button>
		<RouterLink to="/my/carpools"><Button>返回我的车源</Button></RouterLink>
	  </div>
	</Card>

	<Card v-else-if="profileQuery.isError.value || !profile" class="mx-auto max-w-2xl p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <LogIn class="h-5 w-5" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold tracking-tight">登录后发布车源</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">
            发布车源前需要先登录账号，并完成 linux.do 身份绑定。
          </p>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ profileErrorMessage }}</p>
          <div class="mt-5 flex flex-wrap gap-2">
            <RouterLink :to="publishLoginRoute">
              <Button><LogIn class="h-4 w-4" />登录 / 注册</Button>
            </RouterLink>
            <Button variant="outline" :disabled="profileQuery.isFetching.value" @click="profileQuery.refetch()">
              <RefreshCw class="h-4 w-4" :class="profileQuery.isFetching.value ? 'animate-spin' : ''" />
              重新读取
            </Button>
          </div>
        </div>
      </div>
    </Card>

    <Card v-else-if="!profile.linuxDoBinding.bound" class="mx-auto max-w-2xl p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <ShieldCheck class="h-5 w-5" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold tracking-tight">完成 linux.do 身份绑定后发布车源</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">
            当前账号已登录，但还没有绑定 linux.do。完成身份绑定后即可发布车源。
          </p>
          <div class="mt-5 flex flex-wrap gap-2">
            <Button :disabled="oauthPending" @click="startLinuxDoPublishAuth">
              <Loader2 v-if="oauthPending" class="h-4 w-4 animate-spin" />
              <ShieldCheck v-else class="h-4 w-4" />
              使用 linux.do 登录 / 绑定
            </Button>
            <RouterLink to="/my/account">
              <Button variant="outline">查看账号与认证</Button>
            </RouterLink>
          </div>
        </div>
      </div>
    </Card>

	<Card v-else-if="isEditMode && !editableBackendStatus" class="mx-auto max-w-2xl p-6">
	  <h2 class="text-lg font-semibold">当前车源不可编辑</h2>
	  <p class="mt-2 text-sm text-muted-foreground">只有草稿和管理员要求修改的车源可以编辑。请返回车源管理页查看当前状态。</p>
	  <RouterLink class="mt-5 inline-flex" to="/my/carpools"><Button>返回我的车源</Button></RouterLink>
	</Card>

    <template v-else>
      <div class="rounded-lg border border-primary/15 bg-primary/5 p-3 sm:hidden">
        <div class="flex items-center gap-3 text-sm font-medium">
          <span>发布必填 {{ completedPublishTasks.length }} / {{ publishTasks.length }}</span>
        </div>
        <div class="mt-2 h-2 overflow-hidden rounded-full bg-background">
          <div class="h-full rounded-full bg-primary" :style="{ width: `${publishProgressPercent}%` }" />
        </div>
        <div class="mt-2 flex items-center justify-between gap-3">
          <p class="text-xs leading-5 text-muted-foreground">{{ mobileStatusText }}</p>
          <Button size="sm" variant="outline" @click="mobileCheckOpen = true">查看待补项</Button>
        </div>
      </div>

      <div class="hidden rounded-lg border border-border bg-card p-4 shadow-sm sm:block">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <div class="text-sm font-medium">发布必填 {{ completedPublishTasks.length }} / {{ publishTasks.length }}</div>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ pendingPublishTasks.length ? `还差 ${pendingPublishTasks.length} 项可提交审核` : '发布必填项已完成，可提交审核。' }}
              <span class="ml-1">系统已自动处理 {{ defaultItems.filter(item => item.status === 'defaulted').length }} 项。</span>
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <span class="rounded-full border border-warning/25 bg-warning/10 px-3 py-1 text-xs font-medium text-warning">待补 {{ pendingPublishTasks.length }}</span>
            <span class="rounded-full border border-success/25 bg-success/10 px-3 py-1 text-xs font-medium text-success">已完成 {{ completedPublishTasks.length }}</span>
            <Button size="sm" variant="outline" class="sm:hidden" @click="mobileCheckOpen = true">发布前检查</Button>
          </div>
        </div>
        <div class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
          <div class="h-full rounded-full bg-primary" :style="{ width: `${publishProgressPercent}%` }" />
        </div>
      </div>

      <div
        v-if="hasTriedPublish && Object.keys(errors).length"
        class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div class="font-medium">还差 {{ pendingPublishTasks.length }} 项才能发布</div>
            <p class="mt-1 text-xs leading-5">{{ errorSummaryText || errors.sensitive || '请先处理发布前检查提示。' }}</p>
          </div>
          <Button size="sm" variant="outline" class="border-destructive/30 bg-background text-destructive hover:bg-destructive/10" @click="focusFirstInvalidTask">
            跳到第一个待补项
          </Button>
        </div>
      </div>

      <div v-if="errors.sensitive && !hasTriedPublish" class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
        {{ errors.sensitive }}
      </div>
      <div v-if="submittedId" class="rounded-lg border border-border bg-accent px-4 py-3 text-sm">
        <div class="font-medium">{{ submittedMessage }}</div>
        <div class="mt-2 flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" :disabled="!canCopyShareText" @click="copyShareText">复制车源分享文案</Button>
        </div>
      </div>

      <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,360px)] lg:items-start">
        <section class="space-y-4">
          <CarpoolBasicInfoSection
            :form="form"
            :catalog="catalog"
            :regions="regionOptions"
            :errors="errors"
            :field-states="basicFieldStates"
            :highlighted-key="highlightedTaskKey ?? undefined"
          />
          <Card
            v-if="requiresSubscriptionRiskAck(selectedProductForValidation, form)"
            id="carpool-task-riskAcknowledgement"
            class="border-warning/25 bg-warning/10 p-4 text-warning"
            :class="errors.accessArrangement ? 'ring-2 ring-warning/50 ring-offset-2 ring-offset-background' : ''"
          >
            <label class="flex gap-3 text-sm leading-6">
              <Checkbox v-model="form.riskAcknowledged" class="mt-1" />
              <span>
                我确认已理解该套餐发布边界；平台不会填写、保存、交付或要求买家提供主账号、密码、API Key、Session、Cookie、token 或其他登录凭据。
              </span>
            </label>
            <p v-if="errors.accessArrangement" class="mt-2 text-xs text-destructive">{{ errors.accessArrangement }}</p>
          </Card>
          <SeatCapacityEditor :form="form" :errors="errors" />
          <ChannelPaymentSection
            :form="form"
            :opening-channels="channelOptions"
            :payment-methods="paymentOptions"
            :errors="errors"
            :field-states="channelPaymentFieldStates"
            :highlighted-key="highlightedTaskKey ?? undefined"
          />
          <CarpoolWarrantySelector :form="form" :errors="errors" />
          <CarpoolRulesEditor
            :form="form"
            :errors="errors"
            :field-state="stateForTask('rulesNote')"
            :highlighted-key="highlightedTaskKey ?? undefined"
          />
        </section>

        <div class="space-y-3 lg:sticky lg:[top:calc(var(--app-header-height)+16px)]">
          <Dialog>
            <DialogTrigger as-child>
              <Button variant="outline" class="hidden w-full lg:inline-flex">
                <Eye class="h-4 w-4" />预览车源卡
              </Button>
            </DialogTrigger>
            <DialogContent class="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>车源预览</DialogTitle>
                <DialogDescription>发布前确认买家将看到的核心信息。</DialogDescription>
              </DialogHeader>
              <CarpoolPublishPreview
                :form="form"
                :catalog-by-id="catalogById"
                :regions-by-code="regionsByCode"
                :opening-channels-by-code="openingChannelsByCode"
                :payment-methods-by-code="paymentMethodsByCode"
                :completeness="completeness"
                :reminders="[]"
                :submit-pending="submitReviewMutation.isPending.value"
                preview-only
                @save-draft="saveDraft"
                @submit-review="submitReview"
              />
            </DialogContent>
          </Dialog>
          <CarpoolPublishAssistant
            :tasks="publishTasks"
            :default-items="defaultItems"
            :trust-items="trustItems"
            :reminders="reminders"
            :remaining-seats="availableSeats(form)"
            :total-seats="form.totalSeats"
            :copy-enabled="canCopyShareText"
            :copy-disabled-reason="copyDisabledReason"
            :share-text="shareText"
            :submit-pending="submitReviewMutation.isPending.value"
            @save-draft="saveDraft"
            @submit-review="submitReview"
            @copy-share-text="copyShareText"
            @jump-to-task="jumpToTask"
          />
        </div>
      </div>

      <div class="sticky bottom-0 z-30 grid grid-cols-2 gap-2 border-t border-border bg-background/95 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] backdrop-blur sm:hidden">
		<Button variant="outline" :disabled="saveDraftMutation.isPending.value" @click="saveDraft">{{ isEditMode ? '保存修改' : '保存草稿' }}</Button>
		<Button :disabled="submitReviewMutation.isPending.value" @click="submitReview">{{ isEditMode ? '保存并提交' : '检查并发布' }}</Button>
      </div>

      <Dialog v-model:open="mobileCheckOpen">
        <DialogContent class="bottom-0 left-0 top-auto max-h-[80dvh] max-w-full translate-x-0 translate-y-0 rounded-b-none rounded-t-2xl p-0 sm:hidden">
          <div class="mx-auto mt-3 h-1 w-10 rounded-full bg-muted" />
          <div class="px-4 pb-4 pt-3">
            <DialogHeader class="pr-8 text-left">
              <DialogTitle>发布前检查</DialogTitle>
              <DialogDescription>
                {{ pendingPublishTasks.length ? `还差 ${pendingPublishTasks.length} 项可发布，点击任一项可跳转。` : '发布必填项已完成。' }}
              </DialogDescription>
            </DialogHeader>
            <div class="mt-4 h-2 overflow-hidden rounded-full bg-muted">
              <div class="h-full rounded-full bg-primary" :style="{ width: `${publishProgressPercent}%` }" />
            </div>
            <div class="mt-4 space-y-2">
              <Button
                v-for="(task, index) in pendingPublishTasks"
                :key="task.key"
                type="button"
                variant="outline"
                class="h-auto w-full justify-start whitespace-normal px-3 py-3 text-left text-sm"
                :class="hasTriedPublish ? 'border-warning/35' : ''"
                @click="mobileCheckOpen = false; jumpToTask(task.key)"
              >
                <span class="grid h-6 w-6 place-items-center rounded-full bg-warning/10 text-xs font-semibold text-warning">{{ index + 1 }}</span>
                <span class="min-w-0 flex-1">
                  <span class="block font-medium">{{ task.label }}</span>
                  <span class="mt-0.5 block text-xs text-muted-foreground">{{ task.description }}</span>
                </span>
                <span class="text-muted-foreground">→</span>
              </Button>
              <div v-if="!pendingPublishTasks.length" class="rounded-lg border border-success/25 bg-success/10 px-3 py-4 text-sm text-success">
                发布必填项已完成，可以提交审核。
              </div>
            </div>
            <div class="mt-4 grid grid-cols-2 gap-2">
              <Button variant="outline" @click="saveDraft">先存草稿</Button>
              <Button :disabled="submitReviewMutation.isPending.value" @click="submitReview">检查并发布</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </template>
  </div>
</template>
