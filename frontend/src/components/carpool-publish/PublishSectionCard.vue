<script setup lang="ts">
import { Card } from '@/components/ui/card'
import type { PublishFieldState } from './types'

defineProps<{
  index: number
  title: string
  description: string
  status?: PublishFieldState
  statusLabel?: string
  sectionId?: string
}>()
</script>

<template>
  <Card
    :id="sectionId"
    class="overflow-hidden p-0 shadow-sm transition-colors"
    :class="status === 'error' ? 'border-destructive/40' : status === 'pendingRequired' ? 'border-warning/35' : ''"
  >
    <div class="flex items-start gap-3 px-4 pb-2 pt-4">
      <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-primary/10 text-xs font-bold text-primary">
        {{ index }}
      </span>
      <div class="min-w-0 flex-1">
        <h2 class="text-base font-semibold leading-tight">{{ title }}</h2>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ description }}</p>
      </div>
      <span
        v-if="statusLabel"
        class="shrink-0 rounded-full border px-2 py-1 text-xs font-medium"
        :class="{
          'border-destructive/25 bg-destructive/10 text-destructive': status === 'error',
          'border-warning/25 bg-warning/10 text-warning': status === 'pendingRequired',
          'border-success/25 bg-success/10 text-success': status === 'complete' || status === 'defaulted',
          'border-border bg-muted/40 text-muted-foreground': !status || status === 'idle',
        }"
      >
        {{ statusLabel }}
      </span>
    </div>
    <div class="px-4 pb-4 pt-2">
      <slot />
    </div>
  </Card>
</template>
