<script setup lang="ts">
defineProps<{ items: Array<{ label: string, value: string | number, hint?: string }>, loading?: boolean }>()
</script>

<template>
  <div class="grid gap-px overflow-hidden rounded-xl border border-border bg-border sm:grid-flow-col sm:auto-cols-fr">
    <div v-for="item in items" :key="item.label" class="bg-card px-4 py-3">
      <div class="text-xs text-muted-foreground">{{ item.label }}</div>
      <div v-if="loading" class="mt-2 h-6 w-16 animate-pulse rounded bg-muted" />
      <div v-else class="mt-1 text-xl font-semibold">{{ item.value }}</div>
      <div v-if="item.hint && !loading" class="mt-1 text-xs text-muted-foreground">{{ item.hint }}</div>
    </div>
  </div>
</template>
