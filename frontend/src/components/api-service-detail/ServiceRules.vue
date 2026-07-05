<script setup lang="ts">
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'

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
        <dt class="text-muted-foreground">接入细节</dt>
        <dd class="font-semibold">提交意向后与商户站外确认</dd>
      </div>
      <div class="grid grid-cols-[96px_1fr] gap-4 border-b border-border py-3">
        <dt class="text-muted-foreground">用量核对</dt>
        <dd class="font-semibold">商户说明，买家自行核对</dd>
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
        <dt class="text-muted-foreground">平台边界</dt>
        <dd class="font-semibold">不托管凭据，不提供实时校验</dd>
      </div>
    </dl>
  </Card>
</template>
