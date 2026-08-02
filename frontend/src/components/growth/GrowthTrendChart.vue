<script setup lang="ts">
import { computed } from 'vue'
import type { GrowthActivityTrendPoint, GrowthRegistrationTrendPoint } from '@/api/generated/openapi'
import { AxisType, CurveType, FitMode, Position, TextAlign } from '@unovis/ts'
import { VisArea, VisAxis, VisCrosshair, VisGroupedBar, VisLine, VisTooltip, VisXYContainer } from '@unovis/vue'
import { ChartContainer, componentToString, type ChartConfig } from '@/components/ui/chart'
import GrowthTrendTooltip from './GrowthTrendTooltip.vue'

const props = defineProps<{
  registration: GrowthRegistrationTrendPoint[]
  activity: GrowthActivityTrendPoint[]
}>()

type TrendPoint = GrowthRegistrationTrendPoint & {
  activeUsers: number
  index: number
}

const chartConfig = {
  newUsers: { label: '新增注册', color: 'var(--chart-1)' },
  activeUsers: { label: '活跃用户', color: 'var(--chart-3)' },
  cumulativeUsers: { label: '累计用户', color: 'var(--chart-2)' },
} satisfies ChartConfig

const activityByDate = computed(() => new Map(props.activity.map(point => [point.date, point.activeUsers])))
const chartData = computed<TrendPoint[]>(() => props.registration.map((point, index) => ({
  ...point,
  activeUsers: activityByDate.value.get(point.date) ?? 0,
  index,
})))
const xDomain = computed<[number, number]>(() => chartData.value.length <= 1 ? [-0.5, 0.5] : [-0.35, chartData.value.length - 0.65])
const dailyMax = computed(() => Math.max(1, ...chartData.value.flatMap(point => [point.newUsers, point.activeUsers])))
const cumulativeMax = computed(() => Math.max(1, ...chartData.value.map(point => point.cumulativeUsers)))
const tickValues = computed(() => {
  const count = chartData.value.length
  if (count <= 7) return chartData.value.map(point => point.index)
  const step = count <= 30 ? 5 : 15
  return chartData.value.filter((_, index) => index === 0 || index === count - 1 || index % step === 0).map(point => point.index)
})

const indexAccessor = (point: TrendPoint) => point.index
const newUsersAccessor = (point: TrendPoint) => point.newUsers
const activeUsersAccessor = (point: TrendPoint) => point.activeUsers
const cumulativeAccessor = (point: TrendPoint) => point.cumulativeUsers
const idAccessor = (point: TrendPoint) => point.date
const tooltipTemplate = componentToString(chartConfig, GrowthTrendTooltip)

function formatDateTick(tick: number | Date) {
  if (typeof tick !== 'number') return ''
  const value = chartData.value[Math.round(tick)]?.date ?? ''
  return value.slice(5)
}

function formatNumberTick(tick: number | Date) {
  return typeof tick === 'number' ? tick.toLocaleString('zh-CN') : ''
}
</script>

<template>
  <ChartContainer :config="chartConfig" class="growth-trend-chart h-auto w-full justify-start" :cursor="true">
    <div class="mb-3 flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted-foreground">
      <span class="flex items-center gap-2"><i class="h-2.5 w-2.5 rounded-sm bg-[var(--color-newUsers)]" />新增注册</span>
      <span class="flex items-center gap-2"><i class="h-0.5 w-4 bg-[var(--color-activeUsers)]" />活跃用户</span>
      <span class="flex items-center gap-2"><i class="h-0.5 w-4 bg-[var(--color-cumulativeUsers)]" />累计用户</span>
    </div>

    <div class="h-[230px] min-w-[520px]">
      <VisXYContainer :data="chartData" :height="230" :margin="{ top: 10, right: 18, bottom: 34, left: 46 }" :duration="0" :x-domain="xDomain" :y-domain="[0, Math.ceil(dailyMax * 1.15)]" :prevent-empty-domain="true" aria-label="新增注册与活跃用户趋势">
        <VisGroupedBar :x="indexAccessor" :y="newUsersAccessor" :id="idAccessor" color="var(--color-newUsers)" :group-max-width="22" :group-padding="0.55" :rounded-corners="3" />
        <VisLine :x="indexAccessor" :y="activeUsersAccessor" :id="idAccessor" color="var(--color-activeUsers)" :line-width="2.5" :curve-type="CurveType.MonotoneX" />
        <VisAxis :type="AxisType.Y" :position="Position.Left" :grid-line="true" :tick-line="false" :domain-line="false" :num-ticks="5" :tick-format="formatNumberTick" tick-text-font-size="11px" tick-text-color="var(--muted-foreground)" />
        <VisAxis :type="AxisType.X" :position="Position.Bottom" :tick-values="tickValues" :tick-format="formatDateTick" :tick-line="false" :domain-line="false" :tick-text-fit-mode="FitMode.Trim" :tick-text-align="TextAlign.Center" tick-text-font-size="11px" tick-text-color="var(--muted-foreground)" />
        <VisTooltip :follow-cursor="false" />
        <VisCrosshair color="var(--color-activeUsers)" :x="indexAccessor" :y="activeUsersAccessor" :template="tooltipTemplate" :hide-when-far-from-pointer="false" />
      </VisXYContainer>
    </div>

    <div class="mt-3 h-[130px] min-w-[520px] border-t border-border pt-3">
      <VisXYContainer :data="chartData" :height="118" :margin="{ top: 8, right: 18, bottom: 28, left: 46 }" :duration="0" :x-domain="xDomain" :y-domain="[0, Math.ceil(cumulativeMax * 1.08)]" :prevent-empty-domain="true" aria-label="累计注册用户趋势">
        <VisArea :x="indexAccessor" :y="cumulativeAccessor" :baseline="0" :id="idAccessor" color="var(--color-cumulativeUsers)" :opacity="0.16" :curve-type="CurveType.MonotoneX" />
        <VisLine :x="indexAccessor" :y="cumulativeAccessor" :id="idAccessor" color="var(--color-cumulativeUsers)" :line-width="2.5" :curve-type="CurveType.MonotoneX" />
        <VisAxis :type="AxisType.Y" :position="Position.Left" :grid-line="true" :tick-line="false" :domain-line="false" :num-ticks="3" :tick-format="formatNumberTick" tick-text-font-size="11px" tick-text-color="var(--muted-foreground)" />
        <VisAxis :type="AxisType.X" :position="Position.Bottom" :tick-values="tickValues" :tick-format="formatDateTick" :tick-line="false" :domain-line="false" tick-text-font-size="11px" tick-text-color="var(--muted-foreground)" />
        <VisTooltip :follow-cursor="false" />
        <VisCrosshair color="var(--color-cumulativeUsers)" :x="indexAccessor" :y="cumulativeAccessor" :template="tooltipTemplate" :hide-when-far-from-pointer="false" />
      </VisXYContainer>
    </div>
  </ChartContainer>
</template>

<style scoped>
.growth-trend-chart :deep(.vis-line path) { stroke-linecap: round; stroke-linejoin: round; }
.growth-trend-chart :deep(.vis-axis .grid-line) { stroke: var(--border); }
.growth-trend-chart :deep(.vis-axis .tick text) { font-family: var(--font-sans); letter-spacing: 0; }
</style>
