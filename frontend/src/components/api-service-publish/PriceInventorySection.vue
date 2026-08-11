<script setup lang="ts">
import { CircleDollarSign } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import ApiQuotaPolicyFields from '@/components/api-market/ApiQuotaPolicyFields.vue'
import type { ApiServicePublishForm } from './types'

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex items-start gap-2">
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-emerald-50 text-emerald-600">
          <CircleDollarSign class="h-4 w-4" />
        </span>
        <div>
          <h2>自选额度与定价</h2>
          <p>填写售价、可售额度和有效时间。</p>
        </div>
      </div>
    </div>

    <div class="api-publish-card-body">
      <div class="grid gap-3 md:grid-cols-3">
        <label class="space-y-2">
          <span class="text-sm font-medium">每 $1 美元额度售价</span>
          <div class="flex overflow-hidden rounded-md border border-input bg-background">
            <Input
              id="api-publish-cny-per-usd"
              :model-value="form.cnyPerUsdCredit ?? ''"
              :aria-invalid="Boolean(errors.cnyPerUsdCredit)"
              :aria-describedby="errors.cnyPerUsdCredit ? 'api-publish-cny-per-usd-error' : undefined"
              class="border-0 shadow-none focus-visible:ring-0"
              placeholder="0.80"
              @update:model-value="value => form.cnyPerUsdCredit = Number(value)"
            />
            <span class="grid w-14 place-items-center border-l border-border text-sm text-muted-foreground">元</span>
          </div>
          <p v-if="errors.cnyPerUsdCredit" id="api-publish-cny-per-usd-error" class="text-xs text-destructive">{{ errors.cnyPerUsdCredit }}</p>
          <p v-else class="text-xs text-muted-foreground">例如 ¥0.80 / $1，买家按订单金额估算可购额度。</p>
        </label>

        <label class="space-y-2">
          <span class="text-sm font-medium">可售美元额度</span>
          <div class="flex overflow-hidden rounded-md border border-input bg-background">
            <Input
              id="api-publish-available-credit"
              :model-value="form.availableCreditUsd ?? ''"
              :aria-invalid="Boolean(errors.availableCreditUsd)"
              :aria-describedby="errors.availableCreditUsd ? 'api-publish-available-credit-error' : undefined"
              class="border-0 shadow-none focus-visible:ring-0"
              placeholder="500"
              @update:model-value="value => form.availableCreditUsd = Number(value)"
            />
            <span class="grid w-12 place-items-center border-l border-border text-sm text-muted-foreground">$</span>
          </div>
          <p v-if="errors.availableCreditUsd" id="api-publish-available-credit-error" class="text-xs text-destructive">{{ errors.availableCreditUsd }}</p>
          <p v-else class="text-xs text-muted-foreground">表示商户声明可出售的美元额度上限，不是平台余额。</p>
        </label>

        <label class="space-y-2">
          <span class="text-sm font-medium">额度有效至</span>
          <Input
            id="api-publish-quota-expires-at"
            :model-value="form.quotaExpiresAt"
            :aria-invalid="Boolean(errors.quotaExpiresAt)"
            :aria-describedby="errors.quotaExpiresAt ? 'api-publish-quota-expires-at-error' : undefined"
            type="datetime-local"
            @update:model-value="value => form.quotaExpiresAt = String(value)"
          />
          <p v-if="errors.quotaExpiresAt" id="api-publish-quota-expires-at-error" class="text-xs text-destructive">{{ errors.quotaExpiresAt }}</p>
          <p v-else class="text-xs text-muted-foreground">买家按该时间判断可用窗口。</p>
        </label>
      </div>
      <div class="mt-4 border-t border-border pt-4">
        <ApiQuotaPolicyFields v-model="form.quotaUsagePolicy" :error="errors.quotaUsagePolicy" />
      </div>
    </div>
  </Card>
</template>
