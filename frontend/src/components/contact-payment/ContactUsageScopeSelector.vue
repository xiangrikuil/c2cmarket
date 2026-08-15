<script setup lang="ts">
import { Checkbox } from '@/components/ui/checkbox'
import type { ContactUsageScope } from '@/lib/api'
import type { ContactUsageScopeOption } from '@/lib/contactUsageScopes'

const props = defineProps<{
  modelValue: ContactUsageScope[]
  options: ContactUsageScopeOption[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ContactUsageScope[]]
}>()

function toggleScope(scope: ContactUsageScope, checked: boolean) {
  const selected = checked
    ? new Set([...props.modelValue, scope])
    : new Set(props.modelValue.filter(value => value !== scope))
  emit('update:modelValue', props.options.map(option => option.value).filter(value => selected.has(value)))
}
</script>

<template>
  <fieldset class="space-y-2">
    <legend class="text-sm font-medium">适用场景</legend>
    <div class="grid gap-2 sm:grid-cols-2">
      <label
        v-for="option in options"
        :key="option.value"
        class="flex min-h-14 items-start gap-3 rounded-md border border-border px-3 py-2.5 hover:bg-muted/35"
      >
        <Checkbox
          :model-value="modelValue.includes(option.value)"
          class="mt-0.5"
          @update:model-value="value => toggleScope(option.value, Boolean(value))"
        />
        <span class="min-w-0">
          <span class="block text-sm font-medium">{{ option.label }}</span>
          <span class="mt-0.5 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
        </span>
      </label>
    </div>
    <p v-if="!modelValue.length" class="text-xs text-destructive" role="alert">请至少选择一个适用场景。</p>
  </fieldset>
</template>
