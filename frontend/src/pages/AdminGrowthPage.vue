<script setup lang="ts">
import { computed, ref } from 'vue'
import { Activity, CalendarPlus, Clock3, Gift, RefreshCw, ShoppingCart, TrendingUp, UserCheck, Users } from 'lucide-vue-next'
import GrowthTrendChart from '@/components/growth/GrowthTrendChart.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { GrowthWindowDays } from '@/lib/growthBackend'
import { useAdminGrowthOverview } from '@/queries/useGrowthQueries'

const windowDays = ref<GrowthWindowDays>(30)
const growthQuery = useAdminGrowthOverview(windowDays)
const overview = computed(() => growthQuery.data.value)
const summary = computed(() => overview.value?.summary)

function numberText(value: number | null | undefined) {
  return (value ?? 0).toLocaleString('zh-CN')
}

function percentText(value: number | null | undefined) {
  return value == null ? '观察中' : `${(value * 100).toFixed(value >= 0.1 ? 1 : 2)}%`
}

function hoursText(value: number | null | undefined) {
  if (value == null) return '观察中'
  if (value < 24) return `${value.toFixed(1)} 小时`
  return `${(value / 24).toFixed(1)} 天`
}

function dateText(value: string) {
  const date = new Date(`${value}T00:00:00+08:00`)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', timeZone: 'Asia/Shanghai' }).format(date)
}

function dateTimeText(value: string | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'Asia/Shanghai',
  }).format(date)
}

const kpis = computed(() => [
  { label: `${windowDays.value} 日新增`, value: numberText(summary.value?.newUsersInWindow), hint: `今日 ${numberText(summary.value?.newUsersToday)} 人`, icon: CalendarPlus, tone: 'blue' },
  { label: '累计有效用户', value: numberText(summary.value?.cumulativeEffectiveUsers), hint: '不含已归档账号', icon: Users, tone: 'cyan' },
  { label: '7 日激活率', value: percentText(summary.value?.activationRate), hint: `${numberText(summary.value?.activatedUsers)} 人完成高意图行为`, icon: UserCheck, tone: 'green' },
  { label: '激活耗时中位数', value: hoursText(summary.value?.medianActivationHours), hint: '仅统计已激活成熟 Cohort', icon: Clock3, tone: 'amber' },
  { label: 'D1 留存', value: percentText(summary.value?.d1RetentionRate), hint: '按成熟注册 Cohort 加权', icon: TrendingUp, tone: 'violet' },
  { label: 'D7 留存', value: percentText(summary.value?.d7RetentionRate), hint: '未满 7 天不计入', icon: TrendingUp, tone: 'rose' },
  { label: 'DAU', value: numberText(summary.value?.dau), hint: '今日有效活跃用户', icon: Activity, tone: 'green' },
  { label: 'WAU / MAU', value: `${numberText(summary.value?.wau)} / ${numberText(summary.value?.mau)}`, hint: '近 7 日 / 30 日', icon: Activity, tone: 'blue' },
])

const attributionMax = computed(() => Math.max(1, ...(overview.value?.attribution ?? []).map(item => item.registrations)))
const retentionRows = computed(() => [...(overview.value?.retentionCohorts ?? [])].reverse())
const sourceTypeLabels: Record<string, string> = {
  campaign: '推广活动',
  referral: '外部引荐',
  direct: '直接访问',
  unknown: '历史未知',
  other: '其他来源',
}

