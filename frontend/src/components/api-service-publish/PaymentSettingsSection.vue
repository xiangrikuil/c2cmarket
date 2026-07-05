<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { ApiServicePublishForm } from './types'
import { enabledPaymentOptions, publishPaymentMethods } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const enabledCount = computed(() => enabledPaymentOptions(props.form).length)
const missingInstructionCount = computed(() => enabledPaymentOptions(props.form).filter(option => !option.paymentInstructions.trim()).length)
const readyForOrders = computed(() => enabledCount.value > 0 && missingInstructionCount.value === 0)
const statusMessage = computed(() => {
  if (!enabledCount.value) return '未启用收款方式时不能发布；请先选择一种方式并填写站外收款说明。'
  if (missingInstructionCount.value) return '已启用收款方式，还需要填写对应的站外收款说明。'
  return `已配置 ${enabledCount.value} 种收款方式，发布后会直接进入公开服务列表。`
})

function setPaymentWindow(value: string | number) {
  const minutes = Number(value)
  props.form.paymentWindowMinutes = Number.isFinite(minutes) ? minutes : 0
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>2. 收款与接单</h2>
      <p>至少启用一种站外收款方式，发布后买家才能提交购买意向。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div
        class="rounded-md border px-3 py-2 text-xs leading-5"
        :class="readyForOrders ? 'border-success/20 bg-success/5 text-success' : 'border-warning/25 bg-warning/10 text-warning'"
      >
        {{ statusMessage }}
      </div>

      <label class="block max-w-xs space-y-2">
        <span class="text-sm font-medium">买家确认付款窗口</span>
        <div class="flex overflow-hidden rounded-md border border-input bg-background">
          <Input
            type="number"
            min="3"
            max="15"
            :model-value="form.paymentWindowMinutes"
            class="border-0 shadow-none focus-visible:ring-0"
            @update:model-value="setPaymentWindow"
          />
          <span class="grid w-14 place-items-center border-l border-border text-sm text-muted-foreground">分钟</span>
        </div>
        <p v-if="errors.paymentWindowMinutes" class="text-xs text-destructive">{{ errors.paymentWindowMinutes }}</p>
        <p v-else class="text-xs text-muted-foreground">后端要求 3-15 分钟；平台只记录意向窗口，不托管支付。</p>
      </label>

      <div class="api-publish-payment-grid">
        <div
          v-for="option in form.paymentOptions"
          :key="option.paymentMethod"
          class="api-publish-payment-card"
          :class="option.enabled ? 'is-active' : ''"
        >
          <label class="flex cursor-pointer items-start gap-3">
            <input v-model="option.enabled" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
            <span class="min-w-0">
              <strong>{{ publishPaymentMethods.find(item => item.value === option.paymentMethod)?.label }}</strong>
              <span>{{ publishPaymentMethods.find(item => item.value === option.paymentMethod)?.hint }}</span>
            </span>
          </label>

          <Textarea
            v-if="option.enabled"
            v-model="option.paymentInstructions"
            class="mt-3 min-h-24 text-sm"
            maxlength="160"
            placeholder="例如：提交意向后通过商户联系方式确认收款信息。不要填写收款码、银行卡号或完整账号。"
          />
          <p v-if="option.enabled && !option.paymentInstructions.trim()" class="mt-2 text-xs text-warning">启用后必须填写收款说明。</p>
        </div>
      </div>

      <p v-if="errors.paymentOptions" class="text-xs text-destructive">{{ errors.paymentOptions }}</p>
      <p class="rounded-md border border-border bg-muted/50 px-3 py-2 text-xs leading-5 text-muted-foreground">
        收款说明只写站外确认方式，不填写收款码、付款码、银行卡号、API Key、token、账号密码或面板凭据。
      </p>
    </div>
  </Card>
</template>
