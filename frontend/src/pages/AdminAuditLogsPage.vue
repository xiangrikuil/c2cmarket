<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Eye, FilterX, RefreshCw, Search } from 'lucide-vue-next'
import PageTitle from '@/components/market/PageTitle.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useCursorPagination } from '@/composables/useCursorPagination'
import type { AdminAuditLogEntry, AdminAuditLogFilters } from '@/lib/adminAuditBackend'
import { useAdminAuditLogsPage } from '@/queries/useAdminAuditQueries'

const search = ref('')
const sourceKind = ref<AdminAuditLogFilters['sourceKind'] | ''>('')
const domain = ref<AdminAuditLogFilters['domain'] | ''>('')
const action = ref('')
const actorKind = ref<AdminAuditLogFilters['actorKind'] | ''>('')
const actorUserId = ref('')
const targetType = ref('')
const targetId = ref('')
const outcome = ref<AdminAuditLogFilters['outcome'] | ''>('')
const from = ref('')
const to = ref('')
const detailEntry = ref<AdminAuditLogEntry | null>(null)

const pagination = useCursorPagination([
  search,
  sourceKind,
  domain,
  action,
  actorKind,
  actorUserId,
  targetType,
  targetId,
  outcome,
  from,
  to,
], 30)

function dateTimeFilter(value: string) {
  if (!value) return undefined
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toISOString()
}

const filters = computed<AdminAuditLogFilters>(() => ({
  search: search.value.trim() || undefined,
  sourceKind: sourceKind.value || undefined,
  domain: domain.value || undefined,
  action: action.value.trim() || undefined,
  actorKind: actorKind.value || undefined,
  actorUserId: actorUserId.value.trim() || undefined,
  targetType: targetType.value.trim() || undefined,
  targetId: targetId.value.trim() || undefined,
  outcome: outcome.value || undefined,
  from: dateTimeFilter(from.value),
  to: dateTimeFilter(to.value),
}))
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useAdminAuditLogsPage(filters, pageRequest)
const entries = computed(() => pageQuery.data.value?.items ?? [])
const hasFilters = computed(() => Object.values(filters.value).some(Boolean))

function clearFilters() {
  search.value = ''
  sourceKind.value = ''
  domain.value = ''
  action.value = ''
  actorKind.value = ''
  actorUserId.value = ''
  targetType.value = ''
  targetId.value = ''
  outcome.value = ''
  from.value = ''
  to.value = ''
}

function outcomeLabel(value: string) {
  if (value === 'succeeded') return '已完成'
  if (value === 'status_changed') return '状态已变更'
  if (value === 'accessed') return '已访问'
  return value
}

function outcomeVariant(value: string) {
  if (value === 'succeeded') return 'verified' as const
  if (value === 'status_changed') return 'capability' as const
  if (value === 'accessed') return 'secondary' as const
  return 'secondary' as const
}

function actorLabel(item: AdminAuditLogEntry) {
  return item.actorUsername || item.actorUserId || item.actorKind
}

function targetLabel(item: AdminAuditLogEntry) {
  return item.targetLabel || item.targetId
}

