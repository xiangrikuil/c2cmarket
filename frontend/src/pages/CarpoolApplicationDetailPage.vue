<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, Flag, RotateCcw, ShieldAlert, Star, UserCheck, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import {
  acceptCarpoolApplication,
  buyerConfirmCarpoolCompleted,
  buyerConfirmCarpoolJoined,
  cancelCarpoolApplication,
  createManualInterventionReport,
  disputeCarpoolApplication,
  getCarpoolApplicationNextAction,
  getCarpoolApplicationStatusLabel,
  leaveCarpoolMembership,
  markCarpoolApplicationContacted,
  ownerConfirmCarpoolCompleted,
  ownerConfirmCarpoolJoined,
  rejectCarpoolApplication,
  reviewCarpoolApplication,
  withdrawCarpoolAcceptance,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { shouldUseRealBackend } from '@/lib/backendClient'
import { useCarpoolApplication, useCarpoolApplicationContactsQuery, useCarpoolApplicationEvents } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const id = computed(() => String(route.params.id ?? ''))
const ownerMode = computed(() => route.path.startsWith('/merchant/'))
const { data: application, isLoading } = useCarpoolApplication(id)
const { data: events } = useCarpoolApplicationEvents(id)
const { data: contactSnapshot } = useCarpoolApplicationContactsQuery(id)
const actionBusy = ref(false)
const rejectPanelOpen = ref(false)
const rejectReasonCode = ref('seat_full')
const rejectReasonText = ref('')
const realBackend = shouldUseRealBackend()

const canBuyerContact = computed(() => application.value && ['accepted_reserved', 'waiting_contact'].includes(application.value.status))
const joinConfirmStatuses = computed(() => realBackend ? ['accepted_reserved', 'joined_pending_confirmation'] : ['contacted', 'joined_pending_confirmation'])
const canBuyerConfirmJoined = computed(() => application.value && joinConfirmStatuses.value.includes(application.value.status))
const canOwnerConfirmJoined = computed(() => application.value && joinConfirmStatuses.value.includes(application.value.status))
const canOwnerProcess = computed(() => ownerMode.value && application.value?.status === 'pending_owner')
const activeMembershipStatuses = computed(() => realBackend ? ['active', 'pending_completion'] : ['pending_completion'])
const canConfirmCompleted = computed(() => application.value && activeMembershipStatuses.value.includes(application.value.status))
const canRemoveMember = computed(() => application.value && (realBackend ? ownerMode.value && ['active', 'pending_completion'].includes(application.value.status) : ['active', 'pending_completion'].includes(application.value.status)))
const canOwnerWithdrawAcceptance = computed(() => ownerMode.value && application.value && (realBackend ? ['accepted_reserved', 'joined_pending_confirmation'].includes(application.value.status) : ['accepted_reserved', 'waiting_contact'].includes(application.value.status)))
const canBuyerCancelApplication = computed(() => application.value && !ownerMode.value && (realBackend ? ['pending_owner', 'accepted_reserved', 'joined_pending_confirmation'].includes(application.value.status) : ['pending_owner', 'accepted_reserved', 'waiting_contact', 'contacted'].includes(application.value.status)))
const canBuyerLeaveMembership = computed(() => application.value && !ownerMode.value && realBackend && ['active', 'pending_completion'].includes(application.value.status))
const buyerCancelLabel = computed(() => application.value?.status === 'accepted_reserved' ? '取消预留' : '撤回申请')
const canReview = computed(() => application.value?.status === 'completed' && !application.value.buyerReview && !ownerMode.value)
const rejectReasonOptions = [
  { value: 'seat_full', label: '席位已满' },
  { value: 'user_not_fit', label: '用户条件不符合' },
  { value: 'product_rule_mismatch', label: '产品规则不匹配' },
  { value: 'incomplete_application', label: '申请信息不完整' },
  { value: 'other', label: '其他原因' },
]
const rejectReason = computed(() => {
  const label = rejectReasonOptions.find(item => item.value === rejectReasonCode.value)?.label ?? '其他原因'
  const note = rejectReasonText.value.trim()
  return rejectReasonCode.value === 'other' ? note : note ? `${label}：${note}` : label
})

async function refresh() {
  await queryClient.invalidateQueries({ queryKey: ['carpool-application'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-application-events'] })
  await queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] })
  await queryClient.invalidateQueries({ queryKey: ['carpools'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
  await queryClient.invalidateQueries({ queryKey: ['order-contacts', 'carpool-application'] })
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

function markContacted() {
  if (!application.value) return
  runAction(() => markCarpoolApplicationContacted(application.value!.id), '已记录完成站外联系。')
}

function markContactedFromCard() {
  if (realBackend) {
    toast('联系窗口已开放，可直接确认上车。')
    return
  }
  if (!ownerMode.value && canBuyerContact.value) {
    markContacted()
    return
  }
  toast('当前申请已过联系记录步骤。')
}

function acceptApplication() {
  if (!application.value) return
  runAction(() => acceptCarpoolApplication(application.value!.id), '已接受申请，并预留 1 个席位 30 分钟。')
}

function rejectApplication() {
  if (!application.value) return
  if (rejectReasonCode.value === 'other' && !rejectReasonText.value.trim()) {
    toast.warning('选择其他原因时必须填写补充说明。')
    return
  }
  runAction(() => rejectCarpoolApplication(application.value!.id, rejectReason.value), '已拒绝申请，并记录原因。')
}

function buyerConfirmJoined() {
  if (!application.value) return
  runAction(() => buyerConfirmCarpoolJoined(application.value!.id), '已记录买家确认上车。')
}

function ownerConfirmJoined() {
  if (!application.value) return
  runAction(() => ownerConfirmCarpoolJoined(application.value!.id), '已记录车主确认上车。')
}

function buyerConfirmCompleted() {
  if (!application.value) return
  runAction(() => buyerConfirmCarpoolCompleted(application.value!.id), '已记录买家确认完成。')
}

function ownerConfirmCompleted() {
  if (!application.value) return
  runAction(() => ownerConfirmCarpoolCompleted(application.value!.id), '已记录车主确认完成。')
}

function cancelApplication() {
  if (!application.value) return
  runAction(() => cancelCarpoolApplication(application.value!.id, application.value!.status === 'accepted_reserved' ? '买家主动取消预留' : '买家主动撤回申请'), '已取消上车申请。')
}

function leaveMembership() {
  if (!application.value) return
  runAction(() => leaveCarpoolMembership(application.value!.id, '买家主动退出成员关系'), '已退出拼车。')
}

function withdrawAcceptance() {
  if (!application.value) return
  runAction(() => withdrawCarpoolAcceptance(application.value!.id, '车主主动撤回接受并取消预留'), '已取消预留。')
}

function openDispute() {
  if (!application.value) return
  runAction(() => disputeCarpoolApplication(application.value!.id, realBackend ? '车主主动移除成员关系' : '用户发起人工复核'), realBackend ? '已移除成员关系。' : '已进入纠纷记录。')
}

function requestManualIntervention() {
  if (!application.value) return
  const description = window.prompt('请填写 4-1000 字脱敏说明。平台只记录处理状态和公开摘要，不追回付款、不托管、不担保、不裁决站外支付、不验真 API Key。')
  if (!description?.trim()) return
  runAction(async () => {
    await createManualInterventionReport({
      targetType: 'carpool_application',
      targetId: application.value!.id,
      targetLabel: application.value!.snapshot.productName,
      reasonCode: 'seat_rule_dispute',
      title: '举报 / 申请人工介入：规则或席位争议',
      description: description.trim(),
    })
    trackAnalytics('report_submit', {
      source_route: analyticsSourceRoute(),
      entity_type: 'carpool_application',
      reason_code: 'seat_rule_dispute',
    })
  }, '已提交人工介入申请。')
}

function submitReview() {
  if (!application.value) return
  runAction(() => reviewCarpoolApplication(application.value!.id, { rating: 5, tags: ['规则清楚', '服务稳定'], note: '规则清楚，服务稳定。' }), '评价已记录。')
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载上车申请...</div>
  <div v-else-if="!application" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到上车申请</h1>
    <p class="mt-2 text-sm text-muted-foreground">该申请不存在或暂不可见。</p>
    <Button class="mt-5" variant="outline" @click="router.push('/my/rides')">返回我的上车</Button>
  </div>
  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <Badge>{{ getCarpoolApplicationStatusLabel(application.status) }}</Badge>
          <Badge variant="secondary">{{ application.id }}</Badge>
          <span class="text-xs text-muted-foreground">{{ ownerMode ? '车主视角' : '买家视角' }}</span>
        </div>
        <h1 class="mt-2 text-2xl font-semibold tracking-tight">{{ application.snapshot.productName }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">
          {{ application.snapshot.regionName }} · {{ application.applicantUsername }} / {{ application.ownerUsername }} · 快照记录，不随车源编辑变化。
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button v-if="!ownerMode && canBuyerConfirmJoined" :disabled="actionBusy" @click="buyerConfirmJoined"><UserCheck class="h-4 w-4" />确认已经上车</Button>
        <Button v-if="ownerMode && canOwnerConfirmJoined" :disabled="actionBusy" @click="ownerConfirmJoined"><UserCheck class="h-4 w-4" />确认用户已上车</Button>
        <Button v-if="!ownerMode && canConfirmCompleted" :disabled="actionBusy" @click="buyerConfirmCompleted"><CheckCircle2 class="h-4 w-4" />确认本期完成</Button>
        <Button v-if="ownerMode && canConfirmCompleted" :disabled="actionBusy" @click="ownerConfirmCompleted"><CheckCircle2 class="h-4 w-4" />确认本期完成</Button>
        <Button v-if="canReview" variant="outline" :disabled="actionBusy" @click="submitReview"><Star class="h-4 w-4" />评价车主</Button>
        <Button v-if="canOwnerWithdrawAcceptance" variant="outline" :disabled="actionBusy" @click="withdrawAcceptance"><RotateCcw class="h-4 w-4" />撤回接受</Button>
        <Button v-if="canRemoveMember" variant="outline" :disabled="actionBusy" @click="openDispute"><ShieldAlert class="h-4 w-4" />{{ realBackend ? '移除成员' : '纠纷' }}</Button>
        <Button variant="outline" :disabled="actionBusy" @click="requestManualIntervention"><Flag class="h-4 w-4" />申请人工介入</Button>
        <Button v-if="canBuyerCancelApplication" variant="outline" :disabled="actionBusy" @click="cancelApplication"><RotateCcw class="h-4 w-4" />{{ buyerCancelLabel }}</Button>
        <Button v-if="canBuyerLeaveMembership" variant="outline" :disabled="actionBusy" @click="leaveMembership"><RotateCcw class="h-4 w-4" />退出拼车</Button>
      </div>
    </div>

    <Card v-if="canOwnerProcess" class="border-primary/30 p-5">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h2 class="font-semibold">处理申请</h2>
          <p class="mt-1 text-sm text-muted-foreground">接受后预留席位并开启联系窗口，不代表收款、托管或提供账号凭据；拒绝必须记录原因，便于申请人查看和后续审计。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" :disabled="actionBusy" @click="rejectPanelOpen = !rejectPanelOpen">
            <XCircle class="h-4 w-4" />拒绝申请
          </Button>
          <Button :disabled="actionBusy" @click="acceptApplication">
            <CheckCircle2 class="h-4 w-4" />接受申请
          </Button>
        </div>
      </div>
      <div v-if="rejectPanelOpen" class="mt-4 grid gap-3 rounded-lg border border-border bg-muted/40 p-3 md:grid-cols-[220px_1fr_auto] md:items-start">
        <label class="space-y-1 text-sm">
          <span class="font-medium">拒绝原因</span>
          <select v-model="rejectReasonCode" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option v-for="item in rejectReasonOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label class="space-y-1 text-sm">
          <span class="font-medium">补充说明</span>
          <Textarea v-model="rejectReasonText" rows="2" placeholder="说明原因，不要填写联系方式或敏感凭据。" />
        </label>
        <Button class="md:mt-6" variant="destructive" :disabled="actionBusy" @click="rejectApplication">确认拒绝</Button>
      </div>
    </Card>

    <div class="grid gap-3 md:grid-cols-4">
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">下一步</div>
        <div class="mt-1 text-lg font-semibold">{{ getCarpoolApplicationNextAction(application, ownerMode ? 'owner' : 'buyer') }}</div>
        <div class="text-xs text-muted-foreground">{{ application.updatedAt }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">月费快照</div>
        <div class="mt-1 text-lg font-semibold">¥{{ application.snapshot.monthlyPriceCny }}</div>
        <div class="text-xs text-muted-foreground">{{ application.snapshot.priceLabel }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">席位</div>
        <div class="mt-1 text-lg font-semibold">{{ application.seatsRequested }} 席</div>
        <div class="text-xs text-muted-foreground">{{ application.reservedUntil ? `预留至 ${application.reservedUntil}` : '无预留窗口' }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">服务周期</div>
        <div class="mt-1 text-lg font-semibold">{{ application.startedAt ? '已开始' : '未开始' }}</div>
        <div class="text-xs text-muted-foreground">{{ application.expectedEndAt ?? '等待双方确认上车' }}</div>
      </Card>
    </div>

    <div class="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      <Card class="p-5">
        <h2 class="font-semibold">申请快照</h2>
        <div class="mt-4 grid gap-3 text-sm sm:grid-cols-2">
          <div><span class="text-muted-foreground">产品</span><div>{{ application.snapshot.productName }}</div></div>
          <div><span class="text-muted-foreground">开通区</span><div>{{ application.snapshot.regionName }}</div></div>
          <div><span class="text-muted-foreground">开通方式</span><div>{{ application.snapshot.openingChannelName }}</div></div>
          <div><span class="text-muted-foreground">支付方式</span><div>{{ application.snapshot.paymentMethodNames.join(' / ') }}</div></div>
          <div><span class="text-muted-foreground">车主承诺</span><div>{{ application.snapshot.warrantyText }} · 平台不担保、不代赔</div></div>
          <div><span class="text-muted-foreground">访问安排</span><div>{{ application.snapshot.accessArrangementNote ?? '成员邀请、费用分摊或站外访问安排，等待详情确认' }}</div></div>
          <div><span class="text-muted-foreground">规则版本</span><div>{{ application.snapshot.rulesVersion }}</div></div>
          <div><span class="text-muted-foreground">车主</span><div>{{ application.ownerUsername }} · 信任等级{{ application.snapshot.ownerTrustLevel }}</div></div>
          <div><span class="text-muted-foreground">申请人</span><div>{{ application.applicantUsername }} · 信任等级{{ application.applicantStats.trustLevel }}</div></div>
        </div>
        <div class="mt-4 rounded-md border border-border bg-accent/60 p-3 text-sm">{{ application.snapshot.rulesText }}</div>
        <div v-if="application.disputeReason" class="mt-4 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">{{ application.disputeReason }}</div>
      </Card>

      <Card class="p-5">
        <h2 class="font-semibold">申请人摘要</h2>
        <div class="mt-4 space-y-3 text-sm">
          <div class="flex justify-between"><span class="text-muted-foreground">linux.do</span><span>{{ application.applicantStats.linuxdoBound ? '已绑定' : '未绑定' }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">近 30 天完成</span><span>{{ application.applicantStats.completed30d }} 次</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">买家责任取消</span><span>{{ application.applicantStats.buyerResponsibleCancellations }} 次</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">未解决纠纷</span><span>{{ application.applicantStats.unresolvedDisputes }}</span></div>
          <RouterLink :to="`/u/${application.applicantUsername}`">
            <Button class="w-full" variant="outline">查看公开主页</Button>
          </RouterLink>
        </div>
      </Card>
    </div>

    <OrderContactCard
      v-if="contactSnapshot"
      :snapshot="contactSnapshot"
      :title="ownerMode ? '联系申请人' : '联系车主'"
      :side="ownerMode ? 'buyer' : 'seller'"
      :contacted-label="ownerMode ? '已完成站外确认' : '我已联系车主'"
      :show-contacted-action="!realBackend"
      @contacted="markContactedFromCard"
    />

    <Card class="p-5">
      <h2 class="font-semibold">事件时间线</h2>
      <div class="mt-4 space-y-3">
        <div v-for="event in events ?? []" :key="event.id" class="grid gap-1 border-b border-border pb-3 text-sm md:grid-cols-[180px_1fr]">
          <div class="text-muted-foreground">{{ event.createdAt }}</div>
          <div>
            <div class="font-medium">{{ event.actorLabel }} · {{ event.type }}</div>
            <div class="text-xs text-muted-foreground">
              {{ event.fromStatus ? getCarpoolApplicationStatusLabel(event.fromStatus) : '创建' }}
              <span v-if="event.toStatus"> → {{ getCarpoolApplicationStatusLabel(event.toStatus) }}</span>
              <span v-if="event.note"> · {{ event.note }}</span>
            </div>
          </div>
        </div>
      </div>
    </Card>
  </div>
</template>
