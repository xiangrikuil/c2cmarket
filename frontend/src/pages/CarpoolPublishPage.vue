<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { RouterLink, useRoute } from 'vue-router'
import { ChevronDown, ChevronUp, Eye, Loader2, LogIn, RefreshCw, Save, Send, ShieldCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import CarpoolBasicInfoSection from '@/components/carpool-publish/CarpoolBasicInfoSection.vue'
import CarpoolPublishAssistant from '@/components/carpool-publish/CarpoolPublishAssistant.vue'
import CarpoolPublishPreview from '@/components/carpool-publish/CarpoolPublishPreview.vue'
import CarpoolRulesEditor from '@/components/carpool-publish/CarpoolRulesEditor.vue'
import CarpoolWarrantySelector from '@/components/carpool-publish/CarpoolWarrantySelector.vue'
import ChannelPaymentSection from '@/components/carpool-publish/ChannelPaymentSection.vue'
import LinuxDoTopicImport from '@/components/carpool-publish/LinuxDoTopicImport.vue'
import PublishSectionCard from '@/components/carpool-publish/PublishSectionCard.vue'
import SeatCapacityEditor from '@/components/carpool-publish/SeatCapacityEditor.vue'
import type {
  CarpoolProductCatalogItem,
  CarpoolPublishForm,
  CompletenessItem,
  ParsedLinuxDoTopic,
  PublishDefaultItem,
  PublishFieldState,
  PublishTask,
  PublishTaskKey,
  TrustItem,
} from '@/components/carpool-publish/types'
import {
  accessArrangementComplete,
  availableSeats,
  buildLinuxDoPostText,
  canBuildLinuxDoPostText,
  canPublishProduct,
  hasForbiddenCredentialSharingText,
  requiresSubscriptionRiskAck,
  warrantyComplete,
} from '@/components/carpool-publish/utils'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { shouldUseRealBackend, startOAuthLogin } from '@/lib/backendClient'
import { containsSensitiveContent, firstError, isLinuxDoTopicUrl, type FieldErrors } from '@/lib/formValidation'
import { parseLinuxDoTopic, submitCarpool } from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import {
  useCarpoolOpeningChannels,
  useCarpoolPaymentMethods,
  useCarpoolProductCatalog,
  useCarpoolRegions,
  useMyProfileQuery,
} from '@/queries/useMarketQueries'
import { quotaFieldLabel } from '@/lib/quota'

type Field =
  | 'linuxDoTopicUrl'
  | 'product'
  | 'region'
  | 'monthlyPriceCny'
  | 'serviceMultiplier'
  | 'monthlyQuota'
  | 'seats'
  | 'openingChannelCode'
  | 'paymentMethodCodes'
  | 'accessArrangement'
  | 'warranty'
  | 'rulesNote'
  | 'sensitive'

const { data: productCatalog } = useCarpoolProductCatalog()
const { data: regions } = useCarpoolRegions()
const { data: openingChannels } = useCarpoolOpeningChannels()
const { data: paymentMethods } = useCarpoolPaymentMethods()
const profileQuery = useMyProfileQuery()
const profile = profileQuery.data
const queryClient = useQueryClient()
const route = useRoute()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')

const parsedTopic = ref<ParsedLinuxDoTopic | null>(null)
const submittedId = ref('')
const oauthPending = ref(false)
const hasTriedPublish = ref(false)
const mobileCheckOpen = ref(false)
const linuxDoImportOpen = ref(false)
const highlightedTaskKey = ref<string | null>(null)
const errors = reactive<FieldErrors<Field>>({})
const publishReturnTo = '/carpools/new'
const publishLoginRoute = { path: '/login', query: { returnTo: publishReturnTo } }
let highlightTimer: ReturnType<typeof window.setTimeout> | null = null

const form = reactive<CarpoolPublishForm>({
  linuxDoTopicUrl: '',
  parsedTopicId: null,
  productId: '',
  customProductName: null,
  regionCode: '',
  monthlyPriceCny: null,
  serviceMultiplier: 1,
  monthlyQuotaAmount: null,
  totalSeats: 5,
  occupiedSeats: 3,
  openingChannelCode: '',
  paymentMethodCodes: [],
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
const canAccessPublishForm = computed(() => Boolean(profile.value?.linuxDoBinding.bound))
const profileErrorMessage = computed(() => {
  const error = profileQuery.error.value
  return error instanceof Error ? error.message : '请先登录并完成 linux.do 身份绑定。'
})

watch(selectedProductForValidation, product => {
  if (!product) return
  form.policyVersion = product.policyVersion
  form.riskNoticeCode = product.riskNoticeCode ?? null
  form.accessArrangementMode = product.accessMode
  form.accessArrangementNote = defaultAccessArrangementNote(product)
  form.riskAcknowledged = false
})

const parseTopicMutation = useMutation({
  mutationFn: () => parseLinuxDoTopic(form.linuxDoTopicUrl),
  onSuccess(result) {
    parsedTopic.value = result
    applyParsedTopic(result)
    toast.success(`原帖读取成功 · 作者 ${result.authorUsername}`)
  },
  onError() {
    parsedTopic.value = null
    form.parsedTopicId = null
    toast.warning('未能识别完整信息，可继续手动填写。')
  },
})

const saveDraftMutation = useMutation({
  mutationFn: () => submitCarpool(toPayload('draft')),
  async onSuccess(result) {
    submittedId.value = String(result.id)
    await invalidateCarpoolPublishQueries()
    toast.success('车源草稿已保存。')
  },
})

const submitReviewMutation = useMutation({
  mutationFn: () => submitCarpool(toPayload('reviewing')),
  async onSuccess(result) {
    submittedId.value = String(result.id)
    await invalidateCarpoolPublishQueries()
    trackAnalytics('carpool_publish_success', {
      source_route: analyticsSourceRoute(),
      product: selectedProductForValidation.value?.categoryCode ?? form.productId,
      monthly_price_cny: form.monthlyPriceCny,
      seats: form.totalSeats,
      access_mode: form.accessArrangementMode,
      risk_ack_required: Boolean(form.riskNoticeCode),
      risk_notice: form.riskNoticeCode ?? 'none',
    })
    toast.success('车源已提交。')
  },
})

async function invalidateCarpoolPublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['carpools'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['notifications'] })
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
  monthlyQuota: 'carpool-task-monthlyQuota',
  openingChannel: 'carpool-task-openingChannel',
  paymentMethods: 'carpool-task-paymentMethods',
  rulesNote: 'carpool-task-rulesNote',
  linuxDoImport: 'carpool-tool-linuxdo-import',
}

function fieldErrorForTask(key: PublishTaskKey) {
  if (!hasTriedPublish.value) return ''
  if (key === 'product') return errors.product ?? ''
  if (key === 'region') return errors.region ?? ''
  if (key === 'monthlyPrice') return errors.monthlyPriceCny ?? ''
  if (key === 'monthlyQuota') return errors.monthlyQuota ?? ''
  if (key === 'openingChannel') return errors.openingChannelCode ?? ''
  if (key === 'paymentMethods') return errors.paymentMethodCodes ?? ''
  if (key === 'rulesNote') return errors.rulesNote ?? ''
  return ''
}

function taskComplete(key: PublishTaskKey) {
  if (key === 'product') return Boolean(form.productId && (form.productId !== 'other-custom' || form.customProductName?.trim()))
  if (key === 'region') return Boolean(form.regionCode)
  if (key === 'monthlyPrice') return Boolean(form.monthlyPriceCny && form.monthlyPriceCny > 0)
  if (key === 'monthlyQuota') return Boolean(form.monthlyQuotaAmount && form.monthlyQuotaAmount > 0)
  if (key === 'openingChannel') return Boolean(form.openingChannelCode)
  if (key === 'paymentMethods') return Boolean(form.paymentMethodCodes.length)
  if (key === 'rulesNote') return Boolean(form.rulesNote.trim())
  return Boolean(form.linuxDoTopicUrl.trim())
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
    key: 'monthlyQuota',
    label: `填写${quotaFieldLabel(selectedProductForValidation.value)}`,
    shortLabel: quotaFieldLabel(selectedProductForValidation.value),
    section: 'basic',
    fieldId: publishTaskFieldIds.monthlyQuota,
    description: '车源基础信息',
    complete: taskComplete('monthlyQuota'),
    error: fieldErrorForTask('monthlyQuota'),
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
    label: '选择付款方式',
    shortLabel: '付款方式',
    section: 'activationPayment',
    fieldId: publishTaskFieldIds.paymentMethods,
    description: '开通与付款方式',
    complete: taskComplete('paymentMethods'),
    error: fieldErrorForTask('paymentMethods'),
  },
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
  if (field === 'serviceMultiplier') {
    if (hasTriedPublish.value && errors.serviceMultiplier) return 'error'
    return form.serviceMultiplier && form.serviceMultiplier > 0 ? 'defaulted' : 'pendingRequired'
  }
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
  monthlyQuota: stateForTask('monthlyQuota'),
  serviceMultiplier: stateForFullValidation('serviceMultiplier'),
}))

