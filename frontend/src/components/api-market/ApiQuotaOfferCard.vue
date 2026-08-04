<script setup lang="ts">
import { computed } from 'vue'
import { Clock3, Gauge, PackageOpen, ShoppingCart } from 'lucide-vue-next'
import ApiQuotaPolicyStrip from '@/components/api-market/ApiQuotaPolicyStrip.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  getApiQuotaDeliveryModeLabel,
  getApiQuotaDistributionLabel,
  getApiQuotaSaleModeLabel,
  type PublicApiQuotaOffer,
} from '@/lib/api'
import { apiQuotaDurationLabel, apiQuotaOfferCountdown } from '@/lib/apiQuotaOfferUi'
import { formatDecimal } from '@/lib/decimal'
import type { ConcreteProductCategoryKey } from '@/lib/productCategories'

const props = withDefaults(defineProps<{
  offer: PublicApiQuotaOffer
  category: ConcreteProductCategoryKey
  categoryLabel: string
  iconSrc?: string | null
  now: number
  variant?: 'rush' | 'standard'
  pendingOfferId?: string
}>(), {
  iconSrc: null,
  variant: 'standard',
  pendingOfferId: '',
})

const emit = defineEmits<{ purchase: [offer: PublicApiQuotaOffer] }>()

const isRush = computed(() => props.variant === 'rush')
const purchasePending = computed(() => props.pendingOfferId === props.offer.id)
const purchaseDisabled = computed(() => !props.offer.isOrderable || Boolean(props.pendingOfferId))
const statusLabel = computed(() => props.offer.isOrderable
  ? isRush.value ? '正在抢购' : '可购买'
  : props.offer.orderabilityReason)
const statusVariant = computed(() => {
  if (props.offer.isOrderable) return 'verified'
  if (props.offer.orderabilityCode === 'not_started') return 'trust'
  if (props.offer.orderabilityCode === 'sold_out') return 'secondary'
  return 'status'
})
const purchaseLabel = computed(() => {
  if (purchasePending.value) return '正在创建订单...'
  if (!props.offer.isOrderable) return props.offer.orderabilityReason
  return `立即抢购 ¥${formatDecimal(props.offer.priceCny, 2, 2)}`
})
const sellerType = computed(() => props.offer.sellerIdentityType === 'merchant' ? '商户' : '个人卖家')
const stockLabel = computed(() => isRush.value
  ? `${props.offer.availableCopies} 份`
  : props.offer.saleMode === 'scheduled'
    ? `本轮剩余 ${props.offer.availableCopies} 份`
    : `剩余 ${props.offer.availableCopies} 份`)

function formatAbsoluteTime(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '时间待确认'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(parsed)
}
</script>

