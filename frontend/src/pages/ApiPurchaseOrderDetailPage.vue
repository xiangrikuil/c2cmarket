<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { Flag, RotateCcw } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  apiIntentMerchantContactSnapshot,
  cancelApiPurchaseIntent,
  createManualInterventionReport,
  formatUsdQuota,
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  getApiDeliveryModeLabel,
  getApiIntentNextAction,
  getApiStatusLabel,
  getApiUsageVisibilityLabel,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import {
  apiPaymentMethodLabels,
  apiPaymentMethodRequiresQrCode,
  isApiPaymentOptionComplete,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'
import { useApiPurchaseIntent, useApiPurchaseIntentEvents } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const id = computed(() => String(route.params.id ?? ''))
const { data: intent, isLoading } = useApiPurchaseIntent(id)
const { data: events } = useApiPurchaseIntentEvents(id)
const actionBusy = ref(false)

const canCancel = computed(() => intent.value && ['open', 'contacted'].includes(intent.value.status))
const canCreateAgain = computed(() => intent.value && ['buyer_cancelled', 'owner_closed'].includes(intent.value.status))
const merchantContactSnapshot = computed(() => intent.value ? apiIntentMerchantContactSnapshot(intent.value) : null)
const paymentOptions = computed(() => intent.value?.snapshot.paymentOptions?.filter(option => option.enabled) ?? [])

function paymentOptionStatus(option: ApiPaymentOption) {
  if (isApiPaymentOptionComplete(option)) return '已就绪'
  return apiPaymentMethodRequiresQrCode(option.paymentMethod) ? '缺收款码' : '缺说明'
}

async function refresh() {
  await queryClient.invalidateQueries({ queryKey: ['api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['api-purchase-intent-events'] })
  await queryClient.invalidateQueries({ queryKey: ['my-api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['merchant-api-purchase-intents'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
  await queryClient.invalidateQueries({ queryKey: ['order-contacts', 'api-order'] })
}

async function runAction(action: () => Promise<unknown>, message: string) {
  actionBusy.value = true
  try {
    await action()
    await refresh()
    toast.success(message)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    actionBusy.value = false
  }
}

function cancelIntent() {
  if (!intent.value) return
  runAction(() => cancelApiPurchaseIntent(intent.value!.id, '买家不再继续该购买意向。'), '已取消购买意向。')
}

function requestManualIntervention() {
  if (!intent.value) return
  const description = window.prompt('请填写 4-1000 字脱敏说明。平台只记录处理状态和公开摘要，不追回付款、不托管、不担保、不裁决站外支付、不验真 API Key。')
  if (!description?.trim()) return
  runAction(async () => {
    await createManualInterventionReport({
      targetType: 'api_purchase_intent',
      targetId: intent.value!.id,
      targetLabel: intent.value!.snapshot.serviceTitle,
      reasonCode: 'api_quota_dispute',
      title: '举报 / 申请人工介入：API 接入或额度说明争议',
      description: description.trim(),
    })
    trackAnalytics('report_submit', {
      source_route: analyticsSourceRoute(),
      entity_type: 'api_purchase_intent',
      reason_code: 'api_quota_dispute',
    })
  }, '已提交人工介入申请。')
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载 API 意向记录...</div>
  <div v-else-if="!intent" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到 API 意向记录</h1>
    <p class="mt-2 text-sm text-muted-foreground">该意向记录不存在或暂不可见。</p>
    <Button class="mt-5" variant="outline" @click="router.push('/my/api-orders')">返回我的 API 意向</Button>
  </div>
  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <Badge>{{ getApiStatusLabel(intent.status) }}</Badge>
          <Badge variant="secondary">{{ getApiDeliveryModeLabel(intent.selectedDeliveryMode) }}</Badge>
          <span class="text-xs text-muted-foreground">{{ intent.id }}</span>
        </div>
        <h1 class="mt-2 text-2xl font-semibold tracking-tight">{{ intent.snapshot.serviceTitle }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">
          {{ getApiMerchantDisplayName(intent) }} · 信任等级{{ intent.snapshot.trustLevel }} · {{ getApiMerchantVisibilityLabel(intent.snapshot) }} · 快照记录，不随当前服务信息变化。
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button v-if="canCreateAgain" variant="outline" @click="router.push('/api-market')">
          再提交意向
        </Button>
        <Button v-if="canCancel" variant="outline" :disabled="actionBusy" @click="cancelIntent">
          <RotateCcw class="h-4 w-4" />取消
        </Button>
        <Button variant="outline" :disabled="actionBusy" @click="requestManualIntervention">
          <Flag class="h-4 w-4" />申请人工介入
        </Button>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-4">
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">意向金额</div>
        <div class="mt-1 text-xl font-semibold">¥{{ intent.purchaseAmountCny }}</div>
        <div class="text-xs text-muted-foreground">意向额度上限 {{ formatUsdQuota(intent.purchasedCredit) }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">倍率快照</div>
        <div class="mt-1 text-xl font-semibold">{{ intent.snapshot.multiplier }}</div>
        <div class="text-xs text-muted-foreground">¥1 对应 {{ formatUsdQuota(intent.snapshot.creditPerCny) }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">联系记录状态</div>
        <div class="mt-1 text-xl font-semibold">{{ intent.handoff.status === 'contacted' ? '商户已记录' : intent.handoff.status === 'closed' ? '已关闭' : '待商户记录' }}</div>
        <div class="text-xs text-muted-foreground">平台不保存账号、Key、token 或密码</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">下一步</div>
        <div class="mt-1 text-xl font-semibold">{{ getApiIntentNextAction(intent, 'buyer') }}</div>
        <div class="text-xs text-muted-foreground">{{ intent.updatedAt }}</div>
      </Card>
    </div>

    <div class="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      <Card class="p-5">
        <h2 class="font-semibold">意向快照</h2>
        <div class="mt-4 grid gap-3 text-sm sm:grid-cols-2">
          <div><span class="text-muted-foreground">模型</span><div>{{ intent.snapshot.models.join(' / ') }}</div></div>
          <div><span class="text-muted-foreground">目标模型</span><div>{{ intent.targetModel }}</div></div>
          <div><span class="text-muted-foreground">接入细节</span><div>提交意向后与商户站外确认</div></div>
          <div><span class="text-muted-foreground">用量核对</span><div>{{ getApiUsageVisibilityLabel(intent.snapshot.usageVisibility) }}</div></div>
          <div><span class="text-muted-foreground">商户承诺</span><div>{{ intent.snapshot.warranty }} · 平台不作保、不代赔</div></div>
          <div><span class="text-muted-foreground">取消/商户处理</span><div>{{ intent.snapshot.refundPolicy }}</div></div>
          <div><span class="text-muted-foreground">访问说明</span><div>按快照规则展示非敏感说明，站外仅允许确认买家专属、可撤销的子账号或子 Key。</div></div>
        </div>
          <div class="mt-4 rounded-md border border-border bg-accent/60 p-3 text-sm">
          购买意向创建后商户联系方式已向买家披露，商户也可查看买家选择的联系方式；后续站外沟通不代表平台处理支付、作保或验真。
        </div>
        <div class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-xs leading-5 text-muted-foreground">
          美元额度是商户声明的可购买额度上限参考，不代表平台余额、法币兑换、提现能力或平台承诺。
        </div>
        <div v-if="intent.status === 'buyer_cancelled'" class="mt-4 rounded-md border border-border p-3 text-sm">
          取消原因：{{ intent.buyerCancelReason }}
        </div>
        <div v-if="intent.status === 'owner_closed'" class="mt-4 rounded-md border border-border p-3 text-sm">
          商户关闭原因：{{ intent.ownerCloseReason }}
        </div>
      </Card>

      <div class="space-y-4">
        <Card v-if="paymentOptions.length" class="p-5">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="font-semibold">收款资料</h2>
              <p class="mt-1 text-xs text-muted-foreground">来自购买意向创建时的收款快照，仅参与方可见。</p>
            </div>
            <Badge variant="secondary">固定 10 分钟确认</Badge>
          </div>

          <div class="mt-4 space-y-3">
            <div v-for="option in paymentOptions" :key="option.paymentMethod" class="rounded-md border border-border p-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-medium">{{ apiPaymentMethodLabels[option.paymentMethod] }}</span>
                <Badge :variant="isApiPaymentOptionComplete(option) ? 'verified' : 'secondary'">{{ paymentOptionStatus(option) }}</Badge>
              </div>

              <div v-if="apiPaymentMethodRequiresQrCode(option.paymentMethod)" class="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center">
                <div class="grid h-28 w-28 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-muted/40">
                  <img v-if="option.paymentQrCodeDataUrl" :src="option.paymentQrCodeDataUrl" :alt="`${apiPaymentMethodLabels[option.paymentMethod]}收款码`" class="h-full w-full object-cover" />
                  <span v-else class="px-2 text-center text-xs text-muted-foreground">未上传收款码</span>
                </div>
                <p class="text-sm leading-6 text-muted-foreground">{{ option.paymentInstructions || '扫码后请通过商户联系方式站外确认。' }}</p>
              </div>
              <p v-else class="mt-3 whitespace-pre-line text-sm leading-6 text-muted-foreground">
                {{ option.paymentInstructions }}
              </p>
            </div>
          </div>
        </Card>

        <OrderContactCard
          v-if="merchantContactSnapshot"
          :snapshot="merchantContactSnapshot"
          title="联系商户"
          context-label="购买意向创建成功后即展示本次冻结的商户联系方式"
          visible-label="已向本次意向买家展示"
          hidden-label="仅参与方可见"
          footer-text="联系方式来自创建购买意向时冻结的快照；商户后续修改联系方式不会改变当前意向记录。"
          :show-contacted-action="false"
        />
      </div>
    </div>

    <Card class="p-5">
      <h2 class="font-semibold">事件时间线</h2>
      <div class="mt-4 space-y-3">
        <div v-for="event in events ?? []" :key="event.id" class="grid gap-1 border-b border-border pb-3 text-sm md:grid-cols-[180px_1fr]">
          <div class="text-muted-foreground">{{ event.createdAt }}</div>
          <div>
            <div class="font-medium">{{ event.actorLabel }} · {{ event.type }}</div>
            <div class="text-xs text-muted-foreground">
              {{ event.fromStatus ? getApiStatusLabel(event.fromStatus) : '创建' }}
              <span v-if="event.toStatus"> → {{ getApiStatusLabel(event.toStatus) }}</span>
            </div>
          </div>
        </div>
      </div>
    </Card>
  </div>
</template>
