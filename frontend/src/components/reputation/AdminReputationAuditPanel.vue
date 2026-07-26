<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Activity, Ban, History, RefreshCw, ShieldCheck, Undo2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import ReputationMetricsGrid from '@/components/reputation/ReputationMetricsGrid.vue'
import ReputationSummaryCard from '@/components/reputation/ReputationSummaryCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  reputationRoleLabel,
  reputationScopeLabel,
  snapshotToSummary,
} from '@/lib/reputationPresentation'
import {
  useAdminUserReputationQuery,
  useCreateReputationRestrictionMutation,
  useRecalculateAllReputationMutation,
  useRecalculateUserReputationMutation,
  useRevokeReputationRestrictionMutation,
  useUpdateSourceAuthorVerificationMutation,
} from '@/queries/useReputationQueries'
import type {
  ReputationRole,
  SourceAuthorVerificationAudit,
  SourceAuthorVerificationStatus,
} from '@/types/reputation'

const props = defineProps<{
  userId: string
  username: string
  userVersion?: number
}>()

const auditQuery = useAdminUserReputationQuery(computed(() => props.userId))
const recalculateUserMutation = useRecalculateUserReputationMutation()
const recalculateAllMutation = useRecalculateAllReputationMutation()
const createRestrictionMutation = useCreateReputationRestrictionMutation()
const revokeRestrictionMutation = useRevokeReputationRestrictionMutation()
const updateVerificationMutation = useUpdateSourceAuthorVerificationMutation()
const selectedSnapshotKey = ref('buyer:overall')

const restrictionForm = reactive({
  restrictionType: 'manual_review_hold',
  roleScope: 'all' as ReputationRole | 'all',
  actionCode: 'all',
  reasonCode: 'manual_review',
  publicReason: '',
  internalReason: '',
  endsAt: '',
})

type VerificationDraft = {
  status: SourceAuthorVerificationStatus
  actualExternalUserId: string
  verificationMethod: string
  expiresAt: string
  failureReason: string
}

const verificationDrafts = reactive<Record<string, VerificationDraft>>({})
const audit = computed(() => auditQuery.data.value)
const selectedSnapshot = computed(() => audit.value?.items.find(item =>
  `${item.role}:${item.scope}` === selectedSnapshotKey.value,
) ?? audit.value?.items[0] ?? null)
const activeRestrictions = computed(() => audit.value?.restrictions.filter(item => !item.revokedAt) ?? [])
const verificationForms = computed(() => (audit.value?.sourceAuthorVerifications ?? []).map(item => ({
  audit: item,
  draft: verificationDrafts[verificationKey(item)]!,
})).filter(item => item.draft))

watch(
  () => audit.value?.sourceAuthorVerifications,
  (items) => {
    for (const item of items ?? []) {
      verificationDrafts[verificationKey(item)] = {
        status: item.verification.status,
        actualExternalUserId: item.verification.actualExternalUserId ?? '',
        verificationMethod: item.verification.verificationMethod ?? '',
        expiresAt: toDateTimeLocal(item.verification.expiresAt),
        failureReason: item.verification.failureReason ?? '',
      }
    }
  },
  { immediate: true },
)

function verificationKey(item: SourceAuthorVerificationAudit) {
  return `${item.verification.resourceType}:${item.verification.resourceId}`
}

function toDateTimeLocal(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toISOString(value: string) {
  if (!value) return null
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date.toISOString() : null
}

function sourceAuthorStatusLabel(status: SourceAuthorVerificationStatus) {
  if (status === 'verified') return '已验证'
  if (status === 'pending') return '待核验'
  if (status === 'mismatch') return '作者不一致'
  if (status === 'expired') return '已过期'
  return '未提交'
}

async function recalculateUser() {
  try {
    const result = await recalculateUserMutation.mutateAsync(props.userId)
    toast.success(`已重算 ${result.rebuiltStates} 份信誉快照。`)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '用户信誉重算失败。')
  }
}

async function recalculateAll() {
  if (!window.confirm('确认重算全部用户的信誉快照？')) return
  try {
    const result = await recalculateAllMutation.mutateAsync()
    toast.success(`已重算 ${result.requestedUsers} 个用户，共 ${result.rebuiltStates} 份快照。`)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '全量信誉重算失败。')
  }
}

