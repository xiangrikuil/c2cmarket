<script setup lang="ts">
import { computed } from 'vue'
import { AxisType, CurveType, FitMode, Position, TextAlign } from '@unovis/ts'
import { VisArea, VisAxis, VisLine, VisXYContainer } from '@unovis/vue'
import {
  ChartContainer,
  ChartCrosshair,
  ChartTooltip,
  componentToString,
  type ChartConfig,
} from '@/components/ui/chart'
import type { TransactionTrendPoint } from '@/lib/api'
import HomeTrendTooltip from './HomeTrendTooltip.vue'

const props = defineProps<{
  data: TransactionTrendPoint[]
}>()

type HomeTrendPoint = TransactionTrendPoint & {
  index: number
}

const chartHeight = 180
const chartMargin = {
  top: 10,
  right: 16,
  bottom: 28,
  left: 44,
}

const chartConfig = {
  medianPrice: {
    label: '完成参考',
    color: 'var(--chart-1)',
  },
  priceRange: {
    label: '参考区间',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

const chartData = computed<HomeTrendPoint[]>(() =>
  props.data.map((point, index) => ({ ...point, index })),
)

const xDomain = computed<[number, number]>(() => {
  const lastIndex = Math.max(chartData.value.length - 1, 0)
  return chartData.value.length <= 1 ? [-0.5, 0.5] : [-0.28, lastIndex + 0.28]
})

const priceDomain = computed<[number, number]>(() => {
  const values = props.data.flatMap(point => [point.p25Price, point.p75Price, point.medianPrice])
  if (!values.length) return [0, 1]

  const minPrice = Math.min(...values)
  const maxPrice = Math.max(...values)
  const span = Math.max(maxPrice - minPrice, 1)
  const paddedMin = minPrice - span * 0.2
  const paddedMax = maxPrice + span * 0.14
  const step = Math.max(1, Math.ceil(((paddedMax - paddedMin) / 4) / 10) * 10)

  return [
    Math.max(0, Math.floor(paddedMin / step) * step),
    Math.ceil(paddedMax / step) * step,
  ]
})

const tickValues = computed(() => {
  if (chartData.value.length <= 5) return chartData.value.map(point => point.index)
  return chartData.value
    .filter((point, index) => index === 0 || index === chartData.value.length - 1 || index % 2 === 1)
    .map(point => point.index)
})

const priceAccessor = (point: HomeTrendPoint) => point.medianPrice
const priceRangeUpperAccessor = (point: HomeTrendPoint) => point.p75Price
const priceRangeLowerAccessor = (point: HomeTrendPoint) => point.p25Price
const indexAccessor = (point: HomeTrendPoint) => point.index
const idAccessor = (point: HomeTrendPoint) => point.date

const tooltipTemplate = componentToString(chartConfig, HomeTrendTooltip)

function formatPriceTick(tick: number | Date) {
  return typeof tick === 'number' ? `¥${tick.toLocaleString('zh-CN')}` : ''
}

function formatDateTick(tick: number | Date) {
  if (typeof tick !== 'number') return ''
  return chartData.value[Math.round(tick)]?.date ?? ''
}
</script>

<template>
  <ChartContainer
    class="home-trend-chart h-[180px] w-full justify-start"
    :config="chartConfig"
    :cursor="true"
  >
    <VisXYContainer
      :data="chartData"
      :height="chartHeight"
      :margin="chartMargin"
      :auto-margin="false"
      :duration="0"
      :x-domain="xDomain"
      :y-domain="priceDomain"
      :prevent-empty-domain="true"
      aria-label="近 30 日完成参考曲线"
    >
      <VisArea
        :x="indexAccessor"
        :y="priceRangeUpperAccessor"
        :baseline="priceRangeLowerAccessor"
        :id="idAccessor"
        color="var(--color-priceRange)"
        :opacity="0.72"
        :curve-type="CurveType.MonotoneX"
      />
      <VisLine
        :x="indexAccessor"
        :y="priceAccessor"
        :id="idAccessor"
        color="var(--color-medianPrice)"
        :line-width="2.4"
        :curve-type="CurveType.MonotoneX"
      />
      <VisAxis
        :type="AxisType.Y"
        :position="Position.Left"
        :grid-line="true"
        :tick-line="false"
        :domain-line="false"
        :num-ticks="5"
        :tick-format="formatPriceTick"
        tick-text-font-size="11px"
        tick-text-color="var(--muted-foreground)"
        :tick-padding="8"
      />
      <VisAxis
        :type="AxisType.X"
        :position="Position.Bottom"
        :grid-line="false"
        :tick-line="false"
        :domain-line="false"
        :tick-format="formatDateTick"
        :tick-values="tickValues"
        tick-text-font-size="11px"
        tick-text-color="var(--muted-foreground)"
        :tick-text-fit-mode="FitMode.Trim"
        :tick-text-align="TextAlign.Center"
        :tick-padding="8"
      />
      <ChartTooltip :follow-cursor="false" />
      <ChartCrosshair
        color="var(--color-medianPrice)"
        :x="indexAccessor"
        :y="priceAccessor"
        :template="tooltipTemplate"
        :hide-when-far-from-pointer="false"
      />
    </VisXYContainer>
  </ChartContainer>
</template>

<style scoped>
.home-trend-chart :deep(.vis-line path) {
  stroke-linecap: round;
  stroke-linejoin: round;
}

.home-trend-chart :deep(.vis-axis .tick text) {
  font-family: var(--font-sans);
  letter-spacing: 0;
}

.home-trend-chart :deep(.vis-axis .grid-line) {
  stroke: var(--border);
}
</style>
