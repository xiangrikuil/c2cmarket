<script setup lang="ts">
import DeliveryModeTooltip from '@/components/api/DeliveryModeTooltip.vue'
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'
import { deliveryModeLabel, usageVisibilityLabel } from './utils'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0 shadow-sm">
    <div class="flex items-center justify-between border-b border-border px-4 py-3">
      <h2 class="text-base font-semibold">服务规则</h2>
      <span class="text-xs text-muted-foreground">规则完整</span>
    </div>
    <dl class="grid gap-x-8 px-4 py-3 text-sm md:grid-cols-2">
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">接入方式</dt>
        <dd class="space-y-1 font-semibold">
          <span v-for="mode in service.deliveryModes" :key="mode" class="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs">
            {{ deliveryModeLabel(mode) }}
            <DeliveryModeTooltip :mode="mode" />
          </span>
        </dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">用量查看</dt>
        <dd class="font-semibold">{{ usageVisibilityLabel(service.usageVisibility) }}</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">最低意向</dt>
        <dd class="font-semibold">¥{{ service.minimumPurchaseCny }} 起</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">有效期</dt>
        <dd class="font-semibold">{{ service.expiresAt }}</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">商户承诺</dt>
        <dd class="font-semibold">{{ service.warranty }} · 平台不担保、不代赔</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">退款规则</dt>
        <dd class="font-semibold">{{ service.refundPolicy }}</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 py-3">
        <dt class="text-muted-foreground">最近完成</dt>
        <dd class="font-semibold">近 30 天完成 {{ service.completed30d }} 单</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 py-3">
        <dt class="text-muted-foreground">用量同步</dt>
        <dd class="font-semibold">{{ service.usageVisibility === 'panel_realtime' ? '商户面板站外确认' : usageVisibilityLabel(service.usageVisibility) }}</dd>
      </div>
    </dl>
  </Card>
</template>
