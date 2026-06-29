<script setup lang="ts">
import DeliveryModeTooltip from '@/components/api/DeliveryModeTooltip.vue'
import type { ApiDeliveryMode, ApiService } from '@/lib/api'
import { deliveryModeLabel } from './utils'

defineProps<{
  service: ApiService
  modelValue: ApiDeliveryMode
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ApiDeliveryMode]
}>()

function modeDescription(mode: ApiDeliveryMode) {
  return mode === 'sub2api_panel_account' ? '站外确认面板接入方式' : '站外确认请求地址接入方式'
}
</script>

<template>
  <div v-if="service.deliveryModes.length === 1" class="rounded-lg border border-border bg-muted/40 p-3">
    <div class="flex items-center gap-1.5 text-sm font-semibold">
      {{ deliveryModeLabel(service.deliveryModes[0]) }}
      <DeliveryModeTooltip :mode="service.deliveryModes[0]" />
    </div>
    <div class="mt-1 text-xs text-muted-foreground">{{ modeDescription(service.deliveryModes[0]) }}</div>
  </div>
  <div v-else class="grid gap-2 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
    <button
      v-for="mode in service.deliveryModes"
      :key="mode"
      type="button"
      class="rounded-lg border p-3 text-left transition-colors"
      :class="modelValue === mode ? 'border-primary bg-primary/10' : 'border-border bg-background hover:bg-muted'"
      @click="emit('update:modelValue', mode)"
    >
      <div class="flex items-center gap-1.5 text-sm font-semibold">
        {{ deliveryModeLabel(mode) }}
        <DeliveryModeTooltip :mode="mode" />
      </div>
      <div class="mt-1 text-xs text-muted-foreground">{{ modeDescription(mode) }}</div>
    </button>
  </div>
</template>
