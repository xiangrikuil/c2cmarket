<script setup lang="ts">
import type { Component } from 'vue'
import { computed } from 'vue'
import { Card } from '@/components/ui/card'

const props = defineProps<{
  label: string
  value: string | number
  note?: string
  hint?: string
  accent?: boolean
  icon?: Component
}>()

const supportingText = computed(() => props.hint ?? props.note)
</script>

<template>
  <Card
    class="group h-full !gap-3 !p-3.5 transition hover:-translate-y-0.5 hover:shadow-md"
    :class="accent ? 'border-primary/30 bg-primary/5' : 'hover:border-primary/30 hover:bg-accent/35'"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0 text-sm text-muted-foreground">{{ label }}</div>
      <span
        v-if="icon"
        class="grid h-8 w-8 shrink-0 place-items-center rounded-md border text-primary transition group-hover:bg-primary group-hover:text-primary-foreground"
        :class="accent ? 'border-primary/25 bg-primary/10' : 'border-border bg-background'"
      >
        <component :is="icon" class="h-4 w-4" />
      </span>
    </div>
    <div class="text-2xl font-semibold tracking-tight">{{ value }}</div>
    <div v-if="supportingText" class="text-xs leading-5 text-muted-foreground">{{ supportingText }}</div>
  </Card>
</template>
