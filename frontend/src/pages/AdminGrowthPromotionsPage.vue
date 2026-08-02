<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  Ban,
  Gift,
  Loader2,
  RefreshCw,
  Save,
  Search,
  Settings2,
  TicketCheck,
  UserRoundSearch,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { TableCell } from '@/components/ui/table'
import { beijingDateTimeInputToISOString, formatBeijingDateTimeInput } from '@/lib/apiQuotaExpiration'
import {
  defaultAdminPromotionCouponQuery,
  defaultAdminReferralQuery,
  type AdminPromotionCoupon,
  type AdminPromotionCouponQuery,
  type AdminReferralQuery,
  type AdminReferralRecord,
  type PromotionCouponSource,
  type PromotionCouponStatusValue,
  type ReferralStatus,
} from '@/lib/promotionRewardBackend'
import {
  useAdminPromotionCoupons,
  useAdminPromotionRewardCampaign,
  useAdminReferrals,
  useGrantAdminPromotionCouponMutation,
  useRevokeAdminPromotionCouponMutation,
  useRevokeAdminReferralMutation,
  useUpdatePromotionRewardCampaignMutation,
} from '@/queries/usePromotionRewardQueries'

const activeTab = ref('configuration')
const campaignQuery = useAdminPromotionRewardCampaign()
const updateCampaignMutation = useUpdatePromotionRewardCampaignMutation()
const campaignDialogOpen = ref(false)
const campaignForm = reactive({
  programEnabled: false,
  welcomeEnabled: false,
  referralEnabled: false,
  startsAt: '',
  endsAt: '',
  promotionDurationHours: 24,
  couponValidDays: 30,
  rewardDelayHours: 72,
  inviterMonthlyLimit: 10,
  rulesText: '',
  reason: '',
})

watch(() => campaignQuery.data.value, campaign => {
  if (!campaign) return
  campaignForm.programEnabled = campaign.programEnabled
  campaignForm.welcomeEnabled = campaign.welcomeEnabled
  campaignForm.referralEnabled = campaign.referralEnabled
  campaignForm.startsAt = formatBeijingDateTimeInput(new Date(campaign.startsAt))
  campaignForm.endsAt = campaign.endsAt ? formatBeijingDateTimeInput(new Date(campaign.endsAt)) : ''
  campaignForm.promotionDurationHours = campaign.promotionDurationHours
  campaignForm.couponValidDays = campaign.couponValidDays
  campaignForm.rewardDelayHours = campaign.rewardDelayHours
  campaignForm.inviterMonthlyLimit = campaign.inviterMonthlyLimit
  campaignForm.rulesText = campaign.rulesText
  campaignForm.reason = ''
}, { immediate: true })

const referralSearch = ref('')
const referralAppliedSearch = ref('')
const referralPage = ref(1)
const referralStatus = ref<AdminReferralQuery['status']>('all')
const referralQueryInput = computed<AdminReferralQuery>(() => ({
  ...defaultAdminReferralQuery,
  page: referralPage.value,
  status: referralStatus.value,
  search: referralAppliedSearch.value,
}))
const referralsQuery = useAdminReferrals(referralQueryInput)

const couponSearch = ref('')
const couponAppliedSearch = ref('')
const couponPage = ref(1)
const couponStatus = ref<AdminPromotionCouponQuery['status']>('all')
const couponSource = ref<AdminPromotionCouponQuery['source']>('all')
const couponQueryInput = computed<AdminPromotionCouponQuery>(() => ({
  ...defaultAdminPromotionCouponQuery,
  page: couponPage.value,
  status: couponStatus.value,
  source: couponSource.value,
  search: couponAppliedSearch.value,
}))
const couponsQuery = useAdminPromotionCoupons(couponQueryInput)

watch([referralStatus, couponStatus, couponSource], () => {
  referralPage.value = 1
  couponPage.value = 1
})

const grantMutation = useGrantAdminPromotionCouponMutation()
const revokeReferralMutation = useRevokeAdminReferralMutation()
const revokeCouponMutation = useRevokeAdminPromotionCouponMutation()
const grantDialogOpen = ref(false)
const grantForm = reactive({ userId: '', durationHours: 24, validDays: 30, reason: '', confirmed: false })
const revokeTarget = ref<{ kind: 'referral', item: AdminReferralRecord } | { kind: 'coupon', item: AdminPromotionCoupon } | null>(null)
const revokeReason = ref('')
const revokeConfirmed = ref(false)

