<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Megaphone, PackageOpen, ShieldCheck, ShoppingCart } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ApiMerchantAvatar from '@/components/api-market/ApiMerchantAvatar.vue'
import ApiMerchantBadges from '@/components/api-market/ApiMerchantBadges.vue'
import ApiQuotaPolicyStrip from '@/components/api-market/ApiQuotaPolicyStrip.vue'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import { getApiMerchantDisplayName } from '@/lib/api'
import type { ApiPackageRecommendation } from '@/lib/apiPackageRecommendation'

const props = defineProps<{
  row: ApiPackageRecommendation
  rank: number
  productIconSrc: string | null
  promoted?: boolean
  recommendationRanked?: boolean
}>()
const emit = defineEmits<{ activate: [] }>()

const formatNumber = (value: number | null, digits = 2) => value === null ? '—' : value.toFixed(digits).replace(/\.?0+$/, '')
const score = (value: number | null) => value === null ? 0 : Math.round(value)
const visibleModels = computed(() => props.row.matchedModels.slice(0, 3))
const hiddenModelCount = computed(() => Math.max(0, props.row.matchedModels.length - visibleModels.value.length))
const promptAuditEnabled = computed(() => props.row.service.promptAuditEnabled ?? null)
const modelSummary = computed(() => [
  ...visibleModels.value.map(model => `${model.modelName} ${formatNumber(model.merchantMultiplier, 4)}x`),
  ...(hiddenModelCount.value ? [`+${hiddenModelCount.value} 个`] : []),
].join(' / ') || '模型待选择')
</script>

<template>
  <Card
    class="api-package-card api-product-card h-full gap-0 p-0"
    :class="{ 'api-product-card--promoted': promoted }"
  >
    <div class="api-product-card__frame">
      <div class="api-product-card__header">
        <div class="flex min-w-0 items-start gap-2.5">
          <span class="api-product-card__icon">
            <img v-if="productIconSrc" :src="productIconSrc" :alt="`${row.service.title} 图标`" />
            <PackageOpen v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>
              <Badge variant="outline" class="api-product-card__category">短期流量包</Badge>
              <Badge variant="verified">{{ row.package.durationDays }} 天</Badge>
              <Badge v-if="row.recommendationEligible && recommendationRanked && rank === 1 && !promoted" variant="trust">综合推荐</Badge>
            </div>
            <h2 class="mt-1.5 truncate text-sm font-semibold text-slate-950" :title="row.package.name">{{ row.package.name }}</h2>
            <p class="truncate text-xs text-muted-foreground" :title="`${row.service.title} · ${modelSummary}`">{{ row.service.title }} · {{ modelSummary }}</p>
          </div>
        </div>

        <div class="mt-2 flex items-end justify-between gap-2">
          <div class="min-w-0">
            <div class="api-product-card__price text-xl font-semibold">¥{{ formatNumber(row.package.priceCny) }}</div>
            <div class="text-[11px] text-muted-foreground">面板额度 ${{ formatNumber(row.package.panelAllowance, 6) }} · 交付后生效</div>
          </div>
          <div v-if="row.recommendationEligible" class="shrink-0 pb-0.5 text-right text-[11px] font-medium text-primary">综合 {{ score(row.score) }} 分</div>
        </div>
      </div>

      <ApiQuotaPolicyStrip
        :policy="row.package.quotaUsagePolicy"
        :expiry-value="`交付后 ${row.package.durationDays} 天`"
      />
      <div v-if="$slots.health"><slot name="health" /></div>

      <div class="flex flex-wrap gap-1.5 border-t border-border px-4 py-2.5">
        <Badge v-for="model in row.matchedModels" :key="model.serviceModelId" variant="model">
          {{ model.modelName }} · {{ formatNumber(model.merchantMultiplier, 4) }}x
        </Badge>
      </div>

      <dl class="api-product-card__technical-facts">
        <div><dt>匹配模型</dt><dd>{{ row.matchedModels.length }} 个</dd></div>
        <div><dt>最大并发</dt><dd>{{ row.service.declaredMaxConcurrency ?? '—' }}</dd></div>
        <div><dt>提示词审计</dt><dd :class="promptAuditEnabled === true ? 'text-orange-700' : ''">{{ promptAuditEnabled === null ? '未声明' : promptAuditEnabled ? '开启' : '关闭' }}</dd></div>
        <div><dt>交付方式</dt><dd :title="row.service.delivery">{{ row.service.delivery }}</dd></div>
      </dl>

      <div class="api-product-card__details flex-1">
        <dl class="api-product-card__detail-facts">
          <div><dt>面板额度</dt><dd>${{ formatNumber(row.package.panelAllowance, 6) }}</dd></div>
          <div><dt>剩余库存</dt><dd>{{ row.package.stockAvailable }} / {{ row.package.stockTotal }}</dd></div>
          <div v-if="row.recommendationEligible"><dt>价值成本</dt><dd>¥{{ formatNumber(row.declaredUnitCost, 4) }}</dd></div>
          <div><dt>号池</dt><dd :title="row.service.accountPoolLabel">{{ row.service.accountPoolLabel || '历史服务未补充' }}</dd></div>
        </dl>
      </div>

      <div class="api-product-card__merchant">
        <div class="flex min-w-0 items-center gap-2">
          <ApiMerchantAvatar :service="row.service" class="api-product-card__merchant-avatar" />
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 flex-wrap items-center gap-1.5">
              <span class="truncate text-xs font-semibold">{{ getApiMerchantDisplayName(row.service) }}</span>
              <ApiMerchantBadges :service="row.service" />
            </div>
            <div class="mt-0.5 text-[10px] text-muted-foreground">近期完成 {{ row.service.completed30d === null ? '暂无数据' : `${row.service.completed30d} 单` }}</div>
          </div>
          <Badge v-if="row.service.merchantRefundCommitment" variant="outline" class="shrink-0 text-[10px]" title="商户全额退款承诺">
            <ShieldCheck class="h-3 w-3" />全额退款
          </Badge>
        </div>
        <ReputationInlineSummary
          class="mt-1 min-w-0"
          :summary="row.service.sellerReputation"
          :compact="row.service.sellerReputation?.state === 'active'"
        />
      </div>

      <div class="api-product-card__action">
        <RouterLink
          :to="{ path: `/api-market/${row.service.id}`, query: { package: row.package.id } }"
          class="block"
          @click.capture="emit('activate')"
        >
          <Button class="h-11 w-full sm:h-9"><ShoppingCart class="h-4 w-4" />查看并购买套餐</Button>
        </RouterLink>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.api-package-card {
  --api-product-accent: #d97757;
}
</style>
