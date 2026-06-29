<script setup lang="ts">
import DeliveryModeTooltip from '@/components/api/DeliveryModeTooltip.vue'
import { Card } from '@/components/ui/card'
import type { ApiServicePublishForm, PublishDeliveryMode } from './types'
import { deliveryLabels } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  toggleDelivery: [value: PublishDeliveryMode]
}>()

const deliveryOptions: PublishDeliveryMode[] = ['api_key_endpoint', 'sub2api_panel_account']

function visibleDeliveryOptions() {
  return props.form.distributionSystem === 'sub2api' ? deliveryOptions : ['api_key_endpoint'] as PublishDeliveryMode[]
}

function deliveryDescription(value: PublishDeliveryMode) {
  if (value === 'api_key_endpoint') return '买家提交意向后，双方站外确认请求地址和接入细节；平台不保存、不展示 API Key 或 endpoint 密钥。'
  return '买家提交意向后，双方站外确认面板接入方式；平台不保存、不展示面板账号、密码、token 或登录态。'
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>4. 接入方式</h2>
      <p>删除重复组合项，只保留两个独立、清晰的接入方式。</p>
    </div>

    <div class="api-publish-card-body">
      <div class="api-publish-delivery-grid">
        <button
          v-for="option in visibleDeliveryOptions()"
          :key="option"
          type="button"
          class="api-publish-delivery-card"
          :class="{ 'is-active': form.deliveryModes.includes(option) }"
          @click="emit('toggleDelivery', option)"
        >
          <span class="flex items-start justify-between gap-3">
            <span class="text-sm font-semibold">{{ deliveryLabels[option] }}</span>
            <DeliveryModeTooltip :mode="option" />
          </span>
          <span class="mt-2 block text-xs leading-5 text-muted-foreground">{{ deliveryDescription(option) }}</span>
          <span class="mt-3 block text-[11px] font-semibold" :class="form.deliveryModes.includes(option) ? 'text-primary' : 'text-muted-foreground'">
            {{ form.deliveryModes.includes(option) ? '已选择' : '点击选择' }}
          </span>
        </button>
      </div>

      <p v-if="errors.deliveryModes" class="mt-2 text-xs text-destructive">{{ errors.deliveryModes }}</p>
      <p class="mt-3 text-xs leading-5 text-muted-foreground">
        可以同时支持两种方式，但买家提交意向时只选择一种。NewAPI Proxy 和其他系统只显示请求地址接入说明；平台只记录站外确认状态，不保存实际 API Key、endpoint 密钥或面板凭据。
      </p>
    </div>
  </Card>
</template>
