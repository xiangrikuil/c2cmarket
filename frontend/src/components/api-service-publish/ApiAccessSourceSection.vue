<script setup lang="ts">
import { computed } from 'vue'
import type { AcceptableValue } from 'reka-ui'
import { Network } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { sellingModeLabels, type ApiServicePublishForm, type DistributionSystem, type SellingMode } from './types'
import { formatMultiplier, publishDistributionOptions } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
  sellingMode: SellingMode
}>()

const isLimitedQuotaMode = computed(() => props.sellingMode === 'limited')
const isFixedPackageMode = computed(() => !isLimitedQuotaMode.value && props.form.billingMode === 'fixed_package')

const emit = defineEmits<{
  setDistribution: [value: DistributionSystem]
  setDefaultMultiplier: [value: string]
}>()

function setDistribution(value: AcceptableValue) {
  if (value === 'sub2api' || value === 'other') emit('setDistribution', value)
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex items-start gap-2">
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-violet-50 text-violet-600">
          <Network class="h-4 w-4" />
        </span>
        <div>
          <h2>接入类型与默认倍率</h2>
          <p>{{ isLimitedQuotaMode ? `设置基础服务倍率，后续${sellingModeLabels.limited}自动继承。` : isFixedPackageMode ? `设置${sellingModeLabels.package}服务的统一倍率。` : `设置${sellingModeLabels.free}服务的统一倍率。` }}</p>
        </div>
      </div>
    </div>

    <div class="api-publish-card-body space-y-2">
      <div class="space-y-2">
        <label class="text-sm font-medium">接入类型</label>
        <RadioGroup
          :model-value="form.distributionSystem"
          class="grid gap-2 sm:grid-cols-2"
          aria-label="接入类型"
          :aria-invalid="Boolean(errors.distributionSystem)"
          :aria-describedby="errors.distributionSystem ? 'api-publish-distribution-error' : undefined"
          @update:model-value="setDistribution"
        >
          <Label
            v-for="option in publishDistributionOptions"
            :key="option.value"
            :for="`api-access-distribution-${option.value}`"
            class="api-publish-option-card cursor-pointer items-start gap-2 font-normal"
            :class="{ 'is-active': form.distributionSystem === option.value }"
          >
            <RadioGroupItem :id="`api-access-distribution-${option.value}`" :value="option.value" class="mt-0.5" />
            <span class="block text-sm font-semibold">{{ option.title }}</span>
            <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            <span class="mt-2 block text-[11px] leading-5 text-muted-foreground">{{ option.detail }}</span>
          </Label>
        </RadioGroup>
        <p v-if="errors.distributionSystem" id="api-publish-distribution-error" class="text-xs text-destructive">{{ errors.distributionSystem }}</p>
      </div>

      <div v-if="form.distributionSystem === 'sub2api'" class="rounded-md border border-primary/20 bg-primary/5 px-3 py-1.5">
        <div class="text-sm font-semibold text-primary">{{ formatMultiplier(1) }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ isLimitedQuotaMode ? `Sub2API 服务、模型和后续${sellingModeLabels.limited}统一使用 1.00x。` : isFixedPackageMode ? `Sub2API 服务、模型和${sellingModeLabels.package}统一使用 1.00x。` : 'Sub2API 服务及其模型统一使用 1.00x。' }}</p>
      </div>

      <div v-else class="space-y-2">
        <label class="text-sm font-medium">默认服务倍率</label>
        <div class="flex max-w-xs overflow-hidden rounded-md border border-input bg-background">
          <Input
            id="api-publish-default-multiplier"
            :model-value="Number.isFinite(form.defaultMultiplier) ? form.defaultMultiplier : ''"
            :aria-invalid="Boolean(errors.defaultMultiplier)"
            :aria-describedby="errors.defaultMultiplier ? 'api-publish-default-multiplier-error' : undefined"
            class="border-0 shadow-none focus-visible:ring-0"
            min="0.01"
            step="0.01"
            type="number"
            placeholder="1.00"
            @update:model-value="value => emit('setDefaultMultiplier', String(value))"
          />
          <span class="grid w-12 place-items-center border-l border-border text-sm text-muted-foreground">x</span>
        </div>
        <p v-if="errors.defaultMultiplier" id="api-publish-default-multiplier-error" class="text-xs text-destructive">{{ errors.defaultMultiplier }}</p>
        <p v-else class="text-xs text-muted-foreground">模型、{{ sellingModeLabels.package }}和{{ sellingModeLabels.limited }}统一继承该倍率，并写入交易快照。</p>
      </div>

    </div>
  </Card>
</template>
