<script setup lang="ts">
import { computed } from 'vue'
import { AxisType, CurveType, FitMode, Position, TextAlign } from '@unovis/ts'
import { VisArea, VisAxis, VisGroupedBar, VisLine, VisXYContainer } from '@unovis/vue'
import {
  ChartContainer,
  ChartCrosshair,
  ChartLegendContent,
  ChartTooltip,
  componentToString,
  type ChartConfig,
} from '@/components/ui/chart'
import type { TransactionTrendPoint } from '@/lib/api'
import TransactionTrendTooltip from './TransactionTrendTooltip.vue'

const props = defineProps<{
  data: TransactionTrendPoint[]
}>()

type PriceBandPoint = TransactionTrendPoint & {
  index: number
}

const chartHeight = 320
const chartMargin = {
  top: 18,
  right: 72,
  bottom: 42,
  left: 70,
}
const volumeHeightRatio = 0.28

const chartConfig = {
  medianPrice: {
    label: '成交中位价',
    color: 'var(--chart-1)',
  },
  priceRange: {
    label: 'P25-P75 区间',
    color: 'var(--chart-2)',
  },
  transactionCount: {
    label: '成交数量',
    color: 'var(--chart-3)',
  },
} satisfies ChartConfig

const chartData = computed<PriceBandPoint[]>(() =>
  props.data.map((point, index) => ({ ...point, index })),
)

const xDomain = computed<[number, number]>(() => {
  const lastIndex = Math.max(chartData.value.length - 1, 0)
  return chartData.value.length <= 1 ? [-0.5, 0.5] : [-0.35, lastIndex + 0.35]
})

const priceDomain = computed<[number, number]>(() => {
  const priceValues = props.data.flatMap(point => [point.p25Price, point.p75Price, point.medianPrice])
  if (!priceValues.length) return [0, 1]

  const minPrice = Math.min(...priceValues)
  const maxPrice = Math.max(...priceValues)
  const priceSpan = Math.max(maxPrice - minPrice, 1)
  const paddedMin = minPrice - priceSpan * 0.16
  const paddedMax = maxPrice + priceSpan * 0.12
  const step = Math.max(1, Math.ceil(((paddedMax - paddedMin) / 3) / 5) * 5)

  return [
    Math.floor(paddedMin / step) * step,
    Math.ceil(paddedMax / step) * step,
  ]
})

const maxTransactionCount = computed(() => Math.max(...props.data.map(point => point.transactionCount), 0))
const volumeAxisMax = computed(() => Math.max(1, Math.ceil(maxTransactionCount.value / 5) * 5))
const volumeVisualMax = computed(() => Math.ceil(volumeAxisMax.value / volumeHeightRatio))
const volumeTicks = computed(() => [0, Math.round(volumeAxisMax.value / 2), volumeAxisMax.value])

const priceAccessor = (point: PriceBandPoint) => point.medianPrice
const priceRangeUpperAccessor = (point: PriceBandPoint) => point.p75Price
const priceRangeLowerAccessor = (point: PriceBandPoint) => point.p25Price
const transactionCountAccessor = (point: PriceBandPoint) => point.transactionCount
const indexAccessor = (point: PriceBandPoint) => point.index
const idAccessor = (point: PriceBandPoint) => point.date

const tooltipTemplate = componentToString(chartConfig, TransactionTrendTooltip)

function numberText(value: number) {
  return value.toLocaleString('zh-CN')
}

function formatPriceTick(tick: number | Date) {
  return typeof tick === 'number' ? `¥${numberText(tick)}` : ''
}

function formatVolumeTick(tick: number | Date) {
  return typeof tick === 'number' && tick <= volumeAxisMax.value ? numberText(tick) : ''
}

function formatDateTick(tick: number | Date) {
  if (typeof tick !== 'number') return ''

  const point = chartData.value[Math.round(tick)]
  return point?.date ?? ''
}
</script>