function attributionLabel(source: string, medium?: string, campaign?: string) {
  return [source, medium, campaign].filter(Boolean).join(' · ')
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="用户增长" description="以平台注册、业务行为和认证活跃为准，查看增长、激活与留存。">
      <template #action>
        <div class="flex flex-wrap items-center justify-between gap-3 md:justify-end">
          <span class="text-xs text-muted-foreground">更新于 {{ dateTimeText(overview?.generatedAt) }}</span>
          <Tabs v-model="windowDays">
            <TabsList class="grid w-[210px] grid-cols-3" aria-label="增长统计周期">
              <TabsTrigger :value="7">7 天</TabsTrigger>
              <TabsTrigger :value="30">30 天</TabsTrigger>
              <TabsTrigger :value="90">90 天</TabsTrigger>
            </TabsList>
          </Tabs>
          <Button as-child variant="outline"><RouterLink to="/admin/growth-promotions"><Gift class="h-4 w-4" />增长推广</RouterLink></Button>
          <Button variant="outline" size="icon" title="刷新增长数据" aria-label="刷新增长数据" :disabled="growthQuery.isFetching.value" @click="growthQuery.refetch()">
            <RefreshCw class="h-4 w-4" :class="growthQuery.isFetching.value ? 'animate-spin' : ''" />
          </Button>
        </div>
      </template>
    </PageTitle>

    <div v-if="growthQuery.isLoading.value" class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <SkeletonBlock v-for="item in 8" :key="item" :lines="2" />
    </div>
    <ErrorState v-else-if="growthQuery.isError.value" title="增长数据加载失败" description="后台暂时无法读取权威增长统计。" @retry="growthQuery.refetch()" />

    <template v-else-if="overview">
      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="用户增长核心指标">
        <Card v-for="item in kpis" :key="item.label" class="flex min-h-[112px] items-start gap-3 p-4">
          <span class="grid h-9 w-9 shrink-0 place-items-center rounded-md" :class="`growth-tone-${item.tone}`"><component :is="item.icon" class="h-4 w-4" /></span>
          <dl class="min-w-0"><dt class="text-xs text-muted-foreground">{{ item.label }}</dt><dd class="mt-2 text-xl font-semibold">{{ item.value }}</dd><small class="mt-1 block text-xs text-muted-foreground">{{ item.hint }}</small></dl>
        </Card>
      </section>

      <Card class="p-5">
        <div class="mb-4 flex flex-wrap items-end justify-between gap-2"><div><h2 class="font-semibold">注册与活跃趋势</h2><p class="mt-1 text-xs text-muted-foreground">上海自然日口径；柱形为新增注册，折线为有效认证活跃与累计用户。</p></div><Badge variant="secondary">{{ overview.windowDays }} 天</Badge></div>
        <div class="overflow-x-auto pb-1"><GrowthTrendChart :registration="overview.registrationTrend" :activity="overview.activityTrend" /></div>
      </Card>

      <div class="grid gap-5 xl:grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]">
        <Card class="p-5">
          <div class="flex items-center justify-between gap-3"><div><h2 class="font-semibold">注册来源</h2><p class="mt-1 text-xs text-muted-foreground">首触归因；历史账号明确归入“历史未知”。</p></div><span class="text-xs text-muted-foreground">{{ numberText(summary?.newUsersInWindow) }} 人</span></div>
          <div v-if="overview.attribution.length" class="mt-5 space-y-4">
            <div v-for="item in overview.attribution" :key="`${item.sourceType}-${item.source}-${item.medium}-${item.campaign}`" class="grid gap-2 sm:grid-cols-[130px_minmax(0,1fr)_72px] sm:items-center">
              <div><Badge variant="outline">{{ sourceTypeLabels[item.sourceType] ?? item.sourceType }}</Badge></div>
              <div class="min-w-0"><div class="truncate text-sm">{{ attributionLabel(item.source, item.medium, item.campaign) }}</div><div class="mt-1.5 h-2 overflow-hidden rounded-full bg-muted"><i class="block h-full rounded-full bg-primary" :style="{ width: `${Math.max(3, item.registrations / attributionMax * 100)}%` }" /></div></div>
              <div class="text-right text-sm font-medium">{{ numberText(item.registrations) }} <small class="font-normal text-muted-foreground">{{ percentText(item.share) }}</small></div>
            </div>
          </div>
          <p v-else class="mt-8 text-center text-sm text-muted-foreground">当前周期没有新增注册。</p>
        </Card>

        <div class="grid gap-5">
          <Card class="p-5">
            <h2 class="font-semibold">7 日激活</h2><p class="mt-1 text-xs text-muted-foreground">只纳入已完成 7 天观察期的注册用户。</p>
            <dl class="mt-5 grid grid-cols-2 gap-4">
              <div><dt class="text-xs text-muted-foreground">成熟 Cohort</dt><dd class="mt-1 text-2xl font-semibold">{{ numberText(overview.activation.cohortUsers) }}</dd></div>
              <div><dt class="text-xs text-muted-foreground">整体激活</dt><dd class="mt-1 text-2xl font-semibold">{{ percentText(overview.activation.activationRate) }}</dd></div>
              <div><dt class="text-xs text-muted-foreground">买家激活</dt><dd class="mt-1 text-lg font-semibold">{{ percentText(overview.activation.buyerActivationRate) }}</dd><small class="text-xs text-muted-foreground">{{ numberText(overview.activation.buyerActivatedUsers) }} 人</small></div>
              <div><dt class="text-xs text-muted-foreground">卖家激活</dt><dd class="mt-1 text-lg font-semibold">{{ percentText(overview.activation.sellerActivationRate) }}</dd><small class="text-xs text-muted-foreground">{{ numberText(overview.activation.sellerActivatedUsers) }} 人</small></div>
            </dl>
          </Card>
          <Card class="p-5">
            <div class="flex items-center gap-2"><ShoppingCart class="h-4 w-4 text-primary" /><h2 class="font-semibold">完成交易</h2></div>
            <dl class="mt-5 grid grid-cols-2 gap-4"><div><dt class="text-xs text-muted-foreground">拼车交易</dt><dd class="mt-1 text-2xl font-semibold">{{ numberText(summary?.completedCarpoolTransactions) }}</dd></div><div><dt class="text-xs text-muted-foreground">API 交易</dt><dd class="mt-1 text-2xl font-semibold">{{ numberText(summary?.completedApiTransactions) }}</dd></div></dl>
          </Card>
        </div>
      </div>

      <Card class="overflow-hidden p-0">
        <div class="border-b border-border px-5 py-4"><h2 class="font-semibold">注册 Cohort 留存</h2><p class="mt-1 text-xs text-muted-foreground">按注册日追踪次日与第 7 日是否再次产生有效活跃；未到观察日显示“观察中”。</p></div>
        <Alert v-if="summary?.d7RetentionRate == null" class="m-4"><Clock3 class="h-4 w-4" /><AlertTitle>D7 留存仍在观察</AlertTitle><AlertDescription>所选周期内尚无满足 7 天观察期的注册 Cohort。</AlertDescription></Alert>
        <div class="max-h-[520px] overflow-auto">
          <Table class="min-w-[700px]">
            <TableHeader class="sticky top-0 bg-card"><TableRow><TableHead>注册日期</TableHead><TableHead class="text-right">注册人数</TableHead><TableHead class="text-right">D1 留存人数</TableHead><TableHead class="text-right">D1 留存率</TableHead><TableHead class="text-right">D7 留存人数</TableHead><TableHead class="text-right">D7 留存率</TableHead></TableRow></TableHeader>
            <TableBody>
              <TableRow v-if="retentionRows.length === 0"><TableCell colspan="6" class="h-24 text-center text-muted-foreground">当前周期没有注册 Cohort 数据。</TableCell></TableRow>
              <TableRow v-for="cohort in retentionRows" :key="cohort.cohortDate">
                <TableCell class="font-medium">{{ dateText(cohort.cohortDate) }}</TableCell><TableCell class="text-right">{{ numberText(cohort.registeredUsers) }}</TableCell>
                <TableCell class="text-right">{{ cohort.d1RetainedUsers == null ? '—' : numberText(cohort.d1RetainedUsers) }}</TableCell><TableCell class="text-right"><Badge v-if="cohort.d1Rate == null" variant="secondary">观察中</Badge><span v-else>{{ percentText(cohort.d1Rate) }}</span></TableCell>
                <TableCell class="text-right">{{ cohort.d7RetainedUsers == null ? '—' : numberText(cohort.d7RetainedUsers) }}</TableCell><TableCell class="text-right"><Badge v-if="cohort.d7Rate == null" variant="secondary">观察中</Badge><span v-else>{{ percentText(cohort.d7Rate) }}</span></TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.growth-tone-blue { background: #eff6ff; color: #2563eb; }
.growth-tone-cyan { background: #ecfeff; color: #0891b2; }
.growth-tone-green { background: #ecfdf5; color: #059669; }
.growth-tone-amber { background: #fffbeb; color: #d97706; }
.growth-tone-violet { background: #f5f3ff; color: #7c3aed; }
.growth-tone-rose { background: #fff1f2; color: #e11d48; }
</style>