async function createRestriction() {
  if (!props.userVersion) {
    toast.error('用户版本尚未加载，请刷新用户目录后重试。')
    return
  }
  if (!restrictionForm.publicReason.trim() || !restrictionForm.internalReason.trim()) {
    toast.warning('公开原因和内部原因都必须填写。')
    return
  }
  try {
    await createRestrictionMutation.mutateAsync({
      userId: props.userId,
      restrictionType: restrictionForm.restrictionType.trim(),
      roleScope: restrictionForm.roleScope,
      actionCode: restrictionForm.actionCode,
      reasonCode: restrictionForm.reasonCode.trim(),
      publicReason: restrictionForm.publicReason.trim(),
      internalReason: restrictionForm.internalReason.trim(),
      startsAt: new Date().toISOString(),
      endsAt: toISOString(restrictionForm.endsAt),
      expectedUserVersion: props.userVersion,
    })
    restrictionForm.publicReason = ''
    restrictionForm.internalReason = ''
    restrictionForm.endsAt = ''
    toast.success('信誉限制已创建。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '创建信誉限制失败。')
  }
}

async function revokeRestriction(restrictionId: string, version: number) {
  const reason = window.prompt('请填写撤销限制的内部原因。')
  if (!reason?.trim()) return
  try {
    await revokeRestrictionMutation.mutateAsync({
      userId: props.userId,
      restrictionId,
      version,
      reason: reason.trim(),
    })
    toast.success('信誉限制已撤销。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '撤销信誉限制失败。')
  }
}

async function updateVerification(item: SourceAuthorVerificationAudit, draft: VerificationDraft) {
  try {
    await updateVerificationMutation.mutateAsync({
      resourceType: item.verification.resourceType,
      resourceId: item.verification.resourceId,
      status: draft.status,
      actualExternalUserId: draft.actualExternalUserId.trim(),
      verificationMethod: draft.verificationMethod.trim(),
      expiresAt: toISOString(draft.expiresAt),
      failureReason: draft.failureReason.trim(),
      version: item.verification.version,
    })
    toast.success('原帖作者核验已更新。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '更新原帖作者核验失败。')
  }
}
</script>

<template>
  <section class="space-y-5 border-y border-border py-5" aria-labelledby="admin-reputation-audit-title">
    <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <h2 id="admin-reputation-audit-title" class="text-lg font-semibold">@{{ username }} 的信誉审计</h2>
          <Badge variant="outline">{{ audit?.ruleVersion ?? '规则加载中' }}</Badge>
        </div>
        <p class="mt-1 text-sm text-muted-foreground">查看六份快照、原始指标、历史和治理证据。操作结果以服务端版本控制为准。</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button variant="outline" :disabled="recalculateUserMutation.isPending.value" @click="recalculateUser">
          <RefreshCw class="h-4 w-4" />重算该用户
        </Button>
        <Button variant="outline" :disabled="recalculateAllMutation.isPending.value" @click="recalculateAll">
          <Activity class="h-4 w-4" />重算全部
        </Button>
      </div>
    </div>

    <SkeletonBlock v-if="auditQuery.isLoading.value" :lines="8" />
    <ErrorState
      v-else-if="auditQuery.error.value"
      title="信誉审计加载失败"
      description="当前无法读取该用户的真实信誉快照与治理证据。"
      @retry="auditQuery.refetch()"
    />

    <template v-else-if="audit">
      <div class="grid gap-3 lg:grid-cols-2">
        <ReputationSummaryCard
          v-for="snapshot in audit.items"
          :key="`${snapshot.role}:${snapshot.scope}`"
          :summary="snapshotToSummary(snapshot)"
          compact
        />
      </div>

      <Card v-if="selectedSnapshot" class="p-5">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="font-semibold">原始指标</h3>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ reputationRoleLabel(selectedSnapshot.role) }} · {{ reputationScopeLabel(selectedSnapshot.scope) }}
            </p>
          </div>
          <Select v-model="selectedSnapshotKey">
            <SelectTrigger class="w-full sm:w-52"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="snapshot in audit.items" :key="`${snapshot.role}:${snapshot.scope}`" :value="`${snapshot.role}:${snapshot.scope}`">
                {{ reputationRoleLabel(snapshot.role) }} · {{ reputationScopeLabel(snapshot.scope) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="mt-5">
          <ReputationMetricsGrid :metrics="selectedSnapshot.metrics" />
        </div>
      </Card>

      <div class="grid gap-5 xl:grid-cols-2">
        <Card class="p-5">
          <div class="flex items-center gap-2">
            <Ban class="h-4 w-4 text-destructive" />
            <h3 class="font-semibold">创建信誉限制</h3>
            <Badge variant="secondary">有效 {{ activeRestrictions.length }}</Badge>
          </div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <label class="space-y-1.5 text-sm">
              <span>限制类型</span>
              <Input v-model="restrictionForm.restrictionType" />
            </label>
            <label class="space-y-1.5 text-sm">
              <span>原因代码</span>
              <Input v-model="restrictionForm.reasonCode" />
            </label>
            <label class="space-y-1.5 text-sm">
              <span>角色范围</span>
              <Select v-model="restrictionForm.roleScope">
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部角色</SelectItem>
                  <SelectItem value="buyer">买家</SelectItem>
                  <SelectItem value="seller">卖家</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-1.5 text-sm">
              <span>限制动作</span>
              <Select v-model="restrictionForm.actionCode">
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部动作</SelectItem>
                  <SelectItem value="carpool_publish">发布车源</SelectItem>
                  <SelectItem value="carpool_apply">申请上车</SelectItem>
                  <SelectItem value="carpool_accept">接受申请</SelectItem>
                  <SelectItem value="api_service_publish">发布 API 服务</SelectItem>
                  <SelectItem value="api_order_create">创建 API 订单</SelectItem>
                  <SelectItem value="contact_view">查看联系方式</SelectItem>
                  <SelectItem value="review_submit">提交评价</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="space-y-1.5 text-sm sm:col-span-2">
              <span>结束时间（可选）</span>
              <Input v-model="restrictionForm.endsAt" type="datetime-local" />
            </label>
            <label class="space-y-1.5 text-sm sm:col-span-2">
              <span>公开原因</span>
              <Textarea v-model="restrictionForm.publicReason" rows="2" />
            </label>
            <label class="space-y-1.5 text-sm sm:col-span-2">
              <span>内部原因</span>
              <Textarea v-model="restrictionForm.internalReason" rows="2" />
            </label>
          </div>
          <Button class="mt-4 w-full" :disabled="createRestrictionMutation.isPending.value" @click="createRestriction">
            <ShieldCheck class="h-4 w-4" />创建限制
          </Button>
        </Card>

        <Card class="p-5">
          <div class="flex items-center gap-2">
            <History class="h-4 w-4" />
            <h3 class="font-semibold">限制记录</h3>
          </div>
          <div v-if="audit.restrictions.length" class="mt-4 divide-y">
            <div v-for="item in audit.restrictions" :key="item.id" class="space-y-2 py-3 first:pt-0">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex flex-wrap items-center gap-2">
                  <Badge :variant="item.revokedAt ? 'secondary' : 'destructive'">{{ item.revokedAt ? '已撤销' : '有效' }}</Badge>
                  <span class="font-medium">{{ item.actionCode }} · {{ item.roleScope }}</span>
                </div>
                <Button v-if="!item.revokedAt" size="sm" variant="outline" @click="revokeRestriction(item.id, item.version)">
                  <Undo2 class="h-3.5 w-3.5" />撤销
                </Button>
              </div>
              <p class="text-sm">{{ item.publicReason }}</p>
              <p class="text-xs text-muted-foreground">{{ item.internalReason }}</p>
              <p class="text-xs text-muted-foreground">
                <LocalTime :value="item.startsAt" /> - <LocalTime v-if="item.endsAt" :value="item.endsAt" /><span v-else>长期</span>
              </p>
            </div>
          </div>
          <p v-else class="mt-4 text-sm text-muted-foreground">暂无信誉限制记录。</p>
        </Card>
      </div>

      <div class="grid gap-5 xl:grid-cols-3">
        <Card class="p-5">
          <h3 class="font-semibold">快照历史</h3>
          <div v-if="audit.history.length" class="mt-3 divide-y text-sm">
            <div v-for="item in audit.history" :key="item.id" class="py-3 first:pt-0">
              <div>{{ reputationRoleLabel(item.role) }} · {{ reputationScopeLabel(item.scope) }}</div>
              <p class="mt-1 text-muted-foreground">{{ item.fromTier ?? '首次计算' }} → {{ item.toTier }} · {{ item.fromState ?? '首次计算' }} → {{ item.toState }}</p>
              <p class="mt-1 text-xs text-muted-foreground"><LocalTime :value="item.createdAt" /> · {{ item.ruleVersion }}</p>
            </div>
          </div>
          <p v-else class="mt-3 text-sm text-muted-foreground">暂无等级或状态变化。</p>
        </Card>

        <Card class="p-5">
          <h3 class="font-semibold">纠纷信誉裁定</h3>
          <div v-if="audit.outcomes.length" class="mt-3 divide-y text-sm">
            <div v-for="item in audit.outcomes" :key="item.id" class="space-y-1 py-3 first:pt-0">
              <div class="flex flex-wrap items-center gap-2">
                <Badge :variant="item.status === 'active' ? 'destructive' : 'secondary'">{{ item.status }}</Badge>
                <span>{{ item.responsibility }} · {{ item.severity }} · {{ item.roleScope }}</span>
              </div>
              <p>{{ item.publicReason }}</p>
              <p class="text-xs text-muted-foreground">{{ item.internalReason }}</p>
              <p class="text-xs text-muted-foreground"><LocalTime :value="item.decidedAt" /></p>
            </div>
          </div>
          <p v-else class="mt-3 text-sm text-muted-foreground">暂无纠纷信誉裁定。</p>
        </Card>

        <Card class="p-5">
          <h3 class="font-semibold">相关申诉</h3>
          <div v-if="audit.appeals.length" class="mt-3 divide-y text-sm">
            <div v-for="item in audit.appeals" :key="item.id" class="space-y-1 py-3 first:pt-0">
              <div class="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">{{ item.status }}</Badge>
                <span class="font-medium">{{ item.title }}</span>
              </div>
              <p>{{ item.statement }}</p>
              <p v-if="item.adminReason" class="text-xs text-muted-foreground">处理说明：{{ item.adminReason }}</p>
              <p class="text-xs text-muted-foreground"><LocalTime :value="item.createdAt" /></p>
            </div>
          </div>
          <p v-else class="mt-3 text-sm text-muted-foreground">暂无相关申诉。</p>
        </Card>
      </div>

      <section>
        <h3 class="font-semibold">原帖作者核验</h3>
        <div v-if="verificationForms.length" class="mt-3 grid gap-4 xl:grid-cols-2">
          <Card v-for="item in verificationForms" :key="verificationKey(item.audit)" class="p-5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div>
                <div class="font-medium">{{ item.audit.verification.resourceType }} · {{ item.audit.verification.resourceId }}</div>
                <p class="mt-1 text-xs text-muted-foreground">{{ item.audit.verification.sourceUrl || '未提供来源地址' }}</p>
              </div>
              <Badge variant="outline">{{ sourceAuthorStatusLabel(item.audit.verification.status) }}</Badge>
            </div>
            <div class="mt-4 grid gap-3 sm:grid-cols-2">
              <label class="space-y-1.5 text-sm">
                <span>核验状态</span>
                <Select v-model="item.draft.status">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="not_submitted">未提交</SelectItem>
                    <SelectItem value="pending">待核验</SelectItem>
                    <SelectItem value="verified">已验证</SelectItem>
                    <SelectItem value="mismatch">作者不一致</SelectItem>
                    <SelectItem value="expired">已过期</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label class="space-y-1.5 text-sm">
                <span>实际外部用户 ID</span>
                <Input v-model="item.draft.actualExternalUserId" />
              </label>
              <label class="space-y-1.5 text-sm">
                <span>核验方式</span>
                <Input v-model="item.draft.verificationMethod" />
              </label>
              <label class="space-y-1.5 text-sm">
                <span>过期时间</span>
                <Input v-model="item.draft.expiresAt" type="datetime-local" />
              </label>
              <label class="space-y-1.5 text-sm sm:col-span-2">
                <span>失败原因</span>
                <Input v-model="item.draft.failureReason" />
              </label>
            </div>
            <div class="mt-4 flex flex-wrap items-center justify-between gap-2">
              <span class="text-xs text-muted-foreground">审计事件 {{ item.audit.events.length }} 条 · 版本 {{ item.audit.verification.version }}</span>
              <Button size="sm" :disabled="updateVerificationMutation.isPending.value" @click="updateVerification(item.audit, item.draft)">保存核验</Button>
            </div>
          </Card>
        </div>
        <p v-else class="mt-3 text-sm text-muted-foreground">该用户暂无可核验的车源或 API 服务。</p>
      </section>
    </template>
  </section>
</template>
