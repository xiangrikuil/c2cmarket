<script setup lang="ts">
defineProps<{ title: string, description?: string }>()
</script>

<template>
  <div class="grid min-h-48 place-items-center rounded-xl border border-dashed border-border bg-card p-8 text-center">
    <div class="max-w-md"><slot name="icon" /><h2 class="font-semibold">{{ title }}</h2><p v-if="description" class="mt-2 text-sm text-muted-foreground">{{ description }}</p><div v-if="$slots.action" class="mt-5"><slot name="action" /></div></div>
  </div>
</template>
