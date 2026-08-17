<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CarFront, CheckCircle2, CircleDollarSign, Clock3, FileText, Flag, MessageCircle, RotateCcw, ShieldAlert, UsersRound, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import OrderContactCard from '@/components/profile/OrderContactCard.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import ReputationSummaryCard from '@/components/reputation/ReputationSummaryCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  acceptCarpoolApplication,
  cancelCarpoolApplication,
  confirmCarpoolApplicationConditions,
  createManualInterventionReport,
  getCarpoolApplicationNextAction,
  getCarpoolApplicationStatusLabel,
  leaveCarpoolMembership,
  removeCarpoolMember,
  rejectCarpoolApplication,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { functionalMotion } from '@/lib/motion'
import { getProductCategory } from '@/lib/productCategories'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import { useCarpoolApplication, useCarpoolApplicationContactsQuery, useCarpoolApplicationEvents } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const id = computed(() => String(route.params.id ?? ''))
const ownerMode = computed(() => route.path.startsWith('/merchant/'))
const { data: application, isLoading } = useCarpoolApplication(id)
const { data: events } = useCarpoolApplicationEvents(id)
const { data: contactSnapshot } = useCarpoolApplicationContactsQuery(id)
const actionBusy = ref(false)
const rejectPanelOpen = ref(false)
const rejectReasonCode = ref('seat_full')
const rejectReasonText = ref('')
const builtInProductIcons = new Map<string, string>()
const productIconSrc = computed(() => application.value ? getProductCategoryIconSrc(getProductCategory(application.value.snapshot.productName), builtInProductIcons) : null)
const counterpartyUsername = computed(() => application.value ? (ownerMode.value ? application.value.applicantUsername : application.value.ownerUsername) : '')
const counterpartyReputation = computed(() => application.value ? (ownerMode.value ? application.value.buyerReputation : application.value.snapshot.ownerReputation) : null)

const rideProgressSteps = [
  { step: 1, label: '提交申请', description: '申请条件已记录' },
  { step: 2, label: '车主确认上车', description: '确认后直接成为有效成员' },
  { step: 3, label: '成员关系', description: '持续到退出或被移除' },
]
const currentRideStep = computed(() => {
  const status = application.value?.status
  if (['active', 'cancelled_by_buyer', 'cancelled_by_owner', 'disputed'].includes(status ?? '')) return 3
  if (status === 'rejected') return 2
  return 1
})
const canOwnerProcess = computed(() => ownerMode.value && application.value?.status === 'pending_owner')
const canBuyerConfirmConditions = computed(() => !ownerMode.value && application.value?.status === 'pending_owner')
const canBuyerCancelApplication = computed(() => !ownerMode.value && application.value?.status === 'pending_owner')
const canBuyerLeaveMembership = computed(() => !ownerMode.value && application.value?.status === 'active')
const canRemoveMember = computed(() => ownerMode.value && application.value?.status === 'active')
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
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ['carpool-application'] }),
    queryClient.invalidateQueries({ queryKey: ['carpool-application-events'] }),
    queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] }),
    queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] }),
    queryClient.invalidateQueries({ queryKey: ['carpools'] }),
    queryClient.invalidateQueries({ queryKey: ['order-contacts', 'carpool-application'] }),
    queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] }),
    queryClient.invalidateQueries({ queryKey: ['notifications'] }),
    queryClient.invalidateQueries({ queryKey: ['navigation-badges'] }),
  ])
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

function acceptApplication() {
  if (application.value) runAction(() => acceptCarpoolApplication(application.value!.id), '已确认上车，成员关系已生效。')
}

function confirmConditions() {
  if (application.value) runAction(() => confirmCarpoolApplicationConditions(application.value!.id), '已确认最新版车源条件。')
}

function rejectApplication() {
  if (!application.value) return
  if (rejectReasonCode.value === 'other' && !rejectReasonText.value.trim()) return toast.warning('选择其他原因时必须填写补充说明。')
  runAction(() => rejectCarpoolApplication(application.value!.id, rejectReason.value), '已拒绝申请，并记录原因。')
}

function cancelApplication() {
  if (application.value) runAction(() => cancelCarpoolApplication(application.value!.id, '买家主动撤回申请'), '已撤回上车申请。')
}

function leaveMembership() {
  if (application.value) runAction(() => leaveCarpoolMembership(application.value!.id, '买家主动退出成员关系'), '已退出拼车。')
}

function removeMember() {
  if (application.value) runAction(() => removeCarpoolMember(application.value!.id, '车主主动移除成员关系'), '已移除成员关系。')
}

