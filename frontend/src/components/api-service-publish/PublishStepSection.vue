<script setup lang="ts">
import { ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { PublishStepStatus } from './publishWorkflow'

const props = defineProps<{
  step: number
  title: string
  description: string
  status: PublishStepStatus
  summary?: string
}>()

const emit = defineEmits<{
  edit: [step: number]
}>()
</script>

<template>
  <section
    :id="`publish-step-${step}`"
    class="api-publish-step"
    :class="`is-${status}`"
    :aria-labelledby="`publish-step-title-${step}`"
    :aria-current="status === 'active' ? 'step' : undefined"
  >
    <header class="api-publish-step-header">
      <div class="min-w-0 flex-1">
        <h2
          :id="`publish-step-title-${step}`"
          class="text-base font-semibold"
          data-publish-step-heading
          tabindex="-1"
        >
          {{ title }}
        </h2>
        <p v-if="status === 'active'" class="mt-1 text-xs leading-5 text-muted-foreground">{{ description }}</p>
        <p v-else-if="summary" class="mt-1 break-words text-xs leading-5 text-muted-foreground">{{ summary }}</p>
      </div>
      <Button
        v-if="status === 'completed'"
        type="button"
        size="sm"
        variant="ghost"
        class="shrink-0"
        :aria-label="`修改${title}`"
        @click="emit('edit', props.step)"
      >
        修改 <ChevronRight class="h-4 w-4" />
      </Button>
    </header>

    <div v-if="status === 'active'" class="api-publish-step-content">
      <slot />
    </div>
  </section>
</template>
