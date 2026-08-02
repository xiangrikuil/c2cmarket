<script setup lang="ts">
import { Activity, CalendarDays, Clock3, Layers3, Network, ShieldCheck, Tag, Users } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { getApiTTFTBandLabel, type ApiService } from '@/lib/api'
import { formatCny, formatCnyPerUsdQuota, formatCredit, formatMultiplier } from './utils'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <Card class="api-service-detail-summary gap-0 overflow-hidden py-0 shadow-sm">
    <section class="p-5 md:p-6">
      <h2 class="text-base font-semibold">核心信息</h2>
      <dl class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div>
          <dt class="text-xs font-medium text-muted-foreground">购买价格</dt>
          <dd class="mt-1.5 whitespace-nowrap text-2xl font-semibold text-primary">{{ formatCnyPerUsdQuota(service) }}</dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-muted-foreground">可售额度</dt>
          <dd class="mt-1.5 text-2xl font-semibold">{{ formatCredit(service.balance) }}</dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-muted-foreground">最低购买</dt>
          <dd class="mt-1.5 text-2xl font-semibold">{{ formatCny(service.minimumPurchaseCny) }}</dd>
        </div>
        <div>
          <dt class="text-xs font-medium text-muted-foreground">模型计费倍率</dt>
          <dd class="mt-1.5 text-2xl font-semibold">{{ formatMultiplier(service.defaultMultiplier) }}</dd>
        </div>
      </dl>
    </section>

    <dl class="grid grid-cols-2 border-y border-border bg-muted/20 text-sm md:grid-cols-4">
      <div class="flex min-w-0 items-center gap-3 border-b border-r border-border p-4 md:border-b-0">
        <span class="api-service-fact-icon"><Activity class="h-4 w-4" /></span>
        <div class="min-w-0"><dt class="text-xs text-muted-foreground">首字响应</dt><dd class="mt-1 truncate font-semibold">{{ getApiTTFTBandLabel(service.declaredTtftBand) }}</dd></div>
      </div>
      <div class="flex min-w-0 items-center gap-3 border-b border-border p-4 md:border-b-0 md:border-r">
        <span class="api-service-fact-icon"><Users class="h-4 w-4" /></span>
        <div class="min-w-0"><dt class="text-xs text-muted-foreground">商户声明最大并发</dt><dd class="mt-1 truncate font-semibold">{{ service.declaredMaxConcurrency ?? '历史服务未声明' }}</dd></div>
      </div>
      <div class="flex min-w-0 items-center gap-3 border-r border-border p-4">
        <span class="api-service-fact-icon"><Clock3 class="h-4 w-4" /></span>
        <div class="min-w-0"><dt class="text-xs text-muted-foreground">付款窗口</dt><dd class="mt-1 truncate font-semibold">{{ service.expectedResponseMinutes }} 分钟</dd></div>
      </div>
      <div class="flex min-w-0 items-center gap-3 p-4">
        <span class="api-service-fact-icon"><Network class="h-4 w-4" /></span>
        <div class="min-w-0"><dt class="text-xs text-muted-foreground">号池</dt><dd class="mt-1 break-words font-semibold" :title="service.accountPoolLabel">{{ service.accountPoolLabel || '历史服务未补充' }}</dd></div>
      </div>
    </dl>

    <section class="p-5 md:p-6">
      <h2 class="text-base font-semibold">服务说明</h2>
      <p class="mt-2 whitespace-pre-line text-sm leading-6 text-muted-foreground">{{ service.merchantNote }}</p>
      <dl class="mt-5 grid gap-4 border-t border-border pt-5 text-sm sm:grid-cols-2 xl:grid-cols-4">
        <div class="flex items-start gap-2.5"><Layers3 class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><dt class="text-xs text-muted-foreground">接入类型</dt><dd class="mt-1 font-semibold">{{ service.delivery }}</dd></div></div>
        <div class="flex items-start gap-2.5"><CalendarDays class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><dt class="text-xs text-muted-foreground">服务有效期</dt><dd class="mt-1 font-semibold">{{ service.expiresAt }}</dd></div></div>
        <div class="flex items-start gap-2.5"><Tag class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><dt class="text-xs text-muted-foreground">支持模型</dt><dd class="mt-1 font-semibold">{{ service.models.length }} 个</dd></div></div>
        <div class="flex items-start gap-2.5"><ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><dt class="text-xs text-muted-foreground">商户退款承诺</dt><dd class="mt-1 font-semibold">{{ service.merchantRefundCommitment ? '商户全额退款承诺' : '无额外退款承诺' }}</dd></div></div>
      </dl>
    </section>
  </Card>
</template>
