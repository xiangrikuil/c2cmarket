<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  payload?: Record<string, unknown>
  x?: number | Date
}>()

const point = computed(() => props.payload ?? {})

function numberText(key: string) {
  const value = point.value[key]
  return typeof value === 'number' ? value.toLocaleString('zh-CN') : '暂无'
}
</script>

<template>
  <div class="grid min-w-[138px] gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-[11px] text-slate-600 shadow-lg">
    <div class="font-medium text-slate-900">{{ point.date ?? x }}</div>
    <div class="flex items-center justify-between gap-3">
      <span>完成参考</span>
      <span class="font-mono font-semibold text-slate-900">¥{{ numberText('medianPrice') }}</span>
    </div>
    <div class="flex items-center justify-between gap-3">
      <span>参考区间</span>
      <span class="font-mono text-slate-800">¥{{ numberText('p25Price') }}-¥{{ numberText('p75Price') }}</span>
    </div>
    <div class="flex items-center justify-between gap-3">
      <span>样本</span>
      <span class="font-mono text-slate-800">{{ numberText('transactionCount') }}</span>
    </div>
  </div>
</template>
