<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import QRCode from 'qrcode'
import {
  Check,
  Clock3,
  Copy,
  Download,
  Gift,
  Loader2,
  Megaphone,
  Send,
  TicketCheck,
  UserPlus,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { PromotionCoupon, PromotionCouponFilter, ReferralStatus } from '@/lib/promotionRewardBackend'
import { defaultPromotionCouponQuery } from '@/lib/promotionRewardBackend'
import { trackAnalytics } from '@/lib/analytics'
import {
  useApplyPromotionCouponMutation,
  useMyPromotionCoupons,
  useMyReferralSummary,
  usePromotionRewardPublicConfig,
} from '@/queries/usePromotionRewardQueries'
import { useMyApiServices } from '@/queries/useMarketQueries'

const publicConfigQuery = usePromotionRewardPublicConfig()
const programEnabled = computed(() => publicConfigQuery.data.value?.programEnabled === true)
const referralQuery = useMyReferralSummary(programEnabled)
const couponStatus = ref<PromotionCouponFilter>('all')
const couponPage = ref(1)
const couponQueryInput = computed(() => ({
  ...defaultPromotionCouponQuery,
  page: couponPage.value,
  status: couponStatus.value,
}))
const couponQuery = useMyPromotionCoupons(couponQueryInput)
const ownerServicesQuery = useMyApiServices('all', programEnabled)
const applyMutation = useApplyPromotionCouponMutation()
const applyingCoupon = ref<PromotionCoupon | null>(null)
const selectedServiceId = ref('')
const posterLoading = ref(false)

watch(couponStatus, () => {
  couponPage.value = 1
})

const referral = computed(() => referralQuery.data.value)
const inviteLink = computed(() => {
  const code = referral.value?.code
  if (!code) return ''
  const origin = import.meta.client ? window.location.origin : 'https://c2cmarket.shop'
  return `${origin}/login?ref=${encodeURIComponent(code)}`
})
const couponPageData = computed(() => couponQuery.data.value)
const orderableServices = computed(() => (ownerServicesQuery.data.value ?? []).filter(service => service.publiclyOrderable))
const statistics = computed(() => {
  const value = referral.value?.statistics
  return [
    { label: '已邀请', value: value?.invitedCount ?? 0 },
    { label: '已达成首发', value: value?.qualifiedCount ?? 0 },
    { label: '已发放奖励', value: value?.rewardedCount ?? 0 },
    { label: '本月剩余额度', value: value?.inviterRewardsRemaining ?? 0 },
  ]
})

const couponStatusOptions: Array<{ value: PromotionCouponFilter, label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'available', label: '可使用' },
  { value: 'pending', label: '待生效' },
  { value: 'used', label: '已使用' },
  { value: 'expired', label: '已过期' },
  { value: 'revoked', label: '已撤销' },
]

const couponStatusLabels: Record<PromotionCoupon['status'], string> = {
  pending: '待生效',
  available: '可使用',
  used: '已使用',
  expired: '已过期',
  revoked: '已撤销',
}

const couponSourceLabels: Record<PromotionCoupon['sourceType'], string> = {
  welcome_first_api_service: '新人首个 API 服务',
  referral_inviter: '邀请人奖励',
  referral_invitee: '受邀人奖励',
  admin_grant: '管理员发放',
}

const referralStatusLabels: Record<ReferralStatus, string> = {
  bound: '已绑定',
  qualified: '已达成',
  rewarded: '已发奖',
  rejected: '未通过',
  revoked: '已撤销',
}

function couponTone(status: PromotionCoupon['status']) {
  if (status === 'available') return 'success' as const
  if (status === 'pending') return 'waiting' as const
  if (status === 'used') return 'complete' as const
  if (status === 'revoked') return 'risk' as const
  return 'neutral' as const
}

function referralTone(status: ReferralStatus) {
  if (status === 'rewarded') return 'success' as const
  if (status === 'qualified') return 'brand' as const
  if (status === 'bound') return 'waiting' as const
  if (status === 'revoked' || status === 'rejected') return 'risk' as const
  return 'neutral' as const
}

async function copyText(value: string, successMessage: string, action: 'copy_code' | 'copy_link') {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    toast.success(successMessage)
    trackAnalytics('promotion_benefit_action', { action, source_route: '/my/promotion-benefits' })
  } catch {
    toast.error('复制失败，请检查浏览器权限。')
  }
}

function drawPosterBackground(context: CanvasRenderingContext2D) {
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, 1080, 1440)
  context.fillStyle = '#0f172a'
  context.fillRect(0, 0, 1080, 190)
  context.fillStyle = '#2563eb'
  context.fillRect(72, 248, 12, 250)
}

