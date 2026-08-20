<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Input } from '@/components/ui/input'
import type { ApiService } from '@/lib/api'
import { isApiServiceTailOrder } from '@/lib/apiServicePricingPresentation'

const props = defineProps<{
  service: ApiService
  modelValue: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const inputValue = ref(String(props.modelValue))
const tailOrder = computed(() => isApiServiceTailOrder(props.service))

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
      :disabled="tailOrder"
      inputmode="decimal"
      placeholder="请输入订单金额"
      @update:model-value="value => updateAmount(String(value))"
    />
    <p class="text-xs text-muted-foreground">{{ tailOrder ? `当前为尾单，固定 ¥${service.maxBuy}，需一次买完全部剩余额度。` : `可输入 ¥${service.minimumPurchaseCny}–¥${service.maxBuy}` }}</p>
  </div>
</template>
