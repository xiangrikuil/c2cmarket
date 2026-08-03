<script setup lang="ts">
import { computed } from 'vue'
import { CalendarClock, Clock3, PackageCheck, Store } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { formatDecimal } from '@/lib/decimal'

const props = defineProps<{
  step: number
  serviceTitle?: string
  slotLabel?: string
  defaultMultiplier: number
  draft: {
    name: string
    usdAllowance: string
    priceCny: string
    copies: number
    deliveryMode: 'manual' | 'preimported'
    deliveryEtaMinutes: number
    expiresAt: string
  }
}>()

const cnyPerUsd = computed(() => {
  const usd = Number(props.draft.usdAllowance)
  const cny = Number(props.draft.priceCny)
  return usd > 0 && cny > 0 ? cny / usd : 0
})
const totalUsd = computed(() => Number(props.draft.usdAllowance || 0) * Number(props.draft.copies || 0))
const completionPercent = computed(() => Math.round((props.step / 3) * 100))
</script>

<template>
  <div class="min-w-0 space-y-2">
    <div class="rounded-md border border-border bg-card px-3 py-2.5">
      <div class="flex items-center justify-between gap-3 text-xs"><span class="font-medium">发布进度</span><strong>{{ completionPercent }}%</strong></div>
      <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full bg-primary transition-[width]" :style="{ width: `${completionPercent}%` }" /></div>
    </div>

    <Card class="overflow-hidden p-0">
      <div class="border-b border-orange-100 bg-orange-50/70 p-4">
        <div class="flex items-center justify-between gap-3">
          <span class="flex items-center gap-2 text-xs font-medium text-orange-800"><PackageCheck class="h-4 w-4" />买家额度包卡片</span>
          <Badge variant="status">固定场次</Badge>
        </div>
        <h2 class="mt-3 break-words text-lg font-semibold">{{ draft.name || '限时额度包' }}</h2>
        <p class="mt-1 break-words text-sm text-muted-foreground">{{ serviceTitle || '待选择基础服务' }}</p>
      </div>

      <div class="grid grid-cols-2 gap-3 p-4 text-sm">
        <div><span class="text-xs text-muted-foreground">单份额度</span><strong class="mt-1 block text-xl text-primary">${{ formatDecimal(draft.usdAllowance || '0', 0, 6) }}</strong></div>
        <div><span class="text-xs text-muted-foreground">人民币总价</span><strong class="mt-1 block text-xl">¥{{ formatDecimal(draft.priceCny || '0', 2, 2) }}</strong></div>
        <div><span class="text-xs text-muted-foreground">折算售价</span><strong class="mt-1 block">¥{{ cnyPerUsd.toFixed(3) }} / $1</strong></div>
        <div><span class="text-xs text-muted-foreground">库存 / 总额度</span><strong class="mt-1 block">{{ draft.copies }} 份 / ${{ formatDecimal(String(totalUsd), 0, 6) }}</strong></div>
        <div><span class="text-xs text-muted-foreground">服务倍率</span><strong class="mt-1 block">{{ defaultMultiplier.toFixed(2) }}x</strong></div>
        <div><span class="text-xs text-muted-foreground">交付</span><strong class="mt-1 block">{{ draft.deliveryMode === 'manual' ? `手工 ≤ ${draft.deliveryEtaMinutes} 分钟` : '预导入凭据' }}</strong></div>
      </div>

      <div v-if="step >= 3" class="grid gap-3 border-t border-border px-4 py-3 text-sm">
        <div class="flex gap-2"><CalendarClock class="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span><small class="block text-muted-foreground">开抢场次</small><strong>{{ slotLabel || '待选择' }}</strong></span></div>
        <div class="flex gap-2"><Clock3 class="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span><small class="block text-muted-foreground">绝对失效</small><strong>{{ draft.expiresAt ? draft.expiresAt.replace('T', ' ') : '待填写' }}</strong></span></div>
      </div>

      <div class="flex gap-2 border-t border-border bg-muted/35 px-4 py-3 text-xs leading-5 text-muted-foreground">
        <Store class="mt-0.5 h-4 w-4 shrink-0" />
        <span>平台记录订单，不代收款。商户自报额度与性能，平台未测速、未验证上游余额。</span>
      </div>
    </Card>
  </div>
</template>
