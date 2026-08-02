<script setup lang="ts">
import { computed } from 'vue'
import {
  Boxes,
  CalendarClock,
  CircleGauge,
  Clock3,
  FileText,
  Gauge,
  Info,
  PlugZap,
  ShieldCheck,
  UsersRound,
  WalletCards,
  Zap,
} from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { getApiTTFTBandLabel, type ApiService } from '@/lib/api'
import { apiPaymentMethodLabels } from '@/lib/apiPaymentSettings'
import { formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'

const props = defineProps<{ service: ApiService }>()

const quotaExpiration = computed(() => {
  if (props.service.quotaExpiresAt) return formatQuotaExpiresAtLabel(props.service.quotaExpiresAt) || props.service.expiresAt
  return props.service.expiresAt
})
</script>

<template>
  <Card class="rounded-lg p-5 shadow-xs md:p-6">
    <div class="flex items-center gap-2">
      <FileText class="h-5 w-5 text-primary" />
      <h2 class="text-lg font-semibold">服务概览</h2>
      <Popover>
        <PopoverTrigger as-child>
          <Button variant="ghost" size="icon-sm" aria-label="查看订单快照规则">
            <Info class="h-4 w-4 text-muted-foreground" />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" class="w-[min(22rem,calc(100vw-2rem))] text-sm leading-6 text-muted-foreground">
          价格、额度、模型和付款规则的修改只影响新订单；已有订单继续使用创建时冻结的服务、金额、额度和联系方式快照。
        </PopoverContent>
      </Popover>
    </div>

    <div class="mt-5 grid gap-6 lg:grid-cols-2 lg:gap-0">
      <dl class="grid content-start gap-4 lg:pr-8">
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <PlugZap class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">接入类型</dt>
          <dd class="text-right font-medium break-words">{{ service.delivery }}</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <Boxes class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">支持模型</dt>
          <dd class="text-right font-medium break-words">{{ service.models.join(' / ') || '暂未声明' }}</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <Gauge class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">服务倍率</dt>
          <dd class="text-right font-medium">{{ service.rate }}</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <CalendarClock class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">额度有效期</dt>
          <dd class="text-right font-medium">{{ quotaExpiration }}</dd>
        </div>
      </dl>

      <dl class="grid content-start gap-4 border-border lg:border-l lg:pl-8">
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <Clock3 class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">付款窗口</dt>
          <dd class="text-right font-medium">{{ service.expectedResponseMinutes }} 分钟</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <Zap class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">首字响应</dt>
          <dd class="text-right font-medium">{{ getApiTTFTBandLabel(service.declaredTtftBand) }}</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <UsersRound class="mt-0.5 h-4 w-4 text-muted-foreground" />
		  <dt class="text-muted-foreground">商户声明最大并发</dt>
          <dd class="text-right font-medium">{{ service.declaredMaxConcurrency ?? '未声明' }}</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <WalletCards class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">收款方式</dt>
          <dd v-if="service.acceptedPaymentMethods?.length" class="flex flex-wrap items-center justify-end gap-x-3 gap-y-1 font-medium">
            <span v-for="method in service.acceptedPaymentMethods" :key="method" class="inline-flex items-center gap-1.5">
              <ApiPaymentMethodIcon :method="method" size="sm" />
              {{ apiPaymentMethodLabels[method] }}
            </span>
          </dd>
          <dd v-else class="text-right font-medium">待配置</dd>
        </div>
        <div class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <ShieldCheck class="mt-0.5 h-4 w-4 text-muted-foreground" />
          <dt class="text-muted-foreground">未解决纠纷</dt>
          <dd class="text-right font-medium">{{ service.unresolvedDisputes ?? '暂无数据' }}</dd>
        </div>
        <div v-if="service.warning" class="grid grid-cols-[20px_minmax(96px,0.7fr)_minmax(0,1fr)] items-start gap-3 text-sm">
          <CircleGauge class="mt-0.5 h-4 w-4 text-warning" />
          <dt class="text-muted-foreground">经营提示</dt>
          <dd class="text-right font-medium text-warning">{{ service.warning }}</dd>
        </div>
      </dl>
    </div>
  </Card>
</template>