<template>
  <Card
    class="quota-offer-card gap-0 overflow-hidden py-0"
    :data-category="category"
  >
    <img v-if="iconSrc" :src="iconSrc" alt="" aria-hidden="true" class="quota-offer-watermark" />
    <div class="relative z-[1] flex h-full flex-col">
      <div class="p-4 pb-3">
        <div class="flex items-start gap-3">
          <span class="quota-offer-icon-well">
            <img v-if="iconSrc" :src="iconSrc" alt="" class="h-6 w-6 object-contain" />
            <PackageOpen v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge variant="outline" class="quota-offer-category">{{ categoryLabel }}</Badge>
              <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
              <Badge v-if="!isRush" variant="secondary">{{ getApiQuotaSaleModeLabel(offer.saleMode) }}</Badge>
            </div>
            <h2 class="mt-2 break-words text-base font-semibold">{{ offer.name }}</h2>
            <p class="mt-1 break-words text-xs text-muted-foreground">{{ offer.serviceTitle }} · {{ getApiQuotaDistributionLabel(offer.distributionSystem) }}</p>
          </div>
        </div>
        <div class="mt-4 flex flex-wrap items-end justify-between gap-2">
          <div>
            <div class="quota-offer-price text-3xl font-semibold">¥{{ formatDecimal(offer.priceCny, 2, 2) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">一次购买 · ${{ formatDecimal(offer.usdAllowance, 0, 6) }} 美元额度</div>
          </div>
          <div class="text-right text-xs text-muted-foreground">¥{{ formatDecimal(offer.cnyPerUsd, 3, 6) }} / $1</div>
        </div>
      </div>

      <ApiQuotaPolicyStrip
        :policy="offer.quotaUsagePolicy"
        :expiry-value="formatAbsoluteTime(offer.expiresAt)"
      />
      <div v-if="$slots.health" class="api-service-health-slot px-4"><slot name="health" /></div>

      <dl class="quota-offer-metrics grid grid-cols-3 gap-px text-sm">
        <div><dt>模型倍率</dt><dd>{{ Number(offer.modelMultiplier).toFixed(2) }}x</dd></div>
        <div><dt>最大并发</dt><dd>{{ offer.declaredMaxConcurrency }}</dd></div>
        <div><dt>预计交付</dt><dd>≤ {{ offer.deliveryEtaMinutes }} 分钟</dd></div>
      </dl>

      <div class="flex-1 p-4">
        <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
          <div class="min-w-0">
            <dt class="text-xs text-muted-foreground">卖家</dt>
            <dd class="mt-0.5 break-words font-medium">{{ offer.sellerDisplayName }}</dd>
            <dd class="text-xs text-muted-foreground">{{ sellerType }} · {{ offer.sellerLinuxDoBound ? '已绑定 linux.do' : '未绑定 linux.do' }}</dd>
          </div>
          <div class="min-w-0">
            <dt class="text-xs text-muted-foreground">{{ isRush ? '本轮库存' : '剩余库存' }}</dt>
            <dd class="mt-0.5 font-medium">{{ stockLabel }}</dd>
          </div>
          <div class="min-w-0"><dt class="text-xs text-muted-foreground">交付方式</dt><dd class="mt-0.5 break-words font-medium">{{ getApiQuotaDeliveryModeLabel(offer.deliveryMode) }}</dd></div>
          <div v-if="isRush" class="min-w-0"><dt class="text-xs text-muted-foreground">额度有效期</dt><dd class="mt-0.5 break-words font-medium">{{ formatAbsoluteTime(offer.expiresAt) }}</dd><dd class="text-xs text-muted-foreground">约剩 {{ apiQuotaDurationLabel(offer.expiresAt, now) }}</dd></div>
          <div v-else class="min-w-0"><dt class="text-xs text-muted-foreground">销售时间</dt><dd class="mt-0.5 break-words font-medium">{{ formatAbsoluteTime(offer.saleCutoffAt) }}</dd><dd class="min-h-5 font-mono text-xs tabular-nums text-muted-foreground">{{ apiQuotaOfferCountdown(offer, now) }}</dd></div>
        </dl>
        <div v-if="!isRush" class="mt-3 flex gap-2 border-t border-border pt-3 text-xs leading-5 text-muted-foreground">
          <Clock3 class="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>额度有效至 <strong class="font-medium text-foreground">{{ formatAbsoluteTime(offer.expiresAt) }}</strong> · 约剩 {{ apiQuotaDurationLabel(offer.expiresAt, now) }}</span>
        </div>
      </div>

      <div class="border-t border-border p-4">
        <Button :disabled="purchaseDisabled" class="h-10 w-full" @click="emit('purchase', offer)">
          <ShoppingCart class="h-4 w-4" />{{ purchaseLabel }}
        </Button>
        <p class="mt-2 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
          <Gauge class="mt-0.5 h-3.5 w-3.5 shrink-0" />
          额度规则由卖家声明；金额为倍率计费后的美元额度，每份买家凭据独立适用。
        </p>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.quota-offer-card {
  --quota-accent: #64748b;

  position: relative;
  isolation: isolate;
  width: 100%;
  min-height: 540px;
  border-radius: 0.5rem;
  border-color: color-mix(in oklab, var(--quota-accent) 28%, var(--border));
  background-color: color-mix(in oklab, var(--quota-accent) 4%, var(--card));
  box-shadow: inset 0 3px 0 color-mix(in oklab, var(--quota-accent) 72%, transparent);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.quota-offer-card:hover {
  border-color: color-mix(in oklab, var(--quota-accent) 48%, var(--border));
  box-shadow: inset 0 3px 0 color-mix(in oklab, var(--quota-accent) 82%, transparent), 0 8px 24px rgb(15 23 42 / 0.06);
}

.quota-offer-card[data-category='gpt'] { --quota-accent: #7c3aed; }
.quota-offer-card[data-category='claude'] { --quota-accent: #dc5f45; }
.quota-offer-card[data-category='gemini'] { --quota-accent: #2563eb; }
.quota-offer-card[data-category='cursor'] { --quota-accent: #0891b2; }
.quota-offer-card[data-category='perplexity'] { --quota-accent: #059669; }
.quota-offer-card[data-category='other'] { --quota-accent: #64748b; }

.quota-offer-icon-well {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  flex: none;
  place-items: center;
  border: 1px solid color-mix(in oklab, var(--quota-accent) 28%, var(--border));
  border-radius: 0.5rem;
  color: var(--quota-accent);
  background-color: color-mix(in oklab, var(--quota-accent) 10%, var(--card));
}

.quota-offer-category {
  border-color: color-mix(in oklab, var(--quota-accent) 36%, var(--border));
  color: var(--quota-accent);
  background-color: color-mix(in oklab, var(--quota-accent) 8%, var(--card));
}

.quota-offer-price { color: var(--quota-accent); }

.quota-offer-watermark {
  position: absolute;
  top: 3.75rem;
  right: -1rem;
  z-index: 0;
  width: 7.5rem;
  height: 7.5rem;
  object-fit: contain;
  opacity: 0.055;
  pointer-events: none;
}

.quota-offer-metrics {
  border-block: 1px solid color-mix(in oklab, var(--quota-accent) 16%, var(--border));
  background-color: color-mix(in oklab, var(--quota-accent) 14%, var(--border));
}

.quota-offer-metrics > div {
  min-width: 0;
  padding: 0.75rem;
  background-color: color-mix(in oklab, var(--quota-accent) 3%, var(--card));
}

.quota-offer-metrics dt { font-size: 0.75rem; color: var(--muted-foreground); }
.quota-offer-metrics dd { margin-top: 0.25rem; overflow-wrap: anywhere; font-weight: 600; }
</style>