const referralStatusLabels: Record<ReferralStatus, string> = {
  bound: '已绑定', qualified: '已达成', rewarded: '已发奖', rejected: '未通过', revoked: '已撤销',
}
const couponStatusLabels: Record<PromotionCouponStatusValue, string> = {
  pending: '待生效', available: '可使用', used: '已使用', expired: '已过期', revoked: '已撤销',
}
const couponSourceLabels: Record<PromotionCouponSource, string> = {
  welcome_first_api_service: '新人首发', referral_inviter: '邀请人', referral_invitee: '受邀人', admin_grant: '管理员发放',
}

function applyReferralSearch() {
  referralAppliedSearch.value = referralSearch.value.trim().slice(0, 100)
  referralPage.value = 1
}

function applyCouponSearch() {
  couponAppliedSearch.value = couponSearch.value.trim().slice(0, 100)
  couponPage.value = 1
}

function campaignValidationError() {
  const startsAt = beijingDateTimeInputToISOString(campaignForm.startsAt)
  const endsAt = campaignForm.endsAt ? beijingDateTimeInputToISOString(campaignForm.endsAt) : ''
  if (!startsAt) return '请输入有效的活动开始时间。'
  if (campaignForm.endsAt && !endsAt) return '请输入有效的活动结束时间。'
  if (endsAt && Date.parse(endsAt) <= Date.parse(startsAt)) return '活动结束时间必须晚于开始时间。'
  if (campaignForm.promotionDurationHours < 1 || campaignForm.promotionDurationHours > 168) return '推广时长必须在 1 到 168 小时之间。'
  if (campaignForm.couponValidDays < 1 || campaignForm.couponValidDays > 365) return '推广券有效期必须在 1 到 365 天之间。'
  if (campaignForm.rewardDelayHours < 0 || campaignForm.rewardDelayHours > 720) return '奖励延迟必须在 0 到 720 小时之间。'
  if (campaignForm.inviterMonthlyLimit < 0 || campaignForm.inviterMonthlyLimit > 1000) return '邀请人月上限必须在 0 到 1000 之间。'
  if (!campaignForm.rulesText.trim() || Array.from(campaignForm.rulesText.trim()).length > 2000) return '活动规则不能为空且不能超过 2000 字。'
  if (Array.from(campaignForm.reason.trim()).length < 2) return '请填写至少 2 个字的更新原因。'
  return ''
}

function requestCampaignUpdate() {
  const error = campaignValidationError()
  if (error) {
    toast.warning(error)
    return
  }
  campaignDialogOpen.value = true
}

