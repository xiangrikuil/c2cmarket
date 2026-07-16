<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Gauge, PackageOpen } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { getApiMerchantAvatarText, getApiMerchantDisplayName } from '@/lib/api'
import type { ApiPackageRecommendation } from '@/lib/apiPackageRecommendation'

const props = defineProps<{
  row: ApiPackageRecommendation
  rank: number
}>()

const formatNumber = (value: number, digits = 2) => value.toFixed(digits).replace(/\.?0+$/, '')
const score = (value: number) => Math.round(value)
const visibleModels = props.row.package.models.slice(0, 3)
const hiddenModelCount = Math.max(0, props.row.package.models.length - visibleModels.length)
</script>

<template>
  <RouterLink :to="{ path: `/api-market/${row.service.id}`, query: { package: row.package.id } }" class="block min-w-0">
    <Card class="api-package-card h-full min-w-0 overflow-hidden p-0">
      <div class="flex min-w-0 items-start gap-3 p-4 pb-3">
        <span class="api-service-card-logo api-package-card-logo"><PackageOpen class="h-5 w-5" /></span>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="truncate font-semibold text-slate-950">{{ row.package.name }}</h2>
            <Badge variant="verified">{{ row.package.durationDays }} 天</Badge>
            <Badge v-if="rank === 1" variant="trust">综合推荐</Badge>
          </div>
          <p class="mt-1 truncate text-xs text-muted-foreground">{{ row.service.title }}</p>
        </div>
        <div class="shrink-0 text-right">
          <div class="api-service-card-price">¥{{ formatNumber(row.package.priceCny) }}</div>
          <div class="mt-1 text-xs font-medium text-primary">综合 {{ score(row.score) }} 分</div>
        </div>
      </div>

      <div class="flex min-h-8 flex-wrap gap-1.5 px-4 pb-3">
        <Badge v-for="model in visibleModels" :key="model.serviceModelId" variant="model">
          {{ model.modelName }} · {{ formatNumber(model.merchantMultiplier, 4) }}x
        </Badge>
        <Badge v-if="hiddenModelCount" variant="model">+{{ hiddenModelCount }}</Badge>
      </div>

      <dl class="api-service-card-metrics api-package-card-metrics">
        <div><dt>面板额度</dt><dd>{{ formatNumber(row.package.panelAllowance, 6) }}</dd></div>
        <div><dt>剩余库存</dt><dd>{{ row.package.stockAvailable }} / {{ row.package.stockTotal }}</dd></div>
        <div><dt>选中模型倍率</dt><dd>{{ formatNumber(row.selectedModel.merchantMultiplier, 4) }}x</dd></div>
        <div><dt>价值成本</dt><dd>¥{{ formatNumber(row.declaredUnitCost, 4) }}</dd></div>
      </dl>

      <div class="border-b border-border px-4 py-3">
        <div class="mb-2 flex items-center gap-1.5 text-xs font-medium text-slate-700"><Gauge class="h-3.5 w-3.5 text-primary" />综合推荐构成</div>
        <div class="grid grid-cols-4 gap-2 text-center text-[11px] text-muted-foreground">
          <span>性价比 <b class="block text-xs text-foreground">{{ score(row.valueScore) }}</b></span>
          <span>履约 <b class="block text-xs text-foreground">{{ score(row.fulfillmentScore) }}</b></span>
          <span>响应 <b class="block text-xs text-foreground">{{ score(row.responseScore) }}</b></span>
          <span>新鲜度 <b class="block text-xs text-foreground">{{ score(row.freshnessScore) }}</b></span>
        </div>
        <p class="mt-2 text-[11px] text-muted-foreground">价值成本按商家声明估算，越低越划算。</p>
      </div>

      <div class="api-service-card-footer">
        <div class="api-market-merchant">
          <span class="api-market-avatar">{{ getApiMerchantAvatarText(row.service) }}</span>
          <span class="min-w-0">
            <span class="block truncate text-sm font-medium">{{ getApiMerchantDisplayName(row.service) }}</span>
            <span class="mt-0.5 block text-xs text-muted-foreground">信任等级 {{ row.service.trustLevel }} · 近 30 天完成 {{ row.service.completed30d }} 单</span>
          </span>
        </div>
        <span class="shrink-0 text-xs font-medium text-primary">查看套餐 →</span>
      </div>
    </Card>
  </RouterLink>
</template>
