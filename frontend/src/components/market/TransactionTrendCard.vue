<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { ProductTrend, TransactionRecord, TransactionTrendSummary as TransactionTrendSummaryData } from '@/lib/api'
import TransactionPriceChart from './TransactionPriceChart.vue'
import TransactionTrendEmpty from './TransactionTrendEmpty.vue'
import TransactionTrendSummary from './TransactionTrendSummary.vue'

export type TransactionTrendRange = '7d' | '30d' | '90d'

const props = defineProps<{
  trends: ProductTrend[]
  transactions: TransactionRecord[]
  selectedProduct: string
  selectedRange: TransactionTrendRange
  trendSummary?: TransactionTrendSummaryData | null
}>()

const emit = defineEmits<{
  'update:selectedProduct': [value: string]
  'update:selectedRange': [value: TransactionTrendRange]
}>()

const rangeOptions: Array<{ label: string; value: TransactionTrendRange }> = [
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
  { label: '近 90 天', value: '90d' },
]

const selectedTrend = computed(() => props.trends.find(item => item.slug === props.selectedProduct))
const trendPoints = computed(() => props.trendSummary?.points ?? selectedTrend.value?.points[props.selectedRange] ?? [])
const validSampleCount = computed(() => props.trendSummary?.validSampleCount ?? trendPoints.value.reduce((sum, item) => sum + item.transactionCount, 0))
const shouldDrawTrend = computed(() => validSampleCount.value >= 5 && trendPoints.value.length >= 2)

const selectedTransactions = computed(() => props.transactions.filter(item => {
  return item.productSlug === props.selectedProduct
    && item.status === 'completed'
    && !item.hasUnresolvedDispute
    && Number.isFinite(item.finalSettlementPrice)
}))

const summary = computed(() => {
  if (props.trendSummary) {
    return {
      latestPrice: props.trendSummary.latestTransactionPrice,
      medianPrice: props.trendSummary.medianPrice,
      priceRange: props.trendSummary.p25Price === null || props.trendSummary.p75Price === null
        ? '暂无'
        : `¥${props.trendSummary.p25Price}-¥${props.trendSummary.p75Price}`,
      sampleCount: props.trendSummary.validSampleCount,
    }
  }

  const points = trendPoints.value
  const latestPoint = [...points].reverse().find(item => item.transactionCount > 0)
  const latestTransaction = selectedTransactions.value[0]
  const p25 = points.map(item => item.p25Price)
  const p75 = points.map(item => item.p75Price)
  const medianValues = points.map(item => item.medianPrice)
  const rangeLow = p25.length ? Math.min(...p25) : null
  const rangeHigh = p75.length ? Math.max(...p75) : null
  const medianPrice = medianValues.length
    ? Math.round(medianValues.reduce((sum, item) => sum + item, 0) / medianValues.length)
    : null

  return {
    latestPrice: latestTransaction?.finalSettlementPrice ?? latestPoint?.medianPrice ?? null,
    medianPrice,
    priceRange: rangeLow === null || rangeHigh === null ? '暂无' : `¥${rangeLow}-¥${rangeHigh}`,
    sampleCount: validSampleCount.value,
  }
})
</script>

<template>
  <Card class="home-panel overflow-hidden p-0">
    <div class="flex flex-col gap-3 border-b border-border px-4 py-2.5 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h2 class="text-base font-semibold">完成参考价趋势</h2>
        <p class="mt-0.5 text-xs text-muted-foreground">中位价、P25-P75 参考区间与完成样本量</p>
      </div>
      <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
        <Select :model-value="selectedProduct" @update:model-value="value => emit('update:selectedProduct', String(value))">
          <SelectTrigger class="h-9 w-full sm:w-[220px]">
            <SelectValue placeholder="选择产品" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="option in trends" :key="option.slug" :value="option.slug">
              {{ option.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <div class="grid grid-cols-3 rounded-md border border-border bg-background p-1 sm:inline-flex">
          <Button
            v-for="range in rangeOptions"
            :key="range.value"
            class="px-2"
            size="sm"
            :variant="selectedRange === range.value ? 'default' : 'ghost'"
            @click="emit('update:selectedRange', range.value)"
          >
            {{ range.label }}
          </Button>
        </div>
      </div>
    </div>

    <TransactionTrendSummary
      :latest-price="summary.latestPrice"
      :median-price="summary.medianPrice"
      :price-range="summary.priceRange"
      :sample-count="summary.sampleCount"
    />

    <TransactionTrendEmpty v-if="!shouldDrawTrend" :sample-count="summary.sampleCount" />
    <div v-else class="bg-card px-3 pb-3 pt-3">
      <TransactionPriceChart :data="trendPoints" />
    </div>
  </Card>
</template>