const channelPaymentFieldStates = computed<Partial<Record<string, PublishFieldState>>>(() => ({
  openingChannel: stateForTask('openingChannel'),
  paymentMethods: stateForTask('paymentMethods'),
}))

const defaultItems = computed<PublishDefaultItem[]>(() => [
  {
    key: 'serviceMultiplier',
    label: '倍率已默认',
    description: `${form.serviceMultiplier ?? 1}x，可按实际情况修改。`,
    status: stateForFullValidation('serviceMultiplier'),
  },
  {
    key: 'seats',
    label: '名额设置已默认',
    description: `总 ${form.totalSeats} 人，已上车 ${form.occupiedSeats} 人。`,
    status: stateForFullValidation('seats'),
  },
  {
    key: 'accessArrangement',
    label: '访问安排已默认',
    description: accessArrangementComplete(form, selectedProductForValidation.value) ? '可继续修改访问边界说明。' : '需要确认访问安排边界。',
    status: stateForFullValidation('accessArrangement'),
  },
  {
    key: 'warranty',
    label: '车主承诺已默认',
    description: warrantyComplete(form.warranty) ? '可继续修改售后承诺。' : '需要补全售后承诺。',
    status: stateForFullValidation('warranty'),
  },
])

const sectionStatus = computed(() => {
  const basicPending = publishTasks.value.filter(item => item.section === 'basic' && !item.complete).length
  const activationPending = publishTasks.value.filter(item => item.section === 'activationPayment' && !item.complete).length
  const rulesPending = publishTasks.value.filter(item => item.section === 'rules' && !item.complete).length
  return {
    basic: basicPending ? (hasTriedPublish.value ? 'error' : 'pendingRequired') as PublishFieldState : 'complete' as PublishFieldState,
    seats: stateForFullValidation('seats'),
    activationPayment: activationPending ? (hasTriedPublish.value ? 'error' : 'pendingRequired') as PublishFieldState : 'complete' as PublishFieldState,
    accessArrangement: stateForFullValidation('accessArrangement'),
    warranty: stateForFullValidation('warranty'),
    rules: rulesPending ? (hasTriedPublish.value ? 'error' : 'pendingRequired') as PublishFieldState : 'complete' as PublishFieldState,
  }
})

