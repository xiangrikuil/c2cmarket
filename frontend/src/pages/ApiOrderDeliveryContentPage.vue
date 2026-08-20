<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ArrowLeft, CheckCircle2, FileKey2, Headphones, ShieldAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiOrderDeliveryCredentialCard from '@/components/api-order/ApiOrderDeliveryCredentialCard.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import {
  getApiOrderDisputeStatusDescription,
  getApiOrderDisputeStatusLabel,
  getApiOrderDisplayStatus,
  isApiOrderDisputeActive,
} from '@/lib/api'
import {
  API_ORDER_CREDENTIAL_PROBLEM_OPTIONS,
  type ApiOrderCredentialProblemReason,
} from '@/lib/apiOrderUi'
import {
  useApiOrder,
  useOpenApiOrderDisputeMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: order, isLoading, error, refetch } = useApiOrder(id, 'buyer')
const openDisputeMutation = useOpenApiOrderDisputeMutation()

const credentialProblemOpen = ref(false)
const credentialProblemReason = ref<ApiOrderCredentialProblemReason | ''>('')
const credentialProblemOccurredAt = ref('')
const credentialProblemNote = ref('')

const disputeActive = computed(() => Boolean(order.value && isApiOrderDisputeActive(order.value.disputeStatus)))
const catalogPaused = computed(() => order.value?.catalogRiskHold?.status === 'active')
const canReportCredentialProblem = computed(() => Boolean(
  order.value?.deliveryCredential
  && !order.value.deliveryCredential.destroyedAt
  && order.value.status === 'completed'
  && order.value.canOpenDispute
  && !disputeActive.value
  && !catalogPaused.value,
))
const disputePanelId = computed(() => order.value?.disputeCaseId ?? order.value?.latestDisputeCaseId ?? '')
const actionBusy = computed(() => openDisputeMutation.isPending.value)
const credentialProblemOccurrenceMax = computed(() => {
  const validityExpiresAt = order.value?.packageExpiresAt
    ?? order.value?.quotaSnapshot?.expiresAt
    ?? order.value?.intentSnapshot.serviceValidityExpiresAt
  const validityTimestamp = validityExpiresAt ? Date.parse(validityExpiresAt) : Number.NaN
  const value = new Date(Number.isFinite(validityTimestamp) ? Math.min(Date.now(), validityTimestamp) : Date.now())
  value.setMinutes(value.getMinutes() - value.getTimezoneOffset())
  return value.toISOString().slice(0, 16)
})
const credentialProblemSubmitDisabled = computed(() => !credentialProblemReason.value
  || !credentialProblemOccurredAt.value
  || (credentialProblemReason.value === 'other' && !credentialProblemNote.value.trim()))

async function submitCredentialProblem() {
  if (!order.value || !credentialProblemReason.value) return
  const option = API_ORDER_CREDENTIAL_PROBLEM_OPTIONS.find(item => item.value === credentialProblemReason.value)
  if (!option) return
  const note = credentialProblemNote.value.trim()
  const reason = `凭证异常｜${option.label}${note ? `｜补充说明：${note}` : ''}`
  try {
    await openDisputeMutation.mutateAsync({
      id: order.value.id,
      input: {
        issueCode: 'service_unavailable',
        requestedResolution: 'full_refund',
        requestedAmountCny: null,
        issueOccurredAt: new Date(credentialProblemOccurredAt.value).toISOString(),
        reason,
      },
      version: order.value.version,
      perspective: 'buyer',
    })
    credentialProblemOpen.value = false
    credentialProblemReason.value = ''
    credentialProblemOccurredAt.value = ''
    credentialProblemNote.value = ''
    toast.success('凭证问题已提交，请进入纠纷页面查看进度。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交凭证问题失败。')
  }
}
</script>

