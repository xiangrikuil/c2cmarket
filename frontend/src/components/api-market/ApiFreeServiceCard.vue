<script setup lang="ts">
import { computed } from 'vue'
import { Code2, ShieldQuestion, ShoppingCart } from 'lucide-vue-next'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { formatDecimal } from '@/lib/decimal'
import { compactApiServiceModels, type ApiFreeServiceCardData } from './apiFreeServiceCard'

const props = withDefaults(defineProps<{
  card: ApiFreeServiceCardData
  preview?: boolean
}>(), {
  preview: false,
})

const compactModels = computed(() => compactApiServiceModels(props.card.models))
const modelCountLabel = computed(() => props.card.models.length ? `支持 ${props.card.models.length} 个模型` : '模型待选择')
const modelTitle = computed(() => props.card.models.join(' / ') || '模型待选择')
const hasReputationRisk = computed(() => Boolean(props.card.sellerReputation && props.card.sellerReputation.state !== 'active'))
</script>

<template>
  <Card
    class="api-free-service-card gap-0 overflow-hidden py-0"
    :class="{
      'api-free-service-card--risk': hasReputationRisk,
      'api-free-service-card--preview': preview,
    }"
    :data-category="card.category"
  >
    <img
      v-if="card.iconSrc"
      :src="card.iconSrc"
      alt=""
      aria-hidden="true"
      class="api-free-service-card__watermark"
    />

    <div class="relative z-[1] flex h-full flex-col">
      <div class="p-2.5 pb-1.5">
        <div class="flex items-start gap-2.5">
          <span class="api-free-service-card__icon">
            <img v-if="card.iconSrc" :src="card.iconSrc" alt="" class="h-6 w-6 object-contain" />
            <Code2 v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <Badge variant="outline" class="api-free-service-card__category">{{ card.categoryLabel }}</Badge>
            <h2 class="mt-1 truncate text-sm font-semibold" :title="card.title">{{ card.title }}</h2>
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

        <div class="mt-1.5 flex items-end justify-between gap-2">
          <div class="min-w-0">
            <div class="api-free-service-card__price whitespace-nowrap text-2xl font-semibold">
              ¥{{ formatDecimal(card.cnyPerUsdAllowance, 2, 6) }}
              <span class="text-sm font-medium">/ $1</span>
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

      <dl class="api-free-service-card__metrics grid grid-cols-4 gap-px text-xs">
        <div><dt>统一倍率</dt><dd>{{ card.multiplier }}</dd></div>
        <div><dt>首字响应</dt><dd>{{ card.ttftLabel }}</dd></div>
        <div><dt>建议并发</dt><dd>{{ card.recommendedConcurrency || '—' }}</dd></div>
        <div><dt>付款窗口</dt><dd>{{ card.paymentWindowMinutes }} 分钟</dd></div>
      </dl>

      <div class="flex-1 px-3 py-2.5">
        <dl class="grid grid-cols-2 gap-x-3 gap-y-2 text-xs">
          <div class="flex min-w-0 items-center gap-1.5">
            <dt class="shrink-0 text-muted-foreground">卖家</dt>
            <dd class="truncate font-medium" :title="`${card.merchantName} · ${card.merchantType}`">{{ card.merchantName }} · {{ card.merchantType }}</dd>
          </div>
          <div class="flex min-w-0 items-center gap-1.5">
            <dt class="shrink-0 text-muted-foreground">单笔</dt>
            <dd class="truncate font-medium" :title="`¥${card.minimumPurchaseCny} - ¥${card.maximumPurchaseCny}`">¥{{ card.minimumPurchaseCny }} - ¥{{ card.maximumPurchaseCny }}</dd>
          </div>
          <div class="flex min-w-0 items-center gap-1.5">
            <dt class="shrink-0 text-muted-foreground">接入</dt>
            <dd class="truncate font-medium" :title="card.delivery">{{ card.delivery }}</dd>
          </div>
          <div class="flex min-w-0 items-center gap-1.5">
            <dt class="shrink-0 text-muted-foreground">有效期</dt>
            <dd class="truncate font-medium" :title="card.expiresAt">{{ card.expiresAt }}</dd>
          </div>
        </dl>

        <div class="mt-2 min-w-0 border-t border-border pt-2">
          <div v-if="preview" class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <ShieldQuestion class="h-3.5 w-3.5" />
            发布后显示账户公开信誉
          </div>
          <ReputationInlineSummary
            v-else
            class="min-w-0"
            :summary="card.sellerReputation"
            :compact="card.sellerReputation?.state === 'active'"
          />
        </div>
      </div>

      <div class="border-t border-border px-3 py-2">
        <div v-if="preview">
          <Button type="button" class="pointer-events-none h-10 w-full" aria-disabled="true" tabindex="-1">
            <ShoppingCart class="h-4 w-4" />选择金额并下单
          </Button>
          <p class="mt-1 text-center text-[10px] text-muted-foreground">预览状态，不可操作</p>
        </div>
        <RouterLink v-else-if="card.actionHref" :to="card.actionHref" class="block">
          <Button class="h-10 w-full"><ShoppingCart class="h-4 w-4" />选择金额并下单</Button>
        </RouterLink>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.api-free-service-card {
  --api-free-card-accent: #64748b;

  position: relative;
  isolation: isolate;
  width: 100%;
  height: 360px;
  border-radius: 0.5rem;
  border-color: color-mix(in oklab, var(--api-free-card-accent) 28%, var(--border));
  background-color: color-mix(in oklab, var(--api-free-card-accent) 4%, var(--card));
  box-shadow: inset 0 3px 0 color-mix(in oklab, var(--api-free-card-accent) 72%, transparent);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.api-free-service-card:hover {
  border-color: color-mix(in oklab, var(--api-free-card-accent) 48%, var(--border));
  box-shadow:
    inset 0 3px 0 color-mix(in oklab, var(--api-free-card-accent) 82%, transparent),
    0 8px 24px rgb(15 23 42 / 0.06);
}

.api-free-service-card--risk,
.api-free-service-card--preview {
  height: auto;
  min-height: 360px;
}

.api-free-service-card--preview {
  min-height: 380px;
}

.api-free-service-card[data-category='gpt'] {
  --api-free-card-accent: #7c3aed;
}

.api-free-service-card[data-category='claude'] {
  --api-free-card-accent: #dc5f45;
}

.api-free-service-card[data-category='gemini'] {
  --api-free-card-accent: #2563eb;
}

.api-free-service-card[data-category='cursor'] {
  --api-free-card-accent: #0891b2;
}

.api-free-service-card[data-category='perplexity'] {
  --api-free-card-accent: #059669;
}

.api-free-service-card[data-category='other'] {
  --api-free-card-accent: #64748b;
}

.api-free-service-card__icon {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border: 1px solid color-mix(in oklab, var(--api-free-card-accent) 28%, var(--border));
  border-radius: 0.5rem;
  color: var(--api-free-card-accent);
  background-color: color-mix(in oklab, var(--api-free-card-accent) 10%, var(--card));
}

.api-free-service-card__category {
  border-color: color-mix(in oklab, var(--api-free-card-accent) 36%, var(--border));
  color: var(--api-free-card-accent);
  background-color: color-mix(in oklab, var(--api-free-card-accent) 8%, var(--card));
}

.api-free-service-card__price {
  color: var(--api-free-card-accent);
}

.api-free-service-card__watermark {
  position: absolute;
  top: 4.5rem;
  right: -1rem;
  z-index: 0;
  width: 7.5rem;
  height: 7.5rem;
  object-fit: contain;
  opacity: 0.055;
  pointer-events: none;
}

.api-free-service-card__metrics {
  border-block: 1px solid color-mix(in oklab, var(--api-free-card-accent) 16%, var(--border));
  background-color: color-mix(in oklab, var(--api-free-card-accent) 14%, var(--border));
}

.api-free-service-card__metrics > div {
  min-width: 0;
  padding: 0.375rem 0.5rem;
  background-color: color-mix(in oklab, var(--api-free-card-accent) 3%, var(--card));
}

.api-free-service-card__metrics dt,
.api-free-service-card__metrics dd {
  white-space: nowrap;
}

.api-free-service-card__metrics dt {
  color: var(--muted-foreground);
}

.api-free-service-card__metrics dd {
  margin-top: 0.125rem;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 600;
}

@media (max-width: 639px) {
  .api-free-service-card {
    max-width: 375px;
  }
}
</style>
