<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Megaphone, PackageOpen, ShieldCheck } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import ApiMerchantAvatar from '@/components/api-market/ApiMerchantAvatar.vue'
import ApiMerchantBadges from '@/components/api-market/ApiMerchantBadges.vue'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import { getApiMerchantDisplayName } from '@/lib/api'
import type { ApiPackageRecommendation } from '@/lib/apiPackageRecommendation'

const props = defineProps<{
  row: ApiPackageRecommendation
  rank: number
  productIconSrc: string | null
  promoted?: boolean
}>()
const emit = defineEmits<{ activate: [] }>()

const formatNumber = (value: number, digits = 2) => value.toFixed(digits).replace(/\.?0+$/, '')
const score = (value: number) => Math.round(value)
const visibleModels = props.row.package.models.slice(0, 3)
const hiddenModelCount = Math.max(0, props.row.package.models.length - visibleModels.length)
</script>

<template>
  <RouterLink
    :to="{ path: `/api-market/${row.service.id}`, query: { package: row.package.id } }"
    class="block min-w-0"
    @click.capture="emit('activate')"
  >
    <Card class="api-package-card h-full min-w-0 overflow-hidden p-0" :class="{ 'api-package-card--promoted': promoted }">
      <div class="flex min-w-0 items-start gap-3 p-4 pb-3">
        <span class="api-service-card-logo api-package-card-logo">
          <img v-if="productIconSrc" :src="productIconSrc" :alt="`${row.service.title} 图标`" />
          <PackageOpen v-else class="h-5 w-5" />
        </span>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="truncate font-semibold text-slate-950">{{ row.package.name }}</h2>
            <Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>
            <Badge variant="verified">{{ row.package.durationDays }} 天</Badge>
            <Badge v-if="rank === 1 && !promoted" variant="trust">综合推荐</Badge>
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
          <img v-if="productIconSrc" :src="productIconSrc" alt="" class="api-model-badge-icon" />
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

      <dl class="grid grid-cols-3 border-b border-border text-xs">
        <div class="min-w-0 px-3 py-2"><dt class="text-muted-foreground">号池</dt><dd class="mt-1 truncate font-medium" :title="row.service.accountPoolLabel">{{ row.service.accountPoolLabel || '历史服务未补充' }}</dd></div>
        <div class="min-w-0 border-x border-border px-3 py-2"><dt class="text-muted-foreground">最大并发</dt><dd class="mt-1 font-medium">{{ row.service.declaredMaxConcurrency ?? '—' }}</dd></div>
        <div class="min-w-0 px-3 py-2"><dt class="text-muted-foreground">退款承诺</dt><dd class="mt-1 truncate font-medium" :title="row.service.merchantRefundCommitment ? '商户全额退款承诺' : '无额外退款承诺'"><ShieldCheck class="mr-1 inline h-3 w-3" />{{ row.service.merchantRefundCommitment ? '全额退款' : '无额外承诺' }}</dd></div>
      </dl>

      <div class="border-b border-border px-4 py-2">
        <div class="grid grid-cols-4 gap-2 text-center text-[11px] text-muted-foreground" title="综合推荐构成">
          <span>性价比 <b class="block text-xs text-foreground">{{ score(row.valueScore) }}</b></span>
          <span>履约 <b class="block text-xs text-foreground">{{ score(row.fulfillmentScore) }}</b></span>
          <span>响应 <b class="block text-xs text-foreground">{{ score(row.responseScore) }}</b></span>
          <span>新鲜度 <b class="block text-xs text-foreground">{{ score(row.freshnessScore) }}</b></span>
        </div>
      </div>

      <div class="border-b border-border px-4 py-3">
        <ReputationInlineSummary
          :summary="row.service.sellerReputation"
          :compact="row.service.sellerReputation?.state === 'active'"
        />
      </div>

      <div class="api-service-card-footer">
        <div class="api-market-merchant">
          <ApiMerchantAvatar :service="row.service" class="api-market-avatar" />
          <span class="min-w-0">
            <span class="flex flex-wrap items-center gap-1.5">
              <span class="truncate text-sm font-medium">{{ getApiMerchantDisplayName(row.service) }}</span>
              <ApiMerchantBadges :service="row.service" />
            </span>
            <span class="mt-0.5 block text-xs text-muted-foreground">
              近期完成 {{ row.service.completed30d === null ? '暂无数据' : `${row.service.completed30d} 单` }}
            </span>
          </span>
        </div>
        <span class="shrink-0 text-xs font-medium text-primary">查看套餐 →</span>
      </div>
    </Card>
  </RouterLink>
</template>

<style scoped>
.api-package-card--promoted {
  border-color: rgb(249 115 22 / 0.58);
  box-shadow: inset 0 3px 0 rgb(249 115 22 / 0.9);
}

.api-package-card--promoted:hover {
  border-color: rgb(234 88 12 / 0.72);
  box-shadow: inset 0 3px 0 rgb(234 88 12), 0 8px 24px rgb(15 23 42 / 0.06);
}

</style>
