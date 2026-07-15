<script setup lang="ts">
withDefaults(defineProps<{ rows?: number, columns?: number }>(), { rows: 5, columns: 5 })
</script>

<template>
  <div class="overflow-hidden rounded-xl border border-border bg-card" aria-label="正在加载列表" aria-busy="true">
    <div v-for="row in rows" :key="row" class="grid animate-pulse gap-4 border-b border-border p-4 last:border-0" :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }">
      <div v-for="column in columns" :key="column" class="h-4 rounded bg-muted" />
    </div>
  </div>
</template>
