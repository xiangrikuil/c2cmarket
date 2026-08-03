<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import type { AdminApiServicePromotion, ApiServicePromotionStatus } from '@/api/generated/openapi'
import { CalendarClock, CircleCheck, CircleStop, LoaderCircle, Megaphone, Plus, RefreshCw, TriangleAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { backendErrorMessage } from '@/lib/backendClient'
import { beijingDateTimeInputToISOString, formatBeijingDateTimeInput } from '@/lib/apiQuotaExpiration'
import { apiPromotionAvailabilityBlockReasons } from '@/lib/apiPromotionAvailability'
import { useAdminApiPromotionAvailability, useAdminApiPromotions, useAdminApiServiceOptions, useCreateApiPromotionMutation, useStopApiPromotionMutation } from '@/queries/useMarketQueries'

const promotionsQuery = useAdminApiPromotions()
const serviceOptionsQuery = useAdminApiServiceOptions()
const createMutation = useCreateApiPromotionMutation()
const stopMutation = useStopApiPromotionMutation()
const createOpen = ref(false)
const stopping = ref<AdminApiServicePromotion | null>(null)
const stopReason = ref('')

const form = reactive({
  apiServiceId: '',
  startsAt: '',
  endsAt: '',
  reason: '',
})
const proposedStartsAt = computed(() => beijingDateTimeInputToISOString(form.startsAt))
const proposedEndsAt = computed(() => beijingDateTimeInputToISOString(form.endsAt))
const availabilityEnabled = computed(() => createOpen.value
  && Boolean(form.apiServiceId)
  && Boolean(proposedStartsAt.value)
  && Boolean(proposedEndsAt.value)
  && Date.parse(proposedEndsAt.value) > Date.parse(proposedStartsAt.value))
const availabilityQuery = useAdminApiPromotionAvailability(
  computed(() => form.apiServiceId),
  proposedStartsAt,
  proposedEndsAt,
  availabilityEnabled,
)
const availability = computed(() => availabilityQuery.data.value)
const availabilityBlocks = computed(() => availability.value
  ? apiPromotionAvailabilityBlockReasons(availability.value)
  : [])
const createDisabled = computed(() => createMutation.isPending.value
  || !availabilityEnabled.value
  || availabilityQuery.isFetching.value
  || availabilityQuery.isError.value
  || !availability.value
  || availabilityBlocks.value.length > 0
  || form.reason.trim().length === 0
  || form.reason.trim().length > 500)

const items = computed(() => promotionsQuery.data.value ?? [])
const activeItems = computed(() => items.value.filter(item => ['serving', 'suppressed'].includes(item.status)))
const scheduledItems = computed(() => items.value.filter(item => item.status === 'scheduled'))
const occupiedNow = computed(() => activeItems.value.length)
const capacity = computed(() => items.value[0]?.capacity ?? 3)
const stats = computed(() => [
  { label: '当前占用', value: `${occupiedNow.value} / ${capacity.value}`, hint: '展示中和临时抑制' },
  { label: '正在展示', value: items.value.filter(item => item.status === 'serving').length },
  { label: '待开始', value: scheduledItems.value.length },
  { label: '暂不可展示', value: items.value.filter(item => item.status === 'suppressed').length },
])

const statusLabel: Record<ApiServicePromotionStatus, string> = {
  stopped: '已停止',
  scheduled: '待开始',
  finished: '已结束',
  suppressed: '暂不可展示',
  serving: '推广中',
}

function statusTone(status: ApiServicePromotionStatus) {
  if (status === 'serving') return 'success' as const
  if (status === 'scheduled') return 'waiting' as const
  if (status === 'suppressed') return 'warning' as const
  return 'neutral' as const
}

function resetCreateForm() {
  const start = new Date()
  start.setSeconds(0, 0)
  form.apiServiceId = ''
  form.startsAt = formatBeijingDateTimeInput(start)
  form.endsAt = formatBeijingDateTimeInput(new Date(start.getTime() + 7 * 86_400_000))
  form.reason = ''
}

function openCreate() {
  resetCreateForm()
  createOpen.value = true
}

function applyDuration(days: number) {
  const startsAt = Date.parse(beijingDateTimeInputToISOString(form.startsAt))
  if (!Number.isFinite(startsAt)) return
  form.endsAt = formatBeijingDateTimeInput(new Date(startsAt + days * 86_400_000))
}

function validateCreate() {
  if (!form.apiServiceId) return '请选择 API 服务。'
  if (!form.startsAt || !form.endsAt) return '请选择开始和结束时间。'
  if (Date.parse(beijingDateTimeInputToISOString(form.endsAt)) <= Date.parse(beijingDateTimeInputToISOString(form.startsAt))) return '结束时间必须晚于开始时间。'
  if (!form.reason.trim()) return '请填写设置推广的原因。'
  if (form.reason.trim().length > 500) return '设置原因不能超过 500 个字符。'
  if (!availability.value || availabilityQuery.isFetching.value) return '请等待排期校验完成。'
  if (availabilityBlocks.value.length > 0) return availabilityBlocks.value[0]
  return ''
}

async function createPromotion() {
  const message = validateCreate()
  if (message) {
    toast.warning(message)
    return
  }
  try {
    await createMutation.mutateAsync({
      apiServiceId: form.apiServiceId,
      placement: 'api_market_top',
      startsAt: beijingDateTimeInputToISOString(form.startsAt),
      endsAt: beijingDateTimeInputToISOString(form.endsAt),
      reason: form.reason.trim(),
    })
    createOpen.value = false
    toast.success('推广排期已创建。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '创建推广排期失败。'))
  }
}

function openStop(item: AdminApiServicePromotion) {
  stopping.value = item
  stopReason.value = ''
}

async function stopPromotion() {
  if (!stopping.value) return
  if (!stopReason.value.trim()) {
    toast.warning('请填写提前停止原因。')
    return
  }
  try {
    await stopMutation.mutateAsync({
      id: stopping.value.id,
      version: stopping.value.version,
      reason: stopReason.value.trim(),
    })
    stopping.value = null
    toast.success('推广已提前停止。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '停止推广失败，请刷新后重试。'))
    await promotionsQuery.refetch()
  }
}

function formatTime(value: string | null | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function canStop(item: AdminApiServicePromotion) {
  return ['scheduled', 'serving', 'suppressed'].includes(item.status)
}

function firstIssue(item: AdminApiServicePromotion) {
  return item.eligibility.hardBlockReasons[0]
    ?? item.eligibility.warningReasons[0]
    ?? item.eligibility.suppressionReasons[0]
    ?? ''
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="API 推广" description="管理员手动安排 API 市场顶部推广池；推广不改变自然排序、信誉或徽章。">
      <template #action><Button class="gap-2" @click="openCreate"><Plus class="h-4 w-4" />创建推广排期</Button></template>
    </PageTitle>

    <CompactStats :items="stats" :loading="promotionsQuery.isLoading.value" />

    <Alert>
      <Megaphone />
      <AlertTitle>推广池容量为 3</AlertTitle>
      <AlertDescription>时间重叠的待开始和进行中活动都会占用容量；服务售罄或暂停接单时保留排期，但公开区域暂不展示。</AlertDescription>
    </Alert>

    <ErrorState v-if="promotionsQuery.isError.value" description="推广排期读取失败，请确认管理权限后重试。" @retry="promotionsQuery.refetch()" />
    <SkeletonTable v-else-if="promotionsQuery.isLoading.value" :rows="6" :columns="5" />
    <EmptyState v-else-if="items.length === 0" title="暂无推广排期" description="创建第一条管理员运营推广排期后会显示在这里。" />
    <Card v-else class="overflow-hidden p-0">
      <div class="flex items-center justify-between border-b border-border px-4 py-3">
        <div><h2 class="font-semibold">全部排期</h2><p class="mt-1 text-xs text-muted-foreground">状态按当前时间和服务实时资格派生。</p></div>
        <Button variant="outline" size="icon" title="刷新推广排期" @click="promotionsQuery.refetch()"><RefreshCw class="h-4 w-4" /><span class="sr-only">刷新</span></Button>
      </div>
      <div class="overflow-x-auto">
        <Table>
          <TableHeader><TableRow><TableHead>服务</TableHead><TableHead>排期</TableHead><TableHead>状态与资格</TableHead><TableHead>创建原因</TableHead><TableHead class="text-right">操作</TableHead></TableRow></TableHeader>
          <TableBody>
            <TableRow v-for="item in items" :key="item.id">
              <TableCell class="min-w-52"><div class="font-medium">{{ item.service.title }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.service.merchantDisplayName || '未设置展示名' }}</div></TableCell>
              <TableCell class="min-w-48"><div class="flex items-center gap-1.5 text-xs"><CalendarClock class="h-3.5 w-3.5 text-muted-foreground" />{{ formatTime(item.startsAt) }}</div><div class="mt-1 pl-5 text-xs text-muted-foreground">至 {{ formatTime(item.endsAt) }}</div></TableCell>
              <TableCell class="min-w-56"><StatusBadge :status="item.status" :label="statusLabel[item.status]" :tone="statusTone(item.status)" /><div v-if="firstIssue(item)" class="mt-2 flex items-start gap-1.5 text-xs text-muted-foreground"><TriangleAlert class="mt-0.5 h-3.5 w-3.5 shrink-0" /><span>{{ firstIssue(item) }}</span></div><div v-else class="mt-2 text-xs text-muted-foreground">当前资格正常</div></TableCell>
              <TableCell class="max-w-64"><p class="line-clamp-2 text-sm">{{ item.createdReason }}</p><Badge v-if="item.overlappingCampaigns" variant="outline" class="mt-2">同期 {{ item.overlappingCampaigns }} / {{ item.capacity }}</Badge></TableCell>
              <TableCell class="text-right"><Button v-if="canStop(item)" variant="outline" size="sm" class="gap-1.5" @click="openStop(item)"><CircleStop class="h-3.5 w-3.5" />停止</Button><span v-else class="text-xs text-muted-foreground">{{ item.stoppedReason || '无需操作' }}</span></TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </Card>

    <Dialog v-model:open="createOpen">
      <DialogContent class="sm:max-w-xl">
        <DialogHeader><DialogTitle>创建推广排期</DialogTitle><DialogDescription>选择服务与展示时间。系统会检查服务资格、同服务重复排期和推广池容量。</DialogDescription></DialogHeader>
        <div class="grid gap-4 py-2">
          <div class="grid gap-2"><Label for="promotion-service">API 服务</Label><Select v-model="form.apiServiceId"><SelectTrigger id="promotion-service"><SelectValue placeholder="请选择 API 服务" /></SelectTrigger><SelectContent><SelectItem v-for="service in serviceOptionsQuery.data.value ?? []" :key="service.id" :value="service.id"><span>{{ service.title }}</span><span class="ml-2 text-xs text-muted-foreground">{{ service.reviewStatus }} · {{ service.publicationStatus }}</span></SelectItem></SelectContent></Select></div>
          <div class="grid gap-3 sm:grid-cols-2"><div class="grid gap-2"><Label for="promotion-start">开始时间</Label><Input id="promotion-start" v-model="form.startsAt" type="datetime-local" /></div><div class="grid gap-2"><Label for="promotion-end">结束时间</Label><Input id="promotion-end" v-model="form.endsAt" type="datetime-local" /></div></div>
          <div class="flex flex-wrap items-center gap-2"><span class="text-xs text-muted-foreground">快速时长</span><Button v-for="days in [7, 14, 30]" :key="days" type="button" variant="outline" size="sm" @click="applyDuration(days)">{{ days }} 天</Button></div>
          <div v-if="availabilityEnabled" class="space-y-3 border-y border-border py-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-2 text-sm font-medium"><CircleCheck class="h-4 w-4 text-muted-foreground" />排期校验</div>
              <div v-if="availability" class="flex flex-wrap gap-2">
                <Badge :variant="availability.eligibility.configurable ? 'secondary' : 'destructive'">{{ availability.eligibility.configurable ? '可配置' : '不可配置' }}</Badge>
                <Badge :variant="availability.eligibility.displayable ? 'secondary' : 'outline'">{{ availability.eligibility.displayable ? '当前可展示' : '暂不可展示' }}</Badge>
              </div>
            </div>
            <div v-if="availabilityQuery.isFetching.value" class="flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle class="h-4 w-4 animate-spin" />正在校验服务资格和排期容量</div>
            <Alert v-else-if="availabilityQuery.isError.value" variant="destructive">
              <TriangleAlert />
              <AlertTitle>排期校验失败</AlertTitle>
              <AlertDescription class="flex items-center justify-between gap-3"><span>无法确认服务资格或推广池容量。</span><Button type="button" variant="outline" size="sm" @click="availabilityQuery.refetch()">重试</Button></AlertDescription>
            </Alert>
            <template v-else-if="availability">
              <div class="grid gap-2 text-sm sm:grid-cols-3">
                <div><span class="text-muted-foreground">峰值占用</span><div class="mt-0.5 font-medium">{{ availability.overlappingCampaigns }} / {{ availability.capacity }}</div></div>
                <div><span class="text-muted-foreground">剩余容量</span><div class="mt-0.5 font-medium">{{ availability.remainingCapacity }}</div></div>
                <div><span class="text-muted-foreground">同服务重叠</span><div class="mt-0.5 font-medium">{{ availability.sameServiceOverlap ? '有' : '无' }}</div></div>
              </div>
              <Alert v-if="availabilityBlocks.length" variant="destructive">
                <TriangleAlert />
                <AlertTitle>当前排期不可创建</AlertTitle>
                <AlertDescription><ul class="list-disc space-y-1 pl-4"><li v-for="reason in availabilityBlocks" :key="reason">{{ reason }}</li></ul></AlertDescription>
              </Alert>
              <Alert v-if="availability.eligibility.warningReasons.length">
                <TriangleAlert />
                <AlertTitle>需要人工复核</AlertTitle>
                <AlertDescription><ul class="list-disc space-y-1 pl-4"><li v-for="reason in availability.eligibility.warningReasons" :key="reason">{{ reason }}</li></ul></AlertDescription>
              </Alert>
              <Alert v-if="!availability.eligibility.displayable && availability.eligibility.suppressionReasons.length">
                <TriangleAlert />
                <AlertTitle>创建后暂不公开展示</AlertTitle>
                <AlertDescription><ul class="list-disc space-y-1 pl-4"><li v-for="reason in availability.eligibility.suppressionReasons" :key="reason">{{ reason }}</li></ul><p v-if="availabilityBlocks.length === 0" class="mt-2">可以创建排期；服务恢复公开下单资格后会自动展示。</p></AlertDescription>
              </Alert>
            </template>
          </div>
          <div class="grid gap-2"><Label for="promotion-reason">设置原因</Label><Textarea id="promotion-reason" v-model="form.reason" rows="3" maxlength="500" placeholder="记录本次运营安排依据" /></div>
        </div>
        <DialogFooter><Button variant="outline" @click="createOpen = false">取消</Button><Button :disabled="createDisabled" @click="createPromotion">{{ createMutation.isPending.value ? '创建中' : '创建排期' }}</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="Boolean(stopping)" @update:open="value => { if (!value) stopping = null }">
      <DialogContent class="sm:max-w-md">
        <DialogHeader><DialogTitle>提前停止推广</DialogTitle><DialogDescription>{{ stopping?.service.title }} 停止后会立即退出公开推广区，历史排期和审计记录继续保留。</DialogDescription></DialogHeader>
        <div class="grid gap-2 py-2"><Label for="promotion-stop-reason">停止原因</Label><Textarea id="promotion-stop-reason" v-model="stopReason" rows="3" maxlength="500" placeholder="填写提前停止原因" /></div>
        <DialogFooter><Button variant="outline" @click="stopping = null">取消</Button><Button variant="destructive" :disabled="stopMutation.isPending.value" @click="stopPromotion">{{ stopMutation.isPending.value ? '停止中' : '确认停止' }}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
