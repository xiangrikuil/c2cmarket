<script setup lang="ts">
import { CheckCircle2, CircleDollarSign, ShieldCheck } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'
import ModelPriceTable from './ModelPriceTable.vue'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <div class="space-y-4">
    <Card class="gap-0 overflow-hidden py-0 shadow-sm">
      <div class="flex items-start gap-3 border-b border-border px-5 py-4 md:px-6">
        <span class="api-service-section-icon"><CircleDollarSign class="h-4 w-4" /></span>
        <div><h2 class="font-semibold">模型价格</h2><p class="mt-1 text-xs text-muted-foreground">模型消耗按商户声明倍率计算</p></div>
      </div>
      <div class="py-3"><ModelPriceTable :service="service" /></div>
    </Card>

    <Card class="p-5 shadow-sm md:p-6">
      <div class="flex items-start gap-3">
        <span class="api-service-section-icon"><ShieldCheck class="h-4 w-4" /></span>
        <div><h2 class="font-semibold">购买须知</h2><p class="mt-1 text-xs text-muted-foreground">以下内容适用于当前服务订单</p></div>
      </div>
      <ul class="mt-4 grid gap-3 text-sm leading-6 text-muted-foreground">
        <li class="flex gap-2.5"><CheckCircle2 class="mt-1 h-4 w-4 shrink-0 text-primary" /><span>创建订单时会冻结金额、模型、倍率、号池、有效期与商户退款承诺等商品事实。</span></li>
        <li class="flex gap-2.5"><CheckCircle2 class="mt-1 h-4 w-4 shrink-0 text-primary" /><span>付款通过商户在订单中锁定的收款方式完成，平台记录交易状态但不代收或托管资金。</span></li>
        <li class="flex gap-2.5"><CheckCircle2 class="mt-1 h-4 w-4 shrink-0 text-primary" /><span>首字响应和最大并发为商户声明，平台未进行统一测速。</span></li>
      </ul>
    </Card>
  </div>
</template>
