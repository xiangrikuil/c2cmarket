<script setup lang="ts">
withDefaults(defineProps<{ lines?: number }>(), { lines: 3 })
</script>

<template>
  <div class="animate-pulse rounded-xl border border-border bg-card p-5" aria-label="正在加载" aria-busy="true">
    <div class="h-5 w-1/3 rounded bg-muted" />
    <div class="mt-4 space-y-3"><div v-for="line in lines" :key="line" class="h-4 rounded bg-muted" :class="line === lines ? 'w-2/3' : 'w-full'" /></div>
  </div>
</template>
