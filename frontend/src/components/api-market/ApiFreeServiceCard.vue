<script setup lang="ts">
import { computed } from 'vue'
import { Award, Code2, Megaphone, ShieldCheck, ShieldQuestion, ShoppingCart, Zap } from 'lucide-vue-next'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { formatDecimal } from '@/lib/decimal'
import { compactApiServiceModels, type ApiFreeServiceCardData } from './apiFreeServiceCard'
import ApiQuotaPolicyStrip from '@/components/api-market/ApiQuotaPolicyStrip.vue'

const props = withDefaults(defineProps<{
  card: ApiFreeServiceCardData
  preview?: boolean
  promoted?: boolean
}>(), {
  preview: false,
  promoted: false,
})
const emit = defineEmits<{ activate: [] }>()

const compactModels = computed(() => compactApiServiceModels(props.card.models))
const modelCountLabel = computed(() => props.card.models.length ? `支持 ${props.card.models.length} 个模型` : '模型待选择')
const modelTitle = computed(() => props.card.models.join(' / ') || '模型待选择')
const hasReputationRisk = computed(() => Boolean(props.card.sellerReputation && props.card.sellerReputation.state !== 'active'))
const merchantInitial = computed(() => props.card.merchantName.trim().slice(0, 1).toUpperCase() || '店')
</script>

