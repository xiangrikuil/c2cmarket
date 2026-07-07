<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm } from './types'
import { apiQuotaDefaultRuleText, formatUsdQuotaForCny } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const quotaForTwentyCny = computed(() => formatUsdQuotaForCny(props.form.cnyPerUsdCredit, 20))
const quotaForFiftyCny = computed(() => formatUsdQuotaForCny(props.form.cnyPerUsdCredit, 50))
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>1. 出售额度</h2>
      <p>填写商户愿意出售的美元额度和人民币报价，最终金额由双方站外确认。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="grid gap-4 md:grid-cols-2">
        <label class="space-y-2">
          <span class="text-sm font-medium">每 $1 美元额度售价</span>
          <div class="flex overflow-hidden rounded-md border border-input bg-background">
            <Input
              :model-value="form.cnyPerUsdCredit ?? ''"
              class="border-0 shadow-none focus-visible:ring-0"
              placeholder="0.80"
              @update:model-value="value => form.cnyPerUsdCredit = Number(value)"
            />
            <span class="grid w-14 place-items-center border-l border-border text-sm text-muted-foreground">元</span>
          </div>
          <p v-if="errors.cnyPerUsdCredit" class="text-xs text-destructive">{{ errors.cnyPerUsdCredit }}</p>
          <p v-else class="text-xs text-muted-foreground">例如 ¥0.80 / $1，买家按意向金额估算可购额度。</p>
        </label>

        <label class="space-y-2">
          <span class="text-sm font-medium">可售美元额度</span>
          <div class="flex overflow-hidden rounded-md border border-input bg-background">
            <Input
              :model-value="form.availableCreditUsd ?? ''"
              class="border-0 shadow-none focus-visible:ring-0"
              placeholder="500"
              @update:model-value="value => form.availableCreditUsd = Number(value)"
            />
            <span class="grid w-12 place-items-center border-l border-border text-sm text-muted-foreground">$</span>
          </div>
          <p v-if="errors.availableCreditUsd" class="text-xs text-destructive">{{ errors.availableCreditUsd }}</p>
          <p v-else class="text-xs text-muted-foreground">表示商户声明可出售的美元额度上限，不是平台余额。</p>
        </label>
      </div>

      <label class="block space-y-2">
        <span class="text-sm font-medium">额度有效至</span>
        <Input
          :model-value="form.quotaExpiresAt"
          type="datetime-local"
          @update:model-value="value => form.quotaExpiresAt = String(value)"
        />
        <p v-if="errors.quotaExpiresAt" class="text-xs text-destructive">{{ errors.quotaExpiresAt }}</p>
        <p v-else class="text-xs text-muted-foreground">适合发布临近套餐重置前的剩余额度，买家按该时间判断可用窗口。</p>
      </label>

      <div class="api-publish-compute-grid">
        <div class="api-publish-compute-box">
          <b>{{ quotaForTwentyCny }}</b>
          <span>¥20 意向约可购</span>
        </div>
        <div class="api-publish-compute-box">
          <b>{{ quotaForFiftyCny }}</b>
          <span>¥50 意向约可购</span>
        </div>
        <div class="api-publish-compute-box">
          <b>1.00x</b>
          <span>所选模型按实际消耗额度计算</span>
        </div>
      </div>

      <p class="rounded-md border border-border bg-muted/50 px-3 py-2 text-xs leading-5 text-muted-foreground">
        {{ apiQuotaDefaultRuleText }} C2CMarket 不托管支付，不保存 API Key、token、账号密码或面板凭据。
      </p>
    </div>
  </Card>
</template>
