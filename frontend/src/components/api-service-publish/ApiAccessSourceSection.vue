<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm, DistributionSystem } from './types'
import { publishDistributionOptions } from './utils'

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  setDistribution: [value: DistributionSystem]
  setDefaultMultiplier: [value: string]
}>()
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>接入类型与倍率</h2>
      <p>倍率默认 1；如上游按 0.01 等倍率折算，请填写商家实际声明值。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="space-y-2">
        <label class="text-sm font-medium">接入类型</label>
        <div class="grid gap-2 sm:grid-cols-2">
          <button
            v-for="option in publishDistributionOptions"
            :key="option.value"
            type="button"
            class="api-publish-option-card"
            :class="{ 'is-active': form.distributionSystem === option.value }"
            @click="emit('setDistribution', option.value)"
          >
            <span class="block text-sm font-semibold">{{ option.title }}</span>
            <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            <span class="mt-2 block text-[11px] leading-5 text-muted-foreground">{{ option.detail }}</span>
          </button>
        </div>
        <p v-if="errors.distributionSystem" class="text-xs text-destructive">{{ errors.distributionSystem }}</p>
      </div>

      <div class="space-y-2">
        <label class="text-sm font-medium">默认服务倍率</label>
        <div class="flex max-w-xs overflow-hidden rounded-md border border-input bg-background">
          <Input
            :model-value="Number.isFinite(form.defaultMultiplier) ? form.defaultMultiplier : ''"
            class="border-0 shadow-none focus-visible:ring-0"
            min="0.01"
            step="0.01"
            type="number"
            placeholder="1.00"
            @update:model-value="value => emit('setDefaultMultiplier', String(value))"
          />
          <span class="grid w-12 place-items-center border-l border-border text-sm text-muted-foreground">x</span>
        </div>
        <p v-if="errors.defaultMultiplier" class="text-xs text-destructive">{{ errors.defaultMultiplier }}</p>
        <p v-else class="text-xs text-muted-foreground">自建面板通常填 1；按美元面值缩放的中转可填 0.01。模型也可以单独覆盖。</p>
      </div>

    </div>
  </Card>
</template>
