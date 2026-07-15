<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CalendarClock, CarFront, CheckCircle2, CircleDollarSign, Clock3, FileText, Flag, MessageCircle, RotateCcw, ShieldAlert, Star, UserCheck, UsersRound, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import { Card } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
import { getProductCategory } from '@/lib/productCategories'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
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
const builtInProductIcons = new Map<string, string>()
const productIconSrc = computed(() => application.value ? getProductCategoryIconSrc(getProductCategory(application.value.snapshot.productName), builtInProductIcons) : null)

const rideProgressSteps = [
  { step: 1, label: '提交申请', description: '申请快照已记录' },
  { step: 2, label: '车主确认', description: '审核并预留席位' },
  { step: 3, label: '联系与上车', description: '双方完成站外确认' },
  { step: 4, label: '完成确认', description: '确认本次成员流程' },
]
const currentRideStep = computed(() => {
  const status = application.value?.status
  if (!status) return 1
  if (['completed', 'active', 'pending_completion'].includes(status)) return 4
  if (['contacted', 'joined_pending_confirmation'].includes(status)) return 3
  if (['accepted_reserved', 'waiting_contact'].includes(status)) return 2
  return 1
})

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
  await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
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
  <div v-else class="ride-order-detail-reference space-y-5">
    <header class="ride-order-detail-heading">
      <div class="text-xs text-muted-foreground">我的交易　/　我的上车　/　订单详情</div>
      <div class="mt-3 flex items-start gap-4">
        <span class="ride-order-product-icon"><img v-if="productIconSrc" :src="productIconSrc" alt="" /><CarFront v-else /></span>
        <div class="min-w-0"><div class="flex flex-wrap items-center gap-2"><h1>{{ application.snapshot.productName }}</h1><Badge>{{ getCarpoolApplicationStatusLabel(application.status) }}</Badge><Badge variant="secondary">{{ ownerMode ? '车主视角' : '买家视角' }}</Badge></div><p>{{ application.snapshot.regionName }} · 申请与规则均使用创建时快照，不随车源后续编辑变化。</p><div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground"><ShortId :value="application.id" prefix="RIDE" copyable /><span>更新于 <LocalTime :value="application.updatedAt" /></span></div></div>
      </div>
    </header>

    <div class="ride-order-detail-layout">
      <main class="min-w-0 space-y-4">
        <Card class="ride-order-progress p-5"><div class="flex items-center justify-between gap-4"><div><h2>上车进度</h2><p>{{ getCarpoolApplicationNextAction(application, ownerMode ? 'owner' : 'buyer') }}</p></div><Badge variant="secondary">第 {{ currentRideStep }} / 4 步</Badge></div><div class="ride-order-stepper"><div v-for="item in rideProgressSteps" :key="item.step" :class="{ 'is-done': item.step < currentRideStep, 'is-current': item.step === currentRideStep }"><span>{{ item.step < currentRideStep ? '✓' : item.step }}</span><div><strong>{{ item.label }}</strong><small>{{ item.description }}</small></div></div></div></Card>

        <Card class="ride-order-summary p-0"><dl><div><CircleDollarSign /><dt>月费快照</dt><dd>¥{{ application.snapshot.monthlyPriceCny }}</dd><small>{{ application.snapshot.priceLabel }}</small></div><div><UsersRound /><dt>申请席位</dt><dd>{{ application.seatsRequested }} 席</dd><small>{{ application.reservedUntil ? '当前存在预留窗口' : '无预留窗口' }}</small></div><div><CalendarClock /><dt>成员状态</dt><dd>{{ application.startedAt ? '已开始' : '未开始' }}</dd><small>{{ application.expectedEndAt ?? '等待双方确认上车' }}</small></div><div><Clock3 /><dt>当前下一步</dt><dd>{{ ownerMode ? '车主处理' : '申请人处理' }}</dd><small>{{ getCarpoolApplicationStatusLabel(application.status) }}</small></div></dl></Card>

        <Card class="ride-order-snapshot p-5"><div class="ride-order-section-title"><FileText /><div><h2>申请快照</h2><p>以下字段在申请创建时冻结</p></div></div><dl class="ride-order-detail-list"><div><dt>产品与地区</dt><dd>{{ application.snapshot.productName }} · {{ application.snapshot.regionName }}</dd></div><div><dt>开通方式</dt><dd>{{ application.snapshot.openingChannelName }}</dd></div><div><dt>上游订阅支付方式</dt><dd>{{ application.snapshot.paymentMethodNames.join(' / ') }}</dd></div><div><dt>车主与申请人</dt><dd>{{ application.ownerUsername }} / {{ application.applicantUsername }}</dd></div><div><dt>车主承诺</dt><dd>{{ application.snapshot.warrantyText }} · 平台不担保、不代赔</dd></div><div><dt>访问安排</dt><dd>{{ application.snapshot.accessArrangementNote ?? '成员邀请、费用分摊或站外访问安排，等待详情确认' }}</dd></div><div><dt>规则版本</dt><dd>{{ application.snapshot.rulesVersion }}</dd></div></dl><div class="mt-4 rounded-lg border bg-muted/30 p-3 text-sm leading-6">{{ application.snapshot.rulesText }}</div><div v-if="application.disputeReason" class="mt-4 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm">{{ application.disputeReason }}</div></Card>

        <section v-if="contactSnapshot" class="ride-order-contact-section"><div class="ride-order-section-title px-1"><MessageCircle /><div><h2>联系窗口</h2><p>仅在当前申请状态允许时展示参与方联系快照</p></div></div><OrderContactCard :snapshot="contactSnapshot" :title="ownerMode ? '联系申请人' : '联系车主'" :side="ownerMode ? 'buyer' : 'seller'" :contacted-label="ownerMode ? '已完成站外确认' : '我已联系车主'" :show-contacted-action="!realBackend" @contacted="markContactedFromCard" /></section>

        <Card class="ride-order-timeline p-5"><div class="ride-order-section-title"><Clock3 /><div><h2>事件时间线</h2><p>申请状态和双方动作的完整记录</p></div></div><div class="mt-5"><div v-for="event in events ?? []" :key="event.id" class="ride-order-event"><span></span><div><div class="flex flex-wrap justify-between gap-2"><strong>{{ event.actorLabel }} · {{ event.type }}</strong><small><LocalTime :value="event.createdAt" /></small></div><p>{{ event.fromStatus ? getCarpoolApplicationStatusLabel(event.fromStatus) : '创建' }}<span v-if="event.toStatus"> → {{ getCarpoolApplicationStatusLabel(event.toStatus) }}</span><span v-if="event.note"> · {{ event.note }}</span></p></div></div></div></Card>
      </main>

      <aside class="ride-order-aside space-y-4">
        <Card class="ride-order-action-card p-5"><div class="text-xs text-muted-foreground">当前状态与责任人</div><h2>{{ getCarpoolApplicationNextAction(application, ownerMode ? 'owner' : 'buyer') }}</h2><p>{{ ownerMode ? '当前为车主视角，只执行服务端允许的车主动作。' : '当前为申请人视角，等待车主时无需重复提交。' }}</p><div v-if="application.reservedUntil" class="ride-order-reservation"><Clock3 /><span>席位预留至<br /><LocalTime :value="application.reservedUntil" /></span></div><div class="mt-4 grid gap-2"><Button v-if="canOwnerProcess" :disabled="actionBusy" @click="acceptApplication"><CheckCircle2 class="h-4 w-4" />接受申请并预留席位</Button><Button v-else-if="!ownerMode && canBuyerConfirmJoined" :disabled="actionBusy" @click="buyerConfirmJoined"><UserCheck class="h-4 w-4" />确认已经上车</Button><Button v-else-if="ownerMode && canOwnerConfirmJoined" :disabled="actionBusy" @click="ownerConfirmJoined"><UserCheck class="h-4 w-4" />确认用户已上车</Button><Button v-else-if="!ownerMode && canConfirmCompleted" :disabled="actionBusy" @click="buyerConfirmCompleted"><CheckCircle2 class="h-4 w-4" />确认本次完成</Button><Button v-else-if="ownerMode && canConfirmCompleted" :disabled="actionBusy" @click="ownerConfirmCompleted"><CheckCircle2 class="h-4 w-4" />确认本次完成</Button><Button v-else-if="canReview" :disabled="actionBusy" @click="submitReview"><Star class="h-4 w-4" />评价车主</Button><Button v-if="canOwnerProcess" variant="outline" :disabled="actionBusy" @click="rejectPanelOpen = !rejectPanelOpen"><XCircle class="h-4 w-4" />拒绝申请</Button><Button v-if="!canOwnerProcess" variant="outline" :disabled="actionBusy" @click="requestManualIntervention"><Flag class="h-4 w-4" />申请人工介入</Button><Button v-if="canOwnerWithdrawAcceptance" variant="outline" :disabled="actionBusy" @click="withdrawAcceptance"><RotateCcw class="h-4 w-4" />撤回接受</Button><Button v-if="canRemoveMember" variant="outline" :disabled="actionBusy" @click="openDispute"><ShieldAlert class="h-4 w-4" />{{ realBackend ? '移除成员' : '纠纷' }}</Button><Button v-if="canBuyerCancelApplication" variant="outline" :disabled="actionBusy" @click="cancelApplication"><RotateCcw class="h-4 w-4" />{{ buyerCancelLabel }}</Button><Button v-if="canBuyerLeaveMembership" variant="outline" :disabled="actionBusy" @click="leaveMembership"><RotateCcw class="h-4 w-4" />退出拼车</Button></div>

          <div v-if="canOwnerProcess && rejectPanelOpen" class="mt-4 space-y-3 border-t border-border pt-4"><label class="space-y-2 text-sm"><span class="font-medium">拒绝原因</span><Select v-model="rejectReasonCode"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem v-for="item in rejectReasonOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem></SelectContent></Select></label><label class="space-y-2 text-sm"><span class="font-medium">补充说明</span><Textarea v-model="rejectReasonText" rows="2" placeholder="说明原因，不要填写联系方式或敏感凭据。" /></label><Button class="w-full" variant="destructive" :disabled="actionBusy" @click="rejectApplication">确认拒绝</Button></div>
        </Card>

        <Card class="p-5"><div class="ride-order-section-title"><UsersRound /><div><h2>参与方摘要</h2><p>基于公开和订单快照数据</p></div></div><div class="mt-4 space-y-3 text-sm"><div class="flex justify-between"><span class="text-muted-foreground">申请人</span><strong>{{ application.applicantUsername }}</strong></div><div class="flex justify-between"><span class="text-muted-foreground">车主</span><strong>{{ application.ownerUsername }}</strong></div><div class="flex justify-between"><span class="text-muted-foreground">linux.do</span><span>{{ application.applicantStats.linuxdoBound ? '已绑定' : '未绑定' }}</span></div><div class="flex justify-between"><span class="text-muted-foreground">近 30 天完成</span><span>{{ application.applicantStats.completed30d }} 次</span></div><div class="flex justify-between"><span class="text-muted-foreground">未解决纠纷</span><span>{{ application.applicantStats.unresolvedDisputes }}</span></div><RouterLink :to="`/u/${application.applicantUsername}`"><Button class="mt-2 w-full" variant="outline">查看公开主页</Button></RouterLink></div></Card>

        <Card class="p-5"><div class="ride-order-section-title"><ShieldAlert /><div><h2>平台边界</h2><p>交易与联系规则</p></div></div><ul class="mt-4 space-y-2 text-xs leading-5 text-muted-foreground"><li>平台记录申请状态，不代收或托管拼车费用。</li><li>联系方式只在有效联系窗口向参与方展示。</li><li>不要共享密码、Cookie、Session 或其他账号凭据。</li></ul></Card>
      </aside>
    </div>
  </div>
</template>
