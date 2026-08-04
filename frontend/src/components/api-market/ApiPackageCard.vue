<script setup lang="ts">
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
}>()
const emit = defineEmits<{ activate: [] }>()

const formatNumber = (value: number, digits = 2) => value.toFixed(digits).replace(/\.?0+$/, '')
const score = (value: number) => Math.round(value)
const visibleModels = props.row.package.models.slice(0, 3)
const hiddenModelCount = Math.max(0, props.row.package.models.length - visibleModels.length)
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
            <img v-if="productIconSrc" :src="productIconSrc" :alt="`${row.service.title} 图标`" class="h-6 w-6 object-contain" />
            <PackageOpen v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>
              <Badge variant="outline" class="api-product-card__category">限时流量包</Badge>
              <Badge variant="verified">{{ row.package.durationDays }} 天</Badge>
              <Badge v-if="rank === 1 && !promoted" variant="trust">综合推荐</Badge>
            </div>
            <h2 class="mt-1.5 truncate text-sm font-semibold text-slate-950" :title="row.package.name">{{ row.package.name }}</h2>
            <p class="truncate text-xs text-muted-foreground" :title="row.service.title">{{ row.service.title }}</p>
            <div class="mt-1 flex min-w-0 items-center gap-1 overflow-hidden">
              <Badge v-for="model in visibleModels" :key="model.serviceModelId" variant="secondary" class="max-w-32 truncate px-1.5 py-0 text-[10px]">
                {{ model.modelName }}
              </Badge>
              <Badge v-if="hiddenModelCount" variant="outline" class="px-1.5 py-0 text-[10px]">+{{ hiddenModelCount }}</Badge>
            </div>
          </div>
        </div>

        <div class="mt-3 flex items-end justify-between gap-2">
          <div class="min-w-0">
            <div class="api-product-card__price text-2xl font-semibold">¥{{ formatNumber(row.package.priceCny) }}</div>
            <div class="text-[11px] text-muted-foreground">面板额度 ${{ formatNumber(row.package.panelAllowance, 6) }} · 交付后生效</div>
          </div>
          <div class="shrink-0 pb-0.5 text-right text-[11px] font-medium text-primary">综合 {{ score(row.score) }} 分</div>
        </div>
      </div>

      <ApiQuotaPolicyStrip
        :policy="row.package.quotaUsagePolicy"
        :expiry-value="`交付后 ${row.package.durationDays} 天`"
      />
      <div v-if="$slots.health"><slot name="health" /></div>

      <dl class="api-product-card__technical-facts">
        <div><dt>模型倍率</dt><dd>{{ formatNumber(row.selectedModel.merchantMultiplier, 4) }}x</dd></div>
        <div><dt>最大并发</dt><dd>{{ row.service.declaredMaxConcurrency ?? '—' }}</dd></div>
        <div><dt>有效期</dt><dd>{{ row.package.durationDays }} 天</dd></div>
        <div><dt>交付方式</dt><dd :title="row.service.delivery">{{ row.service.delivery }}</dd></div>
      </dl>

      <div class="api-product-card__details flex-1">
        <dl class="api-product-card__detail-facts">
          <div><dt>面板额度</dt><dd>${{ formatNumber(row.package.panelAllowance, 6) }}</dd></div>
          <div><dt>剩余库存</dt><dd>{{ row.package.stockAvailable }} / {{ row.package.stockTotal }}</dd></div>
          <div><dt>价值成本</dt><dd>¥{{ formatNumber(row.declaredUnitCost, 4) }}</dd></div>
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
          <Badge variant="outline" class="shrink-0 text-[10px]" :title="row.service.merchantRefundCommitment ? '商户全额退款承诺' : '无额外退款承诺'">
            <ShieldCheck class="h-3 w-3" />{{ row.service.merchantRefundCommitment ? '全额退款' : '无额外承诺' }}
          </Badge>
        </div>
        <ReputationInlineSummary
          class="mt-1.5 min-w-0"
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
          <Button class="h-10 w-full"><ShoppingCart class="h-4 w-4" />查看并购买套餐</Button>
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
