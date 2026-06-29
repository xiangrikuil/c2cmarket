<script setup lang="ts">
import { Card } from '@/components/ui/card'

defineProps<{
  index: number
  title: string
  description: string
}>()
</script>

<template>
  <Card class="overflow-hidden p-0 shadow-sm">
    <div class="flex items-start gap-3 px-4 pb-2 pt-4">
      <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-primary/10 text-xs font-bold text-primary">
        {{ index }}
      </span>
      <div class="min-w-0">
        <h2 class="text-base font-semibold leading-tight">{{ title }}</h2>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ description }}</p>
      </div>
    </div>
    <div class="px-4 pb-4 pt-2">
      <slot />
    </div>
  </Card>
</template>
