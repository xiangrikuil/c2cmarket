<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, CheckCircle2, ChevronDown, Clock3, Copy, Eye, EyeOff, FileCheck2, Headphones, KeyRound, QrCode, ShieldAlert, Star, WalletCards, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiQuotaPolicyStrip from '@/components/api-market/ApiQuotaPolicyStrip.vue'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import ApiRefundPolicyEvidence from '@/components/api-order/ApiRefundPolicyEvidence.vue'
import DisputeEvidencePicker from '@/components/api-order/DisputeEvidencePicker.vue'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import ReviewDialog from '@/components/review/ReviewDialog.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Stepper, StepperDescription, StepperIndicator, StepperItem, StepperSeparator, StepperTitle, StepperTrigger } from '@/components/ui/stepper'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import {
  apiOrderBuyerContactSnapshot,
  apiOrderMerchantContactSnapshot,
  getApiOrderDeliveryKindLabel,
	getApiOrderDisputeStatusDescription,
	getApiOrderDisputeStatusLabel,
  getApiOrderDisplayStatus,
  getApiOrderEventLabel,
  getApiOrderEvents,
  getApiOrderPaymentIssueLabel,
  getApiOrderStatusLabel,
  getApiQuotaDeliveryModeLabel,
  getApiQuotaDistributionLabel,
  getApiQuotaSaleModeLabel,
  getApiTTFTBandLabel,
  getApiUsageVisibilityLabel,
	isApiOrderDisputeActive,
  readApiOrderPaymentInstructions,
  type ApiOrderDeliveryKind,
  type ApiOrderPaymentIssueReason,
  type ApiServiceCommercialSnapshot,
} from '@/lib/api'
import {
  API_ORDER_CANCEL_OPTIONS,
  buildApiOrderCancelReason,
  formatApiOrderCancelReason,
  formatOrderDateTime,
  orderCountdown,
} from '@/lib/apiOrderUi'
import { apiPaymentMethodLabels, apiPaymentMethodRequiresQrCode } from '@/lib/apiPaymentSettings'
import {
  apiOrderCommercialOutcomeLabels,
  apiOrderDisputeIssueLabels,
  apiOrderDisputeResolutionLabels,
  type ApiOrderDisputeIssueCode,
  type ApiOrderDisputeResolution,
} from '@/lib/apiOrderDispute'
import { formatDecimal } from '@/lib/decimal'
import type { DisputeEvidenceAsset } from '@/lib/disputeEvidenceBackend'
import { functionalMotion } from '@/lib/motion'
import {
  useApiOrder,
  useCancelApiOrderMutation,
  useConfirmApiOrderCompleteMutation,
  useConfirmApiOrderPaymentMutation,
  useOpenApiOrderDisputeMutation,
  useReportApiOrderPaymentIssueMutation,
  useReportLateApiOrderPaymentMutation,
  useResolveLateApiOrderPaymentMutation,
  useReviewCenterRows,
  useSubmitApiOrderDeliveryCredentialMutation,
  useSubmitApiOrderPaymentMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const id = computed(() => String(route.params.id ?? ''))
const perspective = computed<'buyer' | 'merchant'>(() => route.name === 'merchant-api-order-detail' ? 'merchant' : 'buyer')
const isMerchantView = computed(() => perspective.value === 'merchant')
const { data: order, isLoading, error: orderError, refetch: refetchOrder } = useApiOrder(id, perspective)
const { data: reviewCenter } = useReviewCenterRows()
const ordinaryActionsPaused = computed(() => Boolean(order.value && (
  isApiOrderDisputeActive(order.value.disputeStatus)
  || order.value.catalogRiskHold?.status === 'active'
)))
const paymentInstructionsQuery = useQuery({
  queryKey: computed(() => ['api-order-payment-instructions', id.value]),
  queryFn: () => readApiOrderPaymentInstructions(id.value),
	enabled: computed(() => Boolean(order.value && !isMerchantView.value && order.value.status === 'pending_payment' && !ordinaryActionsPaused.value)),
  retry: false,
})

const paymentSummary = ref('')
const deliveryKind = ref<ApiOrderDeliveryKind>('api_key_endpoint')
const apiBaseUrl = ref('')
const apiKey = ref('')
const panelLoginUrl = ref('')
const username = ref('')
const password = ref('')
const deliveryInstructions = ref('')
const paymentDialogOpen = ref(false)
const paymentConfirmOpen = ref(false)
const paymentIssueDialogOpen = ref(false)
const paymentIssueReason = ref<ApiOrderPaymentIssueReason | ''>('')
const paymentIssueNote = ref('')
const paymentIssueResponseOpen = ref(false)
const latePaymentDialogOpen = ref(false)
const latePaymentNote = ref('')
const latePaymentResolutionOpen = ref(false)
const latePaymentResolution = ref<'not_received' | 'received_refund_pending'>('not_received')
const latePaymentResolutionNote = ref('')
const disputeDialogOpen = ref(false)
const disputeIssueCode = ref<ApiOrderDisputeIssueCode>('service_unavailable')
const disputeRequestedResolution = ref<ApiOrderDisputeResolution>('full_refund')
const disputeRequestedAmount = ref('')
const disputeIssueOccurredAt = ref('')
const disputeReason = ref('')
const disputeEvidence = ref<DisputeEvidenceAsset[]>([])
const completionConfirmOpen = ref(false)
const credentialProblemOpen = ref(false)
const credentialProblemReason = ref<CredentialProblemReason | ''>('')
const credentialProblemNote = ref('')
const cancelDrawerOpen = ref(false)
const cancelReason = ref('')
const cancelNote = ref('')
const cancelUnpaidConfirmed = ref(false)
const orderDetailsOpen = ref(true)
const orderRecordsOpen = ref(false)
const apiKeyVisible = ref(false)
const passwordVisible = ref(false)
const now = ref(Date.now())
let countdownTimer: ReturnType<typeof setInterval> | undefined

const submitPaymentMutation = useSubmitApiOrderPaymentMutation()
const cancelOrderMutation = useCancelApiOrderMutation()
const confirmCompleteMutation = useConfirmApiOrderCompleteMutation()
const confirmPaymentMutation = useConfirmApiOrderPaymentMutation()
const reportPaymentIssueMutation = useReportApiOrderPaymentIssueMutation()
const reportLatePaymentMutation = useReportLateApiOrderPaymentMutation()
const resolveLatePaymentMutation = useResolveLateApiOrderPaymentMutation()
const openDisputeMutation = useOpenApiOrderDisputeMutation()
const submitDeliveryMutation = useSubmitApiOrderDeliveryCredentialMutation()

const backPath = computed(() => isMerchantView.value ? '/merchant/api-orders' : '/my/api-orders')
const backLabel = computed(() => isMerchantView.value ? '返回 API 销售订单' : '返回 API 购买订单')
const canSubmitPayment = computed(() => !ordinaryActionsPaused.value && !isMerchantView.value && order.value?.status === 'pending_payment')
const canResubmitPayment = computed(() => !ordinaryActionsPaused.value && !isMerchantView.value && order.value?.status === 'payment_issue')
const canConfirmPayment = computed(() => !ordinaryActionsPaused.value && isMerchantView.value && order.value?.status === 'payment_submitted')
const canReportPaymentIssue = computed(() => !ordinaryActionsPaused.value && isMerchantView.value && order.value?.status === 'payment_submitted')
const canReportLatePayment = computed(() => !isMerchantView.value && Boolean(order.value?.canReportLatePayment))
const canResolveLatePayment = computed(() => isMerchantView.value && order.value?.latePaymentStatus === 'reported')
const canSubmitDelivery = computed(() => !ordinaryActionsPaused.value && isMerchantView.value && order.value?.status === 'paid_confirmed' && !order.value.deliveryCredential)
const canConfirmComplete = computed(() => !ordinaryActionsPaused.value && !isMerchantView.value && order.value?.status === 'delivery_submitted')
const canReportCredentialProblem = computed(() => canConfirmComplete.value)
const canOpenDispute = computed(() => Boolean(
  order.value
	&& (order.value.canOpenDispute ?? (order.value.status !== 'cancelled' && order.value.status !== 'completed' && (order.value.disputeStatus ?? 'none') === 'none')),
))
const showDisputeStatus = computed(() => Boolean(order.value?.hasDisputeHistory || (order.value?.disputeStatus ?? 'none') !== 'none'))
const disputePanelId = computed(() => order.value?.disputeCaseId ?? order.value?.latestDisputeCaseId ?? '')
const canSubmitDispute = computed(() => Boolean(
  disputeReason.value.trim()
	&& (disputeRequestedResolution.value !== 'partial_refund' || disputeRequestedAmount.value.trim())
	&& (order.value?.status !== 'completed' || disputeIssueOccurredAt.value),
))
const canOpenReviewCenter = computed(() => order.value?.status === 'completed')
const reviewRow = computed(() => {
  const matches = reviewCenter.value?.items.filter(item => item.transactionType === 'api_order' && item.transactionId === id.value) ?? []
  return matches.find(item => item.direction === 'pending') ?? matches.find(item => item.direction === 'sent') ?? matches[0] ?? null
})
const reviewDialogOpen = computed(() => route.query.review === 'open')
const counterpartyReputation = computed(() => {
  if (!order.value) return null
  return isMerchantView.value ? order.value.buyerReputation : order.value.sellerReputation
})
const counterpartyName = computed(() => {
  if (!order.value) return ''
  return isMerchantView.value ? order.value.buyer : order.value.seller
})
const counterpartyAvatarText = computed(() => counterpartyName.value.trim().slice(0, 1).toUpperCase() || '用')
const counterpartyCompletedOrders = computed(() => {
  const summary = counterpartyReputation.value
  return summary ? `${summary.completedCount} 单` : '暂无数据'
})
const counterpartyCompletionRate = computed(() => {
  const value = counterpartyReputation.value?.roleCompletionRate
  return value === null || value === undefined ? '暂无数据' : `${Math.round(value * 100)}%`
})
const merchantContactSnapshot = computed(() => !isMerchantView.value && order.value ? apiOrderMerchantContactSnapshot(order.value) : null)
const buyerContactSnapshot = computed(() => isMerchantView.value && order.value ? apiOrderBuyerContactSnapshot(order.value) : null)
const events = computed(() => order.value ? getApiOrderEvents(order.value) : [])
const orderModelSnapshotLabel = computed(() => {
  const snapshot = order.value?.intentSnapshot
  if (!snapshot) return ''
  if (snapshot.models.length) return snapshot.models.join(' / ')
  return snapshot.pricingSnapshotIssue === 'invalid' ? '订单模型快照不可用' : '历史订单未冻结模型信息'
})
const orderUsageVisibilityLabel = computed(() => {
  const snapshot = order.value?.intentSnapshot
  if (!snapshot) return ''
  return snapshot.usageVisibilitySnapshotMissing ? '历史订单未冻结用量核对规则' : getApiUsageVisibilityLabel(snapshot.usageVisibility)
})

function commercialSnapshotFallback(snapshot: ApiServiceCommercialSnapshot) {
  return snapshot.commercialFactsSnapshotIssue === 'invalid' ? '订单快照不可用' : '历史订单未冻结'
}

function accountPoolSnapshotLabel(snapshot: ApiServiceCommercialSnapshot) {
  if (snapshot.accountPoolLabel) return snapshot.accountPoolLabel
  return snapshot.commercialFactsSnapshotIssue ? commercialSnapshotFallback(snapshot) : '历史服务未补充'
}

function concurrencySnapshotLabel(snapshot: ApiServiceCommercialSnapshot) {
  if (snapshot.declaredMaxConcurrency !== undefined) return snapshot.declaredMaxConcurrency
  return snapshot.commercialFactsSnapshotIssue ? commercialSnapshotFallback(snapshot) : '历史服务未声明'
}

function refundCommitmentSnapshotLabel(snapshot: ApiServiceCommercialSnapshot) {
  if (snapshot.merchantRefundCommitment === true) return '商户全额退款承诺'
  if (snapshot.merchantRefundCommitment === false) return '无额外退款承诺'
  return commercialSnapshotFallback(snapshot)
}

function serviceValiditySnapshotLabel(snapshot: ApiServiceCommercialSnapshot) {
  if (snapshot.serviceValidityExpiresAt) return formatOrderDateTime(snapshot.serviceValidityExpiresAt)
  if (snapshot.serviceValidityExpiresAt === null && !snapshot.commercialFactsSnapshotIssue) return '未设置固定失效时间'
  return commercialSnapshotFallback(snapshot)
}

const orderServiceValidityLabel = computed(() => {
  if (!order.value) return ''
  if (order.value.packageExpiresAt) return formatOrderDateTime(order.value.packageExpiresAt)
  if (order.value.packageSnapshot) return `${order.value.packageSnapshot.durationDays} 天（商户交付后开始）`
  return serviceValiditySnapshotLabel(order.value.intentSnapshot)
})
const orderQuotaExpiryLabel = computed(() => {
  if (!order.value) return ''
  if (order.value.quotaSnapshot) return formatOrderDateTime(order.value.quotaSnapshot.expiresAt)
  if (order.value.packageExpiresAt) return formatOrderDateTime(order.value.packageExpiresAt)
  if (order.value.packageSnapshot) return `交付后 ${order.value.packageSnapshot.durationDays} 天`
  return serviceValiditySnapshotLabel(order.value.intentSnapshot)
})
const paymentInstructions = computed(() => paymentInstructionsQuery.data.value ?? null)
const disputeValidityExpiresAt = computed(() => {
	const raw = order.value?.packageExpiresAt
		?? order.value?.quotaSnapshot?.expiresAt
		?? order.value?.intentSnapshot.serviceValidityExpiresAt
	if (!raw) return null
	const timestamp = new Date(raw).getTime()
	return Number.isFinite(timestamp) ? timestamp : null
})
const disputeOccurrenceMax = computed(() => {
	const value = new Date(Math.min(now.value, disputeValidityExpiresAt.value ?? now.value))
	value.setMinutes(value.getMinutes() - value.getTimezoneOffset())
	return value.toISOString().slice(0, 16)
})
const paymentActionLabel = computed(() => {
  const method = order.value?.selectedPaymentMethod
  return method && apiPaymentMethodRequiresQrCode(method) ? '查看收款码' : '查看付款信息'
})
const canConfirmOffPlatformPayment = computed(() => {
  if (!paymentInstructions.value) return false
  return !apiPaymentMethodRequiresQrCode(paymentInstructions.value.paymentMethod) || Boolean(paymentInstructions.value.paymentQrCodeDataUrl)
})
const actionBusy = computed(() => cancelOrderMutation.isPending.value || submitPaymentMutation.isPending.value || confirmCompleteMutation.isPending.value || confirmPaymentMutation.isPending.value || reportPaymentIssueMutation.isPending.value || reportLatePaymentMutation.isPending.value || resolveLatePaymentMutation.isPending.value || openDisputeMutation.isPending.value || submitDeliveryMutation.isPending.value)
const newDisputeResolutionLabels = computed(() => Object.fromEntries(
  Object.entries(apiOrderDisputeResolutionLabels).filter(([value]) => value !== 'continue_fulfillment'),
))
const paymentIssueOptions: Array<{ value: ApiOrderPaymentIssueReason, label: string, description: string }> = [
  { value: 'not_received', label: '未到账', description: '收款记录中暂未找到对应付款。' },
  { value: 'amount_mismatch', label: '金额不符', description: '实收金额与订单金额不一致。' },
  { value: 'remark_mismatch', label: '备注不符', description: '付款备注或订单识别信息不一致。' },
]
type CredentialProblemReason = 'unreachable' | 'invalid_credential' | 'quota_mismatch' | 'permission_mismatch' | 'description_mismatch' | 'other'
const credentialProblemOptions: Array<{ value: CredentialProblemReason, label: string, description: string }> = [
  { value: 'unreachable', label: '无法连接', description: '接入地址无法访问或服务没有响应。' },
  { value: 'invalid_credential', label: '凭证无效', description: 'API Key、账号或初始密码无法使用。' },
  { value: 'quota_mismatch', label: '额度不符', description: '可用额度与订单快照不一致。' },
  { value: 'permission_mismatch', label: '权限不符', description: '模型、并发或接口权限与约定不一致。' },
  { value: 'description_mismatch', label: '与描述不符', description: '交付内容与服务或套餐说明不一致。' },
  { value: 'other', label: '其他问题', description: '以上原因无法准确描述当前问题。' },
]
const credentialProblemSubmitDisabled = computed(() => !credentialProblemReason.value
  || (credentialProblemReason.value === 'other' && !credentialProblemNote.value.trim()))
const flowSteps = ['创建订单', '买家付款', '商户确认收款', '商户交付', '买家核验']
const flowStepDescriptions = ['锁定下单信息', '使用商户收款方式付款', '核对实际到账', '完成一次性交付', '确认可用或核验期自动结束']
const currentFlowIndex = computed(() => {
  if (!order.value || order.value.status === 'cancelled') return -1
  const indexes = {
    pending_payment: 1,
    payment_submitted: 2,
    payment_issue: 1,
    paid_confirmed: 3,
    delivery_submitted: 4,
    completed: 5,
  } as const
  return indexes[order.value.status]
})
const orderAmountText = computed(() => order.value ? formatDecimal(order.value.amountDecimal || String(order.value.amount), 2, 2) : '0.00')
const orderAllowanceText = computed(() => order.value ? formatDecimal(order.value.requestedUsdAllowanceDecimal || String(order.value.requestedUsdAllowance), 2, 6) : '0.00')
const activeDeadline = computed(() => {
  if (order.value?.status === 'pending_payment') return order.value.paymentExpiresAt
  if (order.value?.status === 'payment_submitted') return order.value.merchantConfirmDueAt
  if (order.value?.status === 'paid_confirmed') return order.value.deliveryDueAt
  if (!isMerchantView.value && order.value?.status === 'delivery_submitted') return order.value.deliveryReviewExpiresAt
  return null
})
const countdown = computed(() => orderCountdown(activeDeadline.value, now.value))
const countdownLabel = computed(() => {
  if (order.value?.status !== 'delivery_submitted') return countdown.value.label
  const hours = String(Math.floor(countdown.value.totalSeconds / 3600)).padStart(2, '0')
  const minutes = String(Math.floor((countdown.value.totalSeconds % 3600) / 60)).padStart(2, '0')
  const seconds = String(countdown.value.totalSeconds % 60).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
})
const countdownTitle = computed(() => {
  if (order.value?.status === 'pending_payment') return `请在 ${order.value.paymentWindowMinutes} 分钟内完成付款`
  if (order.value?.status === 'delivery_submitted') return '凭证核验剩余时间'
  if (order.value?.status === 'payment_submitted') return '商户核对收款剩余时间'
  return '商户交付剩余时间'
})
const pageTitle = computed(() => isMerchantView.value ? 'API 销售订单' : 'API 购买订单')
const selectedCancelOption = computed(() => API_ORDER_CANCEL_OPTIONS.find(item => item.value === cancelReason.value))
const canCancelOrder = computed(() => !ordinaryActionsPaused.value && !isMerchantView.value && order.value?.status === 'pending_payment')
const cancelSubmitDisabled = computed(() => {
  if (!selectedCancelOption.value || !cancelUnpaidConfirmed.value) return true
  return Boolean(selectedCancelOption.value.requiresNote && !cancelNote.value.trim())
})
const showMerchantTimeout = computed(() => Boolean(order.value?.merchantConfirmOverdue || order.value?.deliveryOverdue))
const currentActionDescription = computed(() => {
  if (!order.value) return ''
  if (order.value.status === 'cancelled') return '订单已取消，无需继续操作。'
	if (order.value.catalogRiskHold?.status === 'active') return '关联模型目录被紧急阻断，付款、核款、交付、确认完成及自动超时均已暂停；仍可查看订单证据或发起纠纷。'
	if (ordinaryActionsPaused.value) return '订单纠纷处理中，付款、取消、核款、交付、确认完成及自动超时流程均已暂停；请进入独立纠纷页面查看当前处理进度。'
  if (isMerchantView.value) {
    if (order.value.status === 'pending_payment') return '买家尚未标记付款，当前无需操作。'
    if (order.value.status === 'payment_submitted') return '买家已标记付款，请核对收款账户实际到账后确认。'
    if (order.value.status === 'payment_issue') return '已报告付款问题，正在等待买家补充付款信息。'
    if (order.value.status === 'paid_confirmed') return '收款已确认，请填写买家专属的接入信息。'
    if (order.value.status === 'delivery_submitted') return '已完成交付，无需继续操作；买家可在核验期内确认可用或报告问题。'
    if (order.value.completionSource === 'auto_completed') return '核验期已结束，订单由系统自动完成。'
    return '买家已确认凭证可用，这笔交易已完成。'
  }
  if (order.value.status === 'pending_payment') return '查看本次订单的收款信息，完成付款后确认付款状态。'
  if (order.value.status === 'payment_submitted') return '付款状态已提交，等待商户核对收款。'
  if (order.value.status === 'payment_issue') return '商户发现付款信息不匹配，请补充说明后重新提交。'
  if (order.value.status === 'paid_confirmed') return '商户已确认收款，等待商户提交交付凭证。'
  if (order.value.status === 'delivery_submitted') return isApiOrderDisputeActive(order.value.disputeStatus)
    ? '凭证问题正在处理中，自动完成计时已暂停。'
    : '请在核验期内确认凭证可用或报告问题；未反馈时订单将自动完成。'
  if (order.value.completionSource === 'auto_completed') return '核验期内未报告问题，订单已自动完成；交付凭证仍可查看。'
  return '你已确认凭证可用；交付凭证仍可在本页查看。'
})

function legacyRevocationCopy(value: string) {
  return value
    .replace(/买家专属、可撤销的/g, '买家专属的')
    .replace(/支持撤销/g, '支持双方协商更换')
}

function paymentSummaryValue() {
  return paymentSummary.value.trim() || '买家已按商户收款资料完成付款，等待商户核对。'
}

async function refresh(orderId: string) {
  await queryClient.invalidateQueries({ queryKey: ['api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['my-api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['merchant-api-orders'] })
  await queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['api-order-payment-instructions', orderId] })
  await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
}

