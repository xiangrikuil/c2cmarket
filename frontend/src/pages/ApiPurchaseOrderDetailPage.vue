<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, Copy, Flag, KeyRound, WalletCards } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  apiOrderBuyerContactSnapshot,
  apiOrderMerchantContactSnapshot,
  createManualInterventionReport,
  formatUsdQuota,
  getApiMerchantVisibilityLabel,
  getApiOrderDeliveryKindLabel,
  getApiOrderEvents,
  getApiOrderNextAction,
  getApiOrderStatusLabel,
  getApiUsageVisibilityLabel,
  readApiOrderPaymentInstructions,
  type ApiOrderDeliveryKind,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { apiPaymentMethodLabels, apiPaymentMethodRequiresQrCode } from '@/lib/apiPaymentSettings'
import {
  useApiOrder,
  useConfirmApiOrderPaymentMutation,
  useSubmitApiOrderDeliveryCredentialMutation,
  useSubmitApiOrderPaymentMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const id = computed(() => String(route.params.id ?? ''))
const perspective = computed<'buyer' | 'merchant'>(() => route.name === 'merchant-api-order-detail' ? 'merchant' : 'buyer')
const isMerchantView = computed(() => perspective.value === 'merchant')
const { data: order, isLoading } = useApiOrder(id, perspective)
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

const submitPaymentMutation = useSubmitApiOrderPaymentMutation()
const confirmPaymentMutation = useConfirmApiOrderPaymentMutation()
const submitDeliveryMutation = useSubmitApiOrderDeliveryCredentialMutation()

const backPath = computed(() => isMerchantView.value ? '/merchant/api-orders' : '/my/api-orders')
const backLabel = computed(() => isMerchantView.value ? '返回商户订单' : '返回我的 API 订单')
const canSubmitPayment = computed(() => !isMerchantView.value && order.value?.status === 'pending_payment')
const canConfirmPayment = computed(() => isMerchantView.value && order.value?.status === 'payment_submitted')
const canSubmitDelivery = computed(() => isMerchantView.value && order.value?.status === 'paid_confirmed' && !order.value.deliveryCredential)
const merchantContactSnapshot = computed(() => !isMerchantView.value && order.value ? apiOrderMerchantContactSnapshot(order.value) : null)
const buyerContactSnapshot = computed(() => isMerchantView.value && order.value ? apiOrderBuyerContactSnapshot(order.value) : null)
const events = computed(() => order.value ? getApiOrderEvents(order.value) : [])
const paymentInstructions = computed(() => paymentInstructionsQuery.data.value ?? null)
const actionBusy = computed(() => submitPaymentMutation.isPending.value || confirmPaymentMutation.isPending.value || submitDeliveryMutation.isPending.value)

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
}

async function submitPayment() {
  if (!order.value) return
  try {
    await submitPaymentMutation.mutateAsync({ id: order.value.id, paymentSummary: paymentSummaryValue(), version: order.value.version })
    await refresh(order.value.id)
    toast.success('已标记付款，等待商户确认收款。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交付款状态失败。')
  }
}

