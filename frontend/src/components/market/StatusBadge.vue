<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { statusTone, type StatusTone } from '@/lib/presentation'

const props = defineProps<{ status: string, label?: string, tone?: StatusTone }>()
const resolvedTone = computed(() => props.tone ?? statusTone(props.status))
const toneClass: Record<StatusTone, string> = {
  brand: 'border-primary/20 bg-primary/10 text-primary',
  success: 'border-success/20 bg-success/10 text-success',
  waiting: 'border-waiting/20 bg-waiting/10 text-waiting',
  warning: 'border-warning/25 bg-warning/10 text-warning',
  risk: 'border-risk/20 bg-risk/10 text-risk',
  complete: 'border-complete/20 bg-muted text-complete',
  neutral: 'border-border bg-muted text-muted-foreground',
}
</script>

<template>
  <Badge variant="outline" :class="toneClass[resolvedTone]">{{ label ?? status }}</Badge>
</template>
