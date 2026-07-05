<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm, BillingMode, DistributionSystem } from './types'
import { billingLabels, publishDistributionOptions } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  setDistribution: [value: DistributionSystem]
  setBilling: [value: BillingMode]
}>()

const billingOptions = computed<BillingMode[]>(() => props.form.distributionSystem === 'sub2api' ? ['metered_credit'] : ['manual_credit', 'fixed_package'])
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
        <div class="api-publish-option-grid">
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
      </div>

      <div class="space-y-2">
        <label class="text-sm font-medium">售卖计费方式</label>
        <div class="api-publish-billing-grid">
          <button
            v-for="option in billingOptions"
            :key="option"
            type="button"
            class="rounded-md border px-3 py-2 text-left text-sm font-medium disabled:cursor-not-allowed disabled:opacity-45"
            :class="form.billingMode === option ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="emit('setBilling', option)"
          >
            {{ billingLabels[option] }}
          </button>
        </div>
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
