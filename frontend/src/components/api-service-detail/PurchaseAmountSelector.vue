<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Input } from '@/components/ui/input'
import type { ApiService } from '@/lib/api'
import { formatCny } from './utils'

const props = defineProps<{
  service: ApiService
  modelValue: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const customValue = ref('')
const selectedPreset = ref(String(props.modelValue))
const presets = computed(() => [props.service.minimumPurchaseCny, 50, 100].filter((value, index, rows) => value <= props.service.maxBuy && rows.indexOf(value) === index))

watch(() => props.modelValue, value => {
  if (presets.value.includes(value)) {
    selectedPreset.value = String(value)
    customValue.value = ''
  }
})

function selectPreset(value: number) {
  selectedPreset.value = String(value)
  customValue.value = ''
  emit('update:modelValue', value)
}

function selectCustom() {
  selectedPreset.value = 'custom'
  const parsed = Number(customValue.value)
  emit('update:modelValue', Number.isFinite(parsed) ? parsed : 0)
}

function updateCustom(value: string) {
  customValue.value = value
  selectedPreset.value = 'custom'
  const parsed = Number(value)
  emit('update:modelValue', Number.isFinite(parsed) ? parsed : 0)
}
</script>

<template>
  <div class="space-y-2">
    <div class="grid grid-cols-4 gap-2">
      <button
        v-for="preset in presets"
        :key="preset"
        type="button"
        class="h-10 rounded-md border px-2 text-sm font-semibold transition-colors"
        :class="selectedPreset === String(preset) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
        @click="selectPreset(preset)"
      >
        {{ formatCny(preset) }}
      </button>
      <button
        type="button"
        class="h-10 rounded-md border px-2 text-sm font-semibold transition-colors"
        :class="selectedPreset === 'custom' ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
        @click="selectCustom"
      >
        自定义
      </button>
    </div>
    <Input
      v-if="selectedPreset === 'custom'"
      :model-value="customValue"
      inputmode="decimal"
      placeholder="输入意向金额"
      @update:model-value="value => updateCustom(String(value))"
    />
  </div>
</template>
