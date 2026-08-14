<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ArrowLeft, Clock3, FileCheck2, ShieldAlert, UsersRound } from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import {
  getApiOrderCompletionSourceLabel,
  getApiOrderDisputeStatusDescription,
  getApiOrderDisputeStatusLabel,
  getApiOrderPaymentIssueLabel,
  getApiOrderStatusLabel,
  isApiOrderDisputeActive,
} from '@/lib/api'
import { formatOrderDateTime } from '@/lib/apiOrderUi'
import { apiPaymentMethodLabels } from '@/lib/apiPaymentSettings'
import { formatDecimal } from '@/lib/decimal'
import { useAdminApiOrder, useResolveAdminApiOrderCatalogRiskHoldMutation } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id ?? ''))
const { data: order, isLoading, error, refetch } = useAdminApiOrder(id)
const resolveRiskHoldMutation = useResolveAdminApiOrderCatalogRiskHoldMutation()
const riskHoldDialogOpen = ref(false)
const riskHoldAction = ref<'restore' | 'refund-pending' | 'open-dispute'>('restore')
const riskHoldNote = ref('')
const riskHoldActionLabel = computed(() => ({
  restore: '恢复订单流程',
  'refund-pending': '记录退款待处理',
  'open-dispute': '转入纠纷处置',
}[riskHoldAction.value]))

function openRiskHoldDialog(action: typeof riskHoldAction.value) {
  riskHoldAction.value = action
  riskHoldNote.value = ''
  riskHoldDialogOpen.value = true
}

async function resolveRiskHold() {
  const hold = order.value?.catalogRiskHold
  if (!order.value || !hold || hold.status !== 'active' || riskHoldNote.value.trim().length < 2) return
  try {
    await resolveRiskHoldMutation.mutateAsync({
      id: order.value.id,
      action: riskHoldAction.value,
      resolutionNote: riskHoldNote.value.trim(),
      version: hold.version,
    })
    riskHoldDialogOpen.value = false
    toast.success('目录风险暂停处置结果已记录。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '目录风险暂停处置失败。')
  }
}

const timeline = computed(() => {
  if (!order.value) return []
  const rows = [
    { key: 'created', label: '订单创建', actor: '买家', time: order.value.createdAt, note: '订单与服务快照已锁定' },
    order.value.paymentSubmittedAt
      ? { key: 'payment', label: '提交付款状态', actor: '买家', time: order.value.paymentSubmittedAt, note: '等待商户核对实际到账' }
      : null,
    order.value.paymentIssueReportedAt
      ? { key: 'payment-issue', label: '报告付款问题', actor: '商户', time: order.value.paymentIssueReportedAt, note: getApiOrderPaymentIssueLabel(order.value.paymentIssueReason) }
      : null,
    order.value.paidConfirmedAt
      ? { key: 'paid-confirmed', label: '确认收款', actor: '商户', time: order.value.paidConfirmedAt, note: '商户已确认站外收款' }
      : null,
    order.value.deliverySubmittedAt
      ? { key: 'delivery', label: '完成交付', actor: '商户', time: order.value.deliverySubmittedAt, note: order.value.deliveryNote || '已提交一次性交付凭证' }
      : null,
    order.value.completedAt
      ? { key: 'completed', label: '订单完成', actor: order.value.completionSource === 'auto_completed' ? '系统' : '买家', time: order.value.completedAt, note: getApiOrderCompletionSourceLabel(order.value.completionSource) }
      : null,
    order.value.cancelledAt
      ? { key: 'cancelled', label: '订单取消', actor: '系统或买家', time: order.value.cancelledAt, note: order.value.cancelReason || '订单已取消' }
      : null,
  ]
  return rows.filter((row): row is NonNullable<typeof row> => row !== null)
})

const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '管理员订单详情暂时无法加载。')
</script>