async function downloadPoster() {
  if (!inviteLink.value || !referral.value) return
  posterLoading.value = true
  try {
    const qrDataURL = await QRCode.toDataURL(inviteLink.value, {
      width: 420,
      margin: 1,
      color: { dark: '#0f172a', light: '#ffffff' },
      errorCorrectionLevel: 'M',
    })
    const qrImage = new Image()
    qrImage.src = qrDataURL
    await qrImage.decode()

    const canvas = document.createElement('canvas')
    canvas.width = 1080
    canvas.height = 1440
    const context = canvas.getContext('2d')
    if (!context) throw new Error('浏览器不支持海报生成。')

    drawPosterBackground(context)
    context.fillStyle = '#ffffff'
    context.font = '700 58px sans-serif'
    context.fillText('C2CMarket', 72, 112)
    context.fillStyle = '#0f172a'
    context.font = '700 58px sans-serif'
    context.fillText('邀请好友发布 API 服务', 112, 320)
    context.font = '400 34px sans-serif'
    context.fillStyle = '#475569'
    context.fillText(`达成首个有效服务后，双方各得 ${referral.value.campaign.promotionDurationHours} 小时推广权益`, 112, 390)
    context.fillText('推广权益进入站内轮播，不承诺固定位置或曝光量', 112, 452)

    context.fillStyle = '#f8fafc'
    context.fillRect(112, 555, 856, 650)
    context.drawImage(qrImage, 330, 640, 420, 420)
    context.fillStyle = '#64748b'
    context.font = '400 28px sans-serif'
    context.textAlign = 'center'
    context.fillText('邀请码', 540, 1120)
    context.fillStyle = '#0f172a'
    context.font = '700 48px monospace'
    context.fillText(referral.value.code, 540, 1182)
    context.textAlign = 'left'
    context.fillStyle = '#64748b'
    context.font = '400 25px sans-serif'
    context.fillText('C2CMarket · 信息撮合与风险治理平台', 112, 1330)

    const link = document.createElement('a')
    link.download = `c2cmarket-invite-${referral.value.code}.png`
    link.href = canvas.toDataURL('image/png')
    link.click()
    toast.success('邀请海报已生成。')
    trackAnalytics('promotion_benefit_action', { action: 'poster_download', source_route: '/my/promotion-benefits' })
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '海报生成失败。')
  } finally {
    posterLoading.value = false
  }
}

function openApplyDialog(coupon: PromotionCoupon) {
  applyingCoupon.value = coupon
  selectedServiceId.value = orderableServices.value[0]?.id ?? ''
}

function closeApplyDialog() {
  if (applyMutation.isPending.value) return
  applyingCoupon.value = null
  selectedServiceId.value = ''
}

function applyCoupon() {
  if (!applyingCoupon.value || !selectedServiceId.value) {
    toast.warning('请选择一个当前可接单的 API 服务。')
    return
  }
  applyMutation.mutate({
    couponId: applyingCoupon.value.id,
    apiServiceId: selectedServiceId.value,
  }, {
    onSuccess: coupon => {
      toast.success(`推广权益已生效 ${coupon.durationHours} 小时。`)
      trackAnalytics('promotion_benefit_action', { action: 'coupon_apply', source_route: '/my/promotion-benefits' })
      applyingCoupon.value = null
      selectedServiceId.value = ''
    },
    onError: error => toast.error(error instanceof Error ? error.message : '推广券使用失败。'),
  })
}
</script>

