<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { ApiServicePublishForm, BillingMode, DistributionSystem } from './types'
import { billingLabels, publishDistributionOptions, supportedPublishBillingModes } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  setDistribution: [value: DistributionSystem]
  setBilling: [value: BillingMode]
}>()

function setDistribution(value: unknown) {
  if (value === 'sub2api' || value === 'other') emit('setDistribution', value)
}

function setBilling(value: unknown) {
  if (value === 'metered_credit' || value === 'fixed_package') emit('setBilling', value)
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>1. 分发系统</h2>
      <p>选择后动态切换计费、接入和用量展示规则。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="space-y-2">
        <label class="text-sm font-medium">分发系统</label>
        <RadioGroup
          :model-value="form.distributionSystem"
          class="api-publish-option-grid"
          aria-label="分发系统"
          @update:model-value="setDistribution"
        >
          <Label
            v-for="option in publishDistributionOptions"
            :key="option.value"
            :for="`distribution-system-${option.value}`"
            class="api-publish-option-card cursor-pointer items-start gap-2 font-normal"
            :class="{ 'is-active': form.distributionSystem === option.value }"
          >
            <RadioGroupItem :id="`distribution-system-${option.value}`" :value="option.value" class="mt-0.5" />
            <span class="block text-sm font-semibold">{{ option.title }}</span>
            <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            <span class="mt-2 block text-[11px] leading-5 text-muted-foreground">{{ option.detail }}</span>
          </Label>
        </RadioGroup>
      </div>

      <div class="space-y-2">
        <label class="text-sm font-medium">售卖计费方式</label>
        <RadioGroup
          :model-value="form.billingMode"
          class="api-publish-billing-grid"
          aria-label="售卖计费方式"
          @update:model-value="setBilling"
        >
          <Label
            v-for="option in supportedPublishBillingModes"
            :key="option"
            :for="`billing-mode-${option}`"
            class="flex min-h-9 cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-left text-sm font-normal"
            :class="form.billingMode === option ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
          >
            <RadioGroupItem :id="`billing-mode-${option}`" :value="option" />
            {{ billingLabels[option] }}
          </Label>
        </RadioGroup>
        <p v-if="form.distributionSystem !== 'sub2api'" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
          该分发系统无法由平台核验精确额度。前台不会展示平台实时校验或平台已核验余额。
        </p>
      </div>

      <div v-if="form.distributionSystem === 'other'" class="space-y-2">
        <label class="text-sm font-medium">分发系统名称与说明</label>
        <Input
          :model-value="form.distributionSystemNote"
          placeholder="说明分发系统、计费核对和接入边界"
          @update:model-value="value => form.distributionSystemNote = String(value)"
        />
        <p v-if="errors.distributionSystemNote" class="text-xs text-destructive">{{ errors.distributionSystemNote }}</p>
      </div>
    </div>
  </Card>
</template>
