<script setup lang="ts">
import { ClipboardList, ShoppingCart, WalletCards } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'
import { formatDecimal } from '@/lib/decimal'
import {
  apiServiceAvailableUsdAllowance,
  formatCnyPerUsdQuota,
} from '@/components/api-service-detail/utils'

defineProps<{ service: ApiService }>()
</script>

<template>
  <section aria-label="核心经营指标" class="grid gap-3 md:grid-cols-3">
    <Card class="flex min-h-32 items-center gap-4 rounded-lg p-5 shadow-xs">
      <span class="grid h-14 w-14 shrink-0 place-items-center rounded-full bg-primary/10 text-primary">
        <WalletCards class="h-6 w-6" />
      </span>
      <div class="min-w-0">
        <div class="text-sm text-muted-foreground">可售额度</div>
        <div class="mt-1 text-[28px] leading-tight font-semibold">
          ${{ formatDecimal(apiServiceAvailableUsdAllowance(service), 0, 6) }}
        </div>
        <p class="mt-1 text-xs text-muted-foreground">当前可用于销售</p>
      </div>
    </Card>

    <Card class="flex min-h-32 items-center gap-4 rounded-lg p-5 shadow-xs">
      <span class="grid h-14 w-14 shrink-0 place-items-center rounded-full bg-success/10 text-success">
        <ShoppingCart class="h-6 w-6" />
      </span>
      <div class="min-w-0">
        <div class="text-sm text-muted-foreground">销售价格</div>
        <div class="mt-1 text-[28px] leading-tight font-semibold">{{ formatCnyPerUsdQuota(service) }}</div>
        <p class="mt-1 text-xs text-muted-foreground">最低 ¥{{ formatDecimal(service.minimumPurchaseCny, 0, 2) }} 起购</p>
      </div>
    </Card>

    <Card class="flex min-h-32 items-center gap-4 rounded-lg p-5 shadow-xs">
      <span class="grid h-14 w-14 shrink-0 place-items-center rounded-full bg-signal-soft text-signal">
        <ClipboardList class="h-6 w-6" />
      </span>
      <div class="min-w-0">
        <div class="text-sm text-muted-foreground">今日订单</div>
        <div class="mt-1 text-[28px] leading-tight font-semibold">{{ service.todayOrderCount }} / {{ service.dailyOrderLimit }}</div>
        <p class="mt-1 text-xs text-muted-foreground">已接单 / 每日上限</p>
      </div>
    </Card>
  </section>
</template>