function internalDetailPath(item: AdminAuditLogEntry) {
  return item.detailPath?.startsWith('/') ? item.detailPath : undefined
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="审计日志" description="统一查看系统与管理员操作结果；记录只读，使用游标逐页读取。不同来源采用各自的保留策略，本页不代表永久完整历史。" />

    <Card class="p-4 sm:p-5">
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <label class="grid gap-1.5 text-xs text-muted-foreground xl:col-span-2">
          全文搜索
          <span class="relative"><Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" /><Input v-model="search" class="pl-9" placeholder="摘要、动作、人员、对象或请求 ID" /></span>
        </label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">来源类型<Input v-model="sourceKind" placeholder="例如 admin / moderation" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">业务领域<Input v-model="domain" placeholder="例如 governance" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">动作标识<Input v-model="action" placeholder="例如 user.status_changed" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">执行者类型<Input v-model="actorKind" placeholder="user / admin / system" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">执行者用户 ID<Input v-model="actorUserId" placeholder="精确匹配" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">执行结果<Input v-model="outcome" placeholder="succeeded / status_changed / accessed" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">对象类型<Input v-model="targetType" placeholder="例如 user" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">对象 ID<Input v-model="targetId" placeholder="精确匹配" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">起始时间<Input v-model="from" type="datetime-local" /></label>
        <label class="grid gap-1.5 text-xs text-muted-foreground">结束时间<Input v-model="to" type="datetime-local" /></label>
      </div>
      <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
        <p class="text-xs text-muted-foreground">筛选值按后端统一日志字段传递；修改筛选后自动回到第一页。</p>
        <div class="flex gap-2">
          <Button size="sm" variant="outline" :disabled="!hasFilters" @click="clearFilters"><FilterX class="h-4 w-4" />清空筛选</Button>
          <Button size="sm" variant="outline" :disabled="pageQuery.isFetching.value" @click="pageQuery.refetch()"><RefreshCw class="h-4 w-4" />刷新</Button>
        </div>
      </div>
    </Card>

    <SkeletonTable v-if="pageQuery.isPending.value" :rows="8" :columns="7" />
    <ErrorState v-else-if="pageQuery.isError.value" title="审计日志读取失败" description="真实接口失败不会切换到演示数据，请稍后重试。" @retry="pageQuery.refetch()" />
    <EmptyState v-else-if="entries.length === 0" title="没有匹配的审计记录" description="可调整筛选条件，或返回上一页继续查看。" />
    <SoftTable v-else class="[&_table]:min-w-[1080px]" :columns="['时间', '动作', '来源 / 领域', '执行者', '对象', '结果', '摘要', '详情']">
      <tr v-for="item in entries" :key="item.id">
        <td class="whitespace-nowrap text-xs text-muted-foreground"><LocalTime :value="item.createdAt" seconds /></td>
        <td>
          <div class="font-medium">{{ item.actionLabel || item.action }}</div>
          <div class="mt-1 max-w-56 truncate font-mono text-[11px] text-muted-foreground" :title="item.action">{{ item.action }}</div>
        </td>
        <td class="text-sm"><div>{{ item.domain }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.sourceKind }}</div></td>
        <td class="text-sm"><div>{{ actorLabel(item) }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.actorKind }}</div></td>
        <td class="text-sm"><div class="max-w-44 truncate" :title="targetLabel(item)">{{ targetLabel(item) }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.targetType }} · {{ item.targetId }}</div></td>
        <td><Badge :variant="outcomeVariant(item.outcome)">{{ outcomeLabel(item.outcome) }}</Badge></td>
        <td class="max-w-80 text-sm leading-5 text-muted-foreground"><p class="line-clamp-2">{{ item.summary }}</p></td>
        <td><Button size="sm" variant="outline" @click="detailEntry = item"><Eye class="h-4 w-4" />查看</Button></td>
      </tr>
      <template #footer>
        <CursorTablePagination
          :page="pagination.page.value"
          :item-count="entries.length"
          :has-next-page="Boolean(pageQuery.data.value?.nextCursor)"
          :loading="pageQuery.isFetching.value"
          @previous="pagination.previous"
          @next="pagination.next(pageQuery.data.value?.nextCursor)"
        />
      </template>
    </SoftTable>

    <Dialog :open="Boolean(detailEntry)" @update:open="value => { if (!value) detailEntry = null }">
      <DialogContent v-if="detailEntry" class="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ detailEntry.actionLabel || detailEntry.action }}</DialogTitle>
          <DialogDescription>{{ detailEntry.summary }}</DialogDescription>
        </DialogHeader>
        <dl class="grid gap-x-5 gap-y-3 text-sm sm:grid-cols-2">
          <div><dt class="text-xs text-muted-foreground">日志 ID</dt><dd class="mt-1 break-all font-mono text-xs">{{ detailEntry.id }}</dd></div>
          <div><dt class="text-xs text-muted-foreground">请求 ID</dt><dd class="mt-1 break-all font-mono text-xs">{{ detailEntry.requestId }}</dd></div>
          <div><dt class="text-xs text-muted-foreground">来源</dt><dd class="mt-1">{{ detailEntry.sourceKind }} · {{ detailEntry.domain }}</dd></div>
          <div><dt class="text-xs text-muted-foreground">结果</dt><dd class="mt-1"><Badge :variant="outcomeVariant(detailEntry.outcome)">{{ outcomeLabel(detailEntry.outcome) }}</Badge></dd></div>
          <div><dt class="text-xs text-muted-foreground">执行者</dt><dd class="mt-1">{{ actorLabel(detailEntry) }}（{{ detailEntry.actorKind }}）</dd></div>
          <div><dt class="text-xs text-muted-foreground">对象</dt><dd class="mt-1">{{ targetLabel(detailEntry) }}（{{ detailEntry.targetType }} · {{ detailEntry.targetId }}）</dd></div>
          <div><dt class="text-xs text-muted-foreground">动作标识</dt><dd class="mt-1 break-all font-mono text-xs">{{ detailEntry.action }}</dd></div>
          <div><dt class="text-xs text-muted-foreground">发生时间</dt><dd class="mt-1"><LocalTime :value="detailEntry.createdAt" seconds /></dd></div>
        </dl>
        <RouterLink v-if="internalDetailPath(detailEntry)" :to="internalDetailPath(detailEntry)!" class="text-sm font-medium text-primary hover:underline">打开关联对象</RouterLink>
      </DialogContent>
    </Dialog>
  </div>
</template>
