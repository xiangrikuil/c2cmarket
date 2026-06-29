<script setup lang="ts">
import type { ApiService } from '@/lib/api'
import { formatCny, formatCnyPerUsdQuota, formatCredit, formatMultiplier, formatPercentMultiplier } from './utils'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <section class="rounded-xl border border-border bg-card px-4 py-3 shadow-sm">
    <div class="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-5">
      <div>
        <div class="text-xs text-muted-foreground">商户服务倍率</div>
        <div class="mt-1 font-semibold text-primary">{{ formatMultiplier(service.defaultMultiplier) }}</div>
        <div class="mt-1 text-xs text-muted-foreground">官方模型价格的 {{ formatPercentMultiplier(service.defaultMultiplier) }}</div>
      </div>
      <div>
        <div class="text-xs text-muted-foreground">美元额度售价</div>
        <div class="mt-1 font-semibold">{{ formatCnyPerUsdQuota(service) }}</div>
        <div class="mt-1 text-xs text-muted-foreground">商户可售额度参考，不由平台发放</div>
      </div>
      <div>
        <div class="text-xs text-muted-foreground">最低意向</div>
        <div class="mt-1 font-semibold">{{ formatCny(service.minimumPurchaseCny) }} 起</div>
        <div class="mt-1 text-xs text-muted-foreground">建议首次小额测试</div>
      </div>
      <div>
        <div class="text-xs text-muted-foreground">可售美元额度</div>
        <div class="mt-1 font-semibold">{{ formatCredit(service.balance) }}</div>
        <div class="mt-1 text-xs text-muted-foreground">商户维护 · {{ service.lastOnlineConfirmedAt.slice(5) }} 确认</div>
      </div>
      <div>
        <div class="text-xs text-muted-foreground">接单状态</div>
        <div class="mt-1 flex items-center gap-2 font-semibold">
          <span class="h-2 w-2 rounded-full" :class="service.publiclyOrderable ? 'bg-emerald-500' : 'bg-muted-foreground/50'" />
          {{ service.publiclyOrderable ? `可提交意向 · 约 ${service.expectedResponseMinutes} 分钟响应` : '暂不可接单' }}
        </div>
        <div class="mt-1 text-xs text-muted-foreground">提交后立即展示商户联系方式</div>
      </div>
    </div>
  </section>
</template>
