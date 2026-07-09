<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm } from './types'

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>1. 额度与价格</h2>
      <p>填写售价、可售额度和有效时间。</p>
    </div>

    <div class="api-publish-card-body">
      <div class="grid gap-4 md:grid-cols-3">
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

        <label class="space-y-2">
          <span class="text-sm font-medium">额度有效至</span>
          <Input
            :model-value="form.quotaExpiresAt"
            type="datetime-local"
            @update:model-value="value => form.quotaExpiresAt = String(value)"
          />
          <p v-if="errors.quotaExpiresAt" class="text-xs text-destructive">{{ errors.quotaExpiresAt }}</p>
          <p v-else class="text-xs text-muted-foreground">买家按该时间判断可用窗口。</p>
        </label>
      </div>
    </div>
  </Card>
</template>
