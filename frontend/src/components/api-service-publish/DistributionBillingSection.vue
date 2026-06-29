<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm, BillingMode, DistributionSystem } from './types'
import { billingLabels } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  setDistribution: [value: DistributionSystem]
  setBilling: [value: BillingMode]
}>()

const billingOptions = computed<BillingMode[]>(() => props.form.distributionSystem === 'sub2api' ? ['metered_credit'] : ['manual_credit', 'fixed_package'])
const distributionOptions: Array<{ value: DistributionSystem, title: string, description: string, detail: string }> = [
  {
    value: 'sub2api',
    title: 'Sub2API',
    description: '文本模型倍率和生图倍率固定 1.00x。',
    detail: '商户配置额度售价、模型、接入方式、用量可见性、库存、有效期和商户承诺。',
  },
  {
    value: 'new_api_proxy',
    title: 'NewAPI Proxy',
    description: '适用于固定套餐或商户确认用量。',
    detail: '不进入 Sub2API 标准额度榜单，平台不核验精确美元余额。',
  },
  {
    value: 'other',
    title: '其他',
    description: '需要说明分发系统、计费方式和用量查看方式。',
    detail: '进入人工审核，仅展示请求地址接入说明。',
  },
]
function billingDisabled(value: BillingMode) {
  return props.form.distributionSystem === 'new_api_proxy' && value === 'metered_credit'
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
        <div class="api-publish-option-grid">
          <button
            v-for="option in distributionOptions"
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
            :disabled="billingDisabled(option)"
            @click="emit('setBilling', option)"
          >
            {{ billingLabels[option] }}
          </button>
        </div>
        <p v-if="form.distributionSystem !== 'sub2api'" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
          该分发系统无法由平台核验精确额度。前台将标注“商户确认用量”，不能展示“实时用量”或“平台已核验”。
        </p>
      </div>

      <div v-if="form.distributionSystem === 'other'" class="space-y-2">
        <label class="text-sm font-medium">分发系统名称与说明</label>
        <Input
          :model-value="form.distributionSystemNote"
          placeholder="说明分发系统、用量查看和接入边界"
          @update:model-value="value => form.distributionSystemNote = String(value)"
        />
        <p v-if="errors.distributionSystemNote" class="text-xs text-destructive">{{ errors.distributionSystemNote }}</p>
      </div>
    </div>
  </Card>
</template>
