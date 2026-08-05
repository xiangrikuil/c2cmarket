<script setup lang="ts">
import { ref } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{
  id: string
  modelValue: string
  label: string
  autocomplete: 'current-password' | 'new-password'
  invalid?: boolean
  describedBy?: string
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'blur', value: FocusEvent): void
}>()

const passwordVisible = ref(false)
</script>

<template>
  <div class="relative">
    <Input
      :id="id"
      :model-value="modelValue"
      class="h-11 pr-12"
      :class="invalid ? 'border-destructive bg-[#FFF7F7] focus-visible:border-destructive focus-visible:ring-destructive/20 dark:bg-destructive/10' : ''"
      :type="passwordVisible ? 'text' : 'password'"
      :autocomplete="autocomplete"
      :aria-invalid="invalid ? 'true' : undefined"
      :aria-describedby="describedBy"
      @update:model-value="value => emit('update:modelValue', String(value))"
      @blur="emit('blur', $event)"
    />
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
      :aria-label="passwordVisible ? `隐藏${label}` : `显示${label}`"
      :aria-pressed="passwordVisible"
      :title="passwordVisible ? `隐藏${label}` : `显示${label}`"
      @click="passwordVisible = !passwordVisible"
    >
      <EyeOff v-if="passwordVisible" class="h-4 w-4" />
      <Eye v-else class="h-4 w-4" />
    </Button>
  </div>
</template>
