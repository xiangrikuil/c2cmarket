<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { ExternalLink, Info, MessageCircle } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import SourceBadges from '@/components/market/SourceBadges.vue'
import { createCarpoolApplication, getCarpoolAccessArrangementLabel, getCarpoolApplyDisabledReason, isHighRiskSubscriptionCarpool } from '@/lib/api'
import { fullCapacityTooltip, getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { formatMonthlyQuota, quotaFieldLabel } from '@/lib/quota'
import { useCarpool, useFavoriteStatus, useMyCarpoolApplications, useToggleFavoriteMutation } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const route = useRoute()
const queryClient = useQueryClient()
const id = computed(() => String(route.params.id ?? ''))
const { data: carpool, isLoading } = useCarpool(id)
const { data: myApplications } = useMyCarpoolApplications({ sort: 'default_buyer' })
const { data: favoriteStatus } = useFavoriteStatus('carpool', id)
const toggleFavoriteMutation = useToggleFavoriteMutation()
const applyDialogOpen = ref(false)
const rulesAccepted = ref(false)
const applyBusy = ref(false)
const pricing = computed(() => carpool.value ? getPricingDisplay(carpool.value) : null)
const quotaText = computed(() => carpool.value ? formatMonthlyQuota(carpool.value) : '额度待补充')
const quotaLabel = computed(() => carpool.value ? quotaFieldLabel(carpool.value) : '每月额度')
const seatSummary = computed(() => carpool.value?.seatSummary ?? null)
const favorited = computed(() => Boolean(favoriteStatus.value))
const ongoingApplication = computed(() => (myApplications.value ?? []).find(item => {
  return item.carpoolId === id.value && !['completed', 'rejected', 'cancelled_by_buyer', 'cancelled_by_owner', 'expired'].includes(item.status)
}))

const applyDisabledReason = computed(() => {
  if (!carpool.value) return '车源不存在'
  return getCarpoolApplyDisabledReason(carpool.value, seatSummary.value, Boolean(ongoingApplication.value))
})

const totalSeats = computed(() => seatSummary.value?.totalSeats ?? carpool.value?.maxMembers ?? 0)
const activeSeats = computed(() => seatSummary.value?.activeMemberCount ?? carpool.value?.currentConfirmedMembers ?? 0)
const reservedSeats = computed(() => seatSummary.value?.reservedSeatCount ?? 0)
const availableSeats = computed(() => seatSummary.value?.availableSeats ?? (carpool.value ? getRemainingSeats(carpool.value) : 0))
const occupiedPercent = computed(() => getSeatPercent(activeSeats.value, totalSeats.value))
const reservedPercent = computed(() => getSeatPercent(reservedSeats.value, totalSeats.value))
const availablePercent = computed(() => getSeatPercent(availableSeats.value, totalSeats.value))
const applyStatusText = computed(() => applyDisabledReason.value || '可申请上车')
const statusToneClass = computed(() => {
  if (!carpool.value) return 'border-border bg-muted/30 text-muted-foreground'
  if (!applyDisabledReason.value) return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  if (carpool.value.status === '审核中' || carpool.value.accessArrangementMode === 'not_allowed' || (isHighRiskSubscriptionCarpool(carpool.value) && !carpool.value.riskAcknowledged)) return 'border-amber-200 bg-amber-50 text-amber-800'
  return 'border-border bg-muted/30 text-muted-foreground'
})

function getSeatPercent(value: number, total: number) {
  if (!Number.isFinite(value) || !Number.isFinite(total) || total <= 0) return '0%'
  return `${Math.max(0, Math.min(100, Math.round((value / total) * 100)))}%`
}

function toggleFavorite() {
  if (!carpool.value) return
  toggleFavoriteMutation.mutate({ targetType: 'carpool', targetId: carpool.value.id }, {
    onSuccess(data) {
      toast.success(data.favorited ? '已收藏该车源。' : '已取消收藏。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '操作失败')
    },
  })
}

async function applyToJoin() {
  if (!carpool.value) return
  if (!rulesAccepted.value) {
    toast.warning('请先确认平台边界和风险提示。')
    return
  }
  applyBusy.value = true
  try {
    const application = await createCarpoolApplication(carpool.value.id, { rulesAccepted: rulesAccepted.value })
    await queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] })
    await queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] })
    await queryClient.invalidateQueries({ queryKey: ['carpools'] })
    await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    await queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
    applyDialogOpen.value = false
    toast.success(`申请已提交，等待车主处理：${application.id}`)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '申请失败')
  } finally {
    applyBusy.value = false
  }
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载车源详情...</div>
  <div v-else-if="!carpool" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到车源</h1>
    <p class="mt-2 text-sm text-muted-foreground">该车源 ID 不存在，可能已下架或暂不可见。</p>
    <RouterLink to="/carpools"><Button class="mt-5" variant="outline">返回订阅拼车</Button></RouterLink>
  </div>
  <div v-else>
    <div class="mb-5 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <p class="text-sm text-muted-foreground">AI 分类 / {{ carpool.product }} / {{ carpool.region }}</p>
          <Badge :variant="carpool.status === '可上车' ? 'default' : 'secondary'">{{ carpool.status }}</Badge>
        </div>
        <h1 class="mt-2 text-3xl font-semibold tracking-tight">{{ carpool.product }} 拼车</h1>
        <p class="mt-2 max-w-3xl text-sm text-muted-foreground">月付展示，支持个人订阅费用分摊、成员邀请或其他站外安排；不允许共用密码、token、Session 或 Cookie。</p>
      </div>
      <div class="grid gap-2 sm:flex md:pt-1">
        <Button class="w-full sm:w-auto" variant="outline" :disabled="toggleFavoriteMutation.isPending.value" @click="toggleFavorite">{{ favorited ? '已收藏' : '收藏' }}</Button>
        <Button class="w-full sm:w-auto" :variant="applyDisabledReason ? 'secondary' : 'default'" :disabled="Boolean(applyDisabledReason)" @click="applyDialogOpen = true">
          <MessageCircle class="h-4 w-4" />{{ applyDisabledReason || '申请上车' }}
        </Button>
      </div>
    </div>

    <Card class="overflow-hidden p-0">
      <div class="grid lg:grid-cols-[minmax(0,1fr)_380px]">
        <section class="p-5 lg:border-r lg:border-border lg:p-6">
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{{ pricing?.modeLabel }}</Badge>
            <Badge variant="outline">{{ pricing?.primaryLabel }}</Badge>
            <Badge variant="outline">{{ carpool.warranty }} · 平台不担保</Badge>
          </div>
          <div class="mt-5 flex flex-col gap-5 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <div class="text-sm text-muted-foreground">当前申请月费参考</div>
              <div class="mt-2 text-[44px] font-semibold leading-none tracking-tight">¥{{ pricing?.primaryPrice }}</div>
              <div class="mt-1 text-sm text-muted-foreground">/ 月 · 最终账期、起止日和付款方式由双方站外确认</div>
            </div>
            <div class="grid gap-2 text-sm sm:min-w-[220px]">
              <div v-if="pricing?.secondaryPrice" class="flex justify-between gap-4 rounded-md bg-muted/40 px-3 py-2" :title="fullCapacityTooltip">
                <span class="inline-flex items-center gap-1 text-muted-foreground">{{ pricing.detailSecondaryLabel }}<Info class="h-3.5 w-3.5" /></span>
                <span class="font-medium">¥{{ pricing.secondaryPrice }}/月</span>
              </div>
              <div v-if="pricing?.nextTierPrice" class="flex justify-between gap-4 rounded-md bg-muted/40 px-3 py-2">
                <span class="text-muted-foreground">{{ pricing.nextTierLabel }}</span>
                <span class="font-medium">¥{{ pricing.nextTierPrice }}/月</span>
              </div>
              <div class="flex justify-between gap-4 rounded-md bg-muted/40 px-3 py-2">
                <span class="text-muted-foreground">价格说明</span>
                <span class="font-medium">{{ pricing?.note }}</span>
              </div>
              <div class="flex justify-between gap-4 rounded-md bg-muted/40 px-3 py-2">
                <span class="text-muted-foreground">倍率</span>
                <span class="font-medium">{{ carpool.serviceMultiplier ?? '-' }}x</span>
              </div>
              <div class="flex justify-between gap-4 rounded-md bg-muted/40 px-3 py-2">
                <span class="text-muted-foreground">{{ quotaLabel }}</span>
                <span class="font-medium">{{ quotaText }}</span>
              </div>
            </div>
          </div>

          <div class="mt-5 grid gap-3 text-sm md:grid-cols-3">
            <div class="rounded-md border border-border bg-background px-3 py-2">
              <div class="text-muted-foreground">最后确认</div>
              <div class="mt-1 font-medium">{{ carpool.confirmedAt }}</div>
            </div>
            <div class="rounded-md border border-border bg-background px-3 py-2">
              <div class="text-muted-foreground">开通方式</div>
              <div class="mt-1 font-medium">{{ carpool.openingMethod }} · {{ carpool.region }}</div>
            </div>
            <div class="rounded-md border border-border bg-background px-3 py-2">
              <div class="text-muted-foreground">上车状态</div>
              <div class="mt-1 font-medium">{{ applyStatusText }}</div>
            </div>
          </div>
        </section>

        <aside class="border-t border-border p-5 lg:border-t-0 lg:p-6">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-sm text-muted-foreground">名额进度</div>
              <div class="mt-2 text-3xl font-semibold">{{ availableSeats }} <span class="text-base font-medium text-muted-foreground">/ {{ totalSeats }} 可申请</span></div>
            </div>
            <span class="rounded-full border px-2.5 py-1 text-xs font-medium" :class="statusToneClass">{{ applyStatusText }}</span>
          </div>
          <div class="mt-5 h-3 overflow-hidden rounded-full bg-muted">
            <div class="flex h-full w-full">
              <div v-if="activeSeats" class="h-full bg-primary" :style="{ width: occupiedPercent }" />
              <div v-if="reservedSeats" class="h-full bg-amber-400" :style="{ width: reservedPercent }" />
              <div v-if="availableSeats" class="h-full bg-emerald-500" :style="{ width: availablePercent }" />
            </div>
          </div>
          <div class="mt-3 grid grid-cols-3 gap-2 text-center text-xs">
            <div class="rounded-md bg-muted/40 px-2 py-2"><div class="font-semibold">{{ activeSeats }}</div><div class="text-muted-foreground">已上车</div></div>
            <div class="rounded-md bg-muted/40 px-2 py-2"><div class="font-semibold">{{ reservedSeats }}</div><div class="text-muted-foreground">预留中</div></div>
            <div class="rounded-md bg-muted/40 px-2 py-2"><div class="font-semibold">{{ availableSeats }}</div><div class="text-muted-foreground">可申请</div></div>
          </div>
          <Button class="mt-5 w-full" :variant="applyDisabledReason ? 'secondary' : 'default'" :disabled="Boolean(applyDisabledReason)" @click="applyDialogOpen = true">
            <MessageCircle class="h-4 w-4" />{{ applyDisabledReason || '申请上车' }}
          </Button>
          <p class="mt-3 text-xs leading-5 text-muted-foreground">车主接受前不占用正式名额；审核中、风险未确认或需要共享凭据的车源不可申请。</p>
        </aside>
      </div>
    </Card>

    <Card class="mt-4 p-5">
      <div class="flex items-start gap-3">
        <span class="mt-0.5 grid h-6 w-6 place-items-center rounded-full bg-signal-soft text-xs font-semibold">!</span>
        <div>
          <p class="font-medium">该车源按月付展示，加入前请确认付款周期、剩余名额、退出规则和退款条件。</p>
          <p class="mt-1 text-sm text-muted-foreground">平台不托管支付、不保存账号或 Token，不鼓励共用账号、共用密码或转交会话凭据，也不担保车主承诺或代赔。</p>
        </div>
      </div>
    </Card>
    <div class="mt-4 grid gap-6 lg:grid-cols-[1.35fr_0.8fr]">
      <Card class="p-6">
        <h2 class="text-lg font-semibold">车源重点</h2>
        <div class="mt-6 grid gap-4 text-sm">
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">访问安排</span><span>{{ getCarpoolAccessArrangementLabel(carpool.accessArrangementMode) }} · {{ carpool.accessArrangementNote ?? '站外确认访问安排' }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">计价方式</span><span>{{ pricing?.modeLabel }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">{{ pricing?.primaryLabel }}</span><span>¥{{ pricing?.primaryPrice }}/月</span></div>
          <div v-if="pricing?.secondaryPrice" class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between sm:gap-4">
            <span class="inline-flex items-center gap-1 text-muted-foreground" :title="fullCapacityTooltip">
              {{ pricing.detailSecondaryLabel }}
              <Info class="h-3.5 w-3.5" />
            </span>
            <span>¥{{ pricing.secondaryPrice }}/月</span>
          </div>
          <div v-if="pricing?.nextTierPrice" class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between">
            <span class="text-muted-foreground">{{ pricing.nextTierLabel }}</span>
            <span>¥{{ pricing.nextTierPrice }}/月</span>
          </div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">价格说明</span><span>{{ pricing?.note }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">倍率</span><span>{{ carpool.serviceMultiplier ?? '-' }}x</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">{{ quotaLabel }}</span><span>{{ quotaText }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">开通区</span><span>{{ carpool.region }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">开通方式</span><span>{{ carpool.openingMethod }}</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">车主承诺</span><span>{{ carpool.warranty }} · 平台不担保、不代赔</span></div>
          <div class="grid gap-1 border-b border-border pb-3 sm:flex sm:justify-between"><span class="text-muted-foreground">上车状态</span><span>{{ applyStatusText }}</span></div>
        </div>
        <div v-if="carpool.pricingMode === 'tiered' && carpool.pricingTiers?.length" class="mt-6">
          <h3 class="text-sm font-semibold">完整阶梯表</h3>
          <div class="mt-3 overflow-hidden rounded-md border border-border">
            <div v-for="tier in carpool.pricingTiers" :key="tier.memberCount" class="grid grid-cols-2 border-b border-border px-3 py-2 text-sm last:border-b-0">
              <span class="text-muted-foreground">达到 {{ tier.memberCount }} 人</span>
              <span class="text-right font-medium">¥{{ tier.price }}/月</span>
            </div>
          </div>
        </div>
      </Card>
      <Card class="p-6">
        <h2 class="text-lg font-semibold">车主信息</h2>
        <div class="mt-6 space-y-4 text-sm">
          <div class="flex justify-between"><span class="text-muted-foreground">车主</span><span>linux.do @{{ carpool.owner }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">信任</span><span>信任等级{{ carpool.trustLevel }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">车主类型</span><span>{{ carpool.ownerType }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">原帖</span><Badge :variant="carpool.linuxdoBound ? 'default' : 'secondary'">{{ carpool.linuxdoBound ? '已绑定' : '待绑定' }}</Badge></div>
          <SourceBadges :badges="[carpool.linuxdoBound ? '原帖已绑定' : '待绑定原帖', '近期确认', getCarpoolAccessArrangementLabel(carpool.accessArrangementMode), isHighRiskSubscriptionCarpool(carpool) ? '风险已确认' : '普通风险']" />
          <Button class="w-full" variant="outline" @click="toast('当前车源暂未提供可打开的原帖链接。')"><ExternalLink class="h-4 w-4" />打开原帖</Button>
        </div>
      </Card>
    </div>

    <Card class="mt-4 p-5">
      <h2 class="font-semibold">申请前必须确认</h2>
      <div class="mt-4 grid gap-3 text-sm md:grid-cols-2">
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">账期与付款周期</div>
          <p class="mt-1 text-muted-foreground">月费仅为展示口径；实际账期、起止日和付款方式由双方站外确认。</p>
        </div>
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">退出与退款</div>
          <p class="mt-1 text-muted-foreground">退出提前量、已用天数、剩余额度和退款条件必须在上车前确认。</p>
        </div>
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">访问安排方式</div>
          <p class="mt-1 text-muted-foreground">可以是个人订阅费用分摊、成员邀请或车主管理访问；不得要求共享密码、Session、Cookie、token 或其他登录态。</p>
        </div>
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">凭据安全规则</div>
          <p class="mt-1 text-muted-foreground">不得在平台填写、粘贴或上传账号密码、token、session、恢复码或登录态。</p>
        </div>
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">不可上架情形</div>
          <p class="mt-1 text-muted-foreground">如上车过程必须共用账号、共用密码或转交会话凭据，应拒绝上架或下架处理。</p>
        </div>
        <div class="rounded-md border border-border p-3">
          <div class="font-medium">争议处理</div>
          <p class="mt-1 text-muted-foreground">平台仅记录脱敏状态和评价，不查看完整联系方式或任何账号凭据内容。</p>
        </div>
      </div>
    </Card>

    <div v-if="applyDialogOpen" class="fixed inset-0 z-50 grid place-items-center bg-black/35 p-4" role="dialog" aria-modal="true">
      <Card class="w-full max-w-lg p-0">
        <div class="border-b border-border p-4">
          <h2 class="text-lg font-semibold">申请上车</h2>
          <p class="mt-1 text-sm text-muted-foreground">提交后等待车主接受；接受前不占用名额。</p>
        </div>
        <div class="space-y-4 p-4 text-sm">
          <dl class="grid gap-3 sm:grid-cols-2">
            <div><dt class="text-muted-foreground">车源</dt><dd class="font-medium">{{ carpool.product }}</dd></div>
            <div><dt class="text-muted-foreground">开通区</dt><dd>{{ carpool.region }}</dd></div>
            <div><dt class="text-muted-foreground">月费快照</dt><dd>¥{{ pricing?.primaryPrice }}/月 · {{ pricing?.primaryLabel }}</dd></div>
            <div><dt class="text-muted-foreground">申请名额</dt><dd>1 人</dd></div>
            <div><dt class="text-muted-foreground">车主</dt><dd>{{ carpool.owner }} · 信任等级{{ carpool.trustLevel }}</dd></div>
            <div><dt class="text-muted-foreground">可申请</dt><dd>{{ availableSeats }} / {{ totalSeats }} 位</dd></div>
          </dl>
          <label class="flex items-start gap-2 rounded-md border border-border p-3">
            <input v-model="rulesAccepted" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
            <span>我理解平台只记录上车意向和状态，不托管支付、不保存账号或 token、不担保或代赔；如需要共享密码、Session、Cookie 或 token，应放弃申请。</span>
          </label>
        </div>
        <div class="flex justify-end gap-2 border-t border-border p-4">
          <Button variant="outline" @click="applyDialogOpen = false">取消</Button>
          <Button :disabled="applyBusy || !rulesAccepted" @click="applyToJoin">提交申请</Button>
        </div>
      </Card>
    </div>
  </div>
</template>