<template>
  <ChartContainer
    class="transaction-price-chart aspect-auto h-auto min-h-[392px] w-full justify-start rounded-md border border-border bg-card px-4 pb-4 pt-3"
    :config="chartConfig"
    :cursor="true"
  >
    <ChartLegendContent
      vertical-align="top"
      class="transaction-price-chart__legend pb-3 pt-0 text-xs text-muted-foreground"
    />

    <div class="transaction-price-chart__plot relative h-[320px] w-full overflow-visible">
      <span class="transaction-price-chart__axis-label transaction-price-chart__axis-label--left">参考价</span>
      <span class="transaction-price-chart__axis-label transaction-price-chart__axis-label--right">样本量</span>

      <div class="absolute inset-0 pointer-events-none">
        <VisXYContainer
          class="transaction-price-chart__volume"
          :data="chartData"
          :height="chartHeight"
          :margin="chartMargin"
          :auto-margin="false"
          :duration="0"
          :x-domain="xDomain"
          :y-domain="[0, volumeVisualMax]"
          :prevent-empty-domain="true"
          aria-label="完成参考价趋势样本量"
        >
          <VisGroupedBar
            :x="indexAccessor"
            :y="transactionCountAccessor"
            :id="idAccessor"
            color="var(--chart-3)"
            :group-max-width="34"
            :group-padding="0.56"
            :rounded-corners="4"
          />
          <VisAxis
            :type="AxisType.Y"
            :position="Position.Right"
            :grid-line="false"
            :tick-line="true"
            :domain-line="true"
            :tick-values="volumeTicks"
            :tick-format="formatVolumeTick"
            tick-text-font-size="12px"
            tick-text-color="var(--muted-foreground)"
            :tick-padding="8"
          />
        </VisXYContainer>
      </div>

      <div class="absolute inset-0 z-[1]">
        <VisXYContainer
          :data="chartData"
          :height="chartHeight"
          :margin="chartMargin"
          :auto-margin="false"
          :duration="0"
          :x-domain="xDomain"
          :y-domain="priceDomain"
          :prevent-empty-domain="true"
          aria-label="完成参考价趋势组合图"
        >
          <VisArea
            :x="indexAccessor"
            :y="priceRangeUpperAccessor"
            :baseline="priceRangeLowerAccessor"
            :id="idAccessor"
            color="var(--chart-2)"
            :opacity="0.26"
            :curve-type="CurveType.MonotoneX"
          />
          <VisLine
            :x="indexAccessor"
            :y="priceAccessor"
            :id="idAccessor"
            color="var(--chart-1)"
            :line-width="3"
            :curve-type="CurveType.MonotoneX"
          />
          <VisAxis
            :type="AxisType.Y"
            :position="Position.Left"
            :grid-line="true"
            :tick-line="true"
            :domain-line="true"
            :num-ticks="4"
            :tick-format="formatPriceTick"
            tick-text-font-size="12px"
            tick-text-color="var(--muted-foreground)"
            :tick-padding="8"
          />
          <VisAxis
            :type="AxisType.X"
            :position="Position.Bottom"
            :grid-line="false"
            :tick-line="true"
            :domain-line="true"
            :tick-format="formatDateTick"
            :tick-values="chartData.map(point => point.index)"
            tick-text-font-size="12px"
            tick-text-color="var(--muted-foreground)"
            :tick-text-fit-mode="FitMode.Trim"
            :tick-text-align="TextAlign.Center"
            :tick-padding="8"
          />
          <ChartTooltip :follow-cursor="false" />
          <ChartCrosshair
            color="var(--chart-1)"
            :x="indexAccessor"
            :y="priceAccessor"
            :template="tooltipTemplate"
            :hide-when-far-from-pointer="false"
          />
        </VisXYContainer>
      </div>
    </div>
  </ChartContainer>
</template>

<style scoped>
.transaction-price-chart {
  box-shadow: var(--shadow-2xs);
}

.transaction-price-chart__plot {
  background:
    linear-gradient(180deg, color-mix(in oklab, var(--card) 98%, var(--accent)) 0%, var(--card) 100%);
}

.transaction-price-chart :deep(.vis-grouped-bar .bar) {
  opacity: 0.82;
}

.transaction-price-chart :deep(.vis-line path) {
  stroke-linecap: round;
  stroke-linejoin: round;
}

.transaction-price-chart :deep(.vis-axis .tick text),
.transaction-price-chart :deep(.vis-axis .axis-label) {
  font-family: var(--font-sans);
  letter-spacing: 0;
}

.transaction-price-chart :deep(.vis-axis .axis-label) {
  display: none;
}

.transaction-price-chart__legend :deep(.rounded-xs) {
  border-radius: 999px;
}

.transaction-price-chart__axis-label {
  position: absolute;
  top: 50%;
  z-index: 3;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0;
  line-height: 1;
  pointer-events: none;
  transform: translateY(-50%) rotate(-90deg);
  transform-origin: center;
  white-space: nowrap;
}

.transaction-price-chart__axis-label--left {
  left: 0;
}

.transaction-price-chart__axis-label--right {
  right: 0;
}
</style>
