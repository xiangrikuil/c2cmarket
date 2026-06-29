<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  title: string
  hint: string
  icon: Component
}>()
</script>

<template>
  <div class="mb-4 flex items-center justify-between gap-4">
    <div class="flex items-center gap-2">
      <span class="grid h-7 w-7 place-items-center rounded-lg bg-primary/10 text-primary">
        <component :is="icon" class="h-4 w-4" />
      </span>
      <h2 class="text-base font-semibold">{{ title }}</h2>
    </div>
    <p class="hidden text-xs text-muted-foreground sm:block">{{ hint }}</p>
  </div>
</template>
