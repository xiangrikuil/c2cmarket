<script setup lang="ts">
import { computed } from 'vue'
import { PackageOpen, ShieldCheck, ShoppingCart } from 'lucide-vue-next'
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
const statusLabel = computed(() => {
  if (props.offer.isOrderable) return isRush.value ? '正在抢购' : '可购买'
  if (props.offer.orderabilityCode === 'sold_out') return '已售罄'
  if (props.offer.orderabilityCode === 'not_started') return '未开售'
  if (props.offer.orderabilityCode === 'round_ended') return '本轮结束'
  if (props.offer.orderabilityCode === 'batch_expired') return '已过期'
  return '不可购买'
})
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
const sellerInitial = computed(() => props.offer.sellerDisplayName.trim().slice(0, 1).toUpperCase() || '店')
const stockLabel = computed(() => isRush.value
  ? `${props.offer.availableCopies} 份`
  : props.offer.saleMode === 'scheduled'
    ? `本轮剩余 ${props.offer.availableCopies} 份`
    : `剩余 ${props.offer.availableCopies} 份`)
const promptAuditEnabled = computed(() => props.offer.promptAuditEnabled ?? null)

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
    class="quota-offer-card api-product-card gap-0 py-0"
    :data-category="category"
  >
    <div class="api-product-card__frame">
      <div class="api-product-card__header">
        <div class="flex items-start gap-2.5">
          <span class="api-product-card__icon">
            <img v-if="iconSrc" :src="iconSrc" alt="" class="h-6 w-6 object-contain" />
            <PackageOpen v-else class="h-5 w-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge variant="outline" class="api-product-card__category">{{ categoryLabel }}</Badge>
              <Badge :variant="statusVariant" :title="offer.isOrderable ? statusLabel : offer.orderabilityReason">{{ statusLabel }}</Badge>
              <Badge v-if="!isRush" variant="secondary">{{ getApiQuotaSaleModeLabel(offer.saleMode) }}</Badge>
            </div>
            <h2 class="mt-1.5 truncate text-sm font-semibold text-slate-950" :title="offer.name">{{ offer.name }}</h2>
            <p class="truncate text-xs text-muted-foreground" :title="`${offer.serviceTitle} · ${getApiQuotaDistributionLabel(offer.distributionSystem)}`">{{ offer.serviceTitle }} · {{ getApiQuotaDistributionLabel(offer.distributionSystem) }}</p>
          </div>
        </div>
        <div class="mt-2 flex items-end justify-between gap-2">
          <div>
            <div class="api-product-card__price text-xl font-semibold">¥{{ formatDecimal(offer.priceCny, 2, 2) }}</div>
            <div class="text-[11px] text-muted-foreground">一次购买 · ${{ formatDecimal(offer.usdAllowance, 0, 6) }} 美元额度</div>
          </div>
          <div class="pb-0.5 text-right text-[11px] text-muted-foreground">¥{{ formatDecimal(offer.cnyPerUsd, 3, 6) }} / $1</div>
        </div>
      </div>

      <ApiQuotaPolicyStrip
        :policy="offer.quotaUsagePolicy"
        :expiry-value="formatAbsoluteTime(offer.expiresAt)"
      />
      <div v-if="$slots.health"><slot name="health" /></div>

      <dl class="api-product-card__technical-facts">
        <div><dt>模型倍率</dt><dd>{{ Number(offer.modelMultiplier).toFixed(2) }}x</dd></div>
        <div><dt>最大并发</dt><dd>{{ offer.declaredMaxConcurrency }}</dd></div>
        <div><dt>提示词审计</dt><dd :class="promptAuditEnabled === true ? 'text-orange-700' : ''">{{ promptAuditEnabled === null ? '未声明' : promptAuditEnabled ? '开启' : '关闭' }}</dd></div>
        <div><dt>交付方式</dt><dd :title="getApiQuotaDeliveryModeLabel(offer.deliveryMode)">{{ getApiQuotaDeliveryModeLabel(offer.deliveryMode) }}</dd></div>
      </dl>

      <div class="api-product-card__details flex-1">
        <dl class="api-product-card__detail-facts">
          <div><dt>{{ isRush ? '本轮库存' : '剩余库存' }}</dt><dd>{{ stockLabel }}</dd></div>
          <div><dt>预计交付</dt><dd>≤ {{ offer.deliveryEtaMinutes }} 分钟</dd></div>
          <div v-if="offer.deliveryMode === 'preimported'"><dt>可交付凭据</dt><dd>{{ offer.credentialAvailableCopies }} 份</dd></div>
          <div v-if="isRush"><dt>有效剩余</dt><dd :title="formatAbsoluteTime(offer.expiresAt)">约 {{ apiQuotaDurationLabel(offer.expiresAt, now) }}</dd></div>
          <div v-else><dt>销售截止</dt><dd :title="`${formatAbsoluteTime(offer.saleCutoffAt)} · ${apiQuotaOfferCountdown(offer, now)}`">{{ formatAbsoluteTime(offer.saleCutoffAt) }}</dd></div>
          <div><dt>接入系统</dt><dd>{{ getApiQuotaDistributionLabel(offer.distributionSystem) }}</dd></div>
        </dl>
      </div>

      <div class="api-product-card__merchant">
        <div class="flex min-w-0 items-center gap-2">
          <span class="api-product-card__merchant-avatar" aria-hidden="true">{{ sellerInitial }}</span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-xs font-semibold" :title="offer.sellerDisplayName">{{ offer.sellerDisplayName }}</div>
            <div class="mt-0.5 text-[10px] text-muted-foreground">{{ sellerType }} · API 商家</div>
          </div>
          <Badge variant="outline" class="shrink-0 text-[10px]" :class="offer.sellerLinuxDoBound ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'text-muted-foreground'">
            <ShieldCheck class="h-3 w-3" />{{ offer.sellerLinuxDoBound ? 'linux.do 已绑定' : '未绑定 linux.do' }}
          </Badge>
        </div>
      </div>

      <div class="api-product-card__action">
        <Button :disabled="purchaseDisabled" class="h-9 w-full" @click="emit('purchase', offer)">
          <ShoppingCart class="h-4 w-4" />{{ purchaseLabel }}
        </Button>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.quota-offer-card {
  --api-product-accent: #64748b;
}

.quota-offer-card[data-category='gpt'] { --api-product-accent: #10a37f; }
.quota-offer-card[data-category='claude'] { --api-product-accent: #d97757; }
.quota-offer-card[data-category='gemini'] { --api-product-accent: #2563eb; }
.quota-offer-card[data-category='cursor'] { --api-product-accent: #0891b2; }
.quota-offer-card[data-category='perplexity'] { --api-product-accent: #059669; }
.quota-offer-card[data-category='other'] { --api-product-accent: #64748b; }
</style>
