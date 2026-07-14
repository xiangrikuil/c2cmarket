<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { AlertTriangle, ArrowRight, BarChart3, CarFront, Clock3, Code2, FileCheck2, Gauge, ListChecks, MessageSquareWarning, ScrollText, ShieldCheck } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { useAdminSectionRows } from '@/queries/useMarketQueries'
import type { AdminRow, AdminSection } from '@/lib/api'

type TaskRow = AdminRow & { section: AdminSection, sectionLabel: string }

const reportsQuery = useAdminSectionRows('reports')
const carpoolsQuery = useAdminSectionRows('carpools')
const apiServicesQuery = useAdminSectionRows('api-services')
const ordersQuery = useAdminSectionRows('trade-intents')
const feedbackQuery = useAdminSectionRows('feedback')
const logsQuery = useAdminSectionRows('logs')

const isLoading = computed(() => [reportsQuery, carpoolsQuery, apiServicesQuery, ordersQuery, feedbackQuery, logsQuery].some(query => query.isLoading.value))

function actionable(row: AdminRow) {
  return !/已完成|已关闭|已通过|已恢复|已处理|正常|completed|closed/i.test(row.status)
}

function highRisk(row: AdminRow) {
  return /高风险|纠纷|举报|封禁|异常|超时|未解决|危险/i.test(`${row.risk} ${row.status}`)
}

function riskRank(row: AdminRow) {
  if (highRisk(row)) return 0
  if (/待|审核|复核|处理中|submitted|pending/i.test(row.status)) return 1
  return 2
}

const actionableRows = computed<TaskRow[]>(() => [
  ...(reportsQuery.data.value ?? []).filter(actionable).map(row => ({ ...row, section: 'reports' as const, sectionLabel: '举报纠纷' })),
  ...(carpoolsQuery.data.value ?? []).filter(actionable).map(row => ({ ...row, section: 'carpools' as const, sectionLabel: '车源异常' })),
  ...(apiServicesQuery.data.value ?? []).filter(actionable).map(row => ({ ...row, section: 'api-services' as const, sectionLabel: '服务审核' })),
  ...(ordersQuery.data.value ?? []).filter(actionable).map(row => ({ ...row, section: 'trade-intents' as const, sectionLabel: '订单追踪' })),
  ...(feedbackQuery.data.value ?? []).filter(actionable).map(row => ({ ...row, section: 'feedback' as const, sectionLabel: '问题反馈' })),
].sort((a, b) => riskRank(a) - riskRank(b)))
const tasks = computed(() => actionableRows.value.slice(0, 10))

const allRows = computed(() => [
  ...(reportsQuery.data.value ?? []),
  ...(carpoolsQuery.data.value ?? []),
  ...(apiServicesQuery.data.value ?? []),
  ...(ordersQuery.data.value ?? []),
  ...(feedbackQuery.data.value ?? []),
])
const recentLogs = computed(() => (logsQuery.data.value ?? []).slice(0, 6))
const overviewStats = computed(() => [
  { label: '全部待处理', value: actionableRows.value.length, hint: '五类工作队列', icon: ListChecks, tone: 'blue' },
  { label: '高风险举报/纠纷', value: actionableRows.value.filter(highRisk).length, hint: '优先进入处理', icon: MessageSquareWarning, tone: 'red' },
  { label: '即将/已经超时', value: actionableRows.value.filter(row => /超时|SLA|临近/i.test(`${row.risk} ${row.secondary}`)).length, hint: '来自队列上下文', icon: Clock3, tone: 'amber' },
  { label: '待审核服务', value: (apiServicesQuery.data.value ?? []).filter(actionable).length, hint: 'API 服务审核', icon: Code2, tone: 'cyan' },
  { label: '最近管理动作', value: recentLogs.value.length, hint: '只读审计记录', icon: ScrollText, tone: 'violet' },
  { label: '当前查询记录', value: allRows.value.length, hint: '非平台经营总量', icon: BarChart3, tone: 'emerald' },
])
const marketHealth = computed(() => [
  { label: '车源记录', value: carpoolsQuery.data.value?.length ?? 0 },
  { label: 'API 服务记录', value: apiServicesQuery.data.value?.length ?? 0 },
  { label: '订单记录', value: ordersQuery.data.value?.length ?? 0 },
  { label: '治理记录', value: reportsQuery.data.value?.length ?? 0 },
])
const queueDistribution = computed(() => [
  { label: '举报纠纷', value: (reportsQuery.data.value ?? []).filter(actionable).length, tone: 'red' },
  { label: '车源异常', value: (carpoolsQuery.data.value ?? []).filter(actionable).length, tone: 'amber' },
  { label: '服务审核', value: (apiServicesQuery.data.value ?? []).filter(actionable).length, tone: 'cyan' },
  { label: '订单追踪', value: (ordersQuery.data.value ?? []).filter(actionable).length, tone: 'blue' },
  { label: '问题反馈', value: (feedbackQuery.data.value ?? []).filter(actionable).length, tone: 'violet' },
])
const maxQueueValue = computed(() => Math.max(1, ...queueDistribution.value.map(item => item.value)))
</script>