async function submitPayment() {
  if (!order.value) return
  try {
    await submitPaymentMutation.mutateAsync({ id: order.value.id, paymentSummary: paymentSummaryValue(), version: order.value.version })
    paymentConfirmOpen.value = false
    paymentDialogOpen.value = false
    await refresh(order.value.id)
    toast.success('已标记付款，等待商户确认收款。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交付款状态失败。')
  }
}

async function cancelOrder() {
  if (!order.value) return
  try {
    const reason = buildApiOrderCancelReason(cancelReason.value, cancelNote.value)
    await cancelOrderMutation.mutateAsync({ id: order.value.id, reason, version: order.value.version })
    cancelDrawerOpen.value = false
    await refresh(order.value.id)
    toast.success('订单已取消，商户将收到取消说明。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '取消订单失败。')
  }
}

async function reportLatePayment() {
  if (!order.value) return
  try {
    await reportLatePaymentMutation.mutateAsync({ id: order.value.id, note: latePaymentNote.value.trim(), version: order.value.version })
    latePaymentDialogOpen.value = false
    latePaymentNote.value = ''
    await refresh(order.value.id)
    toast.success('逾期付款已记录，等待卖家核对。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '逾期付款报告失败。')
  }
}

async function resolveLatePayment() {
  if (!order.value) return
  try {
    await resolveLatePaymentMutation.mutateAsync({
      id: order.value.id,
      status: latePaymentResolution.value,
      note: latePaymentResolutionNote.value.trim(),
      version: order.value.version,
    })
    latePaymentResolutionOpen.value = false
    latePaymentResolutionNote.value = ''
    await refresh(order.value.id)
    toast.success('逾期付款核对结果已记录。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '核对结果提交失败。')
  }
}

async function confirmPayment() {
  if (!order.value) return
  try {
    await confirmPaymentMutation.mutateAsync({ id: order.value.id, version: order.value.version })
    await refresh(order.value.id)
    toast.success('已确认收款，请填写交付信息。')
    await nextTick()
    scrollToDeliveryForm()
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '确认收款失败。')
  }
}

async function reportPaymentIssue() {
  if (!order.value || !paymentIssueReason.value) return
  try {
    await reportPaymentIssueMutation.mutateAsync({
      id: order.value.id,
      reason: paymentIssueReason.value,
      note: paymentIssueNote.value,
      version: order.value.version,
    })
    paymentIssueDialogOpen.value = false
    paymentIssueReason.value = ''
    paymentIssueNote.value = ''
    await refresh(order.value.id)
    toast.success('已通知买家补充付款信息。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '报告付款问题失败。')
  }
}

async function submitOrderDispute() {
	if (!order.value || !canSubmitDispute.value) return
  try {
    await openDisputeMutation.mutateAsync({
      id: order.value.id,
      input: {
        issueCode: disputeIssueCode.value,
        requestedResolution: disputeRequestedResolution.value,
        requestedAmountCny: disputeRequestedResolution.value === 'partial_refund' ? disputeRequestedAmount.value.trim() : null,
				issueOccurredAt: disputeIssueOccurredAt.value ? new Date(disputeIssueOccurredAt.value).toISOString() : null,
        reason: disputeReason.value.trim(),
        evidenceAssetIds: disputeEvidence.value.map(item => item.id),
      },
      version: order.value.version,
      perspective: perspective.value,
    })
    disputeDialogOpen.value = false
    disputeRequestedAmount.value = ''
		disputeIssueOccurredAt.value = ''
    disputeReason.value = ''
    disputeEvidence.value = []
    await refresh(order.value.id)
    toast.success('订单纠纷已发起，已进入平台处理。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交订单问题失败。')
  }
}

function openPaymentIssueResponse() {
  paymentSummary.value = order.value?.paymentSummary ?? ''
  paymentIssueResponseOpen.value = true
}

async function resubmitPayment() {
  if (!order.value || !paymentSummary.value.trim()) return
  try {
    await submitPaymentMutation.mutateAsync({ id: order.value.id, paymentSummary: paymentSummary.value.trim(), version: order.value.version })
    paymentIssueResponseOpen.value = false
    await refresh(order.value.id)
    toast.success('付款信息已重新提交，等待商户核对。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '重新提交付款信息失败。')
  }
}

async function confirmComplete() {
  if (!order.value) return
  try {
    await confirmCompleteMutation.mutateAsync({ id: order.value.id, version: order.value.version })
    completionConfirmOpen.value = false
    await refresh(order.value.id)
    toast.success('已确认凭证可用，订单完成。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '确认凭证可用失败。')
  }
}

async function submitCredentialProblem() {
  if (!order.value || !credentialProblemReason.value) return
  const option = credentialProblemOptions.find(item => item.value === credentialProblemReason.value)
  if (!option) return
  const reason = `凭证异常｜${option.label}${credentialProblemNote.value.trim() ? `｜补充说明：${credentialProblemNote.value.trim()}` : ''}`
  try {
    await openDisputeMutation.mutateAsync({
      id: order.value.id,
      input: {
        issueCode: 'service_unavailable',
        requestedResolution: 'full_refund',
        requestedAmountCny: null,
        reason,
      },
      version: order.value.version,
      perspective: 'buyer',
    })
    credentialProblemOpen.value = false
    credentialProblemReason.value = ''
    credentialProblemNote.value = ''
    await refresh(order.value.id)
    toast.success('凭证问题已提交，自动完成计时已暂停。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交凭证问题失败。')
  }
}

function openReviewCenter() {
  router.push({ query: { ...route.query, review: 'open' } })
}

function setReviewDialogOpen(open: boolean) {
  if (open) return router.push({ query: { ...route.query, review: 'open' } })
  const query = { ...route.query }
  delete query.review
  router.push({ query })
}

function scrollToDeliveryForm() {
  document.getElementById('api-order-delivery-form')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function deliveryPayload() {
  if (deliveryKind.value === 'login_account') {
    return {
      deliveryKind: deliveryKind.value,
      panelLoginUrl: panelLoginUrl.value,
      username: username.value,
      password: password.value,
      instructions: deliveryInstructions.value,
    }
  }
  return {
    deliveryKind: deliveryKind.value,
    apiBaseUrl: apiBaseUrl.value,
    apiKey: apiKey.value,
    instructions: deliveryInstructions.value,
  }
}

async function submitDelivery() {
  if (!order.value) return
  try {
    await submitDeliveryMutation.mutateAsync({ id: order.value.id, payload: deliveryPayload(), version: order.value.version })
    await refresh(order.value.id)
    toast.success('交付信息已提交，买家可长期查看。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交交付信息失败。')
  }
}

async function copyValue(value: string | undefined, label: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    toast.success(`已复制${label}。`)
  } catch {
    toast.error('复制失败，请手动选择文本。')
  }
}

function maskCredential(value: string | undefined) {
  if (!value) return ''
  if (value.length <= 8) return '••••••••'
  const maskedLength = Math.min(18, Math.max(8, value.length - 7))
  return `${value.slice(0, 3)}${'•'.repeat(maskedLength)}${value.slice(-4)}`
}

async function openPaymentConfirmation() {
  if (!paymentInstructions.value) {
    toast.warning('收款资料仍在加载，请稍后重试。')
    return
  }
  paymentDialogOpen.value = false
  await nextTick()
  paymentConfirmOpen.value = true
}

async function returnToPaymentDetails() {
  paymentConfirmOpen.value = false
  await nextTick()
  paymentDialogOpen.value = true
}

onMounted(() => {
  countdownTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})
</script>

<template>
  <ReviewDialog :open="reviewDialogOpen && Boolean(reviewRow)" :row="reviewRow" @update:open="setReviewDialogOpen" />
  <SkeletonBlock v-if="isLoading" :lines="9" />
  <ErrorState v-else-if="orderError" description="API 订单暂时无法加载，或当前账号无权查看。" @retry="refetchOrder()" />
  <EmptyState v-else-if="!order" title="未找到 API 订单" description="该订单不存在或暂不可见。"><template #action><Button variant="outline" @click="router.push(backPath)">{{ backLabel }}</Button></template></EmptyState>
  <div v-else class="space-y-4">
    <header class="flex flex-col gap-3 px-1 md:flex-row md:items-start md:justify-between">
      <div class="min-w-0">
        <Button class="-ml-3 mb-2" variant="ghost" size="sm" @click="router.push(backPath)"><ArrowLeft class="h-4 w-4" />{{ backLabel }}</Button>
        <div v-auto-animate="functionalMotion" class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold">{{ pageTitle }}</h1>
          <StatusBadge :key="order.status" :status="order.status" :label="getApiOrderDisplayStatus(order, perspective)" />
          <Badge v-if="order.purchaseKind === 'limited_quota_offer'" variant="capability">限量额度包</Badge>
        </div>
        <p class="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-1 gap-y-1 text-sm text-muted-foreground" :title="order.serviceTitle">
          <span class="min-w-0 truncate">{{ order.serviceTitle }}</span>
          <span aria-hidden="true">·</span>
          <span>订单号</span>
          <ShortId :value="order.orderNo" full copyable />
        </p>
      </div>
      <Button v-if="canCancelOrder" variant="outline" class="border-destructive/40 text-destructive hover:bg-destructive/5 hover:text-destructive" @click="cancelDrawerOpen = true">
        <XCircle class="h-4 w-4" />取消订单
      </Button>
    </header>

    <Alert v-if="order.status === 'cancelled'" variant="destructive">
      <XCircle />
      <AlertTitle>订单已取消</AlertTitle>
      <AlertDescription class="space-y-3">
        <div>{{ formatApiOrderCancelReason(order.cancelReason) }}</div>
        <Button v-if="canReportLatePayment" size="sm" variant="outline" @click="latePaymentDialogOpen = true"><WalletCards class="h-4 w-4" />我已发生逾期付款</Button>
      </AlertDescription>
    </Alert>

    <Alert v-if="order.latePaymentStatus" class="border-warning/35 bg-warning/10">
      <WalletCards class="text-warning" />
      <AlertTitle>{{ order.latePaymentStatus === 'reported' ? '逾期付款待核对' : order.latePaymentStatus === 'not_received' ? '卖家未查到到账' : '已到账，待线下退款' }}</AlertTitle>
      <AlertDescription class="space-y-3">
        <div>该记录不会恢复订单、库存或抢购资格。<span v-if="order.latePaymentNote">说明：{{ order.latePaymentNote }}</span></div>
        <Button v-if="canResolveLatePayment" size="sm" variant="outline" @click="latePaymentResolutionOpen = true">核对付款</Button>
      </AlertDescription>
    </Alert>

    <Alert v-if="!isMerchantView && order.status === 'pending_payment'">
      <ShieldAlert />
      <AlertTitle>转账不会自动更新订单</AlertTitle>
      <AlertDescription>完成站外转账后，请在付款截止前立即点击“我已完成付款”，否则订单仍会按未付款超时处理。</AlertDescription>
    </Alert>

    <Alert v-if="order.status === 'payment_issue'" class="border-warning/35 bg-warning/10">
      <ShieldAlert class="text-warning" />
      <AlertTitle>等待买家补充付款信息</AlertTitle>
      <AlertDescription>
        <div>商户核对结果：{{ getApiOrderPaymentIssueLabel(order.paymentIssueReason) }}</div>
        <div v-if="order.paymentIssueNote" class="mt-1">商户说明：{{ order.paymentIssueNote }}</div>
        <div class="mt-1 text-xs">请不要重复付款；核对实际付款记录后补充时间、金额、备注或尾号并重新提交。</div>
      </AlertDescription>
    </Alert>

    <Alert v-if="showDisputeStatus" :class="isApiOrderDisputeActive(order.disputeStatus) ? 'border-warning/35 bg-warning/10' : 'border-border bg-muted/20'">
      <ShieldAlert :class="isApiOrderDisputeActive(order.disputeStatus) ? 'text-warning' : 'text-muted-foreground'" />
      <AlertTitle>{{ getApiOrderDisputeStatusLabel(order.disputeStatus) }}</AlertTitle>
      <AlertDescription>
        <p>{{ getApiOrderDisputeStatusDescription(order.disputeStatus) || '该订单有历史纠纷记录，可查看最近案件的终局与申诉期限。' }}</p>
        <Button v-if="disputePanelId" class="mt-3" size="sm" variant="outline" @click="router.push({ path: `/my/disputes/${disputePanelId}`, query: { from: isMerchantView ? 'merchant' : 'buyer', orderId: order.id } })">
          <Headphones class="h-4 w-4" />进入纠纷处理
        </Button>
      </AlertDescription>
    </Alert>

    <Alert v-if="order.quotaValidityIssueAt" variant="destructive">
      <Clock3 />
      <AlertTitle>首次交付有效期不足</AlertTitle>
      <AlertDescription>交付时额度剩余不足 60 分钟，系统已拒绝本次交付并记录异常；不会自动替换额度、延长有效期或恢复库存。</AlertDescription>
    </Alert>

    <Alert v-if="order.commercialOutcome !== 'pending'">
      <FileCheck2 />
      <AlertTitle>{{ apiOrderCommercialOutcomeLabels[order.commercialOutcome] }}</AlertTitle>
      <AlertDescription>该商业结果独立于订单履约状态，用于评价资格和售后事实，不代表平台代收、退款或验真。</AlertDescription>
    </Alert>

    <Alert v-if="order.catalogRiskHold?.status === 'active'" variant="destructive">
      <ShieldAlert />
      <AlertTitle>目录风险暂停处理中</AlertTitle>
      <AlertDescription>
        {{ order.catalogRiskHold.reason }} 付款、核款、交付、确认完成和自动超时已暂停；订单证据与纠纷入口保持可用。
      </AlertDescription>
    </Alert>

    <Alert v-if="isMerchantView && order.status === 'delivery_submitted'" class="border-success/35 bg-success/10">
      <CheckCircle2 class="text-success" />
      <AlertTitle>已完成交付</AlertTitle>
      <AlertDescription>你的履约任务已经结束，无需等待买家点击确认。订单将在买家确认可用或 24 小时核验期结束后完成。</AlertDescription>
    </Alert>

    <section v-if="isMerchantView" class="border-y border-border bg-card px-4 py-5 sm:px-5" aria-labelledby="merchant-payment-check-title">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 id="merchant-payment-check-title" class="font-semibold">收款核对</h2>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">先核对实际到账、买家备注和订单联系方式，再确认收款。</p>
        </div>
        <Badge :variant="canConfirmPayment ? 'trust' : 'secondary'">{{ canConfirmPayment ? '待核款' : getApiOrderDisplayStatus(order, perspective) }}</Badge>
      </div>

      <dl class="mt-4 grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
        <div class="border-l-2 border-primary pl-3"><dt class="text-xs text-muted-foreground">应收金额</dt><dd class="mt-1 text-xl font-semibold text-primary">¥{{ orderAmountText }}</dd></div>
        <div><dt class="text-xs text-muted-foreground">付款方式</dt><dd class="mt-1 flex items-center gap-2 font-medium"><ApiPaymentMethodIcon :method="order.selectedPaymentMethod" size="sm" />{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</dd></div>
        <div v-if="order.buyerNote"><dt class="text-xs text-muted-foreground">下单备注</dt><dd class="mt-1 whitespace-pre-line break-words">{{ order.buyerNote }}</dd></div>
        <div v-if="order.paymentSummary"><dt class="text-xs text-muted-foreground">付款备注</dt><dd class="mt-1 whitespace-pre-line break-words">{{ order.paymentSummary }}</dd></div>
      </dl>

      <OrderContactCard
        v-if="buyerContactSnapshot"
        class="mt-4"
        :snapshot="buyerContactSnapshot"
        side="buyer"
        title="买家联系方式"
        compact
        :show-contacted-action="false"
        :show-issue-actions="false"
      />

      <div v-if="canConfirmPayment" class="mt-4 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button v-if="canReportPaymentIssue" variant="outline" class="border-warning/50 text-warning" :disabled="actionBusy" @click="paymentIssueDialogOpen = true">
          <ShieldAlert class="h-4 w-4" />付款有问题
        </Button>
        <Button :disabled="actionBusy" @click="confirmPayment"><CheckCircle2 class="h-4 w-4" />确认已收款</Button>
      </div>
    </section>

    <Card class="overflow-hidden border-border/80">
      <div class="border-b border-border bg-muted/20 px-4 py-5">
        <Stepper v-if="order.status !== 'cancelled'" :model-value="Math.min(flowSteps.length, Math.max(1, currentFlowIndex + 1))" class="w-full items-start overflow-x-auto px-1">
          <StepperItem v-for="(step, index) in flowSteps" :key="step" :step="index + 1" class="relative flex min-w-[112px] flex-1 flex-col items-center">
            <StepperTrigger class="flex w-full flex-col items-center gap-2" disabled>
              <StepperIndicator class="c2c-motion-state relative z-10 h-8 w-8 border border-border bg-background">{{ currentFlowIndex > index ? '✓' : index + 1 }}</StepperIndicator>
              <div class="text-center">
                <StepperTitle class="text-sm">{{ step }}</StepperTitle>
                <StepperDescription>{{ flowStepDescriptions[index] }}</StepperDescription>
              </div>
            </StepperTrigger>
            <StepperSeparator v-if="index < flowSteps.length - 1" class="pointer-events-none absolute left-[calc(50%+1.5rem)] right-[calc(-50%+1.5rem)] top-4 h-0.5 rounded-full bg-border group-data-[state=completed]:bg-primary/60" />
          </StepperItem>
        </Stepper>
        <div v-else class="flex items-center justify-center gap-2 py-2 text-sm text-muted-foreground"><XCircle class="h-4 w-4" />交易流程已终止</div>
      </div>

      <div class="grid gap-0 md:grid-cols-[0.8fr_1fr_1.15fr_auto]">
        <div class="border-b border-border p-5 md:border-b-0 md:border-r">
          <div class="text-xs text-muted-foreground">{{ isMerchantView ? '应收金额' : '实付金额' }}</div>
          <div class="mt-2 text-3xl font-semibold text-primary">¥{{ orderAmountText }}</div>
          <div class="mt-1 text-xs text-muted-foreground">锁定额度 ${{ orderAllowanceText }} 美元额度</div>
        </div>
        <div class="border-b border-border p-5 md:border-b-0 md:border-r">
          <div class="text-xs text-muted-foreground">付款方式</div>
          <div class="mt-2 flex items-center gap-2 font-semibold"><ApiPaymentMethodIcon :method="order.selectedPaymentMethod" size="md" />{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</div>
          <div class="mt-2 text-xs text-muted-foreground">付款由你与商户直接完成，平台不代收或托管资金</div>
        </div>
        <div class="border-b border-border p-5 text-center md:border-b-0 md:border-r">
          <template v-if="activeDeadline">
            <div class="text-xs font-medium" :class="countdown.urgent || countdown.expired ? 'text-destructive' : 'text-muted-foreground'">{{ countdownTitle }}</div>
            <div class="mt-2 font-mono text-3xl font-semibold" :class="countdown.urgent || countdown.expired ? 'text-destructive' : 'text-foreground'">{{ countdownLabel }}</div>
            <div class="mt-2 text-xs text-muted-foreground">{{ countdown.expired ? '本阶段处理时间已结束' : `截止 ${formatOrderDateTime(activeDeadline)}` }}</div>
          </template>
          <template v-else>
            <div class="text-xs text-muted-foreground">当前状态</div>
            <div class="mt-3 text-xl font-semibold">{{ getApiOrderDisplayStatus(order, perspective) }}</div>
            <div class="mt-2 text-xs text-muted-foreground">{{ order.deliveryCredential ? (isMerchantView ? '你的交付任务已结束' : '交付凭证已提交') : '无需倒计时' }}</div>
          </template>
        </div>
        <div v-auto-animate="functionalMotion" class="flex min-w-56 flex-col justify-center gap-2 p-5">
          <div class="text-center text-xs font-medium text-muted-foreground">当前可执行操作</div>
          <Button v-if="canSubmitPayment" size="lg" :disabled="actionBusy || paymentInstructionsQuery.isLoading.value || countdown.expired" @click="paymentDialogOpen = true">
            <QrCode class="h-4 w-4" />{{ paymentActionLabel }}
          </Button>
          <Button v-else-if="canResubmitPayment" size="lg" :disabled="actionBusy" @click="openPaymentIssueResponse">
            <WalletCards class="h-4 w-4" />补充并重新提交
          </Button>
          <template v-else-if="canConfirmPayment">
            <Badge variant="trust">请在上方“收款核对”区处理</Badge>
          </template>
          <Button v-else-if="canSubmitDelivery" size="lg" :disabled="actionBusy" @click="scrollToDeliveryForm">
            <KeyRound class="h-4 w-4" />继续填写交付信息
          </Button>
          <template v-else-if="canConfirmComplete">
            <Button size="lg" :disabled="actionBusy" @click="completionConfirmOpen = true">
              <CheckCircle2 class="h-4 w-4" />确认凭证可用
            </Button>
            <Button v-if="canReportCredentialProblem" variant="outline" class="border-warning/50 text-warning" :disabled="actionBusy" @click="credentialProblemOpen = true">
              <ShieldAlert class="h-4 w-4" />凭证存在问题
            </Button>
          </template>
          <Button v-else-if="canOpenReviewCenter" size="lg" :disabled="actionBusy" @click="openReviewCenter">
            <Star class="h-4 w-4" />{{ isMerchantView ? '评价买家' : '评价卖家' }}
          </Button>
          <p class="text-center text-xs leading-5 text-muted-foreground">{{ currentActionDescription }}</p>
        </div>
      </div>

    </Card>

    <Alert v-if="showMerchantTimeout">
      <Clock3 />
      <AlertTitle>商户处理已超时，订单不会自动取消</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
        <span>你已提交付款状态，请勿重复付款。如需平台处理，可直接发起订单纠纷。</span>
        <Button v-if="canOpenDispute" size="sm" variant="outline" @click="disputeDialogOpen = true"><Headphones class="h-4 w-4" />发起纠纷</Button>
      </AlertDescription>
    </Alert>

		<Alert v-if="order.status === 'completed' && canOpenDispute" class="border-warning/30 bg-warning/5">
			<Clock3 class="text-warning" />
			<AlertTitle>仍在 24 小时补报期内</AlertTitle>
			<AlertDescription>
				仅可补报服务有效期内已经发生的问题；补报期不会延长 API 有效期，也不代表平台保证退款。
				<span v-if="order.afterSalesExpiresAt" class="mt-1 block">补报截止：{{ formatOrderDateTime(order.afterSalesExpiresAt) }}</span>
			</AlertDescription>
		</Alert>

    <div class="grid items-start gap-4 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.1fr)_minmax(280px,0.8fr)]">
      <Collapsible v-model:open="orderDetailsOpen" as-child>
        <Card class="h-fit p-5">
          <CollapsibleTrigger class="flex w-full items-center justify-between text-left">
            <div><h2 class="font-semibold">订单信息</h2><p class="mt-1 text-xs text-muted-foreground">查看下单时锁定的服务与订单信息</p></div>
            <ChevronDown class="h-4 w-4 transition-transform" :class="orderDetailsOpen ? 'rotate-180' : ''" />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <section class="mt-5" aria-labelledby="order-quota-policy-title">
              <h3 id="order-quota-policy-title" class="mb-2 text-sm font-medium">购买时额度规则</h3>
              <ApiQuotaPolicyStrip
                :policy="order.quotaUsagePolicySnapshot"
                :expiry-value="orderQuotaExpiryLabel"
              />
            </section>
            <div v-if="order.quotaSnapshot" class="mt-5 grid gap-4 text-sm sm:grid-cols-2">
              <div><span class="text-muted-foreground">额度包</span><div>{{ order.quotaSnapshot.offerName }}</div></div>
              <div><span class="text-muted-foreground">固定额度 / 总价</span><div>${{ formatDecimal(order.quotaSnapshot.usdAllowance, 0, 6) }} / ¥{{ formatDecimal(order.quotaSnapshot.priceCny, 2, 2) }}</div></div>
              <div><span class="text-muted-foreground">有效售价</span><div>¥{{ formatDecimal(order.quotaSnapshot.cnyPerUsd, 3, 6) }} / $1</div></div>
              <div><span class="text-muted-foreground">模型倍率</span><div>{{ Number(order.quotaSnapshot.modelMultiplier).toFixed(2) }}x</div></div>
              <div><span class="text-muted-foreground">销售方式</span><div>{{ getApiQuotaSaleModeLabel(order.quotaSnapshot.saleMode) }}</div></div>
              <div><span class="text-muted-foreground">接入系统</span><div>{{ getApiQuotaDistributionLabel(order.quotaSnapshot.distributionSystem) }}</div></div>
              <div><span class="text-muted-foreground">最晚下单</span><div>{{ formatOrderDateTime(order.quotaSnapshot.saleCutoffAt) }}</div></div>
              <div><span class="text-muted-foreground">额度失效</span><div>{{ formatOrderDateTime(order.quotaSnapshot.expiresAt) }}</div></div>
              <div v-if="order.quotaSnapshot.roundStartsAt"><span class="text-muted-foreground">购买轮次</span><div>{{ formatOrderDateTime(order.quotaSnapshot.roundStartsAt) }} - {{ formatOrderDateTime(order.quotaSnapshot.roundEndsAt) }}</div></div>
              <div><span class="text-muted-foreground">首字响应</span><div>{{ getApiTTFTBandLabel(order.quotaSnapshot.ttftBand) }} · 商户自报，平台未测速</div></div>
              <div><span class="text-muted-foreground">号池</span><div>{{ accountPoolSnapshotLabel(order.quotaSnapshot) }}</div></div>
              <div><span class="text-muted-foreground">商户声明最大并发</span><div>{{ concurrencySnapshotLabel(order.quotaSnapshot) }}</div></div>
              <div><span class="text-muted-foreground">商户退款承诺</span><div>{{ refundCommitmentSnapshotLabel(order.quotaSnapshot) }}</div></div>
							<div><span class="text-muted-foreground">退款规则版本</span><ApiRefundPolicyEvidence class="mt-1" :snapshot="order.quotaSnapshot" /></div>
              <div><span class="text-muted-foreground">承诺适用截止</span><div>{{ serviceValiditySnapshotLabel(order.quotaSnapshot) }}</div></div>
              <div><span class="text-muted-foreground">预计交付</span><div>{{ getApiQuotaDeliveryModeLabel(order.quotaSnapshot.deliveryMode) }} · ≤ {{ order.quotaSnapshot.deliveryEtaMinutes }} 分钟</div></div>
              <div v-if="order.quotaSnapshot.performanceConfirmedAt"><span class="text-muted-foreground">体验确认时间</span><div>{{ formatOrderDateTime(order.quotaSnapshot.performanceConfirmedAt) }}</div></div>
              <div><span class="text-muted-foreground">付款截止</span><div>{{ formatOrderDateTime(order.paymentExpiresAt) }}</div></div>
            </div>
            <div v-else class="mt-5 grid gap-4 text-sm sm:grid-cols-2">
              <div><span class="text-muted-foreground">服务</span><div>{{ order.serviceTitle }}</div></div>
              <div><span class="text-muted-foreground">商户</span><div>{{ order.seller }}</div></div>
              <div><span class="text-muted-foreground">模型</span><div>{{ orderModelSnapshotLabel }}</div></div>
              <div><span class="text-muted-foreground">倍率快照</span><div>{{ order.intentSnapshot.multiplier }}</div></div>
              <div><span class="text-muted-foreground">号池</span><div>{{ accountPoolSnapshotLabel(order.intentSnapshot) }}</div></div>
              <div><span class="text-muted-foreground">商户声明最大并发</span><div>{{ concurrencySnapshotLabel(order.intentSnapshot) }}</div></div>
              <div><span class="text-muted-foreground">商户退款承诺</span><div>{{ refundCommitmentSnapshotLabel(order.intentSnapshot) }}</div></div>
							<div><span class="text-muted-foreground">退款规则版本</span><ApiRefundPolicyEvidence class="mt-1" :snapshot="order.intentSnapshot" /></div>
              <div><span class="text-muted-foreground">服务有效期</span><div>{{ orderServiceValidityLabel }}</div></div>
              <div><span class="text-muted-foreground">用量核对</span><div>{{ orderUsageVisibilityLabel }}</div></div>
              <div><span class="text-muted-foreground">付款截止</span><div>{{ formatOrderDateTime(order.paymentExpiresAt) }}</div></div>
              <div v-if="order.intentSnapshot.commercialFactsSnapshotIssue"><span class="text-muted-foreground">历史售后说明</span><div>{{ legacyRevocationCopy(order.intentSnapshot.warranty) }}</div></div>
              <div class="sm:col-span-2"><span class="text-muted-foreground">平台交易边界</span><div>{{ legacyRevocationCopy(order.intentSnapshot.refundPolicy) }}</div></div>
            </div>
            <div v-if="order.paymentSummary" class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-sm">买家付款备注：{{ order.paymentSummary }}</div>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      <div v-auto-animate="functionalMotion" class="min-w-0 space-y-4">

        <Card v-if="order.deliveryCredential" class="p-5">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold">接入凭证</h2>
              <p class="mt-1 text-xs text-muted-foreground">{{ getApiOrderDeliveryKindLabel(order.deliveryCredential.deliveryKind) }} · {{ formatOrderDateTime(order.deliveryCredential.submittedAt) }}</p>
            </div>
            <Badge :variant="order.deliveryCredential.destroyedAt ? 'secondary' : 'verified'">{{ order.deliveryCredential.destroyedAt ? '已销毁' : '保留期内可查看' }}</Badge>
          </div>
          <div v-if="order.deliveryCredential.destroyedAt" class="mt-4 rounded-md border border-border bg-muted/40 p-4 text-sm leading-6 text-muted-foreground">
            历史凭证已按保留策略销毁，平台仅保留交付类型、提交时间和销毁时间等审计事实。
            <div class="mt-1 text-xs">销毁时间：{{ formatOrderDateTime(order.deliveryCredential.destroyedAt) }}</div>
          </div>
          <div v-else class="mt-4 space-y-3 text-sm">
            <div v-if="order.deliveryCredential.apiBaseUrl" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">API Base URL</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.apiBaseUrl, 'API Base URL')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.apiBaseUrl }}</div>
            </div>
            <div v-if="order.deliveryCredential.apiKey" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted-foreground">API Key</span>
                <span class="flex gap-1.5">
                  <Button size="icon" variant="outline" :title="apiKeyVisible ? '隐藏 API Key' : '显示 API Key'" :aria-label="apiKeyVisible ? '隐藏 API Key' : '显示 API Key'" @click="apiKeyVisible = !apiKeyVisible">
                    <EyeOff v-if="apiKeyVisible" class="h-4 w-4" /><Eye v-else class="h-4 w-4" /><span class="sr-only">{{ apiKeyVisible ? '隐藏 API Key' : '显示 API Key' }}</span>
                  </Button>
                  <Button size="icon" variant="outline" title="复制 API Key" aria-label="复制 API Key" @click="copyValue(order.deliveryCredential.apiKey, 'API Key')"><Copy class="h-4 w-4" /><span class="sr-only">复制 API Key</span></Button>
                </span>
              </div>
              <div class="mt-2 break-all font-mono text-xs">{{ apiKeyVisible ? order.deliveryCredential.apiKey : maskCredential(order.deliveryCredential.apiKey) }}</div>
            </div>
            <div v-if="order.deliveryCredential.panelLoginUrl" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">登录地址</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.panelLoginUrl, '登录地址')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.panelLoginUrl }}</div>
            </div>
            <div v-if="order.deliveryCredential.username" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">用户名</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.username, '用户名')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.username }}</div>
            </div>
            <div v-if="order.deliveryCredential.password" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted-foreground">初始密码</span>
                <span class="flex gap-1.5">
                  <Button size="icon" variant="outline" :title="passwordVisible ? '隐藏初始密码' : '显示初始密码'" :aria-label="passwordVisible ? '隐藏初始密码' : '显示初始密码'" @click="passwordVisible = !passwordVisible">
                    <EyeOff v-if="passwordVisible" class="h-4 w-4" /><Eye v-else class="h-4 w-4" /><span class="sr-only">{{ passwordVisible ? '隐藏初始密码' : '显示初始密码' }}</span>
                  </Button>
                  <Button size="icon" variant="outline" title="复制初始密码" aria-label="复制初始密码" @click="copyValue(order.deliveryCredential.password, '初始密码')"><Copy class="h-4 w-4" /><span class="sr-only">复制初始密码</span></Button>
                </span>
              </div>
              <div class="mt-2 break-all font-mono text-xs">{{ passwordVisible ? order.deliveryCredential.password : maskCredential(order.deliveryCredential.password) }}</div>
            </div>
            <div v-if="order.deliveryCredential.instructions" class="rounded-md border border-border bg-muted/40 p-3 whitespace-pre-line">{{ order.deliveryCredential.instructions }}</div>
          </div>
        </Card>

        <Card v-else-if="canSubmitDelivery" id="api-order-delivery-form" class="scroll-mt-4 border-primary/25 p-5">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="font-semibold">填写接入信息</h2>
              <p class="mt-1 text-xs text-muted-foreground">只提交买家专属的 API Key 或初始登录账号；提交后不可修改。</p>
            </div>
            <Badge variant="secondary">一次性交付</Badge>
          </div>
          <RadioGroup v-model="deliveryKind" class="mt-4 flex flex-wrap gap-2" aria-label="接入方式">
            <div class="flex items-center gap-2 rounded-md border border-border px-3 py-2 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
              <RadioGroupItem id="api-order-delivery-api-key" value="api_key_endpoint" />
              <Label for="api-order-delivery-api-key" class="flex cursor-pointer items-center gap-2 text-sm font-medium"><KeyRound class="h-4 w-4" />API Key 接入</Label>
            </div>
            <div class="flex items-center gap-2 rounded-md border border-border px-3 py-2 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
              <RadioGroupItem id="api-order-delivery-login-account" value="login_account" />
              <Label for="api-order-delivery-login-account" class="cursor-pointer text-sm font-medium">登录账号接入</Label>
            </div>
          </RadioGroup>
          <div v-if="deliveryKind === 'api_key_endpoint'" class="mt-4 grid gap-3">
            <label class="space-y-2"><span class="text-sm font-medium">API Base URL</span><Input v-model="apiBaseUrl" placeholder="https://api.example.com/v1" /></label>
            <label class="space-y-2"><span class="text-sm font-medium">买家专属 API Key</span><Input v-model="apiKey" placeholder="sk-proj-..." /></label>
          </div>
          <div v-else class="mt-4 grid gap-3">
            <label class="space-y-2"><span class="text-sm font-medium">登录地址</span><Input v-model="panelLoginUrl" placeholder="https://panel.example.com/login" /></label>
            <label class="space-y-2"><span class="text-sm font-medium">用户名</span><Input v-model="username" placeholder="buyer-demo" /></label>
            <label class="space-y-2"><span class="text-sm font-medium">初始密码</span><Input v-model="password" placeholder="首次登录后按面板提示处理" /></label>
          </div>
          <label class="mt-4 block space-y-2">
            <span class="text-sm font-medium">使用说明</span>
            <Textarea v-model="deliveryInstructions" class="min-h-24" maxlength="4000" placeholder="说明限速、模型范围、后续更换 Key 或重置密码的联系方式。不要提交 Cookie、Session、OAuth token、恢复码、订阅链接或主账号凭据。" />
          </label>
          <div class="mt-4 flex justify-end">
            <Button :disabled="actionBusy" @click="submitDelivery">{{ actionBusy ? '提交中…' : '确认已交付' }}</Button>
          </div>
        </Card>

        <Card v-else class="p-5">
          <h2 class="font-semibold">接入凭证</h2>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ order.status === 'cancelled' ? '订单已取消，未产生接入凭证。' : '商户确认收款并提交接入信息后，本区域将展示订单专属凭证。' }}</p>
        </Card>
      </div>

      <div class="min-w-0 space-y-4">
        <Card class="p-5">
          <h2 class="font-semibold">{{ isMerchantView ? '买家信息' : '商户信息' }}</h2>
          <div class="mt-4 flex min-w-0 items-center gap-3">
            <span class="grid h-11 w-11 shrink-0 place-items-center rounded-full bg-muted text-sm font-semibold text-foreground">{{ counterpartyAvatarText }}</span>
            <span class="min-w-0"><strong class="block truncate">{{ counterpartyName }}</strong><span class="mt-1 block text-xs text-muted-foreground">订单创建时锁定的参与方</span></span>
          </div>
          <dl class="mt-4 grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
            <div><dt class="text-xs text-muted-foreground">已完成订单</dt><dd class="mt-1 font-semibold">{{ counterpartyCompletedOrders }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">完成率</dt><dd class="mt-1 font-semibold">{{ counterpartyCompletionRate }}</dd></div>
          </dl>
        </Card>

        <OrderContactCard
          v-if="merchantContactSnapshot"
          :snapshot="merchantContactSnapshot"
          title="商户联系方式"
          context-label="订单创建成功后展示下单时锁定的商户联系方式"
          visible-label="已向本次订单买家展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自下单时锁定的信息；商户后续修改联系方式不会改变当前订单。"
          :show-contacted-action="false"
          :show-issue-actions="false"
        />
        <OrderContactCard
          v-if="buyerContactSnapshot"
          :snapshot="buyerContactSnapshot"
          side="buyer"
          title="买家联系方式"
          context-label="订单创建成功后展示下单时锁定的买家联系方式"
          visible-label="已向本次订单商户展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自下单时锁定的信息；买家后续修改联系方式不会改变当前订单。"
          :show-contacted-action="false"
          :show-issue-actions="false"
        />
      </div>
    </div>

    <Collapsible v-model:open="orderRecordsOpen" as-child>
      <Card class="p-5">
        <CollapsibleTrigger class="flex w-full items-center justify-between text-left">
          <div><h2 class="font-semibold">订单记录</h2><p class="mt-1 text-xs text-muted-foreground">{{ events.length }} 条状态记录，默认收起</p></div>
          <ChevronDown class="h-4 w-4 transition-transform" :class="orderRecordsOpen ? 'rotate-180' : ''" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div v-auto-animate="functionalMotion" class="mt-4 space-y-3">
            <div v-for="event in events" :key="event.id" class="grid gap-1 border-b border-border pb-3 text-sm md:grid-cols-[180px_1fr]">
              <div class="text-muted-foreground">{{ formatOrderDateTime(event.createdAt) }}</div>
              <div>
                <div class="font-medium">{{ event.actorLabel }} · {{ getApiOrderEventLabel(event.type) }}</div>
                <div class="text-xs text-muted-foreground">
                  {{ event.fromStatus ? getApiOrderStatusLabel(event.fromStatus, perspective) : '创建' }}
                  <span v-if="event.toStatus"> → {{ getApiOrderStatusLabel(event.toStatus, perspective) }}</span>
                  <span v-if="event.note"> · {{ event.note }}</span>
                </div>
              </div>
            </div>
          </div>
        </CollapsibleContent>
      </Card>
    </Collapsible>

    <div class="flex flex-wrap items-center justify-between gap-3 border-y border-border px-1 py-4">
      <div><div class="text-sm font-medium">订单履约存在争议？</div><div class="mt-1 text-xs leading-5 text-muted-foreground">发起后直接进入平台处理，被申请方可提交一次正式答复。</div></div>
      <Button v-if="canOpenDispute" variant="outline" @click="disputeDialogOpen = true"><Headphones class="h-4 w-4" />发起纠纷</Button>
      <Badge v-else-if="showDisputeStatus" variant="status">{{ getApiOrderDisputeStatusLabel(order.disputeStatus) }}</Badge>
    </div>

    <Dialog v-model:open="completionConfirmOpen">
      <DialogContent class="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>确认凭证可以使用？</DialogTitle>
          <DialogDescription>确认后订单将立即完成并开放评价。交付凭证仅在平台保留期内可查看，请妥善保存买家专属接入信息。</DialogDescription>
        </DialogHeader>
        <Alert class="border-success/25 bg-success/10">
          <CheckCircle2 class="text-success" />
          <AlertTitle>请先完成实际核验</AlertTitle>
          <AlertDescription>请确认接入地址、凭证、额度和权限均符合订单说明；平台不会代替你测试 API。</AlertDescription>
        </Alert>
        <DialogFooter>
          <Button variant="outline" @click="completionConfirmOpen = false">返回核验</Button>
          <Button :disabled="actionBusy" @click="confirmComplete">{{ actionBusy ? '提交中…' : '确认凭证可用' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="latePaymentDialogOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>报告逾期付款</DialogTitle>
          <DialogDescription>仅用于订单因付款超时取消，但你已经实际转账的情况。</DialogDescription>
        </DialogHeader>
        <Alert class="border-warning/35 bg-warning/10">
          <ShieldAlert class="text-warning" />
          <AlertTitle>不会恢复原订单</AlertTitle>
          <AlertDescription>报告后不会恢复库存或抢购资格。如卖家确认到账，原则上由双方线下协商退款。</AlertDescription>
        </Alert>
        <label class="block space-y-2">
          <span class="text-sm font-medium">付款核对信息（选填）</span>
          <Textarea v-model="latePaymentNote" class="min-h-24" maxlength="500" placeholder="可填付款时间、金额、备注或交易尾号，不要填写完整账号。" />
        </label>
        <DialogFooter><Button variant="outline" @click="latePaymentDialogOpen = false">取消</Button><Button :disabled="actionBusy" @click="reportLatePayment">确认报告</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="latePaymentResolutionOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader><DialogTitle>核对逾期付款</DialogTitle><DialogDescription>只记录实际到账结果，不得恢复原订单履约。</DialogDescription></DialogHeader>
        <RadioGroup v-model="latePaymentResolution" class="space-y-2">
          <label class="flex items-start gap-3 rounded-md border border-border p-3"><RadioGroupItem value="not_received" class="mt-0.5" /><span><strong class="text-sm">未查到到账</strong><span class="mt-1 block text-xs text-muted-foreground">收款记录中没有对应转账。</span></span></label>
          <label class="flex items-start gap-3 rounded-md border border-border p-3"><RadioGroupItem value="received_refund_pending" class="mt-0.5" /><span><strong class="text-sm">已到账，待退款</strong><span class="mt-1 block text-xs text-muted-foreground">原订单保持取消，与买家线下协商退款。</span></span></label>
        </RadioGroup>
        <label class="block space-y-2"><span class="text-sm font-medium">核对说明（选填）</span><Textarea v-model="latePaymentResolutionNote" class="min-h-24" maxlength="500" /></label>
        <DialogFooter><Button variant="outline" @click="latePaymentResolutionOpen = false">取消</Button><Button :disabled="actionBusy" @click="resolveLatePayment">提交结果</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="credentialProblemOpen">
      <DialogContent class="max-h-[92dvh] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>凭证存在问题</DialogTitle>
          <DialogDescription>选择最符合的原因。提交后订单进入问题处理，24 小时自动完成计时将暂停。</DialogDescription>
        </DialogHeader>
        <RadioGroup v-model="credentialProblemReason" class="space-y-2">
          <label
            v-for="option in credentialProblemOptions"
            :key="option.value"
            class="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-4 transition-colors hover:bg-muted/40"
            :class="credentialProblemReason === option.value ? 'border-warning/60 bg-warning/10' : ''"
          >
            <RadioGroupItem :value="option.value" class="mt-0.5" />
            <span>
              <span class="block text-sm font-medium">{{ option.label }}</span>
              <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            </span>
          </label>
        </RadioGroup>
        <label class="block space-y-2">
          <span class="text-sm font-medium">补充说明{{ credentialProblemReason === 'other' ? '' : '（选填）' }}</span>
          <Textarea v-model="credentialProblemNote" class="min-h-24" maxlength="400" placeholder="说明实际表现和核验时间，不要填写 API Key、密码或验证码。" />
          <span class="block text-right text-xs text-muted-foreground">{{ credentialProblemNote.length }} / 400</span>
        </label>
        <DialogFooter>
          <Button variant="outline" @click="credentialProblemOpen = false">暂不提交</Button>
          <Button :disabled="credentialProblemSubmitDisabled || actionBusy" @click="submitCredentialProblem">{{ actionBusy ? '提交中…' : '提交凭证问题' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="disputeDialogOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>发起订单纠纷</DialogTitle>
          <DialogDescription>提交后直接进入平台处理。被申请方可提交一次正式答复，双方后续按平台要求补充材料。</DialogDescription>
        </DialogHeader>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block space-y-2">
            <span class="text-sm font-medium">问题类型</span>
            <Select v-model="disputeIssueCode">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="(label, value) in apiOrderDisputeIssueLabels" :key="value" :value="value">{{ label }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">处理诉求</span>
            <Select v-model="disputeRequestedResolution">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="(label, value) in newDisputeResolutionLabels" :key="value" :value="value">{{ label }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
        </div>
        <label v-if="disputeRequestedResolution === 'partial_refund'" class="block space-y-2">
          <span class="text-sm font-medium">部分退款金额</span>
          <Input v-model="disputeRequestedAmount" inputmode="decimal" placeholder="不超过订单金额" />
        </label>
				<label v-if="order.status === 'completed'" class="block space-y-2">
					<span class="text-sm font-medium">问题实际发生时间</span>
					<Input v-model="disputeIssueOccurredAt" type="datetime-local" :max="disputeOccurrenceMax" />
					<span class="block text-xs leading-5 text-muted-foreground">必须发生在所购服务有效期内。24 小时补报期只延长提交入口，不延长服务有效期。</span>
				</label>
        <label class="block space-y-2">
          <span class="text-sm font-medium">问题说明</span>
          <Textarea v-model="disputeReason" class="min-h-32" maxlength="500" placeholder="请描述发生时间、当前状态和希望协助处理的事项。不要填写密码、API Key、验证码等敏感信息。" />
          <span class="block text-right text-xs text-muted-foreground">{{ disputeReason.length }} / 500</span>
        </label>
        <DisputeEvidencePicker v-model="disputeEvidence" :order-id="order.id" />
        <Alert>
          <ShieldAlert />
          <AlertTitle>请勿提交敏感信息</AlertTitle>
          <AlertDescription>不要填写完整 API Key、密码、验证码、Cookie 或支付账号。</AlertDescription>
        </Alert>
        <DialogFooter>
          <Button variant="outline" @click="disputeDialogOpen = false">取消</Button>
          <Button :disabled="!canSubmitDispute || actionBusy" @click="submitOrderDispute">{{ actionBusy ? '提交中…' : '发起纠纷' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="paymentDialogOpen">
      <DialogContent class="max-h-[92dvh] overflow-y-auto sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <ApiPaymentMethodIcon :method="order.selectedPaymentMethod" size="md" />
            {{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}{{ apiPaymentMethodRequiresQrCode(order.selectedPaymentMethod) ? '收款码' : '付款信息' }}
          </DialogTitle>
          <DialogDescription>请核对订单金额和收款方，再使用对应应用完成站外付款。</DialogDescription>
        </DialogHeader>

        <div v-if="paymentInstructionsQuery.isLoading.value" class="rounded-lg border border-border p-8 text-center text-sm text-muted-foreground">正在读取收款资料…</div>
        <div v-else-if="paymentInstructions" class="space-y-4">
          <div v-if="apiPaymentMethodRequiresQrCode(paymentInstructions.paymentMethod)" class="mx-auto grid h-[260px] w-[260px] place-items-center overflow-hidden rounded-xl border border-border bg-white p-2 shadow-sm">
            <img v-if="paymentInstructions.paymentQrCodeDataUrl" :src="paymentInstructions.paymentQrCodeDataUrl" :alt="`${apiPaymentMethodLabels[paymentInstructions.paymentMethod]}收款码`" class="h-full w-full object-contain" />
            <span v-else class="px-6 text-center text-sm text-muted-foreground">商户未上传收款码，请先联系商户。</span>
          </div>
          <p v-else class="whitespace-pre-line rounded-lg border border-border bg-muted/30 p-4 text-sm leading-6">{{ paymentInstructions.paymentInstructions }}</p>

          <div class="divide-y divide-border rounded-lg border border-border text-sm">
            <div class="flex items-center justify-between px-4 py-3"><span class="text-muted-foreground">订单金额</span><strong class="text-lg text-destructive">¥{{ orderAmountText }}</strong></div>
            <div class="flex items-center justify-between px-4 py-3"><span class="text-muted-foreground">订单商户</span><span>{{ order.seller }}</span></div>
            <div v-if="paymentInstructions.paymentInstructions" class="px-4 py-3"><div class="text-muted-foreground">商户说明</div><div class="mt-1 whitespace-pre-line leading-6">{{ paymentInstructions.paymentInstructions }}</div></div>
          </div>

          <OrderContactCard
            v-if="merchantContactSnapshot"
            :snapshot="merchantContactSnapshot"
            title="付款有疑问？直接联系商户"
            compact
            :show-contacted-action="false"
            :show-issue-actions="false"
          />

          <label class="block space-y-2">
            <span class="text-sm font-medium">付款备注（选填）</span>
            <Textarea v-model="paymentSummary" class="min-h-20" maxlength="500" placeholder="可填写付款时间、订单号后 6 位或交易尾号，便于商户核对。" />
          </label>

          <Alert>
            <ShieldAlert />
            <AlertTitle>付款前请再次核对</AlertTitle>
            <AlertDescription>实际付款金额应为 ¥{{ orderAmountText }}，并请以扫码应用显示的收款人为准。平台不代收或托管资金，请勿重复付款。</AlertDescription>
          </Alert>
        </div>
        <DialogFooter class="gap-2 sm:justify-between">
          <Button variant="outline" @click="paymentDialogOpen = false">关闭</Button>
          <Button :disabled="!canConfirmOffPlatformPayment || actionBusy || countdown.expired" @click="openPaymentConfirmation"><CheckCircle2 class="h-4 w-4" />我已完成付款</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="paymentConfirmOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>是否确认已经成功付款？</DialogTitle>
          <DialogDescription>确认后将进入商户 10 分钟处理倒计时，付款状态不能撤回，请勿重复付款。</DialogDescription>
        </DialogHeader>
        <Alert>
          <WalletCards />
          <AlertTitle>订单金额 ¥{{ orderAmountText }}</AlertTitle>
          <AlertDescription>只有实际付款成功后才能确认。</AlertDescription>
        </Alert>
        <DialogFooter>
          <Button variant="outline" @click="returnToPaymentDetails">返回核对</Button>
          <Button :disabled="actionBusy" @click="submitPayment">{{ actionBusy ? '提交中…' : '确认已付款' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="paymentIssueDialogOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>报告付款核对问题</DialogTitle>
          <DialogDescription>请先通过订单内微信或 linux.do 私信联系买家核对；仍需补充时再选择明确原因。订单将保留当前锁定额度。</DialogDescription>
        </DialogHeader>
        <RadioGroup v-model="paymentIssueReason" class="gap-3">
          <div
            v-for="option in paymentIssueOptions"
            :key="option.value"
            class="flex items-start gap-3"
          >
            <RadioGroupItem :id="`payment-issue-${option.value}`" :value="option.value" class="mt-0.5" />
            <Label :for="`payment-issue-${option.value}`" class="min-w-0 flex-1 cursor-pointer items-start gap-0 leading-normal">
              <span class="block">
                <span class="block text-sm font-medium">{{ option.label }}</span>
                <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
              </span>
            </Label>
          </div>
        </RadioGroup>
        <label class="block space-y-2">
          <span class="text-sm font-medium">补充说明（选填）</span>
          <Textarea v-model="paymentIssueNote" class="min-h-24" maxlength="500" placeholder="例如：实际到账 ¥9.80，或收款记录中未找到订单备注。请勿填写完整账号等敏感信息。" />
          <span class="block text-right text-xs text-muted-foreground">{{ paymentIssueNote.length }} / 500</span>
        </label>
        <Alert class="border-warning/35 bg-warning/10">
          <ShieldAlert class="text-warning" />
          <AlertTitle>提交后等待买家补充</AlertTitle>
          <AlertDescription>买家重新提交付款信息后，订单会回到“等待商户确认收款”。</AlertDescription>
        </Alert>
        <DialogFooter>
          <Button variant="outline" @click="paymentIssueDialogOpen = false">返回</Button>
          <Button :disabled="!paymentIssueReason || actionBusy" @click="reportPaymentIssue">{{ actionBusy ? '提交中…' : '通知买家补充' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="paymentIssueResponseOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>补充付款信息</DialogTitle>
          <DialogDescription>请先核对实际付款记录，不要重复付款。补充后将重新交由商户核对。</DialogDescription>
        </DialogHeader>
        <Alert class="border-warning/35 bg-warning/10">
          <ShieldAlert class="text-warning" />
          <AlertTitle>{{ getApiOrderPaymentIssueLabel(order.paymentIssueReason) }}</AlertTitle>
          <AlertDescription>{{ order.paymentIssueNote || '商户未填写额外说明。' }}</AlertDescription>
        </Alert>
        <label class="block space-y-2">
          <span class="text-sm font-medium">付款核对信息</span>
          <Textarea v-model="paymentSummary" class="min-h-28" maxlength="500" placeholder="请填写付款时间、实际金额、付款备注或交易尾号，便于商户定位收款记录。" />
          <span class="block text-right text-xs text-muted-foreground">{{ paymentSummary.length }} / 500</span>
        </label>
        <DialogFooter>
          <Button variant="outline" @click="paymentIssueResponseOpen = false">暂不提交</Button>
          <Button :disabled="!paymentSummary.trim() || actionBusy" @click="resubmitPayment">{{ actionBusy ? '提交中…' : '重新提交付款信息' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="cancelDrawerOpen">
      <DialogContent class="bottom-0 left-auto right-0 top-0 flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 grid-cols-1 gap-0 overflow-hidden rounded-none border-l border-r-0 p-0 shadow-xl duration-200 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 sm:max-w-xl">
        <div class="flex min-h-0 flex-1 flex-col">
          <DialogHeader class="border-b border-border px-5 py-5 pr-12 text-left sm:px-6">
            <DialogTitle>取消订单</DialogTitle>
            <DialogDescription>订单尚未付款时可以立即取消。商户会收到你选择的原因，但无需再次确认。</DialogDescription>
          </DialogHeader>

          <div class="flex-1 space-y-6 overflow-y-auto px-5 py-5 sm:px-6">
            <div>
              <div class="text-sm font-semibold">请选择取消原因</div>
              <RadioGroup v-model="cancelReason" class="mt-3 gap-3">
                <div v-for="option in API_ORDER_CANCEL_OPTIONS" :key="option.value" class="flex items-start gap-3">
                  <RadioGroupItem :id="`cancel-reason-${option.value}`" :value="option.value" class="mt-0.5" />
                  <Label :for="`cancel-reason-${option.value}`" class="min-w-0 flex-1 cursor-pointer items-start gap-0 leading-normal">
                    <span class="flex flex-wrap items-center gap-2"><span class="font-medium">{{ option.label }}</span><Badge :variant="option.responsibility === 'merchant' ? 'status' : 'secondary'">{{ option.responsibilityLabel }}</Badge></span>
                  </Label>
                </div>
              </RadioGroup>
            </div>

            <label v-if="selectedCancelOption" class="block space-y-2">
              <span class="text-sm font-semibold">补充说明{{ selectedCancelOption.requiresNote ? '' : '（选填）' }}</span>
              <Textarea v-model="cancelNote" class="min-h-28" maxlength="200" placeholder="请补充说明本次取消情况（最多 200 字）" />
              <span class="block text-right text-xs text-muted-foreground">{{ cancelNote.length }} / 200</span>
            </label>

            <Alert variant="destructive">
              <ShieldAlert />
              <AlertTitle>请确认尚未付款</AlertTitle>
              <AlertDescription>如果已经付款，请不要取消订单。你可以通过微信或 linux.do 私信联系商户，也可以直接发起纠纷申请平台处理。</AlertDescription>
            </Alert>

            <label class="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-4">
              <Checkbox v-model="cancelUnpaidConfirmed" class="mt-0.5" />
              <span><span class="block text-sm font-medium">我确认尚未向商户付款</span><span class="mt-1 block text-xs leading-5 text-muted-foreground">取消后订单立即关闭，无法继续提交付款状态。</span></span>
            </label>
          </div>

          <DialogFooter class="border-t border-border px-5 py-4 sm:px-6">
            <Button variant="outline" @click="cancelDrawerOpen = false">返回</Button>
            <Button variant="destructive" :disabled="cancelSubmitDisabled || actionBusy" @click="cancelOrder">{{ actionBusy ? '提交中…' : '确认取消订单' }}</Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