function requestManualIntervention() {
  if (!application.value) return
  const description = window.prompt('请填写 4-1000 字脱敏说明。平台只记录处理状态和公开摘要，不追回付款、不托管、不担保站外履约。')
  if (!description?.trim()) return
  runAction(async () => {
    await createManualInterventionReport({ targetType: 'carpool_application', targetId: application.value!.id, targetLabel: application.value!.snapshot.productName, reasonCode: 'seat_rule_dispute', title: '举报 / 申请人工介入：规则或席位争议', description: description.trim() })
    trackAnalytics('report_submit', { source_route: String(route.name ?? 'unknown'), entity_type: 'carpool_application', reason_code: 'seat_rule_dispute' })
  }, '已提交人工介入申请。')
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载上车申请…</div>
  <div v-else-if="!application" class="rounded-xl border border-border bg-card p-8"><h1 class="text-xl font-semibold">未找到上车申请</h1><p class="mt-2 text-sm text-muted-foreground">该申请不存在或暂不可见。</p><Button class="mt-5" variant="outline" @click="router.push('/my/rides')">返回我的上车</Button></div>
  <div v-else class="ride-order-detail-reference space-y-5">
    <header class="ride-order-detail-heading"><div class="text-xs text-muted-foreground">我的交易　/　我的上车　/　上车申请详情</div><div class="mt-3 flex items-start gap-4"><span class="ride-order-product-icon"><img v-if="productIconSrc" :src="productIconSrc" alt="" /><CarFront v-else /></span><div class="min-w-0"><div v-auto-animate="functionalMotion" class="flex flex-wrap items-center gap-2"><h1>{{ application.snapshot.productName }}</h1><Badge :key="application.status">{{ getCarpoolApplicationStatusLabel(application.status) }}</Badge><Badge variant="secondary">{{ ownerMode ? '车主视角' : '买家视角' }}</Badge></div><p>{{ application.snapshot.regionName }} · 以买家确认的条件快照为准。</p><div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground"><ShortId :value="application.id" prefix="RIDE" copyable /><span>更新于 <LocalTime :value="application.updatedAt" /></span></div></div></div></header>

    <div class="ride-order-detail-layout">
      <main class="min-w-0 space-y-4">
        <Card class="ride-order-progress p-5"><div class="flex items-center justify-between gap-4"><div><h2>上车进度</h2><p>{{ getCarpoolApplicationNextAction(application, ownerMode ? 'owner' : 'buyer') }}</p></div><Badge variant="secondary">第 {{ currentRideStep }} / 3 步</Badge></div><div v-auto-animate="functionalMotion" class="ride-order-stepper"><div v-for="item in rideProgressSteps" :key="item.step" class="c2c-motion-state" :class="{ 'is-done': item.step < currentRideStep, 'is-current': item.step === currentRideStep }"><span>{{ item.step < currentRideStep ? '✓' : item.step }}</span><div><strong>{{ item.label }}</strong><small>{{ item.description }}</small></div></div></div></Card>
        <Card class="ride-order-summary p-0"><dl><div><CircleDollarSign /><dt>月费快照</dt><dd>¥{{ application.snapshot.monthlyPriceCny }}</dd><small>{{ application.snapshot.priceLabel }}</small></div><div><UsersRound /><dt>申请席位</dt><dd>{{ application.seatsRequested }} 席</dd><small>不设预留窗口</small></div><div><UsersRound /><dt>成员状态</dt><dd>{{ application.startedAt ? '已生效' : '未生效' }}</dd><small>车主确认后立即生效</small></div><div><Clock3 /><dt>当前状态</dt><dd>{{ ownerMode ? '车主视角' : '买家视角' }}</dd><small>{{ getCarpoolApplicationStatusLabel(application.status) }}</small></div></dl></Card>
        <Card class="ride-order-snapshot p-5"><div class="ride-order-section-title"><FileText /><div><h2>申请条件快照</h2><p>成员加入后继续保留本次确认的条件</p></div></div><dl class="ride-order-detail-list"><div><dt>产品与地区</dt><dd>{{ application.snapshot.productName }} · {{ application.snapshot.regionName }}</dd></div><div><dt>每日最大花费额度</dt><dd>{{ application.snapshot.dailyQuotaAmount ? `$${application.snapshot.dailyQuotaAmount}` : '不限' }}</dd></div><div><dt>每周最大花费额度</dt><dd>{{ application.snapshot.weeklyQuotaAmount ? `$${application.snapshot.weeklyQuotaAmount}` : '不限' }}</dd></div><div><dt>开通方式</dt><dd>{{ application.snapshot.openingChannelName }}</dd></div><div><dt>上游订阅支付方式</dt><dd>{{ application.snapshot.paymentMethodNames.join(' / ') }}</dd></div><div><dt>车主与申请人</dt><dd>{{ application.ownerUsername }} / {{ application.applicantUsername }}</dd></div><div><dt>访问安排</dt><dd>{{ application.snapshot.accessArrangementNote ?? '成员邀请、费用分摊或站外访问安排' }}</dd></div><div><dt>条件版本</dt><dd>{{ application.conditionsVersionSnapshot ?? application.snapshot.rulesVersion }}</dd></div></dl><div class="mt-4 rounded-lg border bg-muted/30 p-3 text-sm leading-6">{{ application.snapshot.rulesText }}</div></Card>
        <section v-if="contactSnapshot" class="ride-order-contact-section"><div class="ride-order-section-title px-1"><MessageCircle /><div><h2>成员联系方式</h2><p>成员关系有效期间向双方展示联系快照</p></div></div><OrderContactCard :snapshot="contactSnapshot" :title="ownerMode ? '联系成员' : '联系车主'" :side="ownerMode ? 'buyer' : 'seller'" :show-contacted-action="false" /></section>
        <Card class="ride-order-timeline p-5"><div class="ride-order-section-title"><Clock3 /><div><h2>事件时间线</h2><p>申请、加入、退出与治理动作记录</p></div></div><div v-auto-animate="functionalMotion" class="mt-5"><div v-for="event in events ?? []" :key="event.id" class="ride-order-event"><span></span><div><div class="flex flex-wrap justify-between gap-2"><strong>{{ event.actorLabel }} · {{ event.type }}</strong><small><LocalTime :value="event.createdAt" /></small></div><p>{{ event.fromStatus ? getCarpoolApplicationStatusLabel(event.fromStatus) : '创建' }}<span v-if="event.toStatus"> → {{ getCarpoolApplicationStatusLabel(event.toStatus) }}</span><span v-if="event.note"> · {{ event.note }}</span></p></div></div></div></Card>
      </main>

      <aside class="ride-order-aside space-y-4">
        <Card class="ride-order-action-card p-5"><div class="text-xs text-muted-foreground">当前状态</div><h2>{{ getCarpoolApplicationNextAction(application, ownerMode ? 'owner' : 'buyer') }}</h2><p>{{ ownerMode ? '确认上车后会立即创建有效成员关系。' : '申请期间可确认最新版条件；车主确认后直接加入。' }}</p><div :key="application.status" class="mt-4 grid gap-2" v-auto-animate="functionalMotion"><Button v-if="canOwnerProcess" :disabled="actionBusy" @click="acceptApplication"><CheckCircle2 class="h-4 w-4" />确认上车</Button><Button v-if="canBuyerConfirmConditions" variant="outline" :disabled="actionBusy" @click="confirmConditions"><CheckCircle2 class="h-4 w-4" />确认最新版条件</Button><Button v-if="canOwnerProcess" variant="outline" :disabled="actionBusy" @click="rejectPanelOpen = !rejectPanelOpen"><XCircle class="h-4 w-4" />拒绝申请</Button><Button v-if="!canOwnerProcess" variant="outline" :disabled="actionBusy" @click="requestManualIntervention"><Flag class="h-4 w-4" />申请人工介入</Button><Button v-if="canRemoveMember" variant="outline" :disabled="actionBusy" @click="removeMember"><ShieldAlert class="h-4 w-4" />移除成员</Button><Button v-if="canBuyerCancelApplication" variant="outline" :disabled="actionBusy" @click="cancelApplication"><RotateCcw class="h-4 w-4" />撤回申请</Button><Button v-if="canBuyerLeaveMembership" variant="outline" :disabled="actionBusy" @click="leaveMembership"><RotateCcw class="h-4 w-4" />退出拼车</Button></div><div v-if="canOwnerProcess && rejectPanelOpen" class="mt-4 space-y-3 border-t border-border pt-4"><label class="space-y-2 text-sm"><span class="font-medium">拒绝原因</span><Select v-model="rejectReasonCode"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem v-for="item in rejectReasonOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem></SelectContent></Select></label><label class="space-y-2 text-sm"><span class="font-medium">补充说明</span><Textarea v-model="rejectReasonText" rows="2" placeholder="说明原因，不要填写联系方式或敏感凭据。" /></label><Button class="w-full" variant="destructive" :disabled="actionBusy" @click="rejectApplication">确认拒绝</Button></div></Card>
        <Card class="p-5"><div class="ride-order-section-title"><UsersRound /><div><h2>交易对手资料</h2><p>拼车不计入公开评价分</p></div></div><div class="mt-4 flex items-center justify-between gap-3 text-sm"><span class="text-muted-foreground">{{ ownerMode ? '申请人' : '车主' }}</span><strong>{{ counterpartyUsername }}</strong></div><div class="mt-4 border-t border-border pt-4"><ReputationSummaryCard :summary="counterpartyReputation" compact :framed="false" :show-source-author-verification="false" /></div><RouterLink :to="`/u/${counterpartyUsername}`"><Button class="mt-4 w-full" variant="outline">查看公开主页</Button></RouterLink></Card>
        <Card class="p-5"><div class="ride-order-section-title"><ShieldAlert /><div><h2>平台边界</h2><p>交易与联系规则</p></div></div><ul class="mt-4 space-y-2 text-xs leading-5 text-muted-foreground"><li>平台记录申请和成员状态，不代收或托管拼车费用。</li><li>拼车不产生公开评价，不影响公开信誉分。</li><li>举报、纠纷、审计与账号治理仍然保留。</li></ul></Card>
      </aside>
    </div>
  </div>
</template>
