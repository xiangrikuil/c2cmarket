<script setup lang="ts">
import { computed } from 'vue'
import type { ChartConfig } from '@/components/ui/chart'

const props = defineProps<{
  payload?: Record<string, unknown>
  config?: ChartConfig
  x?: number | Date
}>()

const point = computed(() => props.payload ?? {})

function numberText(key: string) {
  const value = point.value[key]
  return typeof value === 'number' ? value.toLocaleString('zh-CN') : '暂无'
}
</script>

<template>
  <div class="grid min-w-[160px] gap-1.5 rounded-lg border border-border/60 bg-background px-2.5 py-2 text-xs shadow-xl">
    <div class="font-medium">{{ point.date ?? x }}</div>
    <div class="flex items-center justify-between gap-3">
      <span class="text-muted-foreground">成交中位价</span>
      <span class="font-mono font-medium">¥{{ numberText('medianPrice') }}</span>
    </div>
    <div class="flex items-center justify-between gap-3">
      <span class="text-muted-foreground">P25-P75 区间</span>
      <span class="font-mono font-medium">¥{{ numberText('p25Price') }}-¥{{ numberText('p75Price') }}</span>
    </div>
    <div class="flex items-center justify-between gap-3">
      <span class="text-muted-foreground">成交数量</span>
      <span class="font-mono font-medium">{{ numberText('transactionCount') }} 单</span>
    </div>
  </div>
</template>
