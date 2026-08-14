<script setup lang="ts">
import { computed, ref } from 'vue'
import { Star } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const props = defineProps<{ modelValue: number | null, disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: number] }>()
const hovered = ref<number | null>(null)
const visibleValue = computed(() => hovered.value ?? props.modelValue ?? 0)

function select(value: number) {
  if (!props.disabled) emit('update:modelValue', value)
}

function onKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  const current = props.modelValue ?? 0
  let next = current
  if (event.key === 'ArrowRight' || event.key === 'ArrowUp') next = Math.min(5, current + 1 || 1)
  else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') next = Math.max(1, current - 1 || 1)
  else if (event.key === 'Home') next = 1
  else if (event.key === 'End') next = 5
  else return
  event.preventDefault()
  select(next)
}
</script>

<template>
  <div
    role="radiogroup"
    aria-label="交易评分"
    :aria-disabled="disabled"
    class="inline-flex min-h-11 items-center gap-1"
    @mouseleave="hovered = null"
    @keydown="onKeydown"
  >
    <Button
      v-for="value in 5"
      :key="value"
      type="button"
      variant="ghost"
      size="icon-lg"
      role="radio"
      :aria-label="`${value} 分`"
      :aria-checked="modelValue === value"
      :disabled="disabled"
      :tabindex="value === (modelValue ?? 1) ? 0 : -1"
      class="rounded-md p-1.5 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed"
      @mouseenter="hovered = value"
      @focus="hovered = value"
      @blur="hovered = null"
      @click="select(value)"
    >
      <Star :class="cn('h-7 w-7', value <= visibleValue ? 'fill-amber-400 text-amber-500' : 'text-muted-foreground/30')" />
    </Button>
  </div>
</template>