<template>
  <Card
    class="api-free-service-card api-product-card gap-0 py-0"
    :class="{
      'api-free-service-card--risk': hasReputationRisk,
      'api-free-service-card--preview': preview,
      'api-product-card--promoted': promoted,
    }"
    :data-category="card.category"
  >
    <div class="api-product-card__frame">
      <div class="api-product-card__header">
        <div class="flex items-start gap-2.5">
          <span class="api-free-service-card__icon api-product-card__icon">
            <img v-if="card.iconSrc" :src="card.iconSrc" alt="" class="h-6 w-6 object-contain" />
            <Code2 v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge v-if="promoted" variant="status"><Megaphone class="h-3 w-3" />推广</Badge>
              <Badge variant="outline" class="api-product-card__category">{{ card.categoryLabel }}</Badge>
              <Badge variant="verified">可购买</Badge>
            </div>
            <h2 class="mt-1.5 truncate text-sm font-semibold text-slate-950" :title="card.title">{{ card.title }}</h2>
            <p class="truncate text-xs text-muted-foreground" :title="`${card.delivery} · ${modelTitle}`">
              {{ card.delivery }} · {{ modelCountLabel }}
            </p>
            <div class="mt-1 flex min-w-0 items-center gap-1 overflow-hidden" :title="modelTitle">
              <Badge
                v-for="model in compactModels.visibleModels"
                :key="model"
                variant="secondary"
                class="max-w-[8.5rem] shrink truncate px-1.5 py-0 text-[10px] font-medium"
              >
                {{ model }}
              </Badge>
              <Badge
                v-if="compactModels.hiddenModelCount"
                variant="outline"
                class="shrink-0 px-1.5 py-0 text-[10px] font-semibold"
              >
                +{{ compactModels.hiddenModelCount }}
              </Badge>
              <span v-if="!card.models.length" class="text-[10px] text-muted-foreground">待选择具体模型</span>
            </div>
          </div>
        </div>

        <div class="mt-3 flex items-end justify-between gap-2">
          <div class="min-w-0">
            <div class="api-free-service-card__price api-product-card__price whitespace-nowrap text-2xl font-semibold">
              ¥{{ formatDecimal(card.cnyPerUsdAllowance, 2, 6) }}
              <span class="text-xs font-medium">/ $1</span>
            </div>
            <div class="text-[11px] text-muted-foreground">
              按金额购买 · 最低 ¥{{ card.minimumPurchaseCny }} 起
            </div>
          </div>
          <div class="shrink-0 pb-0.5 text-right text-xs text-muted-foreground">
            可售 ${{ formatDecimal(card.availableUsdAllowance || 0, 0, 6) }}
          </div>
        </div>
      </div>

      <ApiQuotaPolicyStrip :policy="card.quotaUsagePolicy" :expiry-value="card.expiresAt" />
      <div v-if="$slots.health"><slot name="health" /></div>

      <dl class="api-product-card__technical-facts">
        <div><dt>统一倍率</dt><dd>{{ card.multiplier }}</dd></div>
        <div><dt title="商户声明最大并发">最大并发</dt><dd>{{ card.declaredMaxConcurrency || '—' }}</dd></div>
        <div><dt>支持模型</dt><dd>{{ card.models.length || '—' }} 个</dd></div>
        <div><dt>交付方式</dt><dd :title="card.delivery">{{ card.delivery }}</dd></div>
      </dl>

      <div class="api-product-card__details flex-1">
        <dl class="api-product-card__detail-facts">
          <div><dt>可售额度</dt><dd :title="`$${card.availableUsdAllowance}`">${{ formatDecimal(card.availableUsdAllowance || 0, 0, 6) }}</dd></div>
          <div><dt>单笔范围</dt><dd :title="`¥${card.minimumPurchaseCny} - ¥${card.maximumPurchaseCny}`">¥{{ card.minimumPurchaseCny }} - ¥{{ card.maximumPurchaseCny }}</dd></div>
          <div><dt>预计响应</dt><dd>{{ card.paymentWindowMinutes }} 分钟内</dd></div>
          <div><dt>号池</dt><dd :title="card.accountPoolLabel">{{ card.accountPoolLabel || '历史服务未补充' }}</dd></div>
        </dl>
      </div>

      <div class="api-product-card__merchant">
        <div class="flex min-w-0 items-center gap-2">
          <span class="api-product-card__merchant-avatar" aria-hidden="true">{{ merchantInitial }}</span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-xs font-semibold" :title="card.merchantName">{{ card.merchantName }}</div>
            <div class="mt-0.5 text-[10px] text-muted-foreground">{{ card.merchantType }} · API 商家</div>
          </div>
        </div>
        <div class="mt-2 flex min-h-5 min-w-0 flex-wrap items-center gap-1.5">
          <div v-if="preview" class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <ShieldQuestion class="h-3.5 w-3.5" />
            发布后显示账户公开信誉
          </div>
          <template v-else>
            <Badge
              v-for="badge in card.merchantBadges"
              :key="badge.kind"
              variant="outline"
              :title="badge.description"
              class="api-merchant-achievement-badge"
              :class="`api-merchant-achievement-badge--${badge.kind}`"
            >
              <Award v-if="badge.kind === 'quality'" />
              <Zap v-else />
              {{ badge.label }}
            </Badge>
            <Badge variant="outline" :class="card.merchantRefundCommitment ? 'border-orange-300 bg-orange-50 text-orange-800' : 'text-muted-foreground'">
              <ShieldCheck class="h-3 w-3" />{{ card.merchantRefundCommitment ? '商户全额退款承诺' : '无额外退款承诺' }}
            </Badge>
          </template>
        </div>
        <div class="mt-1.5 min-w-0">
          <ReputationInlineSummary
            v-if="!preview"
            class="min-w-0"
            :summary="card.sellerReputation"
            :compact="card.sellerReputation?.state === 'active'"
          />
        </div>
      </div>

      <div class="api-product-card__action">
        <div v-if="preview">
          <Button type="button" class="pointer-events-none h-10 w-full" aria-disabled="true" tabindex="-1">
            <ShoppingCart class="h-4 w-4" />选择金额并下单
          </Button>
          <p class="mt-1 text-center text-[10px] text-muted-foreground">预览状态，不可操作</p>
        </div>
        <RouterLink v-else-if="card.actionHref" :to="card.actionHref" class="block" @click.capture="emit('activate')">
          <Button class="h-10 w-full"><ShoppingCart class="h-4 w-4" />选择金额并下单</Button>
        </RouterLink>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.api-free-service-card {
  --api-product-accent: #64748b;
}

.api-free-service-card--risk {
  border-left-color: var(--destructive);
}

.api-free-service-card[data-category='gpt'] {
  --api-product-accent: #10a37f;
}

.api-free-service-card[data-category='claude'] {
  --api-product-accent: #d97757;
}

.api-free-service-card[data-category='gemini'] {
  --api-product-accent: #2563eb;
}

.api-free-service-card[data-category='cursor'] {
  --api-product-accent: #0891b2;
}

.api-free-service-card[data-category='perplexity'] {
  --api-product-accent: #059669;
}

.api-free-service-card[data-category='other'] {
  --api-product-accent: #64748b;
}
</style>