<template>
  <div class="mx-auto w-full max-w-3xl space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <Button as-child variant="ghost" class="-ml-3">
        <RouterLink to="/my/api-orders"><ArrowLeft class="h-4 w-4" />返回 API 购买订单</RouterLink>
      </Button>
      <Button v-if="order" as-child variant="outline" size="sm">
        <RouterLink :to="`/my/api-orders/${order.id}`">查看完整订单</RouterLink>
      </Button>
    </div>

    <header v-if="order" class="border-b border-border pb-4">
      <div class="flex items-start gap-3">
        <span class="grid h-10 w-10 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"><FileKey2 class="h-5 w-5" /></span>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h1 class="text-2xl font-semibold">卖家交付内容</h1>
            <Badge variant="secondary">{{ getApiOrderDisplayStatus(order, 'buyer') }}</Badge>
          </div>
          <p class="mt-1 break-words text-sm text-muted-foreground">{{ order.serviceTitle }}</p>
          <div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <ShortId :value="order.orderNo" full copyable />
            <span v-if="order.deliveryCredential">交付于 <LocalTime :value="order.deliveryCredential.submittedAt" /></span>
          </div>
        </div>
      </div>
    </header>

    <SkeletonBlock v-if="isLoading" :lines="6" />

    <div v-else-if="error" class="space-y-3">
      <ErrorState description="交付内容暂时无法加载，可能是订单不存在或当前账号无权查看。" @retry="refetch()" />
      <div class="text-center"><Button as-child variant="outline"><RouterLink to="/my/api-orders">返回订单列表</RouterLink></Button></div>
    </div>

    <EmptyState v-else-if="order && !order.deliveryCredential" title="卖家尚未提交交付内容" description="交付完成后，API Key、接入地址或登录账密会集中显示在这里。">
      <template #action>
        <Button as-child variant="outline"><RouterLink :to="`/my/api-orders/${order.id}`">查看完整订单</RouterLink></Button>
      </template>
    </EmptyState>

    <template v-else-if="order?.deliveryCredential">
      <ApiOrderDeliveryCredentialCard :credential="order.deliveryCredential" />

      <Alert v-if="catalogPaused" variant="destructive">
        <ShieldAlert />
        <AlertTitle>订单流程已暂停</AlertTitle>
        <AlertDescription>关联模型目录正在处理风险问题，交付内容仍可查看，普通售后操作暂时不可用。</AlertDescription>
      </Alert>

      <Alert v-else-if="disputeActive" class="border-warning/40 bg-warning/10">
        <ShieldAlert class="text-warning" />
        <AlertTitle>{{ getApiOrderDisputeStatusLabel(order.disputeStatus) }}</AlertTitle>
        <AlertDescription>
          <p>{{ getApiOrderDisputeStatusDescription(order.disputeStatus) || '凭证问题正在处理中，普通核验动作已暂停。' }}</p>
          <Button v-if="disputePanelId" as-child size="sm" variant="outline" class="mt-3">
            <RouterLink :to="`/my/disputes/${disputePanelId}?orderId=${order.id}`"><Headphones class="h-4 w-4" />进入纠纷处理</RouterLink>
          </Button>
        </AlertDescription>
      </Alert>

      <section v-else-if="canReportCredentialProblem" class="border-y border-border py-4" aria-labelledby="credential-review-title">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 id="credential-review-title" class="font-semibold">订单已完成，交付内容可继续查看</h2>
            <p class="mt-1 text-sm leading-6 text-muted-foreground">
							如接入地址、凭证、额度或权限存在问题，请联系商家或发起纠纷。平台不会代替你验证 API。
							<span v-if="order.afterSalesExpiresAt" class="block">纠纷截止：<LocalTime :value="order.afterSalesExpiresAt" /></span>
            </p>
          </div>
          <div class="flex shrink-0 flex-col gap-2 sm:flex-row">
						<Button as-child variant="outline">
							<RouterLink :to="`/my/api-orders/${order.id}`"><Headphones class="h-4 w-4" />联系商家</RouterLink>
						</Button>
            <Button variant="outline" class="border-warning/50 text-warning" :disabled="actionBusy" @click="credentialProblemOpen = true">
              <ShieldAlert class="h-4 w-4" />凭证存在问题
            </Button>
          </div>
        </div>
      </section>

      <Alert v-else-if="order.status === 'completed'">
        <CheckCircle2 class="text-success" />
        <AlertTitle>订单已完成</AlertTitle>
        <AlertDescription>交付内容仍可在平台保留期内查看；平台未对凭证可用性作出验证。</AlertDescription>
      </Alert>
    </template>

    <Dialog v-model:open="credentialProblemOpen">
      <DialogContent class="max-h-[92dvh] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>凭证存在问题</DialogTitle>
          <DialogDescription>选择最符合的原因。提交后订单进入纠纷处理，订单完成事实不会阻止售后处理。</DialogDescription>
        </DialogHeader>
        <RadioGroup v-model="credentialProblemReason" class="space-y-2">
          <div
            v-for="option in API_ORDER_CREDENTIAL_PROBLEM_OPTIONS"
            :key="option.value"
            class="flex items-start gap-3 rounded-md border border-border p-4 transition-colors has-[[data-state=checked]]:border-warning/60 has-[[data-state=checked]]:bg-warning/10 hover:bg-muted/40"
          >
            <RadioGroupItem :id="`credential-problem-${option.value}`" :value="option.value" class="mt-0.5" />
            <Label :for="`credential-problem-${option.value}`" class="min-w-0 flex-1 cursor-pointer font-normal">
              <span class="block text-sm font-medium">{{ option.label }}</span>
              <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            </Label>
          </div>
        </RadioGroup>
        <label class="block space-y-2">
          <span class="text-sm font-medium">问题实际发生时间</span>
          <Input v-model="credentialProblemOccurredAt" type="datetime-local" :max="credentialProblemOccurrenceMax" />
          <span class="block text-xs leading-5 text-muted-foreground">必须发生在所购服务有效期内；售后补报期不会延长服务有效期。</span>
        </label>
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
  </div>
</template>
