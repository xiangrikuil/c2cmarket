<script setup lang="ts">
import type { ApiService } from '@/lib/api'
import { formatCredit, formatCreditConversion, formatCny, formatMultiplier } from './utils'

defineProps<{
  service: ApiService
  amount: number
}>()
</script>

<template>
  <dl class="rounded-lg border border-border bg-muted/40 text-sm">
    <div class="flex justify-between gap-4 border-b border-border px-3 py-2.5">
      <dt class="text-muted-foreground">意向金额</dt>
      <dd class="font-semibold text-primary">{{ formatCny(amount) }}</dd>
    </div>
    <div class="flex justify-between gap-4 border-b border-border px-3 py-2.5">
      <dt class="text-muted-foreground">意向额度上限</dt>
      <dd class="font-semibold">{{ formatCredit(Math.round(amount * service.creditPerCny)) }}</dd>
    </div>
    <div class="flex justify-between gap-4 border-b border-border px-3 py-2.5">
      <dt class="text-muted-foreground">锁定倍率</dt>
      <dd class="font-semibold">{{ formatMultiplier(service.defaultMultiplier) }}</dd>
    </div>
    <div class="flex justify-between gap-4 border-b border-border px-3 py-2.5">
      <dt class="text-muted-foreground">额度参考</dt>
      <dd class="font-semibold">{{ formatCreditConversion(service) }}</dd>
    </div>
    <div class="flex justify-between gap-4 border-b border-border px-3 py-2.5">
      <dt class="text-muted-foreground">有效期</dt>
      <dd class="font-semibold">{{ service.expiresAt }}</dd>
    </div>
    <div class="flex justify-between gap-4 px-3 py-2.5">
      <dt class="text-muted-foreground">商户承诺</dt>
      <dd class="text-right font-semibold">{{ service.warranty }}</dd>
    </div>
  </dl>
</template>
