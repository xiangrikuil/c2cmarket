<script setup lang="ts">
import { ref, watch } from 'vue'
import { Input } from '@/components/ui/input'
import type { ApiService } from '@/lib/api'

const props = defineProps<{
  service: ApiService
  modelValue: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const inputValue = ref(String(props.modelValue))

watch(() => props.modelValue, value => {
  const parsedInput = Number(inputValue.value)
  if ((inputValue.value === '' || !Number.isFinite(parsedInput)) && value === 0) return
  if (parsedInput !== value) inputValue.value = String(value)
})

function updateAmount(value: string) {
  inputValue.value = value
  const parsed = Number(value)
  emit('update:modelValue', Number.isFinite(parsed) ? parsed : 0)
}
</script>

<template>
  <div class="space-y-2">
    <Input
      :model-value="inputValue"
      inputmode="decimal"
      placeholder="请输入订单金额"
      @update:model-value="value => updateAmount(String(value))"
    />
    <p class="text-xs text-muted-foreground">可输入 ¥{{ service.minimumPurchaseCny }}–¥{{ service.maxBuy }}</p>
  </div>
</template>
