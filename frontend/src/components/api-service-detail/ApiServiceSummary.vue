<script setup lang="ts">
import { CalendarDays, CircleDollarSign, Layers3, Tag } from 'lucide-vue-next'
import type { ApiService } from '@/lib/api'
import { formatCny, formatCnyPerUsdQuota, formatCredit, formatMultiplier } from './utils'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <section class="api-service-detail-summary flex h-full flex-col rounded-xl border border-border bg-card p-5 shadow-sm md:p-7">
    <div class="flex flex-wrap items-end gap-x-8 gap-y-4 border-b border-border pb-6">
      <div>
        <div class="text-xs font-medium text-muted-foreground">美元额度售价</div>
        <div class="mt-2 text-4xl font-semibold tracking-tight text-primary md:text-5xl">{{ formatCnyPerUsdQuota(service) }}</div>
      </div>
      <div class="border-l border-border pl-8">
        <div class="text-xs font-medium text-muted-foreground">商户倍率</div>
        <div class="mt-2 text-3xl font-semibold tracking-tight">{{ formatMultiplier(service.defaultMultiplier) }}</div>
      </div>
    </div>

    <dl class="grid gap-4 border-b border-border py-6 text-sm sm:grid-cols-2 xl:grid-cols-4">
      <div class="flex items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-primary/8 text-primary"><CircleDollarSign class="h-4 w-4" /></span>
        <div><dt class="text-xs text-muted-foreground">可售额度</dt><dd class="mt-1 font-semibold">{{ formatCredit(service.balance) }}</dd></div>
      </div>
      <div class="flex items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground"><Tag class="h-4 w-4" /></span>
        <div><dt class="text-xs text-muted-foreground">最低订单</dt><dd class="mt-1 font-semibold">{{ formatCny(service.minimumPurchaseCny) }}</dd></div>
      </div>
      <div class="flex items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground"><CalendarDays class="h-4 w-4" /></span>
        <div><dt class="text-xs text-muted-foreground">API 额度有效期</dt><dd class="mt-1 font-semibold">{{ service.expiresAt }}</dd></div>
      </div>
      <div class="flex items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground"><Layers3 class="h-4 w-4" /></span>
        <div><dt class="text-xs text-muted-foreground">接入类型</dt><dd class="mt-1 font-semibold">{{ service.delivery }}</dd></div>
      </div>
    </dl>

    <div class="api-service-value-guide">
      <div><span>充值汇率</span><strong>{{ formatCnyPerUsdQuota(service) }}</strong><small>人民币换取商户声明的美元额度</small></div>
      <div><span>模型消耗倍率</span><strong>{{ formatMultiplier(service.defaultMultiplier) }}</strong><small>实际模型用量按服务规则扣减</small></div>
      <div><span>支持模型</span><strong>{{ service.models.length }} 个</strong><small>{{ service.models.slice(0, 3).join(' / ') }}</small></div>
    </div>

    <div class="pt-6">
      <h2 class="text-base font-semibold">服务说明</h2>
      <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ service.merchantNote }}</p>
    </div>
  </section>
</template>
