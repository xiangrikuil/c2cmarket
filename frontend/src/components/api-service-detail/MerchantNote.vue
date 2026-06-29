<script setup lang="ts">
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'
import MerchantMiniProfile from './MerchantMiniProfile.vue'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0 shadow-sm">
    <div class="flex items-center justify-between border-b border-border px-4 py-3">
      <h2 class="text-base font-semibold">商户备注</h2>
      <span class="text-xs text-muted-foreground">最近活跃 {{ service.responseMedianMinutes }} 分钟前</span>
    </div>
    <div class="space-y-3 p-4">
      <MerchantMiniProfile :service="service" />
      <p class="rounded-lg bg-muted/50 p-3 text-sm leading-6 text-foreground">
        {{ service.merchantNote }}
      </p>
    </div>
  </Card>
</template>