function updateCampaign() {
  const campaign = campaignQuery.data.value
  if (!campaign) return
  updateCampaignMutation.mutate({
    version: campaign.version,
    payload: {
      programEnabled: campaignForm.programEnabled,
      welcomeEnabled: campaignForm.welcomeEnabled,
      referralEnabled: campaignForm.referralEnabled,
      startsAt: beijingDateTimeInputToISOString(campaignForm.startsAt),
      endsAt: campaignForm.endsAt ? beijingDateTimeInputToISOString(campaignForm.endsAt) : '',
      promotionDurationHours: campaignForm.promotionDurationHours,
      couponValidDays: campaignForm.couponValidDays,
      rewardDelayHours: campaignForm.rewardDelayHours,
      inviterMonthlyLimit: campaignForm.inviterMonthlyLimit,
      rulesText: campaignForm.rulesText.trim(),
      reason: campaignForm.reason.trim(),
    },
  }, {
    onSuccess: () => {
      campaignDialogOpen.value = false
      toast.success('增长推广配置已更新。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '配置更新失败。'),
  })
}

function openGrantDialog() {
  const campaign = campaignQuery.data.value
  grantForm.userId = ''
  grantForm.durationHours = campaign?.promotionDurationHours ?? 24
  grantForm.validDays = campaign?.couponValidDays ?? 30
  grantForm.reason = ''
  grantForm.confirmed = false
  grantDialogOpen.value = true
}

function grantCoupon() {
  if (!grantForm.userId.trim() || grantForm.reason.trim().length < 2 || !grantForm.confirmed) {
    toast.warning('请填写用户 ID、操作原因并确认发放。')
    return
  }
  grantMutation.mutate({
    userId: grantForm.userId.trim(),
    durationHours: grantForm.durationHours,
    validDays: grantForm.validDays,
    reason: grantForm.reason.trim(),
  }, {
    onSuccess: () => {
      grantDialogOpen.value = false
      toast.success('推广券已发放。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '推广券发放失败。'),
  })
}

function openRevokeReferral(item: AdminReferralRecord) {
  revokeTarget.value = { kind: 'referral', item }
  revokeReason.value = ''
  revokeConfirmed.value = false
}

function openRevokeCoupon(item: AdminPromotionCoupon) {
  revokeTarget.value = { kind: 'coupon', item }
  revokeReason.value = ''
  revokeConfirmed.value = false
}

function closeRevokeDialog() {
  if (revokeReferralMutation.isPending.value || revokeCouponMutation.isPending.value) return
  revokeTarget.value = null
}

function revokeSelected() {
  const target = revokeTarget.value
  if (!target || revokeReason.value.trim().length < 2 || !revokeConfirmed.value) {
    toast.warning('请填写撤销原因并完成确认。')
    return
  }
  const callbacks = {
    onSuccess: () => {
      revokeTarget.value = null
      toast.success(target.kind === 'referral' ? '邀请关系及关联奖励已撤销。' : '推广券已撤销。')
    },
    onError: (error: Error) => toast.error(error.message || '撤销失败。'),
  }
  if (target.kind === 'referral') {
    revokeReferralMutation.mutate({ referralId: target.item.id, version: target.item.version, reason: revokeReason.value }, callbacks)
  } else {
    revokeCouponMutation.mutate({ couponId: target.item.id, version: target.item.version, reason: revokeReason.value }, callbacks)
  }
}

function rangeStart(page: number, limit: number, total: number) {
  return total === 0 ? 0 : (page - 1) * limit + 1
}

function rangeEnd(page: number, limit: number, total: number) {
  return Math.min(page * limit, total)
}
</script>

<template>
  <div class="min-w-0 space-y-5">
    <PageTitle title="增长推广" description="配置邀请活动并审计邀请关系与推广券。">
      <template #action>
        <div class="flex gap-2">
          <Button variant="outline" size="icon" title="刷新增长推广数据" aria-label="刷新增长推广数据" @click="campaignQuery.refetch(); referralsQuery.refetch(); couponsQuery.refetch()"><RefreshCw class="h-4 w-4" /></Button>
          <Button @click="openGrantDialog"><Gift class="h-4 w-4" />发放推广券</Button>
        </div>
      </template>
    </PageTitle>

    <Tabs v-model="activeTab">
      <TabsList class="max-w-full overflow-x-auto">
        <TabsTrigger value="configuration"><Settings2 class="h-4 w-4" />活动配置</TabsTrigger>
        <TabsTrigger value="referrals"><UserRoundSearch class="h-4 w-4" />邀请关系</TabsTrigger>
        <TabsTrigger value="coupons"><TicketCheck class="h-4 w-4" />推广券</TabsTrigger>
      </TabsList>

      <TabsContent value="configuration" class="mt-5">
        <SkeletonTable v-if="campaignQuery.isLoading.value" :rows="5" :columns="2" />
        <ErrorState v-else-if="campaignQuery.isError.value" title="活动配置读取失败" description="请确认管理权限后重试。" @retry="campaignQuery.refetch()" />
        <form v-else-if="campaignQuery.data.value" class="space-y-6" @submit.prevent="requestCampaignUpdate">
          <Alert>
            <Settings2 />
            <AlertTitle>配置版本 {{ campaignQuery.data.value.version }}</AlertTitle>
            <AlertDescription>关闭活动只会停止新增关系、奖励和用券，已经生效的推广按原结束时间运行。</AlertDescription>
          </Alert>

          <section class="border-y border-border py-5" aria-labelledby="campaign-switches-title">
            <h2 id="campaign-switches-title" class="text-sm font-semibold">活动开关</h2>
            <div class="mt-4 grid gap-3 md:grid-cols-3">
              <label class="flex items-start gap-3 rounded-md border border-border p-3"><Checkbox v-model="campaignForm.programEnabled" class="mt-0.5" /><span><strong class="block text-sm">总开关</strong><span class="mt-1 block text-xs text-muted-foreground">控制新增关系、奖励和用券</span></span></label>
              <label class="flex items-start gap-3 rounded-md border border-border p-3"><Checkbox v-model="campaignForm.welcomeEnabled" class="mt-0.5" /><span><strong class="block text-sm">新人首发</strong><span class="mt-1 block text-xs text-muted-foreground">首个有效 API 服务权益</span></span></label>
              <label class="flex items-start gap-3 rounded-md border border-border p-3"><Checkbox v-model="campaignForm.referralEnabled" class="mt-0.5" /><span><strong class="block text-sm">邀请奖励</strong><span class="mt-1 block text-xs text-muted-foreground">邀请双方延迟发券</span></span></label>
            </div>
          </section>

          <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4" aria-label="活动参数">
            <div class="grid gap-2"><Label for="campaign-start">开始时间</Label><Input id="campaign-start" v-model="campaignForm.startsAt" type="datetime-local" required /></div>
            <div class="grid gap-2"><Label for="campaign-end">结束时间</Label><Input id="campaign-end" v-model="campaignForm.endsAt" type="datetime-local" /></div>
            <div class="grid gap-2"><Label for="campaign-duration">推广时长（小时）</Label><Input id="campaign-duration" v-model.number="campaignForm.promotionDurationHours" type="number" min="1" max="168" /></div>
            <div class="grid gap-2"><Label for="campaign-valid-days">券有效期（天）</Label><Input id="campaign-valid-days" v-model.number="campaignForm.couponValidDays" type="number" min="1" max="365" /></div>
            <div class="grid gap-2"><Label for="campaign-delay">奖励延迟（小时）</Label><Input id="campaign-delay" v-model.number="campaignForm.rewardDelayHours" type="number" min="0" max="720" /></div>
            <div class="grid gap-2"><Label for="campaign-limit">邀请人月上限</Label><Input id="campaign-limit" v-model.number="campaignForm.inviterMonthlyLimit" type="number" min="0" max="1000" /></div>
          </section>

          <div class="grid gap-4 lg:grid-cols-2">
            <div class="grid gap-2"><Label for="campaign-rules">公开活动规则</Label><Textarea id="campaign-rules" v-model="campaignForm.rulesText" rows="7" maxlength="2000" /></div>
            <div class="grid gap-2"><Label for="campaign-reason">更新原因</Label><Textarea id="campaign-reason" v-model="campaignForm.reason" rows="7" maxlength="500" placeholder="记录本次配置调整依据" /></div>
          </div>
          <div class="flex justify-end"><Button type="submit"><Save class="h-4 w-4" />保存配置</Button></div>
        </form>
      </TabsContent>

      <TabsContent value="referrals" class="mt-5 space-y-4">
        <div class="flex flex-col gap-3 md:flex-row">
          <Select v-model="referralStatus"><SelectTrigger class="w-full md:w-44"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem v-for="(label, value) in referralStatusLabels" :key="value" :value="value">{{ label }}</SelectItem></SelectContent></Select>
          <div class="flex min-w-0 flex-1 gap-2"><Input v-model="referralSearch" maxlength="100" placeholder="搜索邀请人或受邀人" @keyup.enter="applyReferralSearch" /><Button variant="outline" size="icon" aria-label="搜索邀请关系" @click="applyReferralSearch"><Search class="h-4 w-4" /></Button></div>
        </div>
        <SkeletonTable v-if="referralsQuery.isLoading.value" :rows="6" :columns="6" />
        <ErrorState v-else-if="referralsQuery.isError.value" title="邀请关系读取失败" description="筛选结果暂时不可用。" @retry="referralsQuery.refetch()" />
        <EmptyState v-else-if="referralsQuery.data.value?.items.length === 0" title="没有邀请关系" description="当前筛选条件下没有记录。" />
        <SoftTable v-else-if="referralsQuery.data.value" class="[&_table]:min-w-[820px]" :columns="['邀请人', '受邀人', '状态', '绑定时间', '风险标记', '操作']">
          <tr v-for="item in referralsQuery.data.value.items" :key="item.id">
            <TableCell><div class="font-medium">{{ item.inviterDisplayName }}</div><ShortId v-if="item.inviterUserId" :value="item.inviterUserId" prefix="USR" /></TableCell>
            <TableCell><div class="font-medium">{{ item.inviteeDisplayName }}</div><ShortId v-if="item.inviteeUserId" :value="item.inviteeUserId" prefix="USR" /></TableCell>
            <TableCell><StatusBadge :status="item.status" :label="referralStatusLabels[item.status]" /></TableCell>
            <TableCell><LocalTime :value="item.boundAt" /></TableCell>
            <TableCell><div class="flex max-w-64 flex-wrap gap-1"><Badge v-for="flag in item.riskFlags ?? []" :key="flag" variant="outline">{{ flag }}</Badge><span v-if="!item.riskFlags?.length" class="text-xs text-muted-foreground">无</span></div></TableCell>
            <TableCell><Button v-if="item.status !== 'revoked'" variant="outline" size="sm" @click="openRevokeReferral(item)"><Ban class="h-4 w-4" />撤销</Button></TableCell>
          </tr>
          <template #footer><TablePagination :page="referralsQuery.data.value.pagination.page" :page-count="referralsQuery.data.value.pagination.totalPages" :total="referralsQuery.data.value.pagination.totalItems" :start-item="rangeStart(referralsQuery.data.value.pagination.page, referralsQuery.data.value.pagination.limit, referralsQuery.data.value.pagination.totalItems)" :end-item="rangeEnd(referralsQuery.data.value.pagination.page, referralsQuery.data.value.pagination.limit, referralsQuery.data.value.pagination.totalItems)" @update:page="referralPage = $event" /></template>
        </SoftTable>
      </TabsContent>

      <TabsContent value="coupons" class="mt-5 space-y-4">
        <div class="grid gap-3 md:grid-cols-[180px_180px_minmax(0,1fr)]">
          <Select v-model="couponStatus"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem v-for="(label, value) in couponStatusLabels" :key="value" :value="value">{{ label }}</SelectItem></SelectContent></Select>
          <Select v-model="couponSource"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部来源</SelectItem><SelectItem v-for="(label, value) in couponSourceLabels" :key="value" :value="value">{{ label }}</SelectItem></SelectContent></Select>
          <div class="flex min-w-0 gap-2"><Input v-model="couponSearch" maxlength="100" placeholder="搜索用户" @keyup.enter="applyCouponSearch" /><Button variant="outline" size="icon" aria-label="搜索推广券" @click="applyCouponSearch"><Search class="h-4 w-4" /></Button></div>
        </div>
        <SkeletonTable v-if="couponsQuery.isLoading.value" :rows="6" :columns="7" />
        <ErrorState v-else-if="couponsQuery.isError.value" title="推广券读取失败" description="筛选结果暂时不可用。" @retry="couponsQuery.refetch()" />
        <EmptyState v-else-if="couponsQuery.data.value?.items.length === 0" title="没有推广券" description="当前筛选条件下没有记录。" />
        <SoftTable v-else-if="couponsQuery.data.value" class="[&_table]:min-w-[980px]" :columns="['用户', '来源', '状态', '时长', '可用 / 失效', '使用对象', '操作']">
          <tr v-for="item in couponsQuery.data.value.items" :key="item.id">
            <TableCell><div class="font-medium">{{ item.userDisplayName || '未知用户' }}</div><ShortId v-if="item.userId" :value="item.userId" prefix="USR" /></TableCell>
            <TableCell>{{ couponSourceLabels[item.sourceType] }}</TableCell>
            <TableCell><StatusBadge :status="item.status" :label="couponStatusLabels[item.status]" /></TableCell>
            <TableCell>{{ item.durationHours }} 小时</TableCell>
            <TableCell class="min-w-48"><div><LocalTime :value="item.availableAt" /></div><div class="mt-1 text-xs text-muted-foreground">至 <LocalTime :value="item.expiresAt" /></div></TableCell>
            <TableCell class="max-w-56 truncate">{{ item.usedApiServiceTitle || '未使用' }}</TableCell>
            <TableCell><Button v-if="item.status !== 'revoked' && item.status !== 'expired'" variant="outline" size="sm" @click="openRevokeCoupon(item)"><Ban class="h-4 w-4" />撤销</Button></TableCell>
          </tr>
          <template #footer><TablePagination :page="couponsQuery.data.value.pagination.page" :page-count="couponsQuery.data.value.pagination.totalPages" :total="couponsQuery.data.value.pagination.totalItems" :start-item="rangeStart(couponsQuery.data.value.pagination.page, couponsQuery.data.value.pagination.limit, couponsQuery.data.value.pagination.totalItems)" :end-item="rangeEnd(couponsQuery.data.value.pagination.page, couponsQuery.data.value.pagination.limit, couponsQuery.data.value.pagination.totalItems)" @update:page="couponPage = $event" /></template>
        </SoftTable>
      </TabsContent>
    </Tabs>

    <Dialog v-model:open="campaignDialogOpen">
      <DialogContent class="sm:max-w-lg"><DialogHeader><DialogTitle>确认更新活动配置</DialogTitle><DialogDescription>将以版本 {{ campaignQuery.data.value?.version }} 提交，并记录管理员审计日志。</DialogDescription></DialogHeader><div class="text-sm text-muted-foreground">原因：{{ campaignForm.reason.trim() }}</div><DialogFooter><Button variant="outline" @click="campaignDialogOpen = false">取消</Button><Button :disabled="updateCampaignMutation.isPending.value" @click="updateCampaign"><Loader2 v-if="updateCampaignMutation.isPending.value" class="h-4 w-4 animate-spin" /><Save v-else class="h-4 w-4" />确认保存</Button></DialogFooter></DialogContent>
    </Dialog>

    <Dialog v-model:open="grantDialogOpen">
      <DialogContent class="sm:max-w-lg"><DialogHeader><DialogTitle>发放推广券</DialogTitle><DialogDescription>推广券只能由目标用户用于自己的可接单 API 服务。</DialogDescription></DialogHeader><div class="grid gap-4 py-2"><div class="grid gap-2"><Label for="grant-user">用户 ID</Label><Input id="grant-user" v-model="grantForm.userId" /></div><div class="grid gap-3 sm:grid-cols-2"><div class="grid gap-2"><Label for="grant-duration">推广时长（小时）</Label><Input id="grant-duration" v-model.number="grantForm.durationHours" type="number" min="1" max="168" /></div><div class="grid gap-2"><Label for="grant-valid-days">有效期（天）</Label><Input id="grant-valid-days" v-model.number="grantForm.validDays" type="number" min="1" max="365" /></div></div><div class="grid gap-2"><Label for="grant-reason">发放原因</Label><Textarea id="grant-reason" v-model="grantForm.reason" maxlength="500" /></div><label class="flex items-start gap-3 text-sm"><Checkbox v-model="grantForm.confirmed" class="mt-0.5" /><span>确认向该用户发放一张不可转让、不可兑换现金的推广券。</span></label></div><DialogFooter><Button variant="outline" @click="grantDialogOpen = false">取消</Button><Button :disabled="grantMutation.isPending.value" @click="grantCoupon"><Loader2 v-if="grantMutation.isPending.value" class="h-4 w-4 animate-spin" /><Gift v-else class="h-4 w-4" />确认发放</Button></DialogFooter></DialogContent>
    </Dialog>

    <Dialog :open="Boolean(revokeTarget)" @update:open="open => { if (!open) closeRevokeDialog() }">
      <DialogContent class="sm:max-w-lg"><DialogHeader><DialogTitle>{{ revokeTarget?.kind === 'referral' ? '撤销邀请关系' : '撤销推广券' }}</DialogTitle><DialogDescription>{{ revokeTarget?.kind === 'referral' ? '关联的未使用奖励会同步撤销，正在展示的奖励推广会立即停止。' : '已经生效的奖励推广会立即停止，推广券不会恢复可用。' }}</DialogDescription></DialogHeader><div class="grid gap-4 py-2"><div class="grid gap-2"><Label for="revoke-reason">撤销原因</Label><Textarea id="revoke-reason" v-model="revokeReason" maxlength="500" /></div><label class="flex items-start gap-3 text-sm"><Checkbox v-model="revokeConfirmed" class="mt-0.5" /><span>确认执行不可恢复的撤销操作。</span></label></div><DialogFooter><Button variant="outline" @click="closeRevokeDialog">取消</Button><Button variant="destructive" :disabled="revokeReferralMutation.isPending.value || revokeCouponMutation.isPending.value" @click="revokeSelected"><Loader2 v-if="revokeReferralMutation.isPending.value || revokeCouponMutation.isPending.value" class="h-4 w-4 animate-spin" /><Ban v-else class="h-4 w-4" />确认撤销</Button></DialogFooter></DialogContent>
    </Dialog>
  </div>
</template>