<template>
  <div class="admin-dashboard-page space-y-5">
    <header class="admin-reference-heading"><div><h1>管理工作台</h1><p>集中查看审核、举报、纠纷与异常队列，按真实风险上下文排序处理。</p></div><div class="flex items-center gap-3"><span>当前查询 {{ allRows.length }} 条记录</span><RouterLink to="/admin/logs"><Button variant="outline">查看审计日志</Button></RouterLink></div></header>

    <section class="admin-reference-stats" aria-label="管理工作台统计">
      <Card v-for="item in overviewStats" :key="item.label" class="p-4"><span :class="`is-${item.tone}`"><component :is="item.icon" /></span><dl><dt>{{ item.label }}</dt><dd>{{ isLoading ? '—' : item.value }}</dd><small>{{ item.hint }}</small></dl></Card>
    </section>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_340px] xl:items-start">
      <Card class="p-0">
        <div class="flex items-center justify-between border-b border-border px-5 py-4"><div><h2 class="font-semibold">管理待办队列 · 全部待处理</h2><p class="mt-1 text-xs text-muted-foreground">共 {{ actionableRows.length }} 条；按风险排序展示前 10 条，进入对应队列查看完整对象和证据</p></div><Badge variant="secondary">{{ tasks.length }} / {{ actionableRows.length }}</Badge></div>
        <SkeletonTable v-if="isLoading" :rows="6" :columns="4" class="rounded-none border-0" />
        <EmptyState v-else-if="tasks.length === 0" title="当前没有管理待办" description="新的审核、举报、纠纷或异常交易会进入这里。" />
        <div v-else class="divide-y divide-border">
          <RouterLink v-for="row in tasks" :key="`${row.section}-${row.id}`" :to="`/admin/${row.section}`" class="admin-task-row grid gap-3 px-5 py-4 transition hover:bg-accent/60 md:grid-cols-[110px_minmax(0,1fr)_160px_auto] md:items-center" :class="highRisk(row) ? 'admin-task-row--risk' : 'admin-task-row--normal'">
            <div><Badge :variant="highRisk(row) ? 'destructive' : 'secondary'">{{ row.sectionLabel }}</Badge></div>
            <div class="min-w-0"><div class="truncate font-medium">{{ row.primary }}</div><div class="mt-1 truncate text-xs text-muted-foreground">{{ row.secondary }} · {{ row.owner }}</div></div>
            <div><StatusBadge :status="row.status" :label="row.status" /><div class="mt-1 truncate text-xs text-muted-foreground">{{ row.risk }}</div></div>
            <span class="inline-flex items-center gap-1 text-xs font-medium text-primary">继续处理<ArrowRight class="h-3.5 w-3.5" /></span>
          </RouterLink>
        </div>
      </Card>

      <div class="space-y-5">
        <Card class="p-5"><div class="flex items-center gap-2 font-semibold"><FileCheck2 class="h-4 w-4 text-primary" />平台记录概况</div><dl class="mt-4 grid grid-cols-2 gap-3"><div v-for="item in marketHealth" :key="item.label" class="rounded-lg bg-muted/40 p-3"><dt class="text-xs text-muted-foreground">{{ item.label }}</dt><dd class="mt-1 text-xl font-semibold">{{ item.value }}</dd></div></dl><p class="mt-4 text-xs leading-5 text-muted-foreground">以上仅为当前管理查询返回的记录，不等同平台经营指标。</p></Card>
        <Card class="p-5"><div class="flex items-center gap-2 font-semibold"><Gauge class="h-4 w-4 text-primary" />快捷入口</div><div class="admin-reference-shortcuts"><RouterLink to="/admin/carpools"><CarFront /><span>车源复核</span></RouterLink><RouterLink to="/admin/api-services"><Code2 /><span>服务审核</span></RouterLink><RouterLink to="/admin/reports"><MessageSquareWarning /><span>举报纠纷</span></RouterLink><RouterLink to="/admin/logs"><ScrollText /><span>审计日志</span></RouterLink></div></Card>
        <Card class="p-5"><div class="flex items-center gap-2 font-semibold"><Clock3 class="h-4 w-4 text-primary" />最近审计动作</div><div v-if="recentLogs.length" class="mt-4 space-y-3"><RouterLink v-for="row in recentLogs" :key="row.id" to="/admin/logs" class="block border-b border-border pb-3 last:border-0 last:pb-0"><div class="text-sm font-medium">{{ row.primary }}</div><div class="mt-1 text-xs text-muted-foreground">{{ row.secondary }} · {{ row.owner }}</div></RouterLink></div><p v-else class="mt-4 text-sm text-muted-foreground">暂无审计记录。</p></Card>
        <Card v-if="allRows.some(highRisk)" class="border-destructive/25 p-5"><div class="flex gap-3"><AlertTriangle class="mt-0.5 h-5 w-5 shrink-0 text-destructive" /><div><h2 class="font-semibold">最近异常</h2><p class="mt-2 text-sm text-muted-foreground">检测到 {{ allRows.filter(highRisk).length }} 条高风险或异常上下文，请优先从待办队列进入处理。</p></div></div></Card>
      </div>
    </div>

    <section class="admin-reference-insights">
      <Card class="p-5"><div class="flex items-center gap-2 font-semibold"><BarChart3 class="h-4 w-4 text-primary" />待办队列分布</div><p class="mt-2 text-xs text-muted-foreground">按当前五类真实队列的待处理数量展示。</p><div class="admin-reference-bars"><div v-for="item in queueDistribution" :key="item.label"><span>{{ item.label }}</span><div><i :class="`is-${item.tone}`" :style="{ width: `${Math.max(6, (item.value / maxQueueValue) * 100)}%` }"></i></div><strong>{{ item.value }}</strong></div></div></Card>
      <Card class="p-5"><div class="flex items-center gap-2 font-semibold"><ShieldCheck class="h-4 w-4 text-primary" />风险处置概况</div><p class="mt-2 text-xs text-muted-foreground">只反映当前查询记录的风险标记，不生成展示型假趋势。</p><dl class="admin-reference-risk-summary"><div><dt>高风险上下文</dt><dd>{{ allRows.filter(highRisk).length }}</dd></div><div><dt>全部待处理</dt><dd>{{ actionableRows.length }}</dd></div><div><dt>当前已无动作记录</dt><dd>{{ Math.max(0, allRows.length - actionableRows.length) }}</dd></div></dl></Card>
    </section>
  </div>
</template>
