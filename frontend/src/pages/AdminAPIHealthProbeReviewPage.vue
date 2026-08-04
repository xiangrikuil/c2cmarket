<script setup lang="ts">
import { computed, ref } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import CheckCircle2 from 'lucide-vue-next/dist/esm/icons/circle-check-big.js'
import RefreshCcw from 'lucide-vue-next/dist/esm/icons/refresh-ccw.js'
import ShieldX from 'lucide-vue-next/dist/esm/icons/shield-x.js'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { backendErrorMessage } from '@/lib/backendClient'
import { useAdminAPIHealthProbes, useReviewAPIHealthProbeMutation } from '@/queries/useApiHealthQueries'
import type { AdminAPIHealthProbeReview, ApiHealthAuthorizationStatus } from '@/types/apiHealth'

const status = ref<ApiHealthAuthorizationStatus>('pending')
const query = useAdminAPIHealthProbes(status)
const reviewMutation = useReviewAPIHealthProbeMutation()
const dialogOpen = ref(false)
const selected = ref<AdminAPIHealthProbeReview | null>(null)
const decision = ref<'approve' | 'reject'>('approve')
const reason = ref('')

const rows = computed(() => query.data.value?.items ?? [])
const errorMessage = computed(() => backendErrorMessage(query.error.value, '探针授权列表暂时无法读取。'))
const actionTitle = computed(() => decision.value === 'approve' ? '批准当前 Origin' : '拒绝当前 Origin')

const statusLabels: Record<ApiHealthAuthorizationStatus, string> = {
  pending: '待审核',
  verified: '自动验证通过',
  approved: '已批准',
  rejected: '已拒绝',
}

function badgeVariant(value: ApiHealthAuthorizationStatus) {
  if (value === 'approved' || value === 'verified') return 'verified' as const
  if (value === 'rejected') return 'destructive' as const
  return 'secondary' as const
}

function shortId(value: string) {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value
}

function serviceLabel(row: AdminAPIHealthProbeReview) {
  return row.serviceTitle || `API 服务 ${shortId(row.apiServiceId)}`
}

function ownerLabel(row: AdminAPIHealthProbeReview) {
  return row.ownerDisplayName || row.ownerUsername || `卖家 ${shortId(row.ownerUserId)}`
}

function openReview(row: AdminAPIHealthProbeReview, nextDecision: 'approve' | 'reject') {
  selected.value = row
  decision.value = nextDecision
  reason.value = ''
  dialogOpen.value = true
}

async function submitReview() {
  if (!selected.value || !reason.value.trim() || reviewMutation.isPending.value) return
  try {
    await reviewMutation.mutateAsync({
      id: selected.value.id,
      version: selected.value.version,
      decision: decision.value,
      reason: reason.value,
    })
    dialogOpen.value = false
    toast.success(decision.value === 'approve' ? '当前 Origin 已批准。' : '当前 Origin 已拒绝。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '探针授权审核失败。'))
  }
}
</script>

<template>
  <main class="mx-auto w-full max-w-[1440px]">
    <PageTitle title="API 探针授权" description="审核卖家无法自行验证的精确 Origin；批准只作用于当前配置版本。">
      <template #action>
        <div class="flex w-full gap-2 md:w-auto">
          <Select v-model="status">
            <SelectTrigger class="w-full md:w-44" aria-label="授权状态筛选"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="pending">待审核</SelectItem>
              <SelectItem value="approved">已批准</SelectItem>
              <SelectItem value="rejected">已拒绝</SelectItem>
              <SelectItem value="verified">自动验证通过</SelectItem>
            </SelectContent>
          </Select>
          <Button size="icon" variant="outline" title="刷新授权列表" aria-label="刷新授权列表" @click="query.refetch()"><RefreshCcw class="h-4 w-4" /></Button>
        </div>
      </template>
    </PageTitle>

    <SkeletonTable v-if="query.isLoading.value" :rows="5" />
    <ErrorState v-else-if="query.error.value" title="无法读取探针授权" :description="errorMessage" @retry="query.refetch()" />
    <EmptyState v-else-if="rows.length === 0" title="当前没有探针授权记录" description="切换状态可查看其他审核结果。" />

    <div v-else class="grid gap-3">
      <Card v-for="row in rows" :key="row.id">
        <CardContent class="grid gap-4 p-4 lg:grid-cols-[minmax(12rem,1fr)_minmax(12rem,1fr)_minmax(18rem,1.5fr)_auto] lg:items-center">
          <div class="min-w-0">
            <div class="text-xs text-muted-foreground">API 服务</div>
            <div class="mt-1 truncate font-medium" :title="serviceLabel(row)">{{ serviceLabel(row) }}</div>
          </div>
          <div class="min-w-0">
            <div class="text-xs text-muted-foreground">卖家</div>
            <div class="mt-1 truncate font-medium" :title="ownerLabel(row)">{{ ownerLabel(row) }}</div>
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span>精确 Origin</span>
              <Badge :variant="badgeVariant(row.authorizationStatus)">{{ statusLabels[row.authorizationStatus] }}</Badge>
            </div>
            <div class="mt-1 break-all font-mono text-xs" :title="row.normalizedOrigin">{{ row.normalizedOrigin }}</div>
            <div class="mt-1 text-xs text-muted-foreground">更新于 <LocalTime :value="row.updatedAt" /></div>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <Button v-if="row.authorizationStatus !== 'approved'" size="sm" :disabled="reviewMutation.isPending.value" @click="openReview(row, 'approve')"><CheckCircle2 class="h-4 w-4" />批准</Button>
            <Button v-if="row.authorizationStatus !== 'rejected'" size="sm" variant="outline" :disabled="reviewMutation.isPending.value" @click="openReview(row, 'reject')"><ShieldX class="h-4 w-4" />拒绝</Button>
          </div>
        </CardContent>
      </Card>
    </div>

    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2"><Activity class="h-5 w-5 text-primary" />{{ actionTitle }}</DialogTitle>
          <DialogDescription>
            {{ selected ? `${serviceLabel(selected)} · ${selected.normalizedOrigin}` : '' }}
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-2 py-2">
          <Label for="api-health-review-reason">审核理由</Label>
          <Textarea id="api-health-review-reason" v-model="reason" rows="4" maxlength="500" placeholder="记录当前精确 Origin 的审批依据" />
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="reviewMutation.isPending.value" @click="dialogOpen = false">取消</Button>
          <Button :variant="decision === 'reject' ? 'destructive' : 'default'" :disabled="!reason.trim() || reviewMutation.isPending.value" @click="submitReview">
            {{ decision === 'approve' ? '确认批准' : '确认拒绝' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </main>
</template>
