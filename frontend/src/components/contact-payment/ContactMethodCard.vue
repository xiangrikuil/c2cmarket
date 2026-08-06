<script setup lang="ts">
import type { BadgeVariants } from '@/components/ui/badge'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

withDefaults(defineProps<{
  title: string
  description: string
  statusLabel: string
  statusVariant?: BadgeVariants['variant']
  dirty: boolean
  isDefault?: boolean
  currentSummary?: string
}>(), {
  statusVariant: 'secondary',
  isDefault: false,
  currentSummary: '',
})
</script>

<template>
  <Card class="contact-method-card p-5">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 gap-3">
        <span class="contact-method-card__icon">
          <slot name="icon" />
        </span>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-base font-semibold">{{ title }}</h2>
            <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
            <Badge v-if="isDefault" variant="outline">默认联系方式</Badge>
            <Badge v-if="dirty" variant="secondary">有未保存更改</Badge>
          </div>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">{{ description }}</p>
          <p v-if="currentSummary" class="mt-2 text-xs leading-5 text-muted-foreground">{{ currentSummary }}</p>
        </div>
      </div>
      <div v-if="$slots.actions" class="flex shrink-0 flex-wrap gap-2">
        <slot name="actions" />
      </div>
    </div>

    <div class="mt-4">
      <slot />
    </div>
  </Card>
</template>