<template>
  <div class="min-w-0 space-y-6">
    <PageTitle title="推广权益" description="管理邀请奖励和 API 服务推广券。" />

    <div v-if="publicConfigQuery.isLoading.value" class="grid gap-4 md:grid-cols-2">
      <SkeletonBlock class="h-44" />
      <SkeletonBlock class="h-44" />
    </div>
    <ErrorState
      v-else-if="publicConfigQuery.isError.value"
      title="推广活动状态读取失败"
      description="暂时无法确认活动是否开放。"
      @retry="publicConfigQuery.refetch()"
    />
    <Alert v-else-if="!programEnabled">
      <Clock3 />
      <AlertTitle>推广活动暂未开放</AlertTitle>
      <AlertDescription>历史推广仍按原结束时间展示；活动开启后可在这里查看邀请和推广券。</AlertDescription>
    </Alert>

    <template v-else>
      <ErrorState
        v-if="referralQuery.isError.value"
        title="邀请权益读取失败"
        description="当前邀请关系和统计暂时不可用。"
        @retry="referralQuery.refetch()"
      />
      <div v-else-if="referralQuery.isLoading.value" class="grid gap-4 lg:grid-cols-2">
        <SkeletonBlock class="h-56" />
        <SkeletonBlock class="h-56" />
      </div>

      <template v-else-if="referral">
        <section class="grid gap-6 border-y border-border py-6 lg:grid-cols-[minmax(0,1.15fr)_minmax(320px,.85fr)]">
          <div class="min-w-0">
            <div class="flex items-center gap-2 text-sm font-semibold"><UserPlus class="h-4 w-4 text-primary" />邀请好友</div>
            <p class="mt-2 text-sm text-muted-foreground">好友发布首个符合条件的 API 服务后，双方各得 {{ referral.campaign.promotionDurationHours }} 小时推广权益。</p>
            <div class="mt-5 grid gap-3 sm:grid-cols-[1fr_auto]">
              <div class="min-w-0 rounded-md border border-border bg-muted/35 px-4 py-3">
                <div class="text-xs text-muted-foreground">邀请码</div>
                <div class="mt-1 break-all font-mono text-xl font-semibold tracking-normal">{{ referral.code }}</div>
              </div>
              <Button variant="outline" class="h-full min-h-14" title="复制邀请码" @click="copyText(referral.code, '邀请码已复制。', 'copy_code')"><Copy class="h-4 w-4" />复制代码</Button>
            </div>
            <div class="mt-3 flex min-w-0 flex-col gap-2 sm:flex-row">
              <Button class="min-w-0 flex-1" @click="copyText(inviteLink, '邀请链接已复制。', 'copy_link')"><Send class="h-4 w-4" />复制邀请链接</Button>
              <Button variant="outline" :disabled="posterLoading" @click="downloadPoster">
                <Loader2 v-if="posterLoading" class="h-4 w-4 animate-spin" />
                <Download v-else class="h-4 w-4" />
                下载海报
              </Button>
            </div>
          </div>

          <div class="min-w-0">
            <div class="flex items-center gap-2 text-sm font-semibold"><Gift class="h-4 w-4 text-primary" />本期规则</div>
            <dl class="mt-4 grid grid-cols-2 gap-x-4 gap-y-4 text-sm">
              <div><dt class="text-xs text-muted-foreground">奖励延迟</dt><dd class="mt-1 font-medium">{{ referral.campaign.rewardDelayHours }} 小时</dd></div>
              <div><dt class="text-xs text-muted-foreground">券有效期</dt><dd class="mt-1 font-medium">{{ referral.campaign.couponValidDays }} 天</dd></div>
              <div><dt class="text-xs text-muted-foreground">单次时长</dt><dd class="mt-1 font-medium">{{ referral.campaign.promotionDurationHours }} 小时</dd></div>
              <div><dt class="text-xs text-muted-foreground">邀请月上限</dt><dd class="mt-1 font-medium">{{ referral.campaign.inviterMonthlyLimit }} 份</dd></div>
            </dl>
            <p class="mt-5 whitespace-pre-line text-sm leading-6 text-muted-foreground">{{ referral.campaign.rulesText }}</p>
          </div>
        </section>

        <div class="grid gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="item in statistics" :key="item.label" class="bg-card px-4 py-3">
            <div class="text-xs text-muted-foreground">{{ item.label }}</div>
            <div class="mt-1 text-xl font-semibold">{{ item.value }}</div>
          </div>
        </div>

        <section aria-labelledby="referral-history-title">
          <div class="mb-3 flex items-center gap-2"><Check class="h-4 w-4 text-primary" /><h2 id="referral-history-title" class="text-base font-semibold">邀请记录</h2></div>
          <EmptyState v-if="referral.records.length === 0" title="暂无邀请记录" description="通过邀请链接注册的新用户会显示在这里。" />
          <div v-else class="divide-y divide-border border-y border-border">
            <div v-for="record in referral.records" :key="record.id" class="grid gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
              <div class="min-w-0">
                <div class="truncate font-medium">{{ record.inviteeDisplayName }}</div>
                <div class="mt-1 text-xs text-muted-foreground">绑定于 <LocalTime :value="record.boundAt" /></div>
              </div>
              <StatusBadge :status="record.status" :label="referralStatusLabels[record.status]" :tone="referralTone(record.status)" />
            </div>
          </div>
        </section>
      </template>

      <section aria-labelledby="coupon-wallet-title">
        <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-2"><TicketCheck class="h-4 w-4 text-primary" /><h2 id="coupon-wallet-title" class="text-base font-semibold">推广券</h2></div>
          <Tabs v-model="couponStatus" class="max-w-full overflow-x-auto">
            <TabsList class="w-max">
              <TabsTrigger v-for="item in couponStatusOptions" :key="item.value" :value="item.value">{{ item.label }}</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>

        <SkeletonBlock v-if="couponQuery.isLoading.value" class="h-64" />
        <ErrorState v-else-if="couponQuery.isError.value" title="推广券读取失败" description="券状态暂时无法加载。" @retry="couponQuery.refetch()" />
        <EmptyState v-else-if="couponPageData?.items.length === 0" title="当前没有推广券" description="达成新人首发、邀请任务或管理员发放后会显示在这里。" />
        <template v-else-if="couponPageData">
          <div class="grid gap-3 lg:grid-cols-2">
            <Card v-for="coupon in couponPageData.items" :key="coupon.id" class="rounded-md p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0"><div class="font-medium">{{ couponSourceLabels[coupon.sourceType] }}</div><div class="mt-1 text-xs text-muted-foreground">{{ coupon.durationHours }} 小时推广权益</div></div>
                <StatusBadge :status="coupon.status" :label="couponStatusLabels[coupon.status]" :tone="couponTone(coupon.status)" />
              </div>
              <dl class="mt-4 grid grid-cols-2 gap-3 text-xs">
                <div><dt class="text-muted-foreground">可用时间</dt><dd class="mt-1"><LocalTime :value="coupon.availableAt" /></dd></div>
                <div><dt class="text-muted-foreground">失效时间</dt><dd class="mt-1"><LocalTime :value="coupon.expiresAt" /></dd></div>
                <div v-if="coupon.promotionStartsAt"><dt class="text-muted-foreground">推广开始</dt><dd class="mt-1"><LocalTime :value="coupon.promotionStartsAt" /></dd></div>
                <div v-if="coupon.promotionEndsAt"><dt class="text-muted-foreground">推广结束</dt><dd class="mt-1"><LocalTime :value="coupon.promotionEndsAt" /></dd></div>
              </dl>
              <div class="mt-4 flex items-center justify-between gap-3 border-t border-border pt-3">
                <Badge variant="outline">API 服务</Badge>
                <Button v-if="coupon.status === 'available'" size="sm" @click="openApplyDialog(coupon)"><Megaphone class="h-4 w-4" />立即使用</Button>
                <span v-else-if="coupon.usedApiServiceTitle" class="min-w-0 truncate text-xs text-muted-foreground">{{ coupon.usedApiServiceTitle }}</span>
              </div>
            </Card>
          </div>
          <TablePagination
            v-if="couponPageData.pagination.totalPages > 1"
            :page="couponPageData.pagination.page"
            :page-count="couponPageData.pagination.totalPages"
            :total="couponPageData.pagination.totalItems"
            :start-item="(couponPageData.pagination.page - 1) * couponPageData.pagination.limit + 1"
            :end-item="Math.min(couponPageData.pagination.page * couponPageData.pagination.limit, couponPageData.pagination.totalItems)"
            @update:page="couponPage = $event"
          />
        </template>
      </section>
    </template>

    <Dialog :open="Boolean(applyingCoupon)" @update:open="open => { if (!open) closeApplyDialog() }">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>使用推广券</DialogTitle>
          <DialogDescription>推广权益生效后进入奖励轮播，不与现有推广叠加。</DialogDescription>
        </DialogHeader>
        <div class="grid gap-2 py-2">
          <label for="promotion-coupon-service" class="text-sm font-medium">API 服务</label>
          <Select v-model="selectedServiceId">
            <SelectTrigger id="promotion-coupon-service"><SelectValue placeholder="选择当前可接单的服务" /></SelectTrigger>
            <SelectContent><SelectItem v-for="service in orderableServices" :key="service.id" :value="service.id">{{ service.title }}</SelectItem></SelectContent>
          </Select>
          <Alert v-if="ownerServicesQuery.isError.value" variant="destructive"><AlertTitle>服务列表读取失败</AlertTitle><AlertDescription>暂时无法选择服务。</AlertDescription></Alert>
          <Alert v-else-if="!ownerServicesQuery.isLoading.value && orderableServices.length === 0"><AlertTitle>没有可使用的 API 服务</AlertTitle><AlertDescription>需要已审核、在线并可接单的 API 服务。</AlertDescription></Alert>
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="applyMutation.isPending.value" @click="closeApplyDialog">取消</Button>
          <Button :disabled="applyMutation.isPending.value || !selectedServiceId" @click="applyCoupon">
            <Loader2 v-if="applyMutation.isPending.value" class="h-4 w-4 animate-spin" />
            <Megaphone v-else class="h-4 w-4" />
            确认使用
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