function sectionStatusLabel(status: PublishFieldState, pendingCount = 0) {
  if (status === 'error') return pendingCount ? `待补 ${pendingCount} 项` : '需要处理'
  if (status === 'pendingRequired') return pendingCount ? `待填写 ${pendingCount} 项` : '待填写'
  if (status === 'defaulted') return '系统默认'
  if (status === 'complete') return '已完成'
  return ''
}

function taskFromErrorKey(key: Field): PublishTaskKey | null {
  if (key === 'product') return 'product'
  if (key === 'region') return 'region'
  if (key === 'monthlyPriceCny') return 'monthlyPrice'
  if (key === 'monthlyQuota') return 'monthlyQuota'
  if (key === 'openingChannelCode') return 'openingChannel'
  if (key === 'paymentMethodCodes') return 'paymentMethods'
  if (key === 'rulesNote') return 'rulesNote'
  if (key === 'serviceMultiplier') return 'monthlyPrice'
  if (key === 'seats') return 'monthlyPrice'
  if (key === 'accessArrangement') return 'rulesNote'
  if (key === 'warranty') return 'rulesNote'
  if (key === 'sensitive') return 'rulesNote'
  return null
}

async function jumpToTask(key: PublishTaskKey | string) {
  if (key === 'linuxDoImport') linuxDoImportOpen.value = true
  await nextTick()
  const targetId = publishTaskFieldIds[key as PublishTaskKey]
  const target = targetId ? document.getElementById(targetId) : null
  if (!target) return
  target.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'center' })
  const focusable = target.querySelector<HTMLElement>('input, textarea, button, [tabindex]:not([tabindex="-1"])')
  focusable?.focus({ preventScroll: true })
  highlightedTaskKey.value = key
  if (highlightTimer) window.clearTimeout(highlightTimer)
  highlightTimer = window.setTimeout(() => {
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

function applyParsedTopic(topic: ParsedLinuxDoTopic) {
  form.parsedTopicId = topic.topicId
  if (topic.detected.productId && !form.productId) form.productId = topic.detected.productId
  if (topic.detected.regionCode && !form.regionCode) form.regionCode = topic.detected.regionCode
  if (topic.detected.monthlyPriceCny && !form.monthlyPriceCny) form.monthlyPriceCny = topic.detected.monthlyPriceCny
  if (topic.detected.totalSeats && form.totalSeats === 5) form.totalSeats = topic.detected.totalSeats
  if (topic.detected.occupiedSeats !== null && form.occupiedSeats === 3) form.occupiedSeats = Math.min(topic.detected.occupiedSeats, form.totalSeats)
  if (topic.detected.openingChannelId && !form.openingChannelCode) form.openingChannelCode = topic.detected.openingChannelId
  if (topic.detected.paymentMethodIds.length && !form.paymentMethodCodes.length) form.paymentMethodCodes = topic.detected.paymentMethodIds
  if (topic.detected.warrantyMode) form.warranty.mode = topic.detected.warrantyMode
  const detectedProduct = topic.detected.productId ? catalogById.value.get(topic.detected.productId) : null
  if (detectedProduct) {
    form.policyVersion = detectedProduct.policyVersion
    form.riskNoticeCode = detectedProduct.riskNoticeCode ?? null
    form.accessArrangementMode = detectedProduct.accessMode
    form.accessArrangementNote = defaultAccessArrangementNote(detectedProduct)
    form.riskAcknowledged = false
  }
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

watch(() => form.regionCode, () => {
  if (taskComplete('region')) clearError('region')
})

watch(() => form.monthlyPriceCny, () => {
  if (taskComplete('monthlyPrice')) clearError('monthlyPriceCny')
})

watch(() => form.monthlyQuotaAmount, () => {
  if (taskComplete('monthlyQuota')) clearError('monthlyQuota')
})

watch(() => form.serviceMultiplier, () => {
  if (form.serviceMultiplier && form.serviceMultiplier > 0) clearError('serviceMultiplier')
})

watch(() => [form.totalSeats, form.occupiedSeats], () => {
  if (form.totalSeats >= 1 && form.totalSeats <= 20 && form.occupiedSeats >= 0 && form.occupiedSeats <= form.totalSeats) clearError('seats')
})

watch(() => form.openingChannelCode, () => {
  if (taskComplete('openingChannel')) clearError('openingChannelCode')
})

watch(() => form.paymentMethodCodes.length, () => {
  if (taskComplete('paymentMethods')) clearError('paymentMethodCodes')
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
  if (form.linuxDoTopicUrl.trim() && !isLinuxDoTopicUrl(form.linuxDoTopicUrl)) next.linuxDoTopicUrl = '原帖链接必须是 https://linux.do/t/*。'
  if (!form.productId) next.product = '请选择产品目录。'
  if (form.productId === 'other-custom' && !form.customProductName?.trim()) next.product = '请填写自定义产品名称。'
  if (selectedProductForValidation.value && !canPublishProduct(selectedProductForValidation.value)) {
    next.product = selectedProductForValidation.value.publishPolicy === 'info_only'
      ? '该产品当前仅允许行情和线索展示，不能发布车源。'
      : '该产品当前不允许发布车源。'
  }
  if (!form.regionCode) next.region = '请选择开通区。'
  if (!Number.isFinite(form.monthlyPriceCny) || !form.monthlyPriceCny || form.monthlyPriceCny <= 0) next.monthlyPriceCny = '月费必须大于 0。'
  if (!Number.isFinite(form.serviceMultiplier) || !form.serviceMultiplier || form.serviceMultiplier <= 0) next.serviceMultiplier = '倍率必须大于 0。'
  if (!Number.isFinite(form.monthlyQuotaAmount) || !form.monthlyQuotaAmount || form.monthlyQuotaAmount <= 0) {
    next.monthlyQuota = `${quotaFieldLabel(selectedProductForValidation.value)}必须大于 0。`
  }
  if (form.totalSeats < 1 || form.totalSeats > 20 || form.occupiedSeats < 0 || form.occupiedSeats > form.totalSeats) next.seats = '名额必须满足总名额 1-20，且已上车人数不超过总名额。'
  if (!form.openingChannelCode) next.openingChannelCode = '请选择开通渠道。'
  if (!form.paymentMethodCodes.length) next.paymentMethodCodes = '至少选择一种付款方式。'
  if (!accessArrangementComplete(form, selectedProductForValidation.value)) {
    if (form.accessArrangementMode === 'not_allowed') next.accessArrangement = '共用账号、密码或登录态方案不能发布。'
    else if (hasForbiddenCredentialSharingText(form.accessArrangementNote)) next.accessArrangement = '安排说明不能包含共享主账号、密码、API Key、Session、Cookie、token 或登录态。'
    else if (requiresSubscriptionRiskAck(selectedProductForValidation.value, form) && !form.riskAcknowledged) next.accessArrangement = '请先确认该套餐的发布边界。'
    else next.accessArrangement = '请填写成员邀请、个人订阅费用分摊或站外访问安排说明。'
  }
  if (!warrantyComplete(form.warranty)) next.warranty = '请补全车主承诺规则。'
  if (!form.rulesNote.trim()) next.rulesNote = '请填写规则说明。'
  if (containsSensitiveContent([
    form.customProductName ?? '',
    form.warranty.compensationMethod ?? '',
    form.warranty.exclusions ?? '',
    form.accessArrangementNote,
    form.rulesNote,
  ])) next.sensitive = '请移除账号密码、session token、refresh token、API Key、付款二维码、银行卡号或其他敏感凭据。'

  if (!requireComplete) {
    delete next.product
    delete next.region
    delete next.monthlyPriceCny
    delete next.serviceMultiplier
    delete next.monthlyQuota
    delete next.openingChannelCode
    delete next.paymentMethodCodes
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
    linuxDoTopicUrl: form.linuxDoTopicUrl.trim(),
    parsedTopicId: form.parsedTopicId,
    productId: form.productId,
    customProductName: form.customProductName,
    regionCode: form.regionCode,
    monthlyPriceCny: form.monthlyPriceCny,
    serviceMultiplier: form.serviceMultiplier,
    monthlyQuotaAmount: form.monthlyQuotaAmount,
    totalSeats: form.totalSeats,
    occupiedSeats: form.occupiedSeats,
    openingChannelCode: form.openingChannelCode,
    paymentMethodCodes: form.paymentMethodCodes,
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

function parseTopic() {
  if (!form.linuxDoTopicUrl.trim()) {
    errors.linuxDoTopicUrl = '填写 linux.do 原帖链接后可读取回填。'
    toast.warning(errors.linuxDoTopicUrl)
    return
  }
  if (!isLinuxDoTopicUrl(form.linuxDoTopicUrl)) {
    errors.linuxDoTopicUrl = '请先填写 https://linux.do/t/* 原帖链接。'
    toast.warning(errors.linuxDoTopicUrl)
    return
  }
  parseTopicMutation.mutate()
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
  form.regionCode ? { label: '地区', status: 'done' } : { label: '地区', status: 'pending' },
  form.monthlyPriceCny && form.monthlyPriceCny > 0 ? { label: '月费', status: 'done' } : { label: '月费', status: 'pending' },
  form.serviceMultiplier && form.serviceMultiplier > 0 && form.monthlyQuotaAmount && form.monthlyQuotaAmount > 0 ? { label: '倍率和每月额度', status: 'done' } : { label: '倍率和每月额度', status: 'pending' },
  form.totalSeats >= 1 && form.totalSeats <= 20 && form.occupiedSeats >= 0 && form.occupiedSeats < form.totalSeats ? { label: '名额', status: 'done' } : { label: '名额', status: 'conflict' },
  form.openingChannelCode ? { label: '开通渠道', status: 'done' } : { label: '开通渠道', status: 'pending' },
  form.paymentMethodCodes.length ? { label: '付款方式', status: 'done' } : { label: '付款方式', status: 'pending' },
  accessArrangementComplete(form, selectedProductForValidation.value) ? { label: '访问安排与边界确认', status: 'done' } : { label: '访问安排与边界确认', status: 'conflict' },
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
  {
    label: form.linuxDoTopicUrl.trim() ? '已填写 linux.do 原帖' : '未绑定 linux.do 原帖',
    status: form.linuxDoTopicUrl.trim() ? 'done' : 'pending',
    description: form.linuxDoTopicUrl.trim() ? '原帖作为公开增信信息展示。' : '可发布后再复制文案到 linux.do 发帖并补充链接。',
  },
])

const reminders = computed(() => {
  const rows: string[] = []
  if (!form.linuxDoTopicUrl.trim()) rows.push('未填写原帖链接不影响发布，可发布后再补充原帖提升可信度。')
  if (form.linuxDoTopicUrl.trim() && !parsedTopic.value) rows.push('已填写原帖链接，提交前建议读取一次原帖。')
  if (parsedTopic.value && !parsedTopic.value.authorMatchesBoundUser) rows.push('原帖作者与当前绑定用户不一致，请先确认已绑定自己的 linux.do 身份。')
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
  form.warranty.compensationMethod ?? '',
  form.warranty.exclusions ?? '',
  form.accessArrangementNote,
  form.rulesNote,
]))
const canCopyPostText = computed(() => (
  canBuildLinuxDoPostText(form, regionsByCode.value, openingChannelsByCode.value, paymentMethodsByCode.value)
  && !hasSensitiveText.value
))
const postText = computed(() => buildLinuxDoPostText(
  form,
  catalogById.value,
  regionsByCode.value,
  openingChannelsByCode.value,
  paymentMethodsByCode.value,
  submittedId.value ? `${window.location.origin}/carpools/${submittedId.value}` : undefined,
))
const copyDisabledReason = computed(() => {
  if (hasSensitiveText.value) return '请先移除账号密码、token、API Key、付款二维码、银行卡号等敏感凭据。'
  if (!canBuildLinuxDoPostText(form, regionsByCode.value, openingChannelsByCode.value, paymentMethodsByCode.value)) {
    return '填写产品、地区、价格、名额、渠道、付款方式、访问安排、售后和买家须知后可生成发帖文案。'
  }
  return ''
})

async function copyPostText() {
  if (!canCopyPostText.value) {
    toast.warning(copyDisabledReason.value || '请先补全发帖文案所需字段。')
    return
  }
  try {
    await navigator.clipboard.writeText(postText.value)
    toast.success('已复制 linux.do 发帖文案')
  } catch {
    toast.warning('复制失败，请手动选择文案复制')
  }
}
</script>

<template>
  <div class="space-y-5" :class="canAccessPublishForm ? 'pb-[calc(96px+env(safe-area-inset-bottom))] sm:pb-0' : 'pb-0'">
    <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div>
        <h1 class="text-2xl font-semibold md:text-3xl">导入 / 发布车源</h1>
        <p class="mt-2 max-w-3xl text-sm text-muted-foreground">可导入 linux.do 原帖自动回填，也可以手动填写后直接发布。</p>
      </div>
      <div v-if="canAccessPublishForm" class="hidden gap-2 sm:flex">
        <Button variant="outline" :disabled="saveDraftMutation.isPending.value" @click="saveDraft"><Save class="h-4 w-4" />保存草稿</Button>
        <Button :disabled="submitReviewMutation.isPending.value" @click="submitReview"><Send class="h-4 w-4" />检查并发布</Button>
      </div>
    </div>

    <Card v-if="profileQuery.isPending.value" class="mx-auto max-w-2xl p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <Loader2 class="h-5 w-5 animate-spin" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold tracking-tight">正在确认发布资格</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">
            车源发布需要当前账号已绑定 linux.do，确认完成后会进入发布表单。
          </p>
        </div>
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
            发布车源前需要先登录账号，并完成 linux.do 身份绑定。linux.do 原帖链接是可选增信项，不是进入表单的前置条件。
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
            当前账号已登录，但还没有绑定 linux.do。发布资格看账号身份绑定；原帖链接仍然可以在发布后补充，用来提升公开可信度。
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

    <template v-else>
      <div class="rounded-lg border border-primary/15 bg-primary/5 p-3 sm:hidden">
        <div class="flex items-center justify-between gap-3 text-sm font-medium">
          <span>发布必填 {{ completedPublishTasks.length }} / {{ publishTasks.length }}</span>
          <span class="text-xs text-muted-foreground">{{ form.linuxDoTopicUrl.trim() ? '原帖已填写' : '原帖可选 · 手动发布' }}</span>
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
              <span class="ml-1">系统已默认 {{ defaultItems.filter(item => item.status === 'defaulted').length }} 项，可修改。</span>
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
          <span class="text-xs text-muted-foreground">你可以复制标准文案到 linux.do 发帖，发帖后再回来补充原帖链接。</span>
          <Button size="sm" variant="outline" :disabled="!canCopyPostText" @click="copyPostText">复制 linux.do 发帖文案</Button>
        </div>
      </div>

      <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,360px)] lg:items-start">
        <section class="space-y-4">
          <Card id="carpool-tool-linuxdo-import" class="overflow-hidden p-0 shadow-sm" :class="highlightedTaskKey === 'linuxDoImport' ? 'ring-2 ring-primary/60 ring-offset-2 ring-offset-background' : ''">
            <button
              type="button"
              class="flex w-full items-center justify-between gap-3 px-4 py-4 text-left"
              @click="linuxDoImportOpen = !linuxDoImportOpen"
            >
              <span class="min-w-0">
                <span class="block text-base font-semibold">导入 linux.do 原帖（可选）</span>
                <span class="mt-1 block text-xs leading-5 text-muted-foreground">一键导入可自动回填产品、价格、地区、渠道和支付信息；没有原帖也可以直接手动填写。</span>
              </span>
              <ChevronUp v-if="linuxDoImportOpen" class="h-4 w-4 shrink-0 text-muted-foreground" />
              <ChevronDown v-else class="h-4 w-4 shrink-0 text-muted-foreground" />
            </button>
            <div v-if="linuxDoImportOpen" class="border-t border-border px-4 pb-4 pt-3">
              <LinuxDoTopicImport
                v-model:topic-url="form.linuxDoTopicUrl"
                :parsed-topic="parsedTopic"
                :parse-pending="parseTopicMutation.isPending.value"
                :error="errors.linuxDoTopicUrl"
                embedded
                @parse="parseTopic"
              />
            </div>
          </Card>

          <CarpoolBasicInfoSection
            :form="form"
            :catalog="catalog"
            :regions="regionOptions"
            :errors="errors"
            :field-states="basicFieldStates"
            :highlighted-key="highlightedTaskKey ?? undefined"
          />
          <SeatCapacityEditor :form="form" :errors="errors" />
          <ChannelPaymentSection
            :form="form"
            :opening-channels="channelOptions"
            :payment-methods="paymentOptions"
            :errors="errors"
            :field-states="channelPaymentFieldStates"
            :highlighted-key="highlightedTaskKey ?? undefined"
          />
          <PublishSectionCard
            :index="4"
            title="访问安排与边界确认"
            description="选择买家加入方式，并说明访问安排；不得共享主账号、密码或登录态。"
            :status="sectionStatus.accessArrangement"
            :status-label="sectionStatusLabel(sectionStatus.accessArrangement)"
          >
            <div class="space-y-4">
              <div class="grid gap-2 md:grid-cols-2">
                <button
                  type="button"
                  class="rounded-md border px-3 py-2 text-left text-sm font-medium transition"
                  :class="form.accessArrangementMode === 'personal_account_cost_share' ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
                  @click="form.accessArrangementMode = 'personal_account_cost_share'"
                >
                  个人订阅费用分摊
                  <span class="mt-1 block text-xs font-normal text-muted-foreground">说明费用分摊方式，不填写任何登录凭据</span>
                </button>
                <button
                  type="button"
                  class="rounded-md border px-3 py-2 text-left text-sm font-medium transition"
                  :class="form.accessArrangementMode === 'provider_member_invitation' ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
                  @click="form.accessArrangementMode = 'provider_member_invitation'"
                >
                  成员邀请 / 团队席位
                  <span class="mt-1 block text-xs font-normal text-muted-foreground">Business workspace、团队邀请或独立座位</span>
                </button>
                <button
                  type="button"
                  class="rounded-md border px-3 py-2 text-left text-sm font-medium transition"
                  :class="form.accessArrangementMode === 'owner_managed_access' ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
                  @click="form.accessArrangementMode = 'owner_managed_access'"
                >
                  站外托管 / 中转
                  <span class="mt-1 block text-xs font-normal text-muted-foreground">说明中转或托管边界，不在平台保存凭据</span>
                </button>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">访问安排说明 <span class="text-xs text-primary">必填</span></span>
                <Textarea
                  v-model="form.accessArrangementNote"
                  class="min-h-24"
                  placeholder="例如：通过 ChatGPT Business workspace 邀请成员席位；或站外托管 / 中转安排由双方确认。平台不保存、不交付任何密码、管理员凭据、Session、Cookie 或 token。"
                />
              </label>
              <label v-if="requiresSubscriptionRiskAck(selectedProductForValidation, form)" class="flex gap-2 rounded-md border border-warning/25 bg-warning/10 p-3 text-xs leading-5 text-warning">
                <input v-model="form.riskAcknowledged" type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-current">
                <span>我确认已按上述访问安排发布该套餐，不会在平台填写、粘贴或要求买家提供主账号、密码、API Key、Session、Cookie、token 或其他登录凭据。</span>
              </label>
              <p v-if="errors.accessArrangement" class="text-xs text-destructive">{{ errors.accessArrangement }}</p>
              <p class="text-xs leading-5 text-muted-foreground">若流程需要共享主账号、密码、API Key、Session、Cookie、token 或登录态，则不能发布；不得在平台填写、粘贴或上传任何凭据。</p>
            </div>
          </PublishSectionCard>
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
                :parsed-topic="parsedTopic"
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
            :copy-enabled="canCopyPostText"
            :copy-disabled-reason="copyDisabledReason"
            :post-text="postText"
            :submit-pending="submitReviewMutation.isPending.value"
            @save-draft="saveDraft"
            @submit-review="submitReview"
            @copy-post-text="copyPostText"
            @jump-to-task="jumpToTask"
          />
        </div>
      </div>

      <div class="sticky bottom-0 z-30 grid grid-cols-2 gap-2 border-t border-border bg-background/95 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] backdrop-blur sm:hidden">
        <Button variant="outline" :disabled="saveDraftMutation.isPending.value" @click="saveDraft">保存草稿</Button>
        <Button :disabled="submitReviewMutation.isPending.value" @click="submitReview">检查并发布</Button>
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
              <button
                v-for="(task, index) in pendingPublishTasks"
                :key="task.key"
                type="button"
                class="flex w-full items-center gap-3 rounded-lg border border-border bg-background px-3 py-3 text-left text-sm"
                :class="hasTriedPublish ? 'border-warning/35' : ''"
                @click="mobileCheckOpen = false; jumpToTask(task.key)"
              >
                <span class="grid h-6 w-6 place-items-center rounded-full bg-warning/10 text-xs font-semibold text-warning">{{ index + 1 }}</span>
                <span class="min-w-0 flex-1">
                  <span class="block font-medium">{{ task.label }}</span>
                  <span class="mt-0.5 block text-xs text-muted-foreground">{{ task.description }}</span>
                </span>
                <span class="text-muted-foreground">→</span>
              </button>
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