async function confirmPayment() {
  if (!order.value) return
  try {
    await confirmPaymentMutation.mutateAsync({ id: order.value.id, version: order.value.version })
    await refresh(order.value.id)
    toast.success('已确认收款，请填写交付信息。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '确认收款失败。')
  }
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

function requestManualIntervention() {
  if (!order.value) return
  const description = window.prompt('请填写 4-1000 字脱敏说明。平台只记录处理状态和公开摘要，不追回付款、不托管、不担保、不验证 API 可用性。')
  if (!description?.trim()) return
  const target = order.value
  ;(async () => {
    try {
      await createManualInterventionReport({
        targetType: 'api_order',
        targetId: target.id,
        targetLabel: target.serviceTitle,
        reasonCode: 'api_quota_dispute',
        title: '举报 / 申请人工介入：API 订单争议',
        description: description.trim(),
      })
      trackAnalytics('report_submit', {
        source_route: analyticsSourceRoute(),
        entity_type: 'api_order',
        reason_code: 'api_quota_dispute',
      })
      toast.success('已提交人工介入申请。')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '提交人工介入失败。')
    }
  })()
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载 API 订单...</div>
  <div v-else-if="!order" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到 API 订单</h1>
    <p class="mt-2 text-sm text-muted-foreground">该订单不存在或暂不可见。</p>
    <Button class="mt-5" variant="outline" @click="router.push(backPath)">{{ backLabel }}</Button>
  </div>
  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <Badge>{{ getApiOrderStatusLabel(order.status) }}</Badge>
          <span class="text-xs text-muted-foreground">{{ order.id }}</span>
        </div>
        <h1 class="mt-2 text-2xl font-semibold tracking-tight">{{ order.serviceTitle }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">
          {{ order.seller }} · 信任等级{{ order.intentSnapshot.trustLevel }} · {{ getApiMerchantVisibilityLabel(order.intentSnapshot) }} · 快照记录，不随当前服务信息变化。
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button v-if="canConfirmPayment" :disabled="actionBusy" @click="confirmPayment">
          <CheckCircle2 class="h-4 w-4" />确认已收款
        </Button>
        <Button v-if="canSubmitPayment" :disabled="actionBusy || paymentInstructionsQuery.isLoading.value" @click="submitPayment">
          <WalletCards class="h-4 w-4" />我已付款
        </Button>
        <Button variant="outline" :disabled="actionBusy" @click="requestManualIntervention">
          <Flag class="h-4 w-4" />申请人工介入
        </Button>
        <Button variant="outline" @click="router.push(backPath)">{{ backLabel }}</Button>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-4">
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">订单金额</div>
        <div class="mt-1 text-xl font-semibold">¥{{ order.amount }}</div>
        <div class="text-xs text-muted-foreground">额度上限 {{ formatUsdQuota(order.requestedUsdAllowance) }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">付款方式</div>
        <div class="mt-1 text-xl font-semibold">{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</div>
        <div class="text-xs text-muted-foreground">固定 {{ order.paymentWindowMinutes }} 分钟确认</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">交付状态</div>
        <div class="mt-1 text-xl font-semibold">{{ order.deliveryCredential ? '已交付' : '未交付' }}</div>
        <div class="text-xs text-muted-foreground">一次性交付，提交后不可修改</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">下一步</div>
        <div class="mt-1 text-xl font-semibold">{{ getApiOrderNextAction(order, isMerchantView ? 'merchant' : 'buyer') }}</div>
        <div class="text-xs text-muted-foreground">{{ order.updatedAt }}</div>
      </Card>
    </div>

    <div class="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      <Card class="p-5">
        <h2 class="font-semibold">订单快照</h2>
        <div class="mt-4 grid gap-3 text-sm sm:grid-cols-2">
          <div><span class="text-muted-foreground">服务</span><div>{{ order.serviceTitle }}</div></div>
          <div><span class="text-muted-foreground">订单状态</span><div>{{ getApiOrderStatusLabel(order.status) }}</div></div>
          <div><span class="text-muted-foreground">模型</span><div>{{ order.intentSnapshot.models.join(' / ') }}</div></div>
          <div><span class="text-muted-foreground">倍率快照</span><div>{{ order.intentSnapshot.multiplier }}</div></div>
          <div><span class="text-muted-foreground">用量核对</span><div>{{ getApiUsageVisibilityLabel(order.intentSnapshot.usageVisibility) }}</div></div>
          <div><span class="text-muted-foreground">付款截止</span><div>{{ order.paymentExpiresAt }}</div></div>
          <div><span class="text-muted-foreground">商户承诺</span><div>{{ order.intentSnapshot.warranty }}</div></div>
          <div><span class="text-muted-foreground">售后说明</span><div>{{ order.intentSnapshot.refundPolicy }}</div></div>
        </div>
        <div v-if="order.paymentSummary" class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-sm">
          买家付款备注：{{ order.paymentSummary }}
        </div>
        <div class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-xs leading-5 text-muted-foreground">
          平台只记录订单状态和一次性交付凭证；支付、收款核对和后续更换 Key 或重置密码由双方通过已展示联系方式站外处理。
        </div>
      </Card>

      <div class="space-y-4">
        <Card v-if="!isMerchantView && order.status === 'pending_payment'" class="p-5">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="font-semibold">收款资料</h2>
              <p class="mt-1 text-xs text-muted-foreground">来自订单创建时冻结的商户收款快照，仅当前买家可见。</p>
            </div>
            <Badge variant="secondary">{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</Badge>
          </div>

          <div v-if="paymentInstructionsQuery.isLoading.value" class="mt-4 rounded-md border border-border p-4 text-sm text-muted-foreground">正在读取收款资料...</div>
          <div v-else-if="paymentInstructions" class="mt-4 space-y-3">
            <div v-if="apiPaymentMethodRequiresQrCode(paymentInstructions.paymentMethod)" class="flex flex-col gap-3 sm:flex-row sm:items-center">
              <div class="grid h-32 w-32 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-muted/40">
                <img v-if="paymentInstructions.paymentQrCodeDataUrl" :src="paymentInstructions.paymentQrCodeDataUrl" :alt="`${apiPaymentMethodLabels[paymentInstructions.paymentMethod]}收款码`" class="h-full w-full object-cover" />
                <span v-else class="px-2 text-center text-xs text-muted-foreground">未上传收款码</span>
              </div>
              <p class="text-sm leading-6 text-muted-foreground">{{ paymentInstructions.paymentInstructions || '扫码后请通过商户联系方式站外确认。' }}</p>
            </div>
            <p v-else class="whitespace-pre-line text-sm leading-6 text-muted-foreground">{{ paymentInstructions.paymentInstructions }}</p>
            <label class="block space-y-2">
              <span class="text-sm font-medium">付款备注</span>
              <Textarea v-model="paymentSummary" class="min-h-20" maxlength="500" placeholder="可填写付款时间、备注或尾号；不要填写银行卡号、账号密码或 API Key。" />
            </label>
          </div>
          <div v-else class="mt-4 rounded-md border border-border p-4 text-sm text-muted-foreground">当前状态暂不能读取收款资料。</div>
        </Card>

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
          context-label="订单创建成功后展示本次冻结的商户联系方式"
          visible-label="已向本次订单买家展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自订单创建时冻结的快照；商户后续修改联系方式不会改变当前订单。"
          :show-contacted-action="false"
        />
        <OrderContactCard
          v-if="buyerContactSnapshot"
          :snapshot="buyerContactSnapshot"
          side="buyer"
          title="联系买家"
          context-label="订单创建成功后展示本次冻结的买家联系方式"
          visible-label="已向本次订单商户展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自订单创建时冻结的快照；买家后续修改联系方式不会改变当前订单。"
          :show-contacted-action="false"
        />
      </div>
    </div>

    <Card v-if="canSubmitDelivery" class="p-5">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="font-semibold">填写交付信息</h2>
          <p class="mt-1 text-xs text-muted-foreground">只提交买家专属、可撤销的 API Key 或初始登录账号；提交后不可修改。</p>
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
        <Textarea v-model="deliveryInstructions" class="min-h-24" maxlength="4000" placeholder="说明限速、模型范围、后续更换 Key 或重置密码的站外联系方式。不要提交 Cookie、Session、OAuth token、恢复码、订阅链接或主账号凭据。" />
      </label>
      <div class="mt-4 flex justify-end">
        <Button :disabled="actionBusy" @click="submitDelivery">{{ actionBusy ? '提交中...' : '确认已交付' }}</Button>
      </div>
    </Card>

    <Card class="p-5">
      <h2 class="font-semibold">事件时间线</h2>
      <div class="mt-4 space-y-3">
        <div v-for="event in events" :key="event.id" class="grid gap-1 border-b border-border pb-3 text-sm md:grid-cols-[180px_1fr]">
          <div class="text-muted-foreground">{{ event.createdAt }}</div>
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
    </Card>
  </div>
</template>