<template>
  <Dialog v-model:open="riskHoldDialogOpen">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ riskHoldActionLabel }}</DialogTitle>
        <DialogDescription>该动作只记录风险处置事实，不会自动退款、取消订单或改变站外资金关系。</DialogDescription>
      </DialogHeader>
      <div class="space-y-2">
        <Label for="catalog-risk-hold-note">处置说明</Label>
        <Textarea id="catalog-risk-hold-note" v-model="riskHoldNote" maxlength="500" placeholder="填写已核验的处置结果与后续安排" />
      </div>
      <DialogFooter>
        <Button variant="outline" @click="riskHoldDialogOpen = false">取消</Button>
        <Button :disabled="riskHoldNote.trim().length < 2 || resolveRiskHoldMutation.isPending.value" @click="resolveRiskHold">确认记录</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
  <SkeletonBlock v-if="isLoading" :lines="10" />
  <ErrorState v-else-if="error" :description="errorMessage" @retry="refetch()" />
  <EmptyState v-else-if="!order" title="未找到 API 订单" description="订单不存在或当前管理员无权查看。">
    <template #action><Button variant="outline" @click="router.push('/admin/trade-intents')">返回订单监管</Button></template>
  </EmptyState>
  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-start md:justify-between">
      <div class="min-w-0">
        <Button class="-ml-3 mb-2" variant="ghost" size="sm" @click="router.push('/admin/trade-intents')"><ArrowLeft class="h-4 w-4" />返回订单监管</Button>
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold">API 订单监管详情</h1>
          <StatusBadge :status="order.status" :label="getApiOrderStatusLabel(order.status, 'admin')" />
          <Badge variant="secondary">只读</Badge>
        </div>
        <p class="mt-2 text-sm text-muted-foreground">{{ order.serviceTitleSnapshot }} · <ShortId :value="order.id" prefix="API" copyable /></p>
      </div>
      <RouterLink v-if="order.disputeStatus === 'open'" to="/admin/reports"><Button variant="outline"><ShieldAlert class="h-4 w-4" />进入纠纷处理</Button></RouterLink>
    </div>

    <Alert v-if="order.disputeStatus !== 'none'" :class="isApiOrderDisputeActive(order.disputeStatus) ? 'border-warning/35 bg-warning/10' : 'border-border bg-muted/20'">
      <ShieldAlert :class="isApiOrderDisputeActive(order.disputeStatus) ? 'text-warning' : 'text-muted-foreground'" />
      <AlertTitle>{{ getApiOrderDisputeStatusLabel(order.disputeStatus) }}</AlertTitle>
      <AlertDescription>{{ getApiOrderDisputeStatusDescription(order.disputeStatus) }}<span v-if="order.disputeCaseId"> 纠纷编号 {{ order.disputeCaseId }}。</span></AlertDescription>
    </Alert>

    <Alert v-if="order.catalogRiskHold" :class="order.catalogRiskHold.status === 'active' ? 'border-destructive/35 bg-destructive/5' : 'border-border bg-muted/20'">
      <ShieldAlert :class="order.catalogRiskHold.status === 'active' ? 'text-destructive' : 'text-muted-foreground'" />
      <AlertTitle>{{ order.catalogRiskHold.status === 'active' ? '目录风险暂停待处置' : '目录风险暂停已处置' }}</AlertTitle>
      <AlertDescription class="space-y-3">
        <p>{{ order.catalogRiskHold.reason }}<span v-if="order.catalogRiskHold.resolutionNote"> 处置说明：{{ order.catalogRiskHold.resolutionNote }}</span></p>
        <div v-if="order.catalogRiskHold.status === 'active'" class="flex flex-wrap gap-2">
          <Button size="sm" @click="openRiskHoldDialog('restore')">恢复订单流程</Button>
          <Button size="sm" variant="outline" @click="openRiskHoldDialog('refund-pending')">退款待处理</Button>
          <Button size="sm" variant="outline" @click="openRiskHoldDialog('open-dispute')">转入纠纷</Button>
        </div>
      </AlertDescription>
    </Alert>

    <div class="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
      <div class="space-y-4">
        <Card class="p-5">
          <div class="flex items-center gap-2"><FileCheck2 class="h-4 w-4 text-primary" /><h2 class="font-semibold">订单与服务快照</h2></div>
          <dl class="mt-4 grid gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
            <div><dt class="text-xs text-muted-foreground">服务</dt><dd class="mt-1 font-medium">{{ order.serviceTitleSnapshot }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">订单金额</dt><dd class="mt-1 text-lg font-semibold">¥{{ formatDecimal(order.amount, 2, 2) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">购买类型</dt><dd class="mt-1">{{ order.purchaseKind === 'limited_quota_offer' ? '限量额度包' : 'API 服务' }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">购买额度</dt><dd class="mt-1">{{ order.requestedUsdAllowanceSnapshot ? `${formatDecimal(order.requestedUsdAllowanceSnapshot, 2, 6)} 美元额度` : '不适用' }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">付款方式</dt><dd class="mt-1 inline-flex items-center gap-2"><ApiPaymentMethodIcon :method="order.selectedPaymentMethod" size="sm" />{{ apiPaymentMethodLabels[order.selectedPaymentMethod] }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">定价快照</dt><dd class="mt-1">{{ order.cnyPerUsdAllowanceSnapshot ? `¥${formatDecimal(order.cnyPerUsdAllowanceSnapshot, 3, 6)} / $1` : '按套餐快照' }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">服务 ID</dt><dd class="mt-1"><ShortId :value="order.apiServiceId" prefix="SVC" copyable /></dd></div>
            <div><dt class="text-xs text-muted-foreground">购买意向 ID</dt><dd class="mt-1"><ShortId :value="order.apiPurchaseIntentId" prefix="INT" copyable /></dd></div>
          </dl>
        </Card>

        <Card class="p-5">
          <div class="flex items-center gap-2"><Clock3 class="h-4 w-4 text-primary" /><h2 class="font-semibold">履约时间线</h2></div>
          <ol class="mt-5 space-y-0">
            <li v-for="(item, index) in timeline" :key="item.key" class="relative grid grid-cols-[24px_1fr] gap-3 pb-5 last:pb-0">
              <span class="relative z-10 mt-0.5 grid h-6 w-6 place-items-center rounded-full border border-primary/30 bg-background text-xs font-semibold text-primary">{{ index + 1 }}</span>
              <span v-if="index < timeline.length - 1" class="absolute left-[11px] top-6 h-[calc(100%-4px)] w-px bg-border" />
              <div class="min-w-0">
                <div class="flex flex-wrap items-center justify-between gap-2"><strong class="text-sm">{{ item.label }}</strong><time class="text-xs text-muted-foreground">{{ formatOrderDateTime(item.time) }}</time></div>
                <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ item.actor }} · {{ item.note }}</p>
              </div>
            </li>
          </ol>
        </Card>
      </div>

      <div class="space-y-4">
        <Card class="p-5">
          <div class="flex items-center gap-2"><UsersRound class="h-4 w-4 text-primary" /><h2 class="font-semibold">订单参与方</h2></div>
          <dl class="mt-4 space-y-4 text-sm">
            <div><dt class="text-xs text-muted-foreground">买家用户 ID</dt><dd class="mt-1"><ShortId :value="order.buyerUserId" prefix="BUY" copyable /></dd></div>
            <div><dt class="text-xs text-muted-foreground">商户用户 ID</dt><dd class="mt-1"><ShortId :value="order.sellerUserId" prefix="SEL" copyable /></dd></div>
          </dl>
        </Card>

        <Card class="p-5">
          <h2 class="font-semibold">核验与完成</h2>
          <dl class="mt-4 space-y-4 text-sm">
            <div><dt class="text-xs text-muted-foreground">交付时间</dt><dd class="mt-1">{{ formatOrderDateTime(order.deliverySubmittedAt) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">核验截止</dt><dd class="mt-1">{{ formatOrderDateTime(order.deliveryReviewExpiresAt) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">完成方式</dt><dd class="mt-1">{{ getApiOrderCompletionSourceLabel(order.completionSource) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">完成时间</dt><dd class="mt-1">{{ formatOrderDateTime(order.completedAt) }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">纠纷状态</dt><dd class="mt-1">{{ getApiOrderDisputeStatusLabel(order.disputeStatus) }}</dd></div>
          </dl>
        </Card>

        <Alert>
          <ShieldAlert />
          <AlertTitle>监管信息边界</AlertTitle>
          <AlertDescription>管理视图仅保留履约事实，不展示原始交付凭证或双方联系方式。</AlertDescription>
        </Alert>
      </div>
    </div>
  </div>
</template>
