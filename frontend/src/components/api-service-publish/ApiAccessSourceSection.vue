<script setup lang="ts">
import { computed } from 'vue'
import { Network } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm, DistributionSystem, SellingMode } from './types'
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
          <p>{{ isLimitedQuotaMode ? '额度包倍率在下一步独立设置，默认 1.00x。' : isFixedPackageMode ? '设置固定额度包服务的默认倍率。' : '设置自由额度服务的默认倍率。' }}</p>
        </div>
      </div>
    </div>

    <div class="api-publish-card-body space-y-2">
      <div class="space-y-2">
        <label class="text-sm font-medium">接入类型</label>
        <div class="grid gap-2 sm:grid-cols-2">
          <button
            v-for="option in publishDistributionOptions"
            :key="option.value"
            type="button"
            class="api-publish-option-card"
            :class="{ 'is-active': form.distributionSystem === option.value }"
            :aria-invalid="Boolean(errors.distributionSystem)"
            :aria-describedby="errors.distributionSystem ? 'api-publish-distribution-error' : undefined"
            @click="emit('setDistribution', option.value)"
          >
            <span class="block text-sm font-semibold">{{ option.title }}</span>
            <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
            <span class="mt-2 block text-[11px] leading-5 text-muted-foreground">{{ option.detail }}</span>
          </button>
        </div>
        <p v-if="errors.distributionSystem" id="api-publish-distribution-error" class="text-xs text-destructive">{{ errors.distributionSystem }}</p>
      </div>

      <div v-if="form.distributionSystem === 'sub2api'" class="rounded-md border border-primary/20 bg-primary/5 px-3 py-1.5">
        <div class="text-sm font-semibold text-primary">{{ formatMultiplier(1) }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ isLimitedQuotaMode ? '这是基础服务默认值；下一步可为每个额度包填写真实模型倍率。' : isFixedPackageMode ? 'Sub2API 固定额度包按 1.00x 默认倍率展示。' : 'Sub2API 自由额度服务按 1.00x 默认倍率展示。' }}</p>
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
        <p v-else class="text-xs text-muted-foreground">用于前台价格折算；提交后写入服务倍率快照。</p>
      </div>

    </div>
  </Card>
</template>
