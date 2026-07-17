<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, ChevronDown, Clock3, Copy, Headphones, KeyRound, QrCode, ShieldAlert, WalletCards, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
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
  getApiOrderEvents,
  getApiOrderPaymentIssueLabel,
  getApiOrderStatusLabel,
  getApiUsageVisibilityLabel,
  readApiOrderPaymentInstructions,
  type ApiOrderDeliveryKind,
  type ApiOrderPaymentIssueReason,
} from '@/lib/api'
import {
  API_ORDER_CANCEL_OPTIONS,
  buildApiOrderCancelReason,
  formatApiOrderCancelReason,
  formatOrderDateTime,
  merchantHandlingDeadline,
  orderCountdown,
} from '@/lib/apiOrderUi'
import { apiPaymentMethodLabels, apiPaymentMethodRequiresQrCode } from '@/lib/apiPaymentSettings'
import { formatDecimal } from '@/lib/decimal'
import {
  useApiOrder,
  useCancelApiOrderMutation,
  useConfirmApiOrderCompleteMutation,
  useConfirmApiOrderPaymentMutation,
  useOpenApiOrderDisputeMutation,
  useReportApiOrderPaymentIssueMutation,
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
const paymentInstructionsQuery = useQuery({
  queryKey: computed(() => ['api-order-payment-instructions', id.value]),
  queryFn: () => readApiOrderPaymentInstructions(id.value),
  enabled: computed(() => Boolean(order.value && !isMerchantView.value && order.value.status === 'pending_payment')),
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
const disputeDialogOpen = ref(false)
const disputeReason = ref('')
const cancelDrawerOpen = ref(false)
const cancelReason = ref('')
const cancelNote = ref('')
const cancelUnpaidConfirmed = ref(false)
const orderDetailsOpen = ref(true)
const orderRecordsOpen = ref(false)
const now = ref(Date.now())
let countdownTimer: ReturnType<typeof setInterval> | undefined

const submitPaymentMutation = useSubmitApiOrderPaymentMutation()
const cancelOrderMutation = useCancelApiOrderMutation()
const confirmCompleteMutation = useConfirmApiOrderCompleteMutation()
const confirmPaymentMutation = useConfirmApiOrderPaymentMutation()
const reportPaymentIssueMutation = useReportApiOrderPaymentIssueMutation()
const openDisputeMutation = useOpenApiOrderDisputeMutation()
const submitDeliveryMutation = useSubmitApiOrderDeliveryCredentialMutation()

const backPath = computed(() => isMerchantView.value ? '/merchant/api-orders' : '/my/api-orders')
const backLabel = computed(() => isMerchantView.value ? '返回商户订单' : '返回我的 API 订单')
const canSubmitPayment = computed(() => !isMerchantView.value && order.value?.status === 'pending_payment')
const canResubmitPayment = computed(() => !isMerchantView.value && order.value?.status === 'payment_issue')
const canConfirmPayment = computed(() => isMerchantView.value && order.value?.status === 'payment_submitted')
const canReportPaymentIssue = computed(() => isMerchantView.value && order.value?.status === 'payment_submitted')
const canSubmitDelivery = computed(() => isMerchantView.value && order.value?.status === 'paid_confirmed' && !order.value.deliveryCredential)
const canConfirmComplete = computed(() => !isMerchantView.value && order.value?.status === 'delivery_submitted')
const canOpenDispute = computed(() => Boolean(
  order.value
  && order.value.status !== 'cancelled'
  && order.value.status !== 'completed'
  && order.value.disputeStatus !== 'open',
))
const merchantContactSnapshot = computed(() => !isMerchantView.value && order.value ? apiOrderMerchantContactSnapshot(order.value) : null)
const buyerContactSnapshot = computed(() => isMerchantView.value && order.value ? apiOrderBuyerContactSnapshot(order.value) : null)
const events = computed(() => order.value ? getApiOrderEvents(order.value) : [])
const paymentInstructions = computed(() => paymentInstructionsQuery.data.value ?? null)
const paymentActionLabel = computed(() => {
  const method = order.value?.selectedPaymentMethod
  return method && apiPaymentMethodRequiresQrCode(method) ? '查看收款码' : '查看付款信息'
})
const canConfirmOffPlatformPayment = computed(() => {
  if (!paymentInstructions.value) return false
  return !apiPaymentMethodRequiresQrCode(paymentInstructions.value.paymentMethod) || Boolean(paymentInstructions.value.paymentQrCodeDataUrl)
})
const actionBusy = computed(() => cancelOrderMutation.isPending.value || submitPaymentMutation.isPending.value || confirmCompleteMutation.isPending.value || confirmPaymentMutation.isPending.value || reportPaymentIssueMutation.isPending.value || openDisputeMutation.isPending.value || submitDeliveryMutation.isPending.value)
const paymentIssueOptions: Array<{ value: ApiOrderPaymentIssueReason, label: string, description: string }> = [
  { value: 'not_received', label: '未到账', description: '收款记录中暂未找到对应付款。' },
  { value: 'amount_mismatch', label: '金额不符', description: '实收金额与订单金额不一致。' },
  { value: 'remark_mismatch', label: '备注不符', description: '付款备注或订单识别信息不一致。' },
]
const flowSteps = ['创建订单', '买家付款', '商户确认收款', '商户交付', '买家验收']
const flowStepDescriptions = ['锁定下单信息', '使用商户收款方式付款', '核对实际到账', '提交一次性交付凭证', '核对后完成订单']
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
const merchantDeadline = computed(() => merchantHandlingDeadline(order.value?.paymentSubmittedAt, 10))
const activeDeadline = computed(() => {
  if (order.value?.status === 'pending_payment') return order.value.paymentExpiresAt
  if (order.value?.status === 'payment_submitted' || order.value?.status === 'paid_confirmed') return merchantDeadline.value
  return null
})
const countdown = computed(() => orderCountdown(activeDeadline.value, now.value))
const countdownTitle = computed(() => order.value?.status === 'pending_payment' ? `请在 ${order.value.paymentWindowMinutes} 分钟内完成付款` : '商户确认并交付剩余时间')
const selectedCancelOption = computed(() => API_ORDER_CANCEL_OPTIONS.find(item => item.value === cancelReason.value))
const canCancelOrder = computed(() => !isMerchantView.value && order.value?.status === 'pending_payment')
const cancelSubmitDisabled = computed(() => {
  if (!selectedCancelOption.value || !cancelUnpaidConfirmed.value) return true
  return Boolean(selectedCancelOption.value.requiresNote && !cancelNote.value.trim())
})
const showMerchantTimeout = computed(() => Boolean(
  countdown.value.expired
  && (order.value?.status === 'payment_submitted' || order.value?.status === 'paid_confirmed'),
))
const currentActionDescription = computed(() => {
  if (!order.value) return ''
  if (order.value.status === 'cancelled') return '订单已取消，无需继续操作。'
  if (isMerchantView.value) {
    if (order.value.status === 'pending_payment') return '买家尚未标记付款，当前无需操作。'
    if (order.value.status === 'payment_submitted') return '买家已标记付款，请核对收款账户实际到账后确认。'
    if (order.value.status === 'payment_issue') return '已报告付款问题，正在等待买家补充付款信息。'
    if (order.value.status === 'paid_confirmed') return '收款已确认，请填写买家专属的接入信息。'
    if (order.value.status === 'delivery_submitted') return '交付凭证已提交，等待买家核对并确认完成交易。'
    return '双方操作已完成，这笔交易已结束。'
  }
  if (order.value.status === 'pending_payment') return '查看本次订单的收款信息，完成付款后确认付款状态。'
  if (order.value.status === 'payment_submitted') return '付款状态已提交，等待商户核对收款。'
  if (order.value.status === 'payment_issue') return '商户发现付款信息不匹配，请补充说明后重新提交。'
  if (order.value.status === 'paid_confirmed') return '商户已确认收款，等待商户提交交付凭证。'
  if (order.value.status === 'delivery_submitted') return '请核对交付凭证；确认可以使用后完成交易。'
  return '双方操作已完成，交付凭证仍可在本页查看。'
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
  if (!order.value || !disputeReason.value.trim()) return
  try {
    await openDisputeMutation.mutateAsync({
      id: order.value.id,
      reason: disputeReason.value.trim(),
      version: order.value.version,
      perspective: perspective.value,
    })
    disputeDialogOpen.value = false
    disputeReason.value = ''
    await refresh(order.value.id)
    toast.success('订单问题已提交，平台将介入处理。')
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
  if (!order.value || !window.confirm('确认交付凭证可以使用并完成这笔交易？完成后订单将进入已完成状态。')) return
  try {
    await confirmCompleteMutation.mutateAsync({ id: order.value.id, version: order.value.version })
    await refresh(order.value.id)
    toast.success('交易已完成。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '确认完成交易失败。')
  }
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
  <SkeletonBlock v-if="isLoading" :lines="9" />
  <ErrorState v-else-if="orderError" description="API 订单暂时无法加载，或当前账号无权查看。" @retry="refetchOrder()" />
  <EmptyState v-else-if="!order" title="未找到 API 订单" description="该订单不存在或暂不可见。"><template #action><Button variant="outline" @click="router.push(backPath)">{{ backLabel }}</Button></template></EmptyState>
  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <Button class="-ml-3 mb-2" variant="ghost" size="sm" @click="router.push(backPath)">← {{ backLabel }}</Button>
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold tracking-tight">{{ order.serviceTitle }}</h1>
          <StatusBadge :status="order.status" :label="getApiOrderStatusLabel(order.status)" />
        </div>
        <p class="mt-2 text-sm text-muted-foreground">订单号：<ShortId :value="order.id" prefix="API" copyable /></p>
      </div>
      <Button v-if="canCancelOrder" variant="outline" class="border-destructive/40 text-destructive hover:bg-destructive/5 hover:text-destructive" @click="cancelDrawerOpen = true">
        <XCircle class="h-4 w-4" />取消订单
      </Button>
    </div>

    <Alert v-if="order.status === 'cancelled'" variant="destructive">
      <XCircle />
      <AlertTitle>订单已取消</AlertTitle>
      <AlertDescription>{{ formatApiOrderCancelReason(order.cancelReason) }}</AlertDescription>
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

    <Alert v-if="order.disputeStatus === 'open'" class="border-warning/35 bg-warning/10">
      <ShieldAlert class="text-warning" />
      <AlertTitle>平台介入中</AlertTitle>
      <AlertDescription>该订单问题已经提交，请等待平台处理；无需重复提交。</AlertDescription>
    </Alert>

    <Card class="overflow-hidden border-border/80">
      <div class="grid gap-0 md:grid-cols-[0.8fr_1fr_1.15fr_auto]">
        <div class="border-b border-border p-5 md:border-b-0 md:border-r">
          <div class="text-xs text-muted-foreground">订单金额</div>
          <div class="mt-2 text-3xl font-semibold text-primary">¥{{ orderAmountText }}</div>
          <div class="mt-1 text-xs text-muted-foreground">锁定额度 ${{ orderAllowanceText }} 美元额度</div>
        </div>
        <div class="border-b border-border p-5 md:border-b-0 md:border-r">
          <div class="text-xs text-muted-foreground">付款方式</div>
          <div class="mt-2 flex items-center gap-2 font-semibold"><WalletCards class="h-5 w-5 text-primary" />{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</div>
          <div class="mt-2 text-xs text-muted-foreground">付款由你与商户直接完成，平台不代收或托管资金</div>
        </div>
        <div class="border-b border-border p-5 text-center md:border-b-0 md:border-r">
          <template v-if="activeDeadline">
            <div class="text-xs font-medium" :class="countdown.urgent || countdown.expired ? 'text-destructive' : 'text-muted-foreground'">{{ countdownTitle }}</div>
            <div class="mt-2 font-mono text-4xl font-semibold tracking-[0.16em]" :class="countdown.urgent || countdown.expired ? 'text-destructive' : 'text-foreground'">{{ countdown.label }}</div>
            <div class="mt-2 text-xs text-muted-foreground">{{ countdown.expired ? '本阶段处理时间已结束' : `截止 ${formatOrderDateTime(activeDeadline)}` }}</div>
          </template>
          <template v-else>
            <div class="text-xs text-muted-foreground">当前状态</div>
            <div class="mt-3 text-xl font-semibold">{{ getApiOrderStatusLabel(order.status) }}</div>
            <div class="mt-2 text-xs text-muted-foreground">{{ order.deliveryCredential ? '交付凭证已提交' : '无需倒计时' }}</div>
          </template>
        </div>
        <div class="flex min-w-56 flex-col justify-center gap-2 p-5">
          <div class="text-center text-xs font-medium text-muted-foreground">当前可执行操作</div>
          <Button v-if="canSubmitPayment" size="lg" :disabled="actionBusy || paymentInstructionsQuery.isLoading.value || countdown.expired" @click="paymentDialogOpen = true">
            <QrCode class="h-4 w-4" />{{ paymentActionLabel }}
          </Button>
          <Button v-else-if="canResubmitPayment" size="lg" :disabled="actionBusy" @click="openPaymentIssueResponse">
            <WalletCards class="h-4 w-4" />补充并重新提交
          </Button>
          <template v-else-if="canConfirmPayment">
            <Button size="lg" :disabled="actionBusy" @click="confirmPayment">
              <CheckCircle2 class="h-4 w-4" />确认已收款
            </Button>
            <Button v-if="canReportPaymentIssue" variant="outline" class="border-warning/50 text-warning" :disabled="actionBusy" @click="paymentIssueDialogOpen = true">
              <ShieldAlert class="h-4 w-4" />报告付款问题
            </Button>
          </template>
          <Button v-else-if="canSubmitDelivery" size="lg" :disabled="actionBusy" @click="scrollToDeliveryForm">
            <KeyRound class="h-4 w-4" />继续填写交付信息
          </Button>
          <Button v-else-if="canConfirmComplete" size="lg" :disabled="actionBusy" @click="confirmComplete">
            <CheckCircle2 class="h-4 w-4" />确认完成交易
          </Button>
          <p class="text-center text-xs leading-5 text-muted-foreground">{{ currentActionDescription }}</p>
        </div>
      </div>

      <div class="border-t border-border bg-muted/20 px-5 py-4">
        <Stepper v-if="order.status !== 'cancelled'" :model-value="Math.min(flowSteps.length, Math.max(1, currentFlowIndex + 1))" class="w-full items-start">
          <StepperItem v-for="(step, index) in flowSteps" :key="step" :step="index + 1" class="relative flex flex-1 flex-col items-center">
            <StepperTrigger class="flex flex-col items-center gap-2" disabled>
              <StepperIndicator class="h-8 w-8">{{ currentFlowIndex > index ? '✓' : index + 1 }}</StepperIndicator>
              <div class="text-center">
                <StepperTitle class="text-sm">{{ step }}</StepperTitle>
                <StepperDescription>{{ flowStepDescriptions[index] }}</StepperDescription>
              </div>
            </StepperTrigger>
            <StepperSeparator v-if="index < flowSteps.length - 1" class="absolute left-[calc(50%+1.5rem)] right-[calc(-50%+1.5rem)] top-4" />
          </StepperItem>
        </Stepper>
        <div v-else class="flex items-center justify-center gap-2 py-2 text-sm text-muted-foreground"><XCircle class="h-4 w-4" />交易流程已终止</div>
      </div>
    </Card>

    <Alert v-if="showMerchantTimeout">
      <Clock3 />
      <AlertTitle>商户处理已超时，订单不会自动取消</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
        <span>你已提交付款状态，请勿重复付款。可以先联系商户，仍未解决时申请平台介入。</span>
        <Button v-if="canOpenDispute" size="sm" variant="outline" @click="disputeDialogOpen = true"><Headphones class="h-4 w-4" />申请平台介入</Button>
      </AlertDescription>
    </Alert>

    <div class="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      <Collapsible v-model:open="orderDetailsOpen" as-child>
        <Card class="h-fit p-5">
          <CollapsibleTrigger class="flex w-full items-center justify-between text-left">
            <div><h2 class="font-semibold">订单信息</h2><p class="mt-1 text-xs text-muted-foreground">查看下单时锁定的服务与订单信息</p></div>
            <ChevronDown class="h-4 w-4 transition-transform" :class="orderDetailsOpen ? 'rotate-180' : ''" />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div class="mt-5 grid gap-4 text-sm sm:grid-cols-2">
              <div><span class="text-muted-foreground">服务</span><div>{{ order.serviceTitle }}</div></div>
              <div><span class="text-muted-foreground">商户</span><div>{{ order.seller }} · 信任等级{{ order.intentSnapshot.trustLevel }}</div></div>
              <div><span class="text-muted-foreground">模型</span><div>{{ order.intentSnapshot.models.join(' / ') }}</div></div>
              <div><span class="text-muted-foreground">倍率快照</span><div>{{ order.intentSnapshot.multiplier }}</div></div>
              <div><span class="text-muted-foreground">用量核对</span><div>{{ getApiUsageVisibilityLabel(order.intentSnapshot.usageVisibility) }}</div></div>
              <div><span class="text-muted-foreground">付款截止</span><div>{{ formatOrderDateTime(order.paymentExpiresAt) }}</div></div>
              <div><span class="text-muted-foreground">商户承诺</span><div>{{ legacyRevocationCopy(order.intentSnapshot.warranty) }}</div></div>
              <div><span class="text-muted-foreground">售后说明</span><div>{{ legacyRevocationCopy(order.intentSnapshot.refundPolicy) }}</div></div>
            </div>
            <div v-if="order.paymentSummary" class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-sm">买家付款备注：{{ order.paymentSummary }}</div>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      <div class="space-y-4">

        <Card v-if="order.deliveryCredential" class="p-5">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold">交付凭证</h2>
              <p class="mt-1 text-xs text-muted-foreground">{{ getApiOrderDeliveryKindLabel(order.deliveryCredential.deliveryKind) }} · {{ order.deliveryCredential.submittedAt }}</p>
            </div>
            <Badge variant="verified">长期可查看</Badge>
          </div>
          <div class="mt-4 space-y-3 text-sm">
            <div v-if="order.deliveryCredential.apiBaseUrl" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">API Base URL</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.apiBaseUrl, 'API Base URL')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.apiBaseUrl }}</div>
            </div>
            <div v-if="order.deliveryCredential.apiKey" class="rounded-md border border-border p-3">
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">API Key</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.apiKey, 'API Key')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.apiKey }}</div>
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
              <div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">初始密码</span><Button size="sm" variant="outline" @click="copyValue(order.deliveryCredential.password, '初始密码')"><Copy class="h-4 w-4" /></Button></div>
              <div class="mt-2 break-all font-mono text-xs">{{ order.deliveryCredential.password }}</div>
            </div>
            <div v-if="order.deliveryCredential.instructions" class="rounded-md border border-border bg-muted/40 p-3 whitespace-pre-line">{{ order.deliveryCredential.instructions }}</div>
          </div>
        </Card>

        <OrderContactCard
          v-if="merchantContactSnapshot"
          :snapshot="merchantContactSnapshot"
          title="联系商户"
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
          title="联系买家"
          context-label="订单创建成功后展示下单时锁定的买家联系方式"
          visible-label="已向本次订单商户展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自下单时锁定的信息；买家后续修改联系方式不会改变当前订单。"
          :show-contacted-action="false"
          :show-issue-actions="false"
        />
      </div>
    </div>

    <Card v-if="canSubmitDelivery" id="api-order-delivery-form" class="scroll-mt-4 border-primary/25 p-5">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="font-semibold">填写交付信息</h2>
          <p class="mt-1 text-xs text-muted-foreground">只提交买家专属的 API Key 或初始登录账号；提交后不可修改。</p>
        </div>
        <Badge variant="secondary">一次性交付</Badge>
      </div>
      <div class="mt-4 flex flex-wrap gap-2">
        <Button :variant="deliveryKind === 'api_key_endpoint' ? 'default' : 'outline'" @click="deliveryKind = 'api_key_endpoint'"><KeyRound class="h-4 w-4" />API Key 接入</Button>
        <Button :variant="deliveryKind === 'login_account' ? 'default' : 'outline'" @click="deliveryKind = 'login_account'">登录账号接入</Button>
      </div>
      <div v-if="deliveryKind === 'api_key_endpoint'" class="mt-4 grid gap-3 md:grid-cols-2">
        <label class="space-y-2"><span class="text-sm font-medium">API Base URL</span><Input v-model="apiBaseUrl" placeholder="https://api.example.com/v1" /></label>
        <label class="space-y-2"><span class="text-sm font-medium">买家专属 API Key</span><Input v-model="apiKey" placeholder="sk-proj-..." /></label>
      </div>
      <div v-else class="mt-4 grid gap-3 md:grid-cols-3">
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

    <Collapsible v-model:open="orderRecordsOpen" as-child>
      <Card class="p-5">
        <CollapsibleTrigger class="flex w-full items-center justify-between text-left">
          <div><h2 class="font-semibold">订单记录</h2><p class="mt-1 text-xs text-muted-foreground">{{ events.length }} 条状态记录，默认收起</p></div>
          <ChevronDown class="h-4 w-4 transition-transform" :class="orderRecordsOpen ? 'rotate-180' : ''" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div class="mt-4 space-y-3">
            <div v-for="event in events" :key="event.id" class="grid gap-1 border-b border-border pb-3 text-sm md:grid-cols-[180px_1fr]">
              <div class="text-muted-foreground">{{ formatOrderDateTime(event.createdAt) }}</div>
              <div>
                <div class="font-medium">{{ event.actorLabel }} · {{ event.type }}</div>
                <div class="text-xs text-muted-foreground">
                  {{ event.fromStatus ? getApiOrderStatusLabel(event.fromStatus) : '创建' }}
                  <span v-if="event.toStatus"> → {{ getApiOrderStatusLabel(event.toStatus) }}</span>
                  <span v-if="event.note"> · {{ event.note }}</span>
                </div>
              </div>
            </div>
          </div>
        </CollapsibleContent>
      </Card>
    </Collapsible>

    <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-muted/20 px-5 py-4">
      <div><div class="text-sm font-medium">遇到订单问题？</div><div class="mt-1 text-xs text-muted-foreground">付款、收款或交付出现异常时，可从当前订单申请平台介入。</div></div>
      <Button v-if="canOpenDispute" variant="outline" @click="disputeDialogOpen = true"><Headphones class="h-4 w-4" />申请平台介入</Button>
      <Badge v-else-if="order.disputeStatus === 'open'" variant="status">平台介入中</Badge>
    </div>

    <Dialog v-model:open="disputeDialogOpen">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>提交订单问题</DialogTitle>
          <DialogDescription>请说明付款、收款或交付中遇到的问题。平台会将说明关联到当前订单。</DialogDescription>
        </DialogHeader>
        <label class="block space-y-2">
          <span class="text-sm font-medium">问题说明</span>
          <Textarea v-model="disputeReason" class="min-h-32" maxlength="500" placeholder="请描述发生时间、当前状态和希望协助处理的事项。不要填写密码、API Key、验证码等敏感信息。" />
          <span class="block text-right text-xs text-muted-foreground">{{ disputeReason.length }} / 500</span>
        </label>
        <Alert>
          <ShieldAlert />
          <AlertTitle>提交后进入平台处理</AlertTitle>
          <AlertDescription>同一订单无需重复提交；处理进展请留意通知。</AlertDescription>
        </Alert>
        <DialogFooter>
          <Button variant="outline" @click="disputeDialogOpen = false">取消</Button>
          <Button :disabled="!disputeReason.trim() || actionBusy" @click="submitOrderDispute">{{ actionBusy ? '提交中…' : '提交订单问题' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="paymentDialogOpen">
      <DialogContent class="max-h-[92dvh] overflow-y-auto sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}{{ apiPaymentMethodRequiresQrCode(order.selectedPaymentMethod) ? '收款码' : '付款信息' }}</DialogTitle>
          <DialogDescription>请核对订单金额和收款方，再使用对应应用扫码完成付款。</DialogDescription>
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

          <label class="block space-y-2">
            <span class="text-sm font-medium">付款备注（选填）</span>
            <Textarea v-model="paymentSummary" class="min-h-20" maxlength="500" placeholder="可填写付款时间、备注或尾号，便于商户核对。" />
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
          <DialogDescription>请选择明确原因并通知买家补充。订单将保留当前锁定额度，不会自动取消。</DialogDescription>
        </DialogHeader>
        <RadioGroup v-model="paymentIssueReason" class="space-y-2">
          <label
            v-for="option in paymentIssueOptions"
            :key="option.value"
            class="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-4 transition-colors hover:bg-muted/40"
            :class="paymentIssueReason === option.value ? 'border-warning/60 bg-warning/10' : ''"
          >
            <RadioGroupItem :value="option.value" class="mt-0.5" />
            <span>
              <span class="block text-sm font-medium">{{ option.label }}</span>
              <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            </span>
          </label>
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
              <RadioGroup v-model="cancelReason" class="mt-3">
                <label v-for="option in API_ORDER_CANCEL_OPTIONS" :key="option.value" class="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-4 transition-colors hover:bg-muted/40" :class="cancelReason === option.value ? 'border-primary bg-primary/5' : ''">
                  <RadioGroupItem :value="option.value" class="mt-0.5" />
                  <span class="min-w-0 flex-1"><span class="flex flex-wrap items-center gap-2"><span class="font-medium">{{ option.label }}</span><Badge :variant="option.responsibility === 'merchant' ? 'status' : 'secondary'">{{ option.responsibilityLabel }}</Badge></span></span>
                </label>
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
              <AlertDescription>如果已经付款，请不要取消订单，应等待商户处理或申请平台介入。</AlertDescription>
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
