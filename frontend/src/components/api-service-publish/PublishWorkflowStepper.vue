<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import {
  Stepper,
  StepperDescription,
  StepperIndicator,
  StepperItem,
  StepperSeparator,
  StepperTitle,
  StepperTrigger,
} from '@/components/ui/stepper'
import type { PublishWorkflowStep } from './publishWorkflow'
import { canVisitPublishStep } from './publishWorkflow'

const props = defineProps<{
  steps: PublishWorkflowStep[]
  currentStep: number
  completedSteps: number[]
}>()

const emit = defineEmits<{
  select: [step: number]
}>()

function selectStep(step: number) {
  if (canVisitPublishStep(step, props.currentStep, props.completedSteps)) emit('select', step)
}
</script>

<template>
  <Stepper :model-value="currentStep" class="api-publish-stepper w-full items-start overflow-x-auto px-1 sm:px-3">
    <StepperItem
      v-for="(item, index) in steps"
      :key="item.title"
      :step="index + 1"
      class="relative flex min-w-[112px] flex-1 flex-col items-center"
    >
      <StepperTrigger
        class="flex w-full flex-col items-center gap-1"
        :disabled="!canVisitPublishStep(index + 1, currentStep, completedSteps)"
        :aria-controls="`publish-step-${index + 1}`"
        :aria-current="currentStep === index + 1 ? 'step' : undefined"
        :aria-expanded="currentStep === index + 1"
        @click="selectStep(index + 1)"
      >
        <StepperIndicator class="h-7 w-7 shrink-0 text-xs">
          <Check v-if="completedSteps.includes(index + 1) && currentStep !== index + 1" class="h-4 w-4" />
          <span v-else>{{ index + 1 }}</span>
        </StepperIndicator>
        <div class="min-w-0 text-center">
          <StepperTitle class="whitespace-normal text-[11px] leading-4 sm:text-xs">{{ item.title }}</StepperTitle>
          <StepperDescription class="hidden text-[10px] leading-4 lg:block">{{ item.description }}</StepperDescription>
        </div>
      </StepperTrigger>
      <StepperSeparator
        v-if="index < steps.length - 1"
        class="pointer-events-none absolute left-[calc(50%+1rem)] right-[calc(-50%+1rem)] top-3.5 h-0.5 rounded-full bg-border group-data-[state=completed]:bg-primary/60"
      />
    </StepperItem>
  </Stepper>
</template>
