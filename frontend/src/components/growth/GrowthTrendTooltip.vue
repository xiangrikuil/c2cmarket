<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  payload?: Record<string, unknown>
  x?: number | Date
}>()

const point = computed(() => props.payload ?? {})

function numberText(key: string) {
  const value = point.value[key]
  return typeof value === 'number' ? value.toLocaleString('zh-CN') : '0'
}
</script>

<template>
  <div class="grid min-w-[148px] gap-1 rounded-md border border-border bg-background px-3 py-2 text-xs shadow-lg">
    <strong class="font-medium">{{ point.date ?? x }}</strong>
    <div class="flex justify-between gap-4 text-muted-foreground"><span>新增注册</span><b class="text-foreground">{{ numberText('newUsers') }}</b></div>
    <div class="flex justify-between gap-4 text-muted-foreground"><span>活跃用户</span><b class="text-foreground">{{ numberText('activeUsers') }}</b></div>
    <div class="flex justify-between gap-4 text-muted-foreground"><span>累计用户</span><b class="text-foreground">{{ numberText('cumulativeUsers') }}</b></div>
  </div>
</template>
