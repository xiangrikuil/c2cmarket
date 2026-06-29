<script setup lang="ts">
defineProps<{
  latestPrice: number | null
  medianPrice: number | null
  priceRange: string
  sampleCount: number
}>()

function priceText(value: number | null) {
  return value === null ? '暂无' : `¥${value}`
}
</script>

<template>
  <div class="grid border-b border-border md:grid-cols-4 md:divide-x md:divide-border">
    <div class="bg-background/55 px-4 py-1.5">
      <div class="text-xs text-muted-foreground">最新完成参考价</div>
      <div class="mt-0.5 text-xl font-semibold">{{ priceText(latestPrice) }}</div>
    </div>
    <div class="bg-background/55 px-4 py-1.5">
      <div class="text-xs text-muted-foreground">近 30 日中位价</div>
      <div class="mt-0.5 text-xl font-semibold">{{ priceText(medianPrice) }}</div>
    </div>
    <div class="bg-background/55 px-4 py-1.5">
      <div class="text-xs text-muted-foreground">参考区间</div>
      <div class="mt-0.5 text-xl font-semibold">{{ priceRange }}</div>
    </div>
    <div class="bg-background/55 px-4 py-1.5">
      <div class="text-xs text-muted-foreground">有效样本数</div>
      <div class="mt-0.5 text-xl font-semibold">{{ sampleCount }} 单</div>
    </div>
  </div>
</template>
