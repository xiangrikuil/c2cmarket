<script setup lang="ts">
import { Star } from 'lucide-vue-next'

withDefaults(defineProps<{ rating: number, compact?: boolean, showValue?: boolean }>(), {
  compact: false,
  showValue: true,
})
</script>

<template>
  <span class="inline-flex items-center gap-1" :aria-label="`${rating} 分（满分 5 分）`">
    <span class="inline-flex" aria-hidden="true">
      <Star
        v-for="value in 5"
        :key="value"
        :class="[compact ? 'h-3.5 w-3.5' : 'h-5 w-5', value <= rating ? 'fill-amber-400 text-amber-500' : 'text-muted-foreground/30']"
      />
    </span>
    <span v-if="showValue" class="font-medium">{{ rating }} 分</span>
  </span>
</template>
